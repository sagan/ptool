package status

import (
	"fmt"
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

// @todo : azuretls 有时报错 pin verification failed。特别是在同时访问大量站点时大范围出现
// 部分站点设置 Insecure flag 能够解决？

var (
	filter         = ""
	category       = ""
	dense          = false
	showTorrents   = false
	showFull       = false
	showAll        = false
	showAllClients = false
	showAllSites   = false
	showScore      = false
	largestFlag    = false
	newestFlag     = false
)

var command = &cobra.Command{
	Use: "status [client | site | group]... [-a | -c | -s]",
	// Args:  cobra.MatchAll(cobra.MinimumNArgs(1), cobra.OnlyValidArgs),
	Annotations: map[string]string{"cobra-prompt-dynamic-suggestions": "status"},
	Short:       "Show clients or sites status.",
	Long: `Show clients or sites status.
[client | site | group]: name of a client, site or group.
`,
	RunE: status,
}

func init() {
	command.Flags().StringVarP(&filter, "filter", "", "", constants.HELP_ARG_FILTER_TORRENT)
	command.Flags().StringVarP(&category, "category", "", "", "Filter client torrents by category")
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

	responses := []*StatusResponse{}
	for i := int64(0); i < cnt; i++ {
		responses = append(responses, <-ch)
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

	// type, name, ↑info, ↓info, others
	var format = "%-6s  %-15s  %-27s  %-27s  %-s\n"
	errorsStr := ""
	for _, response := range responses {
		if response.Kind == 1 {
			if response.Error != nil {
				errorsStr += fmt.Sprintf("Error get client %s status: error=%v\n", response.Name, response.Error)
				errorCnt++
			}
			if response.ClientStatus != nil {
				info := fmt.Sprintf("FreeSpace: %s; Unfinished(All/DL): %s/%s",
					util.BytesSizeAround(float64(response.ClientStatus.FreeSpaceOnDisk)),
					util.BytesSizeAround(float64(response.ClientStatus.UnfinishedSize)),
					util.BytesSizeAround(float64(response.ClientStatus.UnfinishedDownloadingSize)),
				)
				if len(response.ClientTorrents) > 0 {
					info += fmt.Sprintf("; Torrents: %d", len(response.ClientTorrents))
				}
				fmt.Printf(format,
					"Client",
					response.Name,
					fmt.Sprintf("↑Spd/Lmt: %s / %s/s", util.BytesSize(float64(response.ClientStatus.UploadSpeed)),
						util.BytesSizeAround(float64(response.ClientStatus.UploadSpeedLimit))),
					fmt.Sprintf("↓Spd/Lmt: %s / %s/s", util.BytesSize(float64(response.ClientStatus.DownloadSpeed)),
						util.BytesSizeAround(float64(response.ClientStatus.DownloadSpeedLimit))),
					info,
				)
			} else {
				fmt.Printf(format,
					"Client",
					response.Name,
					"-",
					"-",
					"// <error>",
				)
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
				client.PrintTorrents(response.ClientTorrents, filter, 0, dense)
				fmt.Printf("\n")
			}
		} else if response.Kind == 2 {
			if response.Error != nil {
				errorsStr += fmt.Sprintf("Error get site %s status: error=%v\n", response.Name, response.Error)
				errorCnt++
			}
			if response.SiteStatus != nil {
				info := fmt.Sprintf("UserName: %s", response.SiteStatus.UserName)
				if len(response.SiteTorrents) > 0 {
					info += fmt.Sprintf("; Torrents: %d", len(response.SiteTorrents))
				}
				fmt.Printf(format,
					"Site",
					response.Name,
					fmt.Sprintf("↑: %s", util.BytesSize(float64(response.SiteStatus.UserUploaded))),
					fmt.Sprintf("↓: %s", util.BytesSize(float64(response.SiteStatus.UserDownloaded))),
					info,
				)
			} else {
				fmt.Printf(format,
					"Site",
					response.Name,
					"-",
					"-",
					"// <error>",
				)
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
				site.PrintTorrents(response.SiteTorrents, filter, now, false, dense, response.SiteTorrentScores)
				fmt.Printf("\n")
			}
		}
	}

	if errorsStr != "" {
		fmt.Printf("\nErrors:\n%s", errorsStr)
	}

	if errorCnt > 0 {
		return fmt.Errorf("%d errors", errorCnt)
	}
	return nil
}
