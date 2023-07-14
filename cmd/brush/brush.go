package brush

import (
	"fmt"
	"math/rand"
	"os"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/sagan/ptool/client"
	"github.com/sagan/ptool/cmd"
	"github.com/sagan/ptool/config"
	"github.com/sagan/ptool/site"
	"github.com/sagan/ptool/stats"
	"github.com/sagan/ptool/torrentutil"
	"github.com/sagan/ptool/utils"
)

var command = &cobra.Command{
	Use:   "brush <client> <siteOrGroup>...",
	Short: "Brush sites using client.",
	Long:  `Brush sites using client.`,
	Args:  cobra.MatchAll(cobra.MinimumNArgs(2), cobra.OnlyValidArgs),
	Run:   brush,
}

var (
	dryRun    = false
	addPaused = false
	ordered   = false
	force     = false
	maxSites  = int64(0)
)

func init() {
	command.Flags().BoolVarP(&dryRun, "dry-run", "d", false, "Dry run. Do not actually controlling client")
	command.Flags().BoolVarP(&addPaused, "add-paused", "", false, "Add torrents to client in paused state")
	command.Flags().BoolVarP(&ordered, "ordered", "o", false, "Brush sites provided in order")
	command.Flags().BoolVarP(&force, "force", "", false, "Force mode. Ignore _noadd flag in client")
	command.Flags().Int64VarP(&maxSites, "max-sites", "", 0, "Allowed max succcess sites number, Default (0) == unlimited")
	cmd.RootCmd.AddCommand(command)
}

