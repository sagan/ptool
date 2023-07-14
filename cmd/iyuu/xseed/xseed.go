package xseed

import (
	"fmt"
	"sort"
	"strings"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"golang.org/x/exp/slices"
	"gorm.io/gorm/clause"

	"github.com/sagan/ptool/client"
	"github.com/sagan/ptool/cmd/iyuu"
	"github.com/sagan/ptool/config"
	"github.com/sagan/ptool/site"
	"github.com/sagan/ptool/torrentutil"
	"github.com/sagan/ptool/utils"
)

var command = &cobra.Command{
	Use:   "xseed <client>...",
	Short: "Cross seed using iyuu API.",
	Long: `Cross seed using iyuu API.
By default it will add xseed torrents from All sites unless --include-sites or --exclude-sites flag is set.`,
	Args: cobra.MatchAll(cobra.MinimumNArgs(1), cobra.OnlyValidArgs),
	Run:  xseed,
}

var (
	dryRun                 = false
	addPaused              = false
	check                  = false
	slowMode               = false
	maxXseedTorrents       = int64(0)
	iyuuRequestMaxTorrents = int64(0)
	includeSites           = ""
	excludeSites           = ""
	category               = ""
	addCategory            = ""
	addTags                = ""
	tag                    = ""
	filter                 = ""
	minTorrentSizeStr      = ""
	maxTorrentSizeStr      = ""
	iyuuRequestServer      = ""
)

func init() {
	command.Flags().BoolVarP(&slowMode, "slow", "", false, "Slow mode. wait after handling each xseed torrent. For dev / test purpose")
	command.Flags().BoolVarP(&dryRun, "dry-run", "d", false, "Dry run. Do NOT actually add xseed torrents to client")
	command.Flags().BoolVarP(&addPaused, "add-paused", "", false, "Add xseed torrents to client in paused state")
	command.Flags().BoolVarP(&check, "check", "", false, "Let client do hash checking when add xseed torrents")
	command.Flags().Int64VarP(&maxXseedTorrents, "max-torrents", "m", 0, "Number limit of xseed torrents added. Default (0) == unlimited")
	command.Flags().Int64VarP(&iyuuRequestMaxTorrents, "max-request-torrents", "", 2000, "Number limit of target torrents sent to iyuu server at once")
	command.Flags().StringVarP(&includeSites, "include-sites", "", "", "Only add xseed torrents from these sites or groups (comma-separated)")
	command.Flags().StringVarP(&excludeSites, "exclude-sites", "", "", "Do NOT add xseed torrents from these sites or groups (comma-separated)")
	command.Flags().StringVarP(&category, "category", "c", "", "Only xseed torrents that belongs to this category")
	command.Flags().StringVarP(&tag, "tag", "t", "", "Only xseed torrents that has this tag")
	command.Flags().StringVarP(&filter, "filter", "f", "", "Only xseed torrents which name contains this")
	command.Flags().StringVarP(&addCategory, "add-category", "", "", "Manually set category of added xseed torrent. By Default it uses the original torrent's")
	command.Flags().StringVarP(&addTags, "add-tags", "", "", "Set tags of added xseed torrent (comma-separated)")
	command.Flags().StringVarP(&minTorrentSizeStr, "min-torrent-size", "", "1GiB", "Torrents with size smaller than (<) this value will NOT be xseeded")
	command.Flags().StringVarP(&maxTorrentSizeStr, "max-torrent-size", "", "1PiB", "Torrents with size larger than (>) this value will NOT be xseeded")
	command.Flags().StringVarP(&iyuuRequestServer, "request-server", "", "auto", "Whether send request to iyuu server to update local xseed db. Possible values: auto|yes|no")
	iyuu.Command.AddCommand(command)
}

