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
	Use:   "xseed client",
	Short: "Cross seed",
	Long:  `Cross seed`,
	Args:  cobra.MatchAll(cobra.MinimumNArgs(1), cobra.OnlyValidArgs),
	Run:   xseed,
}

var (
	dryRun = false
)

func init() {
	command.Flags().BoolVarP(&dryRun, "dry-run", "d", false, "Dry run. Do not actually controlling client")
	iyuu.Command.AddCommand(command)
}

func xseed(cmd *cobra.Command, args []string) {
	log.Print(config.ConfigFile, " ", args)
	log.Print("token", config.Get().IyuuToken)

	clientNames := args
	clientInstanceMap := map[string](client.Client){} // clientName => clientInstance
	clientInfoHashesMap := map[string]([]string){}
	reqInfoHashes := []string{}

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
			return torrent.State == "seeding" && torrent.IsFullComplete() &&
				!torrent.HasTag("_xseed") && !strings.HasPrefix(torrent.Category, "_")
		})
		sort.Slice(torrents, func(i, j int) bool {
			return torrents[i].Size < torrents[j].Size
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
	for _, clientName := range clientNames {
		log.Tracef("Start xseeding client %s", clientName)
		clientInstance := clientInstanceMap[clientName]
		for _, infoHash := range clientInfoHashesMap[clientName] {
			targetTorrent, err := clientInstance.GetTorrent(infoHash)
			if err != nil {
				log.Tracef("Failed to get target torrent %s info from client: %v", infoHash, err)
				continue
			}
			log.Tracef("client torrent %s _ %s: name=%s, savePath=%s",
				infoHash,
				targetTorrent.InfoHash, targetTorrent.Name, targetTorrent.SavePath,
			)
			targetTorrentContentFiles, err := clientInstance.GetTorrentContents(infoHash)
			if err != nil {
				log.Tracef("Failed to get target torrent %s contents from client: %v", infoHash, err)
				continue
			}
			xseedTorrents := clientTorrentsMap[infoHash]
			if len(xseedTorrents) == 0 {
				log.Tracef("torrent %s skipped or has no xseed candidates", infoHash)
				continue
			} else {
				log.Tracef("torrent %s has %d xseed candidates", infoHash, len(xseedTorrents))
			}
			for _, xseedTorrent := range xseedTorrents {
				clientExistingTorrent, err := clientInstance.GetTorrent(xseedTorrent.InfoHash)
				if err != nil {
					log.Tracef("Failed to get client existing torrent info for %s", xseedTorrent.InfoHash)
					continue
				}
				if clientExistingTorrent != nil {
					log.Tracef("xseed candidate %s already existed in client")
					if !clientExistingTorrent.HasTag("xseed") {
						clientInstance.ModifyTorrent(clientExistingTorrent.InfoHash, &client.TorrentOption{
							Tags: []string{"xseed"},
						}, nil)
					}
					continue
				}
				if site2LocalMap[xseedTorrent.Sid] == "" {
					log.Tracef("torrent %s xseed candidate torrent %s site sid %d not found in local",
						infoHash, xseedTorrent.InfoHash, xseedTorrent.Sid,
					)
					continue
				}
				if siteInstancesMap[site2LocalMap[xseedTorrent.Sid]] == nil {
					siteInstance, err := site.CreateSite(site2LocalMap[xseedTorrent.Sid])
					if err != nil {
						log.Fatalf("Failed to create iyuu sid %d (local %s) site instance: %v",
							xseedTorrent.Sid,
							site2LocalMap[xseedTorrent.Sid],
							err,
						)
					}
					siteInstancesMap[site2LocalMap[xseedTorrent.Sid]] = siteInstance
				}
				siteInstance := siteInstancesMap[site2LocalMap[xseedTorrent.Sid]]
				log.Tracef("Xseed torrent %s from site %s (iyuu sid %d) / tid %d",
					xseedTorrent.InfoHash,
					siteInstance.GetName(),
					xseedTorrent.Sid,
					xseedTorrent.Tid,
				)
				xseedTorrentContent, err := siteInstance.DownloadTorrentById(fmt.Sprint(xseedTorrent.Tid))
				if err != nil {
					log.Tracef("Failed to download torrent from site: %v", err)
					continue
				}
				xseedTorrentInfo, err := goTorrentParser.Parse(bytes.NewReader(xseedTorrentContent))
				if err != nil {
					log.Tracef("Failed to parse xseed torrent contents")
					continue
				}
				compareResult := client.XseedCheckTorrentContents(targetTorrentContentFiles, xseedTorrentInfo.Files)
				if compareResult < 0 {
					log.Tracef("xseed candidate is NOT identital with client torrent.")
					continue
				}
				err = clientInstance.AddTorrent(xseedTorrentContent, &client.TorrentOption{
					SavePath:     targetTorrent.SavePath,
					Tags:         []string{"xseed"},
					SkipChecking: true,
				}, nil)
				log.Infof("Add xseed torrent %s result: error=%v", xseedTorrent.InfoHash, err)
				utils.Sleep(2)
			}
		}
	}
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
		log.Debugf("data len(data)=%d\n", len(data))
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
