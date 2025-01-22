package iyuu

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"slices"
	"sort"
	"strings"

	log "github.com/sirupsen/logrus"

	"github.com/sagan/ptool/config"
	"github.com/sagan/ptool/site"
	"github.com/sagan/ptool/util"
)

type IyuuApiSite struct {
	Id            int64  `json:"id"`
	Site          string `json:"site"`
	Base_url      string `json:"base_url"`
	Nickname      string `json:"nickname"`      // "朋友" / "馒头"
	Download_page string `json:"download_page"` // torrent download url. params: {passkey}, {authkey}, {} (id)
	Is_https      int64  `json:"is_https"`      // 2 / 1 / 0
}

type IyuuApiRecommendSite struct {
	Id         int64  `json:"id"`
	Site       string `json:"site"`
	Nickname   string `json:"nickname"`
	Bind_check string `json:"bind_check"`
}

type IyuuApiResponse struct {
	Code int64  `json:"code"`
	Msg  string `json:"msg"`
	Data any    `json:"data"`
}

type IyuuApiReportExistingResponse struct {
	Code int64  `json:"code"`
	Msg  string `json:"msg"`
	Data struct {
		SidSha1 string `json:"sid_sha1"`
	} `json:"data"`
}

type IyuuApiReportExistingRequest struct {
	SidList []int64 `json:"sid_list"`
}

type IyuuApiSitesResponse struct {
	Code int64  `json:"code"`
	Msg  string `json:"msg"`
	Data struct {
		Sites []*IyuuApiSite `json:"sites"`
	} `json:"data"`
}

type IyuuGetRecommendSitesResponse struct {
	Code int64  `json:"code"`
	Msg  string `json:"msg"`
	Data struct {
		List []IyuuApiRecommendSite `json:"list"`
	} `json:"data"`
}

type IyuuApiHashResponse struct {
	Code int64  `json:"code"`
	Msg  string `json:"msg"`
	Data map[string]*struct {
		Torrent []IyuuTorrentInfoHash `json:"torrent"`
	} `json:"data"`
}

type IyuuTorrentInfoHash struct {
	Sid        int64  `json:"sid"`
	Torrent_id int64  `json:"torrent_id"`
	Info_hash  string `json:"info_hash"`
}

const IYUU_VERSION = "8.2.0"
const MAX_INTOHASH_NUMBER = 300 // 单次提交种子info_hash最多300个

// https://doc.iyuu.cn/reference/reseed_index
func IyuuApiHash(token string, infoHashes []string, sid_sha1 string) (map[string][]IyuuTorrentInfoHash, error) {
	header := http.Header{}
	header.Set("Token", token)

	infoHashes = util.CopySlice(infoHashes)
	for i, infoHash := range infoHashes {
		infoHashes[i] = strings.ToLower(infoHash)
	}
	sort.Slice(infoHashes, func(i, j int) bool {
		return infoHashes[i] < infoHashes[j]
	})
	hash, _ := json.Marshal(&infoHashes)
	apiUrl := util.ParseRelativeUrl("/reseed/index/index", config.Get().GetIyuuDomain())

	data := url.Values{
		"sid_sha1":  {sid_sha1},
		"timestamp": {fmt.Sprint(util.Now())},
		"version":   {IYUU_VERSION},
		"hash":      {string(hash)},
		"sha1":      {util.Sha1(hash)},
	}
	resData := &IyuuApiHashResponse{}
	err := util.PostUrlForJson(apiUrl, data, &resData, header, nil)
	log.Tracef("ApiInfoHash response err=%v", err)
	if err != nil {
		return nil, err
	}
	if resData.Code != 0 {
		return nil, fmt.Errorf("iyuu api error: code=%d, msg=%s", resData.Code, resData.Msg)
	}

	result := map[string][]IyuuTorrentInfoHash{}
	for infoHash, data := range resData.Data {
		result[infoHash] = data.Torrent
	}
	return result, nil
}

// Get iyuu user profile.
// New profile api is undocumented (yet) in https://doc.iyuu.cn/.
// See: https://github.com/ledccn/iyuuplus-dev/commit/2bf34e2ab3824caf0939eb8081c8a9d3e97736b0 .
func IyuuApiUsersProfile(token string) (data map[string]any, err error) {
	err = util.FetchJson(util.ParseRelativeUrl("/reseed/users/profile", config.Get().GetIyuuDomain()), &data, nil,
		http.Header{
			"Token": []string{token},
		})
	return
}

