package show

import (
	"bytes"
	"fmt"
	"os"
	"path"
	"sort"
	"strings"
	"text/template"
	"time"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/sagan/ptool/client"
	"github.com/sagan/ptool/cmd"
	"github.com/sagan/ptool/cmd/common"
	"github.com/sagan/ptool/constants"
	"github.com/sagan/ptool/util"
	"github.com/sagan/ptool/util/helper"
)

var command = &cobra.Command{
	Use:         "show {client} [--category category] [--tag tag] [--filter filter] [infoHash]...",
	Annotations: map[string]string{"cobra-prompt-dynamic-suggestions": "show"},
	Short:       "Show torrents of client.",
	Long: `Show torrents of client.
[infoHash]...: Args list, info-hash list of torrents. It's possible to use state filter to select multiple torrents:
  _all, _active, _done, _undone, _downloading, _seeding, _paused, _completed, _error.
If both filter flags (--category & --tag & --filter) and args are not set, it will display current active torrents.
If at least one filter flag is set but no arg is provided, the args is assumed to be "_all"

By default, it displays found torrents of client in the list, which has several fields like "Name" and "State".

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

Specially, if all args is an (1) single info-hash, it displays the details of that torrent instead of the list.

If "--json" flag is set, it prints torrents info in json (array) format.

You can also customize the output format of each torrent using "--format string" flag.
The data passed to the template is the "client.Torrent" struct:

// https://github.com/sagan/ptool/blob/master/client/client.go
type Torrent struct {
	InfoHash           string
	Name               string
	TrackerDomain      string // e.g. tracker.m-team.cc
	TrackerBaseDomain  string // e.g. m-team.cc
	Tracker            string
	State              string // simplified state: seeding|downloading|completed|paused|checking|error|unknown
	LowLevelState      string // original state value returned by bt client
	Atime              int64  // timestamp torrent added
	Ctime              int64  // timestamp torrent completed. <=0 if not completed.
	ActivityTime       int64  // timestamp of torrent latest activity (a chunk being downloaded / uploaded)
	Category           string
	SavePath           string
	ContentPath        string
	Tags               []string
	Downloaded         int64
	DownloadSpeed      int64
	DownloadSpeedLimit int64 // -1 means no limit
	Uploaded           int64
	UploadSpeed        int64
	UploadedSpeedLimit int64 // -1 means no limit
	Size               int64 // size of torrent files that selected for downloading
	SizeTotal          int64 // Total size of all file in the torrent (including unselected ones)
	SizeCompleted      int64
	Seeders            int64 // Cnt of seeders (including self client, if it's seeding), returned by tracker
	Leechers           int64
	Meta               map[string]int64
}

The template render result will be trim spaced.

E.g. '--format "{{.InfoHash}}  {{.ContentPath}}"'`,
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
	addedAfterStr      = ""
	completedBeforeStr = ""
	activeSinceStr     = ""
	notActiveSinceStr  = ""
	filter             = ""
	category           = ""
	tag                = ""
	excludeTag         = ""
	tracker            = ""
	savePath           = ""
	contentPath        = ""
	savePathPrefix     = ""
	minTorrentSizeStr  = ""
	maxTorrentSizeStr  = ""
	maxTotalSizeStr    = ""
	excludes           = ""
	format             = ""
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
	command.Flags().StringVarP(&addedAfterStr, "added-after", "", "",
		`Only showing torrent that was added to client after (>=) this time. `+constants.HELP_ARG_TIMES)
	command.Flags().StringVarP(&completedBeforeStr, "completed-before", "", "",
		`Only showing torrent that was downloaded completed before (<) this time. `+constants.HELP_ARG_TIMES)
	command.Flags().StringVarP(&activeSinceStr, "active-since", "", "",
		`Only showing torrent that has activity since (>=) this time. `+constants.HELP_ARG_TIMES)
	command.Flags().StringVarP(&notActiveSinceStr, "not-active-since", "", "",
		`Only showing torrent that does NOT has activity since (>=) this time. `+constants.HELP_ARG_TIMES)
	command.Flags().StringVarP(&filter, "filter", "", "", constants.HELP_ARG_FILTER_TORRENT)
	command.Flags().StringVarP(&category, "category", "", "", constants.HELP_ARG_CATEGORY)
	command.Flags().StringVarP(&tag, "tag", "", "", constants.HELP_ARG_TAG)
	command.Flags().StringVarP(&excludeTag, "exclude-tag", "", "", `Comma-separated tag list. `+
		`Torrent which tags contain any one in the list will be excluded. Use "`+
		constants.NONE+`" to exlude untagged torrents`)
	command.Flags().StringVarP(&tracker, "tracker", "", "", constants.HELP_ARG_TRACKER)
	command.Flags().StringVarP(&savePath, "save-path", "", "",
		`Filter torrent by it's save path. E.g. "/root/Downloads"`)
	command.Flags().StringVarP(&savePathPrefix, "save-path-prefix", "", "",
		`Filter torrent by it's save path prefix. E.g. "/root/Downloads" will match any torrent which save path `+
			`is "/root/Downloads" or any sub path inside it like "/root/Downloads/Videos"`)
	command.Flags().StringVarP(&contentPath, "content-path", "", "",
		`Filter torrent by it's content path. E.g. "/root/Downloads/[BDMV]Clannad"`)
	command.Flags().StringVarP(&minTorrentSizeStr, "min-torrent-size", "", "-1", constants.HELP_ARG_MIN_TORRENT_SIZE)
	command.Flags().StringVarP(&maxTorrentSizeStr, "max-torrent-size", "", "-1", constants.HELP_ARG_MAX_TORRENT_SIZE)
	command.Flags().StringVarP(&excludes, "exclude", "", "",
		"Comma-separated list that torrent which name contains any one in the list will be skipped")
	command.Flags().StringVarP(&format, "format", "", "", `Manually set the output format of each client torrent. `+
		`Available variable placeholders: {{.InfoHash}}, {{.Size}} and more. `+constants.HELP_ARG_TEMPLATE)
	cmd.AddEnumFlagP(command, &sortFlag, "sort", "", common.ClientTorrentSortFlag)
	cmd.AddEnumFlagP(command, &orderFlag, "order", "", common.OrderFlag)
	cmd.RootCmd.AddCommand(command)
}

