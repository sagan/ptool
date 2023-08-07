package xseedcheck

import (
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"

	"github.com/sagan/ptool/client"
	"github.com/sagan/ptool/cmd"
	"github.com/sagan/ptool/utils/torrentutil"
)

var command = &cobra.Command{
	Use:         "xseedcheck {client} {infoHash} {file.torrent}",
	Annotations: map[string](string){"cobra-prompt-dynamic-suggestions": "xseedcheck"},
	Short:       "Check whether a torrent in client is identical with a torrent file.",
	Long: `Check whether a torrent in client is identical with a torrent file.
Only filename and size will be comared. Not the file contents themselves.`,
	Args: cobra.MatchAll(cobra.ExactArgs(3), cobra.OnlyValidArgs),
	RunE: xseedcheck,
}

var (
	showAll = false
)

func init() {
	command.Flags().BoolVarP(&showAll, "all", "a", false, "show full comparison result")
	cmd.RootCmd.AddCommand(command)
}

func xseedcheck(cmd *cobra.Command, args []string) error {
	clientName := args[0]
	infoHash := args[1]
	torrentFilename := args[2]
	clientInstance, err := client.CreateClient(clientName)
	if err != nil {
		return fmt.Errorf("failed to create client: %v", err)
	}
	var torrentContent []byte
	if torrentFilename == "-" {
		torrentContent, err = io.ReadAll(os.Stdin)
	} else {
		torrentContent, err = os.ReadFile(torrentFilename)
	}
	if err != nil {
		return fmt.Errorf("failed to read %s: %v", torrentFilename, err)
	}
	torrentInfo, err := torrentutil.ParseTorrent(torrentContent, 99)
	if err != nil {
		return fmt.Errorf("failed to parse %s: %v", torrentFilename, err)
	}
	if torrentInfo.InfoHash == infoHash {
		fmt.Printf(
			"Result: identical. Torrent file %s has the same infoHash with client %s torrent.\n",
			torrentFilename,
			clientName,
		)
		return nil
	}
	clientTorrentContents, err := clientInstance.GetTorrentContents(infoHash)
	if err != nil {
		return fmt.Errorf("failed to get client torrent contents info: %v", err)
	}
	compareResult := torrentInfo.XseedCheckWithClientTorrent(clientTorrentContents)
	if compareResult == 0 {
		fmt.Printf(
			"Result: ✓. Torrent file %s has the same contents with client %s torrent.\n",
			torrentFilename,
			clientName,
		)
	} else if compareResult == 1 {
		fmt.Printf(
			"Result: ✓*. Torrent file %s has the same (partial) contents with client %s torrent.\n",
			torrentFilename,
			clientName,
		)
	} else if compareResult == -2 {
		fmt.Printf(
			"Result: X*. Torrent file %s has the DIFFERENT root folder, but same contents with client %s torrent.\n",
			torrentFilename,
			clientName,
		)
	} else {
		fmt.Printf(
			"Result: X. Torrent file %s does NOT has the same contents with client %s torrent.\n",
			torrentFilename,
			clientName,
		)
	}
	if showAll {
		fmt.Printf("\n")
		fmt.Printf("Client: %s torrent\n", infoHash)
		for i, clientTorrentFile := range clientTorrentContents {
			fmt.Printf("%-5d  %-15d  %s\n", i+1, clientTorrentFile.Size, clientTorrentFile.Path)
		}

		fmt.Printf("\n")
		fmt.Printf("Torrent file: %s\n", torrentFilename)
		torrentInfo.PrintFiles(true, true)
	}
	return nil
}
