package batchdl

// 批量下载站点的种子

import (
	"fmt"
	"os"
	"strings"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/sagan/ptool/client"
	"github.com/sagan/ptool/cmd"
	"github.com/sagan/ptool/cmd/common"
	"github.com/sagan/ptool/site"
	"github.com/sagan/ptool/utils"
)

var command = &cobra.Command{
	Use:     "batchdl <site>",
	Aliases: []string{"ebookgod"},
	Short:   "Batch download the smallest (or by any other order) torrents from a site",
	Long:    `Batch download the smallest (or by any other order) torrents from a site`,
	Args:    cobra.MatchAll(cobra.ExactArgs(1), cobra.OnlyValidArgs),
	Run:     batchdl,
}

var (
	addAutoStart                                       = false
	includeDownloaded                                  = false
	freeOnly                                           = false
	allowBreak                                         = false
	maxTorrents                                        = int64(0)
	addCategory                                        = ""
	addClient                                          = ""
	addTags                                            = ""
	filter                                             = ""
	savePath                                           = ""
	minTorrentSizeStr                                  = ""
	maxTorrentSizeStr                                  = ""
	maxTotalSizeStr                                    = ""
	minSeeders                                         = int64(0)
	maxSeeders                                         = int64(0)
	freeTimeAtLeastStr                                 = ""
	action                                             = ""
	startPage                                          = ""
	downloadDir                                        = ""
	outputFile                                         = ""
	baseUrl                                            = ""
	sortFieldEnumFlag  common.SiteTorrentSortFieldEnum = "size"
	orderEnumFlag      common.OrderEnum                = "asc"
)

func init() {
	command.Flags().BoolVarP(&freeOnly, "free", "", false, "Skip none-free torrents")
	command.Flags().BoolVarP(&allowBreak, "break", "", false, "Break (stop finding more torrents) if all torrents of current page does not meet criterion")
	command.Flags().BoolVarP(&addAutoStart, "add-start", "", false, "By default the added torrents in client will be in paused state unless this flag is set")
	command.Flags().BoolVarP(&includeDownloaded, "include-downloaded", "", false, "Do NOT skip torrents that has been downloaded before")
	command.Flags().Int64VarP(&maxTorrents, "max-torrents", "m", 0, "Number limit of torrents handled. Default (0) == unlimited (Press Ctrl+C to stop at any time)")
	command.Flags().StringVarP(&action, "action", "", "show", "Choose action for found torrents: show (print torrent details) | printid (print torrent id to stdout or file) | download (download torrent) | add (add torrent to client)")
	command.Flags().StringVarP(&minTorrentSizeStr, "min-torrent-size", "", "0", "Skip torrents with size smaller than (<) this value")
	command.Flags().StringVarP(&maxTorrentSizeStr, "max-torrent-size", "", "0", "Skip torrents with size large than (>) this value. Default (0) == unlimited")
	command.Flags().StringVarP(&maxTotalSizeStr, "max-total-size", "", "0", "Will at most download torrents with contents of this value. Default (0) == unlimited")
	command.Flags().Int64VarP(&minSeeders, "min-seeders", "", 1, "Skip torrents with seeders less than (<) this value")
	command.Flags().Int64VarP(&maxSeeders, "max-seeders", "", 0, "Skip torrents with seeders large than (>) this value. Default(0) = no limit")
	command.Flags().StringVarP(&freeTimeAtLeastStr, "free-time", "", "", "Used with --free. Set the allowed minimal remaining torrent free time. eg. 12h, 1d")
	command.Flags().StringVarP(&filter, "filter", "f", "", "If set, skip torrents which name does NOT contains this string")
	command.Flags().StringVarP(&startPage, "start-page", "", "", "Start fetching torrents from here (should be the returned LastPage value last time you run this command)")
	command.Flags().StringVarP(&downloadDir, "download-dir", "", ".", "Used with '--action add'. Set the local dir of downloaded torrents. Default = current dir")
	command.Flags().StringVarP(&addClient, "add-client", "", "", "Used with '--action add'. Set the client. Required in this action")
	command.Flags().StringVarP(&addCategory, "add-category", "", "", "Used with '--action add'. Set the category when adding torrent to client")
	command.Flags().StringVarP(&addTags, "add-tags", "", "", "Used with '--action add'. Set the tags when adding torrent to client (comma-separated)")
	command.Flags().StringVarP(&savePath, "add-save-path", "", "", "Set save path of added torrents")
	command.Flags().StringVarP(&outputFile, "output-file", "", "", "Used with '--action printid'. Set the output file. (If not set, will use stdout)")
	command.Flags().StringVarP(&baseUrl, "base-url", "", "", "Manually set the base url of torrents list page. eg. adult.php or https://kp.m-team.cc/adult.php for M-Team site")
	command.Flags().VarP(&sortFieldEnumFlag, "sort", "s", "Manually Set the sort field, "+common.SiteTorrentSortFieldEnumTip)
	command.Flags().VarP(&orderEnumFlag, "order", "o", "Manually Set the sort order, "+common.OrderEnumTip)
	command.RegisterFlagCompletionFunc("sort", common.SiteTorrentSortFieldEnumCompletion)
	command.RegisterFlagCompletionFunc("order", common.OrderEnumCompletion)
	cmd.RootCmd.AddCommand(command)
}

