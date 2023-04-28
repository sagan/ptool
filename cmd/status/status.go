package status

import (
	"fmt"
	"os"
	"sort"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"golang.org/x/exp/slices"

	"github.com/sagan/ptool/client"
	"github.com/sagan/ptool/cmd"
	"github.com/sagan/ptool/config"
	"github.com/sagan/ptool/site"
	"github.com/sagan/ptool/utils"
)

var (
	filter         = ""
	category       = ""
	showTorrents   = false
	showFull       = false
	showAll        = false
	showAllClients = false
	showAllSites   = false
)

var command = &cobra.Command{
	Use: "status <clientOrSites>...",
	// Args:  cobra.MatchAll(cobra.MinimumNArgs(1), cobra.OnlyValidArgs),
	Short: "Show clients or sites status",
	Long:  `Show clients or sites status`,
	Run:   status,
}

func init() {
	command.Flags().StringVar(&filter, "filter", "", "filter client torrents by name")
	command.Flags().StringVar(&category, "category", "", "filter client torrents by category")
	command.Flags().BoolVarP(&showAll, "all", "a", false, "show all clients / sites.")
	command.Flags().BoolVarP(&showAllClients, "clients", "c", false, "show all clients.")
	command.Flags().BoolVarP(&showAllSites, "sites", "s", false, "show all sites.")
	command.Flags().BoolVarP(&showTorrents, "torrents", "t", false, "show torrents (active torrents for client / latest torrents for site).")
	command.Flags().BoolVarP(&showFull, "full", "f", false, "show full info of each client / site")
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
				names = append(names, site.Name)
			}
		}
	}
	if len(names) == 0 {
		log.Fatal("Usage: status ...clientOrSites")
	}
	now := utils.Now()
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
				log.Errorf("Error: failed to create client %s: %v\n", name, err)
				hasError = true
				continue
			}
			go fetchClientStatus(clientInstance, showTorrents, showFull, category, ch)
			cnt++
		} else if site.SiteExists(name) {
			siteInstance, err := site.CreateSite(name)
			if err != nil {
				log.Errorf("Error: failed to create site %s: %v\n", name, err)
				hasError = true
				continue
			}
			go fetchSiteStatus(siteInstance, showTorrents, showFull, ch)
			cnt++
		} else {
			log.Errorf("Error: %s is not a client or site\n", name)
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
				fmt.Printf("Client %s ↓ Speed/Limit: %s/s / %s/s; ↑ Speed/Limit: %s/s / %s/s; Free disk space: %s",
					response.Name,
					utils.BytesSize(float64(response.ClientStatus.DownloadSpeed)),
					utils.BytesSize(float64(response.ClientStatus.DownloadSpeedLimit)),
					utils.BytesSize(float64(response.ClientStatus.UploadSpeed)),
					utils.BytesSize(float64(response.ClientStatus.UploadSpeedLimit)),
					utils.BytesSize(float64(response.ClientStatus.FreeSpaceOnDisk)),
				)
				if len(response.ClientTorrents) > 0 {
					fmt.Printf(" / Torrents: %d", len(response.ClientTorrents))
				}
				fmt.Printf("\n")
			} else {
				fmt.Printf("Client %s: failed to get status\n", response.Name)
			}
			if response.ClientTorrents != nil {
				client.PrintTorrents(response.ClientTorrents, filter)
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
				fmt.Printf("Site %s: %s ↑ %s / ↓ %s",
					response.Name,
					response.SiteStatus.UserName,
					utils.BytesSize(float64(response.SiteStatus.UserUploaded)),
					utils.BytesSize(float64(response.SiteStatus.UserDownloaded)),
				)
				if len(response.SiteTorrents) > 0 {
					fmt.Printf(" / Torrents: %d", len(response.SiteTorrents))
				}
				fmt.Printf("\n")
			} else {
				fmt.Printf("Site %s: failed to get status\n", response.Name)
			}
			if response.SiteTorrents != nil {
				site.PrintTorrents(response.SiteTorrents, filter, now)
				if i != len(responses)-1 {
					fmt.Printf("\n")
				}
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
