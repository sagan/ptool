package client

import (
	"fmt"
	"io"
	"os"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"sync"

	log "github.com/sirupsen/logrus"
	"golang.org/x/term"

	"github.com/sagan/ptool/config"
	"github.com/sagan/ptool/constants"
	"github.com/sagan/ptool/util"
)

// @todo: considering changing it to interface
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

type TorrentContentFile struct {
	Index    int64
	Path     string // full file path
	Size     int64
	Progress float64 // [0, 1]
	Ignored  bool    // true if file is ignored (excluded from downloading)
	Complete bool    // true if file is fullly downloaded
}

type Status struct {
	FreeSpaceOnDisk           int64 // -1 means unknown
	UnfinishedSize            int64
	UnfinishedDownloadingSize int64
	DownloadSpeed             int64
	UploadSpeed               int64
	DownloadSpeedLimit        int64 // <= 0 means no limit
	UploadSpeedLimit          int64 // <= 0 means no limit
	NoAdd                     bool  // if true, brush and other tasks will NOT add any torrent to client
	NoDel                     bool  // if true, brush and other tasks will NOT delete any torrent from client
}

type TorrentTracker struct {
	Status string //working|notcontacted|error|updating|disabled|unknown
	Url    string
	Msg    string
}

type TorrentTrackers []TorrentTracker

type TorrentOption struct {
	Name               string // if not empty, set name of torrent in client to this value
	Category           string
	SavePath           string
	Tags               []string
	RemoveTags         []string // used only in ModifyTorrent
	DownloadSpeedLimit int64
	UploadSpeedLimit   int64
	RatioLimit         float64 // If > 0, will stop seeding after ratio (up/dl) exceeds this value
	SeedingTimeLimit   int64   // If > 0, will stop seeding after be seeded for this time (seconds)
	SkipChecking       bool
	Pause              bool
	Resume             bool // use only in ModifyTorrent, to start a paused torrent
	SequentialDownload bool // qb only
}

type TorrentCategory struct {
	Name     string `json:"name"`
	SavePath string `json:"savePath"`
}

type Client interface {
	// download / export .torrent file for a torrent in client
	ExportTorrentFile(infoHash string) ([]byte, error)
	GetTorrent(infoHash string) (*Torrent, error)
	// category: "none" is a special value to select uncategoried torrents.
	// stateFilter: _all|_active|_done|_undone, or any state value (possibly with a _ prefix)
	GetTorrents(stateFilter string, category string, showAll bool) ([]*Torrent, error)
	GetTorrentsByContentPath(contentPath string) ([]*Torrent, error)
	AddTorrent(torrentContent []byte, option *TorrentOption, meta map[string]int64) error
	ModifyTorrent(infoHash string, option *TorrentOption, meta map[string]int64) error
	DeleteTorrents(infoHashes []string, deleteFiles bool) error
	PauseTorrents(infoHashes []string) error
	ResumeTorrents(infoHashes []string) error
	RecheckTorrents(infoHashes []string) error
	ReannounceTorrents(infoHashes []string) error
	AddTagsToTorrents(infoHashes []string, tags []string) error
	RemoveTagsFromTorrents(infoHashes []string, tags []string) error
	SetTorrentsSavePath(infoHashes []string, savePath string) error
	PauseAllTorrents() error
	ResumeAllTorrents() error
	RecheckAllTorrents() error
	ReannounceAllTorrents() error
	AddTagsToAllTorrents(tags []string) error
	RemoveTagsFromAllTorrents(tags []string) error
	SetAllTorrentsSavePath(savePath string) error
	GetTags() ([]string, error)
	CreateTags(tags ...string) error
	DeleteTags(tags ...string) error
	// create category if not existed, edit category if already exists
	MakeCategory(category string, savePath string) error
	DeleteCategories(categories []string) error
	GetCategories() ([]*TorrentCategory, error)
	SetTorrentsCatetory(infoHashes []string, category string) error
	SetAllTorrentsCatetory(category string) error
	SetTorrentsShareLimits(infoHashes []string, ratioLimit float64, seedingTimeLimit int64) error
	SetAllTorrentsShareLimits(ratioLimit float64, seedingTimeLimit int64) error
	TorrentRootPathExists(rootFolder string) bool
	GetTorrentContents(infoHash string) ([]*TorrentContentFile, error)
	PurgeCache()
	GetStatus() (*Status, error)
	GetName() string
	GetClientConfig() *config.ClientConfigStruct
	SetConfig(variable string, value string) error
	GetConfig(variable string) (string, error)
	GetTorrentTrackers(infoHash string) (TorrentTrackers, error)
	EditTorrentTracker(infoHash string, oldTracker string, newTracker string, replaceHost bool) error
	AddTorrentTrackers(infoHash string, trackers []string, oldTracker string, removeExisting bool) error
	RemoveTorrentTrackers(infoHash string, trackers []string) error
	// QB only, priority: 0	Do not download; 1	Normal priority; 6	High priority; 7	Maximal priority
	SetFilePriority(infoHash string, fileIndexes []int64, priority int64) error
	Cached() bool
	Close()
}

