package config

import (
	"net/url"
	"os"
	"strings"
	"sync"

	toml "github.com/pelletier/go-toml/v2"
	log "github.com/sirupsen/logrus"
	"gopkg.in/yaml.v3"

	"github.com/sagan/ptool/utils"
)

const (
	BRUSH_CAT = "_brush"
	XSEED_TAG = "_xseed"

	DEFAULT_SITE_TIMEZONE                           = "Asia/Shanghai"
	DEFAULT_CLIENT_BRUSH_MIN_DISK_SPACE             = int64(5 * 1024 * 1024 * 1024)
	DEFAULT_CLIENT_BRUSH_SLOW_UPLOAD_SPEED_TIER     = int64(100 * 1024)
	DEFAULT_CLIENT_BRUSH_MAX_DOWNLOADING_TORRENTS   = int64(6)
	DEFAULT_CLIENT_BRUSH_MAX_TORRENTS               = int64(9999)
	DEFAULT_CLIENT_BRUSH_MIN_RATION                 = float64(0.2)
	DEFAULT_CLIENT_BRUSH_DEFAULT_UPLOAD_SPEED_LIMIT = int64(10 * 1024 * 1024)
	DEFAULT_CLIENT_BRUSH_TORRENT_SIZE_LIMIT         = int64(1024 * 1024 * 1024 * 1024 * 1024) // 1PB, that's say, unlimited
	DEFAULT_SITE_TORRENT_UPLOAD_SPEED_LIMIT         = int64(10 * 1024 * 1024)
)

type GroupConfigStruct struct {
	Name  string   `yaml:"name"`
	Sites []string `yaml:"sites"`
}

type ClientConfigStruct struct {
	Type                              string  `yaml:"type"`
	Name                              string  `yaml:"name"`
	Disabled                          bool    `yaml:"disabled"`
	Url                               string  `yaml:"url"`
	Username                          string  `yaml:"username"`
	Password                          string  `yaml:"password"`
	BrushMinDiskSpace                 string  `yaml:"brushMinDiskSpace"`
	BrushSlowUploadSpeedTier          string  `yaml:"brushSlowUploadSpeedTier"`
	BrushMaxDownloadingTorrents       int64   `yaml:"brushMaxDownloadingTorrents"`
	BrushMaxTorrents                  int64   `yaml:"brushMaxTorrents"`
	BrushMinRatio                     float64 `yaml:"brushMinRatio"`
	BrushDefaultUploadSpeedLimit      string  `yaml:"brushDefaultUploadSpeedLimit"`
	BrushTorrentSizeLimit             string  `yaml:"brushTorrentSizeLimit"`
	BrushMinDiskSpaceValue            int64
	BrushSlowUploadSpeedTierValue     int64
	BrushDefaultUploadSpeedLimitValue int64
	BrushTorrentSizeLimitValue        int64
}

type SiteConfigStruct struct {
	Type                         string   `yaml:"type"`
	Name                         string   `yaml:"name"`
	Aliases                      []string // for internal use only
	Comment                      string   `yaml:"comment"`
	Disabled                     bool     `yaml:"disabled"`
	Url                          string   `yaml:"url"`
	TorrentsUrl                  string   `yaml:"torrentsUrl"`
	SearchUrl                    string   `yaml:"searchUrl"`
	TorrentsExtraUrls            []string `yaml:"torrentsExtraUrls"`
	Cookie                       string   `yaml:"cookie"`
	UserAgent                    string   `yaml:"userAgent"`
	Proxy                        string   `yaml:"proxy"`
	TorrentUploadSpeedLimit      string   `yaml:"uploadSpeedLimit"`
	GlobalHnR                    bool     `yaml:"globalHnR"`
	Timezone                     string   `yaml:"timezone"`
	SelectorTorrentsListHeader   string   `yaml:"selectorTorrentsListHeader"`
	SelectorTorrentsList         string   `yaml:"selectorTorrentsList"`
	SelectorTorrentBlock         string   `yaml:"selectorTorrentBlock"` // dom block of a torrent in list
	SelectorTorrent              string   `yaml:"selectorTorrent"`
	SelectorTorrentDownloadLink  string   `yaml:"selectorTorrentDownloadLink"`
	SelectorTorrentDetailsLink   string   `yaml:"selectorTorrentDetailsLink"`
	SelectorTorrentTime          string   `yaml:"selectorTorrentTime"`
	SelectorTorrentSeeders       string   `yaml:"selectorTorrentSeeders"`
	SelectorTorrentLeechers      string   `yaml:"selectorTorrentLeechers"`
	SelectorTorrentSnatched      string   `yaml:"selectorTorrentSnatched"`
	SelectorTorrentSize          string   `yaml:"selectorTorrentSize"`
	SelectorTorrentProcessBar    string   `yaml:"selectorTorrentProcessBar"`
	SelectorUserInfo             string   `yaml:"selectorUserInfo"`
	SelectorUserInfoUserName     string   `yaml:"selectorUserInfoUserName"`
	SelectorUserInfoUploaded     string   `yaml:"selectorUserInfoUploaded"`
	SelectorUserInfoDownloaded   string   `yaml:"selectorUserInfoDownloaded"`
	TorrentUploadSpeedLimitValue int64
}

