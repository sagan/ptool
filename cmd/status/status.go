package status

import (
	"fmt"
	"log"
	"os"
	"sort"

	"github.com/sagan/ptool/client"
	"github.com/sagan/ptool/cmd"
	"github.com/sagan/ptool/site"
	"github.com/sagan/ptool/utils"
	"github.com/spf13/cobra"

	"golang.org/x/exp/slices"
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
	doneFlag := make(map[string](bool))
	cnt := int64(0)
	ch := make(chan *StatusResponse, len(names))
	for _, name := range names {
		if doneFlag[name] {
			continue
		}
		doneFlag[name] = true
		if client.ClientExists(name) {
			clientInstance, err := client.CreateClient(name)
			if err != nil {
				log.Printf("Failed to create client %s: %v", name, err)
				hasError = true
				continue
			}
			go fetchClientStatus(clientInstance, !isSingle && showAll, category, ch)
			cnt++
		} else if site.SiteExists(name) {
			siteInstance, err := site.CreateSite(name)
			if err != nil {
				log.Printf("Failed to create site %s: %v", name, err)
				hasError = true
				continue
			}
			go fetchSiteStatus(siteInstance, ch)
			cnt++
		} else {
			log.Printf("Error, name %s is not a client or site", name)
			hasError = true
		}
	}

	responses := make([]*StatusResponse, cnt)
	for i := int64(0); i < cnt; i++ {
		responses[i] = <-ch
	}
	sort.SliceStable(responses, func(i, j int) bool {
		indexA := slices.IndexFunc(names, func(name string) bool {
			return responses[i].Name == name
		})
		indexB := slices.IndexFunc(names, func(name string) bool {
			return responses[j].Name == name
		})
		return indexA < indexB
	})

	for _, response := range responses {
		if response.Kind == 1 {
			if response.Error != nil {
				log.Printf("Error get client %s status: error=%v", response.Name, response.Error)
				hasError = true
			}
			if response.ClientStatus != nil {
				fmt.Printf("Client %s ↓ Speed/Limit: %s/s / %s/s; ↑ Speed/Limit: %s/s / %s/s; Free disk space: %s\n",
					response.Name,
					utils.BytesSize(float64(response.ClientStatus.DownloadSpeed)),
					utils.BytesSize(float64(response.ClientStatus.DownloadSpeedLimit)),
					utils.BytesSize(float64(response.ClientStatus.UploadSpeed)),
					utils.BytesSize(float64(response.ClientStatus.UploadSpeedLimit)),
					utils.BytesSize(float64(response.ClientStatus.FreeSpaceOnDisk)),
				)
			}
			if response.ClientTorrents != nil {
				fmt.Printf("\nName  InfoHash  Tracker  State  ↓S  ↑S  Meta\n")
				for _, torrent := range response.ClientTorrents {
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
		} else if response.Kind == 2 {
			if response.Error != nil {
				log.Printf("Error get site %s status: error=%v", response.Name, response.Error)
				hasError = true
			}
			if response.SiteStatus != nil {
				fmt.Printf("Site %s: %s ↑ %s / ↓ %s\n",
					response.Name,
					response.SiteStatus.UserName,
					utils.BytesSize(float64(response.SiteStatus.UserUploaded)),
					utils.BytesSize(float64(response.SiteStatus.UserDownloaded)),
				)
			}
		}
	}

	if hasError {
		os.Exit(1)
	}
}
