package xseedcheck

import (
	"fmt"
	"sort"
	"strings"

	goTorrentParser "github.com/j-muller/go-torrent-parser"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/sagan/ptool/client"
	"github.com/sagan/ptool/cmd"
)

var command = &cobra.Command{
	Use:   "xseedcheck <client> <infoHash> file.torrent",
	Short: "Check whether a torrent in client is identical with a torrent file",
	Long: `Check whether a torrent in client is identical with a torrent file
Only filename and size will be comared. Not the file contents themselves.`,
	Args: cobra.MatchAll(cobra.ExactArgs(3), cobra.OnlyValidArgs),
	Run:  xseedcheck,
}

var (
	showFull = false
)

func init() {
	command.Flags().BoolVarP(&showFull, "full", "f", false, "show full comparison result")
	cmd.RootCmd.AddCommand(command)
}

func xseedcheck(cmd *cobra.Command, args []string) {
	clientName := args[0]
	infoHash := args[1]
	torrentFileName := args[2]

	clientInstance, err := client.CreateClient(clientName)
	if err != nil {
		log.Fatalf("Failed to create client: %v", err)
	}
	torrentInfo, err := goTorrentParser.ParseFromFile(torrentFileName)
	if err != nil {
		log.Fatalf("Failed to parse %s: %v", torrentFileName, err)
	}
	if torrentInfo.InfoHash == infoHash {
		fmt.Printf(
			"Result: identical. Torrent file %s has the same infoHash with client %s torrent.\n",
			torrentFileName,
			clientName,
		)
		return
	}
	sort.Slice(torrentInfo.Files, func(i, j int) bool {
		return strings.Join(torrentInfo.Files[i].Path, "/") < strings.Join(torrentInfo.Files[j].Path, "/")
	})
	clientTorrentContents, err := clientInstance.GetTorrentContents(infoHash)
	if err != nil {
		log.Fatalf("Failed to get client torrent contents info: %v", err)
	}
	sort.Slice(clientTorrentContents, func(i, j int) bool {
		return clientTorrentContents[i].Path < clientTorrentContents[j].Path
	})
	compareResult := client.XseedCheckTorrentContents(clientTorrentContents, torrentInfo.Files)
	if compareResult == 0 {
		fmt.Printf(
			"Result: ✓. Torrent file %s has the same contents with client %s torrent.\n",
			torrentFileName,
			clientName,
		)
	} else if compareResult == 1 {
		fmt.Printf(
			"Result: ✓*. Torrent file %s has the same (partial) contents with client %s torrent.\n",
			torrentFileName,
			clientName,
		)
	} else if compareResult == -2 {
		fmt.Printf(
			"Result: ✗*. Torrent file %s has the DIFFERENT root folder, but same contents with client %s torrent.\n",
			torrentFileName,
			clientName,
		)
	} else {
		fmt.Printf(
			"Result: ✗. Torrent file %s does NOT has the same contents with client %s torrent.\n",
			torrentFileName,
			clientName,
		)
	}

	if showFull {
		fmt.Printf("\n")
		fmt.Printf("Client: %s torrent\n", infoHash)
		for _, clientTorrentFile := range clientTorrentContents {
			fmt.Printf("%-15d %s\n", clientTorrentFile.Size, clientTorrentFile.Path)
		}

		fmt.Printf("\n")
		fmt.Printf("File: %s\n", torrentFileName)
		for _, tfile := range torrentInfo.Files {
			fmt.Printf("%-15d %s\n", tfile.Length, strings.Join(tfile.Path, "/"))
		}
	}
}
