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
	Use:         "sites [{-b | -a}] [-f filter]",
	Annotations: map[string](string){"cobra-prompt-dynamic-suggestions": "iyuu.sites"},
	Short:       "Show iyuu sites list.",
	Long:        `Show iyuu sites list.`,
	RunE:        sites,
}

var (
	showBindable = false
	showAll      = false
	filter       = ""
)

func init() {
	command.Flags().BoolVarP(&showBindable, "bindable", "b", false, "Show bindable sites")
	command.Flags().StringVarP(&filter, "filter", "", "", "Filter sites. Only show sites which name / url / comment contain this string")
	command.Flags().BoolVarP(&showAll, "all", "a", false, "Show all iyuu sites (instead of only owned sites)")
	iyuu.Command.AddCommand(command)
}

func sites(cmd *cobra.Command, args []string) error {
	if showBindable {
		bindableSites, err := iyuu.IyuuApiGetRecommendSites()
		if err != nil {
			return fmt.Errorf("failed to get iyuu bindable sites: %v", err)
		}
		fmt.Printf("%-20s  %7s  %20s\n", "SiteName", "SiteId", "BindParams")
		for _, site := range bindableSites {
			if filter != "" && (!utils.ContainsI(site.Site, filter) && fmt.Sprint(site.Id) != filter) {
				continue
			}
			fmt.Printf("%-20s  %7d  %20s\n", site.Site, site.Id, site.Bind_check)
		}
		return nil
	}

	log.Tracef("iyuu token: %s", config.Get().IyuuToken)
	if config.Get().IyuuToken == "" {
		return fmt.Errorf("you must config iyuuToken in ptool.toml to use iyuu functions")
	}

	iyuuApiSites, err := iyuu.IyuuApiSites(config.Get().IyuuToken)
	if err != nil {
		return fmt.Errorf("failed to get iyuu sites: %v", err)
	}
	iyuuSites := utils.Map(iyuuApiSites, func(site iyuu.IyuuApiSite) iyuu.Site {
		return site.ToSite()
	})
	log.Printf("Iyuu sites: len(sites)=%v\n", len(iyuuSites))
	iyuu2LocalSiteMap := iyuu.GenerateIyuu2LocalSiteMap(iyuuSites, config.Get().Sites)

	if showAll {
		fmt.Printf("<all iyuu supported sites>\n")
	} else {
		fmt.Printf("<local sites supported by iyuu> (add -a flag to show all iyuu sites)\n")
	}
	fmt.Printf("%-10s  %-15s  %-6s  %-13s  %-30s  %-25s\n", "Nickname", "SiteName", "SiteId", "LocalSite", "SiteUrl", "DlPage")
	for _, iyuuSite := range iyuuSites {
		if iyuu2LocalSiteMap[iyuuSite.Sid] == "" {
			continue
		}
		if filter != "" && !iyuuSite.MatchFilter(filter) {
			continue
		}
		utils.PrintStringInWidth(iyuuSite.Nickname, 10, true)
		fmt.Printf("  %-15s  %-6d  %-13s  %-30s  %-25s\n", iyuuSite.Name, iyuuSite.Sid,
			iyuu2LocalSiteMap[iyuuSite.Sid], iyuuSite.Url, iyuuSite.DownloadPage)
	}

	if showAll {
		for _, iyuuSite := range iyuuSites {
			if iyuu2LocalSiteMap[iyuuSite.Sid] != "" {
				continue
			}
			if filter != "" && !iyuuSite.MatchFilter(filter) {
				continue
			}
			utils.PrintStringInWidth(iyuuSite.Nickname, 10, true)
			fmt.Printf("  %-15s  %-6d  %-13s  %-30s  %-25s\n", iyuuSite.Name, iyuuSite.Sid,
				"X (None)", iyuuSite.Url, iyuuSite.DownloadPage)
		}
	}
	return nil
}
