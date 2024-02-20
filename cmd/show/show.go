package show

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/sagan/ptool/client"
	"github.com/sagan/ptool/cmd"
	"github.com/sagan/ptool/cmd/common"
	"github.com/sagan/ptool/util"
)

var command = &cobra.Command{
	Use:         "show {client} [--category category] [--tag tag] [--filter filter] [infoHash]...",
	Annotations: map[string]string{"cobra-prompt-dynamic-suggestions": "show"},
	Short:       "Show torrents of client.",
	Long: `Show torrents of client.
[infoHash]...: infoHash list of torrents. It's possible to use state filter to target multiple torrents:
_all, _active, _done, _undone, _downloading, _seeding, _paused, _completed, _error.
If no flags or args are provided, it will display current active torrents.`,
	Args: cobra.MatchAll(cobra.MinimumNArgs(1), cobra.OnlyValidArgs),
	RunE: show,
}

var (
	largestFlag       bool
	showTrackers      bool
	showFiles         bool
	showInfoHashOnly  bool
	maxTorrents       = int64(0)
	filter            = ""
	category          = ""
	tag               = ""
	tracker           = ""
	minTorrentSizeStr = ""
	maxTorrentSizeStr = ""
	dense             = false
	showAll           = false
	showRaw           = false
	showJson          = false
	showSum           = false
	sortFlag          string
	orderFlag         string
)

func init() {
	command.Flags().Int64VarP(&maxTorrents, "max-torrents", "", -1,
		"Show at most this number of torrents. -1 == no limit")
	command.Flags().BoolVarP(&dense, "dense", "", false, "Dense mode: show full torrent title & subtitle")
	command.Flags().BoolVarP(&largestFlag, "largest", "l", false,
		"Show largest torrents first. Equavalent with '--sort size --order desc'")
	command.Flags().BoolVarP(&showAll, "all", "a", false, "Show all torrents. Equavalent with pass a '_all' arg")
	command.Flags().BoolVarP(&showRaw, "raw", "", false, "Show torrent size in raw format")
	command.Flags().BoolVarP(&showJson, "json", "", false, "Show output in json format")
	command.Flags().BoolVarP(&showInfoHashOnly, "show-info-hash-only", "", false, "Output torrents info hash only")
	command.Flags().BoolVarP(&showSum, "sum", "", false, "Show torrents summary only")
	command.Flags().BoolVarP(&showTrackers, "show-trackers", "", false, "Show torrent trackers info")
	command.Flags().BoolVarP(&showFiles, "show-files", "", false, "Show torrent content files info")
	command.Flags().StringVarP(&filter, "filter", "", "", "Filter torrents by name")
	command.Flags().StringVarP(&category, "category", "", "", "Filter torrents by category")
	command.Flags().StringVarP(&tag, "tag", "", "",
		"Filter torrents by tag. Comma-separated string list. Torrent which tags contain any one in the list match")
	command.Flags().StringVarP(&tracker, "tracker", "", "", "Filter torrents by tracker domain")
	command.Flags().StringVarP(&minTorrentSizeStr, "min-torrent-size", "", "-1",
		"Skip torrent with size smaller than (<) this value. -1 == no limit")
	command.Flags().StringVarP(&maxTorrentSizeStr, "max-torrent-size", "", "-1",
		"Skip torrent with size larger than (>) this value. -1 == no limit")
	cmd.AddEnumFlagP(command, &sortFlag, "sort", "", common.ClientTorrentSortFlag)
	cmd.AddEnumFlagP(command, &orderFlag, "order", "", common.OrderFlag)
	cmd.RootCmd.AddCommand(command)
}

