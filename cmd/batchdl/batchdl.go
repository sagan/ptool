package batchdl

// 批量下载站点的种子

import (
	"encoding/csv"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/sagan/ptool/client"
	"github.com/sagan/ptool/cmd"
	"github.com/sagan/ptool/cmd/common"
	"github.com/sagan/ptool/config"
	"github.com/sagan/ptool/constants"
	"github.com/sagan/ptool/site"
	"github.com/sagan/ptool/util"
	"github.com/sagan/ptool/util/torrentutil"
)

var command = &cobra.Command{
	Use:         "batchdl {site} [--action add|download|...] [--base-url torrents_page_url]",
	Annotations: map[string]string{"cobra-prompt-dynamic-suggestions": "batchdl"},
	Aliases:     []string{"ebookgod"},
	Short:       "Batch download the smallest (or by any other order) torrents from a site.",
	Long: `Batch download the smallest (or by any other order) torrents from a site.
It also supports directly adding downloaded torrent to a client.

To set the name of added torrent in client or filename of downloaded torrent, use --rename <name> flag,
which supports the following variable placeholders:
* [size] : Torrent size
* [id] :  Torrent id in site
* [site] : Torrent site
* [filename] : Original torrent filename without ".torrent" extension
* [filename128] : The prefix of [filename] which is at max 128 bytes
* [name] : Torrent name
* [name128] : The prefix of torrent name which is at max 128 bytes

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
	slowMode             = false
	downloadSkipExisting = false
	downloadAll          = false
	onePage              = false
	addPaused            = false
	dense                = false
	addRespectNoadd      = false
	includeDownloaded    = false
	onlyDownloaded       = false
	freeOnly             = false
	noPaid               = false
	noNeutral            = false
	nohr                 = false
	allowBreak           = false
	addCategoryAuto      = false
	largestFlag          = false
	newestFlag           = false
	maxTorrents          = int64(0)
	minSeeders           = int64(0)
	maxSeeders           = int64(0)
	maxConsecutiveFail   = int64(0)
	addCategory          = ""
	addClient            = ""
	addTags              = ""
	filter               = ""
	excludes             = ""
	addSavePath          = ""
	minTorrentSizeStr    = ""
	maxTorrentSizeStr    = ""
	maxTotalSizeStr      = ""
	freeTimeAtLeastStr   = ""
	startPage            = ""
	downloadDir          = ""
	exportFile           = ""
	baseUrl              = ""
	rename               = ""
	action               = ""
	sortFlag             = ""
	orderFlag            = ""
	saveFilename         = ""
	saveOkFilename       = ""
	saveFailFilename     = ""
	includes             = []string{}
)

func init() {
	command.Flags().BoolVarP(&slowMode, "slow", "", false, "Slow mode. wait after downloading each torrent")
	command.Flags().BoolVarP(&downloadSkipExisting, "download-skip-existing", "", false,
		`Used with "--action download". Do NOT re-download torrent that same name file already exists in local dir. `+
			`If this flag is set, the download torrent filename ("--rename" flag) will be fixed to `+
			`"[site].[id].torrent" (e.g.: "mteam.12345.torrent") format`)
	command.Flags().BoolVarP(&downloadAll, "all", "a", false,
		`Download all torrents. Equivalent to "--include-downloaded --min-seeders -1"`)
	command.Flags().BoolVarP(&onePage, "one-page", "", false, "Only fetch one page torrents")
	command.Flags().BoolVarP(&addPaused, "add-paused", "", false, "Add torrents to client in paused state")
	command.Flags().BoolVarP(&dense, "dense", "d", false, "Dense mode: show full torrent title & subtitle")
	command.Flags().BoolVarP(&freeOnly, "free", "", false, "Skip none-free torrent")
	command.Flags().BoolVarP(&noPaid, "no-paid", "", false, "Skip paid (use bonus points) torrent")
	command.Flags().BoolVarP(&noNeutral, "no-neutral", "", false,
		"Skip neutral (do not count uploading & downloading & seeding bonus) torrent")
	command.Flags().BoolVarP(&largestFlag, "largest", "l", false,
		`Sort site torrents by size in desc order. Equivalent to "--sort size --order desc"`)
	command.Flags().BoolVarP(&newestFlag, "newest", "n", false,
		`Download newest torrents of site. Equivalent to "--sort time --order desc --one-page"`)
	command.Flags().BoolVarP(&addRespectNoadd, "add-respect-noadd", "", false,
		`Used with "--action add". Check and respect _noadd flag in client`)
	command.Flags().BoolVarP(&nohr, "no-hr", "", false,
		"Skip torrent that has any type of HnR (Hit and Run) restriction")
	command.Flags().BoolVarP(&allowBreak, "break", "", false,
		"Break (stop finding more torrents) if all torrents of current page do not meet criterion")
	command.Flags().BoolVarP(&includeDownloaded, "include-downloaded", "", false,
		"Do NOT skip torrent that had been downloaded before")
	command.Flags().BoolVarP(&onlyDownloaded, "only-downloaded", "", false,
		"Only include torrent that had been downloaded before")
	command.Flags().BoolVarP(&addCategoryAuto, "add-category-auto", "", false,
		"Automatically set category of added torrent to corresponding sitename")
	command.Flags().Int64VarP(&maxTorrents, "max-torrents", "", -1,
		"Number limit of torrents handled. -1 == no limit (Press Ctrl+C to stop)")
	command.Flags().StringVarP(&minTorrentSizeStr, "min-torrent-size", "", "-1",
		"Skip torrent with size smaller than (<) this value. -1 == no limit")
	command.Flags().StringVarP(&maxTorrentSizeStr, "max-torrent-size", "", "-1",
		"Skip torrent with size larger than (>) this value. -1 == no limit")
	command.Flags().StringVarP(&maxTotalSizeStr, "max-total-size", "", "-1",
		"Will at most download torrents with total contents size of this value. -1 == no limit")
	command.Flags().Int64VarP(&minSeeders, "min-seeders", "", 1,
		"Skip torrent with seeders less than (<) this value. -1 == no limit")
	command.Flags().Int64VarP(&maxSeeders, "max-seeders", "", -1,
		"Skip torrent with seeders more than (>) this value. -1 == no limit")
	command.Flags().Int64VarP(&maxConsecutiveFail, "max-consecutive-fail", "", 3,
		"Stop after consecutive fails to download torrent from site of this times. -1 == no limit (never stop)")
	command.Flags().StringVarP(&freeTimeAtLeastStr, "free-time", "", "",
		"Used with --free. Set the allowed minimal remaining torrent free time. e.g.: 12h, 1d")
	command.Flags().StringVarP(&filter, "filter", "", "",
		"If set, skip torrent which title or subtitle does NOT contains this string")
	command.Flags().StringArrayVarP(&includes, "includes", "", nil,
		"Comma-separated list that ONLY torrent which title or subtitle contains any one in the list will be downloaded. "+
			"Can be provided multiple times, in which case every list MUST be matched")
	command.Flags().StringVarP(&excludes, "excludes", "", "",
		"Comma-separated list that torrent which title of subtitle contains any one in the list will be skipped")
	command.Flags().StringVarP(&startPage, "start-page", "", "",
		"Start fetching torrents from here (should be the returned LastPage value last time you run this command)")
	command.Flags().StringVarP(&downloadDir, "download-dir", "", ".",
		`Used with "--action download". Set the local dir of downloaded torrents. Default == current dir`)
	command.Flags().StringVarP(&addClient, "add-client", "", "",
		`Used with "--action add". Set the client. Required in this action`)
	command.Flags().StringVarP(&addCategory, "add-category", "", "",
		`Used with "--action add". Set the category when adding torrent to client`)
	command.Flags().StringVarP(&addTags, "add-tags", "", "",
		`Used with "--action add". Set the tags when adding torrent to client (comma-separated)`)
	command.Flags().StringVarP(&addSavePath, "add-save-path", "", "",
		`Used with "--action add". Set contents save path of added torrents`)
	command.Flags().StringVarP(&exportFile, "export-file", "", "",
		`Used with "--action export|printid". Set the output file. (If not set, will use stdout)`)
	command.Flags().StringVarP(&baseUrl, "base-url", "", "",
		`Manually set the base url of torrents list page. e.g.: "special.php", "torrents.php?cat=100"`)
	command.Flags().StringVarP(&rename, "rename", "", "", "Rename downloaded or added torrents (supports variables)")
	command.Flags().StringVarP(&saveFilename, "save-torrent-list", "", "",
		"Filename. Write the list of torrent ids to it (File will be truncated)")
	command.Flags().StringVarP(&saveOkFilename, "save-torrent-ok-list", "", "",
		"Filename. Write the list of success torrent ids to it (File will be truncated)")
	command.Flags().StringVarP(&saveFailFilename, "save-torrent-fail-list", "", "",
		"Filename. Write the list of failed torrent ids to it (File will be truncated)")
	cmd.AddEnumFlagP(command, &action, "action", "", ActionEnumFlag)
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
	if downloadAll {
		includeDownloaded = true
		onlyDownloaded = false
		minSeeders = -1
	}
	if largestFlag && newestFlag {
		return fmt.Errorf("--largest and --newest flags are NOT compatible")
	}
	if onlyDownloaded && includeDownloaded {
		return fmt.Errorf("--only-downloaded and --include-downloaded flags are NOT compatible")
	}
	if action != "download" && (downloadSkipExisting || downloadDir != ".") {
		return fmt.Errorf(`found flags that are can only be used with "--action download"`)
	} else if action != "add" && util.CountNonZeroVariables(
		addCategoryAuto, addCategory, addClient, addPaused, addRespectNoadd, addSavePath) > 0 {
		return fmt.Errorf(`found flags that are can only be used with "--action add"`)
	}
	if action != "download" && action != "add" && (saveOkFilename != "" || saveFailFilename != "") {
		return fmt.Errorf(`found flags that are can only be used with "--action download" or "--action add"`)
	}
	if util.CountNonZeroVariables(downloadSkipExisting, rename) > 1 {
		return fmt.Errorf("--download-skip-existing and --rename flags are NOT compatible")
	}
	if largestFlag {
		sortFlag = "size"
		orderFlag = "desc"
	} else if newestFlag {
		sortFlag = "time"
		orderFlag = "desc"
		onePage = true
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
	maxTotalSize, _ := util.RAMInBytes(maxTotalSizeStr)
	desc := false
	if orderFlag == "desc" {
		desc = true
	}
	freeTimeAtLeast := int64(0)
	if freeTimeAtLeastStr != "" {
		t, err := util.ParseTimeDuration(freeTimeAtLeastStr)
		if err != nil {
			return fmt.Errorf("invalid --free-time value %s: %v", freeTimeAtLeastStr, err)
		}
		freeTimeAtLeast = t
	}
	if nohr && siteInstance.GetSiteConfig().GlobalHnR {
		log.Errorf("No torrents will be downloaded: site %s enforces global HnR restrictions",
			siteInstance.GetName(),
		)
		return nil
	}
	var clientInstance client.Client
	var clientAddTorrentOption *client.TorrentOption
	var clientAddFixedTags []string
	var outputFileFd *os.File = os.Stdout
	var csvWriter *csv.Writer
	if action == "add" {
		if addClient == "" {
			return fmt.Errorf("you much specify the client used to add torrents to via --add-client flag")
		}
		clientInstance, err = client.CreateClient(addClient)
		if err != nil {
			return fmt.Errorf("failed to create client %s: %v", addClient, err)
		}
		status, err := clientInstance.GetStatus()
		if err != nil {
			return fmt.Errorf("failed to get client %s status: %v", clientInstance.GetName(), err)
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
	} else if action == "export" || action == "printid" {
		if exportFile != "" {
			outputFileFd, err = os.OpenFile(exportFile, os.O_RDWR|os.O_APPEND|os.O_CREATE, constants.PERM)
			if err != nil {
				return fmt.Errorf("failed to create output file %s: %v", exportFile, err)
			}
		}
		if action == "export" {
			csvWriter = csv.NewWriter(outputFileFd)
			csvWriter.Write([]string{"name", "size", "time", "id"})
		}
	}
	flowControlInterval := config.DEFAULT_SITE_FLOW_CONTROL_INTERVAL
	if siteInstance.GetSiteConfig().FlowControlInterval > 0 {
		flowControlInterval = siteInstance.GetSiteConfig().FlowControlInterval
	}
	var saveFile, saveOkFile, saveFailFile *os.File
	var saveFiles = []**os.File{&saveFile, &saveOkFile, &saveFailFile}
	for i, filename := range []string{saveFilename, saveOkFilename, saveFailFilename} {
		if filename != "" {
			*saveFiles[i], err = os.OpenFile(filename, os.O_TRUNC|os.O_CREATE|os.O_WRONLY, constants.PERM)
			if err != nil {
				return fmt.Errorf("failed to create save file %s: %v", filename, err)
			}
		}
	}

	cntTorrents := int64(0)
	cntAllTorrents := int64(0)
	totalSize := int64(0)
	totalAllSize := int64(0)
	errorCnt := int64(0)
	consecutiveFail := int64(0)
	var torrents []site.Torrent
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
		if csvWriter != nil {
			csvWriter.Flush()
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
			if !includeDownloaded && torrent.IsActive {
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
					log.Debugf("Skip none-free torrent %s", torrent.Name)
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
			if saveFile != nil {
				saveFile.WriteString(torrent.Id + "\n")
			}
			cntTorrents++
			cntTorrentsThisPage++
			totalSize += torrent.Size
			var err error
			if action == "show" {
				site.PrintTorrents([]site.Torrent{torrent}, "", now, cntTorrents != 1, dense, nil)
			} else if action == "export" {
				csvWriter.Write([]string{torrent.Name, fmt.Sprint(torrent.Size), fmt.Sprint(torrent.Time), torrent.Id})
			} else if action == "printid" {
				fmt.Fprintf(outputFileFd, "%s\n", torrent.Id)
			} else {
				filename := ""
				if action == "download" && downloadSkipExisting && torrent.Id != "" {
					filename = fmt.Sprintf("%s.%s.torrent", sitename, torrent.Id)
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
					fmt.Printf("torrent %s (%s): failed to download: %v\n", torrent.Id, torrent.Name, err)
					consecutiveFail++
					if maxConsecutiveFail >= 0 && consecutiveFail > maxConsecutiveFail {
						log.Errorf("Abort due to too many fails to download torrent from site")
						break mainloop
					}
				} else {
					consecutiveFail = 0
					if tinfo, err := torrentutil.ParseTorrent(torrentContent, 99); err != nil {
						fmt.Printf("torrent %s (%s): failed to parse: %v\n", torrent.Id, torrent.Name, err)
					} else {
						if action == "download" {
							if filename == "" {
								if rename == "" {
									filename = _filename
								} else {
									filename = torrentutil.RenameTorrent(rename, sitename, torrent.Id, _filename, tinfo)
								}
							}
							err = os.WriteFile(filepath.Join(downloadDir, filename), torrentContent, constants.PERM)
							if err != nil {
								fmt.Printf("torrent %s: failed to write to %s/file %s: %v\n", torrent.Id, downloadDir, _filename, err)
							} else {
								fmt.Printf("torrent %s - %s (%s): downloaded to %s/%s\n", torrent.Id, torrent.Name,
									util.BytesSize(float64(torrent.Size)), downloadDir, filename)
							}
						} else if action == "add" {
							tags := []string{}
							tags = append(tags, clientAddFixedTags...)
							if tinfo.IsPrivate() {
								tags = append(tags, config.PRIVATE_TAG)
							}
							if torrent.HasHnR || siteInstance.GetSiteConfig().GlobalHnR {
								tags = append(tags, config.HR_TAG)
							}
							clientAddTorrentOption.Tags = tags
							if addCategoryAuto {
								clientAddTorrentOption.Category = sitename
							} else {
								clientAddTorrentOption.Category = addCategory
							}
							if rename != "" {
								clientAddTorrentOption.Name = torrentutil.RenameTorrent(rename, sitename, torrent.Id, _filename, tinfo)
							}
							err = clientInstance.AddTorrent(torrentContent, clientAddTorrentOption, nil)
							if err != nil {
								fmt.Printf("torrent %s (%s): failed to add to client: %v\n", torrent.Id, torrent.Name, err)
							} else {
								fmt.Printf("torrent %s - %s (%s) (seeders=%d, time=%s): added to client\n", torrent.Id, torrent.Name,
									util.BytesSize(float64(torrent.Size)), torrent.Seeders, util.FormatDuration(now-torrent.Time))
							}
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
			if maxTorrents >= 0 && cntTorrents >= maxTorrents {
				break mainloop
			}
			if maxTotalSize >= 0 && maxTotalSize-totalSize <= maxTotalSize/100 {
				break mainloop
			}
		}
		if onePage || marker == "" {
			break
		}
		if cntTorrentsThisPage == 0 {
			if allowBreak {
				break
			} else {
				log.Warnf("Warning, current page %s has no required torrents.", lastMarker)
			}
		}
		log.Warnf("Finish handling page %s. Torrents(Size/Cnt) | AllTorrents(Size/Cnt) till now: %s/%d | %s/%d. "+
			"Will process next page %s in %d seconds. Press Ctrl + C to stop",
			lastMarker, util.BytesSize(float64(totalSize)), cntTorrents,
			util.BytesSize(float64(totalAllSize)), cntAllTorrents, marker, flowControlInterval)
		util.Sleep(flowControlInterval)
	}
	doneHandle()
	return nil
}
