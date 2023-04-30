package search

import (
	"sort"
	"strings"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/sagan/ptool/cmd"
	"github.com/sagan/ptool/site"
	"github.com/sagan/ptool/utils"
)

var command = &cobra.Command{
	Use:   "search <site> <keyword>",
	Short: "Search torrents by keyword in a site",
	Long:  `Search torrents by keyword in a site`,
	Args:  cobra.MatchAll(cobra.MinimumNArgs(2), cobra.OnlyValidArgs),
	Run:   search,
}

var (
	largestFlag = false
	baseUrl     = ""
)

func init() {
	command.Flags().BoolVarP(&largestFlag, "largest", "l", false, "Sort search result by torrent size in desc order")
	command.Flags().StringVar(&baseUrl, "base-url", "", "Manually set the base url of search page. eg. adult.php or https://kp.m-team.cc/adult.php for M-Team site")
	cmd.RootCmd.AddCommand(command)
}

func search(cmd *cobra.Command, args []string) {
	siteInstance, err := site.CreateSite(args[0])
	if err != nil {
		log.Fatal(err)
	}
	keyword := strings.Join(args[1:], " ")
	now := utils.Now()

	torrents, err := siteInstance.SearchTorrents(keyword, baseUrl)
	if err != nil {
		log.Fatal(err)
	}
	if largestFlag {
		sort.Slice(torrents, func(i, j int) bool {
			if torrents[i].Size != torrents[j].Size {
				return torrents[i].Size > torrents[j].Size
			}
			return torrents[i].Seeders > torrents[j].Seeders
		})
	}
	site.PrintTorrents(torrents, "", now, false, siteInstance.GetName())
}
