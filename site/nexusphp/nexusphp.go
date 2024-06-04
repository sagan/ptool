package nexusphp

import (
	"fmt"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/Noooste/azuretls-client"
	"github.com/PuerkitoBio/goquery"
	log "github.com/sirupsen/logrus"

	"github.com/sagan/ptool/config"
	"github.com/sagan/ptool/constants"
	"github.com/sagan/ptool/site"
	"github.com/sagan/ptool/util"
)

type Site struct {
	Name                 string
	Location             *time.Location
	SiteConfig           *config.SiteConfigStruct
	Config               *config.ConfigStruct
	HttpClient           *azuretls.Session
	HttpHeaders          [][]string
	siteStatus           *site.Status
	latestTorrents       []*site.Torrent
	extraTorrents        []*site.Torrent
	datatime             int64
	datetimeExtra        int64
	cuhash               string
	digitHashPasskey     string
	digitHashErr         error
	torrentsParserOption *TorrentsParserOption
}

// PublishTorrent implements site.Site.
func (npclient *Site) PublishTorrent(contents []byte, metadata url.Values) (id string, err error) {
	coverFile := metadata.Get("_cover")
	if coverFile != "" {
		if npclient.SiteConfig.ImageUploadUrl != "" {
			_, err := util.PostUploadFile(npclient.HttpClient, npclient.SiteConfig.ImageUploadUrl, coverFile,
				npclient.SiteConfig.ImageUploadFileField, nil, npclient.SiteConfig.ImageUploadResponseUrlField)
			log.Debugf("upload cover: err=%v", err)
		}
	}
	return "", site.ErrUnimplemented
}

const (
	DEFAULT_TORRENTS_URL = "torrents.php"
)

var sortFields = map[string]string{
	"name":     "1",
	"time":     "4",
	"size":     "5",
	"seeders":  "7",
	"leechers": "8",
	"snatched": "6",
}

func (npclient *Site) GetDefaultHttpHeaders() [][]string {
	return npclient.HttpHeaders
}

func (npclient *Site) PurgeCache() {
	npclient.datatime = 0
	npclient.latestTorrents = nil
	npclient.extraTorrents = nil
	npclient.siteStatus = nil
	npclient.cuhash = ""
}

func (npclient *Site) GetName() string {
	return npclient.Name
}

func (npclient *Site) GetSiteConfig() *config.SiteConfigStruct {
	return npclient.SiteConfig
}

func (npclient *Site) SearchTorrents(keyword string, baseUrl string) ([]*site.Torrent, error) {
	if baseUrl == "" {
		if npclient.SiteConfig.SearchUrl != "" {
			baseUrl = npclient.SiteConfig.SearchUrl
		} else if npclient.SiteConfig.TorrentsUrl != "" {
			baseUrl = npclient.SiteConfig.TorrentsUrl
		} else {
			baseUrl = DEFAULT_TORRENTS_URL
		}
	}
	searchUrl := npclient.SiteConfig.ParseSiteUrl(baseUrl, true)
	if !strings.Contains(searchUrl, "%s") {
		searchQueryVariable := "search"
		if npclient.SiteConfig.SearchQueryVariable != "" {
			searchQueryVariable = npclient.SiteConfig.SearchQueryVariable
		}
		searchUrl += searchQueryVariable + "=%s"
	}
	searchUrl = strings.Replace(searchUrl, "%s", url.PathEscape(keyword), 1)

	doc, res, err := util.GetUrlDocWithAzuretls(searchUrl, npclient.HttpClient,
		npclient.SiteConfig.Cookie, site.GetUa(npclient), npclient.GetDefaultHttpHeaders())
	if !npclient.SiteConfig.AcceptAnyHttpStatus && err != nil || doc == nil {
		return nil, fmt.Errorf("failed to parse site page dom: %w", err)
	}
	if strings.Contains(res.Request.Url, "/login.php") {
		return nil, fmt.Errorf("not logined (cookie may has expired)")
	}
	return npclient.parseTorrentsFromDoc(doc, util.Now())
}

