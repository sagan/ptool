package transmission

// use https://github.com/hekmon/transmissionrpc
// protocol: https://github.com/transmission/transmission/blob/3.00/extras/rpc-spec.txt

import (
	"context"
	"encoding/base64"
	"fmt"
	"net/url"

	transmissionrpc "github.com/hekmon/transmissionrpc/v2"
	"github.com/sagan/ptool/client"
	"github.com/sagan/ptool/config"
	"github.com/sagan/ptool/utils"
	log "github.com/sirupsen/logrus"
	"golang.org/x/exp/slices"
)

type Client struct {
	Name           string
	ClientConfig   *config.ClientConfigStruct
	Config         *config.ConfigStruct
	client         *transmissionrpc.Client
	datatime       int64
	datatimeMeta   int64
	torrents       map[string](*transmissionrpc.Torrent)
	sessionStats   *transmissionrpc.SessionStats
	sessionArgs    *transmissionrpc.SessionArguments
	freeSpace      int64
	unfinishedSize int64
	lastTorrent    *transmissionrpc.Torrent // a **really** simple cache with capacity of only one
}

// get a torrent info from rpc. return error if torrent not found
func (trclient *Client) getTorrent(infoHash string, full bool) (*transmissionrpc.Torrent, error) {
	if !full && trclient.torrents[infoHash] != nil {
		return trclient.torrents[infoHash], nil
	}
	if trclient.lastTorrent != nil && *trclient.lastTorrent.HashString == infoHash {
		return trclient.lastTorrent, nil
	}
	transmissionbt := trclient.client
	torrents, err := transmissionbt.TorrentGetAllForHashes(context.TODO(), []string{infoHash})
	if err != nil {
		return nil, err
	}
	if len(torrents) == 0 {
		return nil, fmt.Errorf("torrent not found")
	}
	trclient.lastTorrent = &torrents[0]
	return &torrents[0], err
}

func (trclient *Client) getIds(infoHashes []string) []int64 {
	ids := []int64{}
	if infoHashes == nil {
		for _, torrent := range trclient.torrents {
			ids = append(ids, *torrent.ID)
		}
	} else {
		for _, infoHash := range infoHashes {
			if trclient.torrents[infoHash] != nil {
				ids = append(ids, *trclient.torrents[infoHash].ID)
			}
		}
	}
	return ids
}

func (trclient *Client) getAllInfoHashes() []string {
	infoHashes := []string{}
	for infohash := range trclient.torrents {
		infoHashes = append(infoHashes, infohash)
	}
	return infoHashes
}

func (trclient *Client) sync() error {
	if trclient.datatime > 0 {
		return nil
	}
	transmissionbt := trclient.client
	now := utils.Now()
	torrents, err := transmissionbt.TorrentGet(context.TODO(), []string{
		"addedDate", "doneDate", "downloadDir", "downloadedEver", "downloadLimit", "downloadLimited",
		"hashString", "id", "labels", "name", "peersGettingFromUs", "peersSendingToUs", "percentDone", "rateDownload", "rateUpload",
		"sizeWhenDone", "status", "trackers", "totalSize", "uploadedEver", "uploadLimit", "uploadLimited",
	}, nil)
	if err != nil {
		return err
	}
	torrentsMap := map[string](*transmissionrpc.Torrent){}
	unfinishedSize := int64(0)
	for i := range torrents {
		torrentsMap[*torrents[i].HashString] = &torrents[i]
		unfinishedSize += int64(float64(*torrents[i].SizeWhenDone) * (1 - *torrents[i].PercentDone))
	}
	trclient.datatime = now
	trclient.torrents = torrentsMap
	trclient.unfinishedSize = unfinishedSize
	return nil
}

func (trclient *Client) syncMeta() error {
	if trclient.datatimeMeta > 0 {
		return nil
	}
	transmissionbt := trclient.client
	now := utils.Now()
	sessionStats, err := transmissionbt.SessionStats(context.TODO())
	if err != nil {
		return err
	}
	sessionArgs, err := transmissionbt.SessionArgumentsGet(context.TODO(), nil)
	if err != nil {
		return err
	}
	freeSpace, err := transmissionbt.FreeSpace(context.TODO(), *sessionArgs.DownloadDir)
	if err != nil {
		return err
	}
	trclient.datatimeMeta = now
	trclient.sessionStats = &sessionStats
	trclient.sessionArgs = &sessionArgs
	trclient.freeSpace = int64(freeSpace)
	return nil
}

