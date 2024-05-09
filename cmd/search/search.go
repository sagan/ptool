package search

import (
	"fmt"
	"os"
	"sort"
	"strings"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/sagan/ptool/cmd"
	"github.com/sagan/ptool/config"
	"github.com/sagan/ptool/site"
	"github.com/sagan/ptool/util"
)

type SearchResult struct {
	site     string
	torrents []*site.Torrent
	err      error
}

var command = &cobra.Command{
	Use:         "search {siteOrGroups} {keyword}...",
	Annotations: map[string]string{"cobra-prompt-dynamic-suggestions": "search"},
	Short:       "Search torrents by keyword in sites.",
	Long: `Search torrents by keyword in sites.
{siteOrGroups}: Comma-separated list of sites or groups. Use "_all" to search all sites.

It displays found torrents of site in the list, which has several fields like "Name" and "Free".

The "Name" field by default displays the truncated prefix of the torrent name in site.
If "--dense" flag is set, it will instead display the full name of the torrent as well as it's description and tags.

The "Free" field displays some icon texts:
* ✓ : Torrent is free leech.
* $ : Torrent is paid. It will cost you bonus points when first time downloading it or announcing it in client.
* 2.0 : Torrent counts uploading as double.
* (1d12h) (e.g.) : The remaining time of torrent discount (free or 2.0 uploading).
* N : Neutral torrent, does not count uploading / downloading / bonus points.
* Z : Zero-traffic torrent, does not count uploading / downloading.

The "P" (progress) field also displays some icon texts:
- If you have never downloaded this torrent before, displays a "-".
- If you had ever downloaded or seeded this torrent before, display a "✓".
- If you are currently downloading or seeding this torrent, display a "*%".`,
	Args: cobra.MatchAll(cobra.MinimumNArgs(2), cobra.OnlyValidArgs),
	RunE: search,
}

var (
	dense             = false
	largestFlag       = false
	newestFlag        = false
	showJson          = false
	showIdOnly        = false
	minSeeders        = int64(0)
	maxResults        = int64(0)
	perSiteMaxResults = int64(0)
	baseUrl           = ""
	minTorrentSizeStr = ""
	maxTorrentSizeStr = ""
	publishedInStr    = ""
	filter            = ""
	includes          = []string{}
	excludes          = ""
)

func init() {
	command.Flags().BoolVarP(&dense, "dense", "d", false, "Dense mode: show full torrent title & subtitle")
	command.Flags().BoolVarP(&largestFlag, "largest", "l", false, "Sort search result by torrent size in desc order")
	command.Flags().BoolVarP(&newestFlag, "newest", "n", false, "Sort search result by torrent time in desc order")
	command.Flags().BoolVarP(&showJson, "json", "", false, "Show output in json format")
	command.Flags().BoolVarP(&showIdOnly, "show-id-only", "", false, "Output found torrent ids only")
	command.Flags().Int64VarP(&maxResults, "max-results", "", 100,
		"Number limit of search result of all sites combined. -1 == no limit")
	command.Flags().Int64VarP(&perSiteMaxResults, "per-site-max-results", "", -1,
		"Number limit of search result of any single site. -1 == no limit")
	command.Flags().StringVarP(&baseUrl, "base-url", "", "",
		`Manually set the base url of search page. "%s" can be used as search keyboard placeholder. `+
			`E.g. "special.php", "adult.php?incldead=1&search=%s"`)
	command.Flags().Int64VarP(&minSeeders, "min-seeders", "", 1,
		"Skip torrent with seeders less than (<) this value. -1 == no limit")
	command.Flags().StringVarP(&minTorrentSizeStr, "min-torrent-size", "", "-1",
		"Skip torrent with size smaller than (<) this value. -1 == no limit")
	command.Flags().StringVarP(&maxTorrentSizeStr, "max-torrent-size", "", "-1",
		"Skip torrent with size larger than (>) this value. -1 == no limit")
	command.Flags().StringVarP(&publishedInStr, "published-in", "", "",
		`Time duration. Only showing torrent that was published in the past time of this value. E.g. "30d"`)
	command.Flags().StringVarP(&filter, "filter", "", "", "Filter search result additionally by title or subtitle")
	command.Flags().StringArrayVarP(&includes, "includes", "", nil,
		"Comma-separated list that ONLY torrent which title or subtitle contains any one in the list will be included. "+
			"Can be provided multiple times, in which case every list MUST be matched")
	command.Flags().StringVarP(&excludes, "excludes", "", "",
		"Comma-separated list that torrent which title or subtitle contains any one in the list will be skipped")
	cmd.RootCmd.AddCommand(command)
}

