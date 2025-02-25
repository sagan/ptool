package config

import (
	"bytes"
	"embed"
	"fmt"
	"io"
	"io/fs"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"sync"

	"github.com/gofrs/flock"
	"github.com/natefinch/atomic"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/viper"

	"github.com/sagan/ptool/constants"
	"github.com/sagan/ptool/util"
)

const (
	METADATA_ARRAY_KEYS        = "tags,narrator"
	BRUSH_CAT                  = "_brush"
	SEEDING_CAT                = "_seeding"
	FALLBACK_CAT               = "Others" // --add-category-auto fallback category if does NOT match with any site
	DYNAMIC_SEEDING_CAT_PREFIX = "dynamic-seeding-"
	XSEED_TAG                  = "_xseed"
	NOADD_TAG                  = "_noadd"
	NODEL_TAG                  = "_nodel"
	TORRENT_NODEL_TAG          = "nodel"
	INVALID_TRACKER_TAG_PREFIX = "_invalid_tracker_"
	TRANSFERRED_TAG            = "_transferred" // transferred to another client
	NOXSEED_TAG                = "noxseed"      // BT 客户端里含有此 tag 的种子不会被辅种
	HR_TAG                     = "_hr"
	PRIVATE_TAG                = "_private"
	PUBLIC_TAG                 = "_public"
	STATS_FILENAME             = "ptool_stats.txt"
	HISTORY_FILENAME           = "ptool_history"
	SITE_TORRENTS_WIDTH        = 120 // min width for printing site torrents
	CLIENT_TORRENTS_WIDTH      = 120 // min width for printing client torrents
	GLOBAL_INTERNAL_LOCK_FILE  = "ptool.lock"
	GLOBAL_LOCK_FILE           = "ptool-global.lock"
	CLIENT_LOCK_FILE           = "client-%s.lock"
	EXAMPLE_CONFIG_FILE        = "ptool.example" // .toml , .yaml

	DEFAULT_EXPORT_TORRENT_RENAME = "{{.name128}}.{{.infohash16}}.torrent"
	// New iyuu API.
	// iyuuplus-dev: https://github.com/ledccn/iyuuplus-dev .
	// Docs: https://doc.iyuu.cn/reference/config .
	DEFAULT_IYUU_DOMAIN                             = "2025.iyuu.cn"
	DEFAULT_TIMEOUT                                 = int64(5)
	DEFAULT_SHELL_MAX_SUGGESTIONS                   = int64(5)
	DEFAULT_SHELL_MAX_HISTORY                       = int64(500)
	DEFAULT_SITE_TIMEZONE                           = "Asia/Shanghai"
	DEFAULT_CLIENT_BRUSH_MIN_DISK_SPACE             = int64(5 * 1024 * 1024 * 1024)
	DEFAULT_CLIENT_BRUSH_SLOW_UPLOAD_SPEED_TIER     = int64(100 * 1024)
	DEFAULT_CLIENT_BRUSH_MAX_DOWNLOADING_TORRENTS   = int64(6)
	DEFAULT_CLIENT_BRUSH_MAX_TORRENTS               = int64(9999)
	DEFAULT_CLIENT_BRUSH_MIN_RATION                 = float64(0.2)
	DEFAULT_CLIENT_BRUSH_DEFAULT_UPLOAD_SPEED_LIMIT = int64(10 * 1024 * 1024)
	DEFAULT_SITE_TIMEOUT                            = DEFAULT_TIMEOUT
	DEFAULT_SITE_BRUSH_TORRENT_MIN_SIZE_LIMIT       = int64(0)
	DEFAULT_SITE_BRUSH_TORRENT_MAX_SIZE_LIMIT       = int64(1024 * 1024 * 1024 * 1024 * 1024) //1PB=effectively no limit
	DEFAULT_SITE_TORRENT_UPLOAD_SPEED_LIMIT         = int64(10 * 1024 * 1024)
	DEFAULT_SITE_FLOW_CONTROL_INTERVAL              = int64(3)
	DEFAULT_SITE_MAX_REDIRECTS                      = int64(3)
	DEFAULT_COOKIECLOUD_TIMEOUT                     = DEFAULT_TIMEOUT
)

type CookiecloudConfigStruct struct {
	Name     string   `yaml:"name"`
	Disabled bool     `yaml:"disabled"`
	Server   string   `yaml:"server"` // CookieCloud API Server Url (with API_ROOT, if exists)
	Uuid     string   `yaml:"uuid"`
	Password string   `yaml:"password"`
	Proxy    string   `yaml:"proxy"`
	Sites    []string `yaml:"sites"`
	Timeout  int64    `yaml:"timeout"`
	Comment  string   `yaml:"comment"`
}