func (trclient *Client) GetTorrent(infoHash string) (*client.Torrent, error) {
	if err := trclient.sync(); err != nil {
		return nil, err
	}
	trtorrent := trclient.torrents[infoHash]
	if trtorrent == nil {
		return nil, nil
	}
	return tr2Torrent(trtorrent), nil
}

func (trclient *Client) GetTorrents(stateFilter string, category string, showAll bool) ([]client.Torrent, error) {
	if err := trclient.sync(); err != nil {
		return nil, err
	}
	torrents := make([]client.Torrent, 0)
	for _, trtorrent := range trclient.torrents {
		torrent := tr2Torrent(trtorrent)
		if category != "" && category != torrent.Category {
			continue
		}
		if !showAll && torrent.DownloadSpeed < 1024 && torrent.UploadSpeed < 1024 {
			continue
		}
		if !torrent.MatchStateFilter(stateFilter) {
			continue
		}
		torrents = append(torrents, *torrent)
	}
	return torrents, nil
}

func (trclient *Client) AddTorrent(torrentContent []byte, option *client.TorrentOption, meta map[string](int64)) error {
	transmissionbt := trclient.client
	torrentContentB64 := base64.StdEncoding.EncodeToString(torrentContent)
	var downloadDir *string
	if option.SavePath != "" {
		downloadDir = &option.SavePath
	}
	// returned torrent will only have HashString, ID and Name fields set up.
	torrent, err := transmissionbt.TorrentAdd(context.TODO(), transmissionrpc.TorrentAddPayload{
		MetaInfo:    &torrentContentB64,
		Paused:      &option.Pause,
		DownloadDir: downloadDir,
	})
	if err != nil {
		return err
	}

	name := client.GenerateNameWithMeta(option.Name, meta)
	if name != "" {
		// it's not robust, and will actually rename the root file / folder name on disk
		err := transmissionbt.TorrentRenamePathHash(context.TODO(), *torrent.HashString, *torrent.Name, name)
		log.Tracef("rename tr torrent name=%s err=%v", name, err)
	}

	labels := utils.CopySlice(option.Tags)
	if option.Category != "" {
		// use label to simulate category
		labels = append(labels, client.GenerateTorrentTagFromCategory(option.Category))
	}
	uploadLimit := int64(0)
	downloadLimit := int64(0)
	uploadLimited := false
	downloadLimited := false
	if option.UploadSpeedLimit > 0 {
		uploadLimit = option.UploadSpeedLimit / 1024
		if uploadLimit == 0 {
			uploadLimit = 1
		}
		uploadLimited = true
	}
	if option.DownloadSpeedLimit > 0 {
		downloadLimit = option.DownloadSpeedLimit / 1024
		if downloadLimit == 0 {
			downloadLimit = 1
		}
		downloadLimited = true
	}
	if len(labels) > 0 || uploadLimited || downloadLimited {
		err := transmissionbt.TorrentSet(context.TODO(), transmissionrpc.TorrentSetPayload{
			IDs:             []int64{*torrent.ID},
			Labels:          labels,
			UploadLimited:   &uploadLimited,
			UploadLimit:     &uploadLimit,
			DownloadLimit:   &downloadLimit,
			DownloadLimited: &downloadLimited,
		})
		log.Tracef("set tr torrent err=%v", err)
	}

	return nil
}

