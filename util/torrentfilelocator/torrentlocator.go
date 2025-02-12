package torrentfilelocator

import (
	"bytes"
	"crypto/sha1"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"

	log "github.com/sirupsen/logrus"

	"github.com/sagan/ptool/util"
	"github.com/sagan/ptool/util/torrentutil"
)

type LocateState int64

const (
	LocateStateNone = iota
	LocateStateFail
	LocateStateNeedConfirm
	LocateStateLocated
)

type FsFile struct {
	Path    string // full file system path
	Name    string
	Size    int64
	Located bool
}

type TorrentFileLink struct {
	State             LocateState
	TorrentFile       *torrentutil.TorrentMetaFile
	LinkedFsFileIndex int          // located fs file index in FsFiles, or -1 if not located
	FsFiles           []*FsFile    // candidate same size (even same name) fs files
	FailedFsFiles     map[int]bool // fs file index => true. Marked it as failed
}

// 单个 piece 对应的(可能多个)文件
type PieceFile struct {
	FileLink   *TorrentFileLink
	Offset     int64
	ReadLength int64
}

// Used as both locator and result
type LocateResult struct {
	Ok               bool // true if State == Located (all torrent files successfully located)
	State            LocateState
	LocatedCnt       int64
	TorrentFileLinks []*TorrentFileLink
	Error            error
	fsFileInfos      []*FsFile                // flatten list of all file system files in source content path
	tinfo            *torrentutil.TorrentMeta // parsed .torrent file meta info
	checkPieceResult map[int64]bool
}

// 根据 .torrent 文件内容，在硬盘内容文件夹(contentPath) 里进行匹配，
// 查找每个种子内容文件(TorrentFile)对应的的硬盘文件(FsFile)。
// 定位逻辑类似 TorrentHardLinkHelper，有一些改动。
// 对 .torrent 里定义的每个种子内容文件：
// 1. 在硬盘上寻找文件大小相同的文件，如果结果只有 1个，那么这个文件即作为种子内容文件。
// 2. 如果硬盘上相同大小的文件有多个但其中只有1个文件名相同，则使用这个文件。
// 3. 如果硬盘上相同大小甚至相同文件名的文件有多个，则需要通过 piece hash 进行判断。
// 种子的单个 piece 可能对应多个 TorrentFile，每个 TorrentFile 可能有多个候选的硬盘文件。
// 对于单个 piece 的对应的每种可能硬盘文件组合(permutation)依次进行 hash 验证是否正确。
// 目前，第一个匹配的硬盘文件组合即被作为结果。
// 如果 1个 piece 只包含1个种子内容文件，那么某个硬盘文件 hash 失败则表明这个硬盘文件肯定不是对应的种子内容文件。
func Locate(tinfo *torrentutil.TorrentMeta, contentFolder string) *LocateResult {
	var fsFileInfos []*FsFile
	err := filepath.WalkDir(contentFolder, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		if d.Type().IsRegular() {
			fsFileInfos = append(fsFileInfos, &FsFile{
				Path: path,
				Name: filepath.Base(path),
				Size: info.Size(),
			})
		}
		return nil
	})
	if err != nil {
		return &LocateResult{Error: err}
	}
	result := &LocateResult{
		tinfo:            tinfo,
		fsFileInfos:      fsFileInfos,
		checkPieceResult: map[int64]bool{},
	}
	result.findTorrentFileLinks()
	if result.State == LocateStateNeedConfirm {
		result.confirmFileSystemFiles()
	}
	return result
}

func (l *LocateResult) findTorrentFileLinks() {
	for _, tfile := range l.tinfo.Files {
		torrentFileLink := &TorrentFileLink{
			TorrentFile:       tfile,
			LinkedFsFileIndex: -1,
			FailedFsFiles:     map[int]bool{},
		}
		sameSizeFsFiles := util.Filter(l.fsFileInfos, func(f *FsFile) bool {
			return f.Size == tfile.Size
		})
		tfileName := filepath.Base(tfile.Path)
		sameNameAndSizeFsFiles := util.Filter(sameSizeFsFiles, func(f *FsFile) bool {
			return f.Name == tfileName
		})
		var fsFiles []*FsFile
		if len(sameNameAndSizeFsFiles) > 0 {
			fsFiles = sameNameAndSizeFsFiles
		} else {
			fsFiles = sameSizeFsFiles
		}
		torrentFileLink.FsFiles = fsFiles
		if len(fsFiles) == 1 {
			torrentFileLink.State = LocateStateLocated
			torrentFileLink.LinkedFsFileIndex = 0
		} else if len(fsFiles) > 1 {
			torrentFileLink.State = LocateStateNeedConfirm
		} else { // len(fsFiles) == 0
			torrentFileLink.State = LocateStateFail
		}
		l.TorrentFileLinks = append(l.TorrentFileLinks, torrentFileLink)
	}
	l.update()
}