type GroupConfigStruct struct {
	Name    string   `yaml:"name"`
	Sites   []string `yaml:"sites"`
	Comment string   `yaml:"comment"`
}

type AliasConfigStruct struct {
	Name        string `yaml:"name"`
	Cmd         string `yaml:"cmd"`
	DefaultArgs string `yaml:"defaultArgs"`
	MinArgs     int64  `yaml:"minArgs"`
	Comment     string `yaml:"comment"`
	Internal    bool
}

type ClientConfigStruct struct {
	Type     string `yaml:"type"`
	Name     string `yaml:"name"`
	Comment  string `yaml:"comment"`
	Disabled bool   `yaml:"disabled"`
	Url      string `yaml:"url"`
	Username string `yaml:"username"`
	Password string `yaml:"password"`
	// 适用于本机上的 BT 客户端。当前客户端存放种子 (<info-hash>.torrent)的路径。各个客户端默认值：
	// Transmission on Windows: %SystemRoot%\ServiceProfiles\LocalService\AppData\Local\transmission-daemon\Torrents .
	// qBittorrent on Windows: %USERPROFILE%\AppData\Local\qBittorrent\BT_backup .
	// 对于 TR 需要配置此选项才能使用部分命令（例如“导出种子”）。
	// 对于 QB 此配置是可选的(因为 QB web API 提供导出种子接口)，但配置后会提高相关命令的性能。
	LocalTorrentsPath                 string  `yaml:"localTorrentsPath"`
	BrushMinDiskSpace                 string  `yaml:"brushMinDiskSpace"`
	BrushSlowUploadSpeedTier          string  `yaml:"brushSlowUploadSpeedTier"`
	BrushMaxDownloadingTorrents       int64   `yaml:"brushMaxDownloadingTorrents"`
	BrushMaxTorrents                  int64   `yaml:"brushMaxTorrents"`
	BrushMinRatio                     float64 `yaml:"brushMinRatio"`
	BrushDefaultUploadSpeedLimit      string  `yaml:"brushDefaultUploadSpeedLimit"`
	BrushMinDiskSpaceValue            int64
	BrushSlowUploadSpeedTierValue     int64
	BrushDefaultUploadSpeedLimitValue int64 ``
	QbittorrentNoLogin                bool  `yaml:"qbittorrentNoLogin"`  // if set, will NOT send login request
	QbittorrentNoLogout               bool  `yaml:"qbittorrentNoLogout"` // if set, will NOT send logout request
}

