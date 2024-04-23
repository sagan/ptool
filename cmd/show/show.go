package show

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/sagan/ptool/client"
	"github.com/sagan/ptool/cmd"
	"github.com/sagan/ptool/cmd/common"
	"github.com/sagan/ptool/constants"
	"github.com/sagan/ptool/util"
)

var command = &cobra.Command{
	Use:         "show {client} [--category category] [--tag tag] [--filter filter] [infoHash]...",
	Annotations: map[string]string{"cobra-prompt-dynamic-suggestions": "show"},
	Short:       "Show torrents of client.",
	Long: `Show torrents of client.
[infoHash]...: infoHash list of torrents. It's possible to use state filter to target multiple torrents:
_all, _active, _done, _undone, _downloading, _seeding, _paused, _completed, _error.
If no condition flags or args are set, it will display current active torrents.
If at least one condition flag is set but no arg is provided, the args is assumed to be "_all"

It displays found torrents of client in the list, which has several fields like "Name" and "State".

The "Name" field by default displays the truncated prefix of the torrent name in client.
If "--dense" flag is set, it will instead display the full name of the torrent,
as well as it's category & tags & content path infos.

The "State" field displays some icon texts:
* ✓ : Torrent is downloaded completely (finished).
* - : Torrent is paused and incomplete (unfinished).
* ↓ : Torrent is in downloading state.
* ↑ : Torrent is in seeding (uploading) state.
* 80% (e.g.) : The current download progress of the torrent.
* ! : Torrent is in error state.
* ? : Torrent state is unknown.
* _ : Torrent contents files are partially selected for downloading.

Specially, if all args is an (1) single info-hash, it displays the details of that torrent instead of the list.`,
	Args: cobra.MatchAll(cobra.MinimumNArgs(1), cobra.OnlyValidArgs),
	RunE: show,
}

var (
	largestFlag        bool
	newestFlag         bool
	showTrackers       bool
	showFiles          bool
	showInfoHashOnly   bool
	partial            bool
	maxTorrents        = int64(0)
	addedInStr         = ""
	completedBeforeStr = ""
	activeSinceStr     = ""
	notActiveSinceStr  = ""
	filter             = ""
	category           = ""
	tag                = ""
	tracker            = ""
	savePath           = ""
	minTorrentSizeStr  = ""
	maxTorrentSizeStr  = ""
	maxTotalSizeStr    = ""
	dense              = false
	showAll            = false
	showRaw            = false
	showJson           = false
	showSum            = false
	sortFlag           string
	orderFlag          string
)

