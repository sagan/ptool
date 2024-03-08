package verifytorrent

import (
	"fmt"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/sagan/ptool/cmd"
	"github.com/sagan/ptool/util"
	"github.com/sagan/ptool/util/helper"
)

var command = &cobra.Command{
	Use: "verifytorrent {torrentFilename | torrentId | torrentUrl}... " +
		"{--save-path dir | --content-path path} [--check | --check-quick]",
	Annotations: map[string]string{"cobra-prompt-dynamic-suggestions": "verifytorrent"},
	Aliases:     []string{"verify"},
	Short:       "Verify torrent file(s) are consistent with local disk content files.",
	Long: `Verify torrent file(s) are consistent with local disk files.
Args is torrent list that each one could be a local filename (e.g. "*.torrent" or "[M-TEAM]CLANNAD.torrent"),
site torrent id (e.g.: "mteam.488424") or url (e.g.: "https://kp.m-team.cc/details.php?id=488424").
Torrent url that does NOT belong to any site (e.g.: a public site url) is also supported.
Use a single "-" to read .torrent file contents from stdin.

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
	checkHash   = false
	checkQuick  = false
	forceLocal  = false
	showAll     = false
	contentPath = ""
	defaultSite = ""
	savePath    = ""
)

func init() {
	command.Flags().BoolVarP(&checkHash, "check", "", false, "Do hash checking when verifying torrent files")
	command.Flags().BoolVarP(&checkQuick, "check-quick", "", false,
		"Do quick hash checking when verifying torrent files, "+
			"only the first and last piece of each file will do hash computing")
	command.Flags().BoolVarP(&forceLocal, "force-local", "", false, "Force treat all arg as local torrent filename")
	command.Flags().BoolVarP(&showAll, "all", "a", false, "Show all info")
	command.Flags().StringVarP(&contentPath, "content-path", "", "",
		"The path of torrent content. Can only be used with single torrent arg "+
			"(Exact one of this and --save-path flag MUST be set)")
	command.Flags().StringVarP(&defaultSite, "site", "", "", "Set default site of torrent url")
	command.Flags().StringVarP(&savePath, "save-path", "", "",
		"The parent folder path of torrent content(s). (Exact one of this and --content-path flag MUST be set)")
	cmd.RootCmd.AddCommand(command)
}

func verifytorrent(cmd *cobra.Command, args []string) error {
	if savePath == "" && contentPath == "" || (savePath != "" && contentPath != "") {
		return fmt.Errorf("exact one of the --save-path or --content-path (but not both) flag must be set")
	}
	if checkHash && checkQuick {
		return fmt.Errorf("--check and --check-quick flags are NOT compatible")
	}
	torrents := util.ParseFilenameArgs(args...)
	if len(torrents) > 1 && contentPath != "" {
		return fmt.Errorf("you must use --save-path flag (instead of --content-path) when verifying multiple torrents")
	}
	errorCnt := int64(0)
	checkMode := int64(0)
	checkModeStr := "none"
	if checkQuick {
		checkMode = 1
		checkModeStr = "quick"
	} else if checkHash {
		checkMode = 99
		checkModeStr = "full"
	}

	for i, torrent := range torrents {
		if showAll && i > 0 {
			fmt.Printf("\n")
		}
		_, tinfo, _, _, _, _, err := helper.GetTorrentContent(torrent, defaultSite, forceLocal, false, nil, false)
		if err != nil {
			fmt.Printf("X torrent %s: failed to get: %v\n", torrent, err)
			errorCnt++
			continue
		}
		if showAll {
			tinfo.Print(torrent, true)
		}
		log.Infof("Verifying %s (savepath=%s, contentpath=%s, checkhash=%t)", torrent, savePath, contentPath, checkHash)
		err = tinfo.Verify(savePath, contentPath, checkMode)
		if err != nil {
			fmt.Printf("X torrent %s: contents do NOT match with disk content(s) (hash check = %s): %v\n",
				torrent, checkModeStr, err)
			errorCnt++
		} else {
			fmt.Printf("✓ torrent %s: contents match with disk content(s) (hash check = %s)\n", torrent, checkModeStr)
		}
	}
	if errorCnt > 0 {
		return fmt.Errorf("%d errors", errorCnt)
	}
	return nil
}
