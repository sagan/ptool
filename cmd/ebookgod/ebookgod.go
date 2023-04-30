package ebookgod

// 电子书战神。批量下载最小的种子保种

import (
	"fmt"
	"os"
	"strings"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/sagan/ptool/client"
	"github.com/sagan/ptool/cmd"
	"github.com/sagan/ptool/site"
	"github.com/sagan/ptool/utils"
)

var command = &cobra.Command{
	Use:   "ebookgod <site>",
	Short: "Batch download the smallest torrents from a site",
	Long:  `Batch download the smallest torrents from a site`,
	Args:  cobra.MatchAll(cobra.ExactArgs(1), cobra.OnlyValidArgs),
	Run:   ebookgod,
}

var (
	addAutoStart      = false
	includeDead       = false
	includeDownloaded = false
	maxTorrents       = int64(0)
	addCategory       = ""
	addClient         = ""
	addTags           = ""
	filter            = ""
	minTorrentSizeStr = ""
	maxTorrentSizeStr = ""
	action            = ""
	startPage         = ""
	downloadDir       = ""
	outputFile        = ""
	baseUrl           = ""
)

func init() {
	command.Flags().BoolVar(&addAutoStart, "add-auto-start", false, "By default the added torrents in client will be in paused state unless this flag is set")
	command.Flags().BoolVar(&includeDead, "include-dead", false, "Do NOT skip dead (seeders == 0) torrents")
	command.Flags().BoolVar(&includeDownloaded, "include-downloaded", false, "Do NOT skip torrents that has been downloaded before")
	command.Flags().Int64VarP(&maxTorrents, "max-torrents", "m", 100, "Number limit of torrents handled. Default = 100. <=0 means unlimited")
	command.Flags().StringVar(&action, "action", "show", "Choose action for found torrents: show (print torrent details) | printid (print torrent id to stdout or file) | download (download torrent) | add (add torrent to client)")
	command.Flags().StringVar(&minTorrentSizeStr, "min-torrent-size", "0", "Skip torrents with size smaller than (<) this value")
	command.Flags().StringVar(&maxTorrentSizeStr, "max-torrent-size", "100MB", "Skip torrents with size large than (>=) this value")
	command.Flags().StringVarP(&filter, "filter", "f", "", "If set, skip torrents which name does NOT contains this string")
	command.Flags().StringVar(&startPage, "start-page", "", "Start fetching torrents from here (should be the returned LastPage value last time you run this command)")
	command.Flags().StringVar(&downloadDir, "download-dir", ".", "Used with '--action add'. Set the local dir of downloaded torrents. Default = current dir")
	command.Flags().StringVar(&addClient, "add-client", "", "Used with '--action add'. Set the client. Required in this action")
	command.Flags().StringVar(&addCategory, "add-category", "", "Used with '--action add'. Set the category when adding torrent to client")
	command.Flags().StringVar(&addTags, "add-tags", "", "Used with '--action add'. Set the tags when adding torrent to client (comma-separated)")
	command.Flags().StringVar(&outputFile, "output-file", "", "Used with '--action printid'. Set the output file. (If not set, will use stdout)")
	command.Flags().StringVar(&baseUrl, "base-url", "", "Manually set the base url of torrents list page. eg. adult.php or https://kp.m-team.cc/adult.php for M-Team site")
	cmd.RootCmd.AddCommand(command)
}

func ebookgod(cmd *cobra.Command, args []string) {
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
		}
		if addTags != "" {
			clientAddTorrentOption.Tags = strings.Split(addTags, ",")
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

	cntTorrents := int64(0)
	cntAllTorrents := int64(0)

	var torrents []site.Torrent
	var marker = startPage
	var lastMarker = ""
mainloop:
	for true {
		now := utils.Now()
		lastMarker = marker
		torrents, marker, err = siteInstance.GetAllTorrents("size", false, marker, baseUrl)

		if err != nil {
			log.Errorf("Failed to fetch next page torrents: %v", err)
			break
		}
		cntAllTorrents += int64(len(torrents))
		for _, torrent := range torrents {
			if torrent.Size < minTorrentSize {
				log.Tracef("Skip torrent %s due to size %d < minTorrentSize", torrent.Name, torrent.Size)
				continue
			}
			if torrent.Size >= maxTorrentSize {
				log.Tracef("Skip torrent %s due to size %d >= maxTorrentSize", torrent.Name, torrent.Size)
				break mainloop
			}
			if !includeDownloaded && torrent.IsActive {
				log.Tracef("Skip active torrent %s", torrent.Name)
				continue
			}
			if !includeDead && torrent.Seeders == 0 {
				log.Tracef("Skip dead torrent %s", torrent.Name)
				continue
			}
			if filter != "" && !utils.ContainsI(torrent.Name, filter) {
				log.Tracef("Skip torrent %s due to filter %s does NOT match", torrent.Name, filter)
				continue
			}
			cntTorrents++

			if action == "show" {
				site.PrintTorrents([]site.Torrent{torrent}, "", now, cntTorrents != 1)
			} else if action == "printid" {
				str := fmt.Sprintf("%s\n", torrent.Id)
				if outputFileFd != nil {
					outputFileFd.WriteString(str)
				} else {
					fmt.Printf(str)
				}
			} else {
				torrentContent, filename, err := siteInstance.DownloadTorrent(torrent.Id)
				if err != nil {
					fmt.Printf("torrent %s (%s): failed to download: %v\n", torrent.Id, torrent.Name, err)
				} else {
					if action == "download" {
						filename = fmt.Sprintf("%s.%s.%s.torrent", siteInstance.GetName(), torrent.Id, filename)
						err := os.WriteFile(downloadDir+"/"+filename, torrentContent, 0777)
						if err != nil {
							fmt.Printf("torrent %s: failed to write to %s/file %s: %v\n", torrent.Id, downloadDir, filename, err)
						} else {
							fmt.Printf("torrent %s: downloaded to %s/%s\n", torrent.Id, downloadDir, filename)
						}
					} else if action == "add" {
						err := clientInstance.AddTorrent(torrentContent, clientAddTorrentOption, nil)
						if err != nil {
							fmt.Printf("torrent %s (%s): failed to add to client: %v\n", torrent.Id, torrent.Name, err)
						} else {
							fmt.Printf("torrent %s (%s): added to client\n", torrent.Id, torrent.Name)
						}
					}
				}
			}

			if maxTorrents > 0 && cntTorrents >= maxTorrents {
				break mainloop
			}
		}
		if marker == "" {
			break
		}
		utils.Sleep(3)
	}
	fmt.Printf("\n"+`Done. Torrents / AllTorrents / LastPage: %d / %d / "%s"`+"\n", cntTorrents, cntAllTorrents, lastMarker)
}