// If torrentUrl is (seems) a torrent download url, direct use it.
// Otherwise try to parse torrent id from it and download torrent from id
func (npclient *Site) DownloadTorrent(torrentUrl string) (content []byte, filename string, id string, err error) {
	if !util.IsUrl(torrentUrl) {
		id = strings.TrimPrefix(torrentUrl, npclient.GetName()+".")
		content, filename, err = npclient.DownloadTorrentById(id)
		return
	}
	urlObj, err := url.Parse(torrentUrl)
	if err != nil {
		return nil, "", "", fmt.Errorf("invalid torrent url: %w", err)
	}
	id = parseTorrentIdFromUrl(torrentUrl, npclient.torrentsParserOption.idRegexp)
	downloadUrlPrefix := strings.TrimPrefix(npclient.SiteConfig.TorrentDownloadUrlPrefix, "/")
	if downloadUrlPrefix == "" {
		downloadUrlPrefix = "download"
	}
	if !strings.HasPrefix(urlObj.Path, "/"+downloadUrlPrefix) && id != "" {
		content, filename, err = npclient.DownloadTorrentById(id)
		return
	}
	content, filename, err = site.DownloadTorrentByUrl(npclient, npclient.HttpClient, torrentUrl, id)
	return
}

func (npclient *Site) DownloadTorrentById(id string) ([]byte, string, error) {
	torrentUrl := generateTorrentDownloadUrl(id, npclient.torrentsParserOption.torrentDownloadUrl,
		npclient.torrentsParserOption.npletdown)
	torrentUrl = npclient.SiteConfig.ParseSiteUrl(torrentUrl, false)
	if npclient.SiteConfig.UseCuhash {
		if npclient.cuhash == "" {
			// update cuhash by side effect of sync (fetching latest torrents)
			npclient.sync()
		}
		if npclient.cuhash != "" {
			torrentUrl = util.AppendUrlQueryString(torrentUrl, "cuhash="+npclient.cuhash)
		} else {
			log.Warnf("Failed to get site cuhash. torrent download may fail")
		}
	} else if npclient.SiteConfig.UseDigitHash {
		passkey := ""
		if npclient.SiteConfig.Passkey != "" {
			passkey = npclient.SiteConfig.Passkey
		} else if npclient.digitHashPasskey != "" {
			passkey = npclient.digitHashPasskey
		} else if npclient.digitHashErr == nil { // only try to fetch passkey once
			npclient.digitHashPasskey, npclient.digitHashErr = npclient.getDigithash(id)
			if npclient.digitHashErr != nil {
				log.Warnf("Failed to get site passkey. torrent download may fail")
			} else {
				passkey = npclient.digitHashPasskey
				log.Infof(`Found site passkey. Add the passkey = "%s" line to site config block of ptool.toml `+
					`to speed up the next visit`, passkey)
			}
		}
		if passkey != "" {
			torrentUrl = strings.TrimSuffix(torrentUrl, "/")
			torrentUrl += "/" + passkey
		}
	}
	return site.DownloadTorrentByUrl(npclient, npclient.HttpClient, torrentUrl, id)
}

