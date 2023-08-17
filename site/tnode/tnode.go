package tnode

// TNode
// 朱雀( https://zhuque.in/index )自研架构
// 种子下载链接：https://zhuque.in/api/torrent/download/{id}/{torrent_key} (如果cookie有效，url最后一段可省略)

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
	csrfToken  string
}

func (tnsite *Site) syncCsrfToken() error {
	if tnsite.csrfToken != "" {
		return nil
	}
	doc, _, err := utils.GetUrlDoc(tnsite.SiteConfig.Url, tnsite.HttpClient,
		tnsite.GetSiteConfig().Cookie, tnsite.SiteConfig.UserAgent, nil)
	if err != nil {
		return err
	}
	token := doc.Find(`meta[name="x-csrf-token"]`).AttrOr("content", "")
	if token == "" {
		return fmt.Errorf("no x-csrf-token meta found")
	}
	tnsite.csrfToken = token
	return nil
}

func (tnsite *Site) PurgeCache() {
}

func (tnsite *Site) GetName() string {
	return tnsite.Name
}

func (tnsite *Site) GetSiteConfig() *config.SiteConfigStruct {
	return tnsite.SiteConfig
}

func (tnsite *Site) GetStatus() (*site.Status, error) {
	err := tnsite.syncCsrfToken()
	if err != nil {
		return nil, fmt.Errorf("failed to get csrf token")
	}

	var data = &apiMainInfoResponse{}
	apiUrl := tnsite.SiteConfig.Url + "api/user/getMainInfo"
	headers := map[string]string{
		"x-csrf-token": tnsite.csrfToken,
	}
	err = utils.FetchJson(apiUrl, data, tnsite.HttpClient,
		tnsite.SiteConfig.Cookie, tnsite.SiteConfig.UserAgent, headers)
	if err != nil {
		return nil, fmt.Errorf("failed to get use status: %v", err)
	}
	return &site.Status{
		UserName:       data.Data.Username,
		UserDownloaded: data.Data.Download,
		UserUploaded:   data.Data.Upload,
	}, nil
}

func (tnsite *Site) GetAllTorrents(sort string, desc bool, pageMarker string, baseUrl string) (
	torrents []site.Torrent, nextPageMarker string, err error) {
	return nil, "", fmt.Errorf("not implemented yet")
}

func (tnsite *Site) GetLatestTorrents(full bool) ([]site.Torrent, error) {
	return nil, fmt.Errorf("not implemented yet")
}

func (tnsite *Site) SearchTorrents(keyword string, baseUrl string) ([]site.Torrent, error) {
	return nil, fmt.Errorf("not implemented yet")
}

func (tnsite *Site) DownloadTorrent(torrentUrl string) ([]byte, string, error) {
	if !utils.IsUrl(torrentUrl) {
		id := strings.TrimPrefix(torrentUrl, tnsite.GetName()+".")
		return tnsite.DownloadTorrentById(id)
	}
	if !strings.Contains(torrentUrl, "api/torrent/download/") {
		idRegexp := regexp.MustCompile(`/info/(?P<id>\d+)\b`)
		m := idRegexp.FindStringSubmatch(torrentUrl)
		if m != nil {
			return tnsite.DownloadTorrentById(m[idRegexp.SubexpIndex("id")])
		}
	}
	id := ""
	idRegexp := regexp.MustCompile(`/download/(?P<id>\d+)\b`)
	m := idRegexp.FindStringSubmatch(torrentUrl)
	if m != nil {
		id = m[idRegexp.SubexpIndex("id")]
	}
	return site.DownloadTorrentByUrl(tnsite, tnsite.HttpClient, torrentUrl, id)
}

func (tnsite *Site) DownloadTorrentById(id string) ([]byte, string, error) {
	torrentUrl := tnsite.SiteConfig.Url + "api/torrent/download/" + id
	return site.DownloadTorrentByUrl(tnsite, tnsite.HttpClient, torrentUrl, id)
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
		Name:    "tnode",
		Creator: NewSite,
	})
}