type SiteConfigStruct struct {
	Type                           string     `yaml:"type"`
	Name                           string     `yaml:"name"`
	Aliases                        []string   // for internal use only
	Comment                        string     `yaml:"comment"`
	Disabled                       bool       `yaml:"disabled"`
	Hidden                         bool       `yaml:"hidden"` // exclude from default groups (like "_all")
	Dead                           bool       `yaml:"dead"`   // site is (currently) dead.
	Url                            string     `yaml:"url"`
	Domains                        []string   `yaml:"domains"` // other site domains (do not include subdomain part)
	TorrentsUrl                    string     `yaml:"torrentsUrl"`
	SearchUrl                      string     `yaml:"searchUrl"`
	DynamicSeedingTorrentsUrl      string     `yaml:"dynamicSeedingTorrentsUrl"`
	DynamicSeedingExcludes         []string   `yaml:"dynamicSeedingExcludes"`
	DynamicSeedingSize             string     `yaml:"dynamicSeedingSize"`
	DynamicSeedingTorrentMinSize   string     `yaml:"dynamicSeedingTorrentMinSize"`
	DynamicSeedingTorrentMaxSize   string     `yaml:"dynamicSeedingTorrentMaxSize"`
	DynamicSeedingMaxScan          int64      `yaml:"dynamicSeedingMaxScan"`
	DynamicSeedingMinSeeders       int64      `yaml:"dynamicSeedingMinSeeders"`
	DynamicSeedingMaxSeeders       int64      `yaml:"dynamicSeedingMaxSeeders"`
	DynamicSeedingReplaceSeeders   int64      `yaml:"dynamicSeedingReplaceSeeders"`
	SearchQueryVariable            string     `yaml:"searchQueryVariable"`
	TorrentsExtraUrls              []string   `yaml:"torrentsExtraUrls"`
	Cookie                         string     `yaml:"cookie"`
	UserAgent                      string     `yaml:"userAgent"`
	Impersonate                    string     `yaml:"impersonate"`
	HttpHeaders                    [][]string `yaml:"httpHeaders"`
	Ja3                            string     `yaml:"ja3"`
	Timeout                        int64      `yaml:"timeout"`
	H2Fingerprint                  string     `yaml:"h2Fingerprint"`
	Proxy                          string     `yaml:"proxy"`
	Insecure                       bool       `yaml:"insecure"` // 访问站点时强制跳过TLS证书安全校验
	Secure                         bool       `yaml:"secure"`   // 访问站点时强制TLS证书安全校验
	TorrentUploadSpeedLimit        string     `yaml:"torrentUploadSpeedLimit"`
	GlobalHnR                      bool       `yaml:"globalHnR"`
	Timezone                       string     `yaml:"timezone"`
	BrushTorrentMinSizeLimit       string     `yaml:"brushTorrentMinSizeLimit"`
	BrushTorrentMaxSizeLimit       string     `yaml:"brushTorrentMaxSizeLimit"`
	BrushAllowNoneFree             bool       `yaml:"brushAllowNoneFree"`
	BrushAllowPaid                 bool       `yaml:"brushAllowPaid"`
	BrushAllowHr                   bool       `yaml:"brushAllowHr"`
	BrushAllowZeroSeeders          bool       `yaml:"brushAllowZeroSeeders"`
	BrushExcludes                  []string   `yaml:"brushExcludes"`
	BrushExcludeTags               []string   `yaml:"brushExcludeTags"`
	BrushAcceptAnyFree             bool       `yaml:"brushAcceptAnyFree"`
	SelectorTorrentsListHeader     string     `yaml:"selectorTorrentsListHeader"`
	SelectorTorrentsList           string     `yaml:"selectorTorrentsList"`
	SelectorTorrentBlock           string     `yaml:"selectorTorrentBlock"` // dom block of a torrent in list
	SelectorTorrent                string     `yaml:"selectorTorrent"`
	SelectorTorrentDownloadLink    string     `yaml:"selectorTorrentDownloadLink"`
	SelectorTorrentDetailsLink     string     `yaml:"selectorTorrentDetailsLink"`
	SelectorTorrentTime            string     `yaml:"selectorTorrentTime"`
	SelectorTorrentSeeders         string     `yaml:"selectorTorrentSeeders"`
	SelectorTorrentLeechers        string     `yaml:"selectorTorrentLeechers"`
	SelectorTorrentSnatched        string     `yaml:"selectorTorrentSnatched"`
	SelectorTorrentSize            string     `yaml:"selectorTorrentSize"`
	SelectorTorrentActive          string     `yaml:"selectorTorrentActive"`        // Is or was active
	SelectorTorrentCurrentActive   string     `yaml:"selectorTorrentCurrentActive"` // Is currently active
	SelectorTorrentFree            string     `yaml:"selectorTorrentFree"`
	SelectorTorrentNoTraffic       string     `yaml:"selectorTorrentNoTraffic"`
	SelectorTorrentNeutral         string     `yaml:"selectorTorrentNeutral"`
	SelectorTorrentHnR             string     `yaml:"selectorTorrentHnR"`
	SelectorTorrentPaid            string     `yaml:"selectorTorrentPaid"`
	SelectorTorrentDiscountEndTime string     `yaml:"selectorTorrentDiscountEndTime"`
	SelectorUserInfo               string     `yaml:"selectorUserInfo"`
	SelectorUserInfoUserName       string     `yaml:"selectorUserInfoUserName"`
	SelectorUserInfoUploaded       string     `yaml:"selectorUserInfoUploaded"`
	SelectorUserInfoDownloaded     string     `yaml:"selectorUserInfoDownloaded"`
	ImageUploadUrl                 string     `yaml:"imageUploadUrl"`
	// Additional post payload when uploading image, query string format.
	// E.g. "foo=a&bar=b".
	ImageUploadPayload   string     `yaml:"imageUploadPayload"`
	ImageUploadHeaders   [][]string `yaml:"imageUploadHeaders"`
	ImageUploadFileField string     `yaml:"imageUploadFileField"` // Default: "file"
	// Default: "url". May contain dots, e.g. "data.url" will resolve to "res.data.url",
	// where res is the response json object .
	ImageUploadResponseUrlField string `yaml:"mageUploadResponseUrlField"`
	// Payload that will be sent to site when uploading torrent.
	// Key => value. Values are using jinja2 syntax. E.g. "{{title}}".
	// Jinja context variables:
	//   title: Resource title. (this variable is guaranteed to exist, all others are optional).
	//   _text: full description plain text.
	//   _cover: uploaded cover image url.
	// Rendered results will be TrimSpaced.
	// If UploadTorrentPayload is nil, schema dependent default values will be used,
	// and values of UploadTorrentAdditionalPayload will be assigned to previous values.
	// Jinja rendering uses gonja, which, unfortunately, is not fully compatible with python version.
	// For example, {{% if str.startswith("xxx") %}} will NOT work as startswith is a method of python built in string,
	// which is not available in Go. All these methods must be replaced by filters instead.
	UploadTorrentPayload           map[string]string `yaml:"uploadTorrentPayload"`
	UploadTorrentAdditionalPayload map[string]string `yaml:"uploadTorrentAdditionalPayload"`
	// csv, eg: "type". It will check existence of these keys in metadata prior publishing.
	UploadTorrentPayloadRequiredKeys string `yaml:"uploadTorrentPayloadRequiredKeys"`
	TorrentDownloadUrl               string `yaml:"torrentDownloadUrl"` // use {id} placeholders in url
	TorrentDownloadUrlPrefix         string `yaml:"torrentDownloadUrlPrefix"`
	Passkey                          string `yaml:"passkey"`
	UseCuhash                        bool   `yaml:"useCuhash"` // hdcity 使用机制。种子下载地址里必须有cuhash参数
	// ttg 使用机制。种子下载地址末段必须有4位数字校验码或Passkey参数(即使有 Cookie)
	UseDigitHash                      bool   `yaml:"useDigitHash"`
	UsePasskey                        bool   `yaml:"usePasskey"` // 部分站点(例如 ptt)必须使用包含 passkey 的链接下载种子
	TorrentUrlIdRegexp                string `yaml:"torrentUrlIdRegexp"`
	FlowControlInterval               int64  `yaml:"flowControlInterval"` // 暂定名。两次请求种子列表页间隔时间(秒)
	NexusphpNoLetDown                 bool   `yaml:"nexusphpNoLetDown"`
	MaxRedirects                      int64  `yaml:"maxRedirects"`
	NoCookie                          bool   `yaml:"noCookie"`            // true: 该站点不使用 cookie 鉴权方式
	AcceptAnyHttpStatus               bool   `yaml:"acceptAnyHttpStatus"` // true: 非200的http状态不认为是错误
	TorrentUploadSpeedLimitValue      int64
	BrushTorrentMinSizeLimitValue     int64
	BrushTorrentMaxSizeLimitValue     int64
	DynamicSeedingSizeValue           int64
	DynamicSeedingTorrentMinSizeValue int64
	DynamicSeedingTorrentMaxSizeValue int64
	AutoComment                       string // 自动更新 ptool.toml 时系统生成的 comment。会被写入 Comment 字段
	BrushAllowAddTorrentsPercent      int    `yaml:"brushAllowAddTorrentsPercent"` // Site种子数量占比(0~100]: ConfigStruct.BrushMaxTorrents; 0 = no limit
}

