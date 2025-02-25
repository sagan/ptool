package search

import (
	"bytes"
	"fmt"
	"os"
	"sort"
	"strings"
	"text/template"
	"time"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/sagan/ptool/cmd"
	"github.com/sagan/ptool/config"
	"github.com/sagan/ptool/constants"
	"github.com/sagan/ptool/site"
	"github.com/sagan/ptool/util"
	"github.com/sagan/ptool/util/helper"
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

By default, it displays found torrents of site in the list, which has several fields like "Name" and "Free".

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
- If you are currently downloading or seeding this torrent, display a "*%".

You can also customize the output format of search result torrent using "--format string" flag.
The data passed to the template is the "site.Torrent" struct:

// https://github.com/sagan/ptool/blob/master/site/site.go
type Torrent struct {
	Name               string
	Description        string
	Id                 string // optional torrent id in the site
	InfoHash           string
	DownloadUrl        string
	DownloadMultiplier float64
	UploadMultiplier   float64
	DiscountEndTime    int64
	Time               int64 // torrent timestamp
	Size               int64
	IsSizeAccurate     bool
	Seeders            int64
	Leechers           int64
	Snatched           int64
	HasHnR             bool     // true if has any type of HR
	IsActive           bool     // true if torrent is or had ever been downloaded / seeding
	IsCurrentActive    bool     // true if torrent is currently downloading / seeding. If true, so will be IsActive
	Paid               bool     // "付费"种子: (第一次)下载或汇报种子时扣除魔力/积分
	Bought             bool     // 适用于付费种子：已购买
	Neutral            bool     // 中性种子：不计算上传、下载、做种魔力
	Tags               []string // labels, e.g. category and other meta infos.
}

The render result is trim spaced.
E.g. '--format "{{.Id}} {{.Name}} {{.Size}}"'

If "--json" flag is set, it prints the whole search result (found torrents
along with search meta data) in json object format instead.`,
	Args: cobra.MatchAll(cobra.MinimumNArgs(2), cobra.OnlyValidArgs),
	RunE: search,
}

var (
	dense              = false
	largestFlag        = false
	newestFlag         = false
	showJson           = false
	showIdOnly         = false
	minSeeders         = int64(0)
	maxResults         = int64(0)
	perSiteMaxResults  = int64(0)
	baseUrl            = ""
	minTorrentSizeStr  = ""
	maxTorrentSizeStr  = ""
	publishedAfterStr  = ""
	publishedBeforeStr = ""
	filter             = ""
	format             = ""
	includes           = []string{}
	excludes           = ""
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
	command.Flags().StringVarP(&minTorrentSizeStr, "min-torrent-size", "", "-1", constants.HELP_ARG_MIN_TORRENT_SIZE)
	command.Flags().StringVarP(&maxTorrentSizeStr, "max-torrent-size", "", "-1", constants.HELP_ARG_MAX_TORRENT_SIZE)
	command.Flags().StringVarP(&publishedAfterStr, "published-after", "", "",
		`Only showing torrent that was published after (>=) this time. `+constants.HELP_ARG_TIMES)
	command.Flags().StringVarP(&publishedBeforeStr, "published-before", "", "",
		`Only showing torrent that was published before (<) this time. `+constants.HELP_ARG_TIMES)
	command.Flags().StringVarP(&filter, "filter", "", "", "Filter search result additionally by title or subtitle")
	command.Flags().StringVarP(&format, "format", "", "", `Set the output format of each site torrent. `+
		`Available variable placeholders: {{.Id}}, {{.Size}} and more. `+constants.HELP_ARG_TEMPLATE)
	command.Flags().StringArrayVarP(&includes, "include", "", nil,
		"Comma-separated list that ONLY torrent which title or subtitle contains any one in the list will be included. "+
			"Can be provided multiple times, in which case every list MUST be matched")
	command.Flags().StringVarP(&excludes, "exclude", "", "",
		"Comma-separated list that torrent which title or subtitle contains any one in the list will be skipped")
	cmd.RootCmd.AddCommand(command)
}

func search(cmd *cobra.Command, args []string) error {
	var err error
	if util.CountNonZeroVariables(showIdOnly, showJson, format) > 1 {
		return fmt.Errorf("--json, --show-id-only, --format flags are NOT compatible")
	}
	var includesList [][]string
	for _, include := range includes {
		includesList = append(includesList, util.SplitCsv(include))
	}
	excludesList := util.SplitCsv(excludes)
	minTorrentSize, _ := util.RAMInBytes(minTorrentSizeStr)
	maxTorrentSize, _ := util.RAMInBytes(maxTorrentSizeStr)
	now := time.Now()
	var publishedAfter, publishedBefore int64
	if publishedAfterStr != "" {
		publishedAfter, err = util.ParseTimeWithNow(publishedAfterStr, nil, now)
		if err != nil {
			return fmt.Errorf("invalid published-after: %w", err)
		}
	}
	if publishedBeforeStr != "" {
		publishedBefore, err = util.ParseTimeWithNow(publishedBeforeStr, nil, now)
		if err != nil {
			return fmt.Errorf("invalid published-before: %w", err)
		}
	}

	sitenames := config.ParseGroupAndOtherNames(util.SplitCsv(args[0])...)
	keyword := strings.Join(args[1:], " ")
	siteInstancesMap := map[string]site.Site{}
	var outputTemplate *template.Template
	if format != "" {
		if outputTemplate, err = helper.GetTemplate(format); err != nil {
			return fmt.Errorf("invalid format template: %v", err)
		}
	}

	for _, sitename := range sitenames {
		siteInstance, err := site.CreateSite(sitename)
		if err != nil {
			return fmt.Errorf("failed to create site %s: %w", sitename, err)
		}
		siteInstancesMap[sitename] = siteInstance
	}
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
	cntTorrents := int64(0)
	for range sitenames {
		searchResult := <-ch
		if searchResult.err != nil {
			cntErrorSites++
			errorStr += fmt.Sprintf("failed to search site %s: %v", searchResult.site, searchResult.err)
		} else if len(searchResult.torrents) == 0 {
			cntNoResultSites++
		} else {
			cntSuccessSites++
			cntTorrents += int64(len(searchResult.torrents))
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
					publishedAfter > 0 && torrent.Time < publishedAfter ||
					publishedBefore > 0 && torrent.Time >= publishedBefore ||
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
	if outputTemplate != nil {
		for _, torrent := range torrents {
			buf := &bytes.Buffer{}
			if err := outputTemplate.Execute(buf, torrent); err == nil {
				fmt.Println(strings.TrimSpace(buf.String()))
			} else {
				log.Errorf("Torrent render error: %v", err)
			}
		}
		return nil
	} else if showJson {
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
	fmt.Printf("// Done searching %d sites. Success / NoResult / Error sites: %d / %d / %d. "+
		"Showing %d results filtered from %d hits\n", cntSuccessSites+cntErrorSites+cntNoResultSites, cntSuccessSites,
		cntNoResultSites, cntErrorSites, len(torrents), cntTorrents)
	if errorStr != "" {
		log.Warnf("Errors encountered: %s", errorStr)
	}
	fmt.Printf("\n")
	site.PrintTorrents(os.Stdout, torrents, "", now.Unix(), false, dense, nil)
	return nil
}
