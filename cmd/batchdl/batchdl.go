package batchdl

// 批量下载站点的种子

import (
	"bytes"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"text/template"

	"github.com/natefinch/atomic"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/sagan/ptool/client"
	"github.com/sagan/ptool/cmd"
	"github.com/sagan/ptool/cmd/common"
	"github.com/sagan/ptool/config"
	"github.com/sagan/ptool/constants"
	"github.com/sagan/ptool/site"
	"github.com/sagan/ptool/util"
	"github.com/sagan/ptool/util/helper"
	"github.com/sagan/ptool/util/torrentutil"
)

var command = &cobra.Command{
	Use:         "batchdl {site} [--download | --add-client client] [--base-url torrents_page_url]",
	Annotations: map[string]string{"cobra-prompt-dynamic-suggestions": "batchdl"},
	Aliases:     []string{"ebookgod"},
	Short:       "Batch display or download torrents from a site.",
	Long: `Batch display or download torrents from a site.
By default it displays all non-dead torrents of site that have not been download before, in size asc order,
one page by page infinitely, until reachs the end of all site torrents. Press Ctrl+C to stop in the middle.
If --download flag is set, it will download found torrents to dir specified by "--download dir" flag (default ".").
If --add-client flag is set, it will directly add found torrents to the specified client.

For the default format of displayed torrents list, see help of "ptool search" command.

If "--json" flag is set, it prints torrents info in json format instead, one torrent json object each line.

You can also customize the output format of each torrent using "--format string" flag.
The data passed to the template is the "site.Torrent" struct, see help of "search" cmd.
The render result is trim spaced.
E.g. '--format "{{.Id}} {{.Name}} {{.Size}}"'

To query site torrents by any other order than size asc, use "--sort" and "--order" flags.

It supports resuming from the page that last time this command is interrupted,
using "--start-page" flag, set it to the "LastPage" value last time this command outputed in the end.

It supports saving info of found torrents to disk file in json format,
or exporting the id list of found torrents, using "--save-*" flags.

To set the name of added torrent in client or filename of downloaded torrent, use "--rename string" flag.
The template supports the following variables:
* size : Torrent size in string (e.g. "42GiB")
* id :  Torrent id in site
* site : Torrent site name
* filename : Original torrent filename without ".torrent" extension
* filename128 : The prefix of filename which is at max 128 bytes
* name : Torrent name
* name128 : The prefix of torrent name which is at max 128 bytes
* torrentInfo : The parsed "TorrentMeta" struct of torrent. See help of "parsetorrent" cmd
E.g. '--rename "{{.site}}.{{.id}} - {{.name128}}.torrent"'

It will output the summary of downloads result in the end:
* Torrents : Torrents downloaded
* AllTorrents : All torrents fetched, including not-downloaded (skipped)
* LastPage : The last processed site page. To continue (resume) downloading torrents from here,
  run the same command again with "--start-page page" flag set to this value
* ErrorCnt : Count of all types of errors (failed to download torrent or add torrent to client)`,
	Args: cobra.MatchAll(cobra.ExactArgs(1), cobra.OnlyValidArgs),
	RunE: batchdl,
}

var (
	showJson           = false
	doDownload         = false
	slowMode           = false
	skipExisting       = false
	downloadAll        = false
	onePage            = false
	addPaused          = false
	dense              = false
	addRespectNoadd    = false
	includeDownloaded  = false
	onlyDownloaded     = false
	freeOnly           = false
	noPaid             = false
	noNeutral          = false
	nohr               = false
	allowBreak         = false
	addCategoryAuto    = false
	largestFlag        = false
	latestFlag         = false
	newestFlag         = false
	saveAppend         = false
	maxTorrents        = int64(0)
	minSeeders         = int64(0)
	maxSeeders         = int64(0)
	maxConsecutiveFail = int64(0)
	addCategory        = ""
	addClient          = ""
	addTags            = ""
	tag                = ""
	filter             = ""
	excludes           = ""
	addSavePath        = ""
	minTorrentSizeStr  = ""
	maxTorrentSizeStr  = ""
	maxTotalSizeStr    = ""
	freeTimeAtLeastStr = ""
	publishedAfterStr  = ""
	startPage          = ""
	downloadDir        = ""
	baseUrl            = ""
	rename             = ""
	format             = ""
	sortFlag           = ""
	orderFlag          = ""
	saveFilename       = ""
	saveOkFilename     = ""
	saveFailFilename   = ""
	saveJsonFilename   = ""
	includes           = []string{}
)

