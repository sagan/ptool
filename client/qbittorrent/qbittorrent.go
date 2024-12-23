package qbittorrent

// qb web API: https://github.com/qbittorrent/qBittorrent/wiki/WebUI-API-(qBittorrent-4.1)

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/cookiejar"
	"net/textproto"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"sort"
	"strings"

	log "github.com/sirupsen/logrus"

	"github.com/sagan/ptool/client"
	"github.com/sagan/ptool/config"
	"github.com/sagan/ptool/constants"
	"github.com/sagan/ptool/util"
)

type Client struct {
	Name                      string
	ClientConfig              *config.ClientConfigStruct
	Config                    *config.ConfigStruct
	HttpClient                *http.Client
	data                      *apiSyncMaindata
	preferences               *apiPreferences
	Logined                   bool
	datatime                  int64
	unfinishedSize            int64
	unfinishedDownloadingSize int64
	contentPathTorrents       map[string][]*apiTorrentInfo
}

func (qbclient *Client) GetTorrentsByContentPath(contentPath string) ([]*client.Torrent, error) {
	err := qbclient.sync()
	if err != nil {
		return nil, err
	}
	var torrents []*client.Torrent
	for _, t := range qbclient.contentPathTorrents[contentPath] {
		torrents = append(torrents, t.ToTorrent())
	}
	return torrents, nil
}

func (qbclient *Client) SetAllTorrentsShareLimits(ratioLimit float64, seedingTimeLimit int64) error {
	return qbclient.SetTorrentsShareLimits([]string{"all"}, ratioLimit, seedingTimeLimit)
}

func (qbclient *Client) SetTorrentsShareLimits(infoHashes []string, ratioLimit float64, seedingTimeLimit int64) error {
	if len(infoHashes) == 0 {
		return nil
	}
	data := url.Values{
		"hashes": {strings.Join(infoHashes, "|")},
	}
	if ratioLimit == 0 {
		ratioLimit = -2 // use global limit
	}
	data.Add("ratioLimit", fmt.Sprint(ratioLimit))
	seedingTimeLimitValue := seedingTimeLimit
	if seedingTimeLimitValue == 0 {
		seedingTimeLimitValue = -2 //use global limit
	} else if seedingTimeLimitValue > 0 {
		seedingTimeLimitValue = seedingTimeLimitValue/60 + 1
	}
	data.Add("seedingTimeLimit", fmt.Sprint(seedingTimeLimitValue))
	// The maximum amount of time (minutes) the torrent is allowed to seed while being inactive.
	// -2 means the global limit should be used, -1 means no limit.
	data.Add("inactiveSeedingTimeLimit", fmt.Sprint(-2))
	return qbclient.apiPost("api/v2/torrents/setShareLimits", data)
}

