package ebookgod

// 电子书战神。批量下载最小的种子保种

import (
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/sagan/ptool/cmd"
	"github.com/sagan/ptool/site"
	"github.com/sagan/ptool/utils"
)

var command = &cobra.Command{
	Use:   "ebookgod <site>",
	Short: "Batch download the smallest torrents of the site",
	Long:  `Batch download the smallest torrents of the site`,
	Args:  cobra.MatchAll(cobra.ExactArgs(1), cobra.OnlyValidArgs),
	Run:   ebookgod,
}

func init() {
	cmd.RootCmd.AddCommand(command)
}

func ebookgod(cmd *cobra.Command, args []string) {
	siteInstance, err := site.CreateSite(args[0])
	if err != nil {
		log.Fatal(err)
	}

	var torrents []site.Torrent
	var marker string

	for true {
		now := utils.Now()
		torrents, marker, err = siteInstance.GetAllTorrents("size", false, marker)
		site.PrintTorrents(torrents, "", now)
		if err != nil {
			log.Errorf("Failed to fetch next page torrents: %v", err)
			break
		}
		if marker == "" {
			break
		}
		utils.Sleep(3)
	}
}