func (trclient *Client) ModifyTorrent(infoHash string, option *client.TorrentOption, meta map[string](int64)) error {
	trtorrent, err := trclient.getTorrent(infoHash, false)
	transmissionbt := trclient.client
	if err != nil {
		return err
	}
	torrent := tr2Torrent(trtorrent)

	if option.Name != "" || len(meta) > 0 {
		name := option.Name
		if name == "" {
			name = torrent.Name
		}
		name = client.GenerateNameWithMeta(name, meta)
		if name != *trtorrent.Name {
			err := transmissionbt.TorrentRenamePathHash(context.TODO(), infoHash, *trtorrent.Name, name)
			if err != nil {
				return err
			}
		}
	}

	payload := transmissionrpc.TorrentSetPayload{
		IDs: []int64{*trtorrent.ID},
	}

	if option.Category != "" || len(option.Tags) > 0 || len(option.RemoveTags) > 0 {
		labels := []string{}
		if option.Category != "" && torrent.Category != option.Category {
			categoryTag := client.GenerateTorrentTagFromCategory(option.Category)
			labels = append(labels, categoryTag)
		} else if torrent.Category != "" {
			categoryTag := client.GenerateTorrentTagFromCategory(torrent.Category)
			labels = append(labels, categoryTag)
		}
		labels = append(labels, option.Tags...)
		if len(labels) > 0 || len(option.RemoveTags) > 0 {
			for _, tag := range torrent.Tags {
				if slices.Index(option.RemoveTags, tag) == -1 {
					labels = append(labels, tag)
				}
			}
			payload.Labels = labels
		}
	}

	if option.DownloadSpeedLimit != 0 && option.DownloadSpeedLimit != torrent.DownloadSpeedLimit {
		downloadLimited := true
		downloadLimit := int64(0)
		if option.DownloadSpeedLimit > 0 {
			downloadLimit := option.DownloadSpeedLimit / 1024
			if downloadLimit == 0 {
				downloadLimit = 1
			}
		} else {
			downloadLimited = false
		}

		payload.DownloadLimited = &downloadLimited
		payload.DownloadLimit = &downloadLimit
	}
	if option.UploadSpeedLimit != 0 && option.UploadSpeedLimit != torrent.UploadedSpeedLimit {
		uploadLimited := true
		uploadLimit := int64(0)
		if option.UploadSpeedLimit > 0 {
			uploadLimit = option.UploadSpeedLimit / 1024
			if uploadLimit == 0 {
				uploadLimit = 1
			}
		} else {
			uploadLimited = false
		}

		payload.UploadLimited = &uploadLimited
		payload.UploadLimit = &uploadLimit
	}

	if option.SavePath != "" {
		payload.Location = &option.SavePath
	}

	transmissionbt.TorrentSet(context.TODO(), payload)

	if option.Pause {
		err = trclient.PauseTorrents([]string{infoHash})
	} else if option.Resume {
		err = trclient.ResumeTorrents([]string{infoHash})
	}
	return err
}

// suboptimal due to limit of transmissionrpc library
func (trclient *Client) DeleteTorrents(infoHashes []string, deleteFiles bool) error {
	transmissionbt := trclient.client
	if err := trclient.sync(); err != nil {
		return err
	}
	transmissionbt.TorrentRemove(context.TODO(), transmissionrpc.TorrentRemovePayload{
		IDs:             trclient.getIds(infoHashes),
		DeleteLocalData: deleteFiles,
	})
	return nil
}

func (trclient *Client) PauseTorrents(infoHashes []string) error {
	return trclient.client.TorrentStopHashes(context.TODO(), infoHashes)
}

func (trclient *Client) ResumeTorrents(infoHashes []string) error {
	return trclient.client.TorrentStartHashes(context.TODO(), infoHashes)
}

func (trclient *Client) RecheckTorrents(infoHashes []string) error {
	return trclient.client.TorrentVerifyHashes(context.TODO(), infoHashes)
}

func (trclient *Client) ReannounceTorrents(infoHashes []string) error {
	return trclient.client.TorrentReannounceHashes(context.TODO(), infoHashes)
}

func (trclient *Client) AddTagsToTorrents(infoHashes []string, tags []string) error {
	for _, infoHash := range infoHashes {
		err := trclient.ModifyTorrent(infoHash, &client.TorrentOption{
			Tags: tags,
		}, nil)
		if err != nil {
			return err
		}
	}
	return nil
}

