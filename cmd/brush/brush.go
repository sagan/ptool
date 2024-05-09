package brush

import (
	"fmt"
	"math/rand"
	"path/filepath"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/sagan/ptool/client"
	"github.com/sagan/ptool/cmd"
	"github.com/sagan/ptool/cmd/brush/strategy"
	"github.com/sagan/ptool/config"
	"github.com/sagan/ptool/site"
	"github.com/sagan/ptool/stats"
	"github.com/sagan/ptool/util"
	"github.com/sagan/ptool/util/torrentutil"
)

var command = &cobra.Command{
	Use:         "brush {client} {site | group}...",
	Annotations: map[string]string{"cobra-prompt-dynamic-suggestions": "brush"},
	Short:       "Brush sites using client.",
	Long:        `Brush sites using client.`,
	Args:        cobra.MatchAll(cobra.MinimumNArgs(2), cobra.OnlyValidArgs),
	RunE:        brush,
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
	command.Flags().BoolVarP(&ordered, "ordered", "", false, "Brush sites provided in order")
	command.Flags().BoolVarP(&force, "force", "", false, `Force mode. Ignore "`+config.NOADD_TAG+`" flag tag in client`)
	command.Flags().Int64VarP(&maxSites, "max-sites", "", -1, "Allowed max succcess sites number, -1 == no limit")
	cmd.RootCmd.AddCommand(command)
}

