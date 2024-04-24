package reseed

import (
	"github.com/spf13/cobra"

	"github.com/sagan/ptool/cmd"
)

// 使用 Reseed (https://github.com/tongyifan/Reseed-backend) 后端的自动辅种工具。
// 将找到的所有辅种 .torrent 文件下载到本地。
// 使用 ptool xseedadd 将辅种种子添加到客户端。

var Command = &cobra.Command{
	Use:   "reseed",
	Short: "Cross seed automation tool using Reseed (https://github.com/tongyifan/Reseed-backend) API.",
	Long:  `Cross seed automation tool using Reseed (https://github.com/tongyifan/Reseed-backend) API.`,
}

func init() {
	cmd.RootCmd.AddCommand(Command)
}