type RegInfo struct {
	Name    string
	Creator func(string, *config.ClientConfigStruct, *config.ConfigStruct) (Client, error)
}

type ClientCreator func(*RegInfo) (Client, error)

var (
	STATES             = []string{"seeding", "downloading", "completed", "paused", "checking", "error", "unknown"}
	STATE_FILTERS      = []string{"_all", "_active", "_done", "_undone"}
	Registry           = []*RegInfo{}
	substituteTagRegex = regexp.MustCompile(`^(category|meta\..+):.+$`)
	// all clientInstances created during this ptool program session
	clients = map[string]Client{}
)

var tracker_invalid_torrent_msgs = []string{
	"not registered",
	"not exists",
	"unauthorized",
	"require passkey",
	"require authkey",
	"种子不存在",
	"该种子没有", // monikadesign 的 msg: "该种子没有在我们的 Tracker 上注册."

}

var tracker_exceed_limit_torrent_msgs = []string{
	"already are downloading", // "You already are downloading the same torrent"
	"下载相同种子",
	"下載相同種子",
}

func (trackers TorrentTrackers) SeemsInvalidTorrent(all bool) bool {
	hasOk := false
	hasInvalid := false
	for _, tracker := range trackers {
		if tracker.Status == "working" && tracker.Msg == "" {
			hasOk = true
			break
		}
		if tracker.SeemsInvalidTorrent(all) {
			hasInvalid = true
		}
	}
	return !hasOk && hasInvalid
}

// Return true if the tracker is (seems) working but reports that the torrent does not exist in the tracker
// or current torrent passkey is invalid.
// If all is true, also include torrents that exceed the seeding / downloading clients number limit.
func (tracker *TorrentTracker) SeemsInvalidTorrent(all bool) bool {
	if tracker.Msg != "" {
		if slices.ContainsFunc(tracker_invalid_torrent_msgs, func(msg string) bool {
			return util.ContainsI(tracker.Msg, msg)
		}) {
			return true
		}
		if all && slices.ContainsFunc(tracker_exceed_limit_torrent_msgs, func(msg string) bool {
			return util.ContainsI(tracker.Msg, msg)
		}) {
			return true
		}
	}
	return false
}

func (cs *Status) Print(f io.Writer, name string, additionalInfo string) {
	info := fmt.Sprintf("FreeSpace: %s; Unfinished(All/DL): %s/%s",
		util.BytesSizeAround(float64(cs.FreeSpaceOnDisk)),
		util.BytesSizeAround(float64(cs.UnfinishedSize)),
		util.BytesSizeAround(float64(cs.UnfinishedDownloadingSize)),
	)
	if additionalInfo != "" {
		info += "; " + additionalInfo
	}
	fmt.Fprintf(f, constants.STATUS_FMT, "Client", name,
		fmt.Sprintf("↑Spd/Lmt: %s / %s/s", util.BytesSizeAround(float64(cs.UploadSpeed)),
			util.BytesSizeAround(float64(cs.UploadSpeedLimit))),
		fmt.Sprintf("↓Spd/Lmt: %s / %s/s", util.BytesSizeAround(float64(cs.DownloadSpeed)),
			util.BytesSizeAround(float64(cs.DownloadSpeedLimit))), info)
}

