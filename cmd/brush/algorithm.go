package brush

import (
	"fmt"
	"math"
	"sort"

	log "github.com/sirupsen/logrus"

	"github.com/sagan/ptool/client"
	"github.com/sagan/ptool/site"
	"github.com/sagan/ptool/utils"
)

const (
	STALL_DOWNLOAD_SPEED = int64(10 * 1024)
)

type BrushOptionStruct struct {
	MinDiskSpace            int64
	SlowUploadSpeedTier     int64
	TorrentUploadSpeedLimit int64
	MaxDownloadingTorrents  int64
	MaxTorrents             int64
	MinRatio                float64
	DefaultUploadSpeedLimit int64
	Now                     int64
}

type AlgorithmAddTorrent struct {
	DownloadUrl string
	Name        string
	Meta        map[string](int64)
	Msg         string
}

type AlgorithmModifyTorrent struct {
	InfoHash string
	Name     string
	Meta     map[string](int64)
	Msg      string
}

type AlgorithmOperationTorrent struct {
	InfoHash string
	Name     string
	Msg      string
}

type AlgorithmResult struct {
	AddTorrents    []AlgorithmAddTorrent       // new torrents that will be added to client
	ModifyTorrents []AlgorithmModifyTorrent    // modify meta info of these torrents
	StallTorrents  []AlgorithmModifyTorrent    // torrents that will stop downloading but still uploading
	DeleteTorrents []AlgorithmOperationTorrent // torrents that will be removed from client
	CanAddMore     bool                        // client is able to add more torrents
	Msg            string
}

type candidateTorrentStruct struct {
	Name                  string
	DownloadUrl           string
	Size                  int64
	PredictionUploadSpeed int64
	Score                 float64
	Meta                  map[string](int64)
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
	DeleteCandidateFlag bool
	DeleteFlag          bool
}

func isDownloading(torrent *client.Torrent) bool {
	return notStalled(torrent) && torrent.DownloadSpeed >= STALL_DOWNLOAD_SPEED
}

func notStalled(torrent *client.Torrent) bool {
	return torrent.State == "downloading" && torrent.Meta["stt"] == 0
}

/*
 * Strategy
 * Delete a torrent from client ONLY when it's uploading speed become SLOW enough AND free disk space insufficient.
 * Stall ALL torrent of client (limit download speed to 1B/s, so upload only) when free disk space insufficient
 * Add new torrents to client when server uploading bandwidth is somewhat idle AND there is SOME free disk space.
 *
 */