func xseed(cmd *cobra.Command, args []string) {
	log.Tracef("iyuu token: %s", config.Get().IyuuToken)
	if config.Get().IyuuToken == "" {
		log.Fatalf("You must config iyuuToken in ptool.toml to use iyuu functions")
	}

	includeSitesMode := false
	includeSitesFlag := map[string](bool){}
	excludeSitesFlag := map[string](bool){}
	if includeSites != "" && excludeSites != "" {
		log.Fatalf("--include-sites and --exclude-sites flags can NOT be both set")
	}
	if includeSites != "" {
		includeSitesMode = true
		sites := config.ParseGroupAndOtherNames(strings.Split(includeSites, ",")...)
		for _, site := range sites {
			includeSitesFlag[site] = true
		}
	} else if excludeSites != "" {
		sites := config.ParseGroupAndOtherNames(strings.Split(excludeSites, ",")...)
		for _, site := range sites {
			excludeSitesFlag[site] = true
		}
	}
	minTorrentSize, _ := utils.RAMInBytes(minTorrentSizeStr)
	maxTorrentSize, _ := utils.RAMInBytes(maxTorrentSizeStr)
	if iyuuRequestServer != "auto" && iyuuRequestServer != "yes" && iyuuRequestServer != "no" {
		log.Fatalf("Invalid --request-server flag value %s", iyuuRequestServer)
	}
	filter = strings.ToLower(filter)
	var fixedTags []string
	if addTags != "" {
		fixedTags = strings.Split(addTags, ",")
	}

	clientNames := args
	clientInstanceMap := map[string](client.Client){} // clientName => clientInstance
	clientInfoHashesMap := map[string]([]string){}
	reqInfoHashes := []string{}

	cntCandidateTargetTorrents := int64(0)
	cntTargetTorrents := int64(0)
	cntXseedTorrents := int64(0)
	cntSucccessXseedTorrents := int64(0)

	for _, clientName := range clientNames {
		clientInstance, err := client.CreateClient(clientName)
		if err != nil {
			log.Fatal(err)
		}
		clientInstanceMap[clientName] = clientInstance

		torrents, err := clientInstance.GetTorrents("", "", true)
		if err != nil {
			log.Errorf("client %s failed to get torrents: %v", clientName, err)
			continue
		} else {
			log.Tracef("client %s has %d torrents", clientName, len(torrents))
		}
		torrents = utils.Filter(torrents, func(torrent client.Torrent) bool {
			return torrent.IsFull() && torrent.Category != config.XSEED_TAG && !torrent.HasTag(config.XSEED_TAG)
		})
		sort.Slice(torrents, func(i, j int) bool {
			if torrents[i].Size != torrents[j].Size {
				return torrents[i].Size > torrents[j].Size
			}
			if torrents[i].Atime != torrents[j].Atime {
				return torrents[i].Atime < torrents[j].Atime
			}
			return torrents[i].TrackerDomain < torrents[j].TrackerDomain
		})
		infoHashes := []string{}
		tsize := int64(0)
		var sameSizeTorrentContentPathes []string
		for _, torrent := range torrents {
			// same size torrents may be identical (manually xseeded before)
			if torrent.Size != tsize {
				sameSizeTorrentContentPathes = []string{torrent.ContentPath}
				reqInfoHashes = append(reqInfoHashes, torrent.InfoHash)
				tsize = torrent.Size
			} else if slices.Index(sameSizeTorrentContentPathes, torrent.ContentPath) == -1 {
				sameSizeTorrentContentPathes = append(sameSizeTorrentContentPathes, torrent.ContentPath)
				reqInfoHashes = append(reqInfoHashes, torrent.InfoHash)
			}
			if category != "" {
				if torrent.Category != category {
					continue
				}
			} else if strings.HasPrefix(torrent.Category, "_") {
				continue
			}
			if tag != "" && !torrent.HasTag(tag) {
				continue
			}
			if torrent.State != "seeding" || !torrent.IsFullComplete() ||
				torrent.Size < minTorrentSize || torrent.Size > maxTorrentSize {
				continue
			}
			if filter != "" && !strings.Contains(torrent.Name, filter) {
				continue
			}
			infoHashes = append(infoHashes, torrent.InfoHash)
			cntCandidateTargetTorrents++
		}
		clientInfoHashesMap[clientName] = infoHashes
	}

	if cntCandidateTargetTorrents == 0 {
		fmt.Printf("No cadidate torrents to to xseed.")
		return
	}

	reqInfoHashes = utils.UniqueSlice(reqInfoHashes)
	if len(reqInfoHashes) > int(iyuuRequestMaxTorrents) {
		reqInfoHashes = reqInfoHashes[:iyuuRequestMaxTorrents]
	}
	doRequestServer := false
	if iyuuRequestServer == "auto" {
		var lastUpdateTime iyuu.Meta
		iyuu.Db().Where("key = ?", "lastUpdateTime").First(&lastUpdateTime)
		if lastUpdateTime.Value == "" || utils.Now()-utils.ParseInt(lastUpdateTime.Value) >= 7200 {
			doRequestServer = true
		} else {
			log.Tracef("Fetched iyuu xseed data recently. Do not fetch this time")
		}
	} else if iyuuRequestServer == "yes" {
		doRequestServer = true
	}
	if doRequestServer {
		updateIyuuDatabase(config.Get().IyuuToken, reqInfoHashes)
	}

	var sites []iyuu.Site
	var clientTorrents []*iyuu.Torrent
	var clientTorrentsMap = map[string]([]*iyuu.Torrent){} // targetInfoHash => iyuuTorrent
	iyuu.Db().Find(&sites)
	iyuu.Db().Where("target_info_hash in ?", reqInfoHashes).Find(&clientTorrents)
	site2LocalMap := iyuu.GenerateIyuu2LocalSiteMap(sites, config.Get().Sites)
	log.Tracef("iyuu->local site map: %v; clientTorrents: len=%d", site2LocalMap, len(clientTorrents))
	for _, torrent := range clientTorrents {
		list := clientTorrentsMap[torrent.TargetInfoHash]
		list = append(list, torrent)
		clientTorrentsMap[torrent.TargetInfoHash] = list
	}

	siteInstancesMap := map[string](site.Site){}
mainloop:
	for i, clientName := range clientNames {
		log.Printf("Start xseeding client (%d/%d) %s", i+1, len(clientName), clientName)
		clientInstance := clientInstanceMap[clientName]
		cnt := len(clientInfoHashesMap[clientName])
		for i, infoHash := range clientInfoHashesMap[clientName] {
			if slowMode {
				utils.Sleep(3)
			}
			xseedTorrents := clientTorrentsMap[infoHash]
			if len(xseedTorrents) == 0 {
				log.Debugf("torrent %s skipped or has no xseed candidates", infoHash)
				continue
			} else {
				log.Debugf("torrent %s has %d xseed candidates", infoHash, len(xseedTorrents))
			}
			targetTorrent, err := clientInstance.GetTorrent(infoHash)
			if err != nil {
				log.Errorf("Failed to get target torrent %s info from client: %v", infoHash, err)
				continue
			}
			cntTargetTorrents++
			log.Tracef("client torrent (%d/%d) %s: name=%s, savePath=%s",
				i+1, cnt,
				targetTorrent.InfoHash, targetTorrent.Name, targetTorrent.SavePath,
			)
			targetTorrentContentFiles, err := clientInstance.GetTorrentContents(infoHash)
			if err != nil {
				log.Tracef("Failed to get target torrent %s contents from client: %v", infoHash, err)
				continue
			}
			for _, xseedTorrent := range xseedTorrents {
				clientExistingTorrent, err := clientInstance.GetTorrent(xseedTorrent.InfoHash)
				if err != nil {
					log.Errorf("Failed to get client existing torrent info for %s", xseedTorrent.InfoHash)
					continue
				}
				sitename := site2LocalMap[xseedTorrent.Sid]
				if sitename == "" {
					log.Tracef("torrent %s xseed candidate torrent %s site sid %d not found in local",
						infoHash, xseedTorrent.InfoHash, xseedTorrent.Sid,
					)
					continue
				}
				if clientExistingTorrent != nil {
					log.Tracef("xseed candidate %s already existed in client", xseedTorrent.InfoHash)
					if !dryRun {
						tags := []string{}
						removeTags := []string{}
						if !clientExistingTorrent.HasTag(config.XSEED_TAG) {
							tags = append(tags, config.XSEED_TAG)
						}
						siteTag := client.GenerateTorrentTagFromSite(sitename)
						if !clientExistingTorrent.HasTag(siteTag) {
							tags = append(tags, siteTag)
						}
						oldSite := clientExistingTorrent.GetSiteFromTag()
						if oldSite != "" && oldSite != sitename {
							removeTags = append(removeTags, client.GenerateTorrentTagFromSite(oldSite))
						}
						if len(tags) > 0 || len(removeTags) > 0 {
							clientInstance.ModifyTorrent(clientExistingTorrent.InfoHash, &client.TorrentOption{
								Tags:       tags,
								RemoveTags: removeTags,
							}, nil)
						}
					}
					continue
				}
				if (includeSitesMode && !includeSitesFlag[sitename]) || (!includeSitesMode && excludeSitesFlag[sitename]) {
					log.Tracef("skip site %s torrent", sitename)
					continue
				}
				if siteInstancesMap[sitename] == nil {
					siteInstance, err := site.CreateSite(sitename)
					if err != nil {
						log.Fatalf("Failed to create iyuu sid %d (local %s) site instance: %v",
							xseedTorrent.Sid, sitename, err)
					}
					siteInstancesMap[sitename] = siteInstance
				}
				siteInstance := siteInstancesMap[sitename]
				log.Printf("Xseed torrent %s (target %s) from site %s (iyuu sid %d) / tid %d",
					xseedTorrent.InfoHash,
					targetTorrent.Name,
					siteInstance.GetName(),
					xseedTorrent.Sid,
					xseedTorrent.Tid,
				)
				if dryRun {
					continue
				}
				xseedTorrentContent, _, err := siteInstance.DownloadTorrentById(fmt.Sprint(xseedTorrent.Tid))
				if err != nil {
					log.Errorf("Failed to download torrent from site: %v", err)
					continue
				}
				xseedTorrentInfo, err := torrentutil.ParseTorrent(xseedTorrentContent, 99)
				if err != nil {
					log.Errorf("Failed to parse xseed torrent contents: %v", err)
					continue
				}
				compareResult := xseedTorrentInfo.XseedCheckWithClientTorrent(targetTorrentContentFiles)
				if compareResult < 0 {
					if compareResult == -2 {
						log.Tracef("xseed candidate is NOT identital with client torrent. (Only ROOT folders diff)")
					} else {
						log.Tracef("xseed candidate is NOT identital with client torrent.")
					}
					continue
				}
				cntXseedTorrents++
				xseedTorrentCategory := targetTorrent.Category
				if addCategory != "" {
					xseedTorrentCategory = addCategory
				}
				tags := []string{config.XSEED_TAG, client.GenerateTorrentTagFromSite(sitename)}
				tags = append(tags, fixedTags...)
				err = clientInstance.AddTorrent(xseedTorrentContent, &client.TorrentOption{
					SavePath:     targetTorrent.SavePath,
					Category:     xseedTorrentCategory,
					Tags:         tags,
					Pause:        addPaused,
					SkipChecking: !check,
				}, nil)
				log.Infof("Add xseed torrent %s result: error=%v", xseedTorrent.InfoHash, err)
				if err == nil {
					cntSucccessXseedTorrents++
				}
				if maxXseedTorrents > 0 && cntXseedTorrents >= maxXseedTorrents {
					break mainloop
				}
				utils.Sleep(2)
			}
		}
		clientInstance.Close()
	}
	fmt.Printf("Done xseed %d clients. Target / Xseed / SuccessXseed torrents: %d / %d / %d\n",
		len(clientNames), cntTargetTorrents, cntXseedTorrents, cntSucccessXseedTorrents)
}