type ConfigStruct struct {
	IyuuToken                     string                `yaml:"iyuuToken"`
	SiteProxy                     string                `yaml:"siteProxy"`
	UserAgent                     string                `yaml:"userAgent"`
	BrushEnableStats              bool                  `yaml:"brushEnableStats"`
	TreatZeroFreeDiskSpaceAsError bool                  `yaml:"treatZeroFreeDiskSpaceAsError"`
	Clients                       []*ClientConfigStruct `yaml:"clients"`
	Sites                         []*SiteConfigStruct   `yaml:"sites"`
	Groups                        []*GroupConfigStruct  `yaml:"groups"`
}

var (
	VerboseLevel               = 0
	ConfigDir                  = ""
	ConfigFile                 = ""
	configLoaded               = false
	configData   *ConfigStruct = &ConfigStruct{}
	mu           sync.Mutex
)

func init() {

}

func Get() *ConfigStruct {
	if !configLoaded {
		mu.Lock()
		if !configLoaded {
			log.Debugf("Read config file %s", ConfigFile)
			file, err := os.ReadFile(ConfigFile)
			if err == nil {
				if strings.HasSuffix(ConfigFile, ".yaml") {
					err = yaml.Unmarshal(file, &configData)
					if err != nil {
						log.Fatalf("Error parsing config file: %v", err)
					}
				} else if strings.HasSuffix(ConfigFile, ".toml") {
					err = toml.Unmarshal(file, &configData)
					if err != nil {
						log.Fatalf("Error parsing config file: %v", err)
					}
				} else {
					log.Fatalf("Unsupported config file format. Neither toml nor yaml.")
				}
			}
			for _, client := range configData.Clients {
				v, err := utils.RAMInBytes(client.BrushMinDiskSpace)
				if err != nil || v < 0 {
					v = DEFAULT_CLIENT_BRUSH_MIN_DISK_SPACE
				}
				client.BrushMinDiskSpaceValue = v

				v, err = utils.RAMInBytes(client.BrushSlowUploadSpeedTier)
				if err != nil || v <= 0 {
					v = DEFAULT_CLIENT_BRUSH_SLOW_UPLOAD_SPEED_TIER
				}
				client.BrushSlowUploadSpeedTierValue = v

				v, err = utils.RAMInBytes(client.BrushDefaultUploadSpeedLimit)
				if err != nil || v <= 0 {
					v = DEFAULT_CLIENT_BRUSH_DEFAULT_UPLOAD_SPEED_LIMIT
				}
				client.BrushDefaultUploadSpeedLimitValue = v

				v, err = utils.RAMInBytes(client.BrushTorrentSizeLimit)
				if err != nil || v <= 0 {
					v = DEFAULT_CLIENT_BRUSH_TORRENT_SIZE_LIMIT
				}
				client.BrushTorrentSizeLimitValue = v

				if client.Url != "" {
					urlObj, err := url.Parse(client.Url)
					if err != nil {
						log.Fatalf("Failed to parse client %s url config: %v", client.Name, err)
					}
					client.Url = urlObj.String()
				}

				if client.BrushMaxDownloadingTorrents == 0 {
					client.BrushMaxDownloadingTorrents = DEFAULT_CLIENT_BRUSH_MAX_DOWNLOADING_TORRENTS
				}

				if client.BrushMaxTorrents == 0 {
					client.BrushMaxTorrents = DEFAULT_CLIENT_BRUSH_MAX_TORRENTS
				}

				if client.BrushMinRatio == 0 {
					client.BrushMinRatio = DEFAULT_CLIENT_BRUSH_MIN_RATION
				}

				if client.Name == "" {
					client.Name = client.Type
				}
			}
			for _, site := range configData.Sites {
				v, err := utils.RAMInBytes(site.TorrentUploadSpeedLimit)
				if err != nil || v <= 0 {
					v = DEFAULT_SITE_TORRENT_UPLOAD_SPEED_LIMIT
				}
				site.TorrentUploadSpeedLimitValue = v

				if site.Name == "" {
					site.Name = site.Type
				}

				if site.UserAgent == "" {
					site.UserAgent = configData.UserAgent
				}

				if site.Url != "" {
					urlObj, err := url.Parse(site.Url)
					if err != nil {
						log.Fatalf("Failed to parse site %s url config: %v", site.Name, err)
					}
					site.Url = urlObj.String()
				}

				if site.Timezone == "" {
					site.Timezone = DEFAULT_SITE_TIMEZONE
				}
			}
			configLoaded = true
		}
		configData.Clients = utils.Filter(configData.Clients, func(c *ClientConfigStruct) bool {
			return !c.Disabled
		})
		configData.Sites = utils.Filter(configData.Sites, func(s *SiteConfigStruct) bool {
			return !s.Disabled
		})
		mu.Unlock()
	}
	return configData
}

