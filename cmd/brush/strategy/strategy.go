package strategy

import (
	"fmt"
	"math"
	"sort"

	log "github.com/sirupsen/logrus"

	"github.com/sagan/ptool/client"
	"github.com/sagan/ptool/site"
	"github.com/sagan/ptool/util"
)

const (
	// new torrents timespan during which will NOT be examined at all
	NEW_TORRENTS_TIMESPAN = int64(15 * 60)
	// new torrents timespan during which will NOT be stalled
	NEW_TORRENTS_STALL_EXEMPTION_TIMESPAN = int64(30 * 60)
	NO_PROCESS_TORRENT_DELETEION_TIMESPAN = int64(30 * 60)
	STALL_DOWNLOAD_SPEED                  = int64(10 * 1024)
	SLOW_UPLOAD_SPEED                     = int64(100 * 1024)
	RATIO_CHECK_MIN_DOWNLOAD_SPEED        = int64(100 * 1024)
	SLOW_TORRENTS_CHECK_TIMESPAN          = int64(15 * 60)
	// stalled torrent will be deleted after this time passed
	STALL_TORRENT_DELETEION_TIMESPAN     = int64(30 * 60)
	BANDWIDTH_FULL_PERCENT               = float64(0.8)
	DELETE_TORRENT_IMMEDIATELY_SCORE     = float64(99999)
	RESUME_TORRENTS_FREE_DISK_SPACE_TIER = int64(5 * 1024 * 1024 * 1024)  // 5GB
	DELETE_TORRENTS_FREE_DISK_SPACE_TIER = int64(10 * 1024 * 1024 * 1024) // 10GB
)

type BrushSiteOptionStruct struct {
	AllowNoneFree           bool
	AllowPaid               bool
	AllowHr                 bool
	AllowZeroSeeders        bool
	TorrentUploadSpeedLimit int64
	TorrentMinSizeLimit     int64
	TorrentMaxSizeLimit     int64
	Now                     int64
	Excludes                []string
}

type BrushClientOptionStruct struct {
	MinDiskSpace            int64
	SlowUploadSpeedTier     int64
	MaxDownloadingTorrents  int64
	MaxTorrents             int64
	MinRatio                float64
	DefaultUploadSpeedLimit int64
}

type AlgorithmAddTorrent struct {
	DownloadUrl string
	Name        string
	Meta        map[string]int64
	Msg         string
}

type AlgorithmModifyTorrent struct {
	InfoHash string
	Name     string
	Meta     map[string]int64
	Msg      string
}

type AlgorithmOperationTorrent struct {
	InfoHash string
	Name     string
	Msg      string
}

type AlgorithmResult struct {
	DeleteTorrents  []AlgorithmOperationTorrent // torrents that will be removed from client
	StallTorrents   []AlgorithmModifyTorrent    // torrents that will stop downloading but still uploading
	ResumeTorrents  []AlgorithmOperationTorrent // resume paused / errored torrents
	ModifyTorrents  []AlgorithmModifyTorrent    // modify meta info of these torrents
	AddTorrents     []AlgorithmAddTorrent       // new torrents that will be added to client
	CanAddMore      bool                        // client is able to add more torrents
	FreeSpaceChange int64                       // estimated free space change after apply above operations
	Msg             string
}

type candidateTorrentStruct struct {
	Name                  string
	DownloadUrl           string
	Size                  int64
	PredictionUploadSpeed int64
	Score                 float64
	Meta                  map[string]int64
}

type candidateClientTorrentStruct struct {
	InfoHash    string
	Score       float64
	FutureValue int64 // 预期的该种子未来的刷流上传价值
	Msg         string
}

type clientTorrentInfoStruct struct {
	Torrent             *client.Torrent
	ModifyFlag          bool
	StallFlag           bool
	ResumeFlag          bool
	DeleteCandidateFlag bool
	DeleteFlag          bool
}

