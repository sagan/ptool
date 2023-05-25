package discuz

// 基于 discuz 的 PT 站点。
// 天雪 (https://skyeysnow.com/) 使用的架构。尚未发现其它站点使用此架构。
// 特点：下载种子需要金币（重复下载不多次扣费）
// 种子下载链接：https://skyeysnow.com/download.php?id={id}&passkey={passkey} (如果cookie有效，passkey可省略)
// 注意种子 id 与页面 url 里的 tid (thread id)不同：https://www.skyey2.com/forum.php?mod=viewthread&tid=10907

import (
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/sagan/ptool/config"
	"github.com/sagan/ptool/site"
	"github.com/sagan/ptool/utils"
)

type Site struct {
	Name       string
	Location   *time.Location
	SiteConfig *config.SiteConfigStruct
	Config     *config.ConfigStruct
	HttpClient *http.Client
}

func (dzsite *Site) PurgeCache() {
}

func (dzsite *Site) GetName() string {
	return dzsite.Name
}

func (dzsite *Site) GetSiteConfig() *config.SiteConfigStruct {
	return dzsite.SiteConfig
}

func (dzsite *Site) GetStatus() (*site.Status, error) {
	doc, err := utils.GetUrlDoc(dzsite.SiteConfig.Url+"forum.php?mod=torrents", dzsite.HttpClient,
		dzsite.GetSiteConfig().Cookie, dzsite.SiteConfig.UserAgent, nil)
	if err != nil {
		return nil, err
	}
	usernameEl := doc.Find(`a[href^="home.php?mod=space&uid="]`).First()
	username := utils.DomSanitizedText(usernameEl)
	ratioEl := doc.Find(`#ratio_menu`)
	userUploaded, _ := utils.ExtractSizeStr(utils.DomSanitizedText(ratioEl))

	return &site.Status{
		UserName:       username,
		UserUploaded:   userUploaded,
		UserDownloaded: -1,
	}, nil
}

func (dzsite *Site) GetAllTorrents(sort string, desc bool, pageMarker string, baseUrl string) (
	torrents []site.Torrent, nextPageMarker string, err error) {
	return nil, "", fmt.Errorf("not implemented yet")
}

func (dzsite *Site) GetLatestTorrents(full bool) ([]site.Torrent, error) {
	return nil, fmt.Errorf("not implemented yet")
}

func (dzsite *Site) SearchTorrents(keyword string, baseUrl string) ([]site.Torrent, error) {
	return nil, fmt.Errorf("not implemented yet")
}

func (dzsite *Site) DownloadTorrent(torrentUrl string) ([]byte, string, error) {
	if !utils.IsUrl(torrentUrl) {
		id := strings.TrimPrefix(torrentUrl, dzsite.GetName()+".")
		return dzsite.DownloadTorrentById(id)
	}
	threadUrl := regexp.MustCompile(`mod=viewthread&tid=(?P<id>\d+)\b`)
	if threadUrl.MatchString(torrentUrl) {
		doc, err := utils.GetUrlDoc(torrentUrl, dzsite.HttpClient,
			dzsite.GetSiteConfig().Cookie, dzsite.SiteConfig.UserAgent, nil)
		if err != nil {
			return nil, "", fmt.Errorf("failed to get thread doc: %v", err)
		}
		dlLink := doc.Find(`a[href^="download.php?id="]`).AttrOr("href", "")
		idRegexp := regexp.MustCompile(`\bid=(?P<id>\d+)\b`)
		m := idRegexp.FindStringSubmatch(dlLink)
		if m == nil {
			return nil, "", fmt.Errorf("no torrent download link found")
		}
		return dzsite.DownloadTorrentById(m[idRegexp.SubexpIndex("id")])
	}
	return site.DownloadTorrentByUrl(dzsite, dzsite.HttpClient, torrentUrl, "")
}

func (dzsite *Site) DownloadTorrentById(id string) ([]byte, string, error) {
	torrentUrl := dzsite.SiteConfig.Url + "download.php?id=" + id
	return site.DownloadTorrentByUrl(dzsite, dzsite.HttpClient, torrentUrl, id)
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
	}
	return site, nil
}

func init() {
	site.Register(&site.RegInfo{
		Name:    "discuz",
		Creator: NewSite,
	})
}
