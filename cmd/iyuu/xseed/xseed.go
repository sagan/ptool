package xseed

import (
	"fmt"

	log "github.com/sirupsen/logrus"
	"gorm.io/gorm/clause"

	"github.com/sagan/ptool/client"
	"github.com/sagan/ptool/cmd/iyuu"
	"github.com/sagan/ptool/config"
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

func init() {
	iyuu.Command.AddCommand(command)
}

func xseed(cmd *cobra.Command, args []string) {
	log.Print(config.ConfigFile, " ", args)
	log.Print("token", config.Get().IyuuToken)

	clientNames := args
	clientInstanceMap := map[string](client.Client){}
	clientInfoHashesMap := map[string]([]string){}
	allTorrentsInfoHashes := []string{}

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
			return torrent.State == "seeding" && torrent.IsFullComplete() && !torrent.HasTag("_xseed")
		})
		infoHashes := utils.Map(torrents,
			func(torrent client.Torrent) string {
				return torrent.InfoHash
			})
		clientInfoHashesMap[clientName] = infoHashes
		allTorrentsInfoHashes = append(allTorrentsInfoHashes, infoHashes...)
	}

	allTorrentsInfoHashes = utils.UniqueSlice(allTorrentsInfoHashes)
	if len(allTorrentsInfoHashes) == 0 {
		fmt.Printf("No cadidate torrents to to xseed.")
		return
	}

	var lastUpdateTime iyuu.Meta
	iyuu.Db().Where("key = ?", "lastUpdateTime").First(&lastUpdateTime)
	if lastUpdateTime.Value == "" || utils.Now()-utils.ParseInt(lastUpdateTime.Value) >= 3600 {
		updateIyuuDatabase(config.Config.IyuuToken, allTorrentsInfoHashes)
	}

	var sites []iyuu.Site
	var clientTorrents []iyuu.Torrent
	var clientTorrentsMap = make(map[string]([]iyuu.Torrent))
	iyuu.Db().Find(&sites)
	iyuu.Db().Where("target_info_hash in ?", allTorrentsInfoHashes).Find(&clientTorrents)
	site2LocalMap := iyuu.GenerateIyuu2LocalSiteMap(sites, config.Get().Sites)
	for _, torrent := range clientTorrents {
		list := clientTorrentsMap[torrent.TargetInfoHash]
		list = append(list, torrent)
		clientTorrentsMap[torrent.TargetInfoHash] = list
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
			for _, iyuuRecord := range iyuuRecords {
				iyuu.Db().Where("info_hash in ?", infoHashes).Delete(&iyuu.Torrent{})
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
