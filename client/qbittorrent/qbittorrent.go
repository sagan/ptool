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
	"strings"

	"github.com/sagan/ptool/client"
	"github.com/sagan/ptool/config"
	"github.com/sagan/ptool/utils"
)

type Client struct {
	Name         string
	ClientConfig *config.ClientConfigStruct
	Config       *config.ConfigStruct
	HttpClient   *http.Client
	data         *apiSyncMaindata
	Logined      bool
	datatime     int64
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
	if qbclient.Logined {
		return nil
	}
	data := url.Values{
		"username": {qbclient.ClientConfig.Username},
		"password": {qbclient.ClientConfig.Password},
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

func (qbclient *Client) AddTorrent(torrentContent []byte, option *client.TorrentOption, meta map[string](int64)) error {
	err := qbclient.login()
	if err != nil {
		return fmt.Errorf("login error: %v", err)
	}

	name := client.GenerateNameWithMeta(option.Name, meta)
	body := new(bytes.Buffer)
	mp := multipart.NewWriter(body)
	defer mp.Close()
	// see https://stackoverflow.com/questions/21130566/how-to-set-content-type-for-a-form-filed-using-multipart-in-go
	// torrentPartWriter, _ := mp.CreateFormField("torrents")
	h := make(textproto.MIMEHeader)
	h.Set("Content-Disposition", fmt.Sprintf(`form-data; name="%s"; filename="%s"`, "torrents", "file.torrent"))
	h.Set("Content-Type", "application/x-bittorrent")
	torrentPartWriter, err := mp.CreatePart(h)
	if err != nil {
		return err
	}
	torrentPartWriter.Write(torrentContent)
	mp.WriteField("rename", name)
	mp.WriteField("root_folder", "true")
	if option != nil {
		mp.WriteField("category", option.Category)
		mp.WriteField("tags", strings.Join(option.Tags, ","))
		mp.WriteField("paused", fmt.Sprint(option.Pause))
		mp.WriteField("upLimit", fmt.Sprint(option.UploadSpeedLimit))
		mp.WriteField("dlLimit", fmt.Sprint(option.DownloadSpeedLimit))
	}
	resp, err := qbclient.HttpClient.Post(qbclient.ClientConfig.Url+"api/v2/torrents/add",
		mp.FormDataContentType(), body)
	if err != nil {
		return fmt.Errorf("add torrent error: %v", err)
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
		return fmt.Errorf("login error: %v", err)
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
		return fmt.Errorf("login error: %v", err)
	}
	data := url.Values{
		"hashes": {strings.Join(infoHashes, "|")},
	}
	return qbclient.apiPost("api/v2/torrents/resume", data)
}

func (qbclient *Client) DeleteTorrents(infoHashes []string, deleteFiles bool) error {
	if len(infoHashes) == 0 {
		return nil
	}
	err := qbclient.login()
	if err != nil {
		return fmt.Errorf("login error: %v", err)
	}
	deleteFilesStr := "false"
	if deleteFiles {
		deleteFilesStr = "true"
	}
	data := url.Values{
		"hashes":      {strings.Join(infoHashes, "|")},
		"deleteFiles": {deleteFilesStr},
	}
	return qbclient.apiPost("api/v2/torrents/delete", data)
}

func (qbclient *Client) ModifyTorrent(infoHash string,
	option *client.TorrentOption, meta map[string](int64)) error {
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

	if option.Category != "" && option.Category != qbtorrent.Category {
		data := url.Values{
			"hashes":   {infoHash},
			"category": {option.Category},
		}
		err := qbclient.apiPost("api/v2/torrents/setCategory", data)
		if err != nil {
			return err
		}
	}

	if len(option.Tags) > 0 || len(option.RemoveTags) > 0 {
		qbTags := strings.Split(qbtorrent.Tags, ",")
		addTags := []string{}
		removeTags := []string{}
		for _, addTag := range option.Tags {
			if utils.FindInSlice(qbTags, func(tag string) bool {
				return tag == addTag
			}) == nil {
				addTags = append(addTags, addTag)
			}
		}
		for _, removeTag := range option.RemoveTags {
			if utils.FindInSlice(qbTags, func(tag string) bool {
				return tag == removeTag
			}) != nil {
				removeTags = append(removeTags, removeTag)
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
	qbclient.datatime = 0
}

func (qbclient *Client) sync() error {
	if utils.Now()-qbclient.datatime <= 15 {
		return nil
	}
	err := qbclient.login()
	if err != nil {
		return fmt.Errorf("login error: %v", err)
	}
	err = qbclient.apiRequest("api/v2/sync/maindata", &qbclient.data)
	if err != nil {
		return err
	}
	qbclient.datatime = utils.Now()
	// make hash available in torrent itself as well as map key
	for hash, torrent := range qbclient.data.Torrents {
		torrent.Hash = hash
		qbclient.data.Torrents[hash] = torrent
	}
	return nil
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
		if strings.HasSuffix(torrent.Content_path, "/"+rootFolder) {
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
	if utils.FindInSlice(qbclient.data.Tags, func(tag string) bool {
		return strings.ToLower(tag) == "_noadd"
	}) != nil {
		status.NoAdd = true
	}
	return &status, nil
}

func (qbclient *Client) GetConfig(variable string) (string, error) {
	err := qbclient.login()
	if err != nil {
		return "", fmt.Errorf("login error: %v", err)
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
	default:
		return "", nil
	}
}

func (qbclient *Client) SetConfig(variable string, value string) error {
	err := qbclient.login()
	if err != nil {
		return fmt.Errorf("login error: %v", err)
	}
	switch variable {
	case "global_download_speed_limit":
		err = qbclient.apiRequest("api/v2/transfer/setDownloadLimit?limit="+value, nil)
		return err
	case "global_upload_speed_limit":
		err = qbclient.apiRequest("api/v2/transfer/setUploadLimit?limit="+value, nil)
		return err
	default:
		return nil
	}
}

func (qbclient *Client) GetTorrents(stateFilter string, category string, showAll bool) ([]client.Torrent, error) {
	torrents := make([]client.Torrent, 0)
	err := qbclient.sync()
	if err != nil {
		return nil, err
	}

	for _, qbtorrent := range qbclient.data.Torrents {
		if category != "" && category != qbtorrent.Category {
			continue
		}
		if !showAll && qbtorrent.Dlspeed <= 1024 && qbtorrent.Upspeed <= 1024 {
			continue
		}
		state := ""
		switch qbtorrent.State {
		case "forcedUP", "stalledUP", "queuedUP", "uploading":
			state = "seeding"
		case "metaDL", "stalledDL", "checkingDL", "forcedDL", "downloading":
			state = "downloading"
		case "pausedUP":
			state = "completed"
		case "pausedDL":
			state = "paused"
		default:
			state = qbtorrent.State
		}
		if stateFilter != "" && stateFilter != state {
			continue
		}
		torrent := client.Torrent{
			InfoHash:           qbtorrent.Hash,
			Name:               qbtorrent.Name,
			TrackerDomain:      utils.ParseUrlHostname(qbtorrent.Tracker),
			State:              state,
			Atime:              qbtorrent.Added_on,
			Ctime:              qbtorrent.Completion_on,
			Downloaded:         qbtorrent.Downloaded,
			DownloadSpeed:      qbtorrent.Dlspeed,
			DownloadSpeedLimit: qbtorrent.Dl_limit,
			Uploaded:           qbtorrent.Uploaded,
			UploadSpeed:        qbtorrent.Upspeed,
			UploadedSpeedLimit: qbtorrent.Up_limit,
			Category:           qbtorrent.Category,
			Tags:               strings.Split(qbtorrent.Tags, ","),
			Seeders:            qbtorrent.Num_complete,
			Size:               qbtorrent.Size,
			SizeCompleted:      qbtorrent.Completed,
			Leechers:           qbtorrent.Num_incomplete,
			Meta:               make(map[string]int64),
		}
		torrent.Name, torrent.Meta = client.ParseMetaFromName(torrent.Name)
		torrents = append(torrents, torrent)
	}
	return torrents, nil
}

func (qbclient *Client) GetTorrentContents(infoHash string) ([]client.TorrentContentFile, error) {
	err := qbclient.login()
	if err != nil {
		return nil, fmt.Errorf("login error: %v", err)
	}
	apiUrl := qbclient.ClientConfig.Url + "api/v2/torrents/files?hash=" + infoHash
	qbTorrentContents := []apiTorrentContent{}
	err = utils.FetchJson(apiUrl, &qbTorrentContents, qbclient.HttpClient)
	if err != nil {
		return nil, err
	}
	torrentContents := []client.TorrentContentFile{}
	for _, qbTorrentContent := range qbTorrentContents {
		torrentContents = append(torrentContents, client.TorrentContentFile{
			Index: qbTorrentContent.Index,
			Path:  qbTorrentContent.Name,
			Size:  qbTorrentContent.Size,
		})
	}
	return torrentContents, nil
}

func NewClient(name string, clientConfig *config.ClientConfigStruct, config *config.ConfigStruct) (client.Client, error) {
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