func updateIyuuDatabase(token string, infoHashes []string) error {
	log.Tracef("Querying iyuu server for xseed info of %d torrents.",
		len(infoHashes),
	)

	// update sites
	iyuuSites, err := iyuu.IyuuApiSites(token)
	if err != nil {
		log.Errorf("failed to get iyuu sites: %v", err)
	} else {
		iyuu.Db().Where("1 = 1").Delete(&iyuu.Site{})
		iyuuSiteRecords := utils.Map(iyuuSites, func(iyuuSite iyuu.IyuuApiSite) iyuu.Site {
			return iyuu.Site{
				Sid:          iyuuSite.Id,
				Name:         iyuuSite.Site,
				Nickname:     iyuuSite.Nickname,
				Url:          iyuuSite.GetUrl(),
				DownloadPage: iyuuSite.Download_page,
			}
		})
		iyuu.Db().Create(&iyuuSiteRecords)
	}

	// update xseed torrents data
	data, err := iyuu.IyuuApiHash(token, infoHashes)
	if err != nil {
		log.Errorf("iyuu apiHash error: %v", err)
	} else {
		log.Debugf("iyuu data len(data)=%d\n", len(data))
		for targetInfoHash, iyuuRecords := range data {
			iyuu.Db().Where("target_info_hash = ?", targetInfoHash).Delete(&iyuu.Torrent{})
			infoHashes := utils.Map(iyuuRecords, func(record iyuu.IyuuTorrentInfoHash) string {
				return record.Info_hash
			})
			iyuu.Db().Where("info_hash in ?", infoHashes).Delete(&iyuu.Torrent{})
			iyuuTorrents := utils.Map(iyuuRecords, func(iyuuRecord iyuu.IyuuTorrentInfoHash) iyuu.Torrent {
				return iyuu.Torrent{
					InfoHash:       iyuuRecord.Info_hash,
					Sid:            iyuuRecord.Sid,
					Tid:            iyuuRecord.Torrent_id,
					TargetInfoHash: targetInfoHash,
				}
			})
			iyuu.Db().Create(&iyuuTorrents)
		}

		iyuu.Db().Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "key"}},
			DoUpdates: clause.AssignmentColumns([]string{"value"}),
		}).Create(&iyuu.Meta{
			Key:   "lastUpdateTime",
			Value: fmt.Sprint(utils.Now()),
		})
	}

	return nil
}