// https://doc.iyuu.cn/reference/site_list
func IyuuApiSites(token string) ([]*IyuuApiSite, error) {
	var resData *IyuuApiSitesResponse
	header := http.Header{}
	header.Set("Token", token)
	err := util.FetchJson(util.ParseRelativeUrl("/reseed/sites/index",
		config.Get().GetIyuuDomain()), &resData, nil, header)
	if err != nil {
		return nil, err
	}
	if resData.Code != 0 {
		return nil, fmt.Errorf("iyuu api error: code=%d, msg=%s", resData.Code, resData.Msg)
	}
	return resData.Data.Sites, nil
}

// https://doc.iyuu.cn/reference/site_report_existing
func IyuuApiReportExisting(token string, sites []*IyuuApiSite) (string, error) {
	header := http.Header{}
	header.Set("Token", token)
	reportExistingRequest := &IyuuApiReportExistingRequest{
		SidList: util.Map(sites, func(site *IyuuApiSite) int64 { return site.Id }),
	}
	var reportExistingResponse *IyuuApiReportExistingResponse
	err := util.PostAndFetchJson(util.ParseRelativeUrl("/reseed/sites/reportExisting",
		config.Get().GetIyuuDomain()), reportExistingRequest, &reportExistingResponse, header, nil)
	if err != nil {
		return "", err
	}
	if reportExistingResponse.Code != 0 {
		return "", fmt.Errorf("iyuu api error: code=%d, msg=%s", reportExistingResponse.Code, reportExistingResponse.Msg)
	}
	return reportExistingResponse.Data.SidSha1, nil
}

// https://doc.iyuu.cn/reference/users_bind
func IyuuApiBind(token string, site string, sid int64, uid int64, passkey string) (any, error) {
	apiUrl := util.ParseRelativeUrl("/reseed/users/bind", config.Get().GetIyuuDomain())
	header := http.Header{}
	header.Set("Token", token)
	data := url.Values{
		"token":   {token},
		"site":    {site},
		"sid":     {fmt.Sprint(sid)}, // 新版验证依赖的sid字段 2024年4月24日
		"id":      {fmt.Sprint(uid)},
		"passkey": {util.Sha1String(passkey)},
	}
	var resData *IyuuApiResponse
	err := util.PostUrlForJson(apiUrl, data, &resData, header, nil)
	if err != nil {
		return nil, err
	}
	// 400: 站点：hdhome 用户ID：114053，已被绑定过！绑定的UUID为：78
	// if resData.Code != 0 {
	// 	return nil, fmt.Errorf("iyuu api error: code=%d, msg=%s", resData.Code, resData.Msg)
	// }
	return resData, nil
}

func IyuuApiGetRecommendSites() ([]IyuuApiRecommendSite, error) {
	apiUrl := util.ParseRelativeUrl("/reseed/sites/recommend", config.Get().GetIyuuDomain())

	var resData *IyuuGetRecommendSitesResponse
	err := util.FetchJson(apiUrl, &resData, nil, nil)
	if err != nil {
		return nil, err
	}
	if resData.Code != 0 {
		return nil, fmt.Errorf("iyuu api error: code=%d, msg=%s", resData.Code, resData.Msg)
	}
	return resData.Data.List, nil
}

func (site *IyuuApiSite) GetUrl() string {
	siteUrl := "https://"
	if site.Is_https == 0 {
		siteUrl = "http://"
	}
	siteUrl += site.Base_url
	if !strings.Contains(site.Base_url, "/") {
		siteUrl += "/"
	}
	return siteUrl
}

func (iyuuApiSite IyuuApiSite) ToSite() Site {
	return Site{
		Sid:          iyuuApiSite.Id,
		Name:         iyuuApiSite.Site,
		Nickname:     iyuuApiSite.Nickname,
		Url:          iyuuApiSite.GetUrl(),
		DownloadPage: iyuuApiSite.Download_page,
	}
}

// return iyuuSid => localSiteName map
func GenerateIyuu2LocalSiteMap(iyuuSites []Site,
	localSites []*config.SiteConfigStruct) map[int64]string {
	iyuu2LocalSiteMap := map[int64]string{} // iyuu sid => local site name
	for _, iyuuSite := range iyuuSites {
		localSite := util.FindInSlice(localSites, func(siteConfig *config.SiteConfigStruct) bool {
			if siteConfig.Url != "" && siteConfig.Url == iyuuSite.Url {
				return true
			}
			regInfo := site.GetConfigSiteReginfo(siteConfig.GetName())
			return regInfo != nil && (regInfo.Name == iyuuSite.Name || slices.Contains(regInfo.Aliases, iyuuSite.Name))
		})
		if localSite != nil {
			iyuu2LocalSiteMap[iyuuSite.Sid] = (*localSite).GetName()
		}
	}
	return iyuu2LocalSiteMap
}
