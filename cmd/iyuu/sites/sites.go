package sites

import (
	"fmt"
	"os"

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
	showBindable = false
	showAll      = false
)

func init() {
	command.Flags().BoolVarP(&showBindable, "bindable", "b", false, "Show bindable sites")
	command.Flags().BoolVarP(&showAll, "all", "a", false, "Show all iyuu sites (instead of only owned sites)")
	iyuu.Command.AddCommand(command)
}

func sites(cmd *cobra.Command, args []string) {
	if showBindable {
		bindableSites, err := iyuu.IyuuApiGetRecommendSites()
		if err != nil {
			log.Fatalf("Failed to get iyuu bindable sites: %v", err)
		}
		fmt.Printf("%-20s  %7s  %20s\n", "SiteName", "SiteId", "BindParams")
		for _, site := range bindableSites {
			fmt.Printf("%-20s  %7d  %20s\n", site.Site, site.Id, site.Bind_check)
		}
		os.Exit(0)
	}

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
