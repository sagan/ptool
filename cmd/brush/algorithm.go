package brush

import (
	"fmt"
	"math"
	"sort"

	"github.com/sagan/ptool/client"
	"github.com/sagan/ptool/site"
	"github.com/sagan/ptool/utils"
)

type BrushOptionStruct struct {
	MinDiskSpace            int64
	SlowUploadSpeedTier     int64
	TorrentUploadSpeedLimit int64
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
	StallTorrents  []AlgorithmOperationTorrent // torrents that will stop downloading but still uploading
	DeleteTorrents []AlgorithmOperationTorrent // torrents that will be removed from client
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
	InfoHash string
	Score    float64
	Msg      string
}

type clientTorrentInfoStruct struct {
	Torrent             *client.Torrent
	ModifyFlag          bool
	StallFlag           bool
	DeleteCandidateFlag bool
	DeleteFlag          bool
}

/*
 * Strategy
 * Delete a torrent from client ONLY when it's uploading speed become SLOW enough AND free disk space insufficient.
 * Stall ALL torrent of client (limit download speed to 1B/s, so upload only) when free disk space insufficient
 * Add new torrents to client when server uploading bandwidth is somewhat idle AND there is SOME free disk space.
 *
 */
func Decide(clientStatus *client.Status, clientTorrents []client.Torrent, siteTorrents []site.SiteTorrent, option *BrushOptionStruct) (result *AlgorithmResult) {
	result = &AlgorithmResult{}

	cntTorrents := len(clientTorrents)
	cntDownloadingTorrents := int64(0)
	freespace := clientStatus.FreeSpaceOnDisk
	estimateUploadSpeed := clientStatus.UploadSpeed

	var candidateTorrents []candidateTorrentStruct
	var modifyTorrents []AlgorithmModifyTorrent
	var stallTorrents []candidateClientTorrentStruct
	var deleteCandidateTorrents []candidateClientTorrentStruct
	clientTorrentsMap := make(map[string](*clientTorrentInfoStruct))
	siteTorrentsMap := make(map[string](*site.SiteTorrent))

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
		if torrent.State == "downloading" && torrent.DownloadSpeedLimit != 1 {
			cntDownloadingTorrents++
		}

		// mark torrents that discount time ends as stall and delete
		if torrent.Meta["dcet"] > 0 && torrent.Meta["dcet"]-utils.Now() <= 3600 &&
			(torrent.State != "completed" && torrent.State != "seeding") {
			stallTorrents = append(stallTorrents, candidateClientTorrentStruct{
				InfoHash: torrent.InfoHash,
				Score:    math.Inf(1),
				Msg:      "discount time ends",
			})
			clientTorrentsMap[torrent.InfoHash].StallFlag = true

			if torrent.UploadSpeed < option.SlowUploadSpeedTier {
				deleteCandidateTorrents = append(deleteCandidateTorrents, candidateClientTorrentStruct{
					InfoHash: torrent.InfoHash,
					Score:    -float64(torrent.UploadSpeed),
					Msg:      "discount time ends",
				})
				clientTorrentsMap[torrent.InfoHash].DeleteCandidateFlag = true
			}
			continue
		}

		// skip new added torrents
		if utils.Now()-torrent.Atime <= 15*60 {
			continue
		}

		// check slow torrents, add it to watch list first time and mark as deleteCandidate second time
		if torrent.UploadSpeed < option.SlowUploadSpeedTier {
			if torrent.Meta["sct"] > 0 { // second encounter on slow torrent
				if option.Now-torrent.Meta["sct"] >= 15*60 {
					averageUploadSpeedSinceSct := (torrent.Uploaded - torrent.Meta["sctu"]) / (option.Now - torrent.Meta["sct"])
					if averageUploadSpeedSinceSct < option.SlowUploadSpeedTier {
						deleteCandidateTorrents = append(deleteCandidateTorrents, candidateClientTorrentStruct{
							InfoHash: torrent.InfoHash,
							Score:    -float64(torrent.UploadSpeed),
							Msg:      "slow uploading speed",
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
	sort.SliceStable(stallTorrents, func(i, j int) bool {
		return stallTorrents[i].Score > stallTorrents[j].Score
	})

	// if not enough free space, delete torrents to free space
	for freespace < option.MinDiskSpace && len(deleteCandidateTorrents) > 0 {
		deleteTorrent := deleteCandidateTorrents[0]
		torrent := clientTorrentsMap[deleteTorrent.InfoHash].Torrent
		deleteCandidateTorrents = deleteCandidateTorrents[1:]
		result.DeleteTorrents = append(result.DeleteTorrents, AlgorithmOperationTorrent{
			InfoHash: torrent.InfoHash,
			Name:     torrent.Name,
			Msg:      deleteTorrent.Msg,
		})
		freespace += torrent.SizeCompleted
		clientTorrentsMap[deleteTorrent.InfoHash].DeleteFlag = true
		if torrent.State == "downloading" && torrent.DownloadSpeedLimit != 1 {
			cntDownloadingTorrents--
		}
		estimateUploadSpeed -= torrent.UploadSpeed
	}

	// if still not enough free space, mark ALL torrents as stall
	if freespace < option.MinDiskSpace {
		for _, torrent := range clientTorrents {
			if clientTorrentsMap[torrent.InfoHash].DeleteFlag || clientTorrentsMap[torrent.InfoHash].StallFlag {
				continue
			}
			if torrent.State == "downloading" && torrent.DownloadSpeedLimit != 1 {
				stallTorrents = append(stallTorrents, candidateClientTorrentStruct{
					InfoHash: torrent.InfoHash,
					Score:    math.Inf(1),
					Msg:      "insufficient free disk space",
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
		torrent := clientTorrentsMap[stallTorrent.InfoHash].Torrent
		result.StallTorrents = append(result.StallTorrents, AlgorithmOperationTorrent{
			InfoHash: torrent.InfoHash,
			Name:     torrent.Name,
			Msg:      stallTorrent.Msg,
		})
		if torrent.State == "downloading" && torrent.DownloadSpeedLimit != 1 {
			cntDownloadingTorrents--
		}
	}

	for _, modifyTorrent := range modifyTorrents {
		if clientTorrentsMap[modifyTorrent.InfoHash].DeleteFlag {
			continue
		}

		torrent := clientTorrentsMap[modifyTorrent.InfoHash].Torrent
		result.ModifyTorrents = append(result.ModifyTorrents, AlgorithmModifyTorrent{
			InfoHash: torrent.InfoHash,
			Name:     torrent.Name,
			Msg:      modifyTorrent.Msg,
			Meta:     modifyTorrent.Meta,
		})
	}

	if freespace >= option.MinDiskSpace {
		// add new torrents
		for cntTorrents < 50 && cntDownloadingTorrents <= 5 && estimateUploadSpeed <= clientStatus.UploadSpeedLimit*2 && len(candidateTorrents) > 0 {
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

	return
}

func rateSiteTorrent(siteTorrent *site.SiteTorrent, brushOption *BrushOptionStruct) (score float64, predictionUploadSpeed int64) {
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
		if siteTorrent.Leechers >= 1000 { // 馒头大包特殊处理
			score *= 100
		} else if siteTorrent.Leechers >= 500 { // 馒头大包特殊处理
			score *= 50
		} else {
			score *= 0
		}
	}
	return
}