func (qbclient *Client) apiPost(apiUrl string, data url.Values) error {
	resp, err := qbclient.HttpClient.PostForm(qbclient.ClientConfig.Url+apiUrl, data)
	if err != nil {
		return err
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if string(body) == "Fails." {
		return fmt.Errorf("apiPost error: Fails")
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return fmt.Errorf("apiPost error: status=%d", resp.StatusCode)
	}
	return nil
}

func (qbclient *Client) apiRequest(apiPath string, v any) error {
	resp, err := qbclient.HttpClient.Get(qbclient.ClientConfig.Url + apiPath)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return fmt.Errorf("apiRequest %s response %d status", apiPath, resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if v != nil {
		return json.Unmarshal(body, &v)
	} else {
		return nil
	}
}

func (qbclient *Client) login() error {
	if qbclient.Logined || qbclient.ClientConfig.QbittorrentNoLogin {
		return nil
	}
	username := qbclient.ClientConfig.Username
	password := qbclient.ClientConfig.Password
	// use qb default
	if username == "" {
		username = "admin"
	}
	if username == "admin" && password == "" {
		password = "adminadmin"
	}
	data := url.Values{
		"username": {username},
		"password": {password},
	}
	err := qbclient.apiPost("api/v2/auth/login", data)
	if err == nil {
		qbclient.Logined = true
	}
	return err
}

func (qbclient *Client) GetName() string {
	return qbclient.Name
}

func (qbclient *Client) GetClientConfig() *config.ClientConfigStruct {
	return qbclient.ClientConfig
}

func (qbclient *Client) AddTorrent(torrentContent []byte, option *client.TorrentOption, meta map[string]int64) error {
	err := qbclient.login()
	if err != nil {
		return fmt.Errorf("login error: %w", err)
	}
	name := client.GenerateNameWithMeta(option.Name, meta)
	body := new(bytes.Buffer)
	mp := multipart.NewWriter(body)
	if util.IsTorrentUrl(string(torrentContent)) {
		mp.WriteField("urls", string(torrentContent))
	} else {
		// see https://stackoverflow.com/questions/21130566/how-to-set-content-type-for-a-form-filed-using-multipart-in-go
		// torrentPartWriter, _ := mp.CreateFormField("torrents")
		h := make(textproto.MIMEHeader)
		h.Set("Content-Disposition", `form-data; name="torrents"; filename="file.torrent"`)
		h.Set("Content-Type", "application/x-bittorrent")
		torrentPartWriter, err := mp.CreatePart(h)
		if err != nil {
			return err
		}
		torrentPartWriter.Write(torrentContent)
	}
	mp.WriteField("rename", name)
	mp.WriteField("root_folder", "true")
	if option != nil {
		if option.Category != constants.NONE {
			mp.WriteField("category", option.Category)
		}
		mp.WriteField("tags", strings.Join(option.Tags, ",")) // qb 4.3.2+ new
		mp.WriteField("paused", fmt.Sprint(option.Pause))
		mp.WriteField("upLimit", fmt.Sprint(option.UploadSpeedLimit))
		mp.WriteField("dlLimit", fmt.Sprint(option.DownloadSpeedLimit))
		if option.SavePath != "" {
			mp.WriteField("savepath", option.SavePath)
			mp.WriteField("autoTMM", "false")
		}
		if option.SkipChecking {
			mp.WriteField("skip_checking", "true")
		}
		if option.SequentialDownload {
			mp.WriteField("sequentialDownload", "true")
		}
		if option.RatioLimit != 0 {
			mp.WriteField("ratioLimit", fmt.Sprint(option.RatioLimit))
		}
		if option.SeedingTimeLimit != 0 {
			value := option.SeedingTimeLimit
			if value > 0 {
				value = value/60 + 1
			}
			mp.WriteField("seedingTimeLimit", fmt.Sprint(value))
		}
	}
	mp.Close()
	resp, err := qbclient.HttpClient.Post(qbclient.ClientConfig.Url+"api/v2/torrents/add",
		mp.FormDataContentType(), body)
	if err != nil {
		return fmt.Errorf("add torrent error: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return fmt.Errorf("add torrent error: status=%d", resp.StatusCode)
	}
	return err
}

func (qbclient *Client) PauseTorrents(infoHashes []string) error {
	if len(infoHashes) == 0 {
		return nil
	}
	err := qbclient.login()
	if err != nil {
		return fmt.Errorf("login error: %w", err)
	}
	data := url.Values{
		"hashes": {strings.Join(infoHashes, "|")},
	}
	return qbclient.apiPost("api/v2/torrents/pause", data)
}

func (qbclient *Client) ResumeTorrents(infoHashes []string) error {
	if len(infoHashes) == 0 {
		return nil
	}
	err := qbclient.login()
	if err != nil {
		return fmt.Errorf("login error: %w", err)
	}
	data := url.Values{
		"hashes": {strings.Join(infoHashes, "|")},
	}
	return qbclient.apiPost("api/v2/torrents/resume", data)
}

func (qbclient *Client) RecheckTorrents(infoHashes []string) error {
	if len(infoHashes) == 0 {
		return nil
	}
	err := qbclient.login()
	if err != nil {
		return fmt.Errorf("login error: %w", err)
	}
	data := url.Values{
		"hashes": {strings.Join(infoHashes, "|")},
	}
	return qbclient.apiPost("api/v2/torrents/recheck", data)
}

func (qbclient *Client) ReannounceTorrents(infoHashes []string) error {
	if len(infoHashes) == 0 {
		return nil
	}
	err := qbclient.login()
	if err != nil {
		return fmt.Errorf("login error: %w", err)
	}
	data := url.Values{
		"hashes": {strings.Join(infoHashes, "|")},
	}
	return qbclient.apiPost("api/v2/torrents/reannounce", data)
}

func (qbclient *Client) AddTagsToTorrents(infoHashes []string, tags []string) error {
	if len(infoHashes) == 0 || len(tags) == 0 {
		return nil
	}
	err := qbclient.login()
	if err != nil {
		return fmt.Errorf("login error: %w", err)
	}
	data := url.Values{
		"hashes": {strings.Join(infoHashes, "|")},
		"tags":   {strings.Join(tags, ",")},
	}
	return qbclient.apiPost("api/v2/torrents/addTags", data)
}

func (qbclient *Client) RemoveTagsFromTorrents(infoHashes []string, tags []string) error {
	if len(infoHashes) == 0 || len(tags) == 0 {
		return nil
	}
	err := qbclient.login()
	if err != nil {
		return fmt.Errorf("login error: %w", err)
	}
	data := url.Values{
		"hashes": {strings.Join(infoHashes, "|")},
		"tags":   {strings.Join(tags, ",")},
	}
	return qbclient.apiPost("api/v2/torrents/removeTags", data)
}

func (qbclient *Client) SetTorrentsSavePath(infoHashes []string, savePath string) error {
	if len(infoHashes) == 0 {
		return nil
	}
	savePath = strings.TrimSpace(savePath)
	if savePath == "" {
		return fmt.Errorf("savePath is empty")
	}
	err := qbclient.login()
	if err != nil {
		return fmt.Errorf("login error: %w", err)
	}
	data := url.Values{
		"hashes":   {strings.Join(infoHashes, "|")},
		"location": {savePath},
	}
	return qbclient.apiPost("api/v2/torrents/setLocation", data)
}

func (qbclient *Client) PauseAllTorrents() error {
	return qbclient.PauseTorrents([]string{"all"})
}

func (qbclient *Client) ResumeAllTorrents() error {
	return qbclient.ResumeTorrents([]string{"all"})
}

func (qbclient *Client) RecheckAllTorrents() error {
	return qbclient.RecheckTorrents([]string{"all"})
}

func (qbclient *Client) ReannounceAllTorrents() error {
	return qbclient.ReannounceTorrents([]string{"all"})
}

func (qbclient *Client) AddTagsToAllTorrents(tags []string) error {
	return qbclient.AddTagsToTorrents([]string{"all"}, tags)
}

func (qbclient *Client) RemoveTagsFromAllTorrents(tags []string) error {
	return qbclient.RemoveTagsFromTorrents([]string{"all"}, tags)
}

func (qbclient *Client) SetAllTorrentsSavePath(savePath string) error {
	return qbclient.SetTorrentsSavePath([]string{"all"}, savePath)
}

func (qbclient *Client) GetTags() ([]string, error) {
	err := qbclient.login()
	if err != nil {
		return nil, fmt.Errorf("login error: %w", err)
	}
	var tags []string
	err = qbclient.apiRequest("api/v2/torrents/tags", &tags)
	return tags, err
}

func (qbclient *Client) CreateTags(tags ...string) error {
	err := qbclient.login()
	if err != nil {
		return fmt.Errorf("login error: %w", err)
	}
	data := url.Values{
		"tags": {strings.Join(tags, ",")},
	}
	return qbclient.apiPost("api/v2/torrents/createTags", data)
}

func (qbclient *Client) DeleteTags(tags ...string) error {
	err := qbclient.login()
	if err != nil {
		return fmt.Errorf("login error: %w", err)
	}
	data := url.Values{
		"tags": {strings.Join(tags, ",")},
	}
	return qbclient.apiPost("api/v2/torrents/deleteTags", data)
}

func (qbclient *Client) MakeCategory(category string, savePath string) error {
	err := qbclient.login()
	if err != nil {
		return fmt.Errorf("login error: %w", err)
	}
	data := url.Values{
		"category": {category},
	}
	if savePath != constants.NONE {
		data.Add("savePath", savePath)
	}
	err = qbclient.apiPost("api/v2/torrents/createCategory", data)
	// 简单粗暴
	if err != nil && strings.Contains(err.Error(), "status=409") {
		if data.Has("savePath") {
			return qbclient.apiPost("api/v2/torrents/editCategory", data)
		}
		return nil
	}
	return err
}

func (qbclient *Client) DeleteCategories(categories []string) error {
	err := qbclient.login()
	if err != nil {
		return fmt.Errorf("login error: %w", err)
	}
	data := url.Values{
		"categories": {strings.Join(categories, "\n")},
	}
	return qbclient.apiPost("api/v2/torrents/removeCategories", data)
}

func (qbclient *Client) GetCategories() ([]*client.TorrentCategory, error) {
	err := qbclient.login()
	if err != nil {
		return nil, fmt.Errorf("login error: %w", err)
	}
	var categories map[string]*client.TorrentCategory
	err = qbclient.apiRequest("api/v2/torrents/categories", &categories)
	if err != nil {
		return nil, err
	}
	cats := []*client.TorrentCategory{}
	for _, category := range categories {
		cats = append(cats, category)
	}
	return cats, nil
}

func (qbclient *Client) SetTorrentsCatetory(infoHashes []string, category string) error {
	if len(infoHashes) == 0 {
		return nil
	}
	err := qbclient.login()
	if err != nil {
		return fmt.Errorf("login error: %w", err)
	}
	if category == constants.NONE {
		category = ""
	}
	data := url.Values{
		"hashes":   {strings.Join(infoHashes, "|")},
		"category": {category},
	}
	return qbclient.apiPost("api/v2/torrents/setCategory", data)
}

func (qbclient *Client) SetAllTorrentsCatetory(category string) error {
	return qbclient.SetTorrentsCatetory([]string{"all"}, category)
}

func (qbclient *Client) DeleteTorrents(infoHashes []string, deleteFiles bool) (err error) {
	if len(infoHashes) == 0 {
		return nil
	}
	err = qbclient.login()
	if err != nil {
		return fmt.Errorf("login error: %w", err)
	}
	data := url.Values{
		"hashes":      {strings.Join(infoHashes, "|")},
		"deleteFiles": {fmt.Sprint(deleteFiles)},
	}
	err = qbclient.apiPost("api/v2/torrents/delete", data)
	if err == nil && qbclient.Cached() {
		for _, infoHash := range infoHashes {
			delete(qbclient.data.Torrents, infoHash)
		}
		qbclient.buildDerivative()
	}
	return
}

func (qbclient *Client) ModifyTorrent(infoHash string,
	option *client.TorrentOption, meta map[string]int64) error {
	if option == nil {
		option = &client.TorrentOption{}
	}
	err := qbclient.sync()
	if err != nil {
		return err
	}

	// qbtorrent := &apiTorrentProperties{}
	// err = qbclient.apiRequest("api/v2/torrents/properties?hash="+torrent.InfoHash, qbtorrent)
	qbtorrent, ok := qbclient.data.Torrents[infoHash]
	if !ok {
		return fmt.Errorf("torrent not exists")
	}

	if option.Name != "" || len(meta) > 0 {
		name := option.Name
		if name == "" {
			name, _ = client.ParseMetaFromName(qbtorrent.Name)
		}
		name = client.GenerateNameWithMeta(name, meta)
		if name != qbtorrent.Name {
			data := url.Values{
				"hash": {infoHash},
				"name": {name},
			}
			err := qbclient.apiPost("api/v2/torrents/rename", data)
			if err != nil {
				return err
			}
		}
	}

	// @todo: apply option.SequentialDownload using qb toggleSequentialDownload.
	// However, we must know current sequentialDownload status in ahead.

	if option.Category != "" {
		category := option.Category
		if category == constants.NONE {
			category = ""
		}
		if category != qbtorrent.Category {
			data := url.Values{
				"hashes":   {infoHash},
				"category": {category},
			}
			err := qbclient.apiPost("api/v2/torrents/setCategory", data)
			if err != nil {
				return err
			}
		}
	}

	if len(option.Tags) > 0 || len(option.RemoveTags) > 0 {
		qbTags := util.SplitCsv(qbtorrent.Tags)
		addTags := []string{}
		removeTags := []string{}
		for _, addTag := range option.Tags {
			if !slices.Contains(qbTags, addTag) {
				addTags = append(addTags, addTag)
			}
		}
		for _, removeTag := range option.RemoveTags {
			if slices.Contains(qbTags, removeTag) {
				removeTags = append(removeTags, removeTag)
			}
		}
		if len(removeTags) > 0 {
			data := url.Values{
				"hashes": {infoHash},
				"tags":   {strings.Join(addTags, ",")},
			}
			err := qbclient.apiPost("api/v2/torrents/removeTags", data)
			if err != nil {
				return err
			}
		}
		if len(addTags) > 0 {
			data := url.Values{
				"hashes": {infoHash},
				"tags":   {strings.Join(addTags, ",")},
			}
			err := qbclient.apiPost("api/v2/torrents/addTags", data)
			if err != nil {
				return err
			}
		}
	}

	if option.DownloadSpeedLimit != 0 && option.DownloadSpeedLimit != qbtorrent.Dl_limit {
		data := url.Values{
			"hashes": {infoHash},
			"limit":  {fmt.Sprint(option.DownloadSpeedLimit)},
		}
		err := qbclient.apiPost("api/v2/torrents/setDownloadLimit", data)
		if err != nil {
			return err
		}
	}

	if option.UploadSpeedLimit != 0 && option.UploadSpeedLimit != qbtorrent.Up_limit {
		data := url.Values{
			"hashes": {infoHash},
			"limit":  {fmt.Sprint(option.UploadSpeedLimit)},
		}
		err := qbclient.apiPost("api/v2/torrents/setUploadLimit", data)
		if err != nil {
			return err
		}
	}

	if option.RatioLimit != 0 || option.SeedingTimeLimit != 0 {
		err := qbclient.SetTorrentsShareLimits([]string{infoHash}, option.RatioLimit, option.SeedingTimeLimit)
		if err != nil {
			return err
		}
	}

	if option.Pause {
		if qbtorrent.CanPause() {
			qbclient.PauseTorrents([]string{qbtorrent.Hash})
		}
	} else if option.Resume {
		if qbtorrent.CanResume() {
			qbclient.ResumeTorrents([]string{qbtorrent.Hash})
		}
	}

	return nil
}

func (qbclient *Client) PurgeCache() {
	qbclient.data = nil
	qbclient.preferences = nil
	qbclient.unfinishedSize = 0
	qbclient.unfinishedDownloadingSize = 0
	qbclient.datatime = 0
	qbclient.contentPathTorrents = nil
}

func (qbclient *Client) Cached() bool {
	return qbclient.datatime > 0
}

func (qbclient *Client) sync() error {
	if qbclient.datatime > 0 {
		return nil
	}
	err := qbclient.login()
	if err != nil {
		return fmt.Errorf("login error: %w", err)
	}
	err = qbclient.apiRequest("api/v2/sync/maindata", &qbclient.data)
	if err != nil {
		return err
	}
	qbclient.datatime = util.Now()
	qbclient.buildDerivative()
	return nil
}

func (qbclient *Client) buildDerivative() {
	unfinishedSize := int64(0)
	unfinishedDownloadingSize := int64(0)
	contentPathTorrents := map[string][]*apiTorrentInfo{}
	// make hash available in torrent itself as well as map key
	for hash, torrent := range qbclient.data.Torrents {
		torrent.Hash = hash
		qbclient.data.Torrents[hash] = torrent
		usize := torrent.Size - torrent.Completed
		unfinishedSize += usize
		if torrent.State != "pausedDL" {
			unfinishedDownloadingSize += usize
		}
		contentPathTorrents[torrent.ContentPath()] = append(contentPathTorrents[torrent.ContentPath()], torrent)
	}
	qbclient.unfinishedSize = unfinishedSize
	qbclient.unfinishedDownloadingSize = unfinishedDownloadingSize
	qbclient.contentPathTorrents = contentPathTorrents
}

func (qbclient *Client) TorrentRootPathExists(rootFolder string) bool {
	if rootFolder == "" {
		return false
	}
	err := qbclient.sync()
	if err != nil {
		return false
	}
	for _, torrent := range qbclient.data.Torrents {
		if strings.HasSuffix(torrent.ContentPath(), torrent.Sep()+rootFolder) {
			return true
		}
	}
	return false
}

func (qbclient *Client) GetStatus() (*client.Status, error) {
	err := qbclient.sync()
	if err != nil {
		return nil, err
	}
	var status client.Status
	status.DownloadSpeed = qbclient.data.Server_state.Dl_info_speed
	status.UploadSpeed = qbclient.data.Server_state.Up_info_speed
	status.DownloadSpeedLimit = qbclient.data.Server_state.Dl_rate_limit
	status.UploadSpeedLimit = qbclient.data.Server_state.Up_rate_limit
	status.FreeSpaceOnDisk = qbclient.data.Server_state.Free_space_on_disk
	status.UnfinishedSize = qbclient.unfinishedSize
	status.UnfinishedDownloadingSize = qbclient.unfinishedDownloadingSize
	// @workaround
	// qb 的 Web API 有 bug，有时 FreeSpaceOnDisk 返回 0，但实际硬盘剩余空间充足，原因尚不明确。
	// 目前在 Windows QB 4.5.2 上发现此现象。
	if status.FreeSpaceOnDisk == 0 {
		hasDownloadingTorrent := false
		hasErrorTorrent := false
		for _, qbtorrent := range qbclient.data.Torrents {
			if qbtorrent.State == "downloading" {
				hasDownloadingTorrent = true
				break
			}
			if qbtorrent.State == "error" && qbtorrent.Completed < qbtorrent.Size {
				hasErrorTorrent = true
			}
		}
		if hasDownloadingTorrent || !hasErrorTorrent {
			status.FreeSpaceOnDisk = -1
		}
	}
	if slices.Contains(qbclient.data.Tags, config.NOADD_TAG) {
		status.NoAdd = true
	}
	if slices.Contains(qbclient.data.Tags, config.NODEL_TAG) {
		status.NoDel = true
	}
	return &status, nil
}

func (qbclient *Client) setPreferences(preferences map[string]any) error {
	err := qbclient.login()
	if err != nil {
		return fmt.Errorf("login error: %w", err)
	}
	data, err := json.Marshal(preferences)
	if err != nil {
		return err
	}
	// setPreferences qb API expects a "raw" form data (without %XX escapes)
	dataStr := "json=" + string(data)
	_, err = qbclient.HttpClient.Post(qbclient.ClientConfig.Url+"api/v2/app/setPreferences",
		"application/x-www-form-urlencoded", strings.NewReader(dataStr))
	return err
}

func (qbclient *Client) getPreferences() (*apiPreferences, error) {
	err := qbclient.login()
	if err != nil {
		return nil, fmt.Errorf("login error: %w", err)
	}
	if qbclient.preferences != nil {
		return qbclient.preferences, nil
	}
	err = qbclient.apiRequest("api/v2/app/preferences", &qbclient.preferences)
	return qbclient.preferences, err
}

func (qbclient *Client) GetConfig(variable string) (string, error) {
	err := qbclient.login()
	if err != nil {
		return "", fmt.Errorf("login error: %w", err)
	}
	if strings.HasPrefix(variable, "qb_") && len(variable) > 3 {
		preferences, err := qbclient.getPreferences()
		if err != nil {
			return "", err
		}
		value := reflect.Indirect(reflect.ValueOf(preferences)).FieldByName(util.Capitalize(variable[3:])).Interface()
		return fmt.Sprint(value), nil
	}

	switch variable {
	case "global_download_speed_limit":
		v := 0
		err = qbclient.apiRequest("api/v2/transfer/downloadLimit", &v)
		return fmt.Sprint(v), err
	case "global_upload_speed_limit":
		v := 0
		err = qbclient.apiRequest("api/v2/transfer/uploadLimit", &v)
		return fmt.Sprint(v), err
	case "free_disk_space":
		status, err := qbclient.GetStatus()
		if err != nil {
			return "", err
		}
		return fmt.Sprint(status.FreeSpaceOnDisk), nil
	case "global_download_speed":
		status, err := qbclient.GetStatus()
		if err != nil {
			return "", err
		}
		return fmt.Sprint(status.DownloadSpeed), nil
	case "global_upload_speed":
		status, err := qbclient.GetStatus()
		if err != nil {
			return "", err
		}
		return fmt.Sprint(status.UploadSpeed), nil
	case "save_path":
		preferences, err := qbclient.getPreferences()
		if err != nil {
			return "", err
		}
		return preferences.Save_path, nil
	default:
		return "", nil
	}
}

func (qbclient *Client) SetConfig(variable string, value string) error {
	err := qbclient.login()
	if err != nil {
		return fmt.Errorf("login error: %w", err)
	}
	if strings.HasPrefix(variable, "qb_") && len(variable) > 3 {
		data := map[string]any{}
		data[variable[3:]], _ = util.String2Any(value)
		return qbclient.setPreferences(data)
	}
	switch variable {
	case "global_download_speed_limit":
		{
			data := url.Values{
				"limit": {value},
			}
			err = qbclient.apiPost("api/v2/transfer/setDownloadLimit", data)
			return err
		}
	case "global_upload_speed_limit":
		{
			data := url.Values{
				"limit": {value},
			}
			err = qbclient.apiPost("api/v2/transfer/setUploadLimit", data)
			return err
		}
	case "free_disk_space", "global_download_speed", "global_upload_speed":
		return fmt.Errorf("%s is read-only", variable)
	case "save_path":
		return qbclient.setPreferences(map[string]any{"save_path": value})
	default:
		return nil
	}
}

// The export API of qb exists but currently is not documented in
// https://github.com/qbittorrent/qBittorrent/wiki/WebUI-API-(qBittorrent-4.1) .
// See https://github.com/qbittorrent/qBittorrent/issues/18746 for more info.
func (qbclient *Client) ExportTorrentFile(infoHash string) ([]byte, error) {
	if qbclient.ClientConfig.LocalTorrentsPath != "" {
		return os.ReadFile(filepath.Join(qbclient.ClientConfig.LocalTorrentsPath, infoHash+".torrent"))
	}
	apiUrl := qbclient.ClientConfig.Url + "api/v2/torrents/export?hash=" + infoHash
	res, _, err := util.FetchUrl(apiUrl, qbclient.HttpClient, nil)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	return io.ReadAll(res.Body)
}

func (qbclient *Client) GetTorrent(infoHash string) (*client.Torrent, error) {
	err := qbclient.sync()
	if err != nil {
		return nil, err
	}
	qbtorrent := qbclient.data.Torrents[infoHash]
	if qbtorrent == nil {
		return nil, nil
	}
	return qbtorrent.ToTorrent(), nil
}

func (qbclient *Client) GetTorrents(stateFilter string, category string, showAll bool) ([]*client.Torrent, error) {
	err := qbclient.sync()
	if err != nil {
		return nil, err
	}
	torrents := []*client.Torrent{}

	for _, qbtorrent := range qbclient.data.Torrents {
		if category != "" {
			if category == constants.NONE {
				if qbtorrent.Category != "" {
					continue
				}
			} else if category != qbtorrent.Category {
				continue
			}
		}
		if !showAll && qbtorrent.Dlspeed < 1024 && qbtorrent.Upspeed < 1024 {
			continue
		}
		torrent := qbtorrent.ToTorrent()
		if !torrent.MatchStateFilter(stateFilter) {
			continue
		}
		torrents = append(torrents, torrent)
	}
	return torrents, nil
}

func (qbclient *Client) GetTorrentContents(infoHash string) ([]*client.TorrentContentFile, error) {
	err := qbclient.login()
	if err != nil {
		return nil, fmt.Errorf("login error: %w", err)
	}
	apiUrl := qbclient.ClientConfig.Url + "api/v2/torrents/files?hash=" + infoHash
	var qbTorrentContents []apiTorrentContent
	err = util.FetchJson(apiUrl, &qbTorrentContents, qbclient.HttpClient, nil)
	if err != nil {
		return nil, err
	}
	torrentContents := []*client.TorrentContentFile{}
	for _, qbTorrentContent := range qbTorrentContents {
		torrentContents = append(torrentContents, &client.TorrentContentFile{
			Index:    qbTorrentContent.Index,
			Path:     strings.ReplaceAll(qbTorrentContent.Name, `\`, "/"),
			Size:     qbTorrentContent.Size,
			Ignored:  qbTorrentContent.Priority == 0,
			Complete: qbTorrentContent.Is_seed,
			Progress: qbTorrentContent.Progress,
		})
	}
	sort.Slice(torrentContents, func(i, j int) bool {
		return torrentContents[i].Index < torrentContents[j].Index
	})
	return torrentContents, nil
}

func (qbclient *Client) GetTorrentTrackers(infoHash string) (client.TorrentTrackers, error) {
	err := qbclient.login()
	if err != nil {
		return nil, fmt.Errorf("login error: %w", err)
	}
	apiUrl := qbclient.ClientConfig.Url + "api/v2/torrents/trackers?hash=" + infoHash
	var qbTorrentTrackers []apiTorrentTracker
	err = util.FetchJson(apiUrl, &qbTorrentTrackers, qbclient.HttpClient, nil)
	if err != nil {
		return nil, err
	}
	qbTorrentTrackers = util.Filter(qbTorrentTrackers, func(tracker apiTorrentTracker) bool {
		// exclude qb  "** [DHT] **", "** [PeX] **", "** [LSD] **" trackers
		return !strings.HasPrefix(tracker.Url, "**")
	})
	trackers := util.Map(qbTorrentTrackers, func(qbtracker apiTorrentTracker) client.TorrentTracker {
		status := ""
		switch qbtracker.Status {
		case 0:
			status = "disabled"
		case 1:
			status = "notcontacted"
		case 2:
			status = "working"
		case 3:
			status = "updating"
		case 4:
			status = "error"
		default:
			status = "unknown"
		}
		return client.TorrentTracker{
			Url:    qbtracker.Url,
			Msg:    qbtracker.Msg,
			Status: status,
		}
	})
	return trackers, nil
}

func (qbclient *Client) EditTorrentTracker(infoHash string, oldTracker string,
	newTracker string, replaceHost bool) error {
	if replaceHost {
		torrent, err := qbclient.GetTorrent(infoHash)
		if err != nil {
			return err
		}
		if torrent == nil {
			return fmt.Errorf("torrent %s not found", infoHash)
		}
		trackers, err := qbclient.GetTorrentTrackers(torrent.InfoHash)
		if err != nil {
			return fmt.Errorf("failed to get torrent %s trackers: %w", torrent.InfoHash, err)
		}
		oldTrackerUrl := ""
		newTrackerUrl := ""
		directNewUrlMode := util.IsUrl(newTracker)
		index := trackers.FindIndex(oldTracker)
		if index != -1 {
			oldTrackerUrl = trackers[index].Url
			if directNewUrlMode {
				newTrackerUrl = newTracker
			} else {
				oldTrackerUrlObj, err := url.Parse(oldTrackerUrl)
				if err == nil {
					oldTrackerUrlObj.Host = newTracker
					newTrackerUrl = oldTrackerUrlObj.String()
				}
			}
		}
		if oldTrackerUrl != "" && newTrackerUrl != "" {
			if oldTrackerUrl == newTrackerUrl {
				return nil
			}
			err := qbclient.EditTorrentTracker(torrent.InfoHash, oldTrackerUrl, newTrackerUrl, false)
			if err != nil {
				log.Errorf("Failed to replace torrent %s tracker domain: %v", torrent.InfoHash, err)
			} else {
				log.Debugf("Replaced torrent %s tracker %s => %s", torrent.InfoHash, oldTrackerUrl, newTrackerUrl)
			}
			return err
		}
		return fmt.Errorf("torrent %s old tracker does NOT exist", torrent.InfoHash)
	}
	data := url.Values{
		"hash":    {infoHash},
		"origUrl": {oldTracker},
		"newUrl":  {newTracker},
	}
	return qbclient.apiPost("api/v2/torrents/editTracker", data)
}

// trackers - new trackers full URLs; oldTracker - existing tracker host or URL
func (qbclient *Client) AddTorrentTrackers(infoHash string, trackers []string,
	oldTracker string, removeExisting bool) error {
	var existingTrackers []string
	if oldTracker != "" {
		torrentTrackers, err := qbclient.GetTorrentTrackers(infoHash)
		if err != nil {
			return err
		}
		index := torrentTrackers.FindIndex(oldTracker)
		if index == -1 {
			return nil
		}
		if removeExisting {
			existingTrackers = util.Map(torrentTrackers, func(t client.TorrentTracker) string { return t.Url })
		} else {
			trackers = util.Filter(trackers, func(tracker string) bool {
				return torrentTrackers.FindIndex(tracker) == -1
			})
		}
	}
	if len(trackers) == 0 {
		return nil
	}
	data := url.Values{
		"hash": {infoHash},
		"urls": {strings.Join(trackers, "\n")},
	}
	if err := qbclient.apiPost("api/v2/torrents/addTrackers", data); err != nil {
		return err
	}
	if removeExisting && len(existingTrackers) > 0 {
		return qbclient.RemoveTorrentTrackers(infoHash, existingTrackers)
	}
	return nil
}

func (qbclient *Client) RemoveTorrentTrackers(infoHash string, trackers []string) error {
	data := url.Values{
		"hash": {infoHash},
		"urls": {strings.Join(trackers, "|")},
	}
	return qbclient.apiPost("api/v2/torrents/removeTrackers", data)
}

func (qbclient *Client) SetFilePriority(infoHash string, fileIndexes []int64, priority int64) error {
	if len(fileIndexes) == 0 {
		return fmt.Errorf("must provide at least fileIndex")
	}
	id := ""
	for i, index := range fileIndexes {
		if i > 0 {
			id += "|"
		}
		id += fmt.Sprint(index)
	}
	data := url.Values{
		"hash":     {infoHash},
		"id":       {id},
		"priority": {fmt.Sprint(priority)},
	}
	return qbclient.apiPost("api/v2/torrents/filePrio", data)
}

func (qbclient *Client) Close() {
	qbclient.PurgeCache()
	if qbclient.Logined && !qbclient.ClientConfig.QbittorrentNoLogout {
		qbclient.Logined = false
		qbclient.apiPost("api/v2/auth/logout", nil)
	}
}

func NewClient(name string, clientConfig *config.ClientConfigStruct, config *config.ConfigStruct) (
	client.Client, error) {
	jar, err := cookiejar.New(nil)
	if err != nil {
		return nil, err
	}
	client := &Client{
		Name:         name,
		ClientConfig: clientConfig,
		Config:       config,
		HttpClient: &http.Client{
			Jar: jar,
		},
	}
	return client, nil
}

func init() {
	client.Register(&client.RegInfo{
		Name:    "qbittorrent",
		Creator: NewClient,
	})
}

var (
	_ client.Client = (*Client)(nil)
)