func init() {
	command.Flags().Int64VarP(&maxTorrents, "max-torrents", "", -1,
		"Show at most this number of torrents. -1 == no limit")
	command.Flags().BoolVarP(&dense, "dense", "d", false, "Dense mode: show full torrent title & subtitle")
	command.Flags().BoolVarP(&largestFlag, "largest", "l", false,
		`Show largest torrents first. Equivalent to "--sort size --order desc"`)
	command.Flags().BoolVarP(&newestFlag, "newest", "n", false,
		`Show newest torrents first. Equivalent to "--sort time --order desc"`)
	command.Flags().BoolVarP(&showAll, "all", "a", false, `Show all torrents. Equivalent to passing a "_all" arg`)
	command.Flags().BoolVarP(&showRaw, "raw", "", false, "Show torrent size in raw format")
	command.Flags().BoolVarP(&showJson, "json", "", false, "Show output in json format")
	command.Flags().BoolVarP(&showInfoHashOnly, "show-info-hash-only", "", false, "Output torrents info hash only")
	command.Flags().BoolVarP(&showSum, "sum", "", false, "Show torrents summary only")
	command.Flags().BoolVarP(&showTrackers, "show-trackers", "", false, "Show torrent trackers info")
	command.Flags().BoolVarP(&showFiles, "show-files", "", false, "Show torrent content files info")
	command.Flags().BoolVarP(&partial, "partial", "", false,
		"Only showing torrents that are partially selected for downloading")
	command.Flags().StringVarP(&maxTotalSizeStr, "max-total-size", "", "-1",
		"Show at most torrents with total contents size of this value. -1 == no limit")
	command.Flags().StringVarP(&addedInStr, "added-in", "", "",
		`Time duration. Only showing torrent that was added to client in the past time of this value. E.g. "5d"`)
	command.Flags().StringVarP(&completedBeforeStr, "completed-before", "", "",
		`Only showing torrent that was downloaded completed before this. `+constants.HELP_ARG_TIMES)
	command.Flags().StringVarP(&activeSinceStr, "active-since", "", "",
		`Only showing torrent that has activity since this. `+constants.HELP_ARG_TIMES)
	command.Flags().StringVarP(&notActiveSinceStr, "not-active-since", "", "",
		`Only showing torrent that does NOT has activity since this. `+constants.HELP_ARG_TIMES)
	command.Flags().StringVarP(&filter, "filter", "", "", constants.HELP_ARG_FILTER_TORRENT)
	command.Flags().StringVarP(&category, "category", "", "", constants.HELP_ARG_CATEGORY)
	command.Flags().StringVarP(&tag, "tag", "", "", constants.HELP_ARG_TAG)
	command.Flags().StringVarP(&tracker, "tracker", "", "", constants.HELP_ARG_TRACKER)
	command.Flags().StringVarP(&savePath, "save-path", "", "",
		`Filter torrent by it's save path. E.g. "/root/Downloads"`)
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
	if largestFlag && newestFlag {
		return fmt.Errorf("--largest and --newest flags are NOT compatible")
	}
	if largestFlag {
		sortFlag = "size"
		orderFlag = "desc"
	} else if newestFlag {
		sortFlag = "time"
		orderFlag = "desc"
	}
	desc := false
	if orderFlag == "desc" {
		desc = true
	}
	if savePath != "" {
		savePath = filepath.Clean(savePath)
	}
	minTorrentSize, _ := util.RAMInBytes(minTorrentSizeStr)
	maxTorrentSize, _ := util.RAMInBytes(maxTorrentSizeStr)
	maxTotalSize, _ := util.RAMInBytes(maxTotalSizeStr)
	addedIn, _ := util.ParseTimeDuration(addedInStr)
	var completedBefore, activeSince, notActiveSince int64
	if completedBeforeStr != "" {
		completedBefore, err = util.ParseTime(completedBeforeStr, nil)
		if err != nil {
			return fmt.Errorf("invalid completed-before: %v", err)
		}
	}
	if activeSinceStr != "" {
		activeSince, err = util.ParseTime(activeSinceStr, nil)
		if err != nil {
			return fmt.Errorf("invalid active-since: %v", err)
		}
	}
	if notActiveSinceStr != "" {
		notActiveSince, err = util.ParseTime(notActiveSinceStr, nil)
		if err != nil {
			return fmt.Errorf("invalid not-active-since: %v", err)
		}
	}
	if activeSince > 0 && notActiveSince > 0 && activeSince >= notActiveSince {
		return fmt.Errorf("--active-since must be before --not-active-since flag")
	}
	now := util.Now()

	hasFilterCondition := savePath != "" || tracker != "" || minTorrentSize >= 0 || maxTorrentSize >= 0 ||
		addedIn > 0 || completedBefore > 0 || activeSince > 0 || notActiveSince > 0 || partial
	noConditionFlags := category == "" && tag == "" && filter == "" && !hasFilterCondition
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
		torrent.Print()
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
	if hasFilterCondition {
		torrents = util.Filter(torrents, func(t client.Torrent) bool {
			if savePath != "" && t.SavePath != savePath ||
				tracker != "" && !t.MatchTracker(tracker) ||
				minTorrentSize >= 0 && t.Size < minTorrentSize ||
				maxTorrentSize >= 0 && t.Size > maxTorrentSize ||
				addedIn > 0 && now-t.Atime > addedIn ||
				completedBefore > 0 && (t.Ctime <= 0 || t.Ctime >= completedBefore) ||
				activeSince > 0 && t.ActivityTime < activeSince ||
				notActiveSince > 0 && t.ActivityTime > notActiveSince ||
				partial && t.Size == t.SizeTotal {
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
			case "activity-time":
				return torrents[i].ActivityTime < torrents[j].ActivityTime
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
	if maxTotalSize >= 0 {
		i := 0
		size := int64(0)
		for _, torrent := range torrents {
			if size+torrent.Size > maxTotalSize {
				break
			}
			i++
			size += torrent.Size
		}
		torrents = torrents[:i]
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
				fmt.Sprintf("FreeSpace/UnfinishedDL: %s / %s", util.BytesSize(float64(clientStatus.FreeSpaceOnDisk)),
					util.BytesSize(float64(clientStatus.UnfinishedDownloadingSize))),
				len(torrents),
			)
		}
		showSummary := int64(1)
		if showSum {
			showSummary = 2
		}
		client.PrintTorrents(torrents, "", showSummary, dense)
	} else {
		sep := ""
		for _, torrent := range torrents {
			fmt.Printf("%s%s", sep, torrent.InfoHash)
			sep = " "
		}
	}
	return nil
}
