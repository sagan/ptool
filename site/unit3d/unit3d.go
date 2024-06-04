package unit3d

// UNIT3D ( https://github.com/HDInnovations/UNIT3D-Community-Edition )
// JptvClub、莫妮卡、普斯特等站使用架构
// 种子下载链接格式：https://jptv.club/torrents/download/39683

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
}

// PublishTorrent implements site.Site.
func (usite *Site) PublishTorrent(contents []byte, metadata url.Values) (id string, err error) {
	return "", site.ErrUnimplemented
}

const (
	SELECTOR_USERNAME        = ".top-nav__username"
	SELECTOR_USER_UPLOADED   = ".ratio-bar__uploaded"
	SELECTOR_USER_DOWNLOADED = ".ratio-bar__downloaded"
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
	doc, res, err := util.GetUrlDocWithAzuretls(usite.SiteConfig.Url+"torrents", usite.HttpClient,
		usite.GetSiteConfig().Cookie, site.GetUa(usite), usite.GetDefaultHttpHeaders())
	if err != nil {
		return nil, err
	}
	if strings.Contains(res.Request.Url, "/login") {
		return nil, fmt.Errorf("not logined (cookie may has expired)")
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
	if !strings.Contains(torrentUrl, "/torrents/download/") {
		idRegexp := regexp.MustCompile(`torrents/(?P<id>\d+)\b`)
		if m := idRegexp.FindStringSubmatch(torrentUrl); m != nil {
			id = m[idRegexp.SubexpIndex("id")]
			content, filename, err = usite.DownloadTorrentById(id)
			return
		}
	}
	idRegexp := regexp.MustCompile(`/download/(?P<id>\d+)\b`)
	if m := idRegexp.FindStringSubmatch(torrentUrl); m != nil {
		id = m[idRegexp.SubexpIndex("id")]
	}
	content, filename, err = site.DownloadTorrentByUrl(usite, usite.HttpClient, torrentUrl, id)
	return
}

func (usite *Site) DownloadTorrentById(id string) ([]byte, string, error) {
	torrentUrl := usite.SiteConfig.Url + "torrents/download/" + id
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
		Name:    "unit3d",
		Creator: NewSite,
	})
}
