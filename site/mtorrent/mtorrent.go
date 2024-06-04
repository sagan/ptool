package mtorrent

import (
	"encoding/json"
	"fmt"
	"net/http"
	neturl "net/url"
	"strings"

	"github.com/Noooste/azuretls-client"
	"github.com/sagan/ptool/config"
	"github.com/sagan/ptool/constants"
	"github.com/sagan/ptool/site"
	"github.com/sagan/ptool/util"
	log "github.com/sirupsen/logrus"
)

const (
	APIPath_GenerateDownloadToken = "/api/torrent/genDlToken"
	APIPath_TorrentSearch         = "/api/torrent/search"
	APIPath_Profile               = "/api/member/profile"
)

var (
	// sortFields: https://test2.m-team.cc/api/doc.html#/%E5%B8%B8%E8%A7%84%E6%8E%A5%E5%8F%A3/%E7%A8%AE%E5%AD%90/search
	sortFields = map[string]string{
		"time": "CREATED_DATE",
		"size": "SIZE",
	}

	downloadMultipliers = map[string]float64{
		"FREE":           0,
		"_2X_FREE":       0,
		"PERCENT_50":     0.5,
		"_2X_PERCENT_50": 0.5,
		"PERCENT_70":     0.3,
	}

	uploadMultipliers = map[string]float64{
		"_2X_FREE":       2,
		"_2X_PERCENT_50": 2,
	}
)

var _ site.Site = (*Site)(nil)

type Site struct {
	Name        string
	SiteConfig  *config.SiteConfigStruct
	HttpClient  *azuretls.Session
	HttpHeaders [][]string
}

// PublishTorrent implements site.Site.
func (m *Site) PublishTorrent(contents []byte, metadata neturl.Values) (id string, err error) {
	return "", site.ErrUnimplemented
}

func (m *Site) GetName() string {
	return m.Name
}

func (m *Site) GetDefaultHttpHeaders() [][]string {
	return m.HttpHeaders
}

func (m *Site) GetSiteConfig() *config.SiteConfigStruct {
	return m.SiteConfig
}

// DownloadTorrent download torrent by url like `https://kp.m-team.cc/api/rss/dl?credential=xxx`
// if url is id, find real url by this id then call DownloadTorrentById
func (m *Site) DownloadTorrent(url string) (content []byte, filename string, id string, err error) {
	if !util.IsUrl(url) {
		// not url, try id
		id = strings.TrimPrefix(url, m.GetName()+".")
		content, filename, err = m.DownloadTorrentById(id)
		return
	}

	content, filename, err = site.DownloadTorrentByUrl(m, m.HttpClient, url, id)
	return
}

// DownloadTorrentById download torrent by id
func (m *Site) DownloadTorrentById(id string) (content []byte, filename string, err error) {
	q := make(neturl.Values)
	q.Add("id", id)
	var resp DownloadTokenResponse
	if err = m.do(APIPath_GenerateDownloadToken, q, nil, &resp); err != nil {
		err = fmt.Errorf("%s error: %w", APIPath_GenerateDownloadToken, err)
		return
	}

	log.Tracef("download torrent url %v", resp.Data)
	content, filename, err = site.DownloadTorrentByUrl(m, m.HttpClient, resp.Data, id)
	return
}

func (m *Site) GetLatestTorrents(full bool) ([]*site.Torrent, error) {
	modes := []string{TorrentSearchMode_Normal}
	if full {
		modes = append(modes, TorrentSearchMode_Adult)
	}

	var mergedTorrents []*site.Torrent
	for _, mode := range modes {
		if list, err := m.search(WithMode(mode)); err != nil {
			log.Errorf("search mode %s failed: %v", mode, err)
			continue
		} else {
			torrents := m.convertTorrents(list)
			mergedTorrents = append(mergedTorrents, torrents...)
		}
	}

	if len(mergedTorrents) == 0 {
		return nil, fmt.Errorf("found 0 torrents")
	}

	return mergedTorrents, nil
}

func (m *Site) GetAllTorrents(sort string, desc bool, pageMarker string, baseUrl string) (torrents []*site.Torrent, nextPageMarker string, err error) {
	if sort != "" && sort != constants.NONE && sortFields[sort] == "" {
		err = fmt.Errorf("unsupported sort field: %s", sort)
		return
	}

	var pageNumber int64 = 1
	if pageMarker != "" {
		pageNumber = util.ParseInt(pageMarker)
	}

	// pageNumber starts from 1, NOT 0
	if pageNumber < 1 {
		err = fmt.Errorf("page number must be greater than 0")
		return
	}

	list, err := m.search(WithSortDirection(desc),
		WithSortField(sortFields[sort]),
		WithPageNumber(pageNumber))
	if err != nil {
		return nil, "", err
	}

	torrents = m.convertTorrents(list)
	if list.TotalPages.Value() > pageNumber {
		nextPageMarker = fmt.Sprintf("%d", pageNumber+1)
	}
	return
}

