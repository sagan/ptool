package dynamicseeding

import (
	"fmt"
	"io"
	"math/rand"
	"os"
	"slices"
	"sort"

	log "github.com/sirupsen/logrus"

	"github.com/sagan/ptool/client"
	"github.com/sagan/ptool/cmd/common"
	"github.com/sagan/ptool/config"
	"github.com/sagan/ptool/constants"
	"github.com/sagan/ptool/site"
	"github.com/sagan/ptool/util"
)

type TrackersStatus int

const (
	TRACKER_UNKNOWN TrackersStatus = iota
	TRACKER_OK
	TRACKER_INVALID
)

// site torrent which current seeders >= MAX_SEEDERS will NOT be added;
// client torrent which current seeders (including self client) > MAX_SEEDERS could be safely deleted.
const MAX_SEEDERS = 10

// client torrent which current seeders <= MIN_SEEDERS will NEVER be deleted.
// client torrent that MIN_SEEDERS < current seeders <= MAX_SEEDERS may be deleted sometimes.
const MIN_SEEDERS = 3

// client torrent params
const MIN_SIZE = 10 * 1024 * 1024 * 1024 // minimal dynamicSeedingSize required. 10GiB
const NEW_TORRENT_TIMESPAN = 3600
const MAX_PARALLEL_DOWNLOAD = 3
const INACTIVITY_TIMESPAN = 3600 * 3 // If incomplete torrent has no activity for enough time, abort.
// Could replace a client seeding torrent with a new site torrent if their seeders diff >= this
const MIN_REPLACE_SEEDERS_DIFF = 3
const MIN_SEEDING_TIME = 86400 * 10 // completed torrent will NOT be deleted during this time span. 10d

// site torrent params
const MIN_TORRENT_AGE = 86400 * 15 // site torrent must be published before this time ago
const MIN_FREE_REMAINING_TIME = 3600 * 3
const MAX_SCANNED_TORRENTS = 1000

type Result struct {
	Timestamp         int64
	Sitename          string
	Size              int64
	DeleteTorrents    []client.Torrent
	AddTorrents       []site.Torrent
	AddTorrentsOption *client.TorrentOption
	Msg               string
	Log               string
}

func (result *Result) Print(output io.Writer) {
	fmt.Fprintf(output, "dynamic-seeding of %q site at %s\n", result.Sitename, util.FormatTime(result.Timestamp))
	fmt.Fprintf(output, "Use at most %s of disk to dynamic-seeding\n", util.BytesSize(float64(result.Size)))
	fmt.Fprintf(output, "Message: %s\n", result.Msg)
	if len(result.DeleteTorrents) > 0 {
		fmt.Fprintf(output, "\nDelete torents from client:\n")
		client.PrintTorrents(os.Stdout, result.DeleteTorrents, "", 1, false)
	}
	if len(result.AddTorrents) > 0 {
		fmt.Fprintf(output, "\nAdd torents to client:\n")
		site.PrintTorrents(os.Stdout, result.AddTorrents, "", result.Timestamp, false, false, nil)
	}
	fmt.Fprintf(output, "\nLog:\n%s\n", result.Log)
}

