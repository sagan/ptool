package torrenttrader

// torrenttrader is a legacy torrents software in php
// v3: https://github.com/MicrosoulV3/TorrentTrader-v3
// used by: aidoru-online.me
// torrent download url: https://aidoru-online.me/download.php?id=12345

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

// PublishTorrent implements site.Site.
func (usite *Site) PublishTorrent(contents []byte, metadata url.Values) (id string, err error) {
	return "", site.ErrUnimplemented
}

const (
	SELECTOR_USERNAME        = `.myBlock:has(a[href$="account.php"]) .myBlock-caption`
	SELECTOR_USER_UPLOADED   = `.myBlock:has(a[href$="account.php"]) tr:has(td:contains("Uploaded")) td:last-child`
	SELECTOR_USER_DOWNLOADED = `.myBlock:has(a[href$="account.php"]) tr:has(td:contains("Downloaded")) td:last-child`
)

func (usite *Site) GetDefaultHttpHeaders() [][]string {
	return usite.HttpHeaders
}

func (usite *Site) PurgeCache() {
}

func (usite *Site) GetName() string {
	return usite.Name
}

func (usite *Site) GetSiteConfig() *config.SiteConfigStruct {
	return usite.SiteConfig
}

func (usite *Site) GetStatus() (*site.Status, error) {
	doc, _, err := util.GetUrlDocWithAzuretls(usite.SiteConfig.Url, usite.HttpClient,
		usite.GetSiteConfig().Cookie, site.GetUa(usite), usite.GetDefaultHttpHeaders())
	if err != nil {
		return nil, err
	}
	userNameSelector := SELECTOR_USERNAME
	userUploadedSelector := SELECTOR_USER_UPLOADED
	userDownloadedSelector := SELECTOR_USER_DOWNLOADED
	if usite.SiteConfig.SelectorUserInfoUserName != "" {
		userNameSelector = usite.SiteConfig.SelectorUserInfoUserName
	}
	if usite.SiteConfig.SelectorUserInfoUploaded != "" {
		userUploadedSelector = usite.SiteConfig.SelectorUserInfoUploaded
	}
	if usite.SiteConfig.SelectorUserInfoDownloaded != "" {
		userDownloadedSelector = usite.SiteConfig.SelectorUserInfoDownloaded
	}
	usernameEl := doc.Find(userNameSelector)
	uploadedEl := doc.Find(userUploadedSelector)
	downloadedEl := doc.Find(userDownloadedSelector)
	userUploaded, _ := util.ExtractSizeStr(util.DomSanitizedText(uploadedEl))
	userDownloaded, _ := util.ExtractSizeStr(util.DomSanitizedText(downloadedEl))
	return &site.Status{
		UserName:       util.DomSanitizedText(usernameEl),
		UserUploaded:   userUploaded,
		UserDownloaded: userDownloaded,
	}, nil
}

func (usite *Site) GetAllTorrents(sort string, desc bool, pageMarker string, baseUrl string) (
	torrents []*site.Torrent, nextPageMarker string, err error) {
	return nil, "", site.ErrUnimplemented
}

func (usite *Site) GetLatestTorrents(full bool) ([]*site.Torrent, error) {
	return nil, site.ErrUnimplemented
}

func (usite *Site) SearchTorrents(keyword string, baseUrl string) ([]*site.Torrent, error) {
	return nil, site.ErrUnimplemented
}

func (usite *Site) DownloadTorrent(torrentUrl string) (content []byte, filename string, id string, err error) {
	if !util.IsUrl(torrentUrl) {
		id = strings.TrimPrefix(torrentUrl, usite.GetName()+".")
		content, filename, err = usite.DownloadTorrentById(id)
		return
	}
	if urlObj, err := url.Parse(torrentUrl); err == nil {
		id = urlObj.Query().Get("id")
	}
	if !strings.Contains(torrentUrl, "/download.php") && id != "" {
		content, filename, err = usite.DownloadTorrentById(id)
		return
	}
	content, filename, err = site.DownloadTorrentByUrl(usite, usite.HttpClient, torrentUrl, id)
	return
}

func (usite *Site) DownloadTorrentById(id string) ([]byte, string, error) {
	torrentUrl := usite.SiteConfig.ParseSiteUrl("download.php?id="+id, false)
	return site.DownloadTorrentByUrl(usite, usite.HttpClient, torrentUrl, id)
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
		Name:    "torrenttrader",
		Creator: NewSite,
	})
}