type ConfigStruct struct {
	Hushshell           bool                       `yaml:"hushshell"`
	ShellMaxSuggestions int64                      `yaml:"shellMaxSuggestions"` // -1 禁用
	ShellMaxHistory     int64                      `yaml:"shellMaxHistory"`     // -1 禁用
	IyuuToken           string                     `yaml:"iyuuToken"`
	ReseedUsername      string                     `yaml:"reseedUsername"`
	ReseedPassword      string                     `yaml:"reseedPassword"`
	IyuuDomain          string                     `yaml:"iyuuDomain"` // iyuu API 域名。默认使用 2025.iyuu.cn
	SiteProxy           string                     `yaml:"siteProxy"`
	SiteUserAgent       string                     `yaml:"siteUserAgent"`
	SiteImpersonate     string                     `yaml:"siteImpersonate"`
	SiteHttpHeaders     [][]string                 `yaml:"siteHttpHeaders"`
	SiteJa3             string                     `yaml:"siteJa3"`
	SiteTimeout         int64                      `yaml:"siteTimeout"`  // 访问网站超时时间(秒)
	SiteInsecure        bool                       `yaml:"siteInsecure"` // 强制禁用所有站点 TLS 证书校验。
	SiteH2Fingerprint   string                     `yaml:"siteH2Fingerprint"`
	BrushEnableStats    bool                       `yaml:"brushEnableStats"`
	Clients             []*ClientConfigStruct      `yaml:"clients"`
	Sites               []*SiteConfigStruct        `yaml:"sites"`
	Groups              []*GroupConfigStruct       `yaml:"groups"`
	Aliases             []*AliasConfigStruct       `yaml:"aliases"`
	Cookieclouds        []*CookiecloudConfigStruct `yaml:"cookieclouds"`
	Comment             string                     `yaml:"comment"`
	// 公网 BT 种子的分享率(Up/Dl)限制(到达后停止做种)。"add" 等命令添加公网种子到BT客户端时会自动应用此限制。
	// 0 : unlimited。仅 qBittorrent 支持此选项。
	PublicTorrentRatioLimit float64 `yaml:"publicTorrentRatioLimit"`

	ClientsEnabled []*ClientConfigStruct
	SitesEnabled   []*SiteConfigStruct
}