func search(cmd *cobra.Command, args []string) error {
	if showIdOnly && showJson {
		return fmt.Errorf("--json and --show-id-only flags are NOT compatible")
	}
	var includesList [][]string
	var excludesList []string
	for _, include := range includes {
		includesList = append(includesList, util.SplitCsv(include))
	}
	if excludes != "" {
		excludesList = util.SplitCsv(excludes)
	}
	minTorrentSize, _ := util.RAMInBytes(minTorrentSizeStr)
	maxTorrentSize, _ := util.RAMInBytes(maxTorrentSizeStr)
	publishedIn, _ := util.ParseTimeDuration(publishedInStr)
	sitenames := config.ParseGroupAndOtherNames(util.SplitCsv(args[0])...)
	keyword := strings.Join(args[1:], " ")
	siteInstancesMap := map[string]site.Site{}
	for _, sitename := range sitenames {
		siteInstance, err := site.CreateSite(sitename)
		if err != nil {
			return fmt.Errorf("failed to create site %s: %w", sitename, err)
		}
		siteInstancesMap[sitename] = siteInstance
	}
	now := util.Now()
	ch := make(chan SearchResult, len(sitenames))
	for _, sitename := range sitenames {
		go func(sitename string) {
			torrents, err := siteInstancesMap[sitename].SearchTorrents(keyword, baseUrl)
			ch <- SearchResult{sitename, torrents, err}
		}(sitename)
	}

	torrents := []*site.Torrent{}
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
			if perSiteMaxResults >= 0 && len(siteTorrents) > int(perSiteMaxResults) {
				siteTorrents = siteTorrents[:perSiteMaxResults]
			}
			for _, torrent := range siteTorrents {
				if minSeeders >= 0 && torrent.Seeders < minSeeders ||
					minTorrentSize > 0 && torrent.Size < minTorrentSize ||
					maxTorrentSize > 0 && torrent.Size > maxTorrentSize ||
					publishedIn > 0 && now-torrent.Time > publishedIn ||
					filter != "" && !torrent.MatchFilter(filter) ||
					!torrent.MatchFiltersAndOr(includesList) ||
					torrent.MatchFiltersOr(excludesList) {
					continue
				}
				torrents = append(torrents, torrent)
			}
		}
	}
	if largestFlag {
		sort.Slice(torrents, func(i, j int) bool {
			if torrents[i].Size != torrents[j].Size {
				return torrents[i].Size > torrents[j].Size
			}
			return torrents[i].Seeders > torrents[j].Seeders
		})
	} else if newestFlag {
		sort.Slice(torrents, func(i, j int) bool {
			if torrents[i].Time != torrents[j].Time {
				return torrents[i].Time > torrents[j].Time
			}
			return torrents[i].Seeders > torrents[j].Seeders
		})
	}
	if maxResults >= 0 && len(torrents) > int(maxResults) {
		torrents = torrents[:maxResults]
	}
	if showJson {
		data := map[string]any{
			"successSites":  cntSuccessSites,
			"noResultSites": cntNoResultSites,
			"errorSites":    cntErrorSites,
			"errors":        errorStr,
			"torrents":      torrents,
		}
		return util.PrintJson(os.Stdout, data)
	} else if showIdOnly {
		for _, torrent := range torrents {
			fmt.Printf("%s\n", torrent.Id)
		}
		return nil
	}
	fmt.Printf("// Done searching %d sites. Success / NoResult / Error sites: %d / %d / %d. Showing %d result\n",
		cntSuccessSites+cntErrorSites+cntNoResultSites, cntSuccessSites, cntNoResultSites, cntErrorSites, len(torrents))
	if errorStr != "" {
		log.Warnf("Errors encountered: %s", errorStr)
	}
	fmt.Printf("\n")
	site.PrintTorrents(os.Stdout, torrents, "", now, false, dense, nil)
	return nil
}