func (npclient *Site) getDigithash(id string) (string, error) {
	detailsUrl := npclient.SiteConfig.ParseSiteUrl(fmt.Sprintf("t/%s/", id), false)
	doc, _, err := util.GetUrlDocWithAzuretls(detailsUrl, npclient.HttpClient,
		npclient.SiteConfig.Cookie, site.GetUa(npclient), npclient.GetDefaultHttpHeaders())
	if err != nil {
		return "", fmt.Errorf("failed to get torrent detail page: %w", err)
	}
	downloadUrlPrefix := strings.TrimPrefix(npclient.SiteConfig.TorrentDownloadUrlPrefix, "/")
	if downloadUrlPrefix == "" {
		downloadUrlPrefix = "download"
	}
	torrentDownloadLinks := doc.Find(fmt.Sprintf(`a[href^="/%s"],a[href^="%s"],a[href^="%s%s"]`,
		downloadUrlPrefix, downloadUrlPrefix, npclient.SiteConfig.Url, downloadUrlPrefix))
	passkey := ""
	torrentDownloadLinks.EachWithBreak(func(i int, el *goquery.Selection) bool {
		urlPathes := strings.Split(el.AttrOr("href", ""), "/")
		if len(urlPathes) > 2 {
			key := urlPathes[len(urlPathes)-1]
			if util.IsHexString(key, 32) {
				passkey = key
				return false
			}
		}
		return true
	})
	if passkey == "" {
		return "", fmt.Errorf("no passkey found in torrent detail page")
	}
	return passkey, nil
}

func (npclient *Site) GetStatus() (*site.Status, error) {
	err := npclient.sync()
	if err != nil {
		return nil, fmt.Errorf("failed to fetch site data: %w", err)
	}
	return npclient.siteStatus, nil
}

func (npclient *Site) GetLatestTorrents(full bool) ([]*site.Torrent, error) {
	latestTorrents := []*site.Torrent{}
	err := npclient.sync()
	if err != nil {
		return nil, fmt.Errorf("failed to fetch site data: %w", err)
	}
	if full {
		npclient.syncExtra()
	}
	if npclient.latestTorrents != nil {
		latestTorrents = append(latestTorrents, npclient.latestTorrents...)
	}
	if npclient.extraTorrents != nil {
		latestTorrents = append(latestTorrents, npclient.extraTorrents...)
	}
	return latestTorrents, nil
}

