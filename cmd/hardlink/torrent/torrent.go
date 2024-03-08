package hardlinktorrent

// 待开发，功能类似 TorrentHardLinkHelper ( https://github.com/harrywong/torrenthardlinkhelper ).
// 参考: 种子硬链接工具 ( https://tieba.baidu.com/p/5572480043 ).

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/sagan/ptool/cmd/hardlink"
)

var command = &cobra.Command{
	Use:         "torrent {file.torrent} --content-path {contentPath} --save-path {savePath}",
	Annotations: map[string]string{"cobra-prompt-dynamic-suggestions": "hardlinkcp"},
	Short:       "Create hardlinked xseedable folder for a torrent from existing content folder",
	Long:        `Create hardlinked xseedable folder for a torrent from existing content folder`,
	Args:        cobra.MatchAll(cobra.ExactArgs(1), cobra.OnlyValidArgs),
	RunE:        hardlinkcp,
}

var (
	sizeLimitStr = ""
	contentPath  = ""
	savePath     = ""
)

func init() {
	command.Flags().StringVarP(&contentPath, "content-path", "", "",
		`The existing torrent content path (root folder or single file name)`)
	command.Flags().StringVarP(&savePath, "save-path", "", "",
		`The output base dir of the generated hardlinked torrent content`)
	command.Flags().StringVarP(&sizeLimitStr, "hardlink-min-size", "", "1MiB",
		"File with size smaller than (<) this value will be copied instead of hardlinked. -1 == always hardlink")
	command.MarkFlagRequired("content-path")
	command.MarkFlagRequired("save-path")
	hardlink.Command.AddCommand(command)
}

func hardlinkcp(cmd *cobra.Command, args []string) error {
	// sizeLimit, _ := util.RAMInBytes(sizeLimitStr)
	return fmt.Errorf("unimplemented")
}
