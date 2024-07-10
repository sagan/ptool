package common

import (
	"bytes"
	"fmt"
	"io"
	"path"
	"slices"
	"strings"

	"github.com/natefinch/atomic"
	"github.com/sagan/ptool/client"
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

func (ts *TorrentsStatistics) Update(torrentType TorrentType, size int64, files int64) {
	switch torrentType {
	case TORRENT_SUCCESS:
		ts.TorrentsCnt++
		ts.SuccessCnt++
		ts.SuccessContentFiles += files
		ts.SuccessSize += size
		if ts.LargestSize == -1 || size > ts.LargestSize {
			ts.LargestSize = size
		}
		if ts.SmallestSize == -1 || size < ts.SmallestSize {
			ts.SmallestSize = size
		}
	case TORRENT_FAILURE:
		ts.TorrentsCnt++
		ts.FailureCnt++
		ts.FailureSize += size
	case TORRENT_INVALID:
		ts.InvalidCnt++
	}
}

func (ts *TorrentsStatistics) UpdateClientTorrent(torrentType TorrentType, torrent *client.Torrent) {
	ts.Update(torrentType, torrent.Size, 0)
}

func (ts *TorrentsStatistics) UpdateTinfo(torrentType TorrentType, tinfo *torrentutil.TorrentMeta) {
	if tinfo != nil {
		ts.Update(torrentType, tinfo.Size, int64(len(tinfo.Files)))
	} else {
		ts.Update(torrentType, 0, 0)
	}
}

func (ts *TorrentsStatistics) Print(output io.Writer) {
	averageSize := int64(0)
	if ts.SuccessCnt > 0 {
		averageSize = ts.SuccessSize / ts.SuccessCnt
	}
	fmt.Fprintf(output, "Total (valid) torrents: %d\n", ts.TorrentsCnt)
	fmt.Fprintf(output, "Success torrents contents size: %s (%d Byte)\n",
		util.BytesSize(float64(ts.SuccessSize)), ts.SuccessSize)
	fmt.Fprintf(output, "Success torrents contents files number: %d\n", ts.SuccessContentFiles)
	fmt.Fprintf(output, "Success torrents Smallest / Average / Largest contents size: %s / %s / %s\n",
		util.BytesSize(float64(ts.SmallestSize)), util.BytesSize(float64(averageSize)),
		util.BytesSize(float64(ts.LargestSize)))
	fmt.Fprintf(output, "Failure torrents: %d (%s)\n", ts.FailureCnt, util.BytesSize(float64(ts.FailureSize)))
	fmt.Fprintf(output, "Invalid torrents: %d\n", ts.InvalidCnt)
}

type PathMapper struct {
	mapper  map[string]string
	befores []string
}

func (spm *PathMapper) Before2After(beforePath string) (afterPath string, match bool) {
	beforePath = path.Clean(util.ToSlash(beforePath))
	for _, before := range spm.befores {
		if before == "/" {
			if strings.HasPrefix(beforePath, before) {
				return spm.mapper[before] + strings.TrimPrefix(beforePath, before), true
			}
		} else if strings.HasPrefix(beforePath, before+"/") {
			return spm.mapper[before] + strings.TrimPrefix(beforePath, before), true
		}
	}
	return beforePath, false
}

func (spm *PathMapper) After2Before(afterPath string) (beforePath string, match bool) {
	afterPath = path.Clean(util.ToSlash(afterPath))
	for _, before := range spm.befores {
		after := spm.mapper[before]
		if after == "/" {
			if strings.HasPrefix(afterPath, after) {
				return before + strings.TrimPrefix(afterPath, after), true
			}
		} else if strings.HasPrefix(afterPath, after+"/") {
			return before + strings.TrimPrefix(afterPath, after), true
		}
	}
	return afterPath, false
}

func NewPathMapper(rules []string) (*PathMapper, error) {
	pm := &PathMapper{
		mapper: map[string]string{},
	}
	for _, rule := range rules {
		sep := "|"
		// use ":" as sep only when "|" not exists and no Windows abs path (e.g. "E:\Downloads") exists
		if !strings.Contains(rule, "|") && !strings.Contains(rule, `:\`) {
			sep = ":"
		}
		before, after, found := strings.Cut(rule, sep)
		if !found || before == "" || after == "" {
			return nil, fmt.Errorf("invalid path mapper rule %q", rule)
		}
		before = path.Clean(util.ToSlash(before))
		after = path.Clean(util.ToSlash(after))
		pm.mapper[before] = after
		pm.befores = append(pm.befores, before)
	}
	slices.SortFunc(pm.befores, func(a, b string) int { return len(b) - len(a) }) // longest first
	return pm, nil
}

func ExportClientTorrent(clientInstance client.Client, torrent *client.Torrent,
	filepath string, useCommentMeta bool) (contents []byte, tinfo *torrentutil.TorrentMeta, err error) {
	contents, err = clientInstance.ExportTorrentFile(torrent.InfoHash)
	if err != nil {
		return nil, nil, err
	}
	if tinfo, err = torrentutil.ParseTorrent(contents); err != nil {
		return contents, nil, fmt.Errorf("failed to parse torrent: %w", err)
	}
	if useCommentMeta {
		var useCommentErr error
		if err := tinfo.EncodeComment(&torrentutil.TorrentCommentMeta{
			Category: torrent.Category,
			Tags:     torrent.Tags,
			SavePath: torrent.SavePath,
		}); err != nil {
			useCommentErr = fmt.Errorf("failed to encode comment-meta: %w", err)
		} else if data, err := tinfo.ToBytes(); err != nil {
			useCommentErr = fmt.Errorf("failed to re-generate torrent with comment-meta: %w", err)
		} else {
			contents = data
		}
		if useCommentErr != nil {
			return contents, tinfo, useCommentErr
		}
	}
	if err := atomic.WriteFile(filepath, bytes.NewReader(contents)); err != nil {
		return contents, tinfo, fmt.Errorf("failed to write file: %w", err)
	}
	return contents, tinfo, nil
}