func (trclient *Client) RemoveTagsFromTorrents(infoHashes []string, tags []string) error {
	for _, infoHash := range infoHashes {
		err := trclient.ModifyTorrent(infoHash, &client.TorrentOption{
			RemoveTags: tags,
		}, nil)
		if err != nil {
			return err
		}
	}
	return nil
}

func (trclient *Client) SetTorrentsSavePath(infoHashes []string, savePath string) error {
	// it's a limit imposed by transmissionrpc library that can not batch update savePath
	for _, infoHash := range infoHashes {
		err := trclient.client.TorrentSetLocationHash(context.TODO(), infoHash, savePath, true)
		if err != nil {
			return err
		}
	}
	return nil
}

func (trclient *Client) PauseAllTorrents() error {
	return trclient.client.TorrentStopHashes(context.TODO(), nil)
}

func (trclient *Client) ResumeAllTorrents() error {
	return trclient.client.TorrentStartHashes(context.TODO(), nil)
}

func (trclient *Client) RecheckAllTorrents() error {
	return trclient.client.TorrentVerifyHashes(context.TODO(), nil)
}

func (trclient *Client) ReannounceAllTorrents() error {
	return trclient.client.TorrentReannounceHashes(context.TODO(), nil)
}

func (trclient *Client) AddTagsToAllTorrents(tags []string) error {
	if err := trclient.sync(); err != nil {
		return err
	}
	return trclient.AddTagsToTorrents(trclient.getAllInfoHashes(), tags)
}

func (trclient *Client) RemoveTagsFromAllTorrents(tags []string) error {
	if err := trclient.sync(); err != nil {
		return err
	}
	return trclient.RemoveTagsFromTorrents(trclient.getAllInfoHashes(), tags)
}

func (trclient *Client) SetAllTorrentsSavePath(savePath string) error {
	if err := trclient.sync(); err != nil {
		return err
	}
	return trclient.SetTorrentsSavePath(trclient.getAllInfoHashes(), savePath)
}

func (trclient *Client) GetTags() ([]string, error) {
	if err := trclient.sync(); err != nil {
		return nil, err
	}
	tags := []string{}
	tagsFlag := map[string](bool){}
	for _, trtorrent := range trclient.torrents {
		for _, label := range trtorrent.Labels {
			if label != "" && !tagsFlag[label] && !client.IsCategoryTag(label) {
				tags = append(tags, label)
				tagsFlag[label] = true
			}
		}
	}
	return tags, nil
}

func (trclient *Client) CreateTags(tags ...string) error {
	return fmt.Errorf("unsupported")
}

func (trclient *Client) DeleteTags(tags ...string) error {
	return trclient.RemoveTagsFromAllTorrents(tags)
}

func (trclient *Client) GetCategories() ([]string, error) {
	if err := trclient.sync(); err != nil {
		return nil, err
	}
	cats := []string{}
	catsFlag := map[string](bool){}
	for _, trtorrent := range trclient.torrents {
		torrent := tr2Torrent(trtorrent)
		cat := torrent.GetCategoryFromTag()
		if cat != "" && !catsFlag[cat] {
			cats = append(cats, cat)
			catsFlag[cat] = true
		}
	}
	return cats, nil
}

func (trclient *Client) SetTorrentsCatetory(infoHashes []string, category string) error {
	for _, infoHash := range infoHashes {
		trclient.ModifyTorrent(infoHash, &client.TorrentOption{
			Category: category,
		}, nil)
	}
	return nil
}

func (trclient *Client) SetAllTorrentsCatetory(category string) error {
	if err := trclient.sync(); err != nil {
		return err
	}
	for infoHash := range trclient.torrents {
		trclient.ModifyTorrent(infoHash, &client.TorrentOption{
			Category: category,
		}, nil)
	}
	return nil
}

func (trclient *Client) TorrentRootPathExists(rootFolder string) bool {
	if rootFolder == "" {
		return false
	}
	if err := trclient.sync(); err != nil {
		return false
	}
	for _, torrent := range trclient.torrents {
		if *torrent.Name == rootFolder {
			return true
		}
	}
	return false
}

