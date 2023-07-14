package torrentutil

import (
	"bytes"
	"crypto/sha1"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/Emyrk/torrent/mmap_span"
	"github.com/anacrolix/torrent/metainfo"
	"github.com/bradfitz/iter"
	mmap "github.com/edsrzf/mmap-go"
	"github.com/sagan/ptool/client"
	"github.com/sagan/ptool/site/tpl"
	"github.com/sagan/ptool/utils"
	log "github.com/sirupsen/logrus"
)

type TorrentMetaFile struct {
	Path string // full path joined by '/'
	Size int64
}

type TorrentMeta struct {
	InfoHash          string
	Trackers          []string
	Size              int64
	SingleFileTorrent bool
	RootDir           string
	Files             []TorrentMetaFile
	MetaInfo          *metainfo.MetaInfo
	Info              metainfo.Info
}

// fields: 0 - only infoHash; 1- infoHash + trackers; 2+ - all
func ParseTorrent(torrentdata []byte, fields int64) (*TorrentMeta, error) {
	metaInfo, err := metainfo.Load(bytes.NewReader(torrentdata))
	if err != nil {
		return nil, err
	}
	torrentMeta := &TorrentMeta{}
	torrentMeta.InfoHash = metaInfo.HashInfoBytes().String()
	if fields <= 0 {
		return torrentMeta, nil
	}
	announceList := metaInfo.UpvertedAnnounceList()
	if len(announceList) > 0 {
		torrentMeta.Trackers = announceList[0]
	}
	if fields <= 1 {
		return torrentMeta, nil
	}
	info, err := metaInfo.UnmarshalInfo()
	if err != nil {
		return nil, err
	}
	torrentMeta.Info = info
	// single file torrent
	if len(info.Files) == 0 {
		torrentMeta.Files = append(torrentMeta.Files, TorrentMetaFile{
			Path: info.Name,
			Size: info.Length,
		})
		torrentMeta.SingleFileTorrent = true
		torrentMeta.Size = info.Length
	} else {
		if info.Name != "" && info.Name != metainfo.NoName {
			torrentMeta.RootDir = info.Name
		}
		for _, metafile := range info.Files {
			torrentMeta.Files = append(torrentMeta.Files, TorrentMetaFile{
				Path: strings.Join(metafile.Path, "/"),
				Size: metafile.Length,
			})
			torrentMeta.Size += metafile.Length
		}
	}
	torrentMeta.MetaInfo = metaInfo
	return torrentMeta, nil
}

// fields: 0 - only infoHash; 1- infoHash + trackers; 2+ - all
func ParseTorrentFile(filename string, fields int64) (*TorrentMeta, error) {
	torrentContent, err := os.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to read torrent file %s: %v", filename, err)
	}
	return ParseTorrent(torrentContent, fields)
}

func (meta *TorrentMeta) Print(name string, showAll bool) {
	trackerHostname := ""
	if len(meta.Trackers) > 0 {
		trackerHostname = utils.ParseUrlHostname(meta.Trackers[0])
	}
	sitename := tpl.GuessSiteByTrackers(meta.Trackers, "")
	fmt.Printf("Torrent %s: infohash = %s ; size = %s (%d) ; tracker = %s (site: %s) // %s\n",
		name, meta.InfoHash, utils.BytesSize(float64(meta.Size)), len(meta.Files),
		trackerHostname, sitename, meta.MetaInfo.Comment)
	if showAll {
		if meta.SingleFileTorrent {
			fmt.Printf("RawSize = %d ; SingleFile = %s ; FullTrackerUrls: %s\n",
				meta.Size, meta.Files[0].Path, strings.Join(meta.Trackers, " | "))
		} else {
			fmt.Printf("RawSize = %d ; RootDir = %s ; FullTrackerUrls: %s\n",
				meta.Size, meta.RootDir, strings.Join(meta.Trackers, " | "))
		}
	}
}

func (meta *TorrentMeta) PrintFiles(addRootDirPrefix bool, useRawSize bool) {
	fmt.Printf("Files:\n")
	for i, file := range meta.Files {
		path := file.Path
		if addRootDirPrefix && meta.RootDir != "" {
			path = meta.RootDir + "/" + path
		}
		if useRawSize {
			fmt.Printf("%-5d  %-15d  %s\n", i+1, file.Size, path)
		} else {
			fmt.Printf("%-5d  %-10s  %s\n", i+1, utils.BytesSize(float64(file.Size)), path)
		}
	}
}

