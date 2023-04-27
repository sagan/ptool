package xseed

import (
	"bytes"
	"fmt"
	"sort"
	"strings"

	goTorrentParser "github.com/j-muller/go-torrent-parser"
	log "github.com/sirupsen/logrus"
	"gorm.io/gorm/clause"

	"github.com/sagan/ptool/client"
	"github.com/sagan/ptool/cmd/iyuu"
	"github.com/sagan/ptool/config"
	"github.com/sagan/ptool/site"
	"github.com/sagan/ptool/utils"
	"github.com/spf13/cobra"
)

var command = &cobra.Command{
	Use:   "xseed <client>...",
	Short: "Cross seed.",
	Long:  `Cross seed. By default it will add xseed torrents from All sites unless --include-sites or --exclude-sites flag is set`,
	Args:  cobra.MatchAll(cobra.MinimumNArgs(1), cobra.OnlyValidArgs),
	Run:   xseed,
}

var (
	includeSites      = ""
	excludeSites      = ""
	category          = ""
	setCategory       = ""
	tag               = ""
	dryRun            = false
	paused            = false
	check             = false
	slowMode          = false
	minTorrentSizeStr = ""
	maxXseedTorrents  = int64(0)
)

func init() {
	command.Flags().BoolVar(&slowMode, "slow", false, "Slow mode. wait after handling a xseed torrent. For dev / test purpose.")
	command.Flags().BoolVarP(&dryRun, "dry-run", "d", false, "Dry run. Do not actually controlling client")
	command.Flags().BoolVarP(&paused, "paused", "p", false, "Add xseed torrents to client in paused state")
	command.Flags().BoolVar(&check, "check-hash", false, "Let client do hash checking when add xseed torrents")
	command.Flags().StringVar(&includeSites, "include-sites", "", "Only add xseed torrents from these sites (comma-separated)")
	command.Flags().StringVar(&excludeSites, "exclude-sites", "", "Do NOT add xseed torrents from these sites (comma-separated)")
	command.Flags().StringVar(&category, "category", "", "Only xseed torrents that belongs to this category")
	command.Flags().StringVar(&tag, "tag", "", "Only xseed torrents that has this tag")
	command.Flags().StringVar(&setCategory, "set-category", "", "Manually set category of added xseed torrent. By Default it uses the original torrent's")
	command.Flags().StringVar(&minTorrentSizeStr, "min-torrent-size", "1GB", "Torrents with size less than this value will NOT be xseeded.")
	command.Flags().Int64Var(&maxXseedTorrents, "max-xseed-torrents", 0, "Number limit of xseed torrents added. Default = unlimited")
	iyuu.Command.AddCommand(command)
}

