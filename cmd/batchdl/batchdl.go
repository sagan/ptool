package batchdl

// 批量下载站点的种子

import (
	"encoding/csv"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"golang.org/x/exp/slices"

	"github.com/sagan/ptool/client"
	"github.com/sagan/ptool/cmd"
	"github.com/sagan/ptool/cmd/common"
	"github.com/sagan/ptool/site"
	"github.com/sagan/ptool/utils"
)

var command = &cobra.Command{
	Use:     "batchdl <site>",
	Aliases: []string{"ebookgod"},
	Short:   "Batch download the smallest (or by any other order) torrents from a site.",
	Long:    `Batch download the smallest (or by any other order) torrents from a site.`,
	Args:    cobra.MatchAll(cobra.ExactArgs(1), cobra.OnlyValidArgs),
	Run:     batchdl,
}

var (
	onePage                                            = false
	addPaused                                          = false
	dense                                              = false
	addRespectNoadd                                    = false
	includeDownloaded                                  = false
	freeOnly                                           = false
	noPaid                                             = false
	nohr                                               = false
	allowBreak                                         = false
	addCategoryAuto                                    = false
	maxTorrents                                        = int64(0)
	minSeeders                                         = int64(0)
	maxSeeders                                         = int64(0)
	addCategory                                        = ""
	addClient                                          = ""
	addReserveSpaceStr                                 = ""
	addTags                                            = ""
	filter                                             = ""
	includes                                           = []string{}
	excludes                                           = ""
	savePath                                           = ""
	minTorrentSizeStr                                  = ""
	maxTorrentSizeStr                                  = ""
	maxTotalSizeStr                                    = ""
	freeTimeAtLeastStr                                 = ""
	action                                             = ""
	startPage                                          = ""
	downloadDir                                        = ""
	exportFile                                         = ""
	baseUrl                                            = ""
	sortFieldEnumFlag  common.SiteTorrentSortFieldEnum = "size"
	orderEnumFlag      common.OrderEnum                = "asc"
)

func init() {
	command.Flags().BoolVarP(&onePage, "one-page", "", false, "Only fetch one page torrents")
	command.Flags().BoolVarP(&addPaused, "add-paused", "", false, "Add torrents to client in paused state")
	command.Flags().BoolVarP(&dense, "dense", "", false, "Dense mode: show full torrent title & subtitle")
	command.Flags().BoolVarP(&freeOnly, "free", "", false, "Skip none-free torrent")
	command.Flags().BoolVarP(&noPaid, "no-paid", "", false, "Skip paid (use bonus points) torrent")
	command.Flags().BoolVarP(&addRespectNoadd, "add-respect-noadd", "", false, "Used with '--action add'. Check and respect _noadd flag in client")
	command.Flags().BoolVarP(&nohr, "no-hr", "", false, "Skip torrent that has any type of HnR (Hit and Run) restriction")
	command.Flags().BoolVarP(&allowBreak, "break", "", false, "Break (stop finding more torrents) if all torrents of current page do not meet criterion")
	command.Flags().BoolVarP(&includeDownloaded, "include-downloaded", "", false, "Do NOT skip torrent that has been downloaded before")
	command.Flags().BoolVarP(&addCategoryAuto, "add-category-auto", "", false, "Automatically set category of added torrent to corresponding sitename")
	command.Flags().Int64VarP(&maxTorrents, "max-torrents", "m", 0, "Number limit of torrents handled. Default (0) == unlimited (Press Ctrl+C to stop at any time)")
	command.Flags().StringVarP(&action, "action", "", "show", "Choose action for found torrents: show (print torrent details) | export (export torrents info [csv] to stdout or file) | printid (print torrent id to stdout or file) | download (download torrent) | add (add torrent to client)")
	command.Flags().StringVarP(&minTorrentSizeStr, "min-torrent-size", "", "0", "Skip torrent with size smaller than (<) this value")
	command.Flags().StringVarP(&maxTorrentSizeStr, "max-torrent-size", "", "0", "Skip torrent with size large than (>) this value. Default (0) == unlimited")
	command.Flags().StringVarP(&maxTotalSizeStr, "max-total-size", "", "0", "Will at most download torrents with total contents size of this value. Default (0) == unlimited")
	command.Flags().Int64VarP(&minSeeders, "min-seeders", "", 1, "Skip torrent with seeders less than (<) this value")
	command.Flags().Int64VarP(&maxSeeders, "max-seeders", "", -1, "Skip torrent with seeders large than (>) this value. Default (-1) == no limit")
	command.Flags().StringVarP(&freeTimeAtLeastStr, "free-time", "", "", "Used with --free. Set the allowed minimal remaining torrent free time. eg. 12h, 1d")
	command.Flags().StringVarP(&filter, "filter", "f", "", "If set, skip torrent which name does NOT contains this string")
	command.Flags().StringArrayVarP(&includes, "includes", "", nil, "A comma-separated string list. If set, ONLY torrent which name contains any one in the list will be downloaded. Can be provided multiple times, in which case every list MUST be matched")
	command.Flags().StringVarP(&excludes, "excludes", "", "", "A comma-separated string list that torrent which name contains any one in the list will be skipped")
	command.Flags().StringVarP(&startPage, "start-page", "", "", "Start fetching torrents from here (should be the returned LastPage value last time you run this command)")
	command.Flags().StringVarP(&downloadDir, "download-dir", "", ".", "Used with '--action download'. Set the local dir of downloaded torrents. Default == current dir")
	command.Flags().StringVarP(&addClient, "add-client", "", "", "Used with '--action add'. Set the client. Required in this action")
	command.Flags().StringVarP(&addCategory, "add-category", "", "", "Used with '--action add'. Set the category when adding torrent to client")
	command.Flags().StringVarP(&addReserveSpaceStr, "add-reserve-space", "", "0", "Used with '--action add'. Reserve client free disk space of at least this value. Will stop adding torrents if it would make client into state of insufficient space. eg. 10GiB. Default (0) == no limit")
	command.Flags().StringVarP(&addTags, "add-tags", "", "", "Used with '--action add'. Set the tags when adding torrent to client (comma-separated)")
	command.Flags().StringVarP(&savePath, "add-save-path", "", "", "Set contents save path of added torrents")
	command.Flags().StringVarP(&exportFile, "export-file", "", "", "Used with '--action export|printid'. Set the output file. (If not set, will use stdout)")
	command.Flags().StringVarP(&baseUrl, "base-url", "", "", `Manually set the base url of torrents list page. eg. "adult.php", "adult.php", "torrents.php?cat=100"`)
	command.Flags().VarP(&sortFieldEnumFlag, "sort", "s", "Manually set the sort field, "+common.SiteTorrentSortFieldEnumTip)
	command.Flags().VarP(&orderEnumFlag, "order", "o", "Manually set the sort order, "+common.OrderEnumTip)
	command.RegisterFlagCompletionFunc("sort", common.SiteTorrentSortFieldEnumCompletion)
	command.RegisterFlagCompletionFunc("order", common.OrderEnumCompletion)
	cmd.RootCmd.AddCommand(command)
}

