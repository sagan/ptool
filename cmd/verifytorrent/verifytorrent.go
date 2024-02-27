package verifytorrent

import (
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"

	"github.com/sagan/ptool/cmd"
	"github.com/sagan/ptool/util"
	"github.com/sagan/ptool/util/torrentutil"
)

var command = &cobra.Command{
	Use:         "verifytorrent {file.torrent}... {--save-path dir | --content-path path} [--check]",
	Annotations: map[string]string{"cobra-prompt-dynamic-suggestions": "verifytorrent"},
	Aliases:     []string{"verify"},
	Short:       "Verify *.torrent file(s) are consistent with local disk files.",
	Long: `Verify *.torrent file(s) are consistent with local disk files.

Example:
ptool verifytorrent file1.torrent file2.torrent --save-path /root/Downloads
ptool verifytorrent file.torrent --content-path /root/Downloads/TorrentContentFolder

Exact one (but not both) of the --save-path or --content-path flag must be set.
* --save-path dir : the parent folder of torrent content(s)
* --content-path path : the torrent content(s) path, could be a folder or a single file

If you provide multiple {file.torrent} args, only --save-path flag can be used.

By default it will only examine file meta infos (file path & size).
If --check flag is set, it will also do the hash checking.`,
	Args: cobra.MatchAll(cobra.MinimumNArgs(1), cobra.OnlyValidArgs),
	RunE: verifytorrent,
}

var (
	savePath    = ""
	contentPath = ""
	checkHash   = false
	showAll     = false
)

func init() {
	command.Flags().BoolVarP(&checkHash, "check", "", false, "Do hash checking when verifying torrent files")
	command.Flags().BoolVarP(&showAll, "all", "a", false, "Show all info")
	command.Flags().StringVarP(&savePath, "save-path", "", "", "The parent folder path of torrent content(s). (Exact one of this and --content-path flag MUST be set)")
	command.Flags().StringVarP(&contentPath, "content-path", "", "", "The path of torrent content. Can only be used with single torrent arg (Exact one of this and --save-path flag MUST be set)")
	cmd.RootCmd.AddCommand(command)
}

func verifytorrent(cmd *cobra.Command, args []string) error {
	if savePath == "" && contentPath == "" || (savePath != "" && contentPath != "") {
		return fmt.Errorf("exact one of the --save-path or --content-path (but not both) flag must be set")
	}
	torrentFilenames := util.ParseFilenameArgs(args...)
	if len(torrentFilenames) > 1 && contentPath != "" {
		return fmt.Errorf("you must use --save-path flag (instead of --content-path) when verifying multiple torrents")
	}
	errorCnt := int64(0)

	for i, torrentFilename := range torrentFilenames {
		if showAll && i > 0 {
			fmt.Printf("\n")
		}
		var torrentContent []byte
		var err error
		if torrentFilename == "-" {
			torrentContent, err = io.ReadAll(os.Stdin)
		} else {
			torrentContent, err = os.ReadFile(torrentFilename)
		}
		if err != nil {
			fmt.Printf("X torrent %s: failed to read torrent file: %v\n", torrentFilename, err)
			errorCnt++
			continue
		}
		torrentMeta, err := torrentutil.ParseTorrent(torrentContent, 99)
		if err != nil {
			fmt.Printf("X torrent %s: failed to parse torrent file: %v\n", torrentFilename, err)
			errorCnt++
			continue
		}
		if showAll {
			torrentMeta.Print(torrentFilename, true)
		}
		err = torrentMeta.Verify(savePath, contentPath, checkHash)
		if err != nil {
			fmt.Printf("X torrent %s: contents do NOT match with disk content(s) (did hash check = %t): %v\n",
				torrentFilename, checkHash, err)
			errorCnt++
		} else {
			fmt.Printf("✓ torrent %s: contents match with disk content(s) (did hash check = %t)\n",
				torrentFilename, checkHash)
		}
	}
	if errorCnt > 0 {
		return fmt.Errorf("%d errors", errorCnt)
	}
	return nil
}