func countAsDownloading(torrent *client.Torrent, now int64) bool {
	return !torrent.IsComplete() && torrent.Meta["stt"] == 0 &&
		(torrent.DownloadSpeed >= STALL_DOWNLOAD_SPEED || now-torrent.Atime <= NEW_TORRENTS_TIMESPAN)
}

func canStallTorrent(torrent *client.Torrent) bool {
	return torrent.State == "downloading" && torrent.Meta["stt"] == 0
}

func isTorrentStalled(torrent *client.Torrent) bool {
	return !torrent.IsComplete() && torrent.Meta["stt"] > 0
}

/*
 * @todo : this function requires a major rework. It's a mess right now.
 *
 * Strategy (Desired)
 * Delete a torrent from client when (any of the the follow criterion matches):
 *   a. Tt's uploading speed become SLOW enough AND free disk space insufficient
 *   b. It's consuming too much downloading bandwidth and uploading / downloading speed ratio is too low
 *   c. It's incomplete and been totally stalled (no uploading or downloading activity) for some time
 *   d. It's incomplete and the free discount expired (or will soon expire)
 * Stall ALL incomplete torrent of client (limit download speed to 1B/s, so upload only)
 *   when free disk space insufficient
 *   * This's somwwhat broken in qBittorrent for now (See https://github.com/qbittorrent/qBittorrent/issues/2185 ).
 *   * Simply limiting downloading speed (to a very low tier) will also drop uploading speed to the same level
 *   * Consider removing this behavior
 * Add new torrents to client when server uploading and downloading bandwidth is somewhat idle AND
 *   there is SOME free disk space
 * Also：
 *   * Use the current seeders / leechers info of torrent when make decisions
 */