func xseed(cmd *cobra.Command, args []string) {
	log.Tracef("iyuu token: %s", config.Get().IyuuToken)
	if config.Get().IyuuToken == "" {
		log.Fatalf("You must config iyuuToken in ptool.yaml to use iyuu functions")
	}

	includeSitesMode := false
	includeSitesFlag := map[string](bool){}
	excludeSitesFlag := map[string](bool){}
	if includeSites != "" && excludeSites != "" {
		log.Fatalf("--include-sites and --exclude-sites flags can NOT be both set")
	}
	if includeSites != "" {
		includeSitesMode = true
		for _, site := range strings.Split(includeSites, ",") {
			includeSitesFlag[site] = true
		}
	} else if excludeSites != "" {
		for _, site := range strings.Split(excludeSites, ",") {
			excludeSitesFlag[site] = true
		}
	}
	minTorrentSize, _ := utils.RAMInBytes(minTorrentSizeStr)

	clientNames := args
	clientInstanceMap := map[string](client.Client){} // clientName => clientInstance
	clientInfoHashesMap := map[string]([]string){}
	reqInfoHashes := []string{}

	cntTargetTorrents := int64(0)
	cntXseedTorrents := int64(0)
	cntSucccessXseedTorrents := int64(0)

	for _, clientName := range clientNames {
		clientInstance, err := client.CreateClient(args[0])
		if err != nil {
			log.Fatal(err)
		}
		clientInstanceMap[clientName] = clientInstance

		torrents, err := clientInstance.GetTorrents("", "", true)
		if err != nil {
			log.Errorf("client %s failed to get torrents: %v", clientName, err)
			continue
		}
		torrents = utils.Filter(torrents, func(torrent client.Torrent) bool {
			if category != "" {
				if torrent.Category != category {
					return false
				}
			} else if strings.HasPrefix(torrent.Category, "_") {
				return false
			}
			if tag != "" && !torrent.HasTag(tag) {
				return false
			}
			return torrent.State == "seeding" && torrent.IsFullComplete() &&
				!torrent.HasTag("_xseed") && torrent.Size >= minTorrentSize
		})
		sort.Slice(torrents, func(i, j int) bool {
			if torrents[i].Size != torrents[j].Size {
				return torrents[i].Size < torrents[j].Size
			}
			if torrents[i].Atime != torrents[j].Atime {
				return torrents[i].Atime < torrents[j].Atime
			}
			return torrents[i].TrackerDomain < torrents[j].TrackerDomain
		})
		infoHashes := []string{}
		tsize := int64(0)
		for _, torrent := range torrents {
			infoHashes = append(infoHashes, torrent.InfoHash)
			// same size torrents may be identical (manually xseeded before)
			if torrent.Size != tsize {
				reqInfoHashes = append(reqInfoHashes, torrent.InfoHash)
				tsize = torrent.Size
			}
		}
		clientInfoHashesMap[clientName] = infoHashes
	}

	reqInfoHashes = utils.UniqueSlice(reqInfoHashes)
	if len(reqInfoHashes) == 0 {
		fmt.Printf("No cadidate torrents to to xseed.")
		return
	}

	var lastUpdateTime iyuu.Meta
	iyuu.Db().Where("key = ?", "lastUpdateTime").First(&lastUpdateTime)
	if lastUpdateTime.Value == "" || utils.Now()-utils.ParseInt(lastUpdateTime.Value) >= 3600 {
		updateIyuuDatabase(config.Config.IyuuToken, reqInfoHashes)
	} else {
		log.Tracef("Fetched iyuu xseed data recently. Do not fetch this time")
	}

	var sites []iyuu.Site
	var clientTorrents []*iyuu.Torrent
	var clientTorrentsMap = make(map[string]([]*iyuu.Torrent)) // targetInfoHash => iyuuTorrent
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
	for _, clientName := range clientNames {
		log.Printf("Start xseeding client %s", clientName)
		clientInstance := clientInstanceMap[clientName]
		for _, infoHash := range clientInfoHashesMap[clientName] {
			if slowMode {
				utils.Sleep(3)
			}
			targetTorrent, err := clientInstance.GetTorrent(infoHash)
			if err != nil {
				log.Errorf("Failed to get target torrent %s info from client: %v", infoHash, err)
				continue
			}
			cntTargetTorrents++
			log.Tracef("client torrent %s: name=%s, savePath=%s",
				targetTorrent.InfoHash, targetTorrent.Name, targetTorrent.SavePath,
			)
			targetTorrentContentFiles, err := clientInstance.GetTorrentContents(infoHash)
			if err != nil {
				log.Tracef("Failed to get target torrent %s contents from client: %v", infoHash, err)
				continue
			}
			xseedTorrents := clientTorrentsMap[infoHash]
			if len(xseedTorrents) == 0 {
				log.Debugf("torrent %s skipped or has no xseed candidates", infoHash)
				continue
			} else {
				log.Debugf("torrent %s has %d xseed candidates", infoHash, len(xseedTorrents))
			}
			for _, xseedTorrent := range xseedTorrents {
				clientExistingTorrent, err := clientInstance.GetTorrent(xseedTorrent.InfoHash)
				if err != nil {
					log.Errorf("Failed to get client existing torrent info for %s", xseedTorrent.InfoHash)
					continue
				}
				if clientExistingTorrent != nil {
					log.Tracef("xseed candidate %s already existed in client", xseedTorrent.InfoHash)
					if !dryRun && !clientExistingTorrent.HasTag("xseed") {
						clientInstance.ModifyTorrent(clientExistingTorrent.InfoHash, &client.TorrentOption{
							Tags: []string{"xseed"},
						}, nil)
					}
					continue
				}
				sitename := site2LocalMap[xseedTorrent.Sid]
				if sitename == "" {
					log.Tracef("torrent %s xseed candidate torrent %s site sid %d not found in local",
						infoHash, xseedTorrent.InfoHash, xseedTorrent.Sid,
					)
					continue
				}
				if (includeSitesMode && !includeSitesFlag[sitename]) || (!includeSitesMode && excludeSitesFlag[sitename]) {
					log.Tracef("skip site %d torrent", sitename)
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
				xseedTorrentContent, err := siteInstance.DownloadTorrentById(fmt.Sprint(xseedTorrent.Tid))
				if err != nil {
					log.Errorf("Failed to download torrent from site: %v", err)
					continue
				}
				xseedTorrentInfo, err := goTorrentParser.Parse(bytes.NewReader(xseedTorrentContent))
				if err != nil {
					log.Errorf("Failed to parse xseed torrent contents: %v", err)
					continue
				}
				compareResult := client.XseedCheckTorrentContents(targetTorrentContentFiles, xseedTorrentInfo.Files)
				if compareResult < 0 {
					log.Tracef("xseed candidate is NOT identital with client torrent.")
					continue
				}
				cntXseedTorrents++
				xseedTorrentCategory := targetTorrent.Category
				if setCategory != "" {
					xseedTorrentCategory = setCategory
				}
				err = clientInstance.AddTorrent(xseedTorrentContent, &client.TorrentOption{
					SavePath:     targetTorrent.SavePath,
					Category:     xseedTorrentCategory,
					Tags:         []string{"xseed", "site:" + siteInstance.GetName()},
					Pause:        paused,
					SkipChecking: !check,
				}, nil)
				log.Infof("Add xseed torrent %s result: error=%v", xseedTorrent.InfoHash, err)
				if err == nil {
					cntSucccessXseedTorrents++
				}
				if cntXseedTorrents == maxXseedTorrents {
					break mainloop
				}
				utils.Sleep(2)
			}
		}
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
		for _, iyuuSite := range iyuuSites {
			iyuu.Db().Create(&iyuu.Site{
				Sid:          iyuuSite.Id,
				Name:         iyuuSite.Site,
				Nickname:     iyuuSite.Nickname,
				Url:          iyuuSite.GetUrl(),
				DownloadPage: iyuuSite.Download_page,
			})
		}
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
			for _, iyuuRecord := range iyuuRecords {
				iyuu.Db().Create(&iyuu.Torrent{
					InfoHash:       iyuuRecord.Info_hash,
					Sid:            iyuuRecord.Sid,
					Tid:            iyuuRecord.Torrent_id,
					TargetInfoHash: targetInfoHash,
				})
			}
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