func (l *LocateResult) confirmFileSystemFiles() {
	for _, torrentFileLink := range l.TorrentFileLinks {
		if torrentFileLink.State != LocateStateNeedConfirm {
			continue
		}
		for i := torrentFileLink.TorrentFile.StartPieceIndex; i <= torrentFileLink.TorrentFile.EndPieceIndex; i++ {
			ok := l.checkPiece(i)
			log.Tracef(`Checked torrent file %s at piece %d: %t\n`, torrentFileLink.TorrentFile.Path, i, ok)
			if !ok || torrentFileLink.State == LocateStateLocated {
				break
			}
		}
	}
	l.update()
}

func (l *LocateResult) update() {
	fsFileUsedCnt := map[string]int64{}
	for _, fileLink := range l.TorrentFileLinks {
		if fileLink.State == LocateStateLocated {
			fsFileUsedCnt[fileLink.FsFiles[fileLink.LinkedFsFileIndex].Path]++
		}
	}

	locatedCnt := int64(0)
	hasFailFile := false
	for _, fileLink := range l.TorrentFileLinks {
		if fileLink.State == LocateStateLocated {
			// do not allow a fs file to be used as multiple torrent content file
			if fsFileUsedCnt[fileLink.FsFiles[fileLink.LinkedFsFileIndex].Path] > 1 {
				fileLink.State = LocateStateNeedConfirm
				fileLink.LinkedFsFileIndex = -1
			} else {
				locatedCnt++
			}
		} else if fileLink.State == LocateStateFail {
			hasFailFile = true
		}
	}
	l.LocatedCnt = locatedCnt
	if locatedCnt == int64(len(l.TorrentFileLinks)) {
		l.Ok = true
		l.State = LocateStateLocated
	} else {
		l.Ok = false
		if hasFailFile {
			l.State = LocateStateFail
		} else {
			l.State = LocateStateNeedConfirm
		}
	}
}

// Return whether matched fs file(s) of current piece is found
func (l *LocateResult) checkPiece(pieceIndex int64) bool {
	if result, ok := l.checkPieceResult[pieceIndex]; ok {
		return result
	}

	var pieceFiles []*PieceFile
	for _, fileLink := range l.TorrentFileLinks {
		if fileLink.TorrentFile.StartPieceIndex > pieceIndex {
			break
		}
		if fileLink.TorrentFile.EndPieceIndex < pieceIndex {
			continue
		}
		offset := int64(0)
		readLength := l.tinfo.Info.PieceLength
		if pieceIndex == fileLink.TorrentFile.StartPieceIndex {
			readLength = min(l.tinfo.Info.PieceLength-fileLink.TorrentFile.StartPieceOffset, fileLink.TorrentFile.Size)
		} else {
			offset = l.tinfo.Info.PieceLength - fileLink.TorrentFile.StartPieceOffset +
				(pieceIndex-fileLink.TorrentFile.StartPieceIndex-1)*l.tinfo.Info.PieceLength
		}
		if pieceIndex == fileLink.TorrentFile.EndPieceIndex {
			readLength = fileLink.TorrentFile.LastPieceBytes
		}
		pieceFile := &PieceFile{
			FileLink:   fileLink,
			Offset:     offset,
			ReadLength: readLength,
		}
		pieceFiles = append(pieceFiles, pieceFile)
	}

	// log.Tracef("pieceFiles")
	// util.PrintJson(os.Stdout, pieceFiles)

	var indexes []int = nil
	for {
		indexes = next(pieceFiles, indexes)
		if indexes == nil {
			break
		}
		if hasDupulicate(pieceFiles, indexes) {
			continue
		}
		log.Tracef("check piece files: %v", indexesLabel(pieceFiles, indexes))
		hash, err := hashPiece(pieceFiles, indexes)
		if err != nil {
			l.checkPieceResult[pieceIndex] = false
			l.Error = err
			return false
		}
		if bytes.Equal(hash, l.tinfo.Info.Piece(int(pieceIndex)).V1Hash().Value.Bytes()) {
			for i, index := range indexes {
				if pieceFiles[i].FileLink.State == LocateStateNeedConfirm {
					pieceFiles[i].FileLink.State = LocateStateLocated
					pieceFiles[i].FileLink.LinkedFsFileIndex = index
				}
			}
			l.checkPieceResult[pieceIndex] = true
			return true
		}
		// 如果一个 piece 只包含 1个文件，hash 失败说明硬盘文件肯定不匹配种子文件
		if len(pieceFiles) == 1 {
			pieceFiles[0].FileLink.FailedFsFiles[indexes[0]] = true
			if pieceFiles[0].FileLink.LinkedFsFileIndex == indexes[0] {
				pieceFiles[0].FileLink.LinkedFsFileIndex = -1
				pieceFiles[0].FileLink.State = LocateStateNeedConfirm
			}
		}
	}

	l.checkPieceResult[pieceIndex] = false
	return false
}