func PrintDummyStatus(f io.Writer, name string, info string) {
	if info != "" {
		info = "// " + info
	} else {
		info = "-"
	}
	fmt.Fprintf(f, constants.STATUS_FMT, "Client", name, "-", "-", info)
}

func Register(regInfo *RegInfo) {
	Registry = append(Registry, regInfo)
}

func Find(name string) (*RegInfo, error) {
	for _, item := range Registry {
		if item.Name == name {
			return item, nil
		}
	}
	return nil, fmt.Errorf("didn't find client %q", name)
}

func ClientExists(name string) bool {
	clientConfig := config.GetClientConfig(name)
	return clientConfig != nil
}

func CreateClient(name string) (Client, error) {
	if clients[name] != nil {
		return clients[name], nil
	}
	clientConfig := config.GetClientConfig(name)
	if clientConfig == nil {
		return nil, fmt.Errorf("client %s not existed", name)
	}
	regInfo, err := Find(clientConfig.Type)
	if err != nil {
		return nil, fmt.Errorf("unsupported client type %s", clientConfig.Type)
	}
	clientInstance, err := regInfo.Creator(name, clientConfig, config.Get())
	if err == nil {
		clients[name] = clientInstance
	}
	return clientInstance, err
}

func GenerateNameWithMeta(name string, meta map[string]int64) string {
	str := name
	first := true
	for key, value := range meta {
		if value == 0 {
			continue
		}
		if first {
			str += "__meta."
			first = false
		} else {
			str += "."
		}
		str += fmt.Sprintf("%s_%d", key, value)
	}
	return str
}

func ParseMetaFromName(fullname string) (name string, meta map[string]int64) {
	metaStrReg := regexp.MustCompile(`^(?P<name>.*?)__meta.(?P<meta>[._a-zA-Z0-9]+)$`)
	metaStrMatch := metaStrReg.FindStringSubmatch(fullname)
	if metaStrMatch != nil {
		name = metaStrMatch[metaStrReg.SubexpIndex("name")]
		meta = map[string]int64{}
		ms := metaStrMatch[metaStrReg.SubexpIndex("meta")]
		metas := strings.Split(ms, ".")
		for _, s := range metas {
			kvs := strings.Split(s, "_")
			if len(kvs) >= 2 {
				v := util.ParseInt(kvs[1])
				if v != 0 {
					meta[kvs[0]] = v
				}
			}
		}
	} else {
		name = fullname
	}
	return
}

func (torrent *Torrent) MatchFilter(filter string) bool {
	if filter == "" || util.ContainsI(torrent.Name, filter) {
		return true
	}
	return false
}

func (torrent *Torrent) MatchFiltersOr(filters []string) bool {
	return slices.ContainsFunc(filters, func(filter string) bool {
		return torrent.MatchFilter(filter)
	})
}

// Matches if torrent tracker's url or domain == tracker.
// Specially, if tracker is "none", matches if torrent does NOT have a (working) tracker.
func (torrent *Torrent) MatchTracker(tracker string) bool {
	if tracker == constants.NONE {
		return torrent.Tracker == ""
	}
	if util.IsUrl(tracker) {
		return torrent.Tracker == tracker
	}
	return torrent.TrackerDomain == tracker
}