func (trclient *Client) GetTorrentContents(infoHash string) ([]client.TorrentContentFile, error) {
	torrent, err := trclient.getTorrent(infoHash, true)
	if err != nil {
		return nil, err
	}
	files := []client.TorrentContentFile{}
	for _, trTorrentFile := range torrent.Files {
		files = append(files, client.TorrentContentFile{
			Path:     trTorrentFile.Name,
			Size:     trTorrentFile.Length,
			Complete: trTorrentFile.BytesCompleted == trTorrentFile.Length,
		})
	}
	return files, nil
}

func (trclient *Client) PurgeCache() {
	trclient.datatime = 0
	trclient.datatimeMeta = 0
	trclient.unfinishedSize = 0
	trclient.sessionArgs = nil
	trclient.sessionStats = nil
	trclient.torrents = nil
	trclient.lastTorrent = nil
}

func (trclient *Client) GetStatus() (*client.Status, error) {
	if err := trclient.syncMeta(); err != nil {
		return nil, err
	}
	downloadSpeedLimit := int64(0)
	uploadSpeedLimit := int64(0)
	if *trclient.sessionArgs.SpeedLimitUpEnabled {
		uploadSpeedLimit = *trclient.sessionArgs.SpeedLimitUp * 1024
	}
	if *trclient.sessionArgs.SpeedLimitDownEnabled {
		downloadSpeedLimit = *trclient.sessionArgs.SpeedLimitDown * 1024
	}
	return &client.Status{
		DownloadSpeed:      trclient.sessionStats.DownloadSpeed,
		UploadSpeed:        trclient.sessionStats.UploadSpeed,
		DownloadSpeedLimit: downloadSpeedLimit,
		UploadSpeedLimit:   uploadSpeedLimit,
		FreeSpaceOnDisk:    trclient.freeSpace,
		UnfinishedSize:     trclient.unfinishedSize,
	}, nil
}

func (trclient *Client) GetName() string {
	return trclient.Name
}

func (trclient *Client) GetClientConfig() *config.ClientConfigStruct {
	return trclient.ClientConfig
}

func (trclient *Client) SetConfig(variable string, value string) error {
	transmissionbt := trclient.client
	switch variable {
	case "global_download_speed_limit":
		limit := utils.ParseInt(value)
		limited := false
		if limit > 0 {
			limited = true
			limit = limit / 1024
			if limit == 0 {
				limit = 1
			}
		}
		return transmissionbt.SessionArgumentsSet(context.TODO(), transmissionrpc.SessionArguments{
			SpeedLimitDownEnabled: &limited,
			SpeedLimitDown:        &limit,
		})
	case "global_upload_speed_limit":
		limit := utils.ParseInt(value)
		limited := false
		if limit > 0 {
			limited = true
			limit = limit / 1024
			if limit == 0 {
				limit = 1
			}
		}
		return transmissionbt.SessionArgumentsSet(context.TODO(), transmissionrpc.SessionArguments{
			SpeedLimitUpEnabled: &limited,
			SpeedLimitUp:        &limit,
		})
	default:
		return nil
	}
}

func (trclient *Client) GetConfig(variable string) (string, error) {
	transmissionbt := trclient.client
	switch variable {
	case "global_download_speed_limit":
		sessionStats, err := transmissionbt.SessionArgumentsGet(context.TODO(), nil)
		if err != nil {
			return "", nil
		}
		if *sessionStats.SpeedLimitDownEnabled {
			return fmt.Sprint(*sessionStats.SpeedLimitDown * 1024), nil
		}
		return "0", nil
	case "global_upload_speed_limit":
		sessionStats, err := transmissionbt.SessionArgumentsGet(context.TODO(), nil)
		if err != nil {
			return "", nil
		}
		if *sessionStats.SpeedLimitUpEnabled {
			return fmt.Sprint(*sessionStats.SpeedLimitUp * 1024), nil
		}
		return "0", nil
	default:
		return "", nil
	}
}
func (trclient *Client) GetTorrentTrackers(infoHash string) ([]client.TorrentTracker, error) {
	torrent, err := trclient.getTorrent(infoHash, true)
	if err != nil {
		return nil, err
	}
	trackers := []client.TorrentTracker{}
	for _, trackerStat := range torrent.TrackerStats {
		status := "unknown"
		if trackerStat.LastAnnounceSucceeded {
			status = "working"
		} else {
			status = "error"
		}
		msg := trackerStat.LastAnnounceResult
		if msg == "" {
			msg = trackerStat.LastScrapeResult
		}
		trackers = append(trackers, client.TorrentTracker{
			Url:    trackerStat.Announce,
			Status: status,
			Msg:    msg,
		})
	}
	return trackers, nil
}

