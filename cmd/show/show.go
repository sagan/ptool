package show

import (
	"fmt"
	"os"
	"sort"
	"strings"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/sagan/ptool/client"
	"github.com/sagan/ptool/cmd"
	"github.com/sagan/ptool/cmd/common"
	"github.com/sagan/ptool/utils"
)

var command = &cobra.Command{
	Use:   "show <client> [<infoHash>...]",
	Short: "Show torrents of client",
	Long: `Show torrents of client
<infoHash>...: infoHash list of torrents. It's possible to use state filter to target multiple torrents:
_all, _active, _done,  _downloading, _seeding, _paused, _completed, _error
If no flags or args are provided, it will display current active torrents
`,
	Args: cobra.MatchAll(cobra.MinimumNArgs(1), cobra.OnlyValidArgs),
	Run:  show,
}

var (
	maxTorrents                                         = int64(0)
	filter                                              = ""
	category                                            = ""
	tag                                                 = ""
	showAll                                             = false
	sortFieldEnumFlag common.ClientTorrentSortFieldEnum = "none"
	orderEnumFlag     common.OrderEnum                  = "asc"
)

func init() {
	command.Flags().Int64VarP(&maxTorrents, "max-results", "m", 0, "Show at most this number of torrents. Default (0) == unlimited")
	command.Flags().BoolVarP(&showAll, "all", "a", false, "Show all torrents. Equavalent with pass a '_all' arg")
	command.Flags().StringVarP(&filter, "filter", "f", "", "filter torrents by name")
	command.Flags().StringVarP(&category, "category", "c", "", "filter torrents by category")
	command.Flags().StringVarP(&tag, "tag", "t", "", "filter torrents by tag")
	command.Flags().VarP(&sortFieldEnumFlag, "sort", "s", "Manually Set the sort field, "+common.ClientTorrentSortFieldEnumTip)
	command.Flags().VarP(&orderEnumFlag, "order", "o", "Manually Set the sort order, "+common.OrderEnumTip)
	command.RegisterFlagCompletionFunc("sort", common.ClientTorrentSortFieldEnumCompletion)
	command.RegisterFlagCompletionFunc("order", common.OrderEnumCompletion)
	cmd.RootCmd.AddCommand(command)
}

func show(cmd *cobra.Command, args []string) {
	clientInstance, err := client.CreateClient(args[0])
	if err != nil {
		log.Fatal(err)
	}
	args = args[1:]

	desc := false
	if orderEnumFlag == "desc" {
		desc = true
	}

	var torrents []client.Torrent
	if showAll {
		torrents, err = client.QueryTorrents(clientInstance, "", "", "")
	} else if category == "" && tag == "" && filter == "" && len(args) == 0 {
		torrents, err = client.QueryTorrents(clientInstance, "", "", "", "_active")
	} else if category == "" && tag == "" && filter == "" &&
		len(args) == 1 && !strings.HasPrefix(args[0], "_") {
		// display single torrent details
		torrent, err := clientInstance.GetTorrent(args[0])
		if err != nil {
			log.Fatalf("Failed to get torrent %s details: %v", err)
		}
		client.PrintTorrent(torrent)
		os.Exit(0)
	} else {
		torrents, err = client.QueryTorrents(clientInstance, category, tag, filter, args...)
	}
	if err != nil {
		log.Fatalf("Failed to fetch client torrents: %v", err)
	}
	if sortFieldEnumFlag != "" && sortFieldEnumFlag != "none" {
		sort.Slice(torrents, func(i, j int) bool {
			switch sortFieldEnumFlag {
			case "name":
				return torrents[i].Name < torrents[j].Name
			case "size":
				return torrents[i].Size < torrents[j].Size
			case "speed":
				return torrents[i].DownloadSpeed+torrents[i].UploadSpeed <
					torrents[j].DownloadSpeed+torrents[j].UploadSpeed
			case "state":
				if torrents[i].State != torrents[j].State {
					return torrents[i].State < torrents[j].State
				}
				return torrents[i].LowLevelState < torrents[j].LowLevelState
			case "time":
				return torrents[i].Atime < torrents[j].Atime
			case "tracker":
				if torrents[i].TrackerDomain != torrents[j].TrackerDomain {
					return torrents[i].TrackerDomain < torrents[j].TrackerDomain
				}
				return torrents[i].Atime < torrents[j].Atime
			}
			return i < j
		})
		if desc {
			for i, j := 0, len(torrents)-1; i < j; i, j = i+1, j-1 {
				torrents[i], torrents[j] = torrents[j], torrents[i]
			}
		}
	}
	if maxTorrents > 0 && len(torrents) > int(maxTorrents) {
		torrents = torrents[:maxTorrents]
	}

	clientStatus, err := clientInstance.GetStatus()
	if err != nil {
		log.Errorf("Failed to get client status: %v", err)
		fmt.Printf("Client %s | Showing %d torrents\n\n", clientInstance.GetName(), len(torrents))
	} else {
		fmt.Printf("Client %s | %s | %s | %s | Showing %d torrents\n\n",
			clientInstance.GetName(),
			fmt.Sprintf("↑Spd/Lmt: %s / %s/s", utils.BytesSize(float64(clientStatus.UploadSpeed)),
				utils.BytesSize(float64(clientStatus.UploadSpeedLimit))),
			fmt.Sprintf("↓Spd/Lmt: %s / %s/s", utils.BytesSize(float64(clientStatus.DownloadSpeed)),
				utils.BytesSize(float64(clientStatus.DownloadSpeedLimit))),
			fmt.Sprintf("FreeSpace: %s", utils.BytesSize(float64(clientStatus.FreeSpaceOnDisk))),
			len(torrents),
		)
	}

	client.PrintTorrents(torrents, "")
}
