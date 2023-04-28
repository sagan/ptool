package nexusphp

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	cloudflarebp "github.com/DaRealFreak/cloudflare-bp-go"
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
	torrentsParserOption *TorrentsParserOption
}

func (npclient *Site) PurgeCache() {
	npclient.datatime = 0
}

func (npclient *Site) GetName() string {
	return npclient.Name
}

func (npclient *Site) GetSiteConfig() *config.SiteConfigStruct {
	return npclient.SiteConfig
}

func (npclient *Site) SearchTorrents(keyword string) ([]site.Torrent, error) {
	searchUrl := npclient.SiteConfig.SearchUrl
	if searchUrl == "" {
		searchUrl = npclient.SiteConfig.Url + "torrents.php"
	}
	if !strings.Contains(searchUrl, "%s") {
		if strings.Contains(searchUrl, "?") {
			searchUrl += "&"
		} else {
			searchUrl += "?"
		}
		searchUrl += "search=%s"
	}
	searchUrl = strings.Replace(searchUrl, "%s", url.PathEscape(keyword), 1)

	doc, err := utils.GetUrlDoc(searchUrl, npclient.SiteConfig.Cookie, npclient.HttpClient)
	if err != nil {
		return nil, fmt.Errorf("failed to parse site page dom: %v", err)
	}
	return npclient.parseTorrentsFromDoc(doc, utils.Now())
}

func (npclient *Site) DownloadTorrent(url string) ([]byte, error) {
	if regexp.MustCompile(`^\d+$`).MatchString(url) {
		return npclient.DownloadTorrentById(url)
	}
	if !strings.Contains(url, "/download.php") {
		idRegexp := regexp.MustCompile(`[?&]id=(?P<id>\d+)`)
		m := idRegexp.FindStringSubmatch(url)
		if m != nil {
			return npclient.DownloadTorrentById(m[idRegexp.SubexpIndex("id")])
		}
	}
	res, err := utils.FetchUrl(url, npclient.SiteConfig.Cookie, npclient.HttpClient)
	if err != nil {
		return nil, fmt.Errorf("can not fetch torrents from site: %v", err)
	}
	defer res.Body.Close()
	return io.ReadAll(res.Body)
}

func (npclient *Site) DownloadTorrentById(id string) ([]byte, error) {
	res, err := utils.FetchUrl(npclient.SiteConfig.Url+"download.php?https=1&id="+fmt.Sprint(id), npclient.SiteConfig.Cookie, npclient.HttpClient)
	if err != nil {
		return nil, fmt.Errorf("can not fetch torrents from site: %v", err)
	}
	defer res.Body.Close()
	return io.ReadAll(res.Body)
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

func (npclient *Site) parseTorrentsFromDoc(doc *goquery.Document, datatime int64) ([]site.Torrent, error) {
	return parseTorrents(doc, npclient.torrentsParserOption, datatime)
}

func (npclient *Site) sync() error {
	if npclient.datatime > 0 {
		return nil
	}
	url := npclient.SiteConfig.TorrentsUrl
	if url == "" {
		url = npclient.SiteConfig.Url + "torrents.php"
	}
	doc, err := utils.GetUrlDoc(url, npclient.SiteConfig.Cookie, npclient.HttpClient)
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
		if log.GetLevel() >= log.TraceLevel {
			log.Tracef("Site GetStatus got no data, possible a parser error")
		}
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
		doc, err := utils.GetUrlDoc(extraUrl, npclient.SiteConfig.Cookie, npclient.HttpClient)
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
		return nil, fmt.Errorf("invalid site timezone: %s", siteConfig.Timezone)
	}
	httpClient := &http.Client{}
	httpClient.Transport = cloudflarebp.AddCloudFlareByPass(httpClient.Transport)
	client := &Site{
		Name:       name,
		Location:   location,
		SiteConfig: siteConfig,
		Config:     config,
		HttpClient: httpClient,
		torrentsParserOption: &TorrentsParserOption{
			location:                    location,
			siteurl:                     siteConfig.Url,
			selectorTorrentsList:        siteConfig.SelectorTorrentsList,
			selectorTorrent:             siteConfig.SelectorTorrent,
			selectorTorrentDownloadLink: siteConfig.SelectorTorrentDownloadLink,
			selectorTorrentDetailsLink:  siteConfig.SelectorTorrentDetailsLink,
			selectorTorrentTime:         siteConfig.SelectorTorrentTime,
			SelectorTorrentSeeders:      siteConfig.SelectorTorrentSeeders,
			SelectorTorrentLeechers:     siteConfig.SelectorTorrentLeechers,
			SelectorTorrentSnatched:     siteConfig.SelectorTorrentSnatched,
			SelectorTorrentSize:         siteConfig.SelectorTorrentSize,
			SelectorTorrentProcessBar:   siteConfig.SelectorTorrentProcessBar,
		},
	}
	return client, nil
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
