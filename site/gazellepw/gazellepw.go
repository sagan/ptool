package gazellepw

// GazellePW ( https://github.com/Mosasauroidea/GazellePW )
// 海豹( https://greatposterwall.com/ )使用架构
// 种子下载链接：https://greatposterwall.com/torrents.php?action=download&id={id}&authkey={key}&torrent_pass={pass}
// (如果cookie有效，key 和 pass 可省略)
// 注意下载时的 id 与 torrent.php 页面url里的 id 不同，后者是当前电影整个分组的 id

import (
	"fmt"
	"net/url"
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
}

func (gpwsite *Site) GetDefaultHttpHeaders() [][]string {
	return gpwsite.HttpHeaders
}

func (gpwsite *Site) PurgeCache() {
}

func (gpwsite *Site) GetName() string {
	return gpwsite.Name
}

func (gpwsite *Site) GetSiteConfig() *config.SiteConfigStruct {
	return gpwsite.SiteConfig
}

func (gpwsite *Site) GetStatus() (*site.Status, error) {
	doc, _, err := util.GetUrlDocWithAzuretls(gpwsite.SiteConfig.Url+"torrents.php", gpwsite.HttpClient,
		gpwsite.GetSiteConfig().Cookie, site.GetUa(gpwsite), gpwsite.GetDefaultHttpHeaders())
	if err != nil {
		return nil, err
	}
	usernameEl := doc.Find(`#header-username-value`)
	uploadedEl := doc.Find(`#header-uploaded-value`)
	downloadedEl := doc.Find(`#header-downloaded-value`)

	return &site.Status{
		UserName:       usernameEl.AttrOr("data-value", ""),
		UserUploaded:   util.ParseInt(uploadedEl.AttrOr("data-value", "")),
		UserDownloaded: util.ParseInt(downloadedEl.AttrOr("data-value", "")),
	}, nil
}

func (gpwsite *Site) GetAllTorrents(sort string, desc bool, pageMarker string, baseUrl string) (
	torrents []*site.Torrent, nextPageMarker string, err error) {
	return nil, "", fmt.Errorf("not implemented yet")
}

func (gpwsite *Site) GetLatestTorrents(full bool) ([]*site.Torrent, error) {
	return nil, fmt.Errorf("not implemented yet")
}

func (gpwsite *Site) SearchTorrents(keyword string, baseUrl string) ([]*site.Torrent, error) {
	return nil, fmt.Errorf("not implemented yet")
}

func (gpwsite *Site) DownloadTorrent(torrentUrl string) (content []byte, filename string, id string, err error) {
	if !util.IsUrl(torrentUrl) {
		id = strings.TrimPrefix(torrentUrl, gpwsite.GetName()+".")
		content, filename, err = gpwsite.DownloadTorrentById(id)
		return
	}
	if urlObj, err := url.Parse(torrentUrl); err == nil {
		id = urlObj.Query().Get("id")
	}
	content, filename, err = site.DownloadTorrentByUrl(gpwsite, gpwsite.HttpClient, torrentUrl, id)
	return
}

func (gpwsite *Site) DownloadTorrentById(id string) ([]byte, string, error) {
	torrentUrl := gpwsite.SiteConfig.Url + "torrents.php?action=download&id=" + id
	return site.DownloadTorrentByUrl(gpwsite, gpwsite.HttpClient, torrentUrl, id)
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
		Name:    "gazellepw",
		Creator: NewSite,
	})
}