func (torrent *Torrent) StateIconText() string {
	s := ""
	showProcess := false
	switch torrent.State {
	case "downloading":
		s = "↓"
		showProcess = true
	case "seeding":
		s = "✓↑"
	case "paused":
		s = "-↓" // may be unicode symbol ⏸
		showProcess = true
	case "completed":
		s = "✓"
	case "checking":
		if torrent.IsComplete() {
			s = "→✓"
		} else {
			s = "→↓"
		}
	case "error":
		s = "!"
		showProcess = true
	default:
		s = "?"
		showProcess = true
	}
	if showProcess {
		process := int64(float64(torrent.SizeCompleted) * 100 / float64(torrent.Size))
		if torrent.Size == torrent.SizeTotal {
			s += fmt.Sprint(process, "%")
		} else {
			s += fmt.Sprint(process, "_")
		}
	} else if torrent.Size != torrent.SizeTotal {
		s += "_"
	}
	return s
}

func (torrent *Torrent) GetCategoryFromTag() string {
	return torrent.GetMetaFromTag("category")
}

func (torrent *Torrent) GetSiteFromTag() string {
	return torrent.GetMetaFromTag("site")
}

func (torrent *Torrent) GetMetaFromTag(meta string) string {
	for _, tag := range torrent.Tags {
		if strings.HasPrefix(tag, meta+":") {
			return tag[len(meta)+1:]
		}
	}
	return ""
}

func (torrent *Torrent) GetMetadataFromTags() map[string]int64 {
	metas := map[string]int64{}
	metaTagRegex := regexp.MustCompile(`^meta\.(?P<name>.+):(?P<value>.+)$`)
	for _, tag := range torrent.Tags {
		metaStrMatch := metaTagRegex.FindStringSubmatch(tag)
		if metaStrMatch != nil {
			name := metaStrMatch[metaTagRegex.SubexpIndex("name")]
			value := util.ParseInt(metaStrMatch[metaTagRegex.SubexpIndex("value")])
			metas[name] = value
		}
	}
	return metas
}

func (torrent *Torrent) RemoveSubstituteTags() {
	torrent.Tags = util.Filter(torrent.Tags, func(tag string) bool {
		return !substituteTagRegex.MatchString(tag)
	})
}

func (torrent *Torrent) IsComplete() bool {
	return torrent.SizeCompleted == torrent.Size
}

func (torrent *Torrent) IsFullComplete() bool {
	return torrent.SizeCompleted == torrent.SizeTotal
}

func (torrent *Torrent) IsFull() bool {
	return torrent.Size == torrent.SizeTotal
}

func (torrent *Torrent) HasTag(tag string) bool {
	return slices.ContainsFunc(torrent.Tags, func(t string) bool {
		return strings.EqualFold(tag, t)
	})
}

// Return true if torrent has any tag in the tags.
// tags: comma-separated tag list.
func (torrent *Torrent) HasAnyTag(tags string) bool {
	if tags == "" {
		return false
	}
	return slices.ContainsFunc(util.SplitCsv(tags), func(tag string) bool {
		return torrent.HasTag(tag)
	})
}

// return index or -1
func (trackers TorrentTrackers) FindIndex(hostOrUrl string) int {
	for i, tracker := range trackers {
		if util.MatchUrlWithHostOrUrl(tracker.Url, hostOrUrl) {
			return i
		}
	}
	return -1
}

func GenerateTorrentTagFromSite(site string) string {
	return "site:" + site
}

func GenerateTorrentTagFromCategory(category string) string {
	return "category:" + category
}

func GenerateTorrentTagFromMetadata(name string, value int64) string {
	return "meta." + name + ":" + fmt.Sprint(value)
}

func IsSubstituteTag(tag string) bool {
	return substituteTagRegex.MatchString(tag)
}

func PrintTorrentTrackers(trackers TorrentTrackers) {
	fmt.Printf("Trackers:\n")
	fmt.Printf("%-8s  %-60s  %s\n", "Status", "Msg", "Url")
	for _, tracker := range trackers {
		fmt.Printf("%-8s  ", tracker.Status)
		util.PrintStringInWidth(os.Stdout, tracker.Msg, 60, true)
		fmt.Printf("  %s\n", tracker.Url)
	}
}

