package iyuu

import (
	"encoding/json"
	"fmt"
	"net/url"
	"sort"
	"strings"

	"github.com/sagan/ptool/config"
	"github.com/sagan/ptool/utils"

	log "github.com/sirupsen/logrus"
)

// IYUU 部分站点 name 与本程序有差异
var IYUU_SITE_IDS = map[string](string){
	"leaguehd":  "lemonhd",
	"pt0ffcc":   "0ff",
	"pt2xfree":  "2xFree",
	"redleaves": "leaves",
}

type IyuuSite struct {
	id      int64
	name    string
	canBind bool
}

type IyuuApiSite struct {
	Id            int64  `json:"id"`
	Site          string `json:"site"`
	Base_url      string `json:"base_url"`
	Nickname      string `json:"nickname"`      // "朋友" / "馒头"
	Download_page string `json:"download_page"` // torrent download url. params: {passkey}, {authkey}, {} (id)
	Is_https      int64  `json:"is_https"`      // 1 / 0
	Reseed_check  string `json:"reseed_check"`  // "passkey"
}

type IyuuApiResponse struct {
	Ret  int64            `json:"ret"`
	Msg  string           `json:"msg"`
	Data map[string](any) `json:"data"`
}

type IyuuApiGetUserResponse struct {
	Ret  int64  `json:"ret"`
	Msg  string `json:"msg"`
	Data struct {
		User map[string](any) `json:"user"`
	} `json:"data"`
}

type IyuuApiSitesResponse struct {
	Ret  int64  `json:"ret"`
	Msg  string `json:"msg"`
	Data struct {
		Sites []IyuuApiSite `json:"sites"`
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

var ITUU_SITES = []IyuuSite{
	{2, "pthome", true},
	{7, "hdhome", true},
	{9, "ourbits", true},
	{10, "hddolby", true},
	{25, "chdbits", true},
	{61, "hdai", true},
	{68, "audiences", true},
	{80, "zhuque", true},
}

// https://api.iyuu.cn/docs.php?service=App.Api.Infohash&detail=1&type=fold
func IyuuApiHash(token string, infoHashes []string) (map[string]([]IyuuTorrentInfoHash), error) {
	infoHashes = utils.CopySlice(infoHashes)
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
		"timestamp": {fmt.Sprint(utils.Now())},
		"version":   {IYUU_VERSION},
		"hash":      {string(hash)},
		"sha1":      {utils.Sha1(hash)},
	}
	resData := &IyuuApiHashResponse{}
	err := utils.PostUrlForJson(apiUrl, data, &resData, nil)
	log.Tracef("ApiInfoHash response err=%v", err)
	if err != nil {
		return nil, err
	}
	if resData.Ret != 200 {
		return nil, fmt.Errorf("iyuu api error: ret=%d, msg=%s", resData.Ret, resData.Msg)
	}

	result := make(map[string]([]IyuuTorrentInfoHash))
	for _, data := range resData.Data {
		result[data.Hash] = data.Torrent
	}
	return result, nil
}

func IyuuApiGetUser(token string) (data map[string](any), err error) {
	err = utils.FetchJson("https://api.iyuu.cn/index.php?s=App.Api.GetUser&sign="+token,
		&data, nil)
	return
}

func IyuuApiSites(token string) (sites []IyuuApiSite, err error) {
	resData := &IyuuApiSitesResponse{}
	err = utils.FetchJson("https://api.iyuu.cn/index.php?s=App.Api.Sites&version="+
		IYUU_VERSION+"&sign="+token,
		resData, nil)
	if err != nil {
		return nil, err
	}
	if resData.Ret != 200 {
		return nil, fmt.Errorf("iyuu api error: ret=%d, msg=%s", resData.Ret, resData.Msg)
	}
	sites = resData.Data.Sites
	return
}

func IyuuApiBind(token string, site string, uid int64, passkey string) (data map[string](any), err error) {
	apiUrl := "https://api.iyuu.cn/index.php?s=App.Api.Bind&token=" + token +
		"&site=" + site + "&id" + fmt.Sprint(uid) + "&passkey=" + utils.Sha1String(passkey)

	resData := &IyuuApiResponse{}
	err = utils.FetchJson(apiUrl, &resData, nil)
	if err != nil {
		return nil, err
	}
	if resData.Ret != 200 {
		return nil, fmt.Errorf("iyuu api error: ret=%d, msg=%s", resData.Ret, resData.Msg)
	}
	return resData.Data, nil
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

func GenerateIyuu2LocalSiteMap(iyuuSites []IyuuApiSite,
	localSites []config.SiteConfigStruct) map[int64]string {
	iyuu2LocalSiteMap := map[int64](string){} // iyuu sid => local site name
	for _, iyuuSite := range iyuuSites {
		localSite := utils.FindInSlice(localSites, func(site config.SiteConfigStruct) bool {
			if site.Disabled {
				return false
			}
			if site.Url != "" && site.Url == iyuuSite.GetUrl() {
				return true
			}
			name := iyuuSite.Site
			if IYUU_SITE_IDS[name] != "" {
				name = IYUU_SITE_IDS[name]
			}
			return name == site.GetName()
		})
		if localSite != nil {
			iyuu2LocalSiteMap[iyuuSite.Id] = localSite.GetName()
		}
	}
	return iyuu2LocalSiteMap
}