func Decide(clientStatus *client.Status, clientTorrents []client.Torrent, siteTorrents []site.Torrent,
	siteOption *BrushSiteOptionStruct, clientOption *BrushClientOptionStruct) (result *AlgorithmResult) {
	result = &AlgorithmResult{}

	cntTorrents := int64(len(clientTorrents))
	cntDownloadingTorrents := int64(0)
	freespace := clientStatus.FreeSpaceOnDisk
	freespaceChange := int64(0)
	freespaceTarget := util.Min(clientOption.MinDiskSpace*2,
		clientOption.MinDiskSpace+DELETE_TORRENTS_FREE_DISK_SPACE_TIER)
	estimateUploadSpeed := clientStatus.UploadSpeed

	var candidateTorrents []candidateTorrentStruct
	var modifyTorrents []AlgorithmModifyTorrent
	var stallTorrents []AlgorithmModifyTorrent
	var resumeTorrents []AlgorithmOperationTorrent
	var deleteCandidateTorrents []candidateClientTorrentStruct
	clientTorrentsMap := map[string]*clientTorrentInfoStruct{}
	siteTorrentsMap := map[string]*site.Torrent{}

	targetUploadSpeed := clientStatus.UploadSpeedLimit
	if targetUploadSpeed <= 0 {
		targetUploadSpeed = clientOption.DefaultUploadSpeedLimit
	}

	for i, torrent := range clientTorrents {
		clientTorrentsMap[torrent.InfoHash] = &clientTorrentInfoStruct{
			Torrent: &clientTorrents[i],
		}
	}
	for i, siteTorrent := range siteTorrents {
		siteTorrentsMap[siteTorrent.InfoHash] = &siteTorrents[i]
	}

	for _, siteTorrent := range siteTorrents {
		score, predictionUploadSpeed, _ := RateSiteTorrent(&siteTorrent, siteOption)
		if score > 0 {
			candidateTorrent := candidateTorrentStruct{
				Name:                  siteTorrent.Name,
				Size:                  siteTorrent.Size,
				DownloadUrl:           siteTorrent.DownloadUrl,
				PredictionUploadSpeed: predictionUploadSpeed,
				Score:                 score,
				Meta:                  map[string]int64{},
			}
			if siteTorrent.DiscountEndTime > 0 {
				candidateTorrent.Meta["dcet"] = siteTorrent.DiscountEndTime
			}
			candidateTorrents = append(candidateTorrents, candidateTorrent)
		}
	}
	sort.SliceStable(candidateTorrents, func(i, j int) bool {
		return candidateTorrents[i].Score > candidateTorrents[j].Score
	})

	// mark torrents
	for _, torrent := range clientTorrents {
		if countAsDownloading(&torrent, siteOption.Now) {
			cntDownloadingTorrents++
		}

		// mark torrents that discount time ends as stall
		if torrent.Meta["dcet"] > 0 && torrent.Meta["dcet"]-siteOption.Now <= 3600 && torrent.Ctime <= 0 {
			if canStallTorrent(&torrent) {
				meta := util.CopyMap(torrent.Meta)
				meta["stt"] = siteOption.Now
				stallTorrents = append(stallTorrents, AlgorithmModifyTorrent{
					InfoHash: torrent.InfoHash,
					Name:     torrent.Name,
					Msg:      "discount time ends",
					Meta:     meta,
				})
				clientTorrentsMap[torrent.InfoHash].StallFlag = true
			}
		}

		// skip new added torrents
		if siteOption.Now-torrent.Atime <= NEW_TORRENTS_TIMESPAN {
			continue
		}

		if torrent.State == "error" && (torrent.UploadSpeed < clientOption.SlowUploadSpeedTier ||
			torrent.UploadSpeed < clientOption.SlowUploadSpeedTier*2 && freespace == 0) &&
			len(candidateTorrents) > 0 {
			deleteCandidateTorrents = append(deleteCandidateTorrents, candidateClientTorrentStruct{
				InfoHash:    torrent.InfoHash,
				Score:       DELETE_TORRENT_IMMEDIATELY_SCORE,
				FutureValue: 0,
				Msg:         "torrent in error state",
			})
			clientTorrentsMap[torrent.InfoHash].DeleteCandidateFlag = true
		} else if torrent.DownloadSpeed == 0 && torrent.SizeCompleted == 0 {
			if siteOption.Now-torrent.Atime > NO_PROCESS_TORRENT_DELETEION_TIMESPAN {
				deleteCandidateTorrents = append(deleteCandidateTorrents, candidateClientTorrentStruct{
					InfoHash:    torrent.InfoHash,
					Score:       DELETE_TORRENT_IMMEDIATELY_SCORE,
					FutureValue: 0,
					Msg:         "torrent has no download proccess",
				})
				clientTorrentsMap[torrent.InfoHash].DeleteCandidateFlag = true
			}
		} else if torrent.UploadSpeed < clientOption.SlowUploadSpeedTier {
			// check slow torrents, add it to watch list first time and mark as deleteCandidate second time
			if torrent.Meta["sct"] > 0 { // second encounter on slow torrent
				if siteOption.Now-torrent.Meta["sct"] >= SLOW_TORRENTS_CHECK_TIMESPAN {
					averageUploadSpeedSinceSct := (torrent.Uploaded - torrent.Meta["sctu"]) /
						(siteOption.Now - torrent.Meta["sct"])
					if averageUploadSpeedSinceSct < clientOption.SlowUploadSpeedTier {
						if canStallTorrent(&torrent) &&
							torrent.DownloadSpeed >= RATIO_CHECK_MIN_DOWNLOAD_SPEED &&
							float64(torrent.UploadSpeed)/float64(torrent.DownloadSpeed) < clientOption.MinRatio &&
							siteOption.Now-torrent.Atime >= NEW_TORRENTS_STALL_EXEMPTION_TIMESPAN {
							meta := util.CopyMap(torrent.Meta)
							meta["stt"] = siteOption.Now
							stallTorrents = append(stallTorrents, AlgorithmModifyTorrent{
								InfoHash: torrent.InfoHash,
								Name:     torrent.Name,
								Msg:      "low upload / download ratio",
								Meta:     meta,
							})
							clientTorrentsMap[torrent.InfoHash].StallFlag = true
						}
						score := -float64(torrent.UploadSpeed)
						if torrent.Ctime <= 0 {
							if torrent.Meta["stt"] > 0 {
								score += float64(siteOption.Now) - float64(torrent.Meta["stt"])
							}
						} else {
							score += math.Min(float64(siteOption.Now-torrent.Ctime), 86400)
						}
						deleteCandidateTorrents = append(deleteCandidateTorrents, candidateClientTorrentStruct{
							InfoHash:    torrent.InfoHash,
							Score:       score,
							FutureValue: torrent.UploadSpeed,
							Msg:         "slow uploading speed",
						})
						clientTorrentsMap[torrent.InfoHash].DeleteCandidateFlag = true
					} else {
						meta := util.CopyMap(torrent.Meta)
						meta["sct"] = siteOption.Now
						meta["sctu"] = torrent.Uploaded
						modifyTorrents = append(modifyTorrents, AlgorithmModifyTorrent{
							InfoHash: torrent.InfoHash,
							Name:     torrent.Name,
							Msg:      "reset slow check time mark",
							Meta:     meta,
						})
						clientTorrentsMap[torrent.InfoHash].ModifyFlag = true
					}
				}
			} else { // first encounter on slow torrent
				meta := util.CopyMap(torrent.Meta)
				meta["sct"] = siteOption.Now
				meta["sctu"] = torrent.Uploaded
				modifyTorrents = append(modifyTorrents, AlgorithmModifyTorrent{
					InfoHash: torrent.InfoHash,
					Name:     torrent.Name,
					Msg:      "set slow check time mark",
					Meta:     meta,
				})
				clientTorrentsMap[torrent.InfoHash].ModifyFlag = true
			}
		} else if torrent.Meta["sct"] > 0 { // remove mark on no-longer slow torrents
			meta := util.CopyMap(torrent.Meta)
			delete(meta, "sct")
			delete(meta, "sctu")
			modifyTorrents = append(modifyTorrents, AlgorithmModifyTorrent{
				InfoHash: torrent.InfoHash,
				Name:     torrent.Name,
				Msg:      "remove slow check time mark",
				Meta:     meta,
			})
			clientTorrentsMap[torrent.InfoHash].ModifyFlag = true
		}
	}
	sort.SliceStable(deleteCandidateTorrents, func(i, j int) bool {
		return deleteCandidateTorrents[i].Score > deleteCandidateTorrents[j].Score
	})

	// @todo: use Dynamic Programming to better find torrents suitable for delete
	// delete torrents
	for _, deleteTorrent := range deleteCandidateTorrents {
		torrent := clientTorrentsMap[deleteTorrent.InfoHash].Torrent
		shouldDelete := false
		if deleteTorrent.Score >= DELETE_TORRENT_IMMEDIATELY_SCORE ||
			(freespace >= 0 && freespace <= clientOption.MinDiskSpace && freespace+freespaceChange <= freespaceTarget) {
			shouldDelete = true
		} else if torrent.Ctime <= 0 &&
			torrent.Meta["stt"] > 0 &&
			siteOption.Now-torrent.Meta["stt"] >= STALL_TORRENT_DELETEION_TIMESPAN {
			shouldDelete = true
		}
		if !shouldDelete {
			continue
		}
		result.DeleteTorrents = append(result.DeleteTorrents, AlgorithmOperationTorrent{
			InfoHash: torrent.InfoHash,
			Name:     torrent.Name,
			Msg:      deleteTorrent.Msg,
		})
		freespaceChange += torrent.SizeCompleted
		estimateUploadSpeed -= torrent.UploadSpeed
		clientTorrentsMap[torrent.InfoHash].DeleteFlag = true
		if countAsDownloading(torrent, siteOption.Now) {
			cntDownloadingTorrents--
		}
		cntTorrents--
	}

	// if still not enough free space, delete ALL stalled incomplete torrents
	if freespace >= 0 && freespace <= clientOption.MinDiskSpace && freespace+freespaceChange <= freespaceTarget {
		for _, torrent := range clientTorrents {
			if clientTorrentsMap[torrent.InfoHash].DeleteFlag || !isTorrentStalled(&torrent) {
				continue
			}
			result.DeleteTorrents = append(result.DeleteTorrents, AlgorithmOperationTorrent{
				InfoHash: torrent.InfoHash,
				Name:     torrent.Name,
				Msg:      "delete stalled incomplete torrents due to insufficient disk space",
			})
			freespaceChange += torrent.SizeCompleted
			estimateUploadSpeed -= torrent.UploadSpeed
			clientTorrentsMap[torrent.InfoHash].DeleteFlag = true
			if countAsDownloading(&torrent, siteOption.Now) {
				cntDownloadingTorrents--
			}
			cntTorrents--
		}
	}

	// delete torrents due to max brush torrents limit
	if cntTorrents > clientOption.MaxTorrents && len(candidateTorrents) > 0 {
		cntDeleteDueToMaxTorrents := cntTorrents - clientOption.MaxTorrents
		if cntDeleteDueToMaxTorrents > int64(len(candidateTorrents)) {
			cntDeleteDueToMaxTorrents = int64(len(candidateTorrents))
		}
		for _, deleteTorrent := range deleteCandidateTorrents {
			torrent := clientTorrentsMap[deleteTorrent.InfoHash].Torrent
			if clientTorrentsMap[torrent.InfoHash].DeleteFlag {
				continue
			}
			result.DeleteTorrents = append(result.DeleteTorrents, AlgorithmOperationTorrent{
				InfoHash: torrent.InfoHash,
				Name:     torrent.Name,
				Msg:      deleteTorrent.Msg + " (delete due to max torrents limit)",
			})
			freespaceChange += torrent.SizeCompleted
			estimateUploadSpeed -= torrent.UploadSpeed
			clientTorrentsMap[torrent.InfoHash].DeleteFlag = true
			if countAsDownloading(torrent, siteOption.Now) {
				cntDownloadingTorrents--
			}
			cntTorrents--
			cntDeleteDueToMaxTorrents--
			if cntDeleteDueToMaxTorrents == 0 {
				break
			}
		}
	}

	// if still not enough free space, mark ALL torrents as stall
	if freespace >= 0 && freespace+freespaceChange < clientOption.MinDiskSpace {
		for _, torrent := range clientTorrents {
			if clientTorrentsMap[torrent.InfoHash].DeleteFlag || clientTorrentsMap[torrent.InfoHash].StallFlag {
				continue
			}
			if canStallTorrent(&torrent) {
				meta := util.CopyMap(torrent.Meta)
				meta["stt"] = siteOption.Now
				stallTorrents = append(stallTorrents, AlgorithmModifyTorrent{
					InfoHash: torrent.InfoHash,
					Name:     torrent.Name,
					Msg:      "stall all torrents due to insufficient free disk space",
					Meta:     meta,
				})
				clientTorrentsMap[torrent.InfoHash].StallFlag = true
			}
		}
	}

	// mark torrents as resume
	if freespace+freespaceChange >= util.Max(clientOption.MinDiskSpace, RESUME_TORRENTS_FREE_DISK_SPACE_TIER) {
		for _, torrent := range clientTorrents {
			if torrent.State != "error" || torrent.UploadSpeed < clientOption.SlowUploadSpeedTier*4 ||
				isTorrentStalled(&torrent) || clientTorrentsMap[torrent.InfoHash].ResumeFlag {
				continue
			}
			resumeTorrents = append(resumeTorrents, AlgorithmOperationTorrent{
				InfoHash: torrent.InfoHash,
				Name:     torrent.Name,
				Msg:      "resume fast uploading errored torrent",
			})
			clientTorrentsMap[torrent.InfoHash].ResumeFlag = true
		}
	}

	// stall torrents
	for _, stallTorrent := range stallTorrents {
		if clientTorrentsMap[stallTorrent.InfoHash].DeleteFlag {
			continue
		}
		result.StallTorrents = append(result.StallTorrents, stallTorrent)
		if countAsDownloading(clientTorrentsMap[stallTorrent.InfoHash].Torrent, siteOption.Now) {
			cntDownloadingTorrents--
		}
	}

	// resume torrents
	for _, resumeTorrent := range resumeTorrents {
		if clientTorrentsMap[resumeTorrent.InfoHash].DeleteFlag ||
			clientTorrentsMap[resumeTorrent.InfoHash].StallFlag {
			continue
		}
		result.ResumeTorrents = append(result.ResumeTorrents, resumeTorrent)
		if !countAsDownloading(clientTorrentsMap[resumeTorrent.InfoHash].Torrent, siteOption.Now) {
			cntDownloadingTorrents++
		}
	}

	// modify torrents
	for _, modifyTorrent := range modifyTorrents {
		if clientTorrentsMap[modifyTorrent.InfoHash].DeleteFlag || clientTorrentsMap[modifyTorrent.InfoHash].StallFlag {
			continue
		}
		result.ModifyTorrents = append(result.ModifyTorrents, modifyTorrent)
	}

	// add new torrents
	if (freespace == -1 || freespace+freespaceChange > clientOption.MinDiskSpace) &&
		cntTorrents <= clientOption.MaxTorrents {
		for cntDownloadingTorrents < clientOption.MaxDownloadingTorrents &&
			estimateUploadSpeed <= targetUploadSpeed*2 && len(candidateTorrents) > 0 {
			candidateTorrent := candidateTorrents[0]
			candidateTorrents = candidateTorrents[1:]
			result.AddTorrents = append(result.AddTorrents, AlgorithmAddTorrent{
				DownloadUrl: candidateTorrent.DownloadUrl,
				Name:        candidateTorrent.Name,
				Meta:        candidateTorrent.Meta,
				Msg:         fmt.Sprintf("new torrrent of score %.0f", candidateTorrent.Score),
			})
			cntTorrents++
			cntDownloadingTorrents++
			estimateUploadSpeed += candidateTorrent.PredictionUploadSpeed
		}
	}

	result.FreeSpaceChange = freespaceChange

	if cntTorrents <= clientOption.MaxTorrents &&
		cntDownloadingTorrents < clientOption.MaxDownloadingTorrents &&
		estimateUploadSpeed <= targetUploadSpeed*2 &&
		(freespace == -1 || freespace+freespaceChange > clientOption.MinDiskSpace) {
		result.CanAddMore = true
	}

	return
}

