package client

import (
	"fmt"
	"regexp"
	"strings"

	"golang.org/x/exp/slices"

	"github.com/sagan/ptool/config"
	"github.com/sagan/ptool/utils"
)

type Torrent struct {
	InfoHash           string
	Name               string
	TrackerDomain      string
	Tracker            string
	State              string // simplified state: seeding|downloading|completed|paused|checking|error|unknown
	LowLevelState      string // original state value returned by bt client
	Atime              int64  // timestamp torrent added
	Ctime              int64  // timestamp torrent completed. <=0 if not completed.
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
	Seeders            int64
	Leechers           int64
	Meta               map[string](int64)
}

type TorrentContentFile struct {
	Index    int64
	Path     string // full file path
	Size     int64
	Complete bool // true if file is fullly downloaded
}

type Status struct {
	FreeSpaceOnDisk           int64 // -1 means unknown
	UnfinishedSize            int64
	UnfinishedDownloadingSize int64
	DownloadSpeed             int64
	UploadSpeed               int64
	DownloadSpeedLimit        int64 // <= 0 means no limit
	UploadSpeedLimit          int64 // <= 0 means no limit
	NoAdd                     bool  // if true, brush and other tasks will NOT add any torrents to client
}

type TorrentTracker struct {
	Status string //working|notcontacted|error|updating|disabled|unknown
	Url    string
	Msg    string
}

type TorrentTrackers []TorrentTracker

type TorrentOption struct {
	Name               string
	Category           string
	SavePath           string
	Tags               []string
	RemoveTags         []string // used only in ModifyTorrent
	DownloadSpeedLimit int64
	UploadSpeedLimit   int64
	SkipChecking       bool
	Pause              bool
	Resume             bool // use only in ModifyTorrent, to start a paused torrent
}

type TorrentCategory struct {
	Name     string `json:"name"`
	SavePath string `json:"savePath"`
}

type Client interface {
	GetTorrent(infoHash string) (*Torrent, error)
	// stateFilter: _all|_active|_done|_undone, or any state value (possibly with a _ prefix)
	GetTorrents(stateFilter string, category string, showAll bool) ([]Torrent, error)
	AddTorrent(torrentContent []byte, option *TorrentOption, meta map[string](int64)) error
	ModifyTorrent(infoHash string, option *TorrentOption, meta map[string](int64)) error
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
	RemoveCategories(categories []string) error
	GetCategories() ([]TorrentCategory, error)
	SetTorrentsCatetory(infoHashes []string, category string) error
	SetAllTorrentsCatetory(category string) error
	TorrentRootPathExists(rootFolder string) bool
	GetTorrentContents(infoHash string) ([]TorrentContentFile, error)
	PurgeCache()
	GetStatus() (*Status, error)
	GetName() string
	GetClientConfig() *config.ClientConfigStruct
	SetConfig(variable string, value string) error
	GetConfig(variable string) (string, error)
	GetTorrentTrackers(infoHash string) (TorrentTrackers, error)
	EditTorrentTracker(infoHash string, oldTracker string, newTracker string, replaceHost bool) error
	AddTorrentTrackers(infoHash string, trackers []string, oldTracker string) error
	RemoveTorrentTrackers(infoHash string, trackers []string) error
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
)

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
	clientConfig := config.GetClientConfig(name)
	if clientConfig == nil {
		return nil, fmt.Errorf("client %s not existed", name)
	}
	regInfo, err := Find(clientConfig.Type)
	if err != nil {
		return nil, fmt.Errorf("unsupported client type %s", clientConfig.Type)
	}
	return regInfo.Creator(name, clientConfig, config.Get())
}

