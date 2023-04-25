package xseed

import (
	"fmt"

	log "github.com/sirupsen/logrus"

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
	Run:   xseed,
}

func init() {
	iyuu.Command.AddCommand(command)
}

func xseed(cmd *cobra.Command, args []string) {
	log.Print(config.ConfigFile, " ", args)
	log.Print("token", config.Get().IyuuToken)

	clientInstance, err := client.CreateClient(args[0])
	if err != nil {
		log.Fatal(err)
	}

	torrents, err := clientInstance.GetTorrents("", "", true)
	if err != nil {
		log.Fatal(err)
	}
	torrents = utils.Filter(torrents, func(torrent client.Torrent) bool {
		return torrent.State == "seeding" && torrent.IsFullComplete() && !torrent.HasTag("_xseed")
	})
	if len(torrents) == 0 {
		fmt.Printf("No cadidate torrents to check for xseeding.")
		return
	}

	infoHashes := utils.Map(torrents, func(torrent client.Torrent) string {
		return torrent.InfoHash
	})
	log.Tracef("Querying iyuu server for xseed info of %d torrents in client %s.",
		len(infoHashes),
		clientInstance.GetName(),
	)
	data, err := iyuu.IyuuApiHash(config.Get().IyuuToken, infoHashes)
	if err != nil {
		log.Errorf("iyuu apiHash error: %v", err)
	}
	log.Debugf("len(data)=%d\n", len(data))
	for infoHash, iyuuRecords := range data {
		var dbRecord iyuu.Torrent
		iyuu.Db().First(&dbRecord, "info_hash = ? or same_info_hash = ?", infoHash, infoHash)
		for _, iyuuRecord := range iyuuRecords {
			iyuu.Db().Create(&iyuu.Torrent{
				InfoHash:     infoHash,
				SameInfoHash: iyuuRecord.Info_hash,
				Sid:          iyuuRecord.Sid,
			})
		}
	}
}