func batchdl(cmd *cobra.Command, args []string) {
	siteInstance, err := site.CreateSite(args[0])
	if err != nil {
		log.Fatal(err)
	}

	if action != "show" && action != "printid" && action != "download" && action != "add" {
		log.Fatalf("Invalid action flag value: %s", action)
	}
	var clientInstance client.Client
	var clientAddTorrentOption *client.TorrentOption
	var outputFileFd *os.File
	if action == "add" {
		if addClient == "" {
			log.Fatalf("You much specify the client used to add torrents to via --add-client flag.")
		}
		clientInstance, err = client.CreateClient(addClient)
		if err != nil {
			log.Fatalf("Failed to create client %s: %v", addClient, err)
		}
		clientAddTorrentOption = &client.TorrentOption{
			Category: addCategory,
			Pause:    !addAutoStart,
			SavePath: savePath,
			Tags:     []string{client.GenerateTorrentTagFromSite(siteInstance.GetName())},
		}
		if addTags != "" {
			clientAddTorrentOption.Tags = append(clientAddTorrentOption.Tags, strings.Split(addTags, ",")...)
		}
	} else if action == "printid" {
		if outputFile != "" {
			outputFileFd, err = os.OpenFile(outputFile, os.O_RDWR|os.O_APPEND|os.O_CREATE, 0777)
			if err != nil {
				log.Fatalf("Failed to create output file %s: %v", outputFile, err)
			}
		}
	}
	minTorrentSize, _ := utils.RAMInBytes(minTorrentSizeStr)
	maxTorrentSize, _ := utils.RAMInBytes(maxTorrentSizeStr)
	maxTotalSize, _ := utils.RAMInBytes(maxTotalSizeStr)
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

	cntTorrents := int64(0)
	cntAllTorrents := int64(0)
	totalSize := int64(0)

	var torrents []site.Torrent
	var marker = startPage
	var lastMarker = ""
mainloop:
	for {
		now := utils.Now()
		lastMarker = marker
		log.Printf("Get torrents with page parker '%s'", marker)
		torrents, marker, err = siteInstance.GetAllTorrents(sortFieldEnumFlag.String(), desc, marker, baseUrl)
		cntTorrentsThisPage := 0

		if err != nil {
			log.Errorf("Failed to fetch next page torrents: %v", err)
			break
		}
		cntAllTorrents += int64(len(torrents))
		for _, torrent := range torrents {
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
			if torrent.Seeders < minSeeders {
				log.Tracef("Skip torrent %s due to too few seeders", torrent.Name)
				continue
			}
			if maxSeeders > 0 && torrent.Seeders > maxSeeders {
				log.Tracef("Skip torrent %s due to too more seeders", torrent.Name)
				continue
			}
			if filter != "" && !utils.ContainsI(torrent.Name, filter) {
				log.Tracef("Skip torrent %s due to filter %s does NOT match", torrent.Name, filter)
				continue
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
			cntTorrents++
			cntTorrentsThisPage++
			totalSize += torrent.Size

			if action == "show" {
				site.PrintTorrents([]site.Torrent{torrent}, "", now, cntTorrents != 1)
			} else if action == "printid" {
				str := fmt.Sprintf("%s\n", torrent.Id)
				if outputFileFd != nil {
					outputFileFd.WriteString(str)
				} else {
					fmt.Printf("%s", str)
				}
			} else {
				torrentContent, filename, err := siteInstance.DownloadTorrent(torrent.Id)
				if err != nil {
					fmt.Printf("torrent %s (%s): failed to download: %v\n", torrent.Id, torrent.Name, err)
				} else {
					if action == "download" {
						err := os.WriteFile(downloadDir+"/"+filename, torrentContent, 0777)
						if err != nil {
							fmt.Printf("torrent %s: failed to write to %s/file %s: %v\n", torrent.Id, downloadDir, filename, err)
						} else {
							fmt.Printf("torrent %s - %s (%s): downloaded to %s/%s\n", torrent.Id, torrent.Name, utils.BytesSize(float64(torrent.Size)), downloadDir, filename)
						}
					} else if action == "add" {
						err := clientInstance.AddTorrent(torrentContent, clientAddTorrentOption, nil)
						if err != nil {
							fmt.Printf("torrent %s (%s): failed to add to client: %v\n", torrent.Id, torrent.Name, err)
						} else {
							fmt.Printf("torrent %s - %s (%s): added to client\n", torrent.Id, torrent.Name, utils.BytesSize(float64(torrent.Size)))
						}
					}
				}
			}

			if maxTorrents > 0 && cntTorrents >= maxTorrents {
				break mainloop
			}
			if maxTotalSize > 0 && totalSize >= maxTotalSize {
				break mainloop
			}
		}
		if marker == "" {
			break
		}
		if cntTorrentsThisPage == 0 {
			if allowBreak {
				break
			} else {
				log.Warning("Warning, current page has no required torrents.")
			}
		}
		log.Printf("Finish handling current page. Will process next page in few seconds. Press Ctrl + C to stop")
		utils.Sleep(3)
	}
	fmt.Printf("\n"+`Done. Torrents / AllTorrents / LastPage: %d / %d / "%s"`+"\n", cntTorrents, cntAllTorrents, lastMarker)
}
