package torrentutil

import (
	"bytes"
	"crypto/sha1"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"strconv"
	"strings"

	"github.com/anacrolix/torrent/metainfo"
	"github.com/sagan/ptool/client"
	"github.com/sagan/ptool/constants"
	"github.com/sagan/ptool/site/public"
	"github.com/sagan/ptool/site/tpl"
	"github.com/sagan/ptool/util"
	log "github.com/sirupsen/logrus"
)

type TorrentCommentMeta struct {
	Category string   `json:"category,omitempty"`
	Tags     []string `json:"tags,omitempty"`
	Comment  string   `json:"comment,omitempty"`
	SavePath string   `json:"save_path,omitempty"`
}

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
	ContentPath       string // root folder or single file name
	Files             []TorrentMetaFile
	MetaInfo          *metainfo.MetaInfo
	Info              metainfo.Info
}

var (
	ErrNoChange = errors.New("no change made")
)

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
	// [][]string, first index is tier: lower number has higher priority
	announceList := metaInfo.UpvertedAnnounceList()
	for _, al := range announceList {
		torrentMeta.Trackers = append(torrentMeta.Trackers, al...)
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
		torrentMeta.ContentPath = info.Name
	} else {
		if info.Name != "" && info.Name != metainfo.NoName {
			torrentMeta.RootDir = info.Name
			torrentMeta.ContentPath = info.Name
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

// Encode torrent meta to 'comment' field
func (meta *TorrentMeta) EncodeComment(commentMeta *TorrentCommentMeta) error {
	comment := ""
	if existingCommentMeta := meta.DecodeComment(); existingCommentMeta != nil {
		comment = existingCommentMeta.Comment
	} else {
		comment = meta.MetaInfo.Comment
	}
	commentMeta.Comment = comment
	data, err := json.Marshal(commentMeta)
	if err != nil {
		return err
	}
	meta.MetaInfo.Comment = string(data)
	return nil
}

// Decode torrent meta from 'comment' field
func (meta *TorrentMeta) DecodeComment() *TorrentCommentMeta {
	var commentMeta *TorrentCommentMeta
	json.Unmarshal([]byte(meta.MetaInfo.Comment), &commentMeta)
	return commentMeta
}

func (meta *TorrentMeta) IsPrivate() bool {
	return meta.Info.Private != nil && *meta.Info.Private
}

func (meta *TorrentMeta) UpdateCreatedBy(createdBy string) error {
	if meta.MetaInfo.CreatedBy == createdBy {
		return ErrNoChange
	}
	meta.MetaInfo.CreatedBy = createdBy
	return nil
}

func (meta *TorrentMeta) UpdateComment(comment string) error {
	if meta.MetaInfo.Comment == comment {
		return ErrNoChange
	}
	meta.MetaInfo.Comment = comment
	return nil
}

func (meta *TorrentMeta) UpdateCreationDate(creationDateStr string) error {
	creationDate := int64(0)
	if util.IsIntString(creationDateStr) {
		if i, err := strconv.Atoi(creationDateStr); err != nil {
			return err
		} else {
			creationDate = int64(i)
		}
	} else {
		if time, err := util.ParseTime(creationDateStr, nil); err != nil {
			return err
		} else {
			creationDate = time
		}
	}
	if meta.MetaInfo.CreationDate == creationDate {
		return ErrNoChange
	}
	meta.MetaInfo.CreationDate = creationDate
	return nil
}

// Remove all existing trackers and set the provided one as the sole tracker.
func (meta *TorrentMeta) UpdateTracker(tracker string) error {
	if tracker == "" {
		return ErrNoChange
	}
	hasOtherTracker := false
outer:
	for _, al := range meta.MetaInfo.AnnounceList {
		for _, a := range al {
			if a != tracker {
				hasOtherTracker = true
				break outer
			}
		}
	}
	if meta.MetaInfo.Announce == tracker && !hasOtherTracker {
		return ErrNoChange
	}
	meta.MetaInfo.Announce = tracker
	meta.MetaInfo.AnnounceList = nil
	return nil
}

// Add a tracker to AnnounceList at specified tier.
// Do not add the tracker if it already exists somewhere in AnnounceList.
// tier == -1: create a new tier to the end of AnnounceList and put new tracker here.
func (meta *TorrentMeta) AddTracker(tracker string, tier int) error {
	if tracker == "" || meta.MetaInfo.Announce == tracker {
		return ErrNoChange
	}
	for _, al := range meta.MetaInfo.AnnounceList {
		for _, a := range al {
			if a == tracker {
				return ErrNoChange // tracker already exists
			}
		}
	}
	if len(meta.MetaInfo.AnnounceList) == 0 && meta.MetaInfo.Announce != "" {
		meta.MetaInfo.AnnounceList = append(meta.MetaInfo.AnnounceList, []string{meta.MetaInfo.Announce})
	}
	createNewTier := tier < 0 || tier >= len(meta.MetaInfo.AnnounceList)
	var trackersTier []string
	if !createNewTier {
		trackersTier = meta.MetaInfo.AnnounceList[tier]
	}
	trackersTier = append(trackersTier, tracker)
	if createNewTier {
		meta.MetaInfo.AnnounceList = append(meta.MetaInfo.AnnounceList, trackersTier)
	} else {
		meta.MetaInfo.AnnounceList[tier] = trackersTier
	}
	if meta.MetaInfo.Announce == "" {
		meta.MetaInfo.Announce = tracker
	}
	return nil
}

func (meta *TorrentMeta) RemoveTracker(tracker string) error {
	if tracker == "" {
		return ErrNoChange
	}
	changed := false
	if meta.MetaInfo.Announce == tracker {
		meta.MetaInfo.Announce = ""
		changed = true
	}
outer:
	for i, al := range meta.MetaInfo.AnnounceList {
		for j, a := range al {
			if a == tracker {
				// this is really ugly...
				var newTier []string
				newTier = append(newTier, al[:j]...)
				newTier = append(newTier, al[j+1:]...)
				if len(newTier) > 0 {
					meta.MetaInfo.AnnounceList[i] = newTier
				} else {
					var nal [][]string
					nal = append(nal, meta.MetaInfo.AnnounceList[:i]...)
					nal = append(nal, meta.MetaInfo.AnnounceList[i+1:]...)
					meta.MetaInfo.AnnounceList = nal
				}
				changed = true
				break outer
			}
		}
	}
	if !changed {
		return ErrNoChange
	}
	return nil
}

// Generate .torrent file from current content
func (meta *TorrentMeta) ToBytes() ([]byte, error) {
	buf := bytes.NewBuffer(nil)
	if err := meta.MetaInfo.Write(buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// Generate magnet: url of this torrent.
// Must be used on meta parsed from ParseTorrent with fields >= 2
func (meta *TorrentMeta) MagnetUrl() string {
	return meta.MetaInfo.Magnet(nil, &meta.Info).String()
}

func (meta *TorrentMeta) Print(name string, showAll bool) {
	trackerUrl := ""
	if len(meta.Trackers) > 0 {
		trackerUrl = meta.Trackers[0]
	}

	sitenameStr := ""
	var err error
	if sitenameStr, err = tpl.GuessSiteByTrackers(meta.Trackers, ""); sitenameStr != "" {
		sitenameStr = fmt.Sprintf(" (site: %s)", sitenameStr)
	} else if err != nil {
		log.Warnf("Failed to find match site for %s by trackers: %v", name, err)
	} else if site := public.GetSiteByDomain("", meta.Trackers...); site != nil {
		sitenameStr = fmt.Sprintf(" (site: %s)", site.Name)
	}
	rootFile := "" // root folder or single content file
	if meta.SingleFileTorrent {
		rootFile = meta.Files[0].Path
	} else if meta.RootDir != "" {
		rootFile = meta.RootDir + "/"
	}
	fmt.Printf("%s : infohash = %s ; size = %s (%d) ; root = %q ; tracker = %s%s\n", name, meta.InfoHash,
		util.BytesSize(float64(meta.Size)), len(meta.Files), rootFile, trackerUrl, sitenameStr)
	if showAll {
		comments := []string{}
		if meta.MetaInfo.Comment != "" {
			comments = append(comments, meta.MetaInfo.Comment)
		}
		if meta.IsPrivate() {
			comments = append(comments, "private")
		}
		if meta.Info.Source != "" {
			comments = append(comments, fmt.Sprintf("source:%q", meta.Info.Source))
		}
		if meta.MetaInfo.CreatedBy != "" {
			comments = append(comments, fmt.Sprintf("created_by:%q", meta.Info.Source))
		}
		creationDate := "-"
		if meta.MetaInfo.CreationDate > 0 {
			creationDate = fmt.Sprintf("%q (%d)", util.FormatTime(meta.MetaInfo.CreationDate), meta.MetaInfo.CreationDate)
		}
		comment := ""
		if len(comments) > 0 {
			comment = " // " + strings.Join(comments, ", ")
		}
		if meta.SingleFileTorrent {
			fmt.Printf("! RawSize = %d ; SingleFile = %s ; CreationDate = %s ; AllTrackers: %s ;%s\n",
				meta.Size, meta.Files[0].Path, creationDate, strings.Join(meta.Trackers, " | "), comment)
		} else {
			fmt.Printf("! RawSize = %d ; RootDir = %s ; CreationDate = %s ; AllTrackers: %s ;%s\n",
				meta.Size, meta.RootDir, creationDate, strings.Join(meta.Trackers, " | "), comment)
		}
		if !meta.IsPrivate() {
			fmt.Printf("! MagnetURI: %s\n", meta.MagnetUrl())
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
			fmt.Printf("%-5d  %-10s  %s\n", i+1, util.BytesSize(float64(file.Size)), path)
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
	clientFilesSizeMap := map[string]int64{}

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

// Verify against a fs.FS of save path (e.g.: os.DirFS("D:\Downloads")). It does no hash checking for now.
func (meta *TorrentMeta) VerifyAgaintSavePathFs(savePathFs fs.FS) error {
	relativePath := ""
	if meta.RootDir != "" {
		relativePath = meta.RootDir + "/"
	}
	for _, file := range meta.Files {
		filename := relativePath + file.Path
		f, err := savePathFs.Open(filename)
		if err != nil {
			return fmt.Errorf("failed to access file %q: %v", file.Path, err)
		}
		stat, err := f.Stat()
		if err != nil {
			return fmt.Errorf("failed to get file %q stat: %v", file.Path, err)
		}
		if stat.Size() != file.Size {
			return fmt.Errorf("file %q has wrong length: expect=%d, actual=%d", file.Path, file.Size, stat.Size())
		}
	}
	return nil
}

// checkHash: 0 - none; 1 - quick; 2+ - full.
func (meta *TorrentMeta) Verify(savePath string, contentPath string, checkHash int64) error {
	var filenames []string
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
		stat, err := os.Stat(filename)
		if err != nil {
			return fmt.Errorf("failed to get file %q stat: %v", file.Path, err)
		}
		if stat.Size() != file.Size {
			return fmt.Errorf("file %q has wrong length: expect=%d, actual=%d", file.Path, file.Size, stat.Size())
		}
		if checkHash > 0 {
			filenames = append(filenames, filename)
		}
	}
	if checkHash > 0 && len(meta.Files) > 0 {
		piecesCnt := meta.Info.NumPieces()
		var currentFileIndex = int64(0)
		var currentFileOffset = int64(0)
		var currentFileRemain = int64(0)
		var currentFile *os.File
		var err error
		i := 0
		for {
			if i >= piecesCnt {
				break
			}
			if checkHash == 1 && currentFile != nil && currentFileRemain > meta.Info.PieceLength {
				skipPieces := currentFileRemain / meta.Info.PieceLength
				skipLength := skipPieces * meta.Info.PieceLength
				currentFileOffset += skipLength
				currentFileRemain -= skipLength
				i += int(skipPieces)
			}
			p := meta.Info.Piece(i)
			hash := sha1.New()
			len := p.Length()
			for len > 0 {
				if currentFile == nil {
					if currentFile, err = os.Open(filenames[currentFileIndex]); err != nil {
						return fmt.Errorf("piece %d/%d: failed to open file %s: %v",
							i, piecesCnt-1, filenames[currentFileIndex], err)
					}
					log.Tracef("piece %d/%d: open file %s", i, piecesCnt-1, filenames[currentFileIndex])
					currentFileOffset = 0
					currentFileRemain = meta.Files[currentFileIndex].Size
				}
				readlen := min(currentFileRemain, len)
				_, err := io.Copy(hash, io.NewSectionReader(currentFile, currentFileOffset, readlen))
				if err != nil {
					currentFile.Close()
					return err
				}
				currentFileOffset += readlen
				currentFileRemain -= readlen
				len -= readlen
				if currentFileRemain == 0 {
					currentFile.Close()
					currentFile = nil
					currentFileIndex++
				}
			}
			good := bytes.Equal(hash.Sum(nil), p.Hash().Bytes())
			if !good {
				return fmt.Errorf("piece %d/%d: hash mismatch", i, piecesCnt-1)
			}
			log.Tracef("piece %d/%d verify-hash %x: %v", i, piecesCnt-1, p.Hash(), good)
			i++
		}
	}
	if contentPath != "" {
		fileStats, err := os.Stat(contentPath)
		if err == nil {
			if meta.SingleFileTorrent {
				if fileStats.Name() != meta.Files[0].Path {
					log.Warnf(`This is single-file torrent. The torrent content file on disk "%s" `+
						"has same content with torrent meta, but they have DIFFERENT file name. "+
						"Be careful if you would add this torrent to client to xseed.", contentPath)
				}
			} else {
				if fileStats.Name() != meta.RootDir {
					log.Warnf(`This is multiple-file torrent. The torrent content folder on disk "%s" `+
						"has same contents with torrent meta, but they have DIFFERENT root folder name. "+
						"Be careful if you would add this torrent to client to xseed.", contentPath)
				}
			}
		}
	}
	return nil
}

// Rename torrent (downloaded filename or name of torrent added to client) according to rename template.
// filename: original torrent filename (e.g. "abc.torrent").
// available variable placeholders: [size], [id], [site], [filename], [filename128], [name], [name128].
// tinfo is optional could may be nil.
func RenameTorrent(rename string, sitename string, id string, filename string, tinfo *TorrentMeta) string {
	newname := rename
	basename := filename
	if i := strings.LastIndex(basename, "."); i != -1 {
		basename = basename[:i]
	}
	// id is "mteam.1234" style format
	if i := strings.Index(id, "."); i != -1 {
		sitename = id[:i]
		id = id[i+1:]
	}

	newname = strings.ReplaceAll(newname, "[id]", id)
	newname = strings.ReplaceAll(newname, "[site]", sitename)
	newname = strings.ReplaceAll(newname, "[filename]", basename)
	newname = strings.ReplaceAll(newname, "[filename128]", util.StringPrefixInBytes(basename, 128))
	if tinfo != nil {
		newname = strings.ReplaceAll(newname, "[size]", util.BytesSize(float64(tinfo.Size)))
		newname = strings.ReplaceAll(newname, "[name]", tinfo.Info.Name)
		newname = strings.ReplaceAll(newname, "[name128]", util.StringPrefixInBytes(tinfo.Info.Name, 128))
	}
	newname = constants.FilenameInvalidCharsRegex.ReplaceAllString(newname, "")
	return newname
}

// Get appropriate filename for exported .torrent file.
// available variable placeholders: [client], [size], [infohash], [infohash16], [category], [name], [name128]
func RenameExportedTorrent(client string, torrent client.Torrent, rename string) string {
	filename := rename
	filename = strings.ReplaceAll(filename, "[client]", client)
	filename = strings.ReplaceAll(filename, "[size]", util.BytesSize(float64(torrent.Size)))
	filename = strings.ReplaceAll(filename, "[infohash]", torrent.InfoHash)
	filename = strings.ReplaceAll(filename, "[infohash16]", torrent.InfoHash[:16])
	filename = strings.ReplaceAll(filename, "[category]", torrent.Category)
	filename = strings.ReplaceAll(filename, "[name]", torrent.Name)
	filename = strings.ReplaceAll(filename, "[name128]", util.StringPrefixInBytes(torrent.Name, 128))
	filename = constants.FilenameInvalidCharsRegex.ReplaceAllString(filename, "")
	return filename
}
