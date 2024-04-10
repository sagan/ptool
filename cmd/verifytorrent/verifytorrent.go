package verifytorrent

import (
	"fmt"
	"os"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/sagan/ptool/cmd"
	"github.com/sagan/ptool/util"
	"github.com/sagan/ptool/util/helper"
)

var command = &cobra.Command{
	Use: "verifytorrent {torrentFilename | torrentId | torrentUrl}... " +
		"{--save-path dir | --content-path path | --use-comment-meta} [--check | --check-quick]",
	Annotations: map[string]string{"cobra-prompt-dynamic-suggestions": "verifytorrent"},
	Aliases:     []string{"verify"},
	Short:       "Verify torrent file(s) are consistent with local disk contents.",
	Long: `Verify torrent file(s) are consistent with local disk contents.
Args is torrent list that each one could be a local filename (e.g. "*.torrent" or "[M-TEAM]CLANNAD.torrent"),
site torrent id (e.g.: "mteam.488424") or url (e.g.: "https://kp.m-team.cc/details.php?id=488424").
Torrent url that does NOT belong to any site (e.g.: a public site url) is also supported.
Use a single "-" to read .torrent file contents from stdin.

Example:
ptool verifytorrent file1.torrent file2.torrent --save-path /root/Downloads
ptool verifytorrent file.torrent --content-path /root/Downloads/TorrentContentFolder

Exact one (not less or more) of the --save-path, --content-path or --use-comment-meta flag must be set.
* --save-path dir : the parent folder of torrent content(s).
  It can be used with multiple torrent args.
* --content-path path : the torrent content(s) path, could be a folder or a single file.
  It can only be used with single torrent arg.
* --use-comment-meta : extract save path from torrent's comment field and use it.
  The "ptool export" and some other cmds can use the same flag to write save path to generated torrent files.

By default it will only examine file meta infos (file path & size).
If --check flag is set, it will also do the hash checking.`,
	Args: cobra.MatchAll(cobra.MinimumNArgs(1), cobra.OnlyValidArgs),
	RunE: verifytorrent,
}

var (
	renameFailed = false
	useComment   = false
	checkHash    = false
	checkQuick   = false
	forceLocal   = false
	showAll      = false
	contentPath  = ""
	defaultSite  = ""
	savePath     = ""
)

func init() {
	command.Flags().BoolVarP(&renameFailed, "rename-failed", "", false,
		"Rename verification failed *.torrent file to *.torrent.failed")
	command.Flags().BoolVarP(&useComment, "use-comment-meta", "", false,
		"Extract save path from 'comment' field of .torrent file and verify against it")
	command.Flags().BoolVarP(&checkHash, "check", "", false, "Do hash checking when verifying torrent files")
	command.Flags().BoolVarP(&checkQuick, "check-quick", "", false,
		"Do quick hash checking when verifying torrent files, "+
			"only the first and last piece of each file will do hash computing")
	command.Flags().BoolVarP(&forceLocal, "force-local", "", false, "Force treat all arg as local torrent filename")
	command.Flags().BoolVarP(&showAll, "all", "a", false, "Show all info")
	command.Flags().StringVarP(&contentPath, "content-path", "", "",
		"The path of torrent content. Can only be used with single torrent arg")
	command.Flags().StringVarP(&defaultSite, "site", "", "", "Set default site of torrent url")
	command.Flags().StringVarP(&savePath, "save-path", "", "", "The parent folder path of torrent content(s).")
	cmd.RootCmd.AddCommand(command)
}

func verifytorrent(cmd *cobra.Command, args []string) error {
	if util.CountNonZeroVariables(useComment, savePath, contentPath) != 1 {
		return fmt.Errorf("exact 1 of the --use-comment-meta & --save-path & --content-path flag must be set")
	}
	if checkHash && checkQuick {
		return fmt.Errorf("--check and --check-quick flags are NOT compatible")
	}
	torrents := util.ParseFilenameArgs(args...)
	if len(torrents) > 1 && contentPath != "" {
		return fmt.Errorf("--content-path flag can only be used to verify single torrent")
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
		_, tinfo, _, _, _, _, isLocal, err :=
			helper.GetTorrentContent(torrent, defaultSite, forceLocal, false, nil, false, nil)
		if err != nil {
			fmt.Printf("X torrent %s: failed to get: %v\n", torrent, err)
			errorCnt++
			continue
		}
		if showAll {
			tinfo.Print(torrent, true)
		}
		if useComment {
			if commentMeta := tinfo.DecodeComment(); commentMeta == nil {
				fmt.Printf("✕ %s : failed to parse comment meta\n", torrent)
				errorCnt++
				continue
			} else if commentMeta.SavePath == "" {
				fmt.Printf("✕ %s : comment meta has empty save_path\n", torrent)
				errorCnt++
				continue
			} else {
				log.Debugf("Found torrent %s comment meta %v", torrent, commentMeta)
				savePath = commentMeta.SavePath
			}
		}
		log.Infof("Verifying %s (savepath=%s, contentpath=%s, checkhash=%t)", torrent, savePath, contentPath, checkHash)
		err = tinfo.Verify(savePath, contentPath, checkMode)
		if err != nil {
			fmt.Printf("X torrent %s: contents do NOT match with disk content(s) (hash check = %s): %v\n",
				torrent, checkModeStr, err)
			errorCnt++
			if isLocal && torrent != "-" {
				if renameFailed {
					if err := os.Rename(torrent, torrent+".failed"); err != nil {
						log.Debugf("Failed to rename %s to *.failed: %v", torrent, err)
					}
				}
			}
		} else {
			fmt.Printf("✓ torrent %s: contents match with disk content(s) (hash check = %s). Save path: %v\n",
				torrent, checkModeStr, savePath)
		}
	}
	if errorCnt > 0 {
		return fmt.Errorf("%d errors", errorCnt)
	}
	return nil
}
