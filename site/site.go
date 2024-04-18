package site

import (
	"fmt"
	"mime"
	"os"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/Noooste/azuretls-client"
	log "github.com/sirupsen/logrus"
	"golang.org/x/term"

	"github.com/sagan/ptool/config"
	"github.com/sagan/ptool/constants"
	"github.com/sagan/ptool/util"
	"github.com/sagan/ptool/util/crypto"
	"github.com/sagan/ptool/util/impersonateutil"
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
	// or absolute download url (e.g.: https://kp.m-team.cc/download.php?id=12345).
	DownloadTorrent(url string) (content []byte, filename string, id string, err error)
	// download torrent by torrent id (e.g.: "12345")
	DownloadTorrentById(id string) (content []byte, filename string, err error)
	GetLatestTorrents(full bool) ([]Torrent, error)
	// sort: size|name|none(or "")
	GetAllTorrents(sort string, desc bool, pageMarker string, baseUrl string) (
		torrents []Torrent, nextPageMarker string, err error)
	// can use "%s" as keyword placeholder in baseUrl
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

// Check if (seems) as a valid site status
func (status *Status) IsOk() bool {
	return status.UserName != "" || status.UserDownloaded > 0 || status.UserUploaded > 0
}

// Matches if any filter in list matches
func (torrent *Torrent) MatchFiltersOr(filters []string) bool {
	return slices.ContainsFunc(filters, func(filter string) bool {
		return torrent.MatchFilter(filter)
	})
}

// Matches if every list of filtersArray is successed with MatchFiltersOr().
// If filtersArray is empty, return true.
func (torrent *Torrent) MatchFiltersAndOr(filtersArray [][]string) bool {
	matched := true
	for _, includes := range filtersArray {
		if !torrent.MatchFiltersOr(includes) {
			matched = false
			break
		}
	}
	return matched
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

func PrintTorrents(torrents []Torrent, filter string, now int64,
	noHeader bool, dense bool, scores map[string]float64) {
	width, _, _ := term.GetSize(int(os.Stdout.Fd()))
	if width < config.SITE_TORRENTS_WIDTH {
		width = config.SITE_TORRENTS_WIDTH
	}
	widthExcludingName := 0
	widthName := 0
	if scores == nil {
		widthExcludingName = 81 // 6+11+19+4+4+4+15+2+8*2
		widthName = width - widthExcludingName
		if !noHeader {
			fmt.Printf("%-*s  %6s  %-11s  %-19s  %4s  %4s  %4s  %-15s  %2s\n",
				widthName, "Name", "Size", "Free", "Time", "↑S", "↓L", "✓C", "ID", "P")
		}
	} else {
		widthExcludingName = 88 // 6+11+19+4+4+4+15+5+2+9*2
		widthName = width - widthExcludingName
		if !noHeader {
			fmt.Printf("%-*s  %6s  %-11s  %-19s  %4s  %4s  %4s  %-15s  %5s  %2s\n",
				widthName, "Name", "Size", "Free", "Time", "↑S", "↓L", "✓C", "ID", "Score", "P")
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
		if dense && torrent.Description != "" {
			name += " // " + torrent.Description
		}
		remain := util.PrintStringInWidth(name, int64(widthName), true)
		if scores == nil {
			fmt.Printf("  %6s  %-11s  %-19s  %4d  %4d  %4d  %-15s  %2s\n",
				util.BytesSizeAround(float64(torrent.Size)),
				freeStr,
				util.FormatTime(torrent.Time),
				torrent.Seeders,
				torrent.Leechers,
				torrent.Snatched,
				torrent.Id,
				process,
			)
		} else {
			fmt.Printf("  %6s  %-11s  %-19s  %4s  %4s  %4s  %-15s  %5.0f  %2s\n",
				util.BytesSizeAround(float64(torrent.Size)),
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
		if dense {
			for {
				remain = strings.TrimSpace(remain)
				if remain == "" {
					break
				}
				remain = util.PrintStringInWidth(remain, int64(widthName), true)
				fmt.Printf("\n")
			}
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
			return "",
				fmt.Errorf("ambiguous result - multiple sites in config match: names of first and last match: %s, %s",
					firstMatchSite.GetName(), lastMatchSite.GetName())
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
			return "",
				fmt.Errorf("ambiguous result - multiple sites in config match: names of first and last match: %s, %s",
					firstMatchSite.GetName(), lastMatchSite.GetName())
		}
	}
	return "", nil
}

func CreateSiteHttpClient(siteConfig *config.SiteConfigStruct, globalConfig *config.ConfigStruct) (
	*azuretls.Session, [][]string, error) {
	var impersonate string
	var impersonateProfile *impersonateutil.Profile
	var httpHeaders [][]string
	if siteConfig.Impersonate != "" {
		impersonate = siteConfig.Impersonate
	} else if globalConfig.SiteImpersonate != "" {
		impersonate = globalConfig.SiteImpersonate
	}
	if impersonate != config.NONE {
		if ip := impersonateutil.GetProfile(impersonate); ip == nil {
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
		h2fingerprint = impersonateProfile.H2fingerprint
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
	if proxy == "" || proxy == "env" {
		proxy = util.ParseProxyFromEnv(siteConfig.Url)
	}
	insecure := config.Insecure || !siteConfig.Secure && (siteConfig.Insecure || globalConfig.SiteInsecure)
	timeout := int64(0)
	if siteConfig.Timeout != 0 {
		timeout = siteConfig.Timeout
	} else if globalConfig.SiteTimeout != 0 {
		timeout = globalConfig.SiteTimeout
	}
	if timeout == 0 {
		timeout = config.DEFAULT_SITE_TIMEOUT
	} else if timeout < 0 {
		timeout = constants.INFINITE_TIMEOUT
	}
	if ja3 == config.NONE {
		ja3 = ""
	}
	if h2fingerprint == config.NONE {
		h2fingerprint = ""
	}
	if proxy == config.NONE {
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
		return nil, "", fmt.Errorf("failed to fetch torrents from site: %v", err)
	}
	mimeType, _, _ := mime.ParseMediaType(header.Get("content-type"))
	if mimeType != "" && mimeType != "application/octet-stream" && mimeType != "application/x-bittorrent" {
		return nil, "", fmt.Errorf("server return invalid content-type: %s", mimeType)
	}
	filename := util.ExtractFilenameFromHttpHeader(header)
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
	resourcesWaitGroup.Wait()
}

// Purge site cache
func Purge(sitename string) {
	if sitename == "" {
		for _, siteInstance := range sites {
			siteInstance.PurgeCache()
		}
	} else if sites[sitename] != nil {
		sites[sitename].PurgeCache()
	}
}
