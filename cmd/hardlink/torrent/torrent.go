package hardlinktorrent

// 功能类似 TorrentHardLinkHelper ( https://github.com/harrywong/torrenthardlinkhelper ).
// 参考: 种子硬链接工具 ( https://tieba.baidu.com/p/5572480043 ).
// 原作者: https://u2.dmhy.org/forums.php?action=viewtopic&forumid=7&topicid=6298 .

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/KarpelesLab/reflink"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/sagan/ptool/cmd/hardlink"
	"github.com/sagan/ptool/constants"
	"github.com/sagan/ptool/util"
	"github.com/sagan/ptool/util/torrentfilelocator"
	"github.com/sagan/ptool/util/torrentutil"
)

var command = &cobra.Command{
	Use:         "torrent {file.torrent} --content-path {contentPath} [--link-save-path {savePath}]",
	Annotations: map[string]string{"cobra-prompt-dynamic-suggestions": "hardlinkcp"},
	Short:       "Create hardlinked xseedable folder for a torrent from existing content folder.",
	Long: `Create hardlinked xseedable folder for a torrent from existing content folder.

It tries to locate all content files of {file.torrent} arg from "contentPath" disk folder,
even if existing files in disk have totally different dir structures and / or file names.

Note: the result is NOT guaranteed to be correct, there may be false positives.

By default, it only displays loating result.
If "--link-save-path string" flag is set, it generated hardlinks of located torrent contents
at the specified save path location.

E.g.
  ptool hardlink torrent MyTorrent.torrent --content-path ./MyTorrentContents
  ptool hardlink torrent MyTorrent.torrent --content-path ./MyTorrentContents --link-save-path ./Downloads
`,
	Args: cobra.MatchAll(cobra.ExactArgs(1), cobra.OnlyValidArgs),
	RunE: hardlinkcp,
}

var (
	force        = false
	useReflink   = false
	showJson     = false
	sizeLimitStr = ""
	contentPath  = ""
	linkSavePath = ""
)

func init() {
	command.Flags().BoolVarP(&force, "force", "", false, `Used with "--link-save-path" flag. `+
		`Generate hardlinks even if only partial torrent files are successfully located in disk`)
	command.Flags().BoolVarP(&useReflink, "use-reflink", "", false, constants.HELP_ARG_USE_REF_LINK)
	command.Flags().BoolVarP(&showJson, "json", "", false, "Show output in json format")
	command.Flags().StringVarP(&contentPath, "content-path", "", "",
		`The existing torrent content path (root folder or single file name)`)
	command.Flags().StringVarP(&linkSavePath, "link-save-path", "", "",
		`Generate hardlinks of matched torrent content at this save path location`)
	command.Flags().StringVarP(&sizeLimitStr, "hardlink-min-size", "", "1MiB",
		"File with size smaller than (<) this value will be copied instead of hardlinked. -1 == always hardlink")
	command.MarkFlagRequired("content-path")
	hardlink.Command.AddCommand(command)
}

func hardlinkcp(cmd *cobra.Command, args []string) error {
	if force && linkSavePath == "" {
		return fmt.Errorf(`"force" must be used with "link-save-path" flag`)
	}
	sizeLimit, _ := util.RAMInBytes(sizeLimitStr)
	torrent := args[0]
	torrentContents, err := os.ReadFile(torrent)
	if err != nil {
		return err
	}
	tinfo, err := torrentutil.ParseTorrent(torrentContents)
	if err != nil {
		return err
	}
	result := torrentfilelocator.Locate(tinfo, contentPath)
	if showJson {
		util.PrintJson(os.Stdout, result)
	} else {
		result.Print(os.Stdout)
	}
	if !result.Ok && !force {
		return fmt.Errorf(`failed to locate all torrent files in disk`)
	}
	if linkSavePath == "" {
		return nil
	}
	targetRootPath := filepath.Join(linkSavePath, tinfo.RootDir)
	if util.FileExists(targetRootPath) {
		if !util.IsEmptyDir(targetRootPath) {
			return fmt.Errorf("link target root %q already exists and is not an empty dir", targetRootPath)
		}
	} else if err := os.MkdirAll(targetRootPath, constants.PERM_DIR); err != nil {
		return fmt.Errorf("failed to create link target root dir: %w", err)
	}

	successCnt := int64(0)
	errCnt := int64(0)
	for _, fileLink := range result.TorrentFileLinks {
		if fileLink.State != torrentfilelocator.LocateStateLocated {
			continue
		}
		src := fileLink.FsFiles[fileLink.LinkedFsFileIndex].Path
		dst := filepath.Join(targetRootPath, fileLink.TorrentFile.Path)
		log.Debugf("Link %s => %s", src, dst)
		srcStat, err := os.Stat(src)
		if err == nil {
			if useReflink {
				err = reflink.Always(src, dst)
			} else if sizeLimit >= 0 && srcStat.Size() < sizeLimit {
				err = util.CopyFile(src, dst)
			} else {
				err = os.Link(src, dst)
			}
		}
		if err == nil {
			successCnt++
		} else {
			log.Errorf("Failed to link %s => %s: %v", src, dst, err)
			errCnt++
		}
	}
	if successCnt > 0 {
		log.Warnf("Linked torrent contents to %q. Linked/All files: %d/%d",
			targetRootPath, successCnt, len(result.TorrentFileLinks))
	}
	if errCnt > 0 {
		return fmt.Errorf("%d errors", errCnt)
	}
	return nil
}