func init() {
	command.Flags().BoolVarP(&showJson, "json", "", false,
		"Show output in json format (each line be the json object of a torrent)")
	command.Flags().BoolVarP(&doDownload, "download", "", false, "Do download found torrents to local")
	command.Flags().BoolVarP(&slowMode, "slow", "", false, "Slow mode. wait after downloading each torrent")
	command.Flags().BoolVarP(&skipExisting, "skip-existing", "", false,
		`Used with "--download". Do NOT re-download torrent that same name file already exists in local dir. `+
			`If this flag is set, the download torrent filename ("--rename" flag) will be fixed to `+
			`"[site].[id].torrent" (e.g. "mteam.12345.torrent") format`)
	command.Flags().BoolVarP(&downloadAll, "all", "a", false,
		`Display or download all torrents of site. Equivalent to "--include-downloaded --min-seeders -1"`)
	command.Flags().BoolVarP(&onePage, "one-page", "", false, "Only fetch one page torrents")
	command.Flags().BoolVarP(&addPaused, "add-paused", "", false, "Add torrents to client in paused state")
	command.Flags().BoolVarP(&dense, "dense", "d", false, "Dense mode: show full torrent title & subtitle")
	command.Flags().BoolVarP(&freeOnly, "free", "", false, "Skip non-free torrent")
	command.Flags().BoolVarP(&noPaid, "no-paid", "", false, "Skip paid (cost bonus points) torrent")
	command.Flags().BoolVarP(&noNeutral, "no-neutral", "", false,
		"Skip neutral (do not count uploading & downloading & seeding bonus) torrent")
	command.Flags().BoolVarP(&largestFlag, "largest", "l", false,
		`Sort site torrents by size in desc order. Equivalent to "--sort size --order desc"`)
	command.Flags().BoolVarP(&latestFlag, "latest", "L", false,
		`Only display or download latest (top page) torrents of site. `+
			`Equivalent to "--sort none --start-page 0 --one-page"`)
	command.Flags().BoolVarP(&newestFlag, "newest", "n", false,
		`Only display or download newest torrents of site. Equivalent to "--sort time --order desc --one-page"`)
	command.Flags().BoolVarP(&addRespectNoadd, "add-respect-noadd", "", false,
		`Used with "--add-client". Check and respect "`+config.NOADD_TAG+
			`" flag tag in client. If the tag exists in client, skip the execution (do not add any torrent to client)`)
	command.Flags().BoolVarP(&nohr, "no-hr", "", false,
		"Skip torrent that has any type of HnR (Hit and Run) restriction")
	command.Flags().BoolVarP(&allowBreak, "break", "", false,
		"Break (stop finding more torrents) if all torrents of current page do not meet criterion")
	command.Flags().BoolVarP(&includeDownloaded, "include-downloaded", "", false,
		"If set, it will also display or download torrents that had been downloaded before")
	command.Flags().BoolVarP(&onlyDownloaded, "only-downloaded", "", false,
		"Only display or download torrent that had been downloaded before")
	command.Flags().BoolVarP(&addCategoryAuto, "add-category-auto", "", false,
		"Automatically set category of added torrent to corresponding sitename")
	command.Flags().BoolVarP(&saveAppend, "save-append", "", false,
		`Used with "--save-*" flags, write to those files in append mode`)
	command.Flags().Int64VarP(&maxTorrents, "max-torrents", "", -1,
		"Number limit of torrents handled. -1 == no limit (Press Ctrl+C to stop)")
	command.Flags().StringVarP(&minTorrentSizeStr, "min-torrent-size", "", "-1", constants.HELP_ARG_MIN_TORRENT_SIZE)
	command.Flags().StringVarP(&maxTorrentSizeStr, "max-torrent-size", "", "-1", constants.HELP_ARG_MAX_TORRENT_SIZE)
	command.Flags().StringVarP(&maxTotalSizeStr, "max-total-size", "", "-1",
		"Will at most download torrents with total contents size of this value. -1 == no limit")
	command.Flags().Int64VarP(&minSeeders, "min-seeders", "", 1,
		"Skip torrent with seeders less than (<) this value. -1 == no limit")
	command.Flags().Int64VarP(&maxSeeders, "max-seeders", "", -1,
		"Skip torrent with seeders more than (>) this value. -1 == no limit")
	command.Flags().Int64VarP(&maxConsecutiveFail, "max-consecutive-fail", "", 3,
		"Stop after consecutive fails to download torrent from site of this times. -1 == no limit (never stop)")
	command.Flags().StringVarP(&freeTimeAtLeastStr, "free-time", "", "",
		`Used with "--free". Set the allowed minimal remaining torrent free time. e.g. 12h, 1d`)
	command.Flags().StringVarP(&publishedAfterStr, "published-after", "", "",
		`If set, only display or download torrent that was published after (>=) this. `+constants.HELP_ARG_TIMES)
	command.Flags().StringVarP(&filter, "filter", "", "",
		"If set, only display or download torrent which title or subtitle contains this string")
	command.Flags().StringVarP(&tag, "tag", "", "",
		"Comma-separated list. If set, only display or download torrent which tags contain any one in the list")
	command.Flags().StringArrayVarP(&includes, "include", "", nil,
		"Comma-separated list(s). If set, only torrents which title or subtitle contains any one in the list will be "+
			"displayed or downloaded. Can be set multiple times, in which case every list MUST be matched")
	command.Flags().StringVarP(&excludes, "exclude", "", "",
		"Comma-separated list that torrent which title of subtitle contains any one in the list will be skipped")
	command.Flags().StringVarP(&startPage, "start-page", "", "",
		"Start fetching torrents from here (should be the returned LastPage value last time you run this command). "+
			`To force start from the first / top page, set it to "0"`)
	command.Flags().StringVarP(&downloadDir, "download-dir", "", ".",
		`Used with "--download". Set the local dir of downloaded torrents. Default == current dir`)
	command.Flags().StringVarP(&addClient, "add-client", "", "", `Add found torrents to this client`)
	command.Flags().StringVarP(&addCategory, "add-category", "", "",
		`Used with "--add-client". Set the category when adding torrent to client`)
	command.Flags().StringVarP(&addTags, "add-tags", "", "",
		`Used with "--add-client". Set the tags when adding torrent to client (comma-separated)`)
	command.Flags().StringVarP(&addSavePath, "add-save-path", "", "",
		`Used with "--add-client". Set contents save path of added torrents`)
	command.Flags().StringVarP(&baseUrl, "base-url", "", "",
		`Manually set the base url of torrents list page. e.g. "special.php", "torrents.php?cat=100"`)
	command.Flags().StringVarP(&rename, "rename", "", "", `Rename downloaded or added torrents. `+
		`Available variable placeholders: {{.site}}, {{.id}} and more. `+constants.HELP_ARG_TEMPLATE)
	command.Flags().StringVarP(&format, "format", "", "", `Set the output format of each site torrent. `+
		`Available variable placeholders: {{.Id}}, {{.Size}} and more. `+constants.HELP_ARG_TEMPLATE)
	command.Flags().StringVarP(&saveFilename, "save-list-file", "", "",
		"Filename. Write the id list of found torrents to it. File will be truncated unless --save-apend flag is set")
	command.Flags().StringVarP(&saveOkFilename, "save-ok-list-file", "", "",
		"Filename. Write the id list of success torrents to it. File will be truncated unless --save-apend flag is set")
	command.Flags().StringVarP(&saveFailFilename, "save-fail-list-file", "", "",
		"Filename. Write the id list of failed torrents to it. File will be truncated unless --save-apend flag is set")
	command.Flags().StringVarP(&saveJsonFilename, "save-json-file", "", "",
		"Filename. Write the full info of found torrents to it in json format. "+
			"If --save-append flag is not set, file will be truncated and the whole contents of it will be a "+
			"valid json of array of torrent objects; If --save-append flag is set, each line of the file will be "+
			"json of torrent object")
	cmd.AddEnumFlagP(command, &sortFlag, "sort", "", common.SiteTorrentSortFlag)
	cmd.AddEnumFlagP(command, &orderFlag, "order", "", common.OrderFlag)
	cmd.RootCmd.AddCommand(command)
}