func brush(cmd *cobra.Command, args []string) {
	clientInstance, err := client.CreateClient(args[0])
	if err != nil {
		log.Fatal(err)
	}
	if clientInstance.GetClientConfig().Type == "transmission" {
		log.Warnf("Warning: brush function of transmission client has NOT been tested")
	}
	sitenames := config.ParseGroupAndOtherNamesWithoutDeduplicate(args[1:]...)

	if !ordered {
		rand.Shuffle(len(sitenames), func(i, j int) { sitenames[i], sitenames[j] = sitenames[j], sitenames[i] })
	}
	cntSuccessSite := int64(0)
	cntSkipSite := int64(0)
	cntAddTorrents := int64(0)
	cntDeleteTorrents := int64(0)
	doneSiteFlag := map[string](bool){}
	tmpdir, _ := os.MkdirTemp(os.TempDir(), "ptool")
	var statDb *stats.StatDb
	if config.Get().BrushEnableStats {
		statDb, err = stats.NewDb(config.ConfigDir + "/" + config.STATS_FILENAME)
		if err != nil {
			log.Warnf("Failed to create stats db: %v.", err)
		}
	}

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
		noadd := !force && status.NoAdd
		var siteTorrents []site.Torrent
		if status.UploadSpeedLimit > 0 && (status.UploadSpeedLimit < SLOW_UPLOAD_SPEED ||
			(float64(status.UploadSpeed)/float64(status.UploadSpeedLimit)) >= BANDWIDTH_FULL_PERCENT) {
			log.Printf(
				"Client %s upload bandwidth is already full (Upload speed/limit: %s/s/%s/s). Do not fetch site new torrents\n",
				clientInstance.GetName(),
				utils.BytesSize(float64(status.UploadSpeed)),
				utils.BytesSize(float64(status.UploadSpeedLimit)),
			)
		} else if !siteInstance.GetSiteConfig().BrushAllowHr && siteInstance.GetSiteConfig().GlobalHnR {
			log.Printf("Site %s enforces global HnR. Do not fetch site new torrents", sitename)
		} else if noadd {
			log.Printf("Client %s in NoAdd status. Do not fetch site new torrents", clientInstance.GetName())
		} else {
			siteTorrents, err = siteInstance.GetLatestTorrents(true)
			if err != nil {
				log.Printf("failed to fetch site %s torrents: %v", sitename, err)
			}
		}

		clientTorrents, err := clientInstance.GetTorrents("", config.BRUSH_CAT, true)
		if err != nil {
			log.Printf("Failed to get client %s torrents: %v ", clientInstance.GetName(), err)
			continue
		}
		brushOption := &BrushOptionStruct{
			TorrentMinSizeLimit:     siteInstance.GetSiteConfig().BrushTorrentMinSizeLimitValue,
			TorrentMaxSizeLimit:     siteInstance.GetSiteConfig().BrushTorrentMaxSizeLimitValue,
			TorrentUploadSpeedLimit: siteInstance.GetSiteConfig().TorrentUploadSpeedLimitValue,
			AllowNoneFree:           siteInstance.GetSiteConfig().BrushAllowNoneFree,
			AllowPaid:               siteInstance.GetSiteConfig().BrushAllowPaid,
			AllowHr:                 siteInstance.GetSiteConfig().BrushAllowHr,
			AllowZeroSeeders:        siteInstance.GetSiteConfig().BrushAllowZeroSeeders,
			MinDiskSpace:            clientInstance.GetClientConfig().BrushMinDiskSpaceValue,
			SlowUploadSpeedTier:     clientInstance.GetClientConfig().BrushSlowUploadSpeedTierValue,
			MaxDownloadingTorrents:  clientInstance.GetClientConfig().BrushMaxDownloadingTorrents,
			MaxTorrents:             clientInstance.GetClientConfig().BrushMaxTorrents,
			MinRatio:                clientInstance.GetClientConfig().BrushMinRatio,
			DefaultUploadSpeedLimit: clientInstance.GetClientConfig().BrushDefaultUploadSpeedLimitValue,
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

		// delete
		for _, torrent := range result.DeleteTorrents {
			clientTorrent := utils.FindInSlice(clientTorrents, func(t client.Torrent) bool {
				return t.InfoHash == torrent.InfoHash
			})
			// double check
			if clientTorrent == nil || clientTorrent.Category != config.BRUSH_CAT {
				log.Warnf("Invalid torrent deletion target: %s", torrent.InfoHash)
				continue
			}
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
			err := clientInstance.DeleteTorrents([]string{torrent.InfoHash}, true)
			log.Printf("Delete torrent result: error=%v", err)
			if err == nil {
				cntDeleteTorrents++
				if statDb != nil {
					statDb.AddTorrentStat(brushOption.Now, 1, &stats.TorrentStat{
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

		// stall
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

		// resume
		if len(result.ResumeTorrents) > 0 {
			for _, torrent := range result.ResumeTorrents {
				log.Printf("Resume client %s torrent: %v / %v / %v", clientInstance.GetName(), torrent.Name, torrent.InfoHash, torrent.Msg)
			}
			if !dryRun {
				err := clientInstance.ResumeTorrents(utils.Map(result.ResumeTorrents, func(t AlgorithmOperationTorrent) string {
					return t.InfoHash
				}))
				log.Printf("Resume torrents result: error=%v", err)
			}
		}

		// modify
		for _, torrent := range result.ModifyTorrents {
			log.Printf("Modify client %s torrent: %v / %v / %v / %v ", clientInstance.GetName(), torrent.Name, torrent.InfoHash, torrent.Msg, torrent.Meta)
			if dryRun {
				continue
			}
			err := clientInstance.ModifyTorrent(torrent.InfoHash, nil, torrent.Meta)
			log.Printf("Modify torrent result: error=%v", err)
		}

		// add
		cndAddTorrents := 0
		for _, torrent := range result.AddTorrents {
			log.Printf("Add site %s torrent to client %s: %s / %s / %v", siteInstance.GetName(), clientInstance.GetName(), torrent.Name, torrent.Msg, torrent.Meta)
			if dryRun {
				continue
			}
			torrentdata, _, err := siteInstance.DownloadTorrent(torrent.DownloadUrl)
			if err != nil {
				log.Printf("Failed to download: %s. Skip \n", err)
				continue
			}
			tinfo, err := torrentutil.ParseTorrent(torrentdata, 99)
			if err != nil {
				continue
			}
			pClientTorrent, _ := clientInstance.GetTorrent(tinfo.InfoHash)
			if pClientTorrent != nil {
				log.Printf("Already existing in client. skip\n")
				continue
			}
			if clientInstance.TorrentRootPathExists(tinfo.RootDir) {
				log.Printf("torrent rootpath %s existing in client. skip\n", tinfo.RootDir)
				continue
			}
			log.Printf("torrent info: %s\n", tinfo.InfoHash)
			cndAddTorrents++
			torrentOption := &client.TorrentOption{
				Name:             torrent.Name,
				Pause:            addPaused,
				Category:         config.BRUSH_CAT,
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

		if len(result.AddTorrents) > 0 {
			cntSuccessSite++
		} else {
			cntSkipSite++
		}
		if noadd {
			log.Printf("Client in NoAdd status. Skip follow sites.")
			cntSkipSite += int64(len(sitenames) - 1 - i)
			break
		}
		if maxSites > 0 && cntSuccessSite >= maxSites {
			log.Printf("MaxSites reached. Stop brushing.")
			cntSkipSite += int64(len(sitenames) - 1 - i)
			break
		}
		if !result.CanAddMore {
			log.Printf("Client capacity is full. Stop brushing.")
			cntSkipSite += int64(len(sitenames) - 1 - i)
			break
		}
		if i < len(sitenames)-1 && (len(result.AddTorrents) > 0 || len(result.ModifyTorrents) > 0 || len(result.DeleteTorrents) > 0 || len(result.StallTorrents) > 0) {
			clientInstance.PurgeCache()
			utils.Sleep(3)
		}
	}

	fmt.Printf("Finish brushing %d sites: successSites=%d, skipSites=%d; Added / Deleted torrents: %d / %d to client %s\n",
		len(sitenames), cntSuccessSite, cntSkipSite, cntAddTorrents, cntDeleteTorrents, clientInstance.GetName())
	os.RemoveAll(tmpdir)
	clientInstance.Close()
	if cntSuccessSite == 0 {
		os.Exit(1)
	}
}