func PrintTorrentFiles(files []*TorrentContentFile, showRaw bool) {
	fmt.Printf("Files (%d):\n", len(files))
	fmt.Printf("%-5s  %-5s  %-10s  %-5s  %s\n", "No.", "Index", "Size", "Done?", "Path")
	ignoredFilesCnt := int64(0)
	for i, file := range files {
		isDone := ""
		if file.Complete {
			isDone += "✓"
		} else if !file.Ignored {
			isDone += fmt.Sprintf("%d%%", int(file.Progress*100))
		}
		if file.Ignored {
			isDone += "-"
			ignoredFilesCnt++
		}
		if showRaw {
			fmt.Printf("%-5d  %-5d  %-10d  %-5s  %s\n", i+1, file.Index, file.Size, isDone, file.Path)
		} else {
			fmt.Printf("%-5d  %-5d  %-10s  %-5s  %s\n",
				i+1, file.Index, util.BytesSize(float64(file.Size)), isDone, file.Path)
		}
	}
	if ignoredFilesCnt > 0 {
		fmt.Printf("// Note: some files (marked with -) are ignored (not_download). "+
			"Download / Ignore / All files: %d / %d / %d\n",
			len(files)-int(ignoredFilesCnt), ignoredFilesCnt, len(files))
	}
}

func (torrent *Torrent) Print() {
	ctimeStr := "-"
	if torrent.Ctime > 0 {
		ctimeStr = util.FormatTime(torrent.Ctime)
	}
	fmt.Printf("Torrent name: %s\n", torrent.Name)
	fmt.Printf("- InfoHash: %s\n", torrent.InfoHash)
	fmt.Printf("- Size: %s (%d)", util.BytesSize(float64(torrent.Size)), torrent.Size)
	if torrent.Size != torrent.SizeTotal {
		fmt.Printf(" (partial)")
	}
	fmt.Printf("\n")
	fmt.Printf("- Process: %d%%\n", int64(float64(torrent.SizeCompleted)*100/float64(torrent.Size)))
	fmt.Printf("- Total Size: %s (%d)\n", util.BytesSize(float64(torrent.SizeTotal)), torrent.SizeTotal)
	fmt.Printf("- State (LowLevelState): %s (%s)\n", torrent.State, torrent.LowLevelState)
	fmt.Printf("- Speeds: ↓S: %s/s | ↑S: %s/s\n",
		util.BytesSize(float64(torrent.DownloadSpeed)),
		util.BytesSize(float64(torrent.UploadSpeed)),
	)
	fmt.Printf("- Category: %s\n", torrent.Category)
	fmt.Printf("- Tags: %s\n", strings.Join(torrent.Tags, ","))
	fmt.Printf("- Meta: %v\n", torrent.Meta)
	fmt.Printf("- Add time: %s\n", util.FormatTime(torrent.Atime))
	fmt.Printf("- Completion time: %s\n", ctimeStr)
	fmt.Printf("- Last activity time: %s\n", util.FormatTime(torrent.ActivityTime))
	fmt.Printf("- Tracker: %s\n", torrent.Tracker)
	fmt.Printf("- Seeders / Peers: %d / %d\n", torrent.Seeders, torrent.Leechers)
	fmt.Printf("- Save path: %s\n", torrent.SavePath)
	fmt.Printf("- Content path: %s\n", torrent.ContentPath)
	fmt.Printf("- Downloaded / Uploaded: %s / %s\n",
		util.BytesSize(float64(torrent.Downloaded)),
		util.BytesSize(float64(torrent.Uploaded)),
	)
}