func batchdl(command *cobra.Command, args []string) error {
	sitename := args[0]
	siteInstance, err := site.CreateSite(sitename)
	if err != nil {
		return err
	}
	if util.CountNonZeroVariables(downloadAll, onlyDownloaded, includeDownloaded) > 1 {
		return fmt.Errorf("--all, --only-downloaded and --include-downloaded flags are NOT compatible")
	}
	if downloadAll {
		includeDownloaded = true
		onlyDownloaded = false
		minSeeders = -1
	}
	if util.CountNonZeroVariables(largestFlag, latestFlag, newestFlag) > 1 {
		return fmt.Errorf("--largest, --latest and --newest flags are NOT compatible")
	}
	if util.CountNonZeroVariables(doDownload, addClient) > 1 {
		return fmt.Errorf("--download and --add-client flags are NOT compatible")
	}
	if util.CountNonZeroVariables(showJson, format) > 1 {
		return fmt.Errorf("--json and --format flags are NOT compatible")
	}
	if !doDownload && (skipExisting || downloadDir != ".") {
		return fmt.Errorf(`found flags that are can only be used with "--download"`)
	} else if addClient == "" && util.CountNonZeroVariables(
		addCategoryAuto, addCategory, addClient, addPaused, addRespectNoadd, addSavePath) > 0 {
		return fmt.Errorf(`found flags that are can only be used with "--add-client"`)
	}
	if !doDownload && addClient == "" && (saveOkFilename != "" || saveFailFilename != "") {
		return fmt.Errorf(`found flags that are can only be used with "--download" or "--add-client"`)
	}
	if util.CountNonZeroVariables(skipExisting, rename) > 1 {
		return fmt.Errorf("--skip-existing and --rename flags are NOT compatible")
	}
	if largestFlag {
		sortFlag = "size"
		orderFlag = "desc"
	} else if latestFlag {
		sortFlag = constants.NONE
		startPage = "0"
		onePage = true
	} else if newestFlag {
		sortFlag = "time"
		orderFlag = "desc"
		onePage = true
	}
	var tagList = util.SplitCsv(tag)
	var excludesList = util.SplitCsv(excludes)
	var includesList [][]string
	for _, include := range includes {
		includesList = append(includesList, util.SplitCsv(include))
	}
	minTorrentSize, _ := util.RAMInBytes(minTorrentSizeStr)
	maxTorrentSize, _ := util.RAMInBytes(maxTorrentSizeStr)
	maxTotalSize, _ := util.RAMInBytes(maxTotalSizeStr)
	desc := false
	if orderFlag == "desc" {
		desc = true
	}
	freeTimeAtLeast := int64(0)
	if freeTimeAtLeastStr != "" {
		t, err := util.ParseTimeDuration(freeTimeAtLeastStr)
		if err != nil {
			return fmt.Errorf("invalid --free-time value %s: %w", freeTimeAtLeastStr, err)
		}
		freeTimeAtLeast = t
	}
	var publishedAfter int64
	if publishedAfterStr != "" {
		publishedAfter, err = util.ParseTime(publishedAfterStr, nil)
		if err != nil {
			return fmt.Errorf("invalid published-after: %w", err)
		}
	}
	if nohr && siteInstance.GetSiteConfig().GlobalHnR {
		log.Errorf("No torrents will be downloaded: site %s enforces global HnR policy",
			siteInstance.GetName(),
		)
		return nil
	}
	var clientInstance client.Client
	var clientAddTorrentOption *client.TorrentOption
	var clientAddFixedTags []string
	if addClient != "" {
		clientInstance, err = client.CreateClient(addClient)
		if err != nil {
			return fmt.Errorf("failed to create client %s: %w", addClient, err)
		}
		status, err := clientInstance.GetStatus()
		if err != nil {
			return fmt.Errorf("failed to get client %s status: %w", clientInstance.GetName(), err)
		}
		if addRespectNoadd && status.NoAdd {
			log.Warnf("Client has _noadd flag and --add-respect-noadd flag is set. Abort task")
			return nil
		}
		clientAddTorrentOption = &client.TorrentOption{
			Pause:    addPaused,
			SavePath: addSavePath,
		}
		clientAddFixedTags = []string{client.GenerateTorrentTagFromSite(siteInstance.GetName())}
		if addTags != "" {
			clientAddFixedTags = append(clientAddFixedTags, util.SplitCsv(addTags)...)
		}
	}
	flowControlInterval := config.DEFAULT_SITE_FLOW_CONTROL_INTERVAL
	if siteInstance.GetSiteConfig().FlowControlInterval > 0 {
		flowControlInterval = siteInstance.GetSiteConfig().FlowControlInterval
	}
	var saveFile, saveOkFile, saveFailFile, saveJsonFile *os.File
	var saveFiles = []**os.File{&saveFile, &saveOkFile, &saveFailFile, &saveJsonFile}
	for i, filename := range []string{saveFilename, saveOkFilename, saveFailFilename, saveJsonFilename} {
		if filename != "" {
			if saveAppend {
				*saveFiles[i], err = os.OpenFile(filename, os.O_APPEND|os.O_CREATE|os.O_WRONLY, constants.PERM)
			} else {
				*saveFiles[i], err = os.OpenFile(filename, os.O_TRUNC|os.O_CREATE|os.O_WRONLY, constants.PERM)
			}
			if err != nil {
				return fmt.Errorf("failed to create save file %s: %w", filename, err)
			}
		}
	}
	var renameTemplate, outputTemplate *template.Template
	if rename != "" {
		if renameTemplate, err = helper.GetTemplate(rename); err != nil {
			return fmt.Errorf("invalid rename template: %v", err)
		}
	}
	if format != "" {
		if outputTemplate, err = helper.GetTemplate(format); err != nil {
			return fmt.Errorf("invalid format template: %v", err)
		}
	}

	if saveJsonFile != nil && !saveAppend {
		saveJsonFile.WriteString("[\n")
	}

	cntTorrents := int64(0)
	cntAllTorrents := int64(0)
	totalSize := int64(0)
	totalAllSize := int64(0)
	errorCnt := int64(0)
	consecutiveFail := int64(0)
	var torrents []*site.Torrent
	var marker = startPage
	var lastMarker = ""
	doneHandle := func() {
		fmt.Fprintf(os.Stderr,
			"\n"+`Done. Torrents / AllTorrents / LastPage: %s (%d) / %s (%d) / "%s"; ErrorCnt: %d`+"\n",
			util.BytesSize(float64(totalSize)),
			cntTorrents,
			util.BytesSize(float64(totalAllSize)),
			cntAllTorrents,
			lastMarker,
			errorCnt,
		)
		if saveJsonFile != nil && !saveAppend {
			saveJsonFile.WriteString("]\n")
		}
		for _, file := range saveFiles {
			if *file != nil {
				(*file).Close()
			}
		}
	}
	sigs := make(chan os.Signal, 1)
	go func() {
		sig := <-sigs
		log.Debugf("Received signal %v", sig)
		doneHandle()
		if errorCnt > 0 {
			cmd.Exit(1)
		} else {
			cmd.Exit(0)
		}
	}()
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
mainloop:
	for {
		now := util.Now()
		lastMarker = marker
		log.Printf("Get torrents with page parker '%s'", marker)
		torrents, marker, err = siteInstance.GetAllTorrents(sortFlag, desc, marker, baseUrl)
		cntTorrentsThisPage := 0

		if err != nil {
			log.Errorf("Failed to fetch page %s torrents: %v", lastMarker, err)
			break
		}
		if len(torrents) == 0 {
			log.Warnf("No torrents found in page %s (may be an error). Abort", lastMarker)
			break
		}
		cntAllTorrents += int64(len(torrents))
		for i, torrent := range torrents {
			totalAllSize += torrent.Size
			if minTorrentSize >= 0 && torrent.Size < minTorrentSize {
				log.Debugf("Skip torrent %s due to size %d < minTorrentSize", torrent.Name, torrent.Size)
				if sortFlag == "size" && desc {
					break mainloop
				} else {
					continue
				}
			}
			if maxTorrentSize >= 0 && torrent.Size > maxTorrentSize {
				log.Debugf("Skip torrent %s due to size %d > maxTorrentSize", torrent.Name, torrent.Size)
				if sortFlag == "size" && !desc {
					break mainloop
				} else {
					continue
				}
			}
			if publishedAfter > 0 && torrent.Time < publishedAfter {
				log.Debugf("Skip torrent %s due to too old", torrent.Name)
				if sortFlag == "time" && desc {
					break mainloop
				} else {
					continue
				}
			}
			if !onlyDownloaded && !includeDownloaded && torrent.IsActive {
				log.Debugf("Skip active torrent %s", torrent.Name)
				continue
			}
			if onlyDownloaded && !torrent.IsActive {
				log.Debugf("Skip non-active torrent %s", torrent.Name)
				continue
			}
			if minSeeders >= 0 && torrent.Seeders < minSeeders {
				log.Debugf("Skip torrent %s due to too few seeders", torrent.Name)
				if sortFlag == "seeders" && desc {
					break mainloop
				} else {
					continue
				}
			}
			if maxSeeders >= 0 && torrent.Seeders > maxSeeders {
				log.Debugf("Skip torrent %s due to too more seeders", torrent.Name)
				if sortFlag == "seeders" && !desc {
					break mainloop
				} else {
					continue
				}
			}
			if filter != "" && !torrent.MatchFilter(filter) {
				log.Debugf("Skip torrent %s due to filter %s does NOT match", torrent.Name, filter)
				continue
			}
			if len(tagList) > 0 && !torrent.HasAnyTag(tagList) {
				log.Debugf("Skip torrent %s due to it does not contain any tag of %v", torrent.Name, tagList)
				continue
			}
			if torrent.MatchFiltersOr(excludesList) {
				log.Debugf("Skip torrent %s due to excludes matches", torrent.Name)
				continue
			}
			if !torrent.MatchFiltersAndOr(includesList) {
				log.Debugf("Skip torrent %s due to includes does NOT match", torrent.Name)
				continue
			}
			if freeOnly {
				if torrent.DownloadMultiplier != 0 {
					log.Debugf("Skip non-free torrent %s", torrent.Name)
					continue
				}
				if freeTimeAtLeast > 0 && torrent.DiscountEndTime > 0 && torrent.DiscountEndTime < now+freeTimeAtLeast {
					log.Debugf("Skip torrent %s which remaining free time is too short", torrent.Name)
					continue
				}
			}
			if nohr && torrent.HasHnR {
				log.Debugf("Skip HR torrent %s", torrent.Name)
				continue
			}
			if noPaid && torrent.Paid && !torrent.Bought {
				log.Debugf("Skip paid torrent %s", torrent.Name)
				continue
			}
			if noNeutral && torrent.Neutral {
				log.Debugf("Skip neutral torrent %s", torrent.Name)
				continue
			}
			if maxTotalSize >= 0 && totalSize+torrent.Size > maxTotalSize {
				log.Debugf("Skip torrent %s which would break max total size limit", torrent.Name)
				if sortFlag == "size" && !desc {
					break mainloop
				} else {
					continue
				}
			}
			if maxTorrents >= 0 && cntTorrents+1 > maxTorrents {
				break mainloop
			}
			if maxTotalSize >= 0 && totalSize+torrent.Size > maxTotalSize {
				break mainloop
			}
			if saveFile != nil {
				saveFile.WriteString(torrent.Id + "\n")
			}
			if saveJsonFile != nil {
				if !saveAppend && cntTorrents > 0 {
					saveJsonFile.WriteString(",")
				}
				if err = util.PrintJson(saveJsonFile, torrent); err != nil {
					log.Errorf("Failed to print json of torrent %v: %v", torrent, err)
				}
			}
			cntTorrents++
			cntTorrentsThisPage++
			totalSize += torrent.Size
			if !doDownload && addClient == "" {
				if outputTemplate != nil {
					buf := &bytes.Buffer{}
					if err := outputTemplate.Execute(buf, torrent); err == nil {
						fmt.Println(strings.TrimSpace(buf.String()))
					} else {
						log.Errorf("Torrent render error: %v", err)
					}
				} else if showJson {
					util.PrintJson(os.Stdout, torrent)
				} else {
					site.PrintTorrents(os.Stdout, []*site.Torrent{torrent}, "", now, cntTorrents != 1, dense, nil)
				}
				continue
			}
			var err error
			filename := ""
			if doDownload && skipExisting && torrent.Id != "" {
				filename = fmt.Sprintf("%s.%s.torrent", sitename, torrent.ID())
				if util.FileExistsWithOptionalSuffix(filepath.Join(downloadDir, filename),
					constants.ProcessedFilenameSuffixes...) {
					log.Debugf("Skip downloading local-existing torrent %s (%s)", torrent.Name, torrent.Id)
					continue
				}
			}
			if i > 0 && slowMode {
				util.Sleep(3)
			}
			var torrentContent []byte
			var _filename string
			if torrent.DownloadUrl != "" {
				torrentContent, _filename, _, err = siteInstance.DownloadTorrent(torrent.DownloadUrl)
			} else {
				torrentContent, _filename, _, err = siteInstance.DownloadTorrent(torrent.Id)
			}
			if err != nil {
				fmt.Fprintf(os.Stderr, "torrent %s (%s): failed to download: %v\n", torrent.Id, torrent.Name, err)
				consecutiveFail++
				if maxConsecutiveFail >= 0 && consecutiveFail > maxConsecutiveFail {
					log.Errorf("Abort due to too many consecutive fails to download torrent from site")
					break mainloop
				}
			} else {
				consecutiveFail = 0
				if tinfo, err := torrentutil.ParseTorrent(torrentContent); err != nil {
					fmt.Fprintf(os.Stderr, "torrent %s (%s): failed to parse: %v\n", torrent.Id, torrent.Name, err)
				} else {
					if doDownload {
						if filename == "" {
							if renameTemplate != nil {
								name, err := torrentutil.RenameTorrent(renameTemplate, sitename, torrent.Id, _filename, tinfo, false)
								if err == nil {
									filename = name
								} else {
									filename = _filename
									log.Errorf("torrent %s rename template render failed and is not renamed: %v", torrent.Id, err)
								}
							} else {
								filename = _filename
							}
						}
						err = atomic.WriteFile(filepath.Join(downloadDir, filename), bytes.NewReader(torrentContent))
						if err != nil {
							fmt.Fprintf(os.Stderr, "torrent %s: failed to write to %s/file %s: %v\n",
								torrent.Id, downloadDir, _filename, err)
						} else {
							fmt.Fprintf(os.Stderr, "torrent %s - %s (%s): downloaded to %s/%s\n", torrent.Id, torrent.Name,
								util.BytesSize(float64(torrent.Size)), downloadDir, filename)
						}
					} else if addClient != "" {
						tags := []string{}
						tags = append(tags, clientAddFixedTags...)
						ratioLimit := float64(0)
						if tinfo.IsPrivate() {
							tags = append(tags, config.PRIVATE_TAG)
						} else {
							tags = append(tags, config.PUBLIC_TAG)
							ratioLimit = config.Get().PublicTorrentRatioLimit
						}
						if torrent.HasHnR || siteInstance.GetSiteConfig().GlobalHnR {
							tags = append(tags, config.HR_TAG)
						}
						clientAddTorrentOption.Tags = tags
						clientAddTorrentOption.RatioLimit = ratioLimit
						if addCategoryAuto {
							clientAddTorrentOption.Category = sitename
						} else {
							clientAddTorrentOption.Category = addCategory
						}
						if rename != "" {
							if renameTemplate != nil {
								name, err := torrentutil.RenameTorrent(renameTemplate, sitename, torrent.Id, _filename, tinfo, false)
								if err == nil {
									clientAddTorrentOption.Name = name
								} else {
									log.Errorf("torrent %s rename template render failed and is not renamed: %v", torrent.Id, err)
								}
							}
						}
						err = clientInstance.AddTorrent(torrentContent, clientAddTorrentOption, nil)
						if err != nil {
							fmt.Fprintf(os.Stderr, "torrent %s (%s): failed to add to client: %v\n", torrent.Id, torrent.Name, err)
						} else {
							fmt.Fprintf(os.Stderr, "torrent %s - %s (%s) (seeders=%d, time=%s): added to client\n", torrent.Id,
								torrent.Name, util.BytesSize(float64(torrent.Size)),
								torrent.Seeders, util.FormatDuration(now-torrent.Time))
						}
					}
				}
			}
			if err != nil {
				errorCnt++
				if saveFailFile != nil {
					saveFailFile.WriteString(torrent.Id + "\n")
				}
			} else {
				if saveOkFile != nil {
					saveOkFile.WriteString(torrent.Id + "\n")
				}
			}
		} // current page torrents loop
		if onePage || marker == "" {
			break
		}
		if cntTorrentsThisPage == 0 {
			if allowBreak {
				break
			} else {
				log.Warnf("Warning, current page %s has no torrents that fulfil all conditions.", lastMarker)
			}
		}
		log.Warnf("Finish handling page %s. Torrents(Size/Cnt) | AllTorrents(Size/Cnt) till now: %s/%d | %s/%d. "+
			"Will process next page %s in %d seconds. Press Ctrl + C to stop",
			lastMarker, util.BytesSize(float64(totalSize)), cntTorrents,
			util.BytesSize(float64(totalAllSize)), cntAllTorrents, marker, flowControlInterval)
		util.Sleep(flowControlInterval)
	} // main loop
	doneHandle()
	return nil
}
