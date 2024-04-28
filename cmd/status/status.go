package status

import (
	"fmt"
	"os"
	"slices"
	"sort"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/sagan/ptool/client"
	"github.com/sagan/ptool/cmd"
	"github.com/sagan/ptool/config"
	"github.com/sagan/ptool/constants"
	"github.com/sagan/ptool/site"
	"github.com/sagan/ptool/util"
)

var (
	dataOrder      = false
	dense          = false
	showTorrents   = false
	showFull       = false
	showAll        = false
	showAllClients = false
	showAllSites   = false
	showScore      = false
	largestFlag    = false
	newestFlag     = false
	filter         = ""
	category       = ""
)

var command = &cobra.Command{
	Use: "status [client | site | group]... [-a | -c | -s]",
	// Args:  cobra.MatchAll(cobra.MinimumNArgs(1), cobra.OnlyValidArgs),
	Annotations: map[string]string{"cobra-prompt-dynamic-suggestions": "status"},
	Short:       "Show clients or sites status.",
	Long: `Show clients or sites status.
[client | site | group]: name of a client, site or group.

For client, display following status info:
- ↑Spd/Lmt : Current uploading speed / limit.
- ↓Spd/Lmt : Current downloading speed / limit.
- FreeSpace : Remaining free disk space of default save path (download folder).
- UnfinishedAll : Total size of un-downloaded parts of unfinished (incomplete) torrents.
- UnfinishedDL : Total size of un-downloaded parts of unfinished (incomplete) torrents, excluding paused ones.

For site, display following status info:
- ↑: : Current uploading statistics.
- ↓: : Current downloading statstics.

If "-t" flag is set, it will also show the active / latest torrents list of client / site.
For the list format of client torrents, see help of "ptool show" command.
For the list format of site torrents, see help of "ptool search" command.`,
	RunE: status,
}

func init() {
	command.Flags().BoolVarP(&dataOrder, "data-order", "", false,
		"Sort results of clients or sites by uploading speed or amount in desc order")
	command.Flags().BoolVarP(&dense, "dense", "d", false, "Dense mode: show full torrent title & subtitle")
	command.Flags().BoolVarP(&showAll, "all", "a", false, "Show all clients and sites")
	command.Flags().BoolVarP(&showAllClients, "clients", "c", false, "Show all clients")
	command.Flags().BoolVarP(&showAllSites, "sites", "s", false, "Show all sites")
	command.Flags().BoolVarP(&showTorrents, "torrents", "t", false,
		"Show torrents (active torrents for client / latest torrents for site)")
	command.Flags().BoolVarP(&showFull, "full", "f", false, "Show full info of each client or site")
	command.Flags().BoolVarP(&showScore, "score", "", false, "Show brush score of site torrents")
	command.Flags().BoolVarP(&largestFlag, "largest", "l", false, `Sort torrents by size in desc order"`)
	command.Flags().BoolVarP(&newestFlag, "newest", "n", false, `Sort torrents by time in desc order"`)
	command.Flags().StringVarP(&filter, "filter", "", "", constants.HELP_ARG_FILTER_TORRENT)
	command.Flags().StringVarP(&category, "category", "", "", "Filter client torrents by category")
	cmd.RootCmd.AddCommand(command)
}

