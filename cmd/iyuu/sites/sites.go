package sites

import (
	"fmt"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/sagan/ptool/cmd/iyuu"
	"github.com/sagan/ptool/config"
	"github.com/sagan/ptool/utils"
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
	log.Tracef("iyuu token: %s", config.Get().IyuuToken)
	if config.Get().IyuuToken == "" {
		log.Fatalf("You must config iyuuToken in ptool.yaml to use iyuu functions")
	}

	iyuuApiSites, err := iyuu.IyuuApiSites(config.Get().IyuuToken)
	if err != nil {
		log.Fatalf("Failed to get iyuu sites: %v", err)
	}
	iyuuSites := utils.Map(iyuuApiSites, func(site iyuu.IyuuApiSite) iyuu.Site {
		return site.ToSite()
	})
	log.Printf("Iyuu sites: len(sites)=%v\n", len(iyuuSites))
	iyuu2LocalSiteMap := iyuu.GenerateIyuu2LocalSiteMap(iyuuSites, config.Get().Sites)

	fmt.Printf("%-10s  %20s  %7s  %20s  %40s\n", "Nickname", "SiteName", "SiteId", "LocalSite", "SiteUrl")
	for _, iyuuSite := range iyuuSites {
		if iyuu2LocalSiteMap[iyuuSite.Sid] == "" {
			continue
		}
		utils.PrintStringInWidth(iyuuSite.Nickname, 10, true)
		fmt.Printf("  %20s  %7d  %20s  %40s\n", iyuuSite.Name, iyuuSite.Sid,
			iyuu2LocalSiteMap[iyuuSite.Sid], iyuuSite.Url)
	}

	if showAll {
		for _, iyuuSite := range iyuuSites {
			if iyuu2LocalSiteMap[iyuuSite.Sid] != "" {
				continue
			}
			utils.PrintStringInWidth(iyuuSite.Nickname, 10, true)
			fmt.Printf("  %20s  %7d  %20s  %40s\n", iyuuSite.Name, iyuuSite.Sid,
				"X (None)", iyuuSite.Url)
		}
	}
}
