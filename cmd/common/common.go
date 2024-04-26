package common

import (
	"fmt"
	"io"

	"github.com/sagan/ptool/util"
	"github.com/sagan/ptool/util/torrentutil"
)

type TorrentType int

const (
	TORRENT_SUCCESS TorrentType = iota
	TORRENT_FAILURE
	TORRENT_INVALID
)

type TorrentsStatistics struct {
	TorrentsCnt         int64 // number of valid .torrent files
	SuccessCnt          int64
	SuccessSize         int64
	SuccessContentFiles int64
	FailureCnt          int64
	FailureSize         int64
	InvalidCnt          int64
	SmallestSize        int64
	LargestSize         int64
}

func NewTorrentsStatistics() *TorrentsStatistics {
	return &TorrentsStatistics{
		SmallestSize: -1,
		LargestSize:  -1,
	}
}

func (ts *TorrentsStatistics) Update(torrentType TorrentType, tinfo *torrentutil.TorrentMeta) {
	switch torrentType {
	case TORRENT_SUCCESS:
		ts.TorrentsCnt++
		ts.SuccessCnt++
		ts.SuccessContentFiles += int64(len(tinfo.Files))
		ts.SuccessSize += tinfo.Size
		if ts.LargestSize == -1 || tinfo.Size > ts.LargestSize {
			ts.LargestSize = tinfo.Size
		}
		if ts.SmallestSize == -1 || tinfo.Size < ts.SmallestSize {
			ts.SmallestSize = tinfo.Size
		}
	case TORRENT_FAILURE:
		ts.TorrentsCnt++
		ts.FailureCnt++
		ts.FailureSize += tinfo.Size
	case TORRENT_INVALID:
		ts.InvalidCnt++
	}
}

func (ts *TorrentsStatistics) Print(output io.Writer) {
	averageSize := int64(0)
	if ts.SuccessCnt > 0 {
		averageSize = ts.SuccessSize / ts.SuccessCnt
	}
	fmt.Fprintf(output, "Success torrents: %d\n", ts.TorrentsCnt)
	fmt.Fprintf(output, "Total contents size: %s (%d Byte)\n", util.BytesSize(float64(ts.SuccessSize)), ts.SuccessSize)
	fmt.Fprintf(output, "Total number of content files: %d\n", ts.SuccessContentFiles)
	fmt.Fprintf(output, "Smallest / Average / Largest torrent contents size: %s / %s / %s\n",
		util.BytesSize(float64(ts.SmallestSize)), util.BytesSize(float64(averageSize)),
		util.BytesSize(float64(ts.LargestSize)))
	fmt.Fprintf(output, "Failure torrents: %d (%s)\n", ts.FailureCnt, util.BytesSize(float64(ts.FailureSize)))
	fmt.Fprintf(output, "Invalid torrents: %d\n", ts.InvalidCnt)
}
