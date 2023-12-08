package iyuu

import (
	"encoding/json"
	"fmt"
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
	Is_https      int64  `json:"is_https"`      // 1 / 0
	Reseed_check  string `json:"reseed_check"`  // "passkey"
}

type IyuuApiRecommendSite struct {
	Id         int64  `json:"id"`
	Site       string `json:"site"`
	Bind_check string `json:"bind_check"`
}

type IyuuApiResponse struct {
	Ret  int64          `json:"ret"`
	Msg  string         `json:"msg"`
	Data map[string]any `json:"data"`
}

type IyuuApiGetUserResponse struct {
	Ret  int64  `json:"ret"`
	Msg  string `json:"msg"`
	Data struct {
		User map[string]any `json:"user"`
	} `json:"data"`
}

type IyuuApiSitesResponse struct {
	Ret  int64  `json:"ret"`
	Msg  string `json:"msg"`
	Data struct {
		Sites []IyuuApiSite `json:"sites"`
	} `json:"data"`
}

type IyuuGetRecommendSitesResponse struct {
	Ret  int64  `json:"ret"`
	Msg  string `json:"msg"`
	Data struct {
		Recommend []IyuuApiRecommendSite `json:"recommend"`
	} `json:"data"`
}

type IyuuApiHashResponse struct {
	Ret  int64  `json:"ret"`
	Msg  string `json:"msg"`
	Data []struct {
		Hash    string                `json:"hash"`
		Torrent []IyuuTorrentInfoHash `json:"torrent"`
	} `json:"data"`
}

type IyuuTorrentInfoHash struct {
	Sid        int64  `json:"sid"`
	Torrent_id int64  `json:"torrent_id"`
	Info_hash  string `json:"info_hash"`
}

const IYUU_VERSION = "2.0.0"

// https://api.iyuu.cn/docs.php?service=App.Api.Infohash&detail=1&type=fold
func IyuuApiHash(token string, infoHashes []string) (map[string][]IyuuTorrentInfoHash, error) {
	infoHashes = util.CopySlice(infoHashes)
	for i, infoHash := range infoHashes {
		infoHashes[i] = strings.ToLower(infoHash)
	}
	sort.Slice(infoHashes, func(i, j int) bool {
		return infoHashes[i] < infoHashes[j]
	})
	hash, _ := json.Marshal(&infoHashes)
	apiUrl := "https://api.iyuu.cn/index.php?s=App.Api.Hash"
	data := url.Values{
		"sign":      {token},
		"timestamp": {fmt.Sprint(util.Now())},
		"version":   {IYUU_VERSION},
		"hash":      {string(hash)},
		"sha1":      {util.Sha1(hash)},
	}
	resData := &IyuuApiHashResponse{}
	err := util.PostUrlForJson(apiUrl, data, &resData, nil)
	log.Tracef("ApiInfoHash response err=%v", err)
	if err != nil {
		return nil, err
	}
	if resData.Ret != 200 {
		return nil, fmt.Errorf("iyuu api error: ret=%d, msg=%s", resData.Ret, resData.Msg)
	}

	result := map[string][]IyuuTorrentInfoHash{}
	for _, data := range resData.Data {
		result[data.Hash] = data.Torrent
	}
	return result, nil
}

func IyuuApiGetUser(token string) (data map[string]any, err error) {
	err = util.FetchJson("https://api.iyuu.cn/index.php?s=App.Api.GetUser&sign="+token,
		&data, nil, "", "", nil)
	return
}

func IyuuApiSites(token string) ([]IyuuApiSite, error) {
	resData := &IyuuApiSitesResponse{}
	err := util.FetchJson("https://api.iyuu.cn/index.php?s=App.Api.Sites&version="+
		IYUU_VERSION+"&sign="+token,
		resData, nil, "", "", nil)
	if err != nil {
		return nil, err
	}
	if resData.Ret != 200 {
		return nil, fmt.Errorf("iyuu api error: ret=%d, msg=%s", resData.Ret, resData.Msg)
	}
	return resData.Data.Sites, nil
}

func IyuuApiBind(token string, site string, uid int64, passkey string) (map[string]any, error) {
	apiUrl := "https://api.iyuu.cn/index.php?s=App.Api.Bind&token=" + token +
		"&site=" + site + "&id=" + fmt.Sprint(uid) + "&passkey=" + util.Sha1String(passkey)

	resData := &IyuuApiResponse{}
	err := util.FetchJson(apiUrl, &resData, nil, "", "", nil)
	if err != nil {
		return nil, err
	}
	if resData.Ret != 200 {
		return nil, fmt.Errorf("iyuu api error: ret=%d, msg=%s", resData.Ret, resData.Msg)
	}
	return resData.Data, nil
}

func IyuuApiGetRecommendSites() ([]IyuuApiRecommendSite, error) {
	apiUrl := "https://api.iyuu.cn/index.php?s=App.Api.GetRecommendSites"

	var resData *IyuuGetRecommendSitesResponse
	err := util.FetchJson(apiUrl, &resData, nil, "", "", nil)
	if err != nil {
		return nil, err
	}
	if resData.Ret != 200 {
		return nil, fmt.Errorf("iyuu api error: ret=%d, msg=%s", resData.Ret, resData.Msg)
	}
	return resData.Data.Recommend, nil
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
			return regInfo != nil && (regInfo.Name == iyuuSite.Name || slices.Index(regInfo.Aliases, iyuuSite.Name) != -1)
		})
		if localSite != nil {
			iyuu2LocalSiteMap[iyuuSite.Sid] = (*localSite).GetName()
		}
	}
	return iyuu2LocalSiteMap
}