func (npclient *Site) GetAllTorrents(sort string, desc bool, pageMarker string, baseUrl string) (
	torrents []*site.Torrent, nextPageMarker string, err error) {
	if sort != "" && sort != constants.NONE && sortFields[sort] == "" {
		err = fmt.Errorf("unsupported sort field: %s", sort)
		return
	}
	if pageMarker == constants.NONE {
		pageMarker = ""
	}
	// baseUrl is empty; or is query string, e.g. "?seeders_begin=1"
	if baseUrl == "" || baseUrl == constants.NONE || strings.HasPrefix(baseUrl, "?") {
		torrentsUrl := ""
		if npclient.SiteConfig.TorrentsUrl != "" {
			torrentsUrl = npclient.SiteConfig.TorrentsUrl
		} else {
			torrentsUrl = DEFAULT_TORRENTS_URL
		}
		if strings.HasPrefix(baseUrl, "?") {
			baseUrl = util.AppendUrlQueryString(torrentsUrl, baseUrl)
		} else {
			baseUrl = torrentsUrl
		}
	}
	torrentsPageUrl := npclient.SiteConfig.ParseSiteUrl(baseUrl, false)
	torrentsPageUrlObj, err := url.Parse(torrentsPageUrl)
	if err != nil {
		err = fmt.Errorf("invalid base-url: %w", err)
		return
	}
	torrentsPageUrlQuery := torrentsPageUrlObj.Query()
	sorting := sort != "" && sort != constants.NONE
	if sorting {
		sortOrder := ""
		// 颠倒排序。从np最后一页开始获取。目的是跳过站点的置顶种子
		if desc {
			sortOrder = "asc"
		} else {
			sortOrder = "desc"
		}
		torrentsPageUrlQuery.Set("sort", sortFields[sort])
		torrentsPageUrlQuery.Set("type", sortOrder)
	}
	if pageMarker == "" && torrentsPageUrlQuery.Get("page") != "" {
		pageMarker = torrentsPageUrlQuery.Get("page")
	}
	page := int64(0)
	if pageMarker != "" {
		page = util.ParseInt(pageMarker)
	}
	torrentsPageUrlQuery.Del("page")
	torrentsPageUrlObj.RawQuery = torrentsPageUrlQuery.Encode()
	torrentsPageUrl = util.AppendUrlQueryStringDelimiter(torrentsPageUrlObj.String())
	pageStr := "page=" + fmt.Sprint(page)
	now := util.Now()
	doc, res, _err := util.GetUrlDocWithAzuretls(torrentsPageUrl+pageStr, npclient.HttpClient,
		npclient.SiteConfig.Cookie, site.GetUa(npclient), npclient.GetDefaultHttpHeaders())
	if !npclient.SiteConfig.AcceptAnyHttpStatus && _err != nil || doc == nil {
		err = fmt.Errorf("failed to fetch torrents page dom: %w", _err)
		return
	}
	if strings.Contains(res.Request.Url, "/login.php") {
		return nil, "", fmt.Errorf("not logined (cookie may has expired)")
	}

	lastPage := int64(0)
	if pageMarker == "" {
		paginationEls := doc.Find(`*[href*="&page="]`)
		pageRegexp := regexp.MustCompile(`&page=(?P<page>\d+)`)
		paginationEls.Each(func(i int, s *goquery.Selection) {
			m := pageRegexp.FindStringSubmatch(s.AttrOr("href", ""))
			if m != nil {
				page := util.ParseInt(m[pageRegexp.SubexpIndex("page")])
				if page > lastPage {
					lastPage = page
				}
			}
		})
	}
labelLastPage:
	if pageMarker == "" && lastPage > 0 {
		page = lastPage
		pageStr = "page=" + fmt.Sprint(page)
		now = util.Now()
		doc, res, _err = util.GetUrlDocWithAzuretls(torrentsPageUrl+pageStr, npclient.HttpClient,
			npclient.SiteConfig.Cookie, site.GetUa(npclient), npclient.GetDefaultHttpHeaders())
		if !npclient.SiteConfig.AcceptAnyHttpStatus && _err != nil || doc == nil {
			err = fmt.Errorf("failed to fetch torrents page dom: %w", _err)
			return
		}
		if strings.Contains(res.Request.Url, "/login.php") {
			err = fmt.Errorf("not logined (cookie may has expired)")
			return
		}
	}

	torrents, err = npclient.parseTorrentsFromDoc(doc, now)
	if err != nil {
		log.Tracef("Failed to get torrents from doc: %v", err)
		return
	}
	// 部分站点（如蝴蝶）有 bug，分页栏的最后一页内容有时是空的
	if pageMarker == "" && lastPage > 1 && len(torrents) == 0 {
		lastPage--
		log.Warnf("Last torrents page is empty, access second last page instead")
		goto labelLastPage
	}
	if page > 0 {
		nextPageMarker = fmt.Sprint(page - 1)
	}
	if sorting {
		for i, j := 0, len(torrents)-1; i < j; i, j = i+1, j-1 {
			torrents[i], torrents[j] = torrents[j], torrents[i]
		}
	}
	return
}

func (npclient *Site) parseTorrentsFromDoc(doc *goquery.Document, datatime int64) ([]*site.Torrent, error) {
	torrents, err := parseTorrents(doc, npclient.torrentsParserOption, datatime, npclient.GetName())
	if npclient.SiteConfig.UseCuhash && npclient.cuhash == "" &&
		len(torrents) > 0 && torrents[0].DownloadUrl != "" {
		urlObj, err := url.Parse(torrents[0].DownloadUrl)
		if err == nil {
			cuhash := urlObj.Query().Get("cuhash")
			log.Debugf("Update site %s cuhash=%s", npclient.Name, cuhash)
			npclient.cuhash = cuhash
		}
	}
	return torrents, err
}