// showSum: 0 - no; 1 - yes; 2 - sum only
func PrintTorrents(output io.Writer, torrents []*Torrent, filter string, showSum int64, dense bool) {
	width, _, _ := term.GetSize(int(os.Stdout.Fd()))
	if width < config.CLIENT_TORRENTS_WIDTH {
		width = config.CLIENT_TORRENTS_WIDTH
	}
	widthExcludingName := 105 // 40+6+5+6+6+5+5+16+8*2
	widthName := width - widthExcludingName
	cnt := int64(0)
	var cntPaused, cntDownloading, cntSeeding, cntCompleted, cntOthers int64
	size := int64(0)
	smallestSize := int64(-1)
	largestSize := int64(-1)
	sizeUnfinished := int64(0)
	if showSum < 2 {
		fmt.Fprintf(output, "%-*s  %-40s  %-6s  %-5s  %-6s  %-6s  %-5s  %-5s  %-16s\n",
			widthName, "Name", "InfoHash", "Size", "State", "↓S(/s)", "↑S(/s)", "Seeds", "Peers", "Tracker")
	}
	for _, torrent := range torrents {
		if filter != "" && !torrent.MatchFilter(filter) {
			continue
		}
		cnt++
		switch torrent.State {
		case "paused":
			cntPaused++
		case "downloading":
			cntDownloading++
		case "seeding":
			cntSeeding++
		case "completed":
			cntCompleted++
		default:
			cntOthers++
		}
		size += torrent.Size
		if largestSize == -1 || torrent.Size > largestSize {
			largestSize = torrent.Size
		}
		if smallestSize == -1 || torrent.Size < smallestSize {
			smallestSize = torrent.Size
		}
		sizeUnfinished += torrent.Size - torrent.SizeCompleted
		if showSum >= 2 {
			continue
		}
		name := torrent.Name
		if dense && (torrent.Category != "" || len(torrent.Tags) > 0 || torrent.ContentPath != "") {
			name += " //"
			if torrent.Category != "" {
				name += " " + strconv.Quote(torrent.Category)
			}
			if len(torrent.Tags) > 0 {
				name += fmt.Sprintf(" [%s]", strings.Join(util.Map(torrent.Tags, func(t string) string {
					return fmt.Sprintf("%q", t)
				}), ", "))
			}
			if torrent.ContentPath != "" {
				name += ` >` + torrent.ContentPath
			}
		}
		remain := util.PrintStringInWidth(output, name, int64(widthName), true)
		// 目前遇到的tracker域名最长的: "wintersakura.net"
		trackerBaseDomain, _ := util.StringPrefixInWidth(torrent.TrackerBaseDomain, 16)
		fmt.Fprintf(output, "  %-40s  %-6s  %-5s  %-6s  %-6s  %-5d  %-5d  %-16s\n",
			torrent.InfoHash,
			util.BytesSizeAround(float64(torrent.Size)),
			torrent.StateIconText(),
			util.BytesSizeAround(float64(torrent.DownloadSpeed)),
			util.BytesSizeAround(float64(torrent.UploadSpeed)),
			torrent.Seeders,
			torrent.Leechers,
			trackerBaseDomain,
		)
		if dense {
			for {
				remain = strings.TrimSpace(remain)
				if remain == "" {
					break
				}
				remain = util.PrintStringInWidth(output, remain, int64(widthName), true)
				fmt.Fprintf(output, "\n")
			}
		}
	}
	if showSum > 0 {
		averageSize := int64(0)
		if cnt > 0 {
			averageSize = size / cnt
		}
		fmt.Fprintf(output, "\n")
		fmt.Fprintf(output, "// Summary - Cnt / Size / SizeUnfinished: %d / %s / %s\n",
			cnt, util.BytesSize(float64(size)), util.BytesSize(float64(sizeUnfinished)))
		fmt.Fprintf(output, "// Torrents: ↓%d / -%d / ↑%d / ✓%d / +%d\n",
			cntDownloading, cntPaused, cntSeeding, cntCompleted, cntOthers)
		fmt.Fprintf(output, "// Smallest / Average / Largest torrent size: %s / %s / %s\n",
			util.BytesSize(float64(smallestSize)), util.BytesSize(float64(averageSize)), util.BytesSize(float64(largestSize)))
	}
}

