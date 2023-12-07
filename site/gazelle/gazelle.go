package gazelle

// Gazelle ( https://github.com/WhatCD/Gazelle )
// WhatCD 开源的音乐站 private tracker。
// 种子下载链接：https://dicmusic.club/torrents.php?action=download&id=&authkey=&torrent_pass= (如果cookie有效，authkey 和 torrent_pass 可省略)
// 注意下载时的 id 与 torrent.php 页面url里的 id 不同，后者是当前音乐专辑的 id

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"

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

const (
	SELECTOR_USERNAME        = "#nav_userinfo"
	SELECTOR_USER_UPLOADED   = "#stats_seeding"
	SELECTOR_USER_DOWNLOADED = "#stats_leeching"
)

func (gzsite *Site) PurgeCache() {
}

func (gzsite *Site) GetName() string {
	return gzsite.Name
}

func (gzsite *Site) GetSiteConfig() *config.SiteConfigStruct {
	return gzsite.SiteConfig
}

func (gzsite *Site) GetStatus() (*site.Status, error) {
	doc, _, err := util.GetUrlDoc(gzsite.SiteConfig.Url+"torrents.php", gzsite.HttpClient,
		gzsite.GetSiteConfig().Cookie, site.GetUa(gzsite), site.GetHttpHeaders(gzsite))
	if err != nil {
		return nil, err
	}
	userNameSelector := SELECTOR_USERNAME
	userUploadedSelector := SELECTOR_USER_UPLOADED
	userDownloadedSelector := SELECTOR_USER_DOWNLOADED
	if gzsite.SiteConfig.SelectorUserInfoUserName != "" {
		userNameSelector = gzsite.SiteConfig.SelectorUserInfoUserName
	}
	if gzsite.SiteConfig.SelectorUserInfoUploaded != "" {
		userUploadedSelector = gzsite.SiteConfig.SelectorUserInfoUploaded
	}
	if gzsite.SiteConfig.SelectorUserInfoDownloaded != "" {
		userDownloadedSelector = gzsite.SiteConfig.SelectorUserInfoDownloaded
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

func (gzsite *Site) GetAllTorrents(sort string, desc bool, pageMarker string, baseUrl string) (
	torrents []site.Torrent, nextPageMarker string, err error) {
	return nil, "", fmt.Errorf("not implemented yet")
}

func (gzsite *Site) GetLatestTorrents(full bool) ([]site.Torrent, error) {
	return nil, fmt.Errorf("not implemented yet")
}

func (gzsite *Site) SearchTorrents(keyword string, baseUrl string) ([]site.Torrent, error) {
	return nil, fmt.Errorf("not implemented yet")
}

func (gzsite *Site) DownloadTorrent(torrentUrl string) ([]byte, string, error) {
	if !util.IsUrl(torrentUrl) {
		id := strings.TrimPrefix(torrentUrl, gzsite.GetName()+".")
		return gzsite.DownloadTorrentById(id)
	}
	urlObj, err := url.Parse(torrentUrl)
	id := ""
	if err == nil {
		id = urlObj.Query().Get("id")
	}
	return site.DownloadTorrentByUrl(gzsite, gzsite.HttpClient, torrentUrl, id)
}

func (gzsite *Site) DownloadTorrentById(id string) ([]byte, string, error) {
	torrentUrl := gzsite.SiteConfig.Url + "torrents.php?action=download&id=" + id
	return site.DownloadTorrentByUrl(gzsite, gzsite.HttpClient, torrentUrl, id)
}

func NewSite(name string, siteConfig *config.SiteConfigStruct, config *config.ConfigStruct) (site.Site, error) {
	if siteConfig.Cookie == "" {
		log.Warnf("Site %s has no cookie provided", name)
	}
	location, err := time.LoadLocation(siteConfig.GetTimezone())
	if err != nil {
		return nil, fmt.Errorf("invalid site timezone %s: %v", siteConfig.GetTimezone(), err)
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
		Name:    "gazelle",
		Creator: NewSite,
	})
}