func (npclient *Site) sync() error {
	if npclient.datatime > 0 {
		return nil
	}
	url := npclient.SiteConfig.TorrentsUrl
	if url == "" {
		url = DEFAULT_TORRENTS_URL
	}
	url = npclient.SiteConfig.ParseSiteUrl(url, false)
	doc, res, err := util.GetUrlDocWithAzuretls(url, npclient.HttpClient,
		npclient.SiteConfig.Cookie, site.GetUa(npclient), npclient.GetDefaultHttpHeaders())
	if !npclient.SiteConfig.AcceptAnyHttpStatus && err != nil || doc == nil {
		return fmt.Errorf("failed to get site page dom: %w", err)
	}
	if strings.Contains(res.Request.Url, "/login.php") {
		return fmt.Errorf("not logined (cookie may has expired)")
	}
	html := doc.Find("html")
	npclient.datatime = util.Now()

	siteStatus := &site.Status{}
	selectorUserInfo := npclient.SiteConfig.SelectorUserInfo
	if selectorUserInfo == "" {
		selectorUserInfo = "#info_block"
	}
	infoTr := doc.Find(selectorUserInfo).First()
	if infoTr.Length() == 0 {
		infoTr = doc.Find("body") // fallback
	}
	infoTxt := infoTr.Text()
	infoTxt = strings.ReplaceAll(infoTxt, "\n", " ")
	infoTxt = strings.ReplaceAll(infoTxt, "\r", " ")

	var sstr string

	sstr = ""
	if npclient.SiteConfig.SelectorUserInfoUploaded != "" {
		sstr = util.DomSelectorText(html, npclient.SiteConfig.SelectorUserInfoUploaded)
	} else {
		re := regexp.MustCompile(`(?i)(上傳量|上傳|上传量|上传|Uploaded|Up)[：:\s]+(?P<s>[.\s0-9KMGTEPBkmgtepib]+)`)
		m := re.FindStringSubmatch(infoTxt)
		if m != nil {
			sstr = strings.ReplaceAll(strings.TrimSpace(m[re.SubexpIndex("s")]), " ", "")
		}
	}
	if sstr != "" {
		s, _ := util.RAMInBytes(sstr)
		siteStatus.UserUploaded = s
	}

	sstr = ""
	if npclient.SiteConfig.SelectorUserInfoDownloaded != "" {
		sstr = util.DomSelectorText(html, npclient.SiteConfig.SelectorUserInfoDownloaded)
	} else {
		re := regexp.MustCompile(`(?i)(下載量|下載|下载量|下载|Downloaded|Down)[：:\s]+(?P<s>[.\s0-9KMGTEPBkmgtepib]+)`)
		m := re.FindStringSubmatch(infoTxt)
		if m != nil {
			sstr = strings.ReplaceAll(strings.TrimSpace(m[re.SubexpIndex("s")]), " ", "")
		}
	}
	if sstr != "" {
		s, _ := util.RAMInBytes(sstr)
		siteStatus.UserDownloaded = s
	}

	if npclient.SiteConfig.SelectorUserInfoUserName != "" {
		siteStatus.UserName = util.DomSelectorText(html, npclient.SiteConfig.SelectorUserInfoUserName)
	} else {
		siteStatus.UserName = doc.Find(`*[href*="userdetails.php?"]`).First().Text()
	}
	siteStatus.UserName = strings.TrimSpace(siteStatus.UserName)

	// possibly parsing error or some problem
	if !siteStatus.IsOk() {
		log.TraceFn(func() []any {
			return []any{"Site GetStatus got no data, possible a parser error"}
		})
	}
	npclient.siteStatus = siteStatus

	torrents, err := npclient.parseTorrentsFromDoc(doc, npclient.datatime)
	if err != nil {
		log.Errorf("failed to parse site page torrents: %v", err)
	} else {
		npclient.latestTorrents = torrents
	}
	return nil
}