func RateSiteTorrent(siteTorrent *site.Torrent, siteOption *BrushSiteOptionStruct) (
	score float64, predictionUploadSpeed int64, note string) {
	if log.GetLevel() >= log.TraceLevel {
		defer func() {
			log.Tracef("rateSiteTorrent score=%0.0f name=%s, free=%t, rtime=%d, seeders=%d, leechers=%d, note=%s",
				score,
				siteTorrent.Name,
				siteTorrent.DownloadMultiplier == 0,
				siteOption.Now-siteTorrent.Time,
				siteTorrent.Seeders,
				siteTorrent.Leechers,
				note,
			)
		}()
	}
	if siteTorrent.IsActive || siteTorrent.UploadMultiplier == 0 ||
		(!siteOption.AllowHr && siteTorrent.HasHnR) ||
		(!siteOption.AllowNoneFree && siteTorrent.DownloadMultiplier != 0) ||
		(!siteOption.AllowPaid && siteTorrent.Paid && !siteTorrent.Bought) ||
		siteTorrent.Size < siteOption.TorrentMinSizeLimit ||
		siteTorrent.Size > siteOption.TorrentMaxSizeLimit ||
		(siteTorrent.DiscountEndTime > 0 && siteTorrent.DiscountEndTime-siteOption.Now < 3600) ||
		(!siteOption.AllowZeroSeeders && siteTorrent.Seeders == 0) ||
		siteTorrent.Leechers <= siteTorrent.Seeders {
		score = 0
		return
	}
	if siteTorrent.MatchFiltersOr(siteOption.Excludes) {
		score = 0
		note = "brush excludes matches"
		return
	}
	// 部分站点定期将旧种重新置顶免费。这类种子仍然可以获得很好的上传速度。
	if siteOption.Now-siteTorrent.Time <= 86400*30 {
		if siteOption.Now-siteTorrent.Time >= 86400 {
			score = 0
			return
		} else if siteOption.Now-siteTorrent.Time >= 7200 {
			if siteTorrent.Leechers < 500 {
				score = 0
				return
			}
		}
	}

	predictionUploadSpeed = siteTorrent.Leechers * 100 * 1024
	if predictionUploadSpeed > siteOption.TorrentUploadSpeedLimit {
		predictionUploadSpeed = siteOption.TorrentUploadSpeedLimit
	}

	if siteTorrent.Seeders <= 1 {
		score = 50
	} else if siteTorrent.Seeders <= 3 {
		score = 30
	} else {
		score = 10
	}
	score += float64(siteTorrent.Leechers)

	score *= siteTorrent.UploadMultiplier
	if siteTorrent.DownloadMultiplier != 0 {
		score *= 0.5
	}

	if siteTorrent.Size <= 1024*1024*1024 {
		score *= 10
	} else if siteTorrent.Size <= 1024*1024*1024*10 {
		score *= 2
	} else if siteTorrent.Size <= 1024*1024*1024*20 {
		score *= 1
	} else if siteTorrent.Size <= 1024*1024*1024*50 {
		score *= 0.5
	} else if siteTorrent.Size <= 1024*1024*1024*100 {
		score *= 0.1
	} else {
		// 大包特殊处理
		if siteTorrent.Leechers >= 1000 {
			score *= 100
		} else if siteTorrent.Leechers >= 500 {
			score *= 50
		} else if siteTorrent.Leechers >= 100 {
			score *= 10
		} else {
			score *= 0
		}
	}
	return
}

