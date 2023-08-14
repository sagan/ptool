package site

import (
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"sync"

	"github.com/sagan/ptool/config"
	"github.com/sagan/ptool/ja3transport"
	"github.com/sagan/ptool/utils"
	"golang.org/x/exp/slices"
)

type Torrent struct {
	Name               string
	Description        string
	Id                 string // optional torrent id in the site
	InfoHash           string
	DownloadUrl        string
	DownloadMultiplier float64
	UploadMultiplier   float64
	DiscountEndTime    int64
	Time               int64 // torrent timestamp
	Size               int64
	IsSizeAccurate     bool
	Seeders            int64
	Leechers           int64
	Snatched           int64
	HasHnR             bool // true if has any type of HR
	IsActive           bool // true if torrent is as already downloading / seeding
	Paid               bool // "付费"种子: (第一次)下载或汇报种子时扣除魔力/积分
	Bought             bool // 适用于付费种子：已购买
}

type Status struct {
	UserName            string
	UserDownloaded      int64
	UserUploaded        int64
	TorrentsSeedingCnt  int64
	TorrentsLeechingCnt int64
}

type Site interface {
	GetName() string
	GetSiteConfig() *config.SiteConfigStruct
	// download torrent by id (eg. 12345), sitename.id (eg. mteam.12345), or torrent download url (eg. https://kp.m-team.cc/download.php?id=12345)
	DownloadTorrent(url string) (content []byte, filename string, err error)
	// download torrent by torrent id (eg. 12345)
	DownloadTorrentById(id string) (content []byte, filename string, err error)
	GetLatestTorrents(full bool) ([]Torrent, error)
	// sort: size|name|none(or "")
	GetAllTorrents(sort string, desc bool, pageMarker string, baseUrl string) (torrents []Torrent, nextPageMarker string, err error)
	SearchTorrents(keyword string, baseUrl string) ([]Torrent, error)
	GetStatus() (*Status, error)
	PurgeCache()
}

type RegInfo struct {
	Name    string
	Aliases []string
	Creator func(string, *config.SiteConfigStruct, *config.ConfigStruct) (Site, error)
}

type SiteCreator func(*RegInfo) (Site, error)

var (
	registryMap = map[string](*RegInfo){}
	sites       = map[string](Site){}
)

func (torrent *Torrent) MatchFilter(filter string) bool {
	if filter == "" || utils.ContainsI(torrent.Name, filter) || utils.ContainsI(torrent.Description, filter) {
		return true
	}
	return false
}

// will match if any filter in list matches
func (torrent *Torrent) MatchFiltersOr(filters []string) bool {
	return slices.IndexFunc(filters, func(filter string) bool {
		return torrent.MatchFilter(filter)
	}) != -1
}

func Register(regInfo *RegInfo) {
	registryMap[regInfo.Name] = regInfo
	for _, alias := range regInfo.Aliases {
		registryMap[alias] = regInfo
	}
}

func CreateSiteInternal(name string,
	siteConfig *config.SiteConfigStruct, config *config.ConfigStruct) (Site, error) {
	regInfo := registryMap[siteConfig.Type]
	if regInfo == nil {
		return nil, fmt.Errorf("unsupported site type %s", name)
	}
	return regInfo.Creator(name, siteConfig, config)
}

func GetConfigSiteReginfo(name string) *RegInfo {
	for _, siteConfig := range config.Get().Sites {
		if siteConfig.GetName() == name {
			return registryMap[siteConfig.Type]
		}
	}
	return nil
}

func SiteExists(name string) bool {
	siteConfig := config.GetSiteConfig(name)
	return siteConfig != nil
}

func CreateSite(name string) (Site, error) {
	if sites[name] != nil {
		return sites[name], nil
	}
	siteConfig := config.GetSiteConfig(name)
	if siteConfig == nil {
		return nil, fmt.Errorf("site %s not found", name)
	}
	siteInstance, err := CreateSiteInternal(name, siteConfig, config.Get())
	if err != nil {
		sites[name] = siteInstance
	}
	return siteInstance, err
}