func GetClientConfig(name string) *ClientConfigStruct {
	for _, client := range Get().Clients {
		if client.Name == name {
			return client
		}
	}
	return nil
}

// if name is a group, return it's sites, otherwise return nil
func GetGroupSites(name string) []string {
	if name == "_all" { // special group of all sites
		sitenames := []string{}
		for _, siteConfig := range Get().Sites {
			if siteConfig.Disabled {
				continue
			}
			sitenames = append(sitenames, siteConfig.GetName())
		}
		return sitenames
	}
	for _, group := range Get().Groups {
		if group.Name == name {
			return group.Sites
		}
	}
	return nil
}

func ParseGroupAndOtherNamesWithoutDeduplicate(names ...string) []string {
	names2 := []string{}
	for _, name := range names {
		groupSites := GetGroupSites(name)
		if groupSites != nil {
			names2 = append(names2, groupSites...)
		} else {
			names2 = append(names2, name)
		}
	}
	return names2
}

// parse an slice of groupOrOther names, expand group name to site names, return the final slice of names
func ParseGroupAndOtherNames(names ...string) []string {
	names = ParseGroupAndOtherNamesWithoutDeduplicate(names...)
	return utils.UniqueSlice(names)
}

func (siteConfig *SiteConfigStruct) GetName() string {
	id := siteConfig.Name
	if id == "" {
		id = siteConfig.Type
	}
	return id
}

// parse a site internal url (eg. special.php), return absolute url
func (siteConfig *SiteConfigStruct) ParseSiteUrl(siteUrl string, appendQueryStringDelimiter bool) string {
	pageUrl := ""
	if siteUrl != "" {
		if utils.IsUrl(siteUrl) {
			pageUrl = siteUrl
		} else {
			if strings.HasPrefix(siteUrl, "/") {
				siteUrl = siteUrl[1:]
			}
			pageUrl = siteConfig.Url + siteUrl
		}
	}

	if appendQueryStringDelimiter {
		if strings.Contains(pageUrl, "?") {
			pageUrl += "&"
		} else {
			pageUrl += "?"
		}
	}
	return pageUrl
}