func (trclient *Client) EditTorrentTracker(infoHash string, oldTracker string, newTracker string, replaceHost bool) error {
	trtorrent, err := trclient.getTorrent(infoHash, false)
	if err != nil {
		return err
	}
	oldTrackerId := int64(-1)
	oldTrackerUrl := ""
	newTrackerUrl := newTracker
	directNewUrlMode := utils.IsUrl(newTracker)
	for _, tracker := range trtorrent.Trackers {
		if replaceHost {
			oldTrackerUrlObj, err := url.Parse(tracker.Announce)
			if err != nil {
				continue
			}
			if oldTrackerUrlObj.Host == oldTracker {
				oldTrackerId = tracker.ID
				oldTrackerUrl = tracker.Announce
				if directNewUrlMode {
					newTrackerUrl = newTracker
					break
				}
				oldTrackerUrlObj.Host = newTracker
				newTrackerUrl = oldTrackerUrlObj.String()
				break
			}
		} else if tracker.Announce == oldTracker {
			oldTrackerId = tracker.ID
			break
		}
	}
	if oldTrackerId == -1 {
		return fmt.Errorf("torrent %s old tracker %s does NOT exist", *trtorrent.HashString, oldTracker)
	}
	if oldTrackerUrl == newTrackerUrl {
		return nil
	}
	// this is broken for now as transmission RPC expects trackerReplace to be
	// a mixed types array of ids (integer) and urls(string)
	// it's a problem of transmissionrpc library
	return trclient.client.TorrentSet(context.TODO(), transmissionrpc.TorrentSetPayload{
		IDs:            []int64{*trtorrent.ID},
		TrackerReplace: []any{oldTrackerId, newTrackerUrl},
	})
}

func (trclient *Client) AddTorrentTrackers(infoHash string, trackers []string) error {
	trtorrent, err := trclient.getTorrent(infoHash, false)
	if err != nil {
		return err
	}
	trackers = utils.Filter(trackers, func(tracker string) bool {
		return slices.IndexFunc(trtorrent.Trackers, func(trtracker *transmissionrpc.Tracker) bool {
			return trtracker.Announce == tracker
		}) == -1
	})
	if len(trackers) > 0 {
		return trclient.client.TorrentSet(context.TODO(), transmissionrpc.TorrentSetPayload{
			IDs:        []int64{*trtorrent.ID},
			TrackerAdd: trackers,
		})
	}
	return nil
}

func (trclient *Client) RemoveTorrentTrackers(infoHash string, trackers []string) error {
	trtorrent, err := trclient.getTorrent(infoHash, false)
	if err != nil {
		return err
	}
	trackerIds := []int64{}
	for _, tracker := range trtorrent.Trackers {
		if slices.Index(trackers, tracker.Announce) != -1 {
			trackerIds = append(trackerIds, tracker.ID)
		}
	}
	if len(trackerIds) > 0 {
		return trclient.client.TorrentSet(context.TODO(), transmissionrpc.TorrentSetPayload{
			IDs:           []int64{*trtorrent.ID},
			TrackerRemove: trackerIds,
		})
	}
	return nil
}

func (trclient *Client) Close() {
	trclient.PurgeCache()
}