func brush(cmd *cobra.Command, args []string) (err error) {
	clientName := args[0]
	sitenames := config.ParseGroupAndOtherNamesWithoutDeduplicate(args[1:]...)
	clientInstance, err := client.CreateClient(clientName)
	if err != nil {
		return err
	}
	lock, err := config.LockConfigDirFile(fmt.Sprintf(config.CLIENT_LOCK_FILE, clientName))
	if err != nil {
		return err
	}
	defer lock.Unlock()
	if clientInstance.GetClientConfig().Type == "transmission" {
		log.Warnf("Warning: brush function of transmission client has NOT been tested")
	}
	if !ordered {
		rand.Shuffle(len(sitenames), func(i, j int) { sitenames[i], sitenames[j] = sitenames[j], sitenames[i] })
	}
	cntSuccessSite := int64(0)
	cntSkipSite := int64(0)
	cntAddTorrents := int64(0)
	cntDeleteTorrents := int64(0)
	doneSiteFlag := map[string]bool{}
	var statDb *stats.StatDb
	if config.Get().BrushEnableStats {
		statDb, err = stats.NewDb(filepath.Join(config.ConfigDir, config.STATS_FILENAME))
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
		var siteTorrents []*site.Torrent
		if status.UploadSpeedLimit > 0 && (status.UploadSpeedLimit < strategy.SLOW_UPLOAD_SPEED ||
			(float64(status.UploadSpeed)/float64(status.UploadSpeedLimit)) >= strategy.BANDWIDTH_FULL_PERCENT) {
			log.Printf(
				"Client %s upload bandwidth is already full (Upload speed/limit: %s/s/%s/s). Do not fetch site new torrents\n",
				clientInstance.GetName(),
				util.BytesSize(float64(status.UploadSpeed)),
				util.BytesSize(float64(status.UploadSpeedLimit)),
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
		brushSiteOption := strategy.GetBrushSiteOptions(siteInstance, util.Now())
		brushClientOption := strategy.GetBrushClientOptions(clientInstance)
		log.Printf(
			"Brush Options: minDiskSpace=%v, slowUploadSpeedTier=%v, torrentUploadSpeedLimit=%v/s,"+
				" maxDownloadingTorrents=%d, maxTorrents=%d, minRatio=%f",
			util.BytesSize(float64(brushClientOption.MinDiskSpace)),
			util.BytesSize(float64(brushClientOption.SlowUploadSpeedTier)),
			util.BytesSize(float64(brushSiteOption.TorrentUploadSpeedLimit)),
			brushClientOption.MaxDownloadingTorrents,
			brushClientOption.MaxTorrents,
			brushClientOption.MinRatio,
		)
		result := strategy.Decide(status, clientTorrents, siteTorrents, brushSiteOption, brushClientOption)
		log.Printf(
			"Current client %s torrents: %d; Download speed / limit: %s/s / %s/s; Upload speed / limit: %s/s / %s/s;Free disk space: %s;",
			clientInstance.GetName(),
			len(clientTorrents),
			util.BytesSize(float64(status.DownloadSpeed)),
			util.BytesSize(float64(status.DownloadSpeedLimit)),
			util.BytesSize(float64(status.UploadSpeed)),
			util.BytesSize(float64(status.UploadSpeedLimit)),
			util.BytesSize(float64(status.FreeSpaceOnDisk)),
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
			clientTorrent := *util.FindInSlice(clientTorrents, func(t *client.Torrent) bool {
				return t.InfoHash == torrent.InfoHash
			})
			// double check
			if clientTorrent == nil || clientTorrent.Category != config.BRUSH_CAT {
				log.Warnf("Invalid torrent deletion target: %s", torrent.InfoHash)
				continue
			}
			duration := brushSiteOption.Now - clientTorrent.Atime
			log.Printf("Delete client %s torrent: %v / %v / %v.", clientInstance.GetName(), torrent.Name, torrent.InfoHash, torrent.Msg)
			log.Printf("Torrent total downloads / uploads: %s / %s; Lifespan: %s; Average download / upload speed of lifespan: %s/s / %s/s",
				util.BytesSize(float64(clientTorrent.Downloaded)),
				util.BytesSize(float64(clientTorrent.Uploaded)),
				util.GetDurationString(duration),
				util.BytesSize(float64(clientTorrent.Downloaded)/float64(duration)),
				util.BytesSize(float64(clientTorrent.Uploaded)/float64(duration)),
			)
			if dryRun {
				continue
			}
			err := client.DeleteTorrentsAuto(clientInstance, []string{torrent.InfoHash})
			log.Printf("Delete torrent result: error=%v", err)
			if err == nil {
				cntDeleteTorrents++
				if statDb != nil {
					statDb.AddTorrentStat(brushSiteOption.Now, 1, &stats.TorrentStat{
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
				DownloadSpeedLimit: strategy.STALL_DOWNLOAD_SPEED,
			}, torrent.Meta)
			log.Printf("Stall torrent result: error=%v", err)
		}

		// resume
		if len(result.ResumeTorrents) > 0 {
			for _, torrent := range result.ResumeTorrents {
				log.Printf("Resume client %s torrent: %v / %v / %v", clientInstance.GetName(), torrent.Name, torrent.InfoHash, torrent.Msg)
			}
			if !dryRun {
				err := clientInstance.ResumeTorrents(util.Map(result.ResumeTorrents, func(t strategy.AlgorithmOperationTorrent) string {
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
			torrentdata, _, _, err := siteInstance.DownloadTorrent(torrent.DownloadUrl)
			if err != nil {
				log.Printf("Failed to download: %s. Skip \n", err)
				continue
			}
			tinfo, err := torrentutil.ParseTorrent(torrentdata)
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
			tags := []string{client.GenerateTorrentTagFromSite(siteInstance.GetName())}
			if tinfo.IsPrivate() {
				tags = append(tags, config.PRIVATE_TAG)
			} else {
				tags = append(tags, config.PUBLIC_TAG)
			}
			torrentOption := &client.TorrentOption{
				Name:             torrent.Name,
				Pause:            addPaused,
				Category:         config.BRUSH_CAT,
				Tags:             tags,
				UploadSpeedLimit: siteInstance.GetSiteConfig().TorrentUploadSpeedLimitValue,
			}
			// torrentname := fmt.Sprint(torrent.Name, "_", i, ".torrent")
			// os.WriteFile(tmpdir+"/"+torrentname, torrentdata, constants.PERM)
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
		if maxSites >= 0 && cntSuccessSite >= maxSites {
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
			util.Sleep(3)
		}
	}

	fmt.Printf("Finish brushing %d sites: successSites=%d, skipSites=%d; Added / Deleted torrents: %d / %d to client %s\n",
		len(sitenames), cntSuccessSite, cntSkipSite, cntAddTorrents, cntDeleteTorrents, clientInstance.GetName())
	if cntSuccessSite == 0 {
		return fmt.Errorf("no sites successed")
	}
	return nil
}