func doDynamicSeeding(clientInstance client.Client, siteInstance site.Site, ignores []string) (
	result *Result, err error) {
	timestamp := util.Now()
	if siteInstance.GetSiteConfig().DynamicSeedingSizeValue <= MIN_SIZE {
		return nil, fmt.Errorf("dynamicSeedingSize insufficient. Current value: %s. At least %s is required",
			util.BytesSizeAround(float64(siteInstance.GetSiteConfig().DynamicSeedingSizeValue)),
			util.BytesSizeAround(float64(MIN_SIZE)))
	}
	if siteInstance.GetSiteConfig().GlobalHnR {
		return nil, fmt.Errorf("site that enforces global H&R policy is not supported at this time")
	}
	dynamicSeedingCat := config.DYNAMIC_SEEDING_CAT_PREFIX + siteInstance.GetName()
	dynamicSeedingTag := client.GenerateTorrentTagFromSite(siteInstance.GetName())
	clientStatus, err := clientInstance.GetStatus()
	if err != nil {
		return nil, fmt.Errorf("failed to get client status: %w", err)
	}
	result = &Result{
		Timestamp: timestamp,
		Sitename:  siteInstance.GetName(),
		Size:      siteInstance.GetSiteConfig().DynamicSeedingSizeValue,
		AddTorrentsOption: &client.TorrentOption{
			Category: dynamicSeedingCat,
			Tags:     []string{dynamicSeedingTag},
		},
	}
	if clientStatus.NoAdd {
		result.Msg = fmt.Sprintf("Client has %q tag. Exit", config.NOADD_TAG)
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
		return nil, fmt.Errorf("failed to get client current dynamic seeding torrents: %w", err)
	}
	sort.Slice(clientTorrents, func(i, j int) bool {
		if clientTorrents[i].Seeders != clientTorrents[j].Seeders {
			return clientTorrents[i].Seeders > clientTorrents[j].Seeders
		}
		return clientTorrents[i].Size < clientTorrents[j].Size
	})
	fmt.Fprintf(os.Stderr, "client category %q torrents:\n", dynamicSeedingCat)
	client.PrintTorrents(os.Stderr, clientTorrents, "", 1, false)

	var maxScan = util.FirstNonZeroIntegerArg(siteInstance.GetSiteConfig().DynamicSeedingMaxScan, MAX_SCANNED_TORRENTS)
	var minSeeders = util.FirstNonZeroIntegerArg(siteInstance.GetSiteConfig().DynamicSeedingMinSeeders, MIN_SEEDERS)
	var maxSeeders = util.FirstNonZeroIntegerArg(siteInstance.GetSiteConfig().DynamicSeedingMaxSeeders, MAX_SEEDERS)
	var replaceSeeders = util.FirstNonZeroIntegerArg(siteInstance.GetSiteConfig().DynamicSeedingReplaceSeeders,
		MIN_REPLACE_SEEDERS_DIFF)

	// triage for client torrents
	clientTorrentsMap := map[string]*client.Torrent{}
	// Torrents that are excluded from dynamic seeding deciding stragety.
	// These torrents will neither be counted nor be touched.
	var otherTorrents []string
	var invalidTorrents []string // invalid tracker (may be deleted)
	var stalledTorrents []string // incomplete and no activity for enough time, safe to delete.
	var downloadingTorrents []string
	var safeTorrents []string   // has enough seeders, safe to delete
	var normalTorrents []string // seeding is normal
	var protectedTorrents []string
	var unknownTorrents []string
	// Invalid: otherTorrents.
	// Success: downloadingTorrents + protectedTorrents + unknownTorrents; will never be deleted.
	// Fail: invalidTorrents + stalledTorrents + safeTorrents + normalTorrents; could be deleted.
	var statistics = common.NewTorrentsStatistics()
	for _, torrent := range clientTorrents {
		clientTorrentsMap[torrent.InfoHash] = &torrent
		if !torrent.HasTag(dynamicSeedingTag) {
			otherTorrents = append(otherTorrents, torrent.InfoHash)
			continue
		}
		var trackerStatus TrackersStatus
		if torrent.Seeders == 0 && torrent.Leechers == 0 {
			if trackers, err := clientInstance.GetTorrentTrackers(torrent.InfoHash); err != nil {
				trackerStatus = TRACKER_UNKNOWN
			} else if trackers.SeemsInvalidTorrent() {
				trackerStatus = TRACKER_INVALID
			} else {
				trackerStatus = TRACKER_OK
			}
		}
		if !torrent.IsComplete() {
			if trackerStatus == TRACKER_INVALID && timestamp-torrent.Atime > NEW_TORRENT_TIMESPAN {
				invalidTorrents = append(invalidTorrents, torrent.InfoHash)
				statistics.UpdateClientTorrent(common.TORRENT_INVALID, &torrent)
			} else if (timestamp - torrent.ActivityTime) >= INACTIVITY_TIMESPAN {
				stalledTorrents = append(stalledTorrents, torrent.InfoHash)
				statistics.UpdateClientTorrent(common.TORRENT_FAILURE, &torrent)
			} else {
				downloadingTorrents = append(downloadingTorrents, torrent.InfoHash)
				statistics.UpdateClientTorrent(common.TORRENT_SUCCESS, &torrent)
			}
		} else if torrent.State != "seeding" {
			otherTorrents = append(otherTorrents, torrent.InfoHash)
		} else if trackerStatus == TRACKER_INVALID {
			invalidTorrents = append(invalidTorrents, torrent.InfoHash)
			statistics.UpdateClientTorrent(common.TORRENT_INVALID, &torrent)
		} else if torrent.Seeders == 0 {
			unknownTorrents = append(unknownTorrents, torrent.InfoHash)
			statistics.UpdateClientTorrent(common.TORRENT_SUCCESS, &torrent)
		} else if timestamp-torrent.Ctime < MIN_SEEDING_TIME {
			protectedTorrents = append(protectedTorrents, torrent.InfoHash)
			statistics.UpdateClientTorrent(common.TORRENT_SUCCESS, &torrent)
		} else if torrent.Seeders > maxSeeders {
			safeTorrents = append(safeTorrents, torrent.InfoHash)
			statistics.UpdateClientTorrent(common.TORRENT_FAILURE, &torrent)
		} else if torrent.Seeders > minSeeders {
			normalTorrents = append(normalTorrents, torrent.InfoHash)
			statistics.UpdateClientTorrent(common.TORRENT_FAILURE, &torrent)
		} else {
			protectedTorrents = append(protectedTorrents, torrent.InfoHash)
			statistics.UpdateClientTorrent(common.TORRENT_SUCCESS, &torrent)
		}
	}
	result.Log += fmt.Sprintf("Client torrents: others %d / invalid %d / stalled %d / downloading %d / safe %d "+
		"/ normal %d / protected %d / unknown %d\n", len(otherTorrents), len(invalidTorrents), len(stalledTorrents),
		len(downloadingTorrents), len(safeTorrents), len(normalTorrents), len(protectedTorrents), len(unknownTorrents))
	if len(downloadingTorrents) >= MAX_PARALLEL_DOWNLOAD {
		result.Msg = "Already currently downloading enough torrents. Exit"
		return
	}

	availableSlots := MAX_PARALLEL_DOWNLOAD - len(downloadingTorrents)
	availableSpace := siteInstance.GetSiteConfig().DynamicSeedingSizeValue - statistics.SuccessSize
	if !clientStatus.NoDel {
		availableSpace += statistics.FailureSize
	}
	if availableSpace < min(siteInstance.GetSiteConfig().DynamicSeedingSizeValue/10, MIN_SIZE) {
		result.Msg = "Insufficient dynamic seeding storage space in client. Exit"
		return
	}
	dynamicSeedingUrl := siteInstance.GetSiteConfig().DynamicSeedingTorrentsUrl
	if siteInstance.GetSiteConfig().Type == "nexusphp" {
		// See https://github.com/xiaomlove/nexusphp/blob/php8/public/torrents.php .
		dynamicSeedingUrl = util.AppendUrlQueryString(dynamicSeedingUrl, "seeders_begin=1")
	}
	var siteTorrents []site.Torrent
	var siteTorrentsSize int64
	var scannedTorrents int64
	marker := ""
site_outer:
	for {
		torrents, nextPageMarker, err := siteInstance.GetAllTorrents("seeders", false, marker, dynamicSeedingUrl)
		if err != nil {
			result.Log += fmt.Sprintf("failed to get site torrents: %v\n", err)
			break
		}
		rand.Shuffle(len(torrents), func(i, j int) { torrents[i], torrents[j] = torrents[j], torrents[i] })
		slices.SortStableFunc(torrents, func(a, b site.Torrent) int { return int(a.Seeders - b.Seeders) })
		for _, torrent := range torrents {
			if torrent.Id != "" && slices.Contains(ignores, torrent.ID()) {
				log.Debugf("Ignore site torrent %s (%s) which is recently deleted from client", torrent.Name, torrent.Id)
				continue
			}
			if torrent.Seeders < 1 || torrent.IsCurrentActive {
				continue
			}
			if torrent.Seeders >= maxSeeders {
				break site_outer
			}
			scannedTorrents++
			if maxScan > 0 && scannedTorrents > maxScan {
				break site_outer
			}
			if torrent.HasHnR || (torrent.Paid && !torrent.Bought) || torrent.DownloadMultiplier != 0 {
				continue
			}
			if torrent.DiscountEndTime > 0 {
				estimateDownloadTime := torrent.Size / downloadingSpeedLimit / MAX_PARALLEL_DOWNLOAD
				remainFreeTime := torrent.DiscountEndTime - timestamp
				if remainFreeTime <= max(estimateDownloadTime/2, MIN_FREE_REMAINING_TIME) {
					continue
				}
			}
			if torrent.Size > availableSpace-siteTorrentsSize || (timestamp-torrent.Time < MIN_TORRENT_AGE) ||
				siteInstance.GetSiteConfig().DynamicSeedingTorrentMaxSizeValue > 0 &&
					torrent.Size > siteInstance.GetSiteConfig().DynamicSeedingTorrentMaxSizeValue ||
				siteInstance.GetSiteConfig().DynamicSeedingTorrentMinSizeValue > 0 &&
					torrent.Size < siteInstance.GetSiteConfig().DynamicSeedingTorrentMinSizeValue ||
				torrent.MatchFiltersOr(siteInstance.GetSiteConfig().DynamicSeedingExcludes) ||
				torrent.Seeders+torrent.Leechers >= maxSeeders {
				continue
			}
			siteTorrents = append(siteTorrents, torrent)
			siteTorrentsSize += torrent.Size
		}
		if len(siteTorrents) >= availableSlots {
			break
		}
		if nextPageMarker == "" {
			break
		}
		marker = nextPageMarker
	}
	if len(siteTorrents) == 0 {
		result.Msg = "No candidate site dynamic seeding torrents found"
		return
	}
	fmt.Fprintf(os.Stderr, "site candidate torrents:\n")
	site.PrintTorrents(os.Stderr, siteTorrents, "", timestamp, false, false, nil)

	availableSpace = siteInstance.GetSiteConfig().DynamicSeedingSizeValue - statistics.SuccessSize
	for _, torrent := range siteTorrents {
		var deleteTorrents []string
		var log string
		if !clientStatus.NoDel {
			for availableSpace < torrent.Size {
				if len(invalidTorrents) > 0 {
					availableSpace += clientTorrentsMap[invalidTorrents[0]].Size
					log += fmt.Sprintf("Delete client invalid torrent %s\n", clientTorrentsMap[invalidTorrents[0]].Name)
					deleteTorrents = append(deleteTorrents, invalidTorrents[0])
					invalidTorrents = invalidTorrents[1:]
				} else if len(stalledTorrents) > 0 {
					availableSpace += clientTorrentsMap[stalledTorrents[0]].Size
					log += fmt.Sprintf("Delete client stalled torrent %s\n", clientTorrentsMap[stalledTorrents[0]].Name)
					deleteTorrents = append(deleteTorrents, stalledTorrents[0])
					stalledTorrents = stalledTorrents[1:]
				} else if len(safeTorrents) > 0 {
					availableSpace += clientTorrentsMap[safeTorrents[0]].Size
					log += fmt.Sprintf("Delete safe safe torrent %s\n", clientTorrentsMap[safeTorrents[0]].Name)
					deleteTorrents = append(deleteTorrents, safeTorrents[0])
					safeTorrents = safeTorrents[1:]
				} else if len(normalTorrents) > 0 &&
					clientTorrentsMap[normalTorrents[0]].Seeders-torrent.Seeders >= replaceSeeders {
					availableSpace += clientTorrentsMap[normalTorrents[0]].Size
					log += fmt.Sprintf("Delete client normal torrent %s\n", clientTorrentsMap[normalTorrents[0]].Name)
					deleteTorrents = append(deleteTorrents, normalTorrents[0])
					normalTorrents = normalTorrents[1:]
				} else {
					break
				}
			}
		}
		if availableSpace < torrent.Size {
			break
		}
		availableSpace -= torrent.Size
		result.AddTorrents = append(result.AddTorrents, torrent)
		result.Log += log
		result.Log += fmt.Sprintf("Add site torrent %s\n", torrent.Name)
		for _, torrent := range deleteTorrents {
			result.DeleteTorrents = append(result.DeleteTorrents, *clientTorrentsMap[torrent])
		}
	}

	return
}