func status(cmd *cobra.Command, args []string) error {
	names := args
	if largestFlag && newestFlag {
		return fmt.Errorf("--largest and --newest flags are NOT compatible")
	}
	if showAll || showAllClients || showAllSites {
		if len(args) > 0 {
			return fmt.Errorf("--all, --clients, --sites flags cann't be used with site or client names")
		}
		if showAll || showAllClients {
			for _, client := range config.Get().ClientsEnabled {
				names = append(names, client.Name)
			}
		}
		if showAll || showAllSites {
			for _, site := range config.Get().SitesEnabled {
				if site.Dead || site.Hidden {
					continue
				}
				names = append(names, site.GetName())
			}
		}
	}
	names = config.ParseGroupAndOtherNames(names...)

	if len(names) == 0 {
		return fmt.Errorf("no sites or clients provided")
	}
	now := util.Now()
	errorCnt := int64(0)
	doneFlag := map[string]bool{}
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
				errorCnt++
				continue
			}
			go fetchClientStatus(clientInstance, showTorrents, showFull, category, ch)
			cnt++
		} else if site.GetConfigSiteReginfo(name) != nil {
			siteInstance, err := site.CreateSite(name)
			if err != nil {
				log.Errorf("Error: failed to create site %s: %v\n", name, err)
				errorCnt++
				continue
			}
			go fetchSiteStatus(siteInstance, showTorrents, showFull, showScore, ch)
			cnt++
		} else {
			log.Errorf("Error: %s is not a client or site\n", name)
			errorCnt++
		}
	}

	cntClients := int64(0)
	cntSites := int64(0)
	cntSuccessClients := int64(0)
	cntSuccessSites := int64(0)
	successClientsUploading := int64(0)
	successClientsDownloading := int64(0)
	successSitesUploaded := int64(0)
	successSitesDownloaded := int64(0)

	responses := []*StatusResponse{}
	for i := int64(0); i < cnt; i++ {
		responses = append(responses, <-ch)
	}
	if dataOrder {
		sort.SliceStable(responses, func(i, j int) bool {
			if responses[i].Kind != responses[j].Kind {
				return responses[i].Kind < responses[j].Kind
			}
			if responses[i].Kind == 1 {
				if responses[i].ClientStatus != nil {
					if responses[j].ClientStatus == nil {
						return true
					} else {
						return responses[i].ClientStatus.UploadSpeed > responses[j].ClientStatus.UploadSpeed
					}
				}
			} else if responses[i].Kind == 2 {
				if responses[i].SiteStatus != nil {
					if responses[j].SiteStatus == nil {
						return true
					} else {
						return responses[i].SiteStatus.UserUploaded > responses[j].SiteStatus.UserUploaded
					}
				}
			}
			return false
		})
	} else {
		sort.SliceStable(responses, func(i, j int) bool {
			indexA := slices.IndexFunc(names, func(name string) bool {
				return responses[i].Name == name
			})
			indexB := slices.IndexFunc(names, func(name string) bool {
				return responses[j].Name == name
			})
			return indexA < indexB
		})
	}

	errorsStr := ""
	for _, response := range responses {
		if response.Kind == 1 {
			cntClients++
			if response.Error != nil {
				errorsStr += fmt.Sprintf("Error get client %s status: error=%v\n", response.Name, response.Error)
				errorCnt++
			}
			if response.ClientStatus != nil {
				cntSuccessClients++
				successClientsUploading += response.ClientStatus.UploadSpeed
				successClientsDownloading += response.ClientStatus.DownloadSpeed
				additionalInfo := ""
				if len(response.ClientTorrents) > 0 {
					additionalInfo = fmt.Sprintf("Torrents: %d", len(response.ClientTorrents))
				}
				response.ClientStatus.Print(os.Stdout, response.Name, additionalInfo)
			} else {
				client.PrintDummyStatus(os.Stdout, response.Name, "<error>")
			}
			if response.ClientTorrents != nil {
				fmt.Printf("\n")
				if largestFlag {
					sort.Slice(response.ClientTorrents, func(i, j int) bool {
						return response.ClientTorrents[i].Size > response.ClientTorrents[j].Size
					})
				} else if newestFlag {
					sort.Slice(response.ClientTorrents, func(i, j int) bool {
						return response.ClientTorrents[i].Atime > response.ClientTorrents[j].Atime
					})
				}
				client.PrintTorrents(os.Stdout, response.ClientTorrents, filter, 0, dense)
				fmt.Printf("\n")
			}
		} else if response.Kind == 2 {
			cntSites++
			if response.Error != nil {
				errorsStr += fmt.Sprintf("Error get site %s status: error=%v\n", response.Name, response.Error)
				errorCnt++
			}
			if response.SiteStatus != nil {
				cntSuccessSites++
				successSitesUploaded += response.SiteStatus.UserUploaded
				successSitesDownloaded += response.SiteStatus.UserDownloaded
				additionalInfo := fmt.Sprintf("UserName: %s", response.SiteStatus.UserName)
				if len(response.SiteTorrents) > 0 {
					additionalInfo += fmt.Sprintf("; Torrents: %d", len(response.SiteTorrents))
				}
				response.SiteStatus.Print(os.Stdout, response.Name, additionalInfo)
			} else {
				site.PrintDummyStatus(os.Stdout, response.Name, "<error>")
			}
			if response.SiteTorrents != nil {
				fmt.Printf("\n")
				if largestFlag {
					sort.Slice(response.SiteTorrents, func(i, j int) bool {
						return response.SiteTorrents[i].Size > response.SiteTorrents[j].Size
					})
				} else if newestFlag {
					sort.Slice(response.SiteTorrents, func(i, j int) bool {
						return response.SiteTorrents[i].Time > response.SiteTorrents[j].Time
					})
				}
				site.PrintTorrents(os.Stdout, response.SiteTorrents, filter, now, false, dense, response.SiteTorrentScores)
				fmt.Printf("\n")
			}
		}
	}

	if cntClients+cntSites > 1 {
		fmt.Printf("\n")
		fmt.Printf("// Summary: %d clients, %d sites\n", cntClients, cntSites)
		fmt.Printf("// Success clients: %d, total  ↑Spd / ↓Spd: %s / %s/s\n", cntSuccessClients,
			util.BytesSizeAround(float64(successClientsUploading)),
			util.BytesSizeAround(float64(successClientsDownloading)))
		fmt.Printf("// Failed clients: %d\n", cntClients-cntSuccessClients)
		fmt.Printf("// Success sites: %d, total ↑ / ↓: %s / %s\n", cntSuccessSites,
			util.BytesSizeAround(float64(successSitesUploaded)),
			util.BytesSizeAround(float64(successSitesDownloaded)))
		fmt.Printf("// Failed sites: %d\n", cntSites-cntSuccessSites)
	}

	if errorsStr != "" {
		fmt.Printf("\nErrors:\n%s", errorsStr)
	}

	if errorCnt > 0 {
		return fmt.Errorf("%d errors", errorCnt)
	}
	return nil
}