func Decide(clientStatus *client.Status, clientTorrents []client.Torrent, siteTorrents []site.Torrent, option *BrushOptionStruct) (result *AlgorithmResult) {
	result = &AlgorithmResult{}

	cntTorrents := int64(len(clientTorrents))
	cntDownloadingTorrents := int64(0)
	freespace := clientStatus.FreeSpaceOnDisk
	estimateUploadSpeed := clientStatus.UploadSpeed

	var candidateTorrents []candidateTorrentStruct
	var modifyTorrents []AlgorithmModifyTorrent
	var stallTorrents []AlgorithmModifyTorrent
	var deleteCandidateTorrents []candidateClientTorrentStruct
	clientTorrentsMap := make(map[string](*clientTorrentInfoStruct))
	siteTorrentsMap := make(map[string](*site.Torrent))

	targetUploadSpeed := clientStatus.UploadSpeedLimit
	if targetUploadSpeed <= 0 {
		targetUploadSpeed = option.DefaultUploadSpeedLimit
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
		score, predictionUploadSpeed := rateSiteTorrent(&siteTorrent, option)
		if score > 0 {
			candidateTorrent := candidateTorrentStruct{
				Name:                  siteTorrent.Name,
				Size:                  siteTorrent.Size,
				DownloadUrl:           siteTorrent.DownloadUrl,
				PredictionUploadSpeed: predictionUploadSpeed,
				Score:                 score,
				Meta:                  make(map[string]int64),
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
		if isDownloading(&torrent) {
			cntDownloadingTorrents++
		}

		// mark torrents that discount time ends as stall
		if torrent.Meta["dcet"] > 0 && torrent.Meta["dcet"]-option.Now <= 3600 && torrent.Ctime <= 0 {
			if notStalled(&torrent) {
				meta := utils.CopyMap(torrent.Meta)
				meta["stt"] = option.Now
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
		if option.Now-torrent.Atime <= 15*60 {
			continue
		}

		// check slow torrents, add it to watch list first time and mark as deleteCandidate second time
		if torrent.UploadSpeed < option.SlowUploadSpeedTier {
			if torrent.Meta["sct"] > 0 { // second encounter on slow torrent
				if option.Now-torrent.Meta["sct"] >= 15*60 {
					averageUploadSpeedSinceSct := (torrent.Uploaded - torrent.Meta["sctu"]) / (option.Now - torrent.Meta["sct"])
					if averageUploadSpeedSinceSct < option.SlowUploadSpeedTier {
						if notStalled(&torrent) &&
							torrent.DownloadSpeed >= 100*1024 &&
							float64(torrent.UploadSpeed)/float64(torrent.DownloadSpeed) < option.MinRatio &&
							option.Now-torrent.Atime >= 30*60 {
							meta := utils.CopyMap(torrent.Meta)
							meta["stt"] = option.Now
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
								score += float64(option.Now) - float64(torrent.Meta["stt"])
							}
						} else {
							score += math.Min(float64(option.Now-torrent.Ctime), 86400)
						}
						deleteCandidateTorrents = append(deleteCandidateTorrents, candidateClientTorrentStruct{
							InfoHash:    torrent.InfoHash,
							Score:       score,
							FutureValue: torrent.UploadSpeed,
							Msg:         "slow uploading speed",
						})
						clientTorrentsMap[torrent.InfoHash].DeleteCandidateFlag = true
					} else {
						meta := utils.CopyMap(torrent.Meta)
						meta["sct"] = option.Now
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
				meta := utils.CopyMap(torrent.Meta)
				meta["sct"] = option.Now
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
			meta := utils.CopyMap(torrent.Meta)
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
		if (torrent.Ctime > 0 || torrent.Meta["stt"] == 0 || option.Now-torrent.Meta["stt"] < 30*60) && freespace >= option.MinDiskSpace {
			continue
		}
		result.DeleteTorrents = append(result.DeleteTorrents, AlgorithmOperationTorrent{
			InfoHash: torrent.InfoHash,
			Name:     torrent.Name,
			Msg:      deleteTorrent.Msg,
		})
		freespace += torrent.SizeCompleted
		estimateUploadSpeed -= torrent.UploadSpeed
		clientTorrentsMap[torrent.InfoHash].DeleteFlag = true
		if isDownloading(torrent) {
			cntDownloadingTorrents--
		}
		cntTorrents--
	}

	// if still not enough free space, delete ALL stalled incomplete torrents
	if freespace < option.MinDiskSpace {
		for _, torrent := range clientTorrents {
			if clientTorrentsMap[torrent.InfoHash].DeleteFlag || torrent.Ctime > 0 || torrent.Meta["stt"] == 0 {
				continue
			}
			result.DeleteTorrents = append(result.DeleteTorrents, AlgorithmOperationTorrent{
				InfoHash: torrent.InfoHash,
				Name:     torrent.Name,
				Msg:      "delete stalled incomplete torrents due to insufficient disk space",
			})
			freespace += torrent.SizeCompleted
			estimateUploadSpeed -= torrent.UploadSpeed
			clientTorrentsMap[torrent.InfoHash].DeleteFlag = true
			if isDownloading(&torrent) {
				cntDownloadingTorrents--
			}
			cntTorrents--
		}
	}

	// delete torrents due to max brush torrents limit
	if cntTorrents > option.MaxTorrents && len(candidateTorrents) > 0 {
		cntDeleteDueToMaxTorrents := cntTorrents - option.MaxTorrents
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
			freespace += torrent.SizeCompleted
			estimateUploadSpeed -= torrent.UploadSpeed
			clientTorrentsMap[torrent.InfoHash].DeleteFlag = true
			if isDownloading(torrent) {
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
	if freespace < option.MinDiskSpace {
		for _, torrent := range clientTorrents {
			if clientTorrentsMap[torrent.InfoHash].DeleteFlag || clientTorrentsMap[torrent.InfoHash].StallFlag {
				continue
			}
			if notStalled(&torrent) {
				meta := utils.CopyMap(torrent.Meta)
				meta["stt"] = option.Now
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

	// stall torrents
	for _, stallTorrent := range stallTorrents {
		if clientTorrentsMap[stallTorrent.InfoHash].DeleteFlag {
			continue
		}
		result.StallTorrents = append(result.StallTorrents, stallTorrent)
		if isDownloading(clientTorrentsMap[stallTorrent.InfoHash].Torrent) {
			cntDownloadingTorrents--
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
	if freespace >= option.MinDiskSpace && cntTorrents <= option.MaxTorrents {
		for cntDownloadingTorrents < option.MaxDownloadingTorrents && estimateUploadSpeed <= targetUploadSpeed*2 && len(candidateTorrents) > 0 {
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

	if cntTorrents <= option.MaxTorrents && cntDownloadingTorrents < option.MaxDownloadingTorrents && estimateUploadSpeed <= targetUploadSpeed*2 {
		result.CanAddMore = true
	}

	return
}

func rateSiteTorrent(siteTorrent *site.Torrent, brushOption *BrushOptionStruct) (score float64, predictionUploadSpeed int64) {
	if log.GetLevel() >= log.TraceLevel {
		defer func() {
			log.Tracef("rateSiteTorrent score=%0.0f name=%s, free=%t, rtime=%d, seeders=%d, leechers=%d",
				score,
				siteTorrent.Name,
				siteTorrent.DownloadMultiplier == 0,
				brushOption.Now-siteTorrent.Time,
				siteTorrent.Seeders,
				siteTorrent.Leechers,
			)
		}()
	}
	if siteTorrent.IsActive ||
		siteTorrent.HasHnR ||
		siteTorrent.DownloadMultiplier != 0 ||
		(siteTorrent.DiscountEndTime > 0 && siteTorrent.DiscountEndTime-brushOption.Now < 3600) ||
		siteTorrent.Seeders == 0 ||
		siteTorrent.Leechers <= siteTorrent.Seeders {
		score = 0
		return
	}
	if brushOption.Now-siteTorrent.Time >= 86400 {
		score = 0
		return
	} else if brushOption.Now-siteTorrent.Time >= 7200 {
		if siteTorrent.Leechers < 500 {
			score = 0
			return
		}
	}

	predictionUploadSpeed = siteTorrent.Leechers * 100 * 1024
	if predictionUploadSpeed > brushOption.TorrentUploadSpeedLimit {
		predictionUploadSpeed = brushOption.TorrentUploadSpeedLimit
	}

	if siteTorrent.Seeders == 1 {
		score = 50
	} else if siteTorrent.Seeders <= 3 {
		score = 30
	} else {
		score = 10
	}
	score += float64(siteTorrent.Leechers)

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
