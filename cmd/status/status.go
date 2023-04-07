package status

import (
	"fmt"
	"os"
	"sort"

	log "github.com/sirupsen/logrus"

	"github.com/sagan/ptool/client"
	"github.com/sagan/ptool/cmd"
	"github.com/sagan/ptool/config"
	"github.com/sagan/ptool/site"
	"github.com/sagan/ptool/utils"
	"github.com/spf13/cobra"

	"golang.org/x/exp/slices"
)

var (
	filter         = ""
	category       = ""
	showAll        = false
	showFull       = false
	showAllClients = false
	showAllSites   = false
)

var command = &cobra.Command{
	Use: "status ...clientOrSites",
	// Args:  cobra.MatchAll(cobra.MinimumNArgs(1), cobra.OnlyValidArgs),
	Short: "Show clients or site status",
	Long:  `A longer description`,
	Run:   status,
}

func init() {
	command.Flags().StringVar(&filter, "filter", "", "filter client torrents by name")
	command.Flags().StringVar(&category, "category", "", "filter client torrents by category")
	command.Flags().BoolVar(&showAll, "all", false, "show all clients / sites.")
	command.Flags().BoolVar(&showAllClients, "clients", false, "show all clients.")
	command.Flags().BoolVar(&showAllSites, "sites", false, "show all sites.")
	command.Flags().BoolVar(&showFull, "full", false, "show full info of each client / site")
	cmd.RootCmd.AddCommand(command)
}

func status(cmd *cobra.Command, args []string) {
	names := args
	if showAll || showAllClients || showAllSites {
		if len(args) > 0 {
			log.Fatal("Illegal args: --all, --clients, --sites cann't be used with site or client names")
		}
		if showAll || showAllClients {
			for _, client := range config.Get().Clients {
				names = append(names, client.Name)
			}
		}
		if showAll || showAllSites {
			for _, site := range config.Get().Sites {
				sitename := site.Name
				if sitename == "" {
					sitename = site.Type
				}
				names = append(names, sitename)
			}
		}
	}
	if len(names) == 0 {
		log.Fatal("Usage: status ...clientOrSites")
	}
	isSingle := len(names) == 1
	hasError := false
	doneFlag := make(map[string](bool))
	cnt := int64(0)
	ch := make(chan *StatusResponse, len(names))
	for _, name := range names {
		if name == "_" || doneFlag[name] {
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
			go fetchClientStatus(clientInstance, isSingle || showFull, isSingle && showFull, category, ch)
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

	errorsStr := ""
	for i, response := range responses {
		if response.Kind == 1 {
			if response.Error != nil {
				errorsStr += fmt.Sprintf("Error get client %s status: error=%v\n", response.Name, response.Error)
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
			} else {
				fmt.Printf("Client %s failed to get status\n", response.Name)
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
				if i != len(responses)-1 {
					fmt.Printf("\n")
				}
			}
		} else if response.Kind == 2 {
			if response.Error != nil {
				errorsStr += fmt.Sprintf("Error get site %s status: error=%v\n", response.Name, response.Error)
				hasError = true
			}
			if response.SiteStatus != nil {
				fmt.Printf("Site %s: %s ↑ %s / ↓ %s\n",
					response.Name,
					response.SiteStatus.UserName,
					utils.BytesSize(float64(response.SiteStatus.UserUploaded)),
					utils.BytesSize(float64(response.SiteStatus.UserDownloaded)),
				)
			} else {
				fmt.Printf("Site %s: failed to get status", response.Name)
			}
		}
	}

	if errorsStr != "" {
		log.Printf("\nErrors:\n%s", errorsStr)
	}

	if hasError {
		os.Exit(1)
	}
}
