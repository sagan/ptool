package nexusphp

import (
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	log "github.com/sirupsen/logrus"

	"github.com/sagan/ptool/config"
	"github.com/sagan/ptool/site"
	"github.com/sagan/ptool/utils"
)

type Site struct {
	Name                 string
	Location             *time.Location
	SiteConfig           *config.SiteConfigStruct
	Config               *config.ConfigStruct
	HttpClient           *http.Client
	siteStatus           *site.Status
	latestTorrents       []site.Torrent
	extraTorrents        []site.Torrent
	datatime             int64
	datetimeExtra        int64
	cuhash               string
	idRegexp             *regexp.Regexp
	torrentsParserOption *TorrentsParserOption
}

var sortFields = map[string](string){
	"name":     "1",
	"time":     "4",
	"size":     "5",
	"seeders":  "7",
	"leechers": "8",
	"snatched": "6",
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

func (npclient *Site) SearchTorrents(keyword string, baseUrl string) ([]site.Torrent, error) {
	if baseUrl == "" {
		if npclient.SiteConfig.SearchUrl != "" {
			baseUrl = npclient.SiteConfig.SearchUrl
		} else {
			baseUrl = "torrents.php"
		}
	}
	searchUrl := npclient.SiteConfig.ParseSiteUrl(baseUrl, true)
	if !strings.Contains(searchUrl, "%s") {
		searchUrl += "search=%s"
	}
	searchUrl = strings.Replace(searchUrl, "%s", url.PathEscape(keyword), 1)

	doc, err := utils.GetUrlDoc(searchUrl, npclient.HttpClient,
		npclient.SiteConfig.Cookie, npclient.SiteConfig.UserAgent, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to parse site page dom: %v", err)
	}
	return npclient.parseTorrentsFromDoc(doc, utils.Now())
}

func (npclient *Site) DownloadTorrent(torrentUrl string) ([]byte, string, error) {
	if !utils.IsUrl(torrentUrl) {
		id := strings.TrimPrefix(torrentUrl, npclient.GetName()+".")
		return npclient.DownloadTorrentById(id)
	}
	if !strings.Contains(torrentUrl, "/download") {
		m := npclient.idRegexp.FindStringSubmatch(torrentUrl)
		if m != nil {
			return npclient.DownloadTorrentById(m[npclient.idRegexp.SubexpIndex("id")])
		}
	}
	// skip NP download notice. see https://github.com/xiaomlove/nexusphp/blob/php8/public/download.php
	torrentUrl = utils.AppendUrlQueryString(torrentUrl, "letdown=1")
	return site.DownloadTorrentByUrl(npclient, npclient.HttpClient, torrentUrl, "")
}

func (npclient *Site) DownloadTorrentById(id string) ([]byte, string, error) {
	torrentUrl := npclient.SiteConfig.Url + "download.php?https=1&letdown=1&id=" + id
	if npclient.SiteConfig.UseCuhash {
		if npclient.cuhash == "" {
			// update cuhash by side effect of sync (fetching latest torrents)
			npclient.sync()
		}
		if npclient.cuhash != "" {
			torrentUrl = utils.AppendUrlQueryString(torrentUrl, "cuhash="+npclient.cuhash)
		} else {
			log.Warnf("Failed to get site cuhash. torrent download may fail")
		}
	}
	return site.DownloadTorrentByUrl(npclient, npclient.HttpClient, torrentUrl, id)
}

func (npclient *Site) GetStatus() (*site.Status, error) {
	err := npclient.sync()
	if err != nil {
		return nil, fmt.Errorf("failed to fetch site data: %v", err)
	}
	return npclient.siteStatus, nil
}

func (npclient *Site) GetLatestTorrents(full bool) ([]site.Torrent, error) {
	latestTorrents := make([]site.Torrent, 0)
	err := npclient.sync()
	if err != nil {
		return nil, fmt.Errorf("failed to fetch site data: %v", err)
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

func (npclient *Site) GetAllTorrents(sort string, desc bool, pageMarker string, baseUrl string) (torrents []site.Torrent, nextPageMarker string, err error) {
	if sort != "" && sort != "none" && sortFields[sort] == "" {
		err = fmt.Errorf("unsupported sort field: %s", sort)
		return
	}
	// 颠倒排序。从np最后一页开始获取。目的是跳过站点的置顶种子
	sortOrder := "desc"
	if desc {
		sortOrder = "asc"
	}
	page := int64(0)
	if pageMarker != "" {
		page = utils.ParseInt(pageMarker)
	}
	if baseUrl == "" {
		baseUrl = "torrents.php"
	}
	pageUrl := npclient.SiteConfig.ParseSiteUrl(baseUrl, true)
	queryString := ""
	if sort != "" && sort != "none" {
		queryString += "sort=" + sortFields[sort] + "&type=" + sortOrder + "&"
	}
	pageStr := "page=" + fmt.Sprint(page)
	now := utils.Now()
	doc, error := utils.GetUrlDoc(pageUrl+queryString+pageStr, npclient.HttpClient,
		npclient.SiteConfig.Cookie, npclient.SiteConfig.UserAgent, nil)
	if error != nil {
		err = fmt.Errorf("failed to fetch torrents page dom: %v", error)
		return
	}

	if pageMarker == "" {
		paginationEls := doc.Find(`*[href*="&page="]`)
		lastPage := int64(0)
		pageRegexp := regexp.MustCompile(`&page=(?P<page>\d+)`)
		paginationEls.Each(func(i int, s *goquery.Selection) {
			m := pageRegexp.FindStringSubmatch(s.AttrOr("href", ""))
			if m != nil {
				page := utils.ParseInt(m[pageRegexp.SubexpIndex("page")])
				if page > lastPage {
					lastPage = page
				}
			}
		})
		if lastPage > 0 {
			page = lastPage
			pageStr = "page=" + fmt.Sprint(page)
			now = utils.Now()
			doc, error = utils.GetUrlDoc(pageUrl+queryString+pageStr, npclient.HttpClient,
				npclient.SiteConfig.Cookie, npclient.SiteConfig.UserAgent, nil)
			if error != nil {
				err = fmt.Errorf("failed to fetch torrents page dom: %v", error)
				return
			}
		}
	}

	torrents, err = npclient.parseTorrentsFromDoc(doc, now)
	if err != nil {
		log.Tracef("Failed to get torrents from doc: %v", err)
		return
	}
	if page > 0 {
		nextPageMarker = fmt.Sprint(page - 1)
	}
	for i, j := 0, len(torrents)-1; i < j; i, j = i+1, j-1 {
		torrents[i], torrents[j] = torrents[j], torrents[i]
	}

	return
}

func (npclient *Site) parseTorrentsFromDoc(doc *goquery.Document, datatime int64) ([]site.Torrent, error) {
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
		url = npclient.SiteConfig.Url + "torrents.php"
	}
	doc, err := utils.GetUrlDoc(url, npclient.HttpClient,
		npclient.SiteConfig.Cookie, npclient.SiteConfig.UserAgent, nil)
	if err != nil {
		return fmt.Errorf("failed to get site page dom: %v", err)
	}
	html := doc.Find("html")
	npclient.datatime = utils.Now()

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
		sstr = utils.DomSelectorText(html, npclient.SiteConfig.SelectorUserInfoUploaded)
	} else {
		re := regexp.MustCompile(`(上傳量|上傳|上传量|上传)[：:\s]+(?P<s>[.\s0-9KMGTEPBkmgtepib]+)`)
		m := re.FindStringSubmatch(infoTxt)
		if m != nil {
			sstr = strings.ReplaceAll(strings.TrimSpace(m[re.SubexpIndex("s")]), " ", "")
		}
	}
	if sstr != "" {
		s, _ := utils.RAMInBytes(sstr)
		siteStatus.UserUploaded = s
	}

	sstr = ""
	if npclient.SiteConfig.SelectorUserInfoDownloaded != "" {
		sstr = utils.DomSelectorText(html, npclient.SiteConfig.SelectorUserInfoDownloaded)
	} else {
		re := regexp.MustCompile(`(下載量|下載|下载量|下载)[：:\s]+(?P<s>[.\s0-9KMGTEPBkmgtepib]+)`)
		m := re.FindStringSubmatch(infoTxt)
		if m != nil {
			sstr = strings.ReplaceAll(strings.TrimSpace(m[re.SubexpIndex("s")]), " ", "")
		}
	}
	if sstr != "" {
		s, _ := utils.RAMInBytes(sstr)
		siteStatus.UserDownloaded = s
	}

	if npclient.SiteConfig.SelectorUserInfoUserName != "" {
		siteStatus.UserName = utils.DomSelectorText(html, npclient.SiteConfig.SelectorUserInfoUserName)
	} else {
		siteStatus.UserName = doc.Find(`*[href*="userdetails.php?"]`).First().Text()
	}

	// possibly parsing error or some problem
	if siteStatus.UserName == "" && siteStatus.UserDownloaded == 0 && siteStatus.UserUploaded == 0 {
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

	extraTorrents := make([]site.Torrent, 0)
	for _, extraUrl := range npclient.SiteConfig.TorrentsExtraUrls {
		doc, err := utils.GetUrlDoc(extraUrl, npclient.HttpClient,
			npclient.SiteConfig.Cookie, npclient.SiteConfig.UserAgent, nil)
		if err != nil {
			log.Errorf("failed to parse site page dom: %v", err)
			continue
		}
		torrents, err := npclient.parseTorrentsFromDoc(doc, utils.Now())
		if err != nil {
			log.Errorf("failed to parse site page torrents: %v", err)
			continue
		}
		extraTorrents = append(npclient.extraTorrents, torrents...)
	}
	npclient.extraTorrents = extraTorrents
	npclient.datetimeExtra = utils.Now()
	return nil
}

func NewSite(name string, siteConfig *config.SiteConfigStruct, config *config.ConfigStruct) (site.Site, error) {
	if siteConfig.Cookie == "" {
		return nil, fmt.Errorf("cann't create site: no cookie provided")
	}
	location, err := time.LoadLocation(siteConfig.Timezone)
	if err != nil {
		return nil, fmt.Errorf("invalid site timezone %s: %v", siteConfig.Timezone, err)
	}
	httpClient, err := site.CreateSiteHttpClient(siteConfig, config)
	if err != nil {
		return nil, fmt.Errorf("failed to create site http client: %v", err)
	}
	site := &Site{
		Name:       name,
		Location:   location,
		SiteConfig: siteConfig,
		Config:     config,
		HttpClient: httpClient,
		torrentsParserOption: &TorrentsParserOption{
			location:                       location,
			siteurl:                        siteConfig.Url,
			selectorTorrentsListHeader:     siteConfig.SelectorTorrentsListHeader,
			selectorTorrentsList:           siteConfig.SelectorTorrentsList,
			selectorTorrentBlock:           siteConfig.SelectorTorrentBlock,
			selectorTorrent:                siteConfig.SelectorTorrent,
			selectorTorrentDownloadLink:    siteConfig.SelectorTorrentDownloadLink,
			selectorTorrentDetailsLink:     siteConfig.SelectorTorrentDetailsLink,
			selectorTorrentTime:            siteConfig.SelectorTorrentTime,
			SelectorTorrentSeeders:         siteConfig.SelectorTorrentSeeders,
			SelectorTorrentLeechers:        siteConfig.SelectorTorrentLeechers,
			SelectorTorrentSnatched:        siteConfig.SelectorTorrentSnatched,
			SelectorTorrentSize:            siteConfig.SelectorTorrentSize,
			SelectorTorrentProcessBar:      siteConfig.SelectorTorrentProcessBar,
			SelectorTorrentFree:            siteConfig.SelectorTorrentFree,
			SelectorTorrentDiscountEndTime: siteConfig.SelectorTorrentDiscountEndTime,
		},
	}
	if siteConfig.TorrentUrlIdRegexp != "" {
		site.idRegexp = regexp.MustCompile(siteConfig.TorrentUrlIdRegexp)
	} else {
		site.idRegexp = regexp.MustCompile(`\bid=(?P<id>\d+)\b`)
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
