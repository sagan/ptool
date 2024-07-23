package transmission

// use https://github.com/hekmon/transmissionrpc
// protocol: https://github.com/transmission/transmission/blob/3.00/extras/rpc-spec.txt

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"

	"github.com/ettle/strcase"
	transmissionrpc "github.com/hekmon/transmissionrpc/v2"
	"github.com/sagan/ptool/client"
	"github.com/sagan/ptool/config"
	"github.com/sagan/ptool/constants"
	"github.com/sagan/ptool/util"
	log "github.com/sirupsen/logrus"
)

type Client struct {
	Name                      string
	ClientConfig              *config.ClientConfigStruct
	Config                    *config.ConfigStruct
	client                    *transmissionrpc.Client
	datatime                  int64
	datafull                  bool
	datatimeMeta              int64
	torrents                  map[string]*transmissionrpc.Torrent
	sessionStats              *transmissionrpc.SessionStats
	sessionArgs               *transmissionrpc.SessionArguments
	freeSpace                 int64
	unfinishedSize            int64
	unfinishedDownloadingSize int64
	contentPathTorrents       map[string][]*transmissionrpc.Torrent
	lastTorrent               *transmissionrpc.Torrent // a **really** simple cache with capacity of only one
}

func (trclient *Client) GetTorrentsByContentPath(contentPath string) ([]*client.Torrent, error) {
	if err := trclient.Sync(false); err != nil {
		return nil, err
	}
	torrents := []*client.Torrent{}
	for _, t := range trclient.contentPathTorrents[contentPath] {
		torrents = append(torrents, tr2Torrent(t))
	}
	return torrents, nil
}

var (
	ErrNotImplemented = errors.New("not implemented yet")
)

// SetAllTorrentsShareLimits implements client.Client.
func (trclient *Client) SetAllTorrentsShareLimits(ratioLimit float64, seedingTimeLimit int64) error {
	return ErrNotImplemented
}

// SetTorrentsShareLimits implements client.Client.
func (trclient *Client) SetTorrentsShareLimits(infoHashes []string, ratioLimit float64, seedingTimeLimit int64) error {
	return ErrNotImplemented
}