//go:embed ptool.example.toml
//go:embed ptool.example.yaml
var DefaultConfigFs embed.FS

var (
	Timeout               = int64(0) // network(http) timeout. It has the highest priority. Set by --timeout global flag
	VerboseLevel          = 0
	InShell               = false
	ConfigDir             = "" // "/root/.config/ptool"
	ConfigFile            = "" // "ptool.toml"
	DefaultConfigFile     = "" // set when start
	ConfigName            = "" // "ptool"
	ConfigType            = "" // "toml"
	LockFile              = ""
	Proxy                 = "" // proxy. It has the highest priority. Set by --proxy global flag
	Tz                    = "" // override system timezone (TZ) used by the program. Set by --timezone global flag
	GlobalLock            = false
	LockOrExit            = false
	Fork                  = false
	Insecure              = false // Force disable all TLS / https cert verifications. Set by --insecure global flag
	configData            *ConfigStruct
	clientsConfigMap      = map[string]*ClientConfigStruct{}
	sitesConfigMap        = map[string]*SiteConfigStruct{}
	aliasesConfigMap      = map[string]*AliasConfigStruct{}
	groupsConfigMap       = map[string]*GroupConfigStruct{}
	cookiecloudsConfigMap = map[string]*CookiecloudConfigStruct{}
	internalAliasesMap    = map[string]*AliasConfigStruct{}
	once                  sync.Once
)

var InternalAliases = []*AliasConfigStruct{
	{
		Name:        "add2",
		Cmd:         "add --add-category-auto --sequential-download --rename-added",
		DefaultArgs: "*.torrent",
		MinArgs:     1,
		Internal:    true,
	},
	{
		Name:     "batchadd",
		Cmd:      "batchdl --add-category-auto --add-respect-noadd --add-client",
		MinArgs:  1,
		Internal: true,
	},
	{
		Name:     "dlsite",
		Cmd:      "batchdl --download --skip-existing",
		MinArgs:  1,
		Internal: true,
	},
	{
		Name:        "sum",
		Cmd:         "parsetorrent --sum",
		DefaultArgs: "*.torrent",
		Internal:    true,
	},
	{
		Name:        "verify2",
		Cmd:         "verifytorrent --rename-fail",
		DefaultArgs: "*.torrent",
		MinArgs:     1,
		Internal:    true,
	},
}

func init() {
	for _, aliasConfig := range InternalAliases {
		internalAliasesMap[aliasConfig.Name] = aliasConfig
	}
}

// Update configed sites in place, merge the provided (updated) sites with existing config.
func UpdateSites(updatesites []*SiteConfigStruct) {
	if len(updatesites) == 0 {
		return
	}
	allsites := Get().Sites
	ptoolCommentRegex := regexp.MustCompile(`^(.*?)<!--\{ptool\}.*?-->(.*)$`)
	for _, updatesite := range updatesites {
		if updatesite.AutoComment != "" {
			autoComment := fmt.Sprintf(`<!--{ptool} %s-->`, updatesite.AutoComment)
			comment := ptoolCommentRegex.ReplaceAllString(updatesite.Comment, fmt.Sprintf(`$1%s$2`, autoComment))
			if comment == updatesite.Comment {
				updatesite.Comment += autoComment
			} else {
				updatesite.Comment = comment
			}
		}

		updatesite.Register()
		index := slices.IndexFunc(allsites, func(scs *SiteConfigStruct) bool {
			return scs.GetName() == updatesite.GetName()
		})
		if index != -1 {
			util.Assign(allsites[index], updatesite, nil)
		} else {
			allsites = append(allsites, updatesite)
		}
	}
	configData.Sites = allsites
	configData.UpdateSitesDerivative()
}

// Re-write the whole config file using memory data.
// Currently, only sites will be overrided.
// Due to technical limitations, all existing comments will be LOST.
// For now, new config data will NOT take effect for current ptool process.
func Set() error {
	if err := os.MkdirAll(ConfigDir, constants.PERM_DIR); err != nil {
		return fmt.Errorf("config dir does NOT exists and can not be created: %w", err)
	}
	lock, err := LockConfigDirFile(GLOBAL_INTERNAL_LOCK_FILE)
	if err != nil {
		return err
	}
	defer lock.Unlock()
	sites := Get().Sites
	newsites := []map[string]any{}
	for i := range sites {
		newsite := util.StructToMap(*sites[i], true, true)
		newsites = append(newsites, newsite)
	}
	viper.Set("sites", newsites)
	return viper.WriteConfig()
}