// Separate client torrents into 2 groups: torrentsNoXseed and torrentsXseed.
// The first ones does NOT have any other xseed torrent of same content path,
// or all xseed torrents themselves are also in the group.
// The second ones has other xseed torrent of same content path.
func FilterTorrentsXseed(clientInstance Client, torrents []*Torrent) (
	torrentsNoXseed, torrentsXseed []*Torrent, err error) {
	for _, t := range torrents {
		sameContentPathTorrents, err := clientInstance.GetTorrentsByContentPath(t.ContentPath)
		if err != nil {
			return nil, nil, err
		}
		hasXseedTorrent := slices.ContainsFunc(sameContentPathTorrents, func(st *Torrent) bool {
			return !slices.ContainsFunc(torrents, func(dt *Torrent) bool {
				return dt.InfoHash == st.InfoHash
			})
		})
		if hasXseedTorrent {
			torrentsXseed = append(torrentsXseed, t)
		} else {
			torrentsNoXseed = append(torrentsNoXseed, t)
		}
	}
	return
}

// Delete torrents from client. If torrent has no other xseed torrent (with same content path),
// delete files; Otherwise preserve files.
func DeleteTorrentsAuto(clientInstance Client, infoHashes []string) (err error) {
	var torrents []*Torrent
	for _, infoHash := range infoHashes {
		if torrent, _ := clientInstance.GetTorrent(infoHash); torrent != nil {
			torrents = append(torrents, torrent)
		}
	}
	torrents, torrentsXseed, err := FilterTorrentsXseed(clientInstance, torrents)
	if err != nil {
		return err
	}
	if len(torrentsXseed) > 0 {
		infoHashes := util.Map(torrentsXseed, func(t *Torrent) string { return t.InfoHash })
		err = clientInstance.DeleteTorrents(infoHashes, false)
		if err != nil {
			return fmt.Errorf("failed to delete torrents: %w", err)
		}
	}
	if len(torrents) > 0 {
		infoHashes := util.Map(torrents, func(t *Torrent) string { return t.InfoHash })
		err = clientInstance.DeleteTorrents(infoHashes, true)
		if err != nil {
			return fmt.Errorf("failed to delete torrents: %w", err)
		}
	}
	return nil
}

// Parse and return torrents that meet criterion.
// tag: comma-separated list, a torrent matches if it has any tag that in the list;
// specially, "none" means untagged torrents.
func QueryTorrents(clientInstance Client, category string, tag string, filter string,
	hashOrStateFilters ...string) ([]*Torrent, error) {
	isAll := len(hashOrStateFilters) == 0
	for _, arg := range hashOrStateFilters {
		if !IsValidInfoHashOrStateFilter(arg) {
			return nil, fmt.Errorf("%s is not a valid infoHash nor stateFilter", arg)
		}
		if arg == "_all" {
			isAll = true
		}
	}
	torrents, err := clientInstance.GetTorrents("", category, true)
	if err != nil {
		return nil, err
	}
	if category == "" && tag == "" && filter == "" && isAll {
		return torrents, nil
	}
	torrents2 := []*Torrent{}
	for _, torrent := range torrents {
		if tag != "" {
			if tag == constants.NONE {
				if len(torrent.Tags) > 0 {
					continue
				}
			} else if !torrent.HasAnyTag(tag) {
				continue
			}
		}
		if filter != "" && !torrent.MatchFilter(filter) {
			continue
		}
		if isAll {
			torrents2 = append(torrents2, torrent)
		} else {
			for _, arg := range hashOrStateFilters {
				if strings.HasPrefix(arg, "_") {
					if torrent.MatchStateFilter(arg) {
						torrents2 = append(torrents2, torrent)
					}
				} else if arg == torrent.InfoHash {
					torrents2 = append(torrents2, torrent)
				}
			}
		}
	}
	return torrents2, nil
}

