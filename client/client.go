package client

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/sagan/ptool/config"
	"github.com/sagan/ptool/utils"
)

type Torrent struct {
	InfoHash           string
	Name               string
	TrackerDomain      string
	State              string // simplifiec state: seeding|downloading|completed|paused
	Atime              int64  // timestamp torrent added
	Ctime              int64  // timestamp torrent completed.
	Category           string
	Tags               []string
	Downloaded         int64
	DownloadSpeed      int64
	DownloadSpeedLimit int64 // -1 means no limit
	Uploaded           int64
	UploadSpeed        int64
	UploadedSpeedLimit int64 // -1 means no limit
	Size               int64
	SizeCompleted      int64
	Seeders            int64
	Leechers           int64
	Meta               map[string](int64)
}

type Status struct {
	FreeSpaceOnDisk    int64 // -1 means unknown / unlimited
	DownloadSpeed      int64
	UploadSpeed        int64
	DownloadSpeedLimit int64 // <= 0 means no limit
	UploadSpeedLimit   int64 // <= 0 means no limit
}

type TorrentOption struct {
	Category           string
	Name               string
	DownloadSpeedLimit int64
	UploadSpeedLimit   int64
	Paused             bool
}

type Client interface {
	GetTorrents(state string, category string, showAll bool) ([]Torrent, error)
	AddTorrent(torrentContent []byte, option *TorrentOption, meta map[string](int64)) error
	ModifyTorrent(infoHash string, option *TorrentOption, meta map[string](int64)) error
	DeleteTorrents(infoHashes []string) error
	TorrentRootPathExists(rootFolder string) bool
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

func TorrentStateIconText(state string) string {
	switch state {
	case "downloading":
		return "↓"
	case "seeding":
		return "↑"
	case "paused":
		return "P" // may be unicode symbol ⏸
	case "completed":
		return "✓"
	}
	return "-"
}
func init() {
}