func Get() *ConfigStruct {
	once.Do(func() {
		log.Debugf("Read config file %s/%s", ConfigDir, ConfigFile)
		viper.SetConfigName(ConfigName)
		viper.SetConfigType(ConfigType)
		viper.AddConfigPath(ConfigDir)
		err := viper.ReadInConfig()
		if err != nil { // file does NOT exists
			log.Infof("Fail to read config file: %v", err)
		} else {
			err = viper.Unmarshal(&configData)
			if err != nil {
				log.Errorf("Fail to parse config file: %v", err)
			}
		}
		if err != nil {
			configData = &ConfigStruct{}
		}
		if configData.ShellMaxSuggestions == 0 {
			configData.ShellMaxSuggestions = DEFAULT_SHELL_MAX_SUGGESTIONS
		} else if configData.ShellMaxSuggestions < 0 {
			configData.ShellMaxSuggestions = 0
		}
		if configData.ShellMaxHistory == 0 {
			configData.ShellMaxHistory = DEFAULT_SHELL_MAX_HISTORY
		}
		for _, client := range configData.Clients {
			v, err := util.RAMInBytes(client.BrushMinDiskSpace)
			if err != nil || v < 0 {
				v = DEFAULT_CLIENT_BRUSH_MIN_DISK_SPACE
			}
			client.BrushMinDiskSpaceValue = v

			v, err = util.RAMInBytes(client.BrushSlowUploadSpeedTier)
			if err != nil || v <= 0 {
				v = DEFAULT_CLIENT_BRUSH_SLOW_UPLOAD_SPEED_TIER
			}
			client.BrushSlowUploadSpeedTierValue = v

			v, err = util.RAMInBytes(client.BrushDefaultUploadSpeedLimit)
			if err != nil || v <= 0 {
				v = DEFAULT_CLIENT_BRUSH_DEFAULT_UPLOAD_SPEED_LIMIT
			}
			client.BrushDefaultUploadSpeedLimitValue = v

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

			assertConfigItemNameIsValid("client", client.Name, client)
			if clientsConfigMap[client.Name] != nil {
				log.Fatalf("Invalid config file: duplicate client name %s found", client.Name)
			}
			clientsConfigMap[client.Name] = client
		}
		for _, site := range configData.Sites {
			assertConfigItemNameIsValid("site", site.GetName(), site)
			if sitesConfigMap[site.GetName()] != nil {
				log.Fatalf("Invalid config file: duplicate site name %s found", site.GetName())
			}
			site.Register()
		}
		for _, group := range configData.Groups {
			assertConfigItemNameIsValid("group", group.Name, group)
			if groupsConfigMap[group.Name] != nil {
				log.Fatalf("Invalid config file: duplicate group name %s found", group.Name)
			}
			groupsConfigMap[group.Name] = group
		}
		for _, alias := range configData.Aliases {
			assertConfigItemNameIsValid("alias", alias.Name, alias)
			if alias.Name == "alias" {
				log.Fatalf("Invalid config file: alias name can not be 'alias' itself")
			}
			if aliasesConfigMap[alias.Name] != nil {
				log.Fatalf("Invalid config file: duplicate alias name %s found", alias.Name)
			}
			aliasesConfigMap[alias.Name] = alias
		}
		for _, cookiecloud := range configData.Cookieclouds {
			if cookiecloud.Name == "" {
				continue
			}
			if cookiecloudsConfigMap[cookiecloud.Name] != nil {
				log.Fatalf("Invalid config file: duplicate cookiecloud name %s found", cookiecloud.Name)
			}
			cookiecloudsConfigMap[cookiecloud.Name] = cookiecloud
		}
		configData.ClientsEnabled = util.Filter(configData.Clients, func(c *ClientConfigStruct) bool {
			return !c.Disabled
		})
		configData.UpdateSitesDerivative()
	})
	return configData
}

func GetClientConfig(name string) *ClientConfigStruct {
	Get()
	if name == "" {
		return nil
	}
	return clientsConfigMap[name]
}

func GetSiteConfig(name string) *SiteConfigStruct {
	Get()
	if name == "" {
		return nil
	}
	return sitesConfigMap[name]
}

func GetGroupConfig(name string) *GroupConfigStruct {
	Get()
	if name == "" {
		return nil
	}
	return groupsConfigMap[name]
}

func GetAliasConfig(name string) *AliasConfigStruct {
	Get()
	if name == "" {
		return nil
	}
	if aliasesConfigMap[name] != nil {
		return aliasesConfigMap[name]
	}
	return internalAliasesMap[name]
}

func GetCookiecloudConfig(name string) *CookiecloudConfigStruct {
	Get()
	if name == "" {
		return nil
	}
	return cookiecloudsConfigMap[name]
}