// get a torrent info from rpc. return error if torrent not found
func (trclient *Client) getTorrent(infoHash string, full bool) (*transmissionrpc.Torrent, error) {
	// If TrackerStats is present, it's a full info.
	if trclient.torrents[infoHash] != nil && (!full || trclient.torrents[infoHash].TrackerStats != nil) {
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

func (trclient *Client) Cached() bool {
	return trclient.datatime > 0
}

func (trclient *Client) Sync(full bool) (err error) {
	if trclient.datatime > 0 && (!full || trclient.datafull) {
		return nil
	}
	transmissionbt := trclient.client
	now := util.Now()
	var torrents []transmissionrpc.Torrent
	if full {
		torrents, err = transmissionbt.TorrentGetAll(context.TODO())
	} else {
		torrents, err = transmissionbt.TorrentGet(context.TODO(), []string{
			"activityDate", "addedDate", "doneDate", "downloadDir", "downloadedEver", "downloadLimit", "downloadLimited",
			"hashString", "id", "labels", "name", "peersGettingFromUs", "peersSendingToUs", "percentDone", "rateDownload",
			"rateUpload", "sizeWhenDone", "status", "trackers", "totalSize", "uploadedEver", "uploadLimit", "uploadLimited",
		}, nil)
	}

	if err != nil {
		return err
	}
	torrentsMap := map[string]*transmissionrpc.Torrent{}
	for i := range torrents {
		torrentsMap[*torrents[i].HashString] = &torrents[i]
	}
	trclient.datatime = now
	trclient.datafull = full
	trclient.torrents = torrentsMap
	trclient.buildDerivative()
	return nil
}

func (trclient *Client) buildDerivative() {
	unfinishedSize := int64(0)
	unfinishedDownloadingSize := int64(0)
	var contentPathTorrents = map[string][]*transmissionrpc.Torrent{}
	for _, torrent := range trclient.torrents {
		usize := int64(float64(*torrent.SizeWhenDone) * (1 - *torrent.PercentDone))
		unfinishedSize += usize
		if *torrent.Status != 0 {
			unfinishedDownloadingSize += usize
		}
		contentPath := getContentPath(torrent)
		contentPathTorrents[contentPath] = append(contentPathTorrents[contentPath], torrent)
	}
	trclient.unfinishedSize = unfinishedSize
	trclient.unfinishedDownloadingSize = unfinishedDownloadingSize
	trclient.contentPathTorrents = contentPathTorrents
}

func (trclient *Client) syncMeta() error {
	if trclient.datatimeMeta > 0 {
		return nil
	}
	transmissionbt := trclient.client
	now := util.Now()
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
	trclient.freeSpace = int64(freeSpace / 8) // tr freespace is in bits.
	return nil
}

func (trclient *Client) ExportTorrentFile(infoHash string) ([]byte, error) {
	if trclient.ClientConfig.LocalTorrentsPath != "" {
		return os.ReadFile(filepath.Join(trclient.ClientConfig.LocalTorrentsPath, infoHash+".torrent"))
	}
	return nil, fmt.Errorf("unsupported")
}

func (trclient *Client) GetTorrent(infoHash string) (*client.Torrent, error) {
	if err := trclient.Sync(false); err != nil {
		return nil, err
	}
	trtorrent := trclient.torrents[infoHash]
	if trtorrent == nil {
		return nil, nil
	}
	return tr2Torrent(trtorrent), nil
}

func (trclient *Client) GetTorrents(stateFilter string, category string, showAll bool) ([]*client.Torrent, error) {
	if err := trclient.Sync(false); err != nil {
		return nil, err
	}
	torrents := []*client.Torrent{}
	for _, trtorrent := range trclient.torrents {
		torrent := tr2Torrent(trtorrent)
		if category != "" {
			if category == constants.NONE {
				if torrent.Category != "" {
					continue
				}
			} else if category != torrent.Category {
				continue
			}
		}
		if !showAll && torrent.DownloadSpeed < 1024 && torrent.UploadSpeed < 1024 {
			continue
		}
		if !torrent.MatchStateFilter(stateFilter) {
			continue
		}
		torrents = append(torrents, torrent)
	}
	return torrents, nil
}

func (trclient *Client) AddTorrent(torrentContent []byte, option *client.TorrentOption, meta map[string]int64) error {
	transmissionbt := trclient.client
	var downloadDir *string
	if option.SavePath != "" {
		downloadDir = &option.SavePath
	}
	payload := transmissionrpc.TorrentAddPayload{
		Paused:      &option.Pause,
		DownloadDir: downloadDir,
	}
	if util.IsTorrentUrl(string(torrentContent)) {
		url := string(torrentContent)
		payload.Filename = &url
	} else {
		torrentContentB64 := base64.StdEncoding.EncodeToString(torrentContent)
		payload.MetaInfo = &torrentContentB64
	}
	// returned torrent will only have HashString, ID and Name fields set up.
	torrent, err := transmissionbt.TorrentAdd(context.TODO(), payload)
	if err != nil {
		return err
	}

	name := option.Name
	if name != "" {
		// it's not robust, and will actually rename the root file / folder name on disk
		err := transmissionbt.TorrentRenamePathHash(context.TODO(), *torrent.HashString, *torrent.Name, name)
		log.Tracef("rename tr torrent name=%s err=%v", name, err)
	}

	labels := util.CopySlice(option.Tags)
	if option.Category != "" && option.Category != constants.NONE {
		// use label to simulate category
		labels = append(labels, client.GenerateTorrentTagFromCategory(option.Category))
	}
	for name, value := range meta {
		labels = append(labels, client.GenerateTorrentTagFromMetadata(name, value))
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

func (trclient *Client) ModifyTorrent(infoHash string, option *client.TorrentOption, meta map[string]int64) error {
	transmissionbt := trclient.client
	trtorrent, err := trclient.getTorrent(infoHash, false)
	if err != nil {
		return err
	}
	torrent := tr2Torrent(trtorrent)

	if option.Name != "" && option.Name != torrent.Name {
		err := transmissionbt.TorrentRenamePathHash(context.TODO(), infoHash, *trtorrent.Name, option.Name)
		if err != nil {
			return err
		}
	}

	payload := transmissionrpc.TorrentSetPayload{
		IDs: []int64{*trtorrent.ID},
	}

	if (option.Category != "" && option.Category != constants.NONE) ||
		len(option.Tags) > 0 || len(option.RemoveTags) > 0 ||
		len(meta) > 0 || len(torrent.Meta) > 0 {
		labels := []string{}
		if option.Category != "" && torrent.Category != option.Category {
			categoryTag := client.GenerateTorrentTagFromCategory(option.Category)
			labels = append(labels, categoryTag)
		} else if torrent.Category != "" {
			categoryTag := client.GenerateTorrentTagFromCategory(torrent.Category)
			labels = append(labels, categoryTag)
		}
		labels = append(labels, option.Tags...)
		if len(meta) > 0 {
			for name, value := range meta {
				labels = append(labels, client.GenerateTorrentTagFromMetadata(name, value))
			}
		} else if len(torrent.Meta) > 0 {
			for name, value := range torrent.Meta {
				labels = append(labels, client.GenerateTorrentTagFromMetadata(name, value))
			}
		}
		if len(labels) > 0 || len(option.RemoveTags) > 0 {
			for _, tag := range torrent.Tags {
				if !slices.Contains(option.RemoveTags, tag) {
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
func (trclient *Client) DeleteTorrents(infoHashes []string, deleteFiles bool) (err error) {
	transmissionbt := trclient.client
	if err := trclient.Sync(false); err != nil {
		return err
	}
	err = transmissionbt.TorrentRemove(context.TODO(), transmissionrpc.TorrentRemovePayload{
		IDs:             trclient.getIds(infoHashes),
		DeleteLocalData: deleteFiles,
	})
	if err == nil && trclient.Cached() {
		for _, infoHash := range infoHashes {
			delete(trclient.torrents, infoHash)
		}
		trclient.buildDerivative()
	}
	return
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
	for i, infoHash := range infoHashes {
		log.Tracef("(%d/%d) transmission.AddTagsToTorrents: %s", i+1, len(infoHashes), infoHash)
		if trclient.torrents[infoHash] != nil && !slices.ContainsFunc(tags, func(tag string) bool {
			return !slices.Contains(trclient.torrents[infoHash].Labels, tag)
		}) {
			continue
		}
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
	for i, infoHash := range infoHashes {
		log.Tracef("(%d/%d) transmission.RemoveTagsFromTorrents: %s", i+1, len(infoHashes), infoHash)
		if trclient.torrents[infoHash] != nil && !slices.ContainsFunc(tags, func(tag string) bool {
			return slices.Contains(trclient.torrents[infoHash].Labels, tag)
		}) {
			continue
		}
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
	if err := trclient.Sync(false); err != nil {
		return err
	}
	return trclient.AddTagsToTorrents(trclient.getAllInfoHashes(), tags)
}

func (trclient *Client) RemoveTagsFromAllTorrents(tags []string) error {
	if err := trclient.Sync(false); err != nil {
		return err
	}
	return trclient.RemoveTagsFromTorrents(trclient.getAllInfoHashes(), tags)
}

func (trclient *Client) SetAllTorrentsSavePath(savePath string) error {
	if err := trclient.Sync(false); err != nil {
		return err
	}
	return trclient.SetTorrentsSavePath(trclient.getAllInfoHashes(), savePath)
}

func (trclient *Client) GetTags() ([]string, error) {
	if err := trclient.Sync(false); err != nil {
		return nil, err
	}
	tags := []string{}
	tagsFlag := map[string]bool{}
	for _, trtorrent := range trclient.torrents {
		for _, label := range trtorrent.Labels {
			if label != "" && !tagsFlag[label] && !client.IsSubstituteTag(label) {
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

func (trclient *Client) MakeCategory(category string, savePath string) error {
	return fmt.Errorf("unsupported")
}

func (trclient *Client) DeleteCategories(categories []string) error {
	return fmt.Errorf("unsupported")
}

func (trclient *Client) GetCategories() ([]*client.TorrentCategory, error) {
	if err := trclient.Sync(false); err != nil {
		return nil, err
	}
	cats := []*client.TorrentCategory{}
	catsFlag := map[string]bool{}
	for _, trtorrent := range trclient.torrents {
		torrent := tr2Torrent(trtorrent)
		cat := torrent.GetCategoryFromTag()
		if cat != "" && !catsFlag[cat] {
			cats = append(cats, &client.TorrentCategory{
				Name: cat,
			})
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
	if err := trclient.Sync(false); err != nil {
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
	if err := trclient.Sync(false); err != nil {
		return false
	}
	for _, torrent := range trclient.torrents {
		if *torrent.Name == rootFolder {
			return true
		}
	}
	return false
}

func (trclient *Client) GetTorrentContents(infoHash string) ([]*client.TorrentContentFile, error) {
	torrent, err := trclient.getTorrent(infoHash, true)
	if err != nil {
		return nil, err
	}
	files := []*client.TorrentContentFile{}
	for i, trTorrentFile := range torrent.Files {
		files = append(files, &client.TorrentContentFile{
			Path:     trTorrentFile.Name,
			Size:     trTorrentFile.Length,
			Ignored:  !torrent.FileStats[i].Wanted,
			Complete: trTorrentFile.BytesCompleted == trTorrentFile.Length,
			Progress: float64(trTorrentFile.BytesCompleted) / float64(trTorrentFile.Length),
		})
	}
	return files, nil
}

func (trclient *Client) PurgeCache() {
	trclient.datatime = 0
	trclient.datafull = false
	trclient.datatimeMeta = 0
	trclient.unfinishedSize = 0
	trclient.unfinishedDownloadingSize = 0
	trclient.sessionArgs = nil
	trclient.sessionStats = nil
	trclient.torrents = nil
	trclient.lastTorrent = nil
	trclient.contentPathTorrents = nil
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
		DownloadSpeed:             trclient.sessionStats.DownloadSpeed,
		UploadSpeed:               trclient.sessionStats.UploadSpeed,
		DownloadSpeedLimit:        downloadSpeedLimit,
		UploadSpeedLimit:          uploadSpeedLimit,
		FreeSpaceOnDisk:           trclient.freeSpace,
		UnfinishedSize:            trclient.unfinishedSize,
		UnfinishedDownloadingSize: trclient.unfinishedDownloadingSize,
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
	if strings.HasPrefix(variable, "tr_") && len(variable) > 3 {
		trvariable := strcase.ToKebab(variable[3:])
		key := strcase.ToPascal(trvariable)
		args := transmissionrpc.SessionArguments{}
		argValue, kind := util.String2Any(value)
		// it's ugly for now
		if kind == reflect.Int64 {
			value := argValue.(int64)
			util.SetStructFieldValue(&args, key, &value)
		} else if kind == reflect.Bool {
			value := argValue.(bool)
			util.SetStructFieldValue(&args, key, &value)
		} else if kind == reflect.String {
			value := argValue.(string)
			util.SetStructFieldValue(&args, key, &value)
		} else {
			return fmt.Errorf("invalid value type: %v", kind)
		}
		return transmissionbt.SessionArgumentsSet(context.TODO(), args)
	}
	switch variable {
	case "global_download_speed_limit":
		limit := util.ParseInt(value)
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
		limit := util.ParseInt(value)
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
	case "free_disk_space", "global_download_speed", "global_upload_speed":
		return fmt.Errorf("%s is read-only", variable)
	case "save_path":
		return transmissionbt.SessionArgumentsSet(context.TODO(), transmissionrpc.SessionArguments{
			DownloadDir: &value,
		})
	default:
		return nil
	}
}

func (trclient *Client) GetConfig(variable string) (string, error) {
	err := trclient.syncMeta()
	if err != nil {
		return "", err
	}
	if strings.HasPrefix(variable, "tr_") && len(variable) > 3 {
		trvariable := strcase.ToKebab(variable[3:])
		key := strcase.ToPascal(trvariable)
		defaultValue := ""
		value := util.ResolvePointerValue(util.GetStructFieldValue(trclient.sessionArgs, key, &defaultValue))
		return fmt.Sprint(value), nil
	}
	switch variable {
	case "global_download_speed_limit":
		if *trclient.sessionArgs.SpeedLimitDownEnabled {
			return fmt.Sprint(*trclient.sessionArgs.SpeedLimitDown * 1024), nil
		}
		return "0", nil
	case "global_upload_speed_limit":
		if *trclient.sessionArgs.SpeedLimitUpEnabled {
			return fmt.Sprint(*trclient.sessionArgs.SpeedLimitUp * 1024), nil
		}
		return "0", nil
	case "free_disk_space":
		status, err := trclient.GetStatus()
		if err != nil {
			return "", err
		}
		return fmt.Sprint(status.FreeSpaceOnDisk), nil
	case "global_download_speed":
		status, err := trclient.GetStatus()
		if err != nil {
			return "", err
		}
		return fmt.Sprint(status.DownloadSpeed), nil
	case "global_upload_speed":
		status, err := trclient.GetStatus()
		if err != nil {
			return "", err
		}
		return fmt.Sprint(status.UploadSpeed), nil
	case "save_path":
		return *trclient.sessionArgs.DownloadDir, nil
	default:
		return "", nil
	}
}
func (trclient *Client) GetTorrentTrackers(infoHash string) (client.TorrentTrackers, error) {
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
	directNewUrlMode := util.IsUrl(newTracker)
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

func (trclient *Client) AddTorrentTrackers(infoHash string, trackers []string,
	oldTracker string, removeExisting bool) error {
	trtorrent, err := trclient.getTorrent(infoHash, false)
	if err != nil {
		return err
	}
	if oldTracker != "" {
		if !slices.ContainsFunc(trtorrent.Trackers, func(trtracker *transmissionrpc.Tracker) bool {
			return util.MatchUrlWithHostOrUrl(trtracker.Announce, oldTracker)
		}) {
			return nil
		}
	}
	if !removeExisting {
		trackers = util.Filter(trackers, func(tracker string) bool {
			return !slices.ContainsFunc(trtorrent.Trackers, func(trtracker *transmissionrpc.Tracker) bool {
				return trtracker.Announce == tracker
			})
		})
	}
	if len(trackers) > 0 {
		payload := transmissionrpc.TorrentSetPayload{
			IDs:        []int64{*trtorrent.ID},
			TrackerAdd: trackers,
		}
		if removeExisting {
			payload.TrackerRemove = util.Map(trtorrent.Trackers, func(t *transmissionrpc.Tracker) int64 { return t.ID })
		}
		return trclient.client.TorrentSet(context.TODO(), payload)
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
		if slices.Contains(trackers, tracker.Announce) {
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

func (trclient *Client) SetFilePriority(infoHash string, fileIndexes []int64, priority int64) error {
	return ErrNotImplemented
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
	rpcUri := "" // Leave empty to use default "/transmission/rpc"
	if urlObj.Path != "" && urlObj.Path != "/" {
		rpcUri = urlObj.Path
	}
	if portStr != "" {
		port = util.ParseInt(portStr)
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
			HTTPS:  isHttps,
			Port:   uint16(port),
			RPCURI: rpcUri,
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

func getContentPath(trtorrent *transmissionrpc.Torrent) string {
	sep := "/"
	if strings.Contains(*trtorrent.DownloadDir, `\`) {
		sep = `\`
	}
	return *trtorrent.DownloadDir + sep + *trtorrent.Name
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
		TrackerDomain:      util.ParseUrlHostname(tracker),
		TrackerBaseDomain:  util.GetUrlDomain(tracker),
		Tracker:            tracker,
		State:              tr2State(trtorrent),
		LowLevelState:      fmt.Sprint(trtorrent.Status),
		Atime:              trtorrent.AddedDate.Unix(),
		Ctime:              trtorrent.DoneDate.Unix(), // 0 if not completed
		ActivityTime:       trtorrent.ActivityDate.Unix(),
		Downloaded:         *trtorrent.DownloadedEver,
		DownloadSpeed:      *trtorrent.RateDownload,
		DownloadSpeedLimit: downloadSpeedLimit,
		Uploaded:           *trtorrent.UploadedEver,
		UploadSpeed:        *trtorrent.RateUpload,
		UploadedSpeedLimit: uploadSpeedLimit,
		Category:           "",
		SavePath:           *trtorrent.DownloadDir,
		ContentPath:        getContentPath(trtorrent),
		Tags:               trtorrent.Labels,
		Seeders:            *trtorrent.PeersSendingToUs, // it's meaning is inconsistent with qb for now
		Size:               int64(*trtorrent.SizeWhenDone / 8),
		SizeCompleted:      int64(float64(*trtorrent.SizeWhenDone) * *trtorrent.PercentDone / 8),
		SizeTotal:          int64(*trtorrent.TotalSize / 8),
		Leechers:           *trtorrent.PeersGettingFromUs, // it's meaning is inconsistent with qb for now
		Meta:               nil,
	}
	torrent.Meta = torrent.GetMetadataFromTags()
	torrent.Category = torrent.GetCategoryFromTag()
	torrent.RemoveSubstituteTags()
	return torrent
}

var (
	_ client.Client = (*Client)(nil)
)