// Query torrents that meet criterion and return infoHashes. Specially, return nil slice if all torrents selected.
// If all hashOrStateFilters is plain info-hash and all other conditions empty, just return hashOrStateFilters,nil.
// tag: comma-separated list, a torrent matches if it has any tag that in the list;
// specially, "none" means untagged torrents.
func SelectTorrents(clientInstance Client, category string, tag string, filter string,
	hashOrStateFilters ...string) ([]string, error) {
	noCondition := category == "" && tag == "" && filter == ""
	isAll := len(hashOrStateFilters) == 0
	isPlainInfoHashes := true
	for _, arg := range hashOrStateFilters {
		if IsValidInfoHash(arg) {
			continue
		}
		if !IsValidStateFilter(arg) {
			return nil, fmt.Errorf("%s is not a valid infoHash nor stateFilter", arg)
		}
		isPlainInfoHashes = false
		if arg == "_all" {
			isAll = true
		}
	}
	if noCondition {
		if isAll {
			return nil, nil
		}
		if isPlainInfoHashes {
			return hashOrStateFilters, nil
		}
	}
	torrents, err := clientInstance.GetTorrents("", category, true)
	if err != nil {
		return nil, err
	}
	infoHashes := []string{}
	for _, torrent := range torrents {
		if tag != "" {
			if tag == constants.NONE {
				if len(torrent.Tags) > 0 {
					continue
				}
			} else if !torrent.HasAnyTag(tag) {
				continue
			}
		}
		if filter != "" && !torrent.MatchFilter(filter) {
			continue
		}
		if isAll {
			infoHashes = append(infoHashes, torrent.InfoHash)
		} else {
			for _, arg := range hashOrStateFilters {
				if strings.HasPrefix(arg, "_") {
					if torrent.MatchStateFilter(arg) {
						infoHashes = append(infoHashes, torrent.InfoHash)
					}
				} else if arg == torrent.InfoHash {
					infoHashes = append(infoHashes, torrent.InfoHash)
				}
			}
		}
	}
	return infoHashes, nil
}

func (torrent *Torrent) MatchStateFilter(stateFilter string) bool {
	if stateFilter == "" || stateFilter == "_all" {
		return true
	}
	if strings.HasPrefix(stateFilter, "_") {
		switch stateFilter {
		case "_active":
			return torrent.DownloadSpeed >= 1024 || torrent.UploadSpeed >= 1024
		case "_done":
			return torrent.IsComplete()
		case "_undone":
			return !torrent.IsComplete()
		default:
			stateFilter = stateFilter[1:]
		}
	}
	return stateFilter == torrent.State
}

var infoHashV1Regex = regexp.MustCompile(`^[0-9a-fA-F]{40}$`)
var infoHashV2Regex = regexp.MustCompile(`^[0-9a-fA-F]{64}$`)

func IsValidInfoHash(infoHash string) bool {
	return infoHashV1Regex.MatchString(infoHash) || infoHashV2Regex.MatchString(infoHash)
}

func IsValidStateFilter(stateFilter string) bool {
	if strings.HasPrefix(stateFilter, "_") {
		if slices.Contains(STATE_FILTERS, stateFilter) {
			return true
		}
		stateFilter = stateFilter[1:]
		return slices.Contains(STATES, stateFilter)
	}
	return false
}

func IsValidInfoHashOrStateFilter(stateFilter string) bool {
	return IsValidInfoHash(stateFilter) || IsValidStateFilter(stateFilter)
}

func init() {
}

// called by main codes on program exit. clean resources
func Exit() {
	var resourcesWaitGroup sync.WaitGroup
	for clientName, clientInstance := range clients {
		resourcesWaitGroup.Add(1)
		go func(clientName string, clientInstance Client) {
			defer resourcesWaitGroup.Done()
			log.Tracef("Close client %s instance", clientName)
			clientInstance.Close()
			// delete(clients, clientName) // may lead to race condition
		}(clientName, clientInstance)
	}
	resourcesWaitGroup.Wait()
}

// Purge client cache
func Purge(clientName string) {
	if clientName == "" {
		for _, clientInstance := range clients {
			clientInstance.PurgeCache()
		}
	} else if clients[clientName] != nil {
		clients[clientName].PurgeCache()
	}
}
