package client

import (
	"fmt"
	"regexp"
	"strings"

	goTorrentParser "github.com/j-muller/go-torrent-parser"
	"golang.org/x/exp/slices"

	"github.com/sagan/ptool/config"
	"github.com/sagan/ptool/utils"
)

type Torrent struct {
	InfoHash           string
	Name               string
	TrackerDomain      string
	Tracker            string
	State              string // simplified state: seeding|downloading|completed|paused|checking|<any others>...
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
	FreeSpaceOnDisk    int64 // -1 means unknown
	DownloadSpeed      int64
	UploadSpeed        int64
	DownloadSpeedLimit int64 // <= 0 means no limit
	UploadSpeedLimit   int64 // <= 0 means no limit
	NoAdd              bool  // if true, brush and other tasks will NOT add any torrents to client
}

type TorrentTracker struct {
	Status string //working|notcontacted|error|updating|disabled|unknown
	Url    string
	Msg    string
}

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

type Client interface {
	GetTorrent(infoHash string) (*Torrent, error)
	// stateFilter: _all|_active|_done, or any state value (possibly with a _ prefix)
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
	GetCategories() ([]string, error)
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
	GetTorrentTrackers(infoHash string) ([]TorrentTracker, error)
	EditTorrentTracker(infoHash string, oldTracker string, newTracker string) error
}

type RegInfo struct {
	Name    string
	Creator func(string, *config.ClientConfigStruct, *config.ConfigStruct) (Client, error)
}

type ClientCreator func(*RegInfo) (Client, error)