func PrintTorrents(torrents []Torrent, filter string, now int64, noHeader bool, dense bool) {
	if !noHeader {
		fmt.Printf("%-40s  %8s  %-13s  %-19s  %4s  %4s  %4s  %20s  %2s\n", "Name", "Size", "Free", "Time", "↑S", "↓L", "✓C", "ID", "P")
	}
	for _, torrent := range torrents {
		if filter != "" && !torrent.MatchFilter(filter) {
			continue
		}
		freeStr := ""
		if torrent.HasHnR {
			freeStr += "!"
		}
		if torrent.Paid {
			freeStr += "$"
		}
		if torrent.DownloadMultiplier == 0 {
			freeStr += "✓"
		} else {
			freeStr += "✕"
		}
		if torrent.DiscountEndTime > 0 {
			freeStr += fmt.Sprintf("(%s)", utils.FormatDuration(torrent.DiscountEndTime-now))
		}
		if torrent.UploadMultiplier > 1 {
			freeStr = fmt.Sprintf("%1.1f", torrent.UploadMultiplier) + freeStr
		}
		name := torrent.Name
		process := "-"
		if torrent.IsActive {
			process = "0%"
		}
		if dense {
			fmt.Printf("// %s  %s\n", torrent.Name, torrent.Description)
		}
		utils.PrintStringInWidth(name, 40, true)
		fmt.Printf("  %8s  %-13s  %-19s  %4s  %4s  %4s  %20s  %2s\n",
			utils.BytesSize(float64(torrent.Size)),
			freeStr,
			utils.FormatTime(torrent.Time),
			fmt.Sprint(torrent.Seeders),
			fmt.Sprint(torrent.Leechers),
			fmt.Sprint(torrent.Snatched),
			torrent.Id,
			process,
		)
	}
}

func GetConfigSiteNameByDomain(domain string) string {
	for _, siteConfig := range config.Get().Sites {
		if config.MatchSite(domain, siteConfig) {
			return siteConfig.Name
		}
	}
	return ""
}

func GetConfigSiteNameByTypes(types ...string) string {
	for _, siteConfig := range config.Get().Sites {
		if slices.Index(types, siteConfig.Type) != -1 {
			return siteConfig.GetName()
		}
	}
	return ""
}

func CreateSiteHttpClient(siteConfig *config.SiteConfigStruct, config *config.ConfigStruct) (*http.Client, error) {
	httpClient := &http.Client{}
	ja3 := ""
	// ja3 = utils.CHROME_JA3 // there are still some SERIOUS problems unsolved for now.
	if siteConfig.Ja3 != "" {
		ja3 = siteConfig.Ja3
	}
	var transport *http.Transport
	var err error
	if ja3 != "" && ja3 != "none" {
		transport, err = ja3transport.NewTransport(ja3)
		if err != nil {
			return nil, fmt.Errorf("failed to create site http transport ja3: %v", err)
		}
		transport.ForceAttemptHTTP2 = true
	} else {
		transport = &http.Transport{}
	}
	if siteConfig.Proxy != "" && siteConfig.Proxy != "none" {
		proxyUrl, err := url.Parse(siteConfig.Proxy)
		if err != nil {
			return nil, fmt.Errorf("failed to parse siteProxy %s: %v", siteConfig.Proxy, err)
		}
		transport.Proxy = http.ProxyURL(proxyUrl)
	}
	httpClient.Transport = transport
	return httpClient, nil
}

// general download torrent func
func DownloadTorrentByUrl(siteInstance Site, httpClient *http.Client, torrentUrl string, torrentId string) ([]byte, string, error) {
	res, header, err := utils.FetchUrl(torrentUrl, httpClient,
		siteInstance.GetSiteConfig().Cookie, siteInstance.GetSiteConfig().UserAgent, nil)
	if err != nil {
		return nil, "", fmt.Errorf("can not fetch torrents from site: %v", err)
	}
	mimeType, _, _ := mime.ParseMediaType(header.Get("content-type"))
	if mimeType != "" && mimeType != "application/octet-stream" && mimeType != "application/x-bittorrent" {
		return nil, "", fmt.Errorf("server return invalid content-type: %s", mimeType)
	}
	filename := ""
	_, params, err := mime.ParseMediaType(header.Get("content-disposition"))
	if err == nil {
		unescapedFilename, err := url.QueryUnescape(params["filename"])
		if err == nil {
			filename = unescapedFilename
		}
	}
	filenamePrefix := siteInstance.GetName()
	if torrentId != "" {
		filenamePrefix += "." + torrentId
	}
	if filename != "" {
		filename = fmt.Sprintf("%s.%s", filenamePrefix, filename)
	} else {
		filename = fmt.Sprintf("%s.torrent", filenamePrefix)
	}

	defer res.Body.Close()
	data, err := io.ReadAll(res.Body)
	return data, filename, err
}

func init() {
}

// called by main codes on program exit. clean resources
func Exit() {
	var resourcesWaitGroup sync.WaitGroup
	// for now, nothing to close
	for siteName := range sites {
		delete(sites, siteName)
	}
	resourcesWaitGroup.Wait()
}

// Purge site cache
func Purge(siteName string) {
	if siteName == "" {
		for _, siteInstance := range sites {
			siteInstance.PurgeCache()
		}
	} else if sites[siteName] != nil {
		sites[siteName].PurgeCache()
	}
}
