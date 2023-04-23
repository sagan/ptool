package sites

import (
	"fmt"

	log "github.com/sirupsen/logrus"

	"github.com/sagan/ptool/cmd/iyuu"
	"github.com/sagan/ptool/config"
	"github.com/sagan/ptool/utils"
	"github.com/spf13/cobra"
)

var command = &cobra.Command{
	Use:   "sites",
	Short: "Show iyuu sites list.",
	Long:  `Show iyuu sites list.`,
	Run:   sites,
}

var (
	showAll = false
)

func init() {
	command.Flags().BoolVarP(&showAll, "all", "a", false, "show all iyuu sites (instead of only owned sites).")
	iyuu.Command.AddCommand(command)
}

func sites(cmd *cobra.Command, args []string) {
	log.Print(config.ConfigFile, " ", args)
	log.Print("token", config.Get().IyuuToken)

	iyuuSites, err := iyuu.IyuuApiSites(config.Get().IyuuToken)
	log.Printf("Iyuu sites: error=%v, len(sites)=%v\n", err, len(iyuuSites))
	iyuu2LocalSiteMap := iyuu.GenerateIyuu2LocalSiteMap(iyuuSites, config.Get().Sites)

	fmt.Printf("%-10s  %20s  %7s  %20s  %40s\n", "Nickname", "SiteName", "SiteId", "LocalSite", "SiteUrl")
	for _, iyuuSite := range iyuuSites {
		if iyuu2LocalSiteMap[iyuuSite.Id] == "" {
			continue
		}
		utils.PrintStringInWidth(iyuuSite.Nickname, 10, true)
		fmt.Printf("  %20s  %7d  %20s  %40s\n", iyuuSite.Site, iyuuSite.Id,
			iyuu2LocalSiteMap[iyuuSite.Id], iyuuSite.GetUrl())
	}

	if showAll {
		for _, iyuuSite := range iyuuSites {
			if iyuu2LocalSiteMap[iyuuSite.Id] != "" {
				continue
			}
			utils.PrintStringInWidth(iyuuSite.Nickname, 10, true)
			fmt.Printf("  %20s  %7d  %20s  %40s\n", iyuuSite.Site, iyuuSite.Id,
				"X (None)", iyuuSite.GetUrl())
		}
	}
}
