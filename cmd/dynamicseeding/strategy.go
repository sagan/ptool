package dynamicseeding

import (
	"fmt"
	"io"

	"github.com/sagan/ptool/client"
	"github.com/sagan/ptool/cmd/common"
	"github.com/sagan/ptool/config"
	"github.com/sagan/ptool/constants"
	"github.com/sagan/ptool/site"
	"github.com/sagan/ptool/util"
)

// site torrent which current seeders >= MAX_SEEDERS will NOT be added;
// client torrent which current seeders (including self client) > MAX_SEEDERS will be actively deleted.
const MAX_SEEDERS = 10

// client torrent which current seeders <= MIN_SEEDERS will NEVER be deleted.
// client torrent that MIN_SEEDERS < current seeders <= MAX_SEEDERS may be deleted sometimes.
const MIN_SEEDERS = 3

const MAX_PARALLEL_DOWNLOAD = 5
const MIN_SIZE = 1024 * 1024 * 1024   // minimal dynamicSeedingSize required. 1GiB
const INACTIVITY_TIMESPAN = 86400 * 3 // If incomplete has no activity for enough time, abort.

type DeleteClientTorrent struct {
	InfoHash    string
	Size        int64
	DeleteFiles bool
}

type Result struct {
	Timestamp      int64
	Sitename       string
	DeleteTorrents []DeleteClientTorrent
	AddTorrents    []site.Torrent
	Msg            string
	Log            string
}

func (result *Result) Print(output io.Writer) {
	fmt.Fprintf(output, "dynamic-seeding of %q site at %s\n", result.Sitename, util.FormatTime(result.Timestamp))
	fmt.Fprintf(output, "Message: %s\n", result.Msg)
	fmt.Fprintf(output, "Log: %s\n", result.Log)
}

