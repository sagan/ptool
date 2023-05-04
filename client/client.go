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
	State              string // simplified state: seeding|downloading|completed|paused|checking|<any others>...
	LowLevelState      string // original state value returned by bt client
	Atime              int64  // timestamp torrent added
	Ctime              int64  // timestamp torrent completed. <=0 if not completed.
	Category           string
	SavePath           string
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
	PauseAllTorrents() error
	ResumeAllTorrents() error
	RecheckAllTorrents() error
	ReannounceAllTorrents() error
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
		process := int64(float64(torrent.SizeCompleted) / float64(torrent.Size) * 100)
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

func PrintTorrents(torrents []Torrent, filter string) {
	fmt.Printf("%-40s  %40s  %10s  %6s  %12s  %12s  %25s\n", "Name", "InfoHash", "Size", "State", "↓S", "↑S", "Tracker")
	for _, torrent := range torrents {
		if filter != "" && !utils.ContainsI(torrent.Name, filter) && !utils.ContainsI(torrent.InfoHash, filter) {
			continue
		}
		name := torrent.Name
		utils.PrintStringInWidth(name, 40, true)
		fmt.Printf("  %40s  %10s  %6s  %10s/s  %10s/s  %25s\n",
			torrent.InfoHash,
			utils.BytesSize(float64(torrent.Size)),
			TorrentStateIconText(&torrent),
			utils.BytesSize(float64(torrent.DownloadSpeed)),
			utils.BytesSize(float64(torrent.UploadSpeed)),
			torrent.TrackerDomain,
		)
	}
}

// return 0 if equal; 1 if clientTorrentContents contains all files of torrentContents. -1 in other cases
func XseedCheckTorrentContents(clientTorrentContents []TorrentContentFile, torrentContents []*goTorrentParser.File) int64 {
	if len(clientTorrentContents) < len(torrentContents) {
		return -1
	}
	length := len(torrentContents)
	for i := 0; i < length; i++ {
		if clientTorrentContents[i].Path != strings.Join(torrentContents[i].Path, "/") ||
			clientTorrentContents[i].Size != torrentContents[i].Length {
			return -1
		}
	}
	if length < len(clientTorrentContents) {
		return 1
	}
	return 0
}

// parse torrents that meet criterion. specially, return nil slice if all torrents selected
func SelectTorrents(clientInstance Client, category string, tag string, filter string,
	hashOrStateFilters ...string) ([]string, error) {
	if slices.Index(hashOrStateFilters, "_all") != -1 {
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
	return stateFilter != torrent.State
}