func show(cmd *cobra.Command, args []string) error {
	clientName := args[0]
	infoHashes := args[1:]
	if cnt := util.CountNonZeroVariables(showSum, showJson, showInfoHashOnly, format, showFiles,
		showTrackers); cnt > 1 && (cnt > 2 || (!(showSum && showJson) && !(showFiles && showTrackers))) {
		return fmt.Errorf(`--sum, --json, --format, --show-files, --show-trackers flags are NOT compatible ` +
			`(unless the first two and the last two)`)
	}
	if util.CountNonZeroVariables(savePath, savePathPrefix, contentPath) > 1 {
		return fmt.Errorf("--save-path, --save-path-prefix and --content-path flags are NOT compatible")
	}
	clientInstance, err := client.CreateClient(clientName)
	if err != nil {
		return fmt.Errorf("failed to create client: %w", err)
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
		savePath = path.Clean(savePath)
	}
	if savePathPrefix != "" {
		savePathPrefix = path.Clean(savePathPrefix)
	}
	if contentPath != "" {
		contentPath = path.Clean(contentPath)
	}
	minTorrentSize, _ := util.RAMInBytes(minTorrentSizeStr)
	maxTorrentSize, _ := util.RAMInBytes(maxTorrentSizeStr)
	maxTotalSize, _ := util.RAMInBytes(maxTotalSizeStr)
	var addedAfter, completedBefore, activeSince, notActiveSince int64
	now := time.Now()
	if addedAfterStr != "" {
		addedAfter, err = util.ParseTimeWithNow(addedAfterStr, nil, now)
		if err != nil {
			return fmt.Errorf("invalid added-after: %w", err)
		}
	}
	if completedBeforeStr != "" {
		completedBefore, err = util.ParseTimeWithNow(completedBeforeStr, nil, now)
		if err != nil {
			return fmt.Errorf("invalid completed-before: %w", err)
		}
	}
	if activeSinceStr != "" {
		activeSince, err = util.ParseTimeWithNow(activeSinceStr, nil, now)
		if err != nil {
			return fmt.Errorf("invalid active-since: %w", err)
		}
	}
	if notActiveSinceStr != "" {
		notActiveSince, err = util.ParseTimeWithNow(notActiveSinceStr, nil, now)
		if err != nil {
			return fmt.Errorf("invalid not-active-since: %w", err)
		}
	}
	if addedAfter > 0 && completedBefore > 0 && addedAfter > completedBefore {
		return fmt.Errorf("--added-after must NOT be after --completed-before flag")
	}
	if activeSince > 0 && notActiveSince > 0 && activeSince >= notActiveSince {
		return fmt.Errorf("--active-since must be before --not-active-since flag")
	}
	excludesList := util.SplitCsv(excludes)
	var outputTemplate *template.Template
	if format != "" {
		if outputTemplate, err = helper.GetTemplate(format); err != nil {
			return fmt.Errorf("invalid format template: %v", err)
		}
	}

	hasFilterCondition := savePath != "" || savePathPrefix != "" || contentPath != "" ||
		tracker != "" || minTorrentSize >= 0 || maxTorrentSize >= 0 || addedAfter > 0 || completedBefore > 0 ||
		activeSince > 0 || notActiveSince > 0 || partial || excludes != "" || excludeTag != ""
	noConditionFlags := category == "" && tag == "" && filter == "" && !hasFilterCondition
	var torrents []*client.Torrent
	if showAll {
		torrents, err = client.QueryTorrents(clientInstance, "", "", "")
	} else if noConditionFlags && len(infoHashes) == 0 {
		torrents, err = client.QueryTorrents(clientInstance, "", "", "", "_active")
	} else if noConditionFlags && len(infoHashes) == 1 && !strings.HasPrefix(infoHashes[0], "_") &&
		format == "" && !showJson && !showSum {
		// display single torrent details
		if !client.IsValidInfoHash(infoHashes[0]) {
			return fmt.Errorf("%s is not a valid infoHash", infoHashes[0])
		}
		torrent, err := clientInstance.GetTorrent(infoHashes[0])
		if err != nil {
			return fmt.Errorf("failed to get torrent %s details: %w", infoHashes[0], err)
		}
		if torrent == nil {
			return fmt.Errorf("torrent %s not found", infoHashes[0])
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
		return fmt.Errorf("failed to fetch client torrents: %w", err)
	}
	if hasFilterCondition {
		torrents = util.Filter(torrents, func(t *client.Torrent) bool {
			if savePath != "" && t.SavePath != savePath ||
				savePathPrefix != "" && t.SavePath != savePathPrefix && !strings.HasPrefix(t.SavePath, savePathPrefix+`/`) &&
					!strings.HasPrefix(t.SavePath, savePathPrefix+`\`) ||
				contentPath != "" && t.ContentPath != contentPath ||
				excludes != "" && t.MatchFiltersOr(excludesList) ||
				tracker != "" && !t.MatchTracker(tracker) ||
				minTorrentSize >= 0 && t.Size < minTorrentSize ||
				maxTorrentSize >= 0 && t.Size > maxTorrentSize ||
				addedAfter > 0 && t.Atime < addedAfter ||
				completedBefore > 0 && (t.Ctime <= 0 || t.Ctime >= completedBefore) ||
				activeSince > 0 && t.ActivityTime < activeSince ||
				notActiveSince > 0 && t.ActivityTime >= notActiveSince ||
				excludeTag != "" && t.HasAnyTag(excludeTag) ||
				partial && t.Size == t.SizeTotal {
				return false
			}
			return true
		})
	}
	if sortFlag != "" && sortFlag != constants.NONE {
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

	if outputTemplate != nil {
		for _, torrent := range torrents {
			buf := &bytes.Buffer{}
			err := outputTemplate.Execute(buf, torrent)
			if err == nil {
				fmt.Println(strings.TrimSpace(buf.String()))
			} else {
				log.Errorf("Torrent render error: %v", err)
			}
		}
	} else if showJson {
		return util.PrintJson(os.Stdout, torrents)
	} else if showInfoHashOnly {
		sep := ""
		for _, torrent := range torrents {
			fmt.Printf("%s%s", sep, torrent.InfoHash)
			sep = " "
		}
	} else {
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
		client.PrintTorrents(os.Stdout, torrents, "", showSummary, dense)
	}
	return nil
}
