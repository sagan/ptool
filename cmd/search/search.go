package search

import (
	"fmt"
	"sort"
	"strings"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/sagan/ptool/cmd"
	"github.com/sagan/ptool/config"
	"github.com/sagan/ptool/site"
	"github.com/sagan/ptool/utils"
)

type SearchResult struct {
	site     string
	torrents []site.Torrent
	err      error
}

var command = &cobra.Command{
	Use:   "search {siteOrGroups} {keyword}...",
	Short: "Search torrents by keyword in a site.",
	Long: `Search torrents by keyword in a site.
{siteOrGroups}: A comma-separated name list of sites or groups. Can use "_all" to search all sites.
`,
	Args: cobra.MatchAll(cobra.MinimumNArgs(2), cobra.OnlyValidArgs),
	RunE: search,
}

var (
	dense             = false
	largestFlag       = false
	maxResults        = int64(0)
	perSiteMaxREsults = int64(0)
	baseUrl           = ""
)

func init() {
	command.Flags().BoolVarP(&dense, "dense", "", false, "Dense mode: show full torrent title & subtitle")
	command.Flags().BoolVarP(&largestFlag, "largest", "l", false, "Sort search result by torrent size in desc order")
	command.Flags().Int64VarP(&maxResults, "max-results", "m", 100, "Number limit of search result of all sites combined. 0 == unlimited")
	command.Flags().Int64VarP(&perSiteMaxREsults, "per-site-max-results", "", 0, "Number limit of search result of any single site. Default (0) == unlimited")
	command.Flags().StringVarP(&baseUrl, "base-url", "", "", "Manually set the base url of search page. eg. adult.php or https://kp.m-team.cc/adult.php for M-Team site")
	cmd.RootCmd.AddCommand(command)
}

func search(cmd *cobra.Command, args []string) error {
	sitenames := config.ParseGroupAndOtherNames(strings.Split(args[0], ",")...)
	keyword := strings.Join(args[1:], " ")
	siteInstancesMap := map[string](site.Site){}
	for _, sitename := range sitenames {
		siteInstance, err := site.CreateSite(sitename)
		if err != nil {
			return fmt.Errorf("failed to create site %s: %v", sitename, err)
		}
		siteInstancesMap[sitename] = siteInstance
	}
	now := utils.Now()
	ch := make(chan SearchResult, len(sitenames))
	for _, sitename := range sitenames {
		go func(sitename string) {
			torrents, err := siteInstancesMap[sitename].SearchTorrents(keyword, baseUrl)
			ch <- SearchResult{sitename, torrents, err}
		}(sitename)
	}

	torrents := []site.Torrent{}
	errorStr := ""
	cntSuccessSites := int64(0)
	cntNoResultSites := int64(0)
	cntErrorSites := int64(0)
	for i := 0; i < len(sitenames); i++ {
		searchResult := <-ch
		if searchResult.err != nil {
			cntErrorSites++
			errorStr += fmt.Sprintf("failed to search site %s: %v", searchResult.site, searchResult.err)
		} else if len(searchResult.torrents) == 0 {
			cntNoResultSites++
		} else {
			cntSuccessSites++
			siteTorrents := searchResult.torrents
			if largestFlag {
				sort.Slice(siteTorrents, func(i, j int) bool {
					if siteTorrents[i].Size != siteTorrents[j].Size {
						return siteTorrents[i].Size > siteTorrents[j].Size
					}
					return siteTorrents[i].Seeders > siteTorrents[j].Seeders
				})
			}
			if perSiteMaxREsults > 0 && len(siteTorrents) > int(perSiteMaxREsults) {
				siteTorrents = siteTorrents[:perSiteMaxREsults]
			}
			torrents = append(torrents, siteTorrents...)
		}
	}
	if largestFlag {
		sort.Slice(torrents, func(i, j int) bool {
			if torrents[i].Size != torrents[j].Size {
				return torrents[i].Size > torrents[j].Size
			}
			return torrents[i].Seeders > torrents[j].Seeders
		})
	}
	if maxResults > 0 && len(torrents) > int(maxResults) {
		torrents = torrents[:maxResults]
	}
	fmt.Printf("Done searching %d sites. Success / NoResult / Error sites: %d / %d / %d. Showing %d result\n", cntSuccessSites+cntErrorSites+cntNoResultSites,
		cntSuccessSites, cntNoResultSites, cntErrorSites, len(torrents))
	if errorStr != "" {
		log.Warnf("Errors encountered: %s", errorStr)
	}
	fmt.Printf("\n")
	site.PrintTorrents(torrents, "", now, false, dense)
	return nil
}