func batchdl(cmd *cobra.Command, args []string) {
	sitename := args[0]
	siteInstance, err := site.CreateSite(sitename)
	if err != nil {
		log.Fatal(err)
	}

	if action != "show" && action != "export" && action != "printid" && action != "download" && action != "add" {
		log.Fatalf("Invalid action flag value: %s", action)
	}
	var includesList [][]string
	var excludesList []string
	for _, include := range includes {
		includesList = append(includesList, strings.Split(include, ","))
	}
	if excludes != "" {
		excludesList = strings.Split(excludes, ",")
	}
	minTorrentSize, _ := utils.RAMInBytes(minTorrentSizeStr)
	maxTorrentSize, _ := utils.RAMInBytes(maxTorrentSizeStr)
	maxTotalSize, _ := utils.RAMInBytes(maxTotalSizeStr)
	addReserveSpace, _ := utils.RAMInBytes(addReserveSpaceStr)
	addReserveSpaceGap := utils.Min(addReserveSpace/10, 10*1024*1024*1024)
	desc := false
	if orderEnumFlag == "desc" {
		desc = true
	}
	freeTimeAtLeast := int64(0)
	if freeTimeAtLeastStr != "" {
		t, err := utils.ParseTimeDuration(freeTimeAtLeastStr)
		if err != nil {
			log.Fatalf("Invalid --free-time value %s: %v", freeTimeAtLeastStr, err)
		}
		freeTimeAtLeast = t
	}
	if nohr && siteInstance.GetSiteConfig().GlobalHnR {
		log.Errorf("No torrents will be downloaded: site %s enforces global HnR restrictions",
			siteInstance.GetName(),
		)
		os.Exit(0)
	}
	var clientInstance client.Client
	var clientAddTorrentOption *client.TorrentOption
	var clientAddFixedTags []string
	var outputFileFd *os.File = os.Stdout
	var csvWriter *csv.Writer
	if action == "add" {
		if addClient == "" {
			log.Fatalf("You much specify the client used to add torrents to via --add-client flag.")
		}
		clientInstance, err = client.CreateClient(addClient)
		if err != nil {
			log.Fatalf("Failed to create client %s: %v", addClient, err)
		}
		status, err := clientInstance.GetStatus()
		if err != nil {
			log.Fatalf("Failed to get client %s status: %v", clientInstance.GetName(), err)
		}
		if addRespectNoadd && status.NoAdd {
			log.Warnf("Client has _noadd flag and --add-respect-noadd flag is set. Abort task")
			os.Exit(0)
		}
		if addReserveSpace > 0 {
			if status.FreeSpaceOnDisk < 0 {
				log.Warnf("Warning: client free space unknown")
			} else {
				freeSpace := int64(0)
				if addPaused {
					freeSpace = status.FreeSpaceOnDisk - status.UnfinishedSize
				} else {
					freeSpace = status.FreeSpaceOnDisk - status.UnfinishedDownloadingSize
				}
				addRemainSpace := freeSpace - addReserveSpace
				if addRemainSpace < addReserveSpaceGap {
					log.Warnf("Client free space insufficient. Abort task")
					os.Exit(0)
				}
				if maxTotalSize <= 0 || maxTotalSize > addRemainSpace {
					maxTotalSize = addRemainSpace
				}
			}
		}
		clientAddTorrentOption = &client.TorrentOption{
			Pause:    addPaused,
			SavePath: savePath,
		}
		clientAddFixedTags = []string{client.GenerateTorrentTagFromSite(siteInstance.GetName())}
		if addTags != "" {
			clientAddFixedTags = append(clientAddFixedTags, strings.Split(addTags, ",")...)
		}
	} else if action == "export" || action == "printid" {
		if exportFile != "" {
			outputFileFd, err = os.OpenFile(exportFile, os.O_RDWR|os.O_APPEND|os.O_CREATE, 0777)
			if err != nil {
				log.Fatalf("Failed to create output file %s: %v", exportFile, err)
			}
		}
		if action == "export" {
			csvWriter = csv.NewWriter(outputFileFd)
			csvWriter.Write([]string{"name", "size", "time", "id"})
		}
	}
	maxTotalSizeGap := utils.Max(maxTotalSize/100, addReserveSpaceGap/2)

	cntTorrents := int64(0)
	cntAllTorrents := int64(0)
	totalSize := int64(0)
	totalAllSize := int64(0)
	errorCnt := int64(0)
	var torrents []site.Torrent
	var marker = startPage
	var lastMarker = ""
	doneHandle := func() {
		fmt.Printf("\nDone. Torrents(Size/Cnt) | AllTorrents(Size/Cnt) | LastPage: %s/%d | %s/%d | \"%s\"; ErrorCnt: %d\n",
			utils.BytesSize(float64(totalSize)),
			cntTorrents,
			utils.BytesSize(float64(totalAllSize)),
			cntAllTorrents,
			lastMarker,
			errorCnt,
		)
		if csvWriter != nil {
			csvWriter.Flush()
		}
		if clientInstance != nil {
			clientInstance.Close()
		}
		if errorCnt > 0 {
			os.Exit(1)
		} else {
			os.Exit(0)
		}
	}
	sigs := make(chan os.Signal, 1)
	go func() {
		sig := <-sigs
		log.Debugf("Received signal %v", sig)
		doneHandle()
	}()
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
mainloop:
	for {
		now := utils.Now()
		lastMarker = marker
		log.Printf("Get torrents with page parker '%s'", marker)
		torrents, marker, err = siteInstance.GetAllTorrents(sortFieldEnumFlag.String(), desc, marker, baseUrl)
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
		for _, torrent := range torrents {
			totalAllSize += torrent.Size
			if torrent.Size < minTorrentSize {
				log.Tracef("Skip torrent %s due to size %d < minTorrentSize", torrent.Name, torrent.Size)
				if sortFieldEnumFlag == "size" && desc {
					break mainloop
				} else {
					continue
				}
			}
			if maxTorrentSize > 0 && torrent.Size > maxTorrentSize {
				log.Tracef("Skip torrent %s due to size %d > maxTorrentSize", torrent.Name, torrent.Size)
				if sortFieldEnumFlag == "size" && !desc {
					break mainloop
				} else {
					continue
				}
			}
			if !includeDownloaded && torrent.IsActive {
				log.Tracef("Skip active torrent %s", torrent.Name)
				continue
			}
			if minSeeders >= 0 && torrent.Seeders < minSeeders {
				log.Tracef("Skip torrent %s due to too few seeders", torrent.Name)
				if sortFieldEnumFlag == "seeders" && desc {
					break mainloop
				} else {
					continue
				}
			}
			if maxSeeders >= 0 && torrent.Seeders > maxSeeders {
				log.Tracef("Skip torrent %s due to too more seeders", torrent.Name)
				if sortFieldEnumFlag == "seeders" && !desc {
					break mainloop
				} else {
					continue
				}
			}
			if filter != "" && !torrent.MatchFilter(filter) {
				log.Tracef("Skip torrent %s due to filter %s does NOT match", torrent.Name, filter)
				continue
			}
			if index := slices.IndexFunc(excludesList, func(excludeStr string) bool {
				return torrent.MatchFilter(excludeStr)
			}); index != -1 {
				log.Tracef("Skip torrent %s due to exclude string %s matchs", torrent.Name, excludesList[index])
				continue
			}
			if len(includesList) > 0 {
				matched := true
				for _, includes := range includesList {
					if slices.IndexFunc(includes, func(includeStr string) bool {
						return torrent.MatchFilter(includeStr)
					}) == -1 {
						matched = false
						break
					}
				}
				if !matched {
					log.Tracef("Skip torrent %s due to includes does NOT match", torrent.Name)
					continue
				}
			}
			if freeOnly {
				if torrent.DownloadMultiplier != 0 {
					log.Tracef("Skip none-free torrent %s", torrent.Name)
					continue
				}
				if freeTimeAtLeast > 0 && torrent.DiscountEndTime > 0 && torrent.DiscountEndTime < now+freeTimeAtLeast {
					log.Tracef("Skip torrent %s which remaining free time is too short", torrent.Name)
					continue
				}
			}
			if nohr && torrent.HasHnR {
				log.Tracef("Skip HR torrent %s", torrent.Name)
				continue
			}
			if noPaid && torrent.Paid && !torrent.Bought {
				log.Tracef("Skip paid torrent %s", torrent.Name)
				continue
			}
			if maxTotalSize > 0 && totalSize+torrent.Size > maxTotalSize {
				log.Tracef("Skip torrent %s which would break max total size limit", torrent.Name)
				if sortFieldEnumFlag == "size" && !desc {
					break mainloop
				} else {
					continue
				}
			}
			cntTorrents++
			cntTorrentsThisPage++
			totalSize += torrent.Size
			var err error
			if action == "show" {
				site.PrintTorrents([]site.Torrent{torrent}, "", now, cntTorrents != 1, dense)
			} else if action == "export" {
				csvWriter.Write([]string{torrent.Name, fmt.Sprint(torrent.Size), fmt.Sprint(torrent.Time), torrent.Id})
			} else if action == "printid" {
				fmt.Fprintf(outputFileFd, "%s\n", torrent.Id)
			} else {
				var torrentContent []byte
				var filename string
				if torrent.DownloadUrl != "" {
					torrentContent, filename, err = siteInstance.DownloadTorrent(torrent.DownloadUrl)
				} else {
					torrentContent, filename, err = siteInstance.DownloadTorrent(torrent.Id)
				}
				if err != nil {
					fmt.Printf("torrent %s (%s): failed to download: %v\n", torrent.Id, torrent.Name, err)
				} else {
					if action == "download" {
						err = os.WriteFile(downloadDir+"/"+filename, torrentContent, 0777)
						if err != nil {
							fmt.Printf("torrent %s: failed to write to %s/file %s: %v\n", torrent.Id, downloadDir, filename, err)
						} else {
							fmt.Printf("torrent %s - %s (%s): downloaded to %s/%s\n", torrent.Id, torrent.Name, utils.BytesSize(float64(torrent.Size)), downloadDir, filename)
						}
					} else if action == "add" {
						tags := []string{}
						tags = append(tags, clientAddFixedTags...)
						if torrent.HasHnR || siteInstance.GetSiteConfig().GlobalHnR {
							tags = append(tags, "_hr")
						}
						clientAddTorrentOption.Tags = tags
						if addCategoryAuto {
							clientAddTorrentOption.Category = sitename
						} else {
							clientAddTorrentOption.Category = addCategory
						}
						err = clientInstance.AddTorrent(torrentContent, clientAddTorrentOption, nil)
						if err != nil {
							fmt.Printf("torrent %s (%s): failed to add to client: %v\n", torrent.Id, torrent.Name, err)
						} else {
							fmt.Printf("torrent %s - %s (%s) (seeders=%d, time=%s): added to client\n", torrent.Id, torrent.Name, utils.BytesSize(float64(torrent.Size)), torrent.Seeders, utils.FormatDuration(now-torrent.Time))
						}
					}
				}
			}
			if err != nil {
				errorCnt++
			}
			if maxTorrents > 0 && cntTorrents >= maxTorrents {
				break mainloop
			}
			if maxTotalSize > 0 && maxTotalSize-totalSize <= maxTotalSizeGap {
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
		log.Warnf("Finish handling page %s. Torrents(Size/Cnt) | AllTorrents(Size/Cnt) till now: %s/%d | %s/%d. Will process next page %s in few seconds. Press Ctrl + C to stop",
			lastMarker, utils.BytesSize(float64(totalSize)), cntTorrents, utils.BytesSize(float64(totalAllSize)), cntAllTorrents, marker)
		utils.Sleep(3)
	}
	doneHandle()
}