// return 0 if this torrent is equal with client torrent;
// return 1 if client torrent contains all files of this torrent.
// return -2 if the ROOT folder(file) of the two are different, but all innner files are SAME.
// return -1 if contents of the two torrents are NOT same.
func (meta *TorrentMeta) XseedCheckWithClientTorrent(clientTorrentContents []client.TorrentContentFile) int64 {
	if len(clientTorrentContents) < len(meta.Files) || len(meta.Files) == 0 {
		return -1
	}
	torrentContents := meta.Files
	clientRootDir := ""
	clientFilesSizeMap := map[string](int64){}

	for _, clientTorrentContent := range clientTorrentContents {
		path := clientTorrentContent.Path
		if meta.RootDir != "" {
			pathes := strings.Split(path, "/")
			if len(pathes) == 1 {
				log.Tracef("CheckWithClientTorrent: torrent has rootDir (%s) but client torrent does NOT (%s)",
					meta.RootDir, clientTorrentContent.Path)
				return -1
			}
			if clientRootDir == "" {
				clientRootDir = pathes[0]
			} else if clientRootDir != pathes[0] {
				log.Tracef("CheckWithClientTorrent: torrent has rootDir (%s) but client torrent does NOT (%s)",
					meta.RootDir, clientTorrentContent.Path)
				return -1
			}
			path = strings.Join(pathes[1:], "/")
		}
		if _, ok := clientFilesSizeMap[path]; ok {
			log.Tracef("CheckWithClientTorrent: client torrent has duplicate file (%s)", clientTorrentContent.Path)
			return -1
		}
		clientFilesSizeMap[path] = clientTorrentContent.Size
	}

	for _, torrentContent := range torrentContents {
		if size, ok := clientFilesSizeMap[torrentContent.Path]; ok {
			if size != torrentContent.Size {
				log.Tracef("CheckWithClientTorrent: torrent file %s size %d does NOT match with client torrent size %d",
					torrentContent.Path, torrentContent.Size, size)
				return -1
			}
		} else {
			log.Tracef("CheckWithClientTorrent: torrent file %s does NOT exist in client torrent", torrentContent.Path)
			return -1
		}
	}
	if meta.RootDir != clientRootDir {
		return -2
	}
	if len(torrentContents) < len(clientTorrentContents) {
		return 1
	}
	return 0
}

func mmapFile(name string) (mm mmap.MMap, err error) {
	f, err := os.Open(name)
	if err != nil {
		return
	}
	defer f.Close()
	fi, err := f.Stat()
	if err != nil {
		return
	}
	if fi.Size() == 0 {
		return
	}
	return mmap.MapRegion(f, -1, mmap.RDONLY, mmap.COPY, 0)
}

func (meta *TorrentMeta) Verify(savePath string, contentPath string, checkHash bool) error {
	span := new(mmap_span.MMapSpan)
	prefixPath := ""
	if contentPath != "" {
		prefixPath = contentPath + "/"
	} else {
		prefixPath = savePath + "/"
		if meta.RootDir != "" {
			prefixPath += meta.RootDir + "/"
		}
	}
	for _, file := range meta.Files {
		filename := ""
		if contentPath != "" && meta.SingleFileTorrent {
			filename = contentPath
		} else {
			filename = prefixPath + file.Path
		}
		mm, err := mmapFile(filename)
		if err != nil {
			return err
		}
		if int64(len(mm)) != file.Size {
			return fmt.Errorf("file %q has wrong length", filename)
		}
		span.Append(mm)
	}
	if checkHash {
		for i := range iter.N(meta.Info.NumPieces()) {
			p := meta.Info.Piece(i)
			hash := sha1.New()
			_, err := io.Copy(hash, io.NewSectionReader(span, p.Offset(), p.Length()))
			if err != nil {
				return err
			}
			good := bytes.Equal(hash.Sum(nil), p.Hash().Bytes())
			if !good {
				return fmt.Errorf("hash mismatch at piece %d", i)
			}
			log.Tracef("verify-hash %d: %x: %v\n", i, p.Hash(), good)
		}
	}
	if contentPath != "" {
		fileStats, err := os.Stat(contentPath)
		if err == nil {
			if meta.SingleFileTorrent {
				if fileStats.Name() != meta.Files[0].Path {
					log.Warnf("This is single-file torrent. The torrent file on disk \"%s\" has same content with torrent info, "+
						"but they have DIFFERENT file name. Be careful if you would add this torrent to local client to xseed.",
						contentPath)
				}
			} else {
				if fileStats.Name() != meta.RootDir {
					log.Warnf("This is multiple-file torrent. The torrent content folder on disk \"%s\" has same contents with torrent info, "+
						"but they have DIFFERENT root folder name. Be careful if you would add this torrent to local client to xseed.",
						contentPath)
				}
			}
		}
	}
	return nil
}