var (
	Registry []*RegInfo = make([]*RegInfo, 0)
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
		meta = make(map[string]int64)
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

func TorrentStateIconText(torrent *Torrent) string {
	switch torrent.State {
	case "downloading":
		process := int64(float64(torrent.SizeCompleted) * 100 / float64(torrent.Size))
		return fmt.Sprint("↓", process, "%")
	case "seeding":
		return "↑U"
	case "paused":
		return "-P" // may be unicode symbol ⏸
	case "completed":
		return "✓C"
	case "checking":
		return "→c"
	case "error":
		return "!e"
	}
	return "-"
}
func init() {
}

func (torrent *Torrent) GetSiteFromTag() string {
	for _, tag := range torrent.Tags {
		if strings.HasPrefix(tag, "site:") {
			return tag[5:]
		}
	}
	return ""
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

func GenerateTorrentTagFromSite(site string) string {
	return "site:" + site
}

func PrintTorrentTrackers(trackers []TorrentTracker) {
	fmt.Printf("Trackers:\n")
	fmt.Printf("%-8s  %-30s  %s\n", "Status", "Msg", "Url")
	for _, tracker := range trackers {
		fmt.Printf("%-8s  ", tracker.Status)
		utils.PrintStringInWidth(tracker.Msg, 30, true)
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
	fmt.Printf("Save path: %s\n", torrent.SavePath)
	fmt.Printf("Content path: %s\n", torrent.ContentPath)
	fmt.Printf("Downloaded / Uploaded: %s / %s\n",
		utils.BytesSize(float64(torrent.Downloaded)),
		utils.BytesSize(float64(torrent.Uploaded)),
	)
}

func PrintTorrents(torrents []Torrent, filter string) {
	fmt.Printf("%-25s  %-40s  %-7s  %-5s  %-8s  %-8s  %-20s\n", "Name", "InfoHash", "Size", "State", "↓S (/s)", "↑S (/s)", "Tracker")
	for _, torrent := range torrents {
		if filter != "" && !utils.ContainsI(torrent.Name, filter) && !utils.ContainsI(torrent.InfoHash, filter) {
			continue
		}
		name := torrent.Name
		utils.PrintStringInWidth(name, 25, true)
		fmt.Printf("  %-40s  %-7s  %-5s  %-8s  %-8s  %-20s\n",
			torrent.InfoHash,
			utils.BytesSize(float64(torrent.Size)),
			TorrentStateIconText(&torrent),
			utils.BytesSize(float64(torrent.DownloadSpeed)),
			utils.BytesSize(float64(torrent.UploadSpeed)),
			torrent.TrackerDomain,
		)
	}
}

// return 0 if equal; 1 if clientTorrentContents contains all files of torrentContents.
// return -2 if the ROOT folder(file) of two torrents are different, but all innner files are SAME.
// return -1 if contents of two torrents are NOT same.
// inputed slices of filenames MUST be lexicographic ordered.
func XseedCheckTorrentContents(clientTorrentContents []TorrentContentFile, torrentContents []*goTorrentParser.File) int64 {
	if len(clientTorrentContents) < len(torrentContents) || len(torrentContents) == 0 {
		return -1
	}
	length := len(torrentContents)
	leftContainerFolder := ""
	rightContainerFolder := ""
	leftNoContainerFolder := false
	rightNoContainerFolder := false
	for i := 0; i < length; i++ {
		if clientTorrentContents[i].Size != torrentContents[i].Length {
			return -1
		}
		leftPathes := strings.Split(clientTorrentContents[i].Path, "/")
		if !leftNoContainerFolder && !rightNoContainerFolder {
			_leftContainerFolder := leftPathes[0]
			if leftContainerFolder == "" {
				leftContainerFolder = _leftContainerFolder
			} else if leftContainerFolder != _leftContainerFolder {
				leftNoContainerFolder = true
			}
			_rightContainerFolder := torrentContents[i].Path[0]
			if rightContainerFolder == "" {
				rightContainerFolder = _rightContainerFolder
			} else if rightContainerFolder != _rightContainerFolder {
				rightNoContainerFolder = true
			}
		}
		if clientTorrentContents[i].Path != strings.Join(torrentContents[i].Path, "/") {
			if !leftNoContainerFolder && !rightNoContainerFolder {
				leftPath := strings.Join(leftPathes[1:], "/")
				rightPath := strings.Join(torrentContents[i].Path[1:], "/")
				if leftPath == rightPath {
					continue
				}
			}
			return -1
		}
	}
	if !leftNoContainerFolder && !rightNoContainerFolder && leftContainerFolder != rightContainerFolder {
		return -2
	}
	// it's somewhat broken for now
	if length < len(clientTorrentContents) {
		return 1
	}
	return 0
}

func QueryTorrents(clientInstance Client, category string, tag string, filter string,
	hashOrStateFilters ...string) ([]Torrent, error) {
	torrents, err := clientInstance.GetTorrents("", category, true)
	if err != nil {
		return nil, err
	}
	if slices.Index(hashOrStateFilters, "_all") != -1 {
		return torrents, nil
	}
	torrents = utils.Filter(torrents, func(torrent Torrent) bool {
		if tag != "" && !torrent.HasTag(tag) {
			return false
		}
		if filter != "" && !utils.ContainsI(torrent.Name, filter) {
			return false
		}
		return true
	})
	if len(hashOrStateFilters) == 0 {
		return torrents, nil
	}

	torrents2 := []Torrent{}
	for _, arg := range hashOrStateFilters {
		if strings.HasPrefix(arg, "_") {
			for _, torrent := range torrents {
				if torrent.MatchStateFilter(arg) {
					torrents2 = append(torrents2, torrent)
				}
			}
		} else {
			torrent, err := clientInstance.GetTorrent(arg)
			if err == nil && torrent != nil {
				torrents2 = append(torrents2, *torrent)
			}
		}
	}
	torrents2 = utils.UniqueSliceFn(torrents2, func(t Torrent) string {
		return t.InfoHash
	})

	return torrents2, nil
}

// parse torrents that meet criterion. specially, return nil slice if all torrents selected
func SelectTorrents(clientInstance Client, category string, tag string, filter string,
	hashOrStateFilters ...string) ([]string, error) {
	if slices.Index(hashOrStateFilters, "_all") != -1 {
		return nil, nil
	}
	if category == "" && tag == "" && filter == "" && len(hashOrStateFilters) == 0 {
		return nil, nil
	}

	torrents, err := clientInstance.GetTorrents("", category, true)
	if err != nil {
		return nil, err
	}
	torrents = utils.Filter(torrents, func(torrent Torrent) bool {
		if tag != "" && !torrent.HasTag(tag) {
			return false
		}
		if filter != "" && !utils.ContainsI(torrent.Name, filter) {
			return false
		}
		return true
	})

	if len(hashOrStateFilters) == 0 {
		infoHashes := utils.Map(torrents, func(t Torrent) string {
			return t.InfoHash
		})
		return infoHashes, nil
	}

	infoHashes := []string{}
	for _, arg := range hashOrStateFilters {
		if strings.HasPrefix(arg, "_") {
			for _, torrent := range torrents {
				if torrent.MatchStateFilter(arg) {
					infoHashes = append(infoHashes, torrent.InfoHash)
				}
			}
		} else {
			infoHashes = append(infoHashes, arg)
		}
	}
	infoHashes = utils.UniqueSlice(infoHashes)
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
		default:
			stateFilter = stateFilter[1:]
		}
	}
	return stateFilter == torrent.State
}
