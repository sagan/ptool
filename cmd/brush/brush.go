package brush

import (
	"bytes"
	"fmt"
	"math/rand"
	"os"

	goTorrentParser "github.com/j-muller/go-torrent-parser"
	log "github.com/sirupsen/logrus"

	"github.com/sagan/ptool/client"
	"github.com/sagan/ptool/cmd"
	"github.com/sagan/ptool/config"
	"github.com/sagan/ptool/site"
	"github.com/sagan/ptool/stats"
	"github.com/sagan/ptool/utils"
	"github.com/spf13/cobra"
)

const (
	CAT = "_brush"
)

var command = &cobra.Command{
	Use:   "brush <client> <site>...",
	Short: "Brush site using client.",
	Long:  `Brush site using client.`,
	Args:  cobra.MatchAll(cobra.MinimumNArgs(2), cobra.OnlyValidArgs),
	Run:   brush,
}

var (
	dryRun = false
	paused = false
)

func init() {
	command.Flags().BoolVar(&dryRun, "dry-run", false, "Dry run. Do not actually controlling client")
	command.Flags().BoolVar(&paused, "paused", false, "Add torrents to client in paused state")
	cmd.RootCmd.AddCommand(command)
}

func brush(cmd *cobra.Command, args []string) {
	clientInstance, err := client.CreateClient(args[0])
	if err != nil {
		log.Fatal(err)
	}
	sitenames := args[1:]

	rand.Shuffle(len(sitenames), func(i, j int) { sitenames[i], sitenames[j] = sitenames[j], sitenames[i] })
	cntSuccessSite := int64(0)
	cntAddTorrents := int64(0)
	cntDeleteTorrents := int64(0)
	doneSiteFlag := make(map[string](bool))
	tmpdir, _ := os.MkdirTemp(os.TempDir(), "ptool")

	for i, sitename := range sitenames {
		if doneSiteFlag[sitename] {
			continue
		}
		doneSiteFlag[sitename] = true
		siteInstance, err := site.CreateSite(sitename)
		if err != nil {
			log.Errorf("Failed to get instance of site %s: %v", sitename, err)
			continue
		}
		log.Printf("Brush client %s site %s", clientInstance.GetName(), sitename)
		status, err := clientInstance.GetStatus()
		if err != nil {
			log.Printf("Failed to get client %s status: %v", clientInstance.GetName(), err)
			continue
		}
		var siteTorrents []site.Torrent
		if status.UploadSpeedLimit > 0 && status.UploadSpeedLimit < 100*1024 ||
			status.UploadSpeed > 0 &&
				(float64(status.UploadSpeed)/float64(status.UploadSpeedLimit)) >= 0.8 {
			log.Printf(
				"Client %s upload bandwidth is already full (Upload speed/limit: %s/s/%s/s). Do not fetch site new torrents\n",
				clientInstance.GetName(),
				utils.BytesSize(float64(status.UploadSpeed)),
				utils.BytesSize(float64(status.UploadSpeedLimit)),
			)
		} else if siteInstance.GetSiteConfig().GlobalHnR {
			log.Printf("Site %s enforces global HnR. Do not fetch site new torrents", sitename)
		} else {
			siteTorrents, err = siteInstance.GetLatestTorrents()
			if err != nil {
				log.Printf("failed to fetch site %s torrents: %v", sitename, err)
			}
		}

		clientTorrents, err := clientInstance.GetTorrents("", CAT, true)
		if err != nil {
			log.Printf("Failed to get client %s torrents: %v ", clientInstance.GetName(), err)
			continue
		}
		brushOption := &BrushOptionStruct{
			MinDiskSpace:            clientInstance.GetClientConfig().BrushMinDiskSpaceValue,
			SlowUploadSpeedTier:     clientInstance.GetClientConfig().BrushSlowUploadSpeedTierValue,
			TorrentUploadSpeedLimit: siteInstance.GetSiteConfig().TorrentUploadSpeedLimitValue,
			MaxDownloadingTorrents:  clientInstance.GetClientConfig().BrushMaxDownloadingTorrents,
			MaxTorrents:             clientInstance.GetClientConfig().BrushMaxTorrents,
			MinRatio:                clientInstance.GetClientConfig().BrushMinRatio,
			Now:                     utils.Now(),
		}
		log.Printf(
			"Brush Options: minDiskSpace=%v, slowUploadSpeedTier=%v, torrentUploadSpeedLimit=%v/s,"+
				" maxDownloadingTorrents=%d, maxTorrents=%d, minRatio=%f",
			utils.BytesSize(float64(brushOption.MinDiskSpace)),
			utils.BytesSize(float64(brushOption.SlowUploadSpeedTier)),
			utils.BytesSize(float64(brushOption.TorrentUploadSpeedLimit)),
			brushOption.MaxDownloadingTorrents,
			brushOption.MaxTorrents,
			brushOption.MinRatio,
		)
		result := Decide(status, clientTorrents, siteTorrents, brushOption)
		log.Printf(
			"Current client %s torrents: %d; Download speed / limit: %s/s / %s/s; Upload speed / limit: %s/s / %s/s;Free disk space: %s;",
			clientInstance.GetName(),
			len(clientTorrents),
			utils.BytesSize(float64(status.DownloadSpeed)),
			utils.BytesSize(float64(status.DownloadSpeedLimit)),
			utils.BytesSize(float64(status.UploadSpeed)),
			utils.BytesSize(float64(status.UploadSpeedLimit)),
			utils.BytesSize(float64(status.FreeSpaceOnDisk)),
		)
		log.Printf(
			"Fetched site %s torrents: %d; Client add / modify / stall / delete torrents: %d / %d / %d / %d. Msg: %s",
			siteInstance.GetName(),
			len(siteTorrents),
			len(result.AddTorrents),
			len(result.ModifyTorrents),
			len(result.StallTorrents),
			len(result.DeleteTorrents),
			result.Msg,
		)

		cndAddTorrents := 0
		for _, torrent := range result.AddTorrents {
			log.Printf("Add site %s torrent to client %s: %s / %s / %v", siteInstance.GetName(), clientInstance.GetName(), torrent.Name, torrent.Msg, torrent.Meta)
			if dryRun {
				continue
			}
			torrentdata, err := siteInstance.DownloadTorrent(torrent.DownloadUrl)
			if err != nil {
				log.Printf("Failed to download: %s. Skip \n", err)
				continue
			}
			tinfo, err := goTorrentParser.Parse(bytes.NewReader(torrentdata))
			if err != nil {
				continue
			}
			pClientTorrent := utils.FindInSlice(clientTorrents, func(ts client.Torrent) bool {
				return ts.InfoHash == tinfo.InfoHash
			})
			if pClientTorrent != nil {
				log.Printf("Already existing in client. skip\n")
				continue
			}
			torrentRootPath := ""
			if len(tinfo.Files) > 0 {
				fp := tinfo.Files[0].Path
				if len(fp) > 0 {
					torrentRootPath = fp[0]
				}
			}
			if clientInstance.TorrentRootPathExists(torrentRootPath) {
				log.Printf("torrent rootpath %s existing in client. skip\n", torrentRootPath)
				continue
			}
			log.Printf("torrent info: %s\n", tinfo.InfoHash)
			cndAddTorrents++
			torrentOption := &client.TorrentOption{
				Name:             torrent.Name,
				Paused:           paused,
				Category:         CAT,
				Tags:             []string{client.GenerateTorrentTagFromSite(siteInstance.GetName())},
				UploadSpeedLimit: siteInstance.GetSiteConfig().TorrentUploadSpeedLimitValue,
			}
			// torrentname := fmt.Sprint(torrent.Name, "_", i, ".torrent")
			// os.WriteFile(tmpdir+"/"+torrentname, torrentdata, 0777)
			if !dryRun {
				err = clientInstance.AddTorrent(torrentdata, torrentOption, torrent.Meta)
				log.Printf("Add torrent result: error=%v", err)
				if err == nil {
					cntAddTorrents++
				}
			}
		}

		for _, torrent := range result.ModifyTorrents {
			log.Printf("Modify client %s torrent: %v / %v / %v / %v ", clientInstance.GetName(), torrent.Name, torrent.InfoHash, torrent.Msg, torrent.Meta)
			if dryRun {
				continue
			}
			err := clientInstance.ModifyTorrent(torrent.InfoHash, nil, torrent.Meta)
			log.Printf("Modify torrent result: error=%v", err)
		}

		for _, torrent := range result.StallTorrents {
			log.Printf("Stall client %s torrent: %v / %v / %v", clientInstance.GetName(), torrent.Name, torrent.InfoHash, torrent.Msg)
			if dryRun {
				continue
			}
			err := clientInstance.ModifyTorrent(torrent.InfoHash, &client.TorrentOption{
				DownloadSpeedLimit: STALL_DOWNLOAD_SPEED,
			}, torrent.Meta)
			log.Printf("Stall torrent result: error=%v", err)
		}

		for _, torrent := range result.DeleteTorrents {
			clientTorrent := utils.FindInSlice(clientTorrents, func(t client.Torrent) bool {
				return t.InfoHash == torrent.InfoHash
			})
			duration := brushOption.Now - clientTorrent.Atime
			log.Printf("Delete client %s torrent: %v / %v / %v.", clientInstance.GetName(), torrent.Name, torrent.InfoHash, torrent.Msg)
			log.Printf("Torrent total downloads / uploads: %s / %s; Lifespan: %s; Average download / upload speed of lifespan: %s/s / %s/s",
				utils.BytesSize(float64(clientTorrent.Downloaded)),
				utils.BytesSize(float64(clientTorrent.Uploaded)),
				utils.GetDurationString(duration),
				utils.BytesSize(float64(clientTorrent.Downloaded)/float64(duration)),
				utils.BytesSize(float64(clientTorrent.Uploaded)/float64(duration)),
			)
			if dryRun {
				continue
			}
			err := clientInstance.DeleteTorrents([]string{torrent.InfoHash})
			log.Printf("Delete torrent result: error=%v", err)
			if err == nil {
				cntDeleteTorrents++
				if config.Get().BrushEnableStats {
					stats.Db.AddTorrentStat(brushOption.Now, 1, &stats.TorrentStat{
						Client:     clientInstance.GetName(),
						Site:       clientTorrent.GetSiteFromTag(),
						InfoHash:   clientTorrent.InfoHash,
						Category:   clientTorrent.Category,
						Name:       clientTorrent.Name,
						Atime:      clientTorrent.Atime,
						Size:       clientTorrent.Size,
						Uploaded:   clientTorrent.Uploaded,
						Downloaded: clientTorrent.Downloaded,
						Msg:        torrent.Msg,
					})
				}
			}
		}
		cntSuccessSite++
		if !result.CanAddMore {
			log.Printf("Client capacity is full. Stop brushing")
			break
		}
		if i < len(sitenames)-1 && cndAddTorrents > 0 || len(result.ModifyTorrents) > 0 || len(result.DeleteTorrents) > 0 || len(result.StallTorrents) > 0 {
			clientInstance.PurgeCache()
			utils.Sleep(3)
		}
	}

	fmt.Printf("Finish brushing %d sites, successSites=%d; Added / Deleted torrents: %d / %d to client %s\n",
		len(sitenames), cntSuccessSite, cntAddTorrents, cntDeleteTorrents, clientInstance.GetName())
	os.RemoveAll(tmpdir)
	if cntSuccessSite == 0 {
		os.Exit(1)
	}
}