func (m *Site) SearchTorrents(keyword string, baseUrl string) (torrents []*site.Torrent, err error) {
	list, err := m.search(WithKeyword(keyword))
	if err == nil {
		torrents = m.convertTorrents(list)
	}
	return
}

func (m *Site) GetStatus() (*site.Status, error) {
	var resp ProfileResponse
	if err := m.do(APIPath_Profile, nil, nil, &resp); err != nil {
		return nil, err
	} else {
		return &site.Status{
			UserName:            resp.Data.UserName,
			UserDownloaded:      resp.Data.MemberCount.Downloaded.Value(),
			UserUploaded:        resp.Data.MemberCount.Uploaded.Value(),
			TorrentsSeedingCnt:  0,
			TorrentsLeechingCnt: 0,
		}, nil
	}
}

func (m *Site) PurgeCache() {
}

func (m *Site) do(path string, query neturl.Values, body any, result any) error {
	fullPath, err := neturl.JoinPath(m.SiteConfig.Url, path)
	if err != nil {
		return err
	}

	if len(query) > 0 {
		fullPath = fullPath + "?" + query.Encode()
	}

	reqHeaders := util.GetHttpReqHeaders(m.GetDefaultHttpHeaders(), m.GetSiteConfig().Cookie, site.GetUa(m))
	res, err := m.HttpClient.Do(&azuretls.Request{
		Method:   http.MethodPost,
		Url:      fullPath,
		Body:     body,
		NoCookie: true, // disable azuretls internal cookie jar
	}, reqHeaders)
	if err != nil {
		return fmt.Errorf("failed to fetch url: %w", err)
	}
	log.Tracef("Azuretls.Do response status=%d", res.StatusCode)
	if res.StatusCode != 200 {
		return fmt.Errorf("failed to fetch url: status=%d", res.StatusCode)
	}

	if err := json.Unmarshal(res.Body, result); err != nil {
		return fmt.Errorf("unmarshal response as json error: %w", err)
	}

	if c, ok := result.(errorGetter); ok {
		return c.GetError()
	}

	return nil
}

func (m *Site) search(options ...TorrentSearchRequestOption) (list *TorrentList, err error) {
	req := NewTorrentSearchRequest(options...)
	var resp TorrentSearchResponse
	if err = m.do(APIPath_TorrentSearch, nil, req, &resp); err != nil {
		return
	}

	list = &resp.Data
	return
}

func (m *Site) convertTorrents(list *TorrentList) []*site.Torrent {
	var torrents []*site.Torrent
	for _, torrent := range list.Data {
		torrents = append(torrents, &site.Torrent{
			Name:               torrent.Name,
			Description:        torrent.Description,
			Id:                 fmt.Sprintf("%s.%s", m.Name, torrent.Id),
			InfoHash:           "",
			DownloadUrl:        torrent.Id, // can't get download url in list, must call /api/torrent/genDlToken after
			DownloadMultiplier: getMultiplier(downloadMultipliers, torrent.Status.Discount),
			UploadMultiplier:   getMultiplier(uploadMultipliers, torrent.Status.Discount),
			DiscountEndTime:    torrent.Status.DiscountEndTime.UnixWithDefault(-1),
			Time:               torrent.CreateDate.Unix(),
			Size:               torrent.Size.Value(),
			IsSizeAccurate:     true,
			Seeders:            torrent.Status.Seeders.Value(),
			Leechers:           torrent.Status.Leechers.Value(),
			Snatched:           0,
			HasHnR:             false,
			IsActive:           false, // TODO: maybe torrent.clientList[*].downloaded > 0?
			Paid:               false,
			Bought:             false,
			Neutral:            false,
		})
	}

	return torrents
}

func getMultiplier(config map[string]float64, discount string) float64 {
	if v, ok := config[discount]; ok {
		return v
	}
	// default discount is 100%
	return 1
}

func NewSite(name string, siteConfig *config.SiteConfigStruct, config *config.ConfigStruct) (site.Site, error) {
	httpClient, httpHeaders, err := site.CreateSiteHttpClient(siteConfig, config)
	if err != nil {
		return nil, fmt.Errorf("failed to create site http client: %w", err)
	}
	s := &Site{
		Name:        name,
		SiteConfig:  siteConfig,
		HttpClient:  httpClient,
		HttpHeaders: httpHeaders,
	}
	return s, nil
}

func init() {
	site.Register(&site.RegInfo{
		Name:    "mtorrent",
		Creator: NewSite,
	})
}
