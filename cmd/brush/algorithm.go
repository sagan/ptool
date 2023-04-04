package brush

import (
	"fmt"
	"sort"

	"github.com/sagan/ptool/client"
	"github.com/sagan/ptool/site"
	"github.com/sagan/ptool/utils"
)

type AlgorithmOperationTorrent struct {
	InfoHash string
	Name     string
	Msg      string
}

type AlgorithmModifyTorrent struct {
	InfoHash string
	Name     string
	Meta     map[string](int64)
	Msg      string
}

type AlgorithmAddTorrent struct {
	DownloadUrl string
	Name        string
	Meta        map[string](int64)
	Msg         string
}

type AlgorithmResult struct {
	DeleteTorrents []AlgorithmOperationTorrent // torrents that will be removed from client
	StallTorrents  []AlgorithmOperationTorrent // torrents that will stop downloading but still uploading
	ModifyTorrents []AlgorithmModifyTorrent    // modify meta info of these torrents
	AddTorrents    []AlgorithmAddTorrent       // new torrents that will be added to client
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

/*
 * Strategy
 * Delete a torrent from client ONLY when it's uploading speed become SLOW enough.
 * Stall ALL torrent of client (limit download speed to 1B/s, so upload only) when free disk space insufficient
 * Add new torrents to client when server (downloads / upload) bandwidth is somewhat idle and there is SOME free disk space.
 *
 * @todo
 * Only delete torrents when more free disk space is needed
 */
func Decide(clientStatus *client.Status, clientTorrents []client.Torrent,
	siteTorrents []site.SiteTorrent) (result *AlgorithmResult) {
	result = &AlgorithmResult{}

	freespace := clientStatus.FreeSpaceOnDisk
	estimateUploadSpeed := clientStatus.UploadSpeed
	var candidateTorrents []candidateTorrentStruct
	deletedFlag := map[string](bool){}
	now := utils.Now()
	cntTorrents := len(clientTorrents)
	cntDownloadingTorrents := int64(0)

	for _, siteTorrent := range siteTorrents {
		score, predictionUploadSpeed := rateSiteTorrent(&siteTorrent, now)
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
	if len(candidateTorrents) > 3 {
		candidateTorrents = candidateTorrents[:3]
	}

	for _, torrent := range clientTorrents {
		if deletedFlag[torrent.InfoHash] {
			continue
		}
		// remove torrents that discount time ends
		if torrent.Meta["dcet"] > 0 && torrent.Meta["dcet"]-utils.Now() <= 3600 &&
			(torrent.State != "completed" && torrent.State != "seeding") {
			if torrent.UploadSpeed < 100*1024 {
				result.DeleteTorrents = append(result.DeleteTorrents,
					AlgorithmOperationTorrent{
						InfoHash: torrent.InfoHash,
						Name:     torrent.Name,
						Msg:      "Delete due to discount time ends",
					})
				deletedFlag[torrent.InfoHash] = true
				cntTorrents--
				freespace += torrent.Downloaded
				estimateUploadSpeed -= torrent.UploadSpeed
			} else {
				result.StallTorrents = append(result.StallTorrents, AlgorithmOperationTorrent{
					Name:     torrent.Name,
					InfoHash: torrent.InfoHash,
					Msg:      "Stall torrent due to discount time ends",
				})
			}
			continue
		}

		// skip new added torrents
		if utils.Now()-torrent.Atime <= 15*60 {
			if torrent.State == "downloading" && torrent.DownloadSpeedLimit > 1 {
				cntDownloadingTorrents++
			}
			continue
		}

		// check slow torrents, mark them on first time and delete them on second time
		if torrent.UploadSpeed < 100*1024 {
			if torrent.Meta["sct"] > 0 && now-torrent.Meta["sct"] >= 15*60 {
				averageUploadSpeedSinceSct := (torrent.Uploaded - torrent.Meta["sctu"]) / (now - torrent.Meta["sct"])
				if averageUploadSpeedSinceSct < 100*1024 {
					result.DeleteTorrents = append(result.DeleteTorrents, AlgorithmOperationTorrent{
						InfoHash: torrent.InfoHash,
						Name:     torrent.Name,
						Msg:      "Delete due to slow upload",
					})
					deletedFlag[torrent.InfoHash] = true
					freespace += torrent.Downloaded
					estimateUploadSpeed -= torrent.UploadSpeed
				} else {
					meta := utils.CopyMap(torrent.Meta)
					meta["sct"] = now
					meta["sctu"] = torrent.Uploaded
					result.ModifyTorrents = append(result.ModifyTorrents, AlgorithmModifyTorrent{
						InfoHash: torrent.InfoHash,
						Name:     torrent.Name,
						Msg:      "Reset slow check time mark",
						Meta:     meta,
					})
					if torrent.State == "downloading" && torrent.DownloadSpeedLimit > 1 {
						cntDownloadingTorrents++
					}
				}
			} else {
				meta := utils.CopyMap(torrent.Meta)
				meta["sct"] = now
				meta["sctu"] = torrent.Uploaded
				result.ModifyTorrents = append(result.ModifyTorrents, AlgorithmModifyTorrent{
					InfoHash: torrent.InfoHash,
					Name:     torrent.Name,
					Msg:      "Set slow check time mark",
					Meta:     meta,
				})
				if torrent.State == "downloading" && torrent.DownloadSpeedLimit > 1 {
					cntDownloadingTorrents++
				}
			}
		} else if torrent.Meta["sct"] > 0 {
			meta := utils.CopyMap(torrent.Meta)
			delete(meta, "sct")
			delete(meta, "sctu")
			result.ModifyTorrents = append(result.ModifyTorrents, AlgorithmModifyTorrent{
				InfoHash: torrent.InfoHash,
				Name:     torrent.Name,
				Msg:      "Remove slow check time mark",
				Meta:     meta,
			})
			if torrent.State == "downloading" {
				cntDownloadingTorrents++
			}
		}
	}

	if freespace >= 0 && freespace < 1024*1024*1024*5 {
		for _, torrent := range clientTorrents {
			if deletedFlag[torrent.InfoHash] {
				continue
			}
			if torrent.State == "downloading" && torrent.DownloadSpeedLimit > 1 {
				result.StallTorrents = append(result.StallTorrents, AlgorithmOperationTorrent{
					Name:     torrent.Name,
					InfoHash: torrent.InfoHash,
					Msg:      "Stall torrent due to insufficient disk space",
				})
			}
		}
		return
	}

	for cntTorrents < 20 && cntDownloadingTorrents < 5 && estimateUploadSpeed <= clientStatus.UploadSpeedLimit {
		if len(candidateTorrents) == 0 {
			break
		}
		candidateTorrent := candidateTorrents[0]
		candidateTorrents = candidateTorrents[1:]
		result.AddTorrents = append(result.AddTorrents, AlgorithmAddTorrent{
			DownloadUrl: candidateTorrent.DownloadUrl,
			Name:        candidateTorrent.Name,
			Meta:        candidateTorrent.Meta,
			Msg:         fmt.Sprintf("new torrrent of score %f", candidateTorrent.Score),
		})
		cntTorrents++
		cntDownloadingTorrents++
		estimateUploadSpeed += candidateTorrent.PredictionUploadSpeed
	}

	return
}

func rateSiteTorrent(siteTorrent *site.SiteTorrent, now int64) (score float64, predictionUploadSpeed int64) {
	if siteTorrent.IsActive ||
		siteTorrent.HasHnR ||
		siteTorrent.DownloadMultiplier != 0 ||
		(siteTorrent.DiscountEndTime > 0 && siteTorrent.DiscountEndTime-now < 3600) ||
		siteTorrent.Seeders == 0 ||
		siteTorrent.Leechers <= siteTorrent.Seeders {
		score = 0
		return
	}

	if now-siteTorrent.Time >= 86400 {
		score = 0
		return
	} else if now-siteTorrent.Time >= 7200 {
		if siteTorrent.Leechers < 500 {
			score = 0
			return
		}
	}

	predictionUploadSpeed = siteTorrent.Leechers * 100 * 1024

	if siteTorrent.Seeders == 1 {
		score = 50
	} else if siteTorrent.Seeders <= 3 {
		score = 30
	} else {
		score = 10
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