func doDynamicSeeding(clientInstance client.Client, siteInstance site.Site) (result *Result, err error) {
	timestamp := util.Now()
	if siteInstance.GetSiteConfig().DynamicSeedingSizeValue <= MIN_SIZE {
		return nil, fmt.Errorf("dynamicSeedingSize insufficient. Current value: %s. At least %s is required",
			util.BytesSizeAround(float64(siteInstance.GetSiteConfig().DynamicSeedingSizeValue)),
			util.BytesSizeAround(float64(MIN_SIZE)))
	}
	dynamicSeedingCat := config.DYNAMIC_SEEDING_CAT_PREFIX + siteInstance.GetName()
	clientStatus, err := clientInstance.GetStatus()
	if err != nil {
		return nil, fmt.Errorf("failed to get client status: %v", err)
	}
	result = &Result{
		Timestamp: timestamp,
		Sitename:  siteInstance.GetName(),
	}
	if clientStatus.NoAdd || clientStatus.NoDel {
		result.Msg = fmt.Sprintf("Client has %q or %q flag tag. Exit", config.NOADD_TAG, config.NODEL_TAG)
		return
	}
	downloadingSpeedLimit := clientStatus.DownloadSpeedLimit
	if downloadingSpeedLimit <= 0 {
		downloadingSpeedLimit = constants.CLIENT_DEFAULT_DOWNLOADING_SPEED_LIMIT
	}
	if float64(clientStatus.DownloadSpeed/downloadingSpeedLimit) >= 0.8 {
		result.Msg = fmt.Sprintf("Client incoming bandwidth is full (spd/lmt): %s / %s. Exit",
			util.BytesSize(float64(clientStatus.DownloadSpeed)), util.BytesSize(float64(downloadingSpeedLimit)))
		return
	}
	clientTorrents, err := clientInstance.GetTorrents("", dynamicSeedingCat, true)
	if err != nil {
		return nil, fmt.Errorf("failed to get client current dynamic seeding torrents: %v", err)
	}

	// triage for client torrents
	clientTorrentsMap := map[string]*client.Torrent{}
	// Torrents that are excluded from dynamic seeding deciding stragety.
	// These torrents will not be touched.
	var clientOtherTorrents []string
	var clientInvalidTorrents []string // invalid tracker (may be deleted)
	var clientStalledTorrents []string // incomplete and no activity for enough time, safe to delete.
	var clientDownloadingTorrents []string
	var clientSafeTorrents []string   // has enough seeders, safe to delete
	var clientNormalTorrents []string // seeding is normal
	var clientDangerousTorrents []string
	var clientUnknownTorrents []string
	// Invalid: clientOtherTorrents.
	// Success: clientDownloadingTorrents  + clientDangerousTorrents + clientUnknownTorrents; will never be deleted.
	// Fail: clientInvalidTorrents + clientStalledTorrents + clientSafeTorrents + clientNormalTorrents; could be deleted.
	var statistics = common.NewTorrentsStatistics()
	for _, torrent := range clientTorrents {
		clientTorrentsMap[torrent.InfoHash] = &torrent
		if !torrent.HasTag(client.GenerateTorrentTagFromSite(siteInstance.GetName())) {
			clientOtherTorrents = append(clientOtherTorrents, torrent.InfoHash)
			statistics.UpdateClientTorrent(common.TORRENT_INVALID, &torrent)
			continue
		}
		if !torrent.IsComplete() && (timestamp-torrent.ActivityTime) >= INACTIVITY_TIMESPAN {
			clientStalledTorrents = append(clientStalledTorrents, torrent.InfoHash)
			statistics.UpdateClientTorrent(common.TORRENT_FAILURE, &torrent)
			continue
		}
		if !torrent.IsComplete() {
			clientDownloadingTorrents = append(clientDownloadingTorrents, torrent.InfoHash)
			statistics.UpdateClientTorrent(common.TORRENT_SUCCESS, &torrent)
			continue
		}
		if torrent.State != "seeding" {
			clientOtherTorrents = append(clientOtherTorrents, torrent.InfoHash)
			statistics.UpdateClientTorrent(common.TORRENT_INVALID, &torrent)
			continue
		}
		if torrent.Seeders > MAX_SEEDERS {
			clientSafeTorrents = append(clientSafeTorrents, torrent.InfoHash)
			statistics.UpdateClientTorrent(common.TORRENT_FAILURE, &torrent)
		} else if torrent.Seeders > MIN_SEEDERS {
			clientNormalTorrents = append(clientNormalTorrents, torrent.InfoHash)
			statistics.UpdateClientTorrent(common.TORRENT_FAILURE, &torrent)
		} else if torrent.Seeders == 0 && torrent.Leechers == 0 {
			if trackers, err := clientInstance.GetTorrentTrackers(torrent.InfoHash); err != nil {
				clientUnknownTorrents = append(clientUnknownTorrents, torrent.InfoHash)
			} else if trackers.SeemsInvalidTorrent() {
				clientInvalidTorrents = append(clientInvalidTorrents, torrent.InfoHash)
			} else {
				clientStalledTorrents = append(clientStalledTorrents, torrent.InfoHash)
			}
		} else {
			clientDangerousTorrents = append(clientDangerousTorrents, torrent.InfoHash)
			statistics.UpdateClientTorrent(common.TORRENT_SUCCESS, &torrent)
		}
	}

	availableSpace := siteInstance.GetSiteConfig().DynamicSeedingSizeValue -
		statistics.SuccessSize + statistics.FailureSize
	if availableSpace < min(siteInstance.GetSiteConfig().DynamicSeedingSizeValue/10, MIN_SIZE) {
		result.Msg = "Insufficient dynamic seeding storage space in client. Exit"
		return
	}
	maxSiteTorrentSize := availableSpace
	if siteInstance.GetSiteConfig().DynamicSeedingTorrentMaxSizeValue > 0 {
		maxSiteTorrentSize = min(maxSiteTorrentSize, siteInstance.GetSiteConfig().DynamicSeedingTorrentMaxSizeValue)
	}
	dynamicSeedingUrl := siteInstance.GetSiteConfig().DynamicSeedingTorrentsUrl
	if siteInstance.GetSiteConfig().Type == "nexusphp" {
		// See https://github.com/xiaomlove/nexusphp/blob/php8/public/torrents.php .
		dynamicSeedingUrl = util.AppendUrlQueryString(dynamicSeedingUrl, "seeders_begin=1")
	}
	var siteTorrents []site.Torrent
	marker := ""
site_outer:
	for {
		torrents, nextPageMarker, err := siteInstance.GetAllTorrents("seeders", false, marker, dynamicSeedingUrl)
		if err != nil {
			result.Log += fmt.Sprintf("failed to get site torrents: %v\n", err)
			break
		}
		for _, torrent := range torrents {
			if torrent.Seeders >= MAX_SEEDERS {
				break site_outer
			}
			if torrent.Seeders < 1 || torrent.Size > maxSiteTorrentSize ||
				torrent.MatchFiltersOr(siteInstance.GetSiteConfig().DynamicSeedingExcludes) {
				continue
			}
			siteTorrents = append(siteTorrents, torrent)
		}
		if len(siteTorrents) >= MAX_PARALLEL_DOWNLOAD {
			break
		}
		if nextPageMarker == "" {
			break
		}
		marker = nextPageMarker
	}
	// site.PrintTorrents(siteTorrents, "", timestamp, false, false, nil)
	if len(siteTorrents) == 0 {
		result.Msg = "No candidate site dynamic seeding torrents found"
		return
	}
	return
}
