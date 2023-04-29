package search

import (
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

func init() {
	cmd.RootCmd.AddCommand(command)
}

func search(cmd *cobra.Command, args []string) {
	siteInstance, err := site.CreateSite(args[0])
	if err != nil {
		log.Fatal(err)
	}
	keyword := strings.Join(args[1:], " ")
	now := utils.Now()

	torrents, err := siteInstance.SearchTorrents(keyword)
	if err != nil {
		log.Fatal(err)
	}
	site.PrintTorrents(torrents, "", now, false)
}