func (npclient *Site) syncExtra() error {
	if npclient.datetimeExtra > 0 {
		return nil
	}
	extraTorrents := []*site.Torrent{}
	for _, extraUrl := range npclient.SiteConfig.TorrentsExtraUrls {
		doc, res, err := util.GetUrlDocWithAzuretls(npclient.SiteConfig.ParseSiteUrl(extraUrl, false), npclient.HttpClient,
			npclient.SiteConfig.Cookie, site.GetUa(npclient), npclient.GetDefaultHttpHeaders())
		if err != nil {
			log.Errorf("failed to parse site page dom: %v", err)
			continue
		}
		if strings.Contains(res.Request.Url, "/login.php") {
			return fmt.Errorf("not logined (cookie may has expired)")
		}
		torrents, err := npclient.parseTorrentsFromDoc(doc, util.Now())
		if err != nil {
			log.Errorf("failed to parse site page torrents: %v", err)
			continue
		}
		extraTorrents = append(npclient.extraTorrents, torrents...)
	}
	npclient.extraTorrents = extraTorrents
	npclient.datetimeExtra = util.Now()
	return nil
}

func NewSite(name string, siteConfig *config.SiteConfigStruct, config *config.ConfigStruct) (site.Site, error) {
	if siteConfig.Cookie == "" {
		log.Warnf("Site %s has no cookie provided", name)
	}
	location, err := time.LoadLocation(siteConfig.GetTimezone())
	if err != nil {
		return nil, fmt.Errorf("invalid site timezone %s: %w", siteConfig.GetTimezone(), err)
	}
	httpClient, httpHeaders, err := site.CreateSiteHttpClient(siteConfig, config)
	if err != nil {
		return nil, fmt.Errorf("failed to create site http client: %w", err)
	}
	site := &Site{
		Name:        name,
		Location:    location,
		SiteConfig:  siteConfig,
		Config:      config,
		HttpClient:  httpClient,
		HttpHeaders: httpHeaders,
		torrentsParserOption: &TorrentsParserOption{
			location:                       location,
			siteurl:                        siteConfig.Url,
			globalHr:                       siteConfig.GlobalHnR,
			npletdown:                      !siteConfig.NexusphpNoLetDown,
			torrentDownloadUrl:             siteConfig.TorrentDownloadUrl,
			selectorTorrentsListHeader:     siteConfig.SelectorTorrentsListHeader,
			selectorTorrentsList:           siteConfig.SelectorTorrentsList,
			selectorTorrentBlock:           siteConfig.SelectorTorrentBlock,
			selectorTorrent:                siteConfig.SelectorTorrent,
			selectorTorrentDownloadLink:    siteConfig.SelectorTorrentDownloadLink,
			selectorTorrentDetailsLink:     siteConfig.SelectorTorrentDetailsLink,
			selectorTorrentTime:            siteConfig.SelectorTorrentTime,
			selectorTorrentSeeders:         siteConfig.SelectorTorrentSeeders,
			selectorTorrentLeechers:        siteConfig.SelectorTorrentLeechers,
			selectorTorrentSnatched:        siteConfig.SelectorTorrentSnatched,
			selectorTorrentSize:            siteConfig.SelectorTorrentSize,
			selectorTorrentActive:          siteConfig.SelectorTorrentActive,
			selectorTorrentCurrentActive:   siteConfig.SelectorTorrentCurrentActive,
			selectorTorrentFree:            siteConfig.SelectorTorrentFree,
			selectorTorrentHnR:             siteConfig.SelectorTorrentHnR,
			selectorTorrentNeutral:         siteConfig.SelectorTorrentNeutral,
			selectorTorrentNoTraffic:       siteConfig.SelectorTorrentNoTraffic,
			selectorTorrentPaid:            siteConfig.SelectorTorrentPaid,
			selectorTorrentDiscountEndTime: siteConfig.SelectorTorrentDiscountEndTime,
		},
	}
	if siteConfig.TorrentUrlIdRegexp != "" {
		site.torrentsParserOption.idRegexp = regexp.MustCompile(siteConfig.TorrentUrlIdRegexp)
	}
	return site, nil
}

func init() {
	site.Register(&site.RegInfo{
		Name:    "nexusphp",
		Creator: NewSite,
	})
}

var (
	_ site.Site = (*Site)(nil)
)
