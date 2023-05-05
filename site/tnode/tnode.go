package tnode

// TNode
// 朱雀( https://zhuque.in/index )自研架构
// 种子下载链接：https://zhuque.in/api/torrent/download/{id}/{torrent_key} (如果cookie有效，url最后一段可省略)

import (
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	cloudflarebp "github.com/DaRealFreak/cloudflare-bp-go"

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

func (tnsite *Site) PurgeCache() {
}

func (tnsite *Site) GetName() string {
	return tnsite.Name
}

func (tnsite *Site) GetSiteConfig() *config.SiteConfigStruct {
	return tnsite.SiteConfig
}

func (tnsite *Site) GetStatus() (*site.Status, error) {
	return nil, fmt.Errorf("not implemented yet")
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
	return site.DownloadTorrentByUrl(tnsite, tnsite.HttpClient, torrentUrl)
}

func (tnsite *Site) DownloadTorrentById(id string) ([]byte, string, error) {
	torrentUrl := tnsite.SiteConfig.Url + "api/torrent/download/" + id
	return site.DownloadTorrentByUrl(tnsite, tnsite.HttpClient, torrentUrl)
}

func NewSite(name string, siteConfig *config.SiteConfigStruct, config *config.ConfigStruct) (site.Site, error) {
	if siteConfig.Cookie == "" {
		return nil, fmt.Errorf("cann't create site: no cookie provided")
	}
	location, err := time.LoadLocation(siteConfig.Timezone)
	if err != nil {
		return nil, fmt.Errorf("invalid site timezone %s: %v", siteConfig.Timezone, err)
	}
	httpClient := &http.Client{}
	transport := &http.Transport{}
	proxy := siteConfig.Proxy
	if proxy == "" {
		proxy = config.SiteProxy
	}
	if proxy != "" {
		proxyUrl, err := url.Parse(proxy)
		if err != nil {
			return nil, fmt.Errorf("failed to parse siteProxy %s: %v", proxy, err)
		}
		transport.Proxy = http.ProxyURL(proxyUrl)
	}
	httpClient.Transport = cloudflarebp.AddCloudFlareByPass(transport, cloudflarebp.Options{
		AddMissingHeaders: false,
	})
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
