package tnode

// TNode
// 朱雀( https://zhuque.in/index )自研架构
// 种子下载链接：https://zhuque.in/api/torrent/download/{id}/{torrent_key} (如果cookie有效，url最后一段可省略)

import (
	"fmt"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/Noooste/azuretls-client"
	log "github.com/sirupsen/logrus"

	"github.com/sagan/ptool/config"
	"github.com/sagan/ptool/site"
	"github.com/sagan/ptool/util"
)

type Site struct {
	Name        string
	Location    *time.Location
	SiteConfig  *config.SiteConfigStruct
	Config      *config.ConfigStruct
	HttpClient  *azuretls.Session
	HttpHeaders [][]string
	csrfToken   string
}

// PublishTorrent implements site.Site.
func (tnsite *Site) PublishTorrent(contents []byte, metadata url.Values) (id string, err error) {
	return "", site.ErrUnimplemented
}

func (tnsite *Site) GetDefaultHttpHeaders() [][]string {
	return tnsite.HttpHeaders
}

func (tnsite *Site) syncCsrfToken() error {
	if tnsite.csrfToken != "" {
		return nil
	}
	doc, _, err := util.GetUrlDocWithAzuretls(tnsite.SiteConfig.Url, tnsite.HttpClient,
		tnsite.GetSiteConfig().Cookie, site.GetUa(tnsite), tnsite.GetDefaultHttpHeaders())
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
	headers := [][]string{
		{"x-csrf-token", tnsite.csrfToken},
	}
	err = util.FetchJsonWithAzuretls(apiUrl, data, tnsite.HttpClient,
		tnsite.SiteConfig.Cookie, site.GetUa(tnsite), headers)
	if err != nil {
		return nil, fmt.Errorf("failed to get use status: %w", err)
	}
	return &site.Status{
		UserName:       data.Data.Username,
		UserDownloaded: data.Data.Download,
		UserUploaded:   data.Data.Upload,
	}, nil
}

func (tnsite *Site) GetAllTorrents(sort string, desc bool, pageMarker string, baseUrl string) (
	torrents []*site.Torrent, nextPageMarker string, err error) {
	return nil, "", site.ErrUnimplemented
}

func (tnsite *Site) GetLatestTorrents(full bool) ([]*site.Torrent, error) {
	return nil, site.ErrUnimplemented
}

func (tnsite *Site) SearchTorrents(keyword string, baseUrl string) ([]*site.Torrent, error) {
	return nil, site.ErrUnimplemented
}

func (tnsite *Site) DownloadTorrent(torrentUrl string) (content []byte, filename string, id string, err error) {
	if !util.IsUrl(torrentUrl) {
		id = strings.TrimPrefix(torrentUrl, tnsite.GetName()+".")
		content, filename, err = tnsite.DownloadTorrentById(id)
		return
	}
	if !strings.Contains(torrentUrl, "api/torrent/download/") {
		idRegexp := regexp.MustCompile(`/info/(?P<id>\d+)\b`)
		if m := idRegexp.FindStringSubmatch(torrentUrl); m != nil {
			id = m[idRegexp.SubexpIndex("id")]
			content, filename, err = tnsite.DownloadTorrentById(id)
			return
		}
	}
	idRegexp := regexp.MustCompile(`/download/(?P<id>\d+)\b`)
	if m := idRegexp.FindStringSubmatch(torrentUrl); m != nil {
		id = m[idRegexp.SubexpIndex("id")]
	}
	content, filename, err = site.DownloadTorrentByUrl(tnsite, tnsite.HttpClient, torrentUrl, id)
	return
}

func (tnsite *Site) DownloadTorrentById(id string) ([]byte, string, error) {
	torrentUrl := tnsite.SiteConfig.Url + "api/torrent/download/" + id
	return site.DownloadTorrentByUrl(tnsite, tnsite.HttpClient, torrentUrl, id)
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
	}
	return site, nil
}

func init() {
	site.Register(&site.RegInfo{
		Name:    "tnode",
		Creator: NewSite,
	})
}
