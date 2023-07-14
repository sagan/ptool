package verifytorrent

import (
	"fmt"
	"os"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/sagan/ptool/cmd"
	"github.com/sagan/ptool/torrentutil"
)

var command = &cobra.Command{
	Use:   "verifytorrent <file.torrent>",
	Short: "Verify a *.torrent file is consistent with local disk files.",
	Long: `Verify a *.torrent file is consistent with local disk files.

Usage:
ptool verifytorrent file.torrent --save-path /root/Downloads
ptool verifytorrent file.torrent --content-path /root/Downloads/TorrentContentFolder

Exact one of the --save-path or --content-path (but not both) flag must be specified.
* --save-path : the parent folder of torrent content(s)
* --content-path : the torrent content(s) path, could be a folder or a single file

By default it will only examine file meta infos (file path & file size).
If --check flag is set, it will also do the hash checking.`,
	Args: cobra.MatchAll(cobra.ExactArgs(1), cobra.OnlyValidArgs),
	Run:  verifytorrent,
}

var (
	savePath    = ""
	contentPath = ""
	checkHash   = false
	showAll     = false
)

func init() {
	command.Flags().BoolVarP(&checkHash, "check", "", false, "Do hash checking when verifying torrent files")
	command.Flags().BoolVarP(&showAll, "all", "a", false, "show all info.")
	command.Flags().StringVarP(&savePath, "save-path", "", "", "The parent folder path of torrent content(s). (One of this and --content-path flag MUST be set)")
	command.Flags().StringVarP(&contentPath, "content-path", "", "", "The path of torrent content(s). (One of this and --save-path flag MUST be set)")
	cmd.RootCmd.AddCommand(command)
}

func verifytorrent(cmd *cobra.Command, args []string) {
	if savePath == "" && contentPath == "" || (savePath != "" && contentPath != "") {
		log.Fatalf("Exact one of the --save-path or --content-path (but not both) flag must be specified.")
	}
	torrentFilename := args[0]

	torrentData, err := os.ReadFile(torrentFilename)
	if err != nil {
		log.Fatal(err)
	}
	torrentMeta, err := torrentutil.ParseTorrent(torrentData, 99)
	if err != nil {
		log.Fatal(err)
	}
	torrentMeta.Print(torrentFilename, true)
	fmt.Printf("\n")
	err = torrentMeta.Verify(savePath, contentPath, checkHash)
	if err != nil {
		fmt.Printf("X torrents contents do NOT match with disk content(s) (did hash check = %t): %v", checkHash, err)
		os.Exit(1)
	} else {
		fmt.Printf("✓ torrents contents match with disk content(s) (did hash check = %t)", checkHash)
		os.Exit(0)
	}
}
