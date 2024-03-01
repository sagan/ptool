package xseedcheck

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/sagan/ptool/client"
	"github.com/sagan/ptool/cmd"
	"github.com/sagan/ptool/util/helper"
)

var command = &cobra.Command{
	Use:         "xseedcheck {client} {infoHash} {torrentFilename | torrentId | torrentUrl}",
	Annotations: map[string]string{"cobra-prompt-dynamic-suggestions": "xseedcheck"},
	Short:       "Check whether a torrent in client is identical with a torrent file.",
	Long: `Check whether a torrent in client is identical with a torrent file.
{torrentFilename | torrentId | torrentUrl}: could be a local filename (e.g. "*.torrent" or "[M-TEAM]CLANNAD.torrent"),
site torrent id (e.g.: "mteam.488424") or url (e.g.: "https://kp.m-team.cc/details.php?id=488424").
Torrent url that does NOT belong to any site (e.g.: a public site url) is also supported.
Use a single "-" to read .torrent file contents from stdin.

Only filename and size will be compared. Not the disk file contents themselves.`,
	Args: cobra.MatchAll(cobra.ExactArgs(3), cobra.OnlyValidArgs),
	RunE: xseedcheck,
}

var (
	showAll     = false
	forceLocal  = false
	defaultSite = ""
)

func init() {
	command.Flags().BoolVarP(&showAll, "all", "a", false, "show full comparison result")
	command.Flags().BoolVarP(&forceLocal, "force-local", "", false, "Force treat the arg as local torrent filename")
	command.Flags().StringVarP(&defaultSite, "site", "", "", "Set default site of torrent url")
	cmd.RootCmd.AddCommand(command)
}

func xseedcheck(cmd *cobra.Command, args []string) error {
	clientName := args[0]
	infoHash := args[1]
	torrent := args[2]
	clientInstance, err := client.CreateClient(clientName)
	if err != nil {
		return fmt.Errorf("failed to create client: %v", err)
	}
	_, tinfo, _, _, _, _, err := helper.GetTorrentContent(torrent, defaultSite, forceLocal, false, nil)
	if err != nil {
		return fmt.Errorf("failed to get %s: %v", torrent, err)
	}
	if tinfo.InfoHash == infoHash {
		fmt.Printf("Result: identical. Torrent %s has the same infoHash with client %s torrent.\n", torrent, clientName)
		return nil
	}
	clientTorrentContents, err := clientInstance.GetTorrentContents(infoHash)
	if err != nil {
		return fmt.Errorf("failed to get client torrent contents info: %v", err)
	}
	compareResult := tinfo.XseedCheckWithClientTorrent(clientTorrentContents)
	if compareResult == 0 {
		fmt.Printf("Result: ✓. Torrent %s has the same contents with client %s torrent.\n", torrent, clientName)
	} else if compareResult == 1 {
		fmt.Printf("Result: ✓*. Torrent %s has the same (partial) contents with client %s torrent.\n", torrent, clientName)
	} else if compareResult == -2 {
		fmt.Printf("Result: X*. Torrent %s has the DIFFERENT root folder, but same contents with client %s torrent.\n",
			torrent, clientName)
	} else {
		fmt.Printf("Result: X. Torrent %s does NOT has the same contents with client %s torrent.\n", torrent, clientName)
	}
	if showAll {
		fmt.Printf("\n")
		fmt.Printf("Client: %s torrent\n", infoHash)
		for i, clientTorrentFile := range clientTorrentContents {
			fmt.Printf("%-5d  %-15d  %s\n", i+1, clientTorrentFile.Size, clientTorrentFile.Path)
		}

		fmt.Printf("\n")
		fmt.Printf("Torrent file: %s\n", torrent)
		tinfo.PrintFiles(true, true)
	}
	return nil
}
