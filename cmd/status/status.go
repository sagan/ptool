package status

import (
	"fmt"
	"log"
	"os"

	"github.com/sagan/ptool/client"
	"github.com/sagan/ptool/cmd"
	"github.com/sagan/ptool/site"
	"github.com/sagan/ptool/utils"
	"github.com/spf13/cobra"
)

var (
	filter   = ""
	category = ""
	showAll  = false
)

var command = &cobra.Command{
	Use:   "status ...clients",
	Args:  cobra.MatchAll(cobra.MinimumNArgs(1), cobra.OnlyValidArgs),
	Short: "Show clients status",
	Long:  `A longer description`,
	Run:   status,
}

func init() {
	command.Flags().StringVar(&filter, "filter", "", "filter torrents by name")
	command.Flags().StringVar(&category, "category", "", "filter torrents by category")
	command.Flags().BoolVar(&showAll, "all", false, "show all torrents.")
	cmd.RootCmd.AddCommand(command)
}

func status(cmd *cobra.Command, args []string) {
	names := args
	isSingle := len(names) == 1
	hasError := false
	for _, name := range names {
		if client.ClientExists(name) {
			clientInstance, err := client.CreateClient(name)
			if err != nil {
				log.Printf("Failed to create client %s: %v", name, err)
				hasError = true
				continue
			}
			status, err := clientInstance.GetStatus()
			if err != nil {
				log.Printf("Cann't get client %s status: error=%v", clientInstance.GetName(), err)
				hasError = true
				continue
			}
			fmt.Printf("Client %s ↓ Speed/Limit: %s/s / %s/s; ↑ Speed/Limit: %s/s / %s/s; Free disk space: %s\n",
				clientInstance.GetName(),
				utils.BytesSize(float64(status.DownloadSpeed)),
				utils.BytesSize(float64(status.DownloadSpeedLimit)),
				utils.BytesSize(float64(status.UploadSpeed)),
				utils.BytesSize(float64(status.UploadSpeedLimit)),
				utils.BytesSize(float64(status.FreeSpaceOnDisk)),
			)

			if isSingle || showAll {
				torrents, err := clientInstance.GetTorrents("", category, !isSingle && showAll)
				if err != nil {
					log.Printf("Cann't get client %s torrents: %v", name, err)
					hasError = true
					continue
				}
				fmt.Printf("\nName  InfoHash  Tracker  State  ↓S  ↑S  Meta\n")
				for _, torrent := range torrents {
					if filter != "" && !utils.ContainsI(torrent.Name, filter) && !utils.ContainsI(torrent.InfoHash, filter) {
						continue
					}
					fmt.Printf("%s  %s  %s  %s %s/s %s/s %v\n",
						torrent.Name,
						torrent.InfoHash,
						torrent.TrackerDomain,
						client.TorrentStateIconText(torrent.State),
						utils.BytesSize(float64(torrent.DownloadSpeed)),
						utils.BytesSize(float64(torrent.UploadSpeed)),
						torrent.Meta,
					)
				}
				fmt.Printf("\n")
			}
		} else if site.SiteExists(name) {
			siteInstance, err := site.CreateSite(name)
			if err != nil {
				log.Printf("Failed to create site %s: %v", name, err)
				hasError = true
				continue
			}
			meta, err := siteInstance.GetMeta()
			if err != nil {
				log.Printf("Failed to get site %s meta: %v", name, err)
				hasError = true
				continue
			}
			fmt.Printf("Site %s: ↑ %s / ↓ %s\n",
				name,
				utils.BytesSize(float64(meta.UserUploaded)),
				utils.BytesSize(float64(meta.UserDownloaded)),
			)
		} else {
			log.Printf("Error, name %s is not a client or site", name)
			hasError = true
		}
	}
	if hasError {
		os.Exit(1)
	}
}
