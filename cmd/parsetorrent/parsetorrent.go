package parsetorrent

import (
	"fmt"
	"log"
	"os"
	"strings"

	goTorrentParser "github.com/j-muller/go-torrent-parser"
	"github.com/sagan/ptool/cmd"
	"github.com/sagan/ptool/utils"
	"github.com/spf13/cobra"
)

var command = &cobra.Command{
	Use:   "parsetorrent file.torrent",
	Short: "Parse torrent file and show it's content",
	Long:  `A longer description`,
	Args:  cobra.MatchAll(cobra.MinimumNArgs(1), cobra.OnlyValidArgs),
	Run:   parsetorrent,
}

var (
	showAll = false
)

func init() {
	command.Flags().BoolVar(&showAll, "all", false, "show all info.")
	cmd.RootCmd.AddCommand(command)
}

func parsetorrent(cmd *cobra.Command, args []string) {
	torrentFileNames := args
	hasError := false
	for _, torrentFileName := range torrentFileNames {
		torrentInfo, err := goTorrentParser.ParseFromFile(torrentFileName)
		if err != nil {
			log.Printf("Failed to parse %s: %v", torrentFileName, err)
			continue
		}
		trackerHostname := ""
		if len(torrentInfo.Announce) > 0 {
			trackerHostname = utils.ParseUrlHostname(torrentInfo.Announce[0])
		}
		fmt.Printf("Torrent %s: infohash = %s  tracker = %s //%s\n", torrentFileName, torrentInfo.InfoHash, trackerHostname, torrentInfo.Comment)

		if showAll {
			fmt.Printf("\n")
			for i, file := range torrentInfo.Files {
				fmt.Printf("%d. %s\n", i, strings.Join(file.Path, "/"))
			}
			fmt.Printf("\n")
		}
	}
	if hasError {
		os.Exit(1)
	}
}