func GenerateNameWithMeta(name string, meta map[string](int64)) string {
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

func ParseMetaFromName(fullname string) (name string, meta map[string](int64)) {
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
				v := utils.ParseInt(kvs[1])
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

func (torrent *Torrent) StateIconText() string {
	s := ""
	showProcess := false
	switch torrent.State {
	case "downloading":
		s = "↓"
		showProcess = true
	case "seeding":
		s = "↑U"
	case "paused":
		s = "-P" // may be unicode symbol ⏸
		showProcess = true
	case "completed":
		s = "✓C"
	case "checking":
		s = "→c"
	case "error":
		s = "!e"
		showProcess = true
	default:
		s = "?"
		showProcess = true
	}
	if showProcess {
		process := int64(float64(torrent.SizeCompleted) * 100 / float64(torrent.Size))
		s += fmt.Sprint(process, "%")
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

func (torrent *Torrent) GetMetadataFromTags() map[string](int64) {
	metas := map[string](int64){}
	metaTagRegex := regexp.MustCompile(`^meta\.(?P<name>.+):(?P<value>.+)$`)
	for _, tag := range torrent.Tags {
		metaStrMatch := metaTagRegex.FindStringSubmatch(tag)
		if metaStrMatch != nil {
			name := metaStrMatch[metaTagRegex.SubexpIndex("name")]
			value := utils.ParseInt(metaStrMatch[metaTagRegex.SubexpIndex("value")])
			metas[name] = value
		}
	}
	return metas
}

func (torrent *Torrent) RemoveSubstituteTags() {
	torrent.Tags = utils.Filter(torrent.Tags, func(tag string) bool {
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
	tag = strings.ToLower(tag)
	return slices.IndexFunc(torrent.Tags, func(t string) bool {
		return strings.ToLower(t) == tag
	}) != -1
}

// return index or -1
func (trackers TorrentTrackers) FindIndex(hostOrUrl string) int {
	for i, tracker := range trackers {
		if utils.MatchUrlWithHostOrUrl(tracker.Url, hostOrUrl) {
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
	fmt.Printf("%-8s  %-40s  %s\n", "Status", "Msg", "Url")
	for _, tracker := range trackers {
		fmt.Printf("%-8s  ", tracker.Status)
		utils.PrintStringInWidth(tracker.Msg, 40, true)
		fmt.Printf("  %s\n", tracker.Url)
	}
}
func PrintTorrentFiles(files []TorrentContentFile) {
	fmt.Printf("Files (%d):\n", len(files))
	fmt.Printf("%-5s  %-10s  %-6s  %s\n", "Index", "Size", "IsDone", "Path")
	for i, file := range files {
		isDone := "✗"
		if file.Complete {
			isDone = "✓"
		}
		fmt.Printf("%-5d  %-10d  %-6s  %s\n", i+1, file.Size, isDone, file.Path)
	}
}

func PrintTorrent(torrent *Torrent) {
	ctimeStr := "-"
	if torrent.Ctime > 0 {
		ctimeStr = utils.FormatTime(torrent.Ctime)
	}
	fmt.Printf("Torrent name: %s\n", torrent.Name)
	fmt.Printf("InfoHash: %s\n", torrent.InfoHash)
	fmt.Printf("Size: %s (%d)\n", utils.BytesSize(float64(torrent.Size)), torrent.Size)
	fmt.Printf("Process: %d%%\n", int64(float64(torrent.SizeCompleted)*100/float64(torrent.Size)))
	fmt.Printf("Total Size: %s (%d)\n", utils.BytesSize(float64(torrent.SizeTotal)), torrent.SizeTotal)
	fmt.Printf("State (LowLevelState): %s (%s)\n", torrent.State, torrent.LowLevelState)
	fmt.Printf("Speeds: ↓S: %s/s | ↑S: %s/s\n",
		utils.BytesSize(float64(torrent.DownloadSpeed)),
		utils.BytesSize(float64(torrent.UploadSpeed)),
	)
	fmt.Printf("Category: %s\n", torrent.Category)
	fmt.Printf("Tags: %s\n", strings.Join(torrent.Tags, ","))
	fmt.Printf("Meta: %v\n", torrent.Meta)
	fmt.Printf("Add time: %s\n", utils.FormatTime(torrent.Atime))
	fmt.Printf("Completion time: %s\n", ctimeStr)
	fmt.Printf("Tracker: %s\n", torrent.Tracker)
	fmt.Printf("Seeders / Peers: %d / %d\n", torrent.Seeders, torrent.Leechers)
	fmt.Printf("Save path: %s\n", torrent.SavePath)
	fmt.Printf("Content path: %s\n", torrent.ContentPath)
	fmt.Printf("Downloaded / Uploaded: %s / %s\n",
		utils.BytesSize(float64(torrent.Downloaded)),
		utils.BytesSize(float64(torrent.Uploaded)),
	)
}

func PrintTorrents(torrents []Torrent, filter string, showSum bool) {
	cnt := int64(0)
	size := int64(0)
	sizeUnfinished := int64(0)
	fmt.Printf("%-25s  %-40s  %-7s  %-5s  %-8s  %-8s  %-5s  %-5s  %-20s\n", "Name", "InfoHash", "Size", "State", "↓S(/s)", "↑S(/s)", "Seeds", "Peers", "Tracker")
	for _, torrent := range torrents {
		if filter != "" && !utils.ContainsI(torrent.Name, filter) && !utils.ContainsI(torrent.InfoHash, filter) {
			continue
		}
		cnt++
		size += torrent.Size
		name := torrent.Name
		sizeUnfinished += torrent.Size - torrent.SizeCompleted
		utils.PrintStringInWidth(name, 25, true)
		fmt.Printf("  %-40s  %-7s  %-5s  %-8s  %-8s  %-5d  %-5d  %-20s\n",
			torrent.InfoHash,
			utils.BytesSize(float64(torrent.Size)),
			torrent.StateIconText(),
			utils.BytesSize(float64(torrent.DownloadSpeed)),
			utils.BytesSize(float64(torrent.UploadSpeed)),
			torrent.Seeders,
			torrent.Leechers,
			torrent.TrackerDomain,
		)
	}
	if showSum {
		fmt.Printf("// Summary - Cnt / Size / SizeUnfinished: %d / %s / %s", cnt, utils.BytesSize(float64(size)), utils.BytesSize(float64(sizeUnfinished)))
	}
}

// parse and return torrents that meet criterion
// tag: comma-separated list, a torrent will match if it has any tag that in the list
func QueryTorrents(clientInstance Client, category string, tag string, filter string,
	hashOrStateFilters ...string) ([]Torrent, error) {
	isAll := len(hashOrStateFilters) == 0
	for _, arg := range hashOrStateFilters {
		if !IsValidInfoHashOrStateFilter(arg) {
			return nil, fmt.Errorf("%s is not a valid infoHash or stateFilter", arg)
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
	var tags []string
	if tag != "" {
		tags = strings.Split(tag, ",")
	}
	torrents2 := []Torrent{}
	for _, torrent := range torrents {
		if tags != nil && slices.IndexFunc(tags, func(tag string) bool {
			return torrent.HasTag(tag)
		}) == -1 {
			continue
		}
		if filter != "" && !utils.ContainsI(torrent.Name, filter) {
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

// parse torrents that meet criterion and return infoHashes. specially, return nil slice if all torrents selected
// tag: comma-separated list, a torrent will match if it has any tag that in the list
func SelectTorrents(clientInstance Client, category string, tag string, filter string,
	hashOrStateFilters ...string) ([]string, error) {
	isAll := len(hashOrStateFilters) == 0
	for _, arg := range hashOrStateFilters {
		if !IsValidInfoHashOrStateFilter(arg) {
			return nil, fmt.Errorf("%s is not a valid infoHash or stateFilter", arg)
		}
		if arg == "_all" {
			isAll = true
		}
	}
	if category == "" && tag == "" && filter == "" && isAll {
		return nil, nil
	}
	torrents, err := clientInstance.GetTorrents("", category, true)
	if err != nil {
		return nil, err
	}
	var tags []string
	if tag != "" {
		tags = strings.Split(tag, ",")
	}
	infoHashes := []string{}
	for _, torrent := range torrents {
		if tags != nil && slices.IndexFunc(tags, func(tag string) bool {
			return torrent.HasTag(tag)
		}) == -1 {
			continue
		}
		if filter != "" && !utils.ContainsI(torrent.Name, filter) {
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
			return torrent.State == "completed" || torrent.State == "seeding"
		case "_undone":
			return torrent.State == "paused" || torrent.State == "downloading"
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

func IsValidInfoHashOrStateFilter(stateFilter string) bool {
	if strings.HasPrefix(stateFilter, "_") {
		if slices.Index(STATE_FILTERS, stateFilter) != -1 {
			return true
		}
		stateFilter = stateFilter[1:]
		return slices.Index(STATES, stateFilter) != -1
	}
	return IsValidInfoHash(stateFilter)
}

func init() {
}
