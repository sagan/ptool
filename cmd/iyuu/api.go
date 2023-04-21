package iyuu

import (
	"encoding/json"
	"fmt"
	"net/url"
	"sort"
	"strings"

	"github.com/sagan/ptool/utils"

	log "github.com/sirupsen/logrus"
)

type IyuuSite struct {
	id      int64
	name    string
	canBind bool
}

type IyuuApiGetUserResponse struct {
	Ret  int64  `json:"ret"`
	Msg  string `json:"msg"`
	Data struct {
		User map[string](any) `json:"user"`
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
