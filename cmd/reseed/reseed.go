package reseed

import (
	"fmt"
	"sort"

	"github.com/spf13/cobra"

	"github.com/sagan/ptool/cmd"
	"github.com/sagan/ptool/config"
)

// 使用 Reseed (https://github.com/tongyifan/Reseed-backend) 后端的自动辅种工具。
// 将找到的所有辅种 .torrent 文件下载到本地。
// 使用 ptool xseedadd 将辅种种子添加到客户端。

var command = &cobra.Command{
	Use:   "reseed",
	Short: "Reseed client of https://github.com/tongyifan/Reseed-backend",
	Long:  `Reseed client of https://github.com/tongyifan/Reseed-backend .`,
	Args:  cobra.MatchAll(cobra.ExactArgs(0), cobra.OnlyValidArgs),
	RunE:  reseed,
}

var (
	showSites = false
)

func init() {
	command.Flags().BoolVarP(&showSites, "sites", "", false, "Show Reseed supported site list and exit")
	cmd.RootCmd.AddCommand(command)
}

func reseed(cmd *cobra.Command, args []string) error {
	token := config.Get().ReseedToken
	if token == "" {
		return fmt.Errorf("to use this cmd, reseedToken must be configed in ptool.toml")
	}
	if showSites {
		return sites()
	}
	return nil
}

func sites() error {
	sites, err := GetSites(config.Get().ReseedToken)
	if err != nil {
		return fmt.Errorf("failed to get reseed sites: %v", err)
	}
	reseed2LocalMap := GenerateReseed2LocalSiteMap(sites, config.Get().Sites)
	sort.Slice(sites, func(i, j int) bool {
		if reseed2LocalMap[sites[i].Name] != "" && reseed2LocalMap[sites[j].Name] == "" {
			return true
		}
		if reseed2LocalMap[sites[i].Name] == "" && reseed2LocalMap[sites[j].Name] != "" {
			return false
		}
		return sites[i].Name < sites[j].Name
	})

	fmt.Printf("%-15s  %-15s  %s\n", "ReseedSite", "LocalSite", "SiteUrl")
	for _, site := range sites {
		fmt.Printf("%-15s  %-15s  %s\n", site.Name, reseed2LocalMap[site.Name], site.BaseUrl)
	}
	return nil
}