// if name is a group, return it's sites, otherwise return nil
func GetGroupSites(name string) []string {
	if name == "_all" { // special group of all sites
		sitenames := []string{}
		for _, siteConfig := range Get().SitesEnabled {
			if siteConfig.Dead || siteConfig.Hidden {
				continue
			}
			sitenames = append(sitenames, siteConfig.GetName())
		}
		return sitenames
	}
	group := GetGroupConfig(name)
	if group != nil {
		return group.Sites
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

// Parse an slice of groupOrOther names, expand group name to site names, return the final slice of names
func ParseGroupAndOtherNames(names ...string) []string {
	names = ParseGroupAndOtherNamesWithoutDeduplicate(names...)
	return util.UniqueSlice(names)
}

func (cookieCloudConfig *CookiecloudConfigStruct) MatchFilter(filter string) bool {
	return util.ContainsI(cookieCloudConfig.Name, filter) || util.ContainsI(cookieCloudConfig.Uuid, filter) ||
		slices.ContainsFunc(cookieCloudConfig.Sites, func(s string) bool {
			return strings.EqualFold(s, filter)
		})
}

func (aliasConfig *AliasConfigStruct) MatchFilter(filter string) bool {
	return util.ContainsI(aliasConfig.Name, filter) || util.ContainsI(aliasConfig.Cmd, filter)
}

func (groupConfig *GroupConfigStruct) MatchFilter(filter string) bool {
	return util.ContainsI(groupConfig.Name, filter) ||
		slices.ContainsFunc(groupConfig.Sites, func(s string) bool {
			return strings.EqualFold(s, filter)
		})
}

func (clientConfig *ClientConfigStruct) MatchFilter(filter string) bool {
	return util.ContainsI(clientConfig.Name, filter) || util.ContainsI(clientConfig.Url, filter)
}

// Generate derivative info from site config and register itself
func (siteConfig *SiteConfigStruct) Register() {
	v, err := util.RAMInBytes(siteConfig.TorrentUploadSpeedLimit)
	if err != nil || v <= 0 {
		v = DEFAULT_SITE_TORRENT_UPLOAD_SPEED_LIMIT
	}
	siteConfig.TorrentUploadSpeedLimitValue = v

	if siteConfig.Url != "" {
		urlObj, err := url.Parse(siteConfig.Url)
		if err != nil {
			log.Fatalf("Failed to parse site %s url config: %v", siteConfig.GetName(), err)
		}
		siteConfig.Url = urlObj.String()
	}

	v, err = util.RAMInBytes(siteConfig.BrushTorrentMinSizeLimit)
	if err != nil || v <= 0 {
		v = DEFAULT_SITE_BRUSH_TORRENT_MIN_SIZE_LIMIT
	}
	siteConfig.BrushTorrentMinSizeLimitValue = v

	v, err = util.RAMInBytes(siteConfig.BrushTorrentMaxSizeLimit)
	if err != nil || v <= 0 {
		v = DEFAULT_SITE_BRUSH_TORRENT_MAX_SIZE_LIMIT
	}
	siteConfig.BrushTorrentMaxSizeLimitValue = v

	if siteConfig.DynamicSeedingSize != "" {
		if v, err = util.RAMInBytes(siteConfig.DynamicSeedingSize); err != nil || v < 0 {
			log.Fatalf("Invalid dynamicSeedingSize value %q in site config: %v", siteConfig.DynamicSeedingSize, err)
		}
		siteConfig.DynamicSeedingSizeValue = v
	}

	if siteConfig.DynamicSeedingTorrentMaxSize != "" {
		if v, err = util.RAMInBytes(siteConfig.DynamicSeedingTorrentMaxSize); err != nil {
			log.Fatalf("Invalid dynamicSeedingTorrentMaxSize value %q in site config: %v",
				siteConfig.DynamicSeedingTorrentMaxSize, err)
		}
		siteConfig.DynamicSeedingTorrentMaxSizeValue = v
	}

	if siteConfig.DynamicSeedingTorrentMinSize != "" {
		if v, err = util.RAMInBytes(siteConfig.DynamicSeedingTorrentMinSize); err != nil {
			log.Fatalf("Invalid dynamicSeedingTorrentMinSize value %q in site config: %v",
				siteConfig.DynamicSeedingTorrentMinSize, err)
		}
		siteConfig.DynamicSeedingTorrentMinSizeValue = v
	}

	if siteConfig.BrushAllowAddTorrentsPercent < 0 || siteConfig.BrushAllowAddTorrentsPercent > 100 {
		log.Fatalf("Invalid allowAddTorrentsPercent value %v in site config, should between [0, 100]", siteConfig.BrushAllowAddTorrentsPercent)
	}

	sitesConfigMap[siteConfig.GetName()] = siteConfig
}

func (siteConfig *SiteConfigStruct) GetName() string {
	id := siteConfig.Name
	if id == "" {
		id = siteConfig.Type
	}
	return id
}

func (siteConfig *SiteConfigStruct) GetTimezone() string {
	tz := siteConfig.Timezone
	if tz == "" {
		tz = DEFAULT_SITE_TIMEZONE
	}
	return tz
}

func (siteConfig *SiteConfigStruct) MatchFilter(filter string) bool {
	return util.ContainsI(siteConfig.GetName(), filter) || util.ContainsI(siteConfig.Type, filter) ||
		util.ContainsI(siteConfig.Url, filter) || util.ContainsI(siteConfig.Comment, filter)
}

// Parse a site internal url (e.g. special.php), return absolute url
func (siteConfig *SiteConfigStruct) ParseSiteUrl(siteUrl string, appendQueryStringDelimiter bool) string {
	pageUrl := ""
	if siteUrl != "" {
		if util.IsUrl(siteUrl) {
			pageUrl = siteUrl
		} else {
			siteUrl = strings.TrimPrefix(siteUrl, "/")
			pageUrl = strings.TrimSuffix(siteConfig.Url, "/") + "/" + siteUrl
		}
	}

	if appendQueryStringDelimiter {
		pageUrl = util.AppendUrlQueryStringDelimiter(pageUrl)
	}
	return pageUrl
}

func MatchSite(domain string, siteConfig *SiteConfigStruct) bool {
	if domain == "" {
		return false
	}
	if siteConfig.Url != "" {
		siteDomain := util.GetUrlDomain(siteConfig.Url)
		if domain == siteDomain || strings.HasSuffix(domain, "."+siteDomain) {
			return true
		}
	}
	for _, siteDomain := range siteConfig.Domains {
		if siteDomain == domain || strings.HasSuffix(domain, "."+siteDomain) {
			return true
		}
	}
	return false
}

func (configData *ConfigStruct) UpdateSitesDerivative() {
	configData.SitesEnabled = util.Filter(configData.Sites, func(s *SiteConfigStruct) bool {
		return !s.Disabled
	})
}

func (configData *ConfigStruct) GetIyuuDomain() string {
	if configData.IyuuDomain == "" {
		return DEFAULT_IYUU_DOMAIN
	}
	return configData.IyuuDomain
}

func CreateDefaultConfig() (err error) {
	if err := os.MkdirAll(ConfigDir, constants.PERM_DIR); err != nil {
		return fmt.Errorf("failed to create config dir: %w", err)
	}
	lock, err := LockConfigDirFile(GLOBAL_INTERNAL_LOCK_FILE)
	if err != nil {
		return err
	}
	defer lock.Unlock()
	configFile := filepath.Join(ConfigDir, ConfigFile)
	if _, err := os.Stat(configFile); !os.IsNotExist(err) {
		if err == nil {
			return fmt.Errorf("config file already exists")
		}
		return fmt.Errorf("failed to access config file: %w", err)
	}
	var file fs.File
	if file, err = DefaultConfigFs.Open(EXAMPLE_CONFIG_FILE + "." + ConfigType); err != nil {
		return fmt.Errorf("unsupported config file type %q: %w", ConfigType, err)
	}
	contents, err := io.ReadAll(file)
	if err != nil {
		panic(err)
	}
	return atomic.WriteFile(configFile, bytes.NewReader(contents))
}

// Assert name is neither empty nor contains invalid characters. If failed, exit the process
func assertConfigItemNameIsValid(itemType string, name string, item any) {
	if name == "" {
		log.Fatalf("Invalid config: %s name can not be empty (item=%v)", itemType, item)
	}
	if strings.ContainsAny(name, `,.:;'"/\<>[]{}|`) {
		log.Fatalf("Invalid config: %s name %s contains invalid characters (item=%v)", itemType, name, item)
	}
}

// Get effective proxy, following the orders:
// Proxy (set by cmdline --proxy flag), proxies...
func GetProxy(proxies ...string) string {
	if Proxy != "" {
		return Proxy
	}
	for _, proxy := range proxies {
		if proxy != "" {
			return proxy
		}
	}
	return ""
}

// Lock the file with provided name in config dir.
func LockConfigDirFile(name string) (*flock.Flock, error) {
	lock := flock.New(filepath.Join(ConfigDir, name))
	if ok, err := lock.TryLock(); err != nil || !ok {
		return nil, fmt.Errorf("unable to acquire lock <config_dir>/%q: %w", name, err)
	}
	return lock, nil
}
