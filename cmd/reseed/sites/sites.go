package sites

import (
	"fmt"
	"sort"

	"github.com/spf13/cobra"

	"github.com/sagan/ptool/cmd/reseed"
	"github.com/sagan/ptool/config"
)

var command = &cobra.Command{
	Use:         "sites",
	Annotations: map[string]string{"cobra-prompt-dynamic-suggestions": "reseed.sites"},
	Short:       "Show Reseed supported sites.",
	Long:        `Show Reseed supported sites.`,
	RunE:        sites,
}

func init() {
	reseed.Command.AddCommand(command)
}

func sites(cmd *cobra.Command, args []string) error {
	if config.Get().ReseedUsername == "" || config.Get().ReseedPassword == "" {
		return fmt.Errorf("you must config reseedUsername & reseedPassword in ptool.toml to use reseed functions")
	}
	token, err := reseed.Login(config.Get().ReseedUsername, config.Get().ReseedPassword)
	if err != nil {
		return fmt.Errorf("failed to login to reseed server: %v", err)
	}
	sites, err := reseed.GetSites(token)
	if err != nil {
		return fmt.Errorf("failed to get reseed sites: %v", err)
	}
	reseed2LocalMap := reseed.GenerateReseed2LocalSiteMap(sites, config.Get().Sites)
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
		localsite := reseed2LocalMap[site.Name]
		if localsite == "" {
			localsite = "X (None)"
		}
		fmt.Printf("%-15s  %-15s  %s\n", site.Name, localsite, site.BaseUrl)
	}
	return nil
}
