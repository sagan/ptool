package site

import (
	"fmt"
	"mime"
	"net/url"
	"slices"
	"sync"
	"time"

	"github.com/Noooste/azuretls-client"
	log "github.com/sirupsen/logrus"

	"github.com/sagan/ptool/config"
	"github.com/sagan/ptool/util"
	"github.com/sagan/ptool/util/crypto"
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
	Neutral            bool // 中性种子：不计算上传、下载、做种魔力
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
	// default sent http request headers
	GetDefaultHttpHeaders() [][]string
	GetSiteConfig() *config.SiteConfigStruct
	// download torrent by id (e.g.: 12345), sitename.id (e.g.: mteam.12345),
	// or absolute download url (e.g.: https://kp.m-team.cc/download.php?id=12345)
	DownloadTorrent(url string) (content []byte, filename string, id string, err error)
	// download torrent by torrent id (e.g.: "12345")
	DownloadTorrentById(id string) (content []byte, filename string, err error)
	GetLatestTorrents(full bool) ([]Torrent, error)
	// sort: size|name|none(or "")
	GetAllTorrents(sort string, desc bool, pageMarker string, baseUrl string) (
		torrents []Torrent, nextPageMarker string, err error)
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
	registryMap  = map[string]*RegInfo{}
	sites        = map[string]Site{}
	siteSessions = map[string]*azuretls.Session{}
	mu           sync.Mutex
)

func (torrent *Torrent) MatchFilter(filter string) bool {
	if filter == "" || util.ContainsI(torrent.Name, filter) || util.ContainsI(torrent.Description, filter) {
		return true
	}
	return false
}