// Return next indexes. Update indexes in place and return it.
// If indexes is nil, return the first indexes.
// If indexes is already the end, return nil.
func next(pieceFiles []*PieceFile, indexes []int) []int {
	if indexes == nil {
		for _, pieceFile := range pieceFiles {
			if pieceFile.FileLink.State == LocateStateLocated {
				indexes = append(indexes, pieceFile.FileLink.LinkedFsFileIndex)
			} else {
				foundIndex := -1
				for index := range pieceFile.FileLink.FsFiles {
					if !pieceFile.FileLink.FailedFsFiles[index] {
						foundIndex = index
						break
					}
				}
				if foundIndex == -1 {
					return nil
				}
				indexes = append(indexes, foundIndex)
			}
		}
		return indexes
	}
	mod := false
main:
	for i, index := range indexes {
		if pieceFiles[i].FileLink.State == LocateStateLocated {
			continue
		}
		for nextIndex := index + 1; nextIndex < len(pieceFiles[i].FileLink.FsFiles); nextIndex++ {
			if !pieceFiles[i].FileLink.FailedFsFiles[nextIndex] {
				indexes[i] = nextIndex
				mod = true
				for j := 0; j < i; j++ {
					if pieceFiles[j].FileLink.State != LocateStateLocated {
						for index := range pieceFiles[j].FileLink.FsFiles {
							if !pieceFiles[j].FileLink.FailedFsFiles[index] {
								indexes[j] = index
								break
							}
						}
					}
				}
				break main
			}
		}
	}
	if mod {
		return indexes
	}
	return nil
}

func hashPiece(pieceFiles []*PieceFile, indexes []int) ([]byte, error) {
	hasher := sha1.New()
	for i, pieceFile := range pieceFiles {
		fsFile := pieceFile.FileLink.FsFiles[indexes[i]]
		fd, err := os.Open(fsFile.Path)
		if err != nil {
			return nil, err
		}
		_, err = io.Copy(hasher, io.NewSectionReader(fd, pieceFile.Offset, pieceFile.ReadLength))
		fd.Close()
		if err != nil {
			return nil, err
		}
	}
	return hasher.Sum(nil), nil
}

func (l *LocateResult) Print(out io.Writer) {
	stateLabels := map[LocateState]string{
		LocateStateFail:        "fail",
		LocateStateLocated:     "located",
		LocateStateNeedConfirm: "need_confirm",
		LocateStateNone:        "none",
	}
	fmt.Fprintf(out, "State: %s\n", stateLabels[l.State])
	fmt.Fprintf(out, "Located files: %d\n", l.LocatedCnt)
	fmt.Fprintf(out, "Unlocated files: %d\n", int64(len(l.TorrentFileLinks))-l.LocatedCnt)
	fmt.Fprintf(out, "Torrent Files:\n")
	fmt.Fprintf(out, "%5s  %-12s  %s\n", "Index", "State", "File")
	for i, fileLink := range l.TorrentFileLinks {
		fmt.Fprintf(out, "%5d  %-12s  %s", i, stateLabels[fileLink.State], fileLink.TorrentFile.Path)
		if fileLink.State == LocateStateLocated {
			fmt.Fprintf(out, " <= %s", fileLink.FsFiles[fileLink.LinkedFsFileIndex].Path)
		}
		fmt.Fprintf(out, "\n")
	}
}

func indexesLabel(pieceFiles []*PieceFile, indexes []int) string {
	s := ""
	for i, index := range indexes {
		if i > 0 {
			s += ", "
		}
		s += fmt.Sprintf("%s #%d (%s)", pieceFiles[i].FileLink.TorrentFile.Path, index,
			pieceFiles[i].FileLink.FsFiles[index].Path)
	}
	return s
}

// duplicate: A fs file is used as mulitple torrent file
func hasDupulicate(pieceFiles []*PieceFile, indexes []int) bool {
	flag := map[string]bool{}
	for i, pieceFile := range pieceFiles {
		if flag[pieceFile.FileLink.FsFiles[indexes[i]].Path] {
			return true
		}
		flag[pieceFile.FileLink.FsFiles[indexes[i]].Path] = true
	}
	return false
}
