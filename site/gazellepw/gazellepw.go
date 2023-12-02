package gazellepw

// GazellePW ( https://github.com/Mosasauroidea/GazellePW )
// 海豹( https://greatposterwall.com/ )使用架构
// 种子下载链接：https://greatposterwall.com/torrents.php?action=download&id={id}&authkey={authkey}&torrent_pass={torrent_pass} (如果cookie有效，authkey 和 torrent_pass 可省略)
// 注意下载时的 id 与 torrent.php 页面url里的 id 不同，后者是当前电影整个分组的 id

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/sagan/ptool/config"
	"github.com/sagan/ptool/site"
	"github.com/sagan/ptool/util"
)

type Site struct {
	Name       string
	Location   *time.Location
	SiteConfig *config.SiteConfigStruct
	Config     *config.ConfigStruct
	HttpClient *http.Client
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
	doc, _, err := util.GetUrlDoc(gpwsite.SiteConfig.Url+"torrents.php", gpwsite.HttpClient,
		gpwsite.GetSiteConfig().Cookie, gpwsite.SiteConfig.UserAgent, site.GetHttpHeaders(gpwsite))
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
	torrents []site.Torrent, nextPageMarker string, err error) {
	return nil, "", fmt.Errorf("not implemented yet")
}

func (gpwsite *Site) GetLatestTorrents(full bool) ([]site.Torrent, error) {
	return nil, fmt.Errorf("not implemented yet")
}

func (gpwsite *Site) SearchTorrents(keyword string, baseUrl string) ([]site.Torrent, error) {
	return nil, fmt.Errorf("not implemented yet")
}

func (gpwsite *Site) DownloadTorrent(torrentUrl string) ([]byte, string, error) {
	if !util.IsUrl(torrentUrl) {
		id := strings.TrimPrefix(torrentUrl, gpwsite.GetName()+".")
		return gpwsite.DownloadTorrentById(id)
	}
	urlObj, err := url.Parse(torrentUrl)
	id := ""
	if err == nil {
		id = urlObj.Query().Get("id")
	}
	return site.DownloadTorrentByUrl(gpwsite, gpwsite.HttpClient, torrentUrl, id)
}

func (gpwsite *Site) DownloadTorrentById(id string) ([]byte, string, error) {
	torrentUrl := gpwsite.SiteConfig.Url + "torrents.php?action=download&id=" + id
	return site.DownloadTorrentByUrl(gpwsite, gpwsite.HttpClient, torrentUrl, id)
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
		Name:    "gazellepw",
		Creator: NewSite,
	})
}