func NewClient(name string, clientConfig *config.ClientConfigStruct, config *config.ConfigStruct) (
	client.Client, error) {
	urlObj, err := url.Parse(clientConfig.Url)
	if err != nil {
		return nil, err
	}
	schema := urlObj.Scheme
	hostname := urlObj.Hostname()
	portStr := urlObj.Port()
	port := int64(80)
	isHttps := schema == "https"
	if portStr != "" {
		port = utils.ParseInt(portStr)
	} else {
		if isHttps {
			port = 443
		} else {
			port = 80
		}
	}
	if (schema != "http" && schema != "https") || hostname == "" || port == 0 {
		return nil, fmt.Errorf("invalid tr url: %s", clientConfig.Url)
	}
	client, err := transmissionrpc.New(hostname, clientConfig.Username, clientConfig.Password,
		&transmissionrpc.AdvancedConfig{
			HTTPS: isHttps,
			Port:  uint16(port),
		})
	if err != nil {
		return nil, err
	}
	return &Client{
		Name:         name,
		ClientConfig: clientConfig,
		Config:       config,
		client:       client,
	}, nil
}

func init() {
	client.Register(&client.RegInfo{
		Name:    "transmission",
		Creator: NewClient,
	})
}

func tr2State(trtorrent *transmissionrpc.Torrent) string {
	switch *trtorrent.Status {
	case 0: // TorrentStatusStopped
		if trtorrent.DoneDate.Unix() > 0 {
			return "completed"
		}
		return "paused"
	case 1: // TorrentStatusCheckWait
		return "checking"
	case 2: // TorrentStatusCheck
		return "checking"
	case 3: // TorrentStatusDownloadWait
		return "downloading"
	case 4: // TorrentStatusDownload
		return "downloading"
	case 5: // TorrentStatusSeedWait
		return "seeding"
	case 6: // TorrentStatusSeed
		return "seeding"
	case 7: // TorrentStatusIsolated
		if trtorrent.DoneDate.Unix() > 0 {
			return "seeding"
		}
		return "downloading"
	default:
		return "unknown"
	}
}

func tr2Torrent(trtorrent *transmissionrpc.Torrent) *client.Torrent {
	uploadSpeedLimit := int64(0)
	downloadSpeedLimit := int64(0)
	if *trtorrent.UploadLimited {
		uploadSpeedLimit = *trtorrent.UploadLimit * 1024
	}
	if *trtorrent.DownloadLimited {
		downloadSpeedLimit = *trtorrent.DownloadLimit * 1024
	}
	tracker := ""
	if len(trtorrent.Trackers) > 0 {
		tracker = trtorrent.Trackers[0].Announce
	}
	torrent := &client.Torrent{
		InfoHash:           *trtorrent.HashString,
		Name:               *trtorrent.Name,
		TrackerDomain:      utils.ParseUrlHostname(tracker),
		Tracker:            tracker,
		State:              tr2State(trtorrent),
		LowLevelState:      fmt.Sprint(trtorrent.Status),
		Atime:              trtorrent.AddedDate.Unix(),
		Ctime:              trtorrent.DoneDate.Unix(), // 0 if not completed
		Downloaded:         *trtorrent.DownloadedEver,
		DownloadSpeed:      *trtorrent.RateDownload,
		DownloadSpeedLimit: downloadSpeedLimit,
		Uploaded:           *trtorrent.UploadedEver,
		UploadSpeed:        *trtorrent.RateUpload,
		UploadedSpeedLimit: uploadSpeedLimit,
		Category:           "",
		SavePath:           *trtorrent.DownloadDir,
		ContentPath:        *trtorrent.DownloadDir + "/" + *trtorrent.Name,
		Tags:               trtorrent.Labels,
		Seeders:            *trtorrent.PeersSendingToUs, // it's meaning is inconsistent with qb for now
		Size:               int64(*trtorrent.SizeWhenDone),
		SizeCompleted:      int64(float64(*trtorrent.SizeWhenDone) * *trtorrent.PercentDone),
		SizeTotal:          int64(*trtorrent.TotalSize),
		Leechers:           *trtorrent.PeersGettingFromUs, // it's meaning is inconsistent with qb for now
		Meta:               make(map[string]int64),
	}
	torrent.Name, torrent.Meta = client.ParseMetaFromName(torrent.Name)
	torrent.Category = torrent.GetCategoryFromTag()
	torrent.Tags = utils.FilterNot(torrent.Tags, client.IsCategoryTag)
	return torrent
}

var (
	_ client.Client = (*Client)(nil)
)