// check if (seems) as a valid site status
func (status *Status) IsOk() bool {
	return status.UserName != "" || status.UserDownloaded > 0 || status.UserUploaded > 0
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
	for _, siteConfig := range config.Get().SitesEnabled {
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

func PrintTorrents(torrents []Torrent, filter string, now int64, noHeader bool, dense bool, scores map[string]float64) {
	if !noHeader {
		if scores == nil {
			fmt.Printf("%-40s  %8s  %-11s  %-19s  %4s  %4s  %4s  %-15s  %2s\n", "Name", "Size", "Free", "Time", "↑S", "↓L", "✓C", "ID", "P")
		} else {
			fmt.Printf("%-40s  %8s  %-11s  %-19s  %4s  %4s  %4s  %-15s  %5s  %2s\n", "Name", "Size", "Free", "Time", "↑S", "↓L", "✓C", "ID", "Score", "P")
		}
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
			freeStr += fmt.Sprintf("(%s)", util.FormatDuration(torrent.DiscountEndTime-now))
		}
		if torrent.UploadMultiplier > 1 {
			freeStr = fmt.Sprintf("%1.1f", torrent.UploadMultiplier) + freeStr
		}
		if torrent.Neutral {
			freeStr += "N"
		} else if torrent.DownloadMultiplier == 0 && torrent.UploadMultiplier == 0 {
			freeStr += "Z"
		}
		name := torrent.Name
		process := "-"
		if torrent.IsActive {
			process = "0%"
		}
		if dense {
			fmt.Printf("// %s  %s\n", torrent.Name, torrent.Description)
		}
		util.PrintStringInWidth(name, 40, true)
		if scores == nil {
			fmt.Printf("  %8s  %-11s  %-19s  %4s  %4s  %4s  %-15s  %2s\n",
				util.BytesSize(float64(torrent.Size)),
				freeStr,
				util.FormatTime(torrent.Time),
				fmt.Sprint(torrent.Seeders),
				fmt.Sprint(torrent.Leechers),
				fmt.Sprint(torrent.Snatched),
				torrent.Id,
				process,
			)
		} else {
			fmt.Printf("  %8s  %-11s  %-19s  %4s  %4s  %4s  %-15s  %5.0f  %2s\n",
				util.BytesSize(float64(torrent.Size)),
				freeStr,
				util.FormatTime(torrent.Time),
				fmt.Sprint(torrent.Seeders),
				fmt.Sprint(torrent.Leechers),
				fmt.Sprint(torrent.Snatched),
				torrent.Id,
				scores[torrent.Id],
				process,
			)
		}
	}
}

func GetConfigSiteNameByDomain(domain string) (string, error) {
	var firstMatchSite, lastMatchSite *config.SiteConfigStruct
	for _, siteConfig := range config.Get().SitesEnabled {
		if config.MatchSite(domain, siteConfig) {
			firstMatchSite = siteConfig
			break
		}
	}
	for i := len(config.Get().SitesEnabled) - 1; i >= 0; i-- {
		if config.MatchSite(domain, config.Get().SitesEnabled[i]) {
			lastMatchSite = config.Get().SitesEnabled[i]
			break
		}
	}
	if firstMatchSite != nil {
		if firstMatchSite == lastMatchSite {
			return firstMatchSite.GetName(), nil
		} else {
			return "", fmt.Errorf("ambiguous result - multiple sites in config match: names of first and last match: %s, %s", firstMatchSite.GetName(), lastMatchSite.GetName())
		}
	}
	return "", nil
}

func GetConfigSiteNameByTypes(types ...string) (string, error) {
	var firstMatchSite, lastMatchSite *config.SiteConfigStruct
	for _, siteConfig := range config.Get().SitesEnabled {
		if slices.Index(types, siteConfig.Type) != -1 {
			firstMatchSite = siteConfig
			break
		}
	}
	for i := len(config.Get().SitesEnabled) - 1; i >= 0; i-- {
		if slices.Index(types, config.Get().SitesEnabled[i].Type) != -1 {
			lastMatchSite = config.Get().SitesEnabled[i]
			break
		}
	}
	if firstMatchSite != nil {
		if firstMatchSite == lastMatchSite {
			return firstMatchSite.GetName(), nil
		} else {
			return "", fmt.Errorf("ambiguous result - multiple sites in config match: names of first and last match: %s, %s", firstMatchSite.GetName(), lastMatchSite.GetName())
		}
	}
	return "", nil
}

func CreateSiteHttpClient(siteConfig *config.SiteConfigStruct, globalConfig *config.ConfigStruct) (
	*azuretls.Session, [][]string, error) {
	var impersonate string
	var impersonateProfile *util.ImpersonateProfile
	var httpHeaders [][]string
	if siteConfig.Impersonate != "" {
		impersonate = siteConfig.Impersonate
	} else if globalConfig.SiteImpersonate != "" {
		impersonate = globalConfig.SiteImpersonate
	}
	if impersonate != "" && impersonate != "none" {
		if ip := util.ImpersonateProfiles[impersonate]; ip == nil {
			return nil, nil, fmt.Errorf("impersonate '%s' not supported", impersonate)
		} else {
			impersonateProfile = ip
		}
	}
	ja3 := ""
	if siteConfig.Ja3 != "" {
		ja3 = siteConfig.Ja3
	} else if globalConfig.SiteJa3 != "" {
		ja3 = globalConfig.SiteJa3
	} else if impersonateProfile != nil {
		ja3 = impersonateProfile.Ja3
	}
	h2fingerprint := ""
	if siteConfig.H2Fingerprint != "" {
		h2fingerprint = siteConfig.H2Fingerprint
	} else if globalConfig.SiteH2Fingerprint != "" {
		h2fingerprint = globalConfig.SiteH2Fingerprint
	} else if impersonateProfile != nil {
		h2fingerprint = impersonateProfile.H2fingerpring
	}
	if impersonateProfile != nil && impersonateProfile.Headers != nil {
		httpHeaders = append(httpHeaders, impersonateProfile.Headers...)
	}
	if globalConfig.SiteHttpHeaders != nil {
		httpHeaders = append(httpHeaders, globalConfig.SiteHttpHeaders...)
	}
	if siteConfig.HttpHeaders != nil {
		httpHeaders = append(httpHeaders, siteConfig.HttpHeaders...)
	}
	proxy := globalConfig.SiteProxy
	if siteConfig.Proxy != "" {
		proxy = siteConfig.Proxy
	}
	if proxy == "" {
		proxy = util.ParseProxyFromEnv(siteConfig.Url)
	}
	// 暂时默认设为 insecure。因为 azuretls 似乎对某些站点(如 byr)的 TLS 证书校验有问题。
	insecure := true
	if globalConfig.SiteSecure {
		insecure = false
	}
	if siteConfig.Insecure && !siteConfig.Secure {
		insecure = true
	}
	timeout := config.DEFAULT_SITE_TIMEOUT
	if siteConfig.Timeoout > 0 {
		timeout = siteConfig.Timeoout
	} else if globalConfig.SiteTimeout > 0 {
		timeout = globalConfig.SiteTimeout
	}
	if ja3 == "none" {
		ja3 = ""
	}
	if h2fingerprint == "none" {
		h2fingerprint = ""
	}
	if proxy == "none" {
		proxy = ""
	}
	sep := "\n"
	specs := fmt.Sprint(ja3, sep, h2fingerprint, sep, proxy, sep, insecure, sep, timeout)
	log.Tracef("Create site %s http client with specs %s", siteConfig.GetName(), specs)
	hash := crypto.Md5String(specs)
	mu.Lock()
	defer mu.Unlock()
	if siteSessions[hash] != nil {
		return siteSessions[hash], httpHeaders, nil
	}
	session := azuretls.NewSession()
	session.SetTimeout(time.Duration(timeout) * time.Second)
	navigator := azuretls.Chrome
	if impersonateProfile != nil {
		navigator = impersonateProfile.Navigator
	}
	if ja3 != "" {
		if err := session.ApplyJa3(ja3, navigator); err != nil {
			return nil, nil, fmt.Errorf("failed to set ja3: %v", err)
		}
	}
	if h2fingerprint != "" {
		if err := session.ApplyHTTP2(h2fingerprint); err != nil {
			return nil, nil, fmt.Errorf("failed to set h2 finterprint: %v", err)
		}
	}
	if proxy != "" {
		if err := session.SetProxy(proxy); err != nil {
			return nil, nil, fmt.Errorf("failed to set proxy: %v", err)
		}
	}
	if insecure {
		session.InsecureSkipVerify = true
	}
	maxRedirects := config.DEFAULT_SITE_MAX_REDIRECTS
	if siteConfig.MaxRedirects != 0 {
		maxRedirects = siteConfig.MaxRedirects
	}
	session.MaxRedirects = uint(maxRedirects)
	siteSessions[hash] = session
	return session, httpHeaders, nil
}

// return site ua from siteConfig and globalConfig
func GetUa(siteInstance Site) string {
	ua := siteInstance.GetSiteConfig().UserAgent
	if ua == "" {
		ua = config.Get().SiteUserAgent
	}
	return ua
}

// General download torrent func. Return torrentContent, filename, err
func DownloadTorrentByUrl(siteInstance Site, httpClient *azuretls.Session, torrentUrl string, torrentId string) (
	[]byte, string, error) {
	res, header, err := util.FetchUrlWithAzuretls(torrentUrl, httpClient,
		siteInstance.GetSiteConfig().Cookie, GetUa(siteInstance), siteInstance.GetDefaultHttpHeaders())
	if err != nil {
		return nil, "", fmt.Errorf("can not fetch torrents from site: %v", err)
	}
	mimeType, _, _ := mime.ParseMediaType(header.Get("content-type"))
	if mimeType != "" && mimeType != "application/octet-stream" && mimeType != "application/x-bittorrent" {
		return nil, "", fmt.Errorf("server return invalid content-type: %s", mimeType)
	}
	filename := ""
	if _, params, err := mime.ParseMediaType(header.Get("content-disposition")); err == nil {
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
	return res.Body, filename, err
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