func show(cmd *cobra.Command, args []string) error {
	clientName := args[0]
	infoHashes := args[1:]
	if showJson && showInfoHashOnly {
		return fmt.Errorf("--json and --info-hash flags are NOT compatible")
	}
	if showInfoHashOnly && (showFiles || showTrackers) {
		return fmt.Errorf("--show-files or --show-trackers is NOT compatible with --show-info-hash-only flag")
	}
	clientInstance, err := client.CreateClient(clientName)
	if err != nil {
		return fmt.Errorf("failed to create client: %v", err)
	}
	desc := false
	if largestFlag {
		sortFlag = "size"
		desc = true
	}
	if orderFlag == "desc" {
		desc = true
	}
	minTorrentSize, _ := util.RAMInBytes(minTorrentSizeStr)
	maxTorrentSize, _ := util.RAMInBytes(maxTorrentSizeStr)

	noConditionFlags := category == "" && tag == "" && filter == "" && tracker == "" &&
		minTorrentSize < 0 && maxTorrentSize < 0
	var torrents []client.Torrent
	if showAll {
		torrents, err = client.QueryTorrents(clientInstance, "", "", "")
	} else if noConditionFlags && len(infoHashes) == 0 {
		torrents, err = client.QueryTorrents(clientInstance, "", "", "", "_active")
	} else if noConditionFlags && len(infoHashes) == 1 && !strings.HasPrefix(infoHashes[0], "_") {
		// display single torrent details
		if !client.IsValidInfoHash(infoHashes[0]) {
			return fmt.Errorf("%s is not a valid infoHash", infoHashes[0])
		}
		torrent, err := clientInstance.GetTorrent(infoHashes[0])
		if err != nil {
			return fmt.Errorf("failed to get torrent %s details: %v", infoHashes[0], err)
		}
		if torrent == nil {
			return fmt.Errorf("torrent %s not found", infoHashes[0])
		}
		if showJson {
			bytes, err := json.Marshal(torrent)
			if err != nil {
				return fmt.Errorf("failed to marshal json: %v", err)
			}
			fmt.Println(string(bytes))
			return nil
		}
		client.PrintTorrent(torrent)
		if showTrackers {
			fmt.Printf("\n")
			trackers, err := clientInstance.GetTorrentTrackers(infoHashes[0])
			if err != nil {
				log.Errorf("Failed to get torrent trackers: %v", err)
			} else {
				client.PrintTorrentTrackers(trackers)
			}
		}
		if showFiles {
			fmt.Printf("\n")
			files, err := clientInstance.GetTorrentContents(infoHashes[0])
			if err != nil {
				log.Errorf("Failed to get torrent contents: %v", err)
			} else {
				client.PrintTorrentFiles(files, showRaw)
			}
		}
		return nil
	} else {
		torrents, err = client.QueryTorrents(clientInstance, category, tag, filter, infoHashes...)
	}
	if err != nil {
		return fmt.Errorf("failed to fetch client torrents: %v", err)
	}
	if tracker != "" || minTorrentSize >= 0 || maxTorrentSize >= 0 {
		torrents = util.Filter(torrents, func(t client.Torrent) bool {
			if tracker != "" && t.TrackerDomain != tracker ||
				minTorrentSize >= 0 && t.Size < minTorrentSize ||
				maxTorrentSize >= 0 && t.Size > maxTorrentSize {
				return false
			}
			return true
		})
	}
	if sortFlag != "" && sortFlag != "none" {
		sort.Slice(torrents, func(i, j int) bool {
			switch sortFlag {
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
	if maxTorrents >= 0 && len(torrents) > int(maxTorrents) {
		torrents = torrents[:maxTorrents]
	}

	if !showInfoHashOnly {
		if showJson {
			bytes, err := json.Marshal(torrents)
			if err != nil {
				return fmt.Errorf("failed to marshal json: %v", err)
			}
			fmt.Println(string(bytes))
			return nil
		}
		clientStatus, err := clientInstance.GetStatus()
		if err != nil {
			log.Errorf("Failed to get client status: %v", err)
			fmt.Printf("Client %s | Showing %d torrents\n\n", clientInstance.GetName(), len(torrents))
		} else {
			fmt.Printf("Client %s | %s | %s | %s | Showing %d torrents\n\n",
				clientInstance.GetName(),
				fmt.Sprintf("↑Spd/Lmt: %s / %s/s", util.BytesSize(float64(clientStatus.UploadSpeed)),
					util.BytesSize(float64(clientStatus.UploadSpeedLimit))),
				fmt.Sprintf("↓Spd/Lmt: %s / %s/s", util.BytesSize(float64(clientStatus.DownloadSpeed)),
					util.BytesSize(float64(clientStatus.DownloadSpeedLimit))),
				fmt.Sprintf("FreeSpace: %s", util.BytesSize(float64(clientStatus.FreeSpaceOnDisk))),
				len(torrents),
			)
		}
		showSummary := int64(1)
		if showSum {
			showSummary = 2
		}
		client.PrintTorrents(torrents, "", showSummary, dense)
	} else {
		for i, torrent := range torrents {
			if i > 0 {
				fmt.Printf(" ")
			}
			fmt.Printf(torrent.InfoHash)
		}
	}
	return nil
}