func GetBrushSiteOptions(siteInstance site.Site, ts int64) *BrushSiteOptionStruct {
	return &BrushSiteOptionStruct{
		TorrentMinSizeLimit:     siteInstance.GetSiteConfig().BrushTorrentMinSizeLimitValue,
		TorrentMaxSizeLimit:     siteInstance.GetSiteConfig().BrushTorrentMaxSizeLimitValue,
		TorrentUploadSpeedLimit: siteInstance.GetSiteConfig().TorrentUploadSpeedLimitValue,
		AllowNoneFree:           siteInstance.GetSiteConfig().BrushAllowNoneFree,
		AllowPaid:               siteInstance.GetSiteConfig().BrushAllowPaid,
		AllowHr:                 siteInstance.GetSiteConfig().BrushAllowHr,
		AllowZeroSeeders:        siteInstance.GetSiteConfig().BrushAllowZeroSeeders,
		Excludes:                siteInstance.GetSiteConfig().BrushExcludes,
		Now:                     ts,
	}
}

func GetBrushClientOptions(clientInstance client.Client) *BrushClientOptionStruct {
	return &BrushClientOptionStruct{
		MinDiskSpace:            clientInstance.GetClientConfig().BrushMinDiskSpaceValue,
		SlowUploadSpeedTier:     clientInstance.GetClientConfig().BrushSlowUploadSpeedTierValue,
		MaxDownloadingTorrents:  clientInstance.GetClientConfig().BrushMaxDownloadingTorrents,
		MaxTorrents:             clientInstance.GetClientConfig().BrushMaxTorrents,
		MinRatio:                clientInstance.GetClientConfig().BrushMinRatio,
		DefaultUploadSpeedLimit: clientInstance.GetClientConfig().BrushDefaultUploadSpeedLimitValue,
	}
}
