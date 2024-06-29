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
	"path/filepath"
	"slices"
	"strings"

	"github.com/anacrolix/torrent/bencode"
	"github.com/anacrolix/torrent/metainfo"
	"github.com/shibumi/go-pathspec"
	log "github.com/sirupsen/logrus"
	"golang.org/x/term"

	"github.com/sagan/ptool/client"
	"github.com/sagan/ptool/constants"
	"github.com/sagan/ptool/site/public"
	"github.com/sagan/ptool/site/tpl"
	"github.com/sagan/ptool/util"
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
	Info              *metainfo.Info
}

type TorrentMakeOptions struct {
	ContentPath                   string
	Output                        string
	Public                        bool
	Private                       bool
	All                           bool
	Force                         bool
	Comment                       string
	InfoName                      string
	UrlList                       metainfo.UrlList
	Trackers                      []string
	CreatedBy                     string
	CreationDate                  string
	PieceLengthStr                string
	MinSize                       int64
	Excludes                      []string
	AllowRestrictedCharInFilename bool
	// By default, limit filename length to at most 240 bytes (UTF-8).
	// It's the limit imposed by libtorrent on Linux.
	AllowLongName bool
}

var (
	ErrNoChange      = errors.New("no change made")
	ErrSmall         = errors.New("torrent contents is too small")
	ErrDifferentName = errors.New("this is single-file torrent. the torrent content file on disk " +
		"has same content with torrent meta, but they have DIFFERENT file name, " +
		"so it can not be directly added to client as xseed torrent")
	ErrDifferentRootName = errors.New("this is multiple-file torrent. the torrent content files on disk " +
		"has same contents with torrent meta, but they have DIFFERENT root folder name, " +
		"so it can not be directly added to client as xseed torrent")
)

func ParseTorrent(torrentdata []byte) (*TorrentMeta, error) {
	metaInfo, err := metainfo.Load(bytes.NewReader(torrentdata))
	if err != nil {
		return nil, err
	}
	return FromMetaInfo(metaInfo, nil)
}

func FromMetaInfo(metaInfo *metainfo.MetaInfo, info *metainfo.Info) (*TorrentMeta, error) {
	torrentMeta := &TorrentMeta{
		MetaInfo: metaInfo,
		Info:     info,
		InfoHash: metaInfo.HashInfoBytes().String(),
	}
	// [][]string, first index is tier: lower number has higher priority
	announceList := metaInfo.UpvertedAnnounceList()
	for _, al := range announceList {
		torrentMeta.Trackers = append(torrentMeta.Trackers, al...)
	}
	if torrentMeta.Info == nil {
		_info, err := metaInfo.UnmarshalInfo()
		if err != nil {
			return nil, err
		}
		torrentMeta.Info = &_info
	}
	info = torrentMeta.Info
	// single file torrent
	if len(info.Files) == 0 {
		torrentMeta.Files = append(torrentMeta.Files, TorrentMetaFile{
			// 个别站点的.torrent文件里的 files.path 字段包含不可见字符。保持与 qb 行为一致：直接忽略这些字符。
			// 例如： keepfrds.1684287 种子里有 \u200e (U+200E, LEFT-TO-RIGHT MARK)
			Path: util.Clean(info.Name),
			Size: info.Length,
		})
		torrentMeta.SingleFileTorrent = true
		torrentMeta.Size = info.Length
		torrentMeta.ContentPath = util.Clean(info.Name)
	} else {
		if info.Name != "" && info.Name != metainfo.NoName {
			torrentMeta.RootDir = util.Clean(info.Name)
			torrentMeta.ContentPath = util.Clean(info.Name)
		}
		for _, metafile := range info.Files {
			torrentMeta.Files = append(torrentMeta.Files, TorrentMetaFile{
				Path: util.Clean(strings.Join(metafile.Path, "/")),
				Size: metafile.Length,
			})
			torrentMeta.Size += metafile.Length
		}
	}
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
	err := json.Unmarshal([]byte(meta.MetaInfo.Comment), &commentMeta)
	if err != nil {
		return nil
	}
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

func (meta *TorrentMeta) UpdateCreationDate(creationDate int64) error {
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
	return meta.MetaInfo.Magnet(nil, meta.Info).String()
}

func (meta *TorrentMeta) Fprint(f io.Writer, name string, showAll bool) {
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
	fmt.Fprintf(f, "%s : infohash = %s ; size = %s (%d) ; root = %q ; tracker = %s%s\n", name, meta.InfoHash,
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
			fmt.Fprintf(f, "! SingleFile = %q ; ", meta.Files[0].Path)
		} else {
			fmt.Fprintf(f, "! RootDir = %q ; ", meta.RootDir)
		}
		fmt.Fprintf(f, "RawSize = %d ; PieceLength = %s ; CreationDate = %s ; AllTrackers (%d): %s ;%s\n",
			meta.Size, util.BytesSizeAround(float64(meta.Info.PieceLength)), creationDate, len(meta.Trackers),
			strings.Join(meta.Trackers, " | "), comment)
		if !meta.IsPrivate() {
			fmt.Fprintf(f, "! MagnetURI: %s\n", meta.MagnetUrl())
		}
	}
}

func (meta *TorrentMeta) FprintFiles(f io.Writer, addRootDirPrefix bool, useRawSize bool) {
	fmt.Fprintf(f, "Files:\n")
	for i, file := range meta.Files {
		path := file.Path
		if addRootDirPrefix && meta.RootDir != "" {
			path = meta.RootDir + "/" + path
		}
		if useRawSize {
			fmt.Fprintf(f, "%-5d  %-15d  %s\n", i+1, file.Size, path)
		} else {
			fmt.Fprintf(f, "%-5d  %-10s  %s\n", i+1, util.BytesSize(float64(file.Size)), path)
		}
	}
}

// return 0 if this torrent is equal with client torrent;
// return 1 if client torrent contains all files of this torrent.
// return -2 if the ROOT folder(file) of the two are different, but all innner files are SAME.
// return -1 if contents of the two torrents are NOT same.
func (meta *TorrentMeta) XseedCheckWithClientTorrent(clientTorrentContents []*client.TorrentContentFile) int64 {
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

func (meta *TorrentMeta) RootFiles() (rootFiles []string) {
	if meta.RootDir != "" {
		rootFiles = append(rootFiles, meta.RootDir)
	} else {
		existFlags := map[string]struct{}{}
		for _, file := range meta.Files {
			rootFile, _, _ := strings.Cut(file.Path, "/")
			if _, ok := existFlags[rootFile]; !ok {
				rootFiles = append(rootFiles, rootFile)
				existFlags[rootFile] = struct{}{}
			}
		}
	}
	return rootFiles
}

// Verify against a fs.FS of save path (e.g. os.DirFS("D:\Downloads")). It does no hash checking for now.
func (meta *TorrentMeta) VerifyAgaintSavePathFs(savePathFs fs.FS) error {
	relativePath := ""
	if meta.RootDir != "" {
		relativePath = meta.RootDir + "/"
	}
	for _, file := range meta.Files {
		filename := relativePath + file.Path
		f, err := savePathFs.Open(filename)
		if err != nil {
			return fmt.Errorf("failed to access file %q: %w", file.Path, err)
		}
		stat, err := f.Stat()
		if err != nil {
			return fmt.Errorf("failed to get file %q stat: %w", file.Path, err)
		}
		if stat.Size() != file.Size {
			return fmt.Errorf("file %q has wrong length: expect=%d, actual=%d", file.Path, file.Size, stat.Size())
		}
	}
	return nil
}

// checkHash: 0 - none; 1 - quick; 2+ - full.
// ts: timestamp of newest file in torrent contents.
func (meta *TorrentMeta) Verify(savePath string, contentPath string, checkHash int64) (ts int64, err error) {
	var filenames []string
	prefixPath := ""
	if contentPath != "" {
		contentPath, err = filepath.Abs(contentPath)
		if err != nil {
			return 0, fmt.Errorf("invalid content-path: %w", err)
		}
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
			return ts, fmt.Errorf("failed to get file %q stat: %w", file.Path, err)
		}
		ts = max(stat.ModTime().Unix(), ts)
		if stat.Size() != file.Size {
			return ts, fmt.Errorf("file %q has wrong length: expect=%d, actual=%d", file.Path, file.Size, stat.Size())
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
						return ts, fmt.Errorf("piece %d/%d: failed to open file %s: %w",
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
					return ts, err
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
				return ts, fmt.Errorf("piece %d/%d: hash mismatch", i, piecesCnt-1)
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
					return ts, ErrDifferentName
				}
			} else {
				if fileStats.Name() != meta.RootDir {
					return ts, ErrDifferentRootName
				}
			}
		}
	}
	return ts, nil
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

	replacerArgs := []string{"[id]", id, "[site]", sitename, "[filename]", basename,
		"[filename128]", util.StringPrefixInBytes(basename, 128)}
	if tinfo != nil {
		replacerArgs = append(replacerArgs, "[size]", util.BytesSize(float64(tinfo.Size)),
			"[name]", tinfo.Info.Name, "[name128]", util.StringPrefixInBytes(tinfo.Info.Name, 128))
	}
	newname = strings.NewReplacer(replacerArgs...).Replace(newname)
	newname = constants.FilenameRestrictedCharacterReplacer.Replace(newname)
	return newname
}

// Get appropriate filename for exported .torrent file.
// available variable placeholders: [client], [size], [infohash], [infohash16], [category], [name], [name128]
func RenameExportedTorrent(client string, torrent *client.Torrent, rename string) string {
	filename := rename
	filename = strings.NewReplacer("[client]", client, "[size]", util.BytesSize(float64(torrent.Size)),
		"[infohash]", torrent.InfoHash, "[infohash16]", torrent.InfoHash[:16], "[category]", torrent.Category,
		"[name]", torrent.Name, "[name128]", util.StringPrefixInBytes(torrent.Name, 128)).Replace(filename)
	filename = constants.FilenameRestrictedCharacterReplacer.Replace(filename)
	return filename
}

// Create a torrent, return info of created torrent.
// It may change the values of any fields in options.
func MakeTorrent(options *TorrentMakeOptions) (tinfo *TorrentMeta, err error) {
	mi := &metainfo.MetaInfo{
		AnnounceList: make([][]string, 0),
		Comment:      options.Comment,
		UrlList:      options.UrlList,
	}
	if options.Public {
		options.Trackers = append(options.Trackers, constants.OpenTrackers...)
		util.UniqueSlice(options.Trackers)
	}
	for _, a := range options.Trackers {
		mi.AnnounceList = append(mi.AnnounceList, []string{a})
	}
	if len(options.Trackers) > 0 {
		mi.Announce = options.Trackers[0]
	}
	mi.SetDefaults()
	if options.CreatedBy != "" {
		if options.CreatedBy == constants.NONE {
			mi.CreatedBy = ""
		} else {
			mi.CreatedBy = options.CreatedBy
		}
	}
	if options.CreationDate != "" {
		if options.CreationDate == constants.NONE {
			mi.CreationDate = 0
		} else {
			ts, err := util.ParseTime(options.CreationDate, nil)
			if err != nil {
				return nil, fmt.Errorf("invalid creation-date: %w", err)
			}
			mi.CreationDate = ts
		}
	}
	info := &metainfo.Info{}
	if pieceLength, err := util.RAMInBytes(options.PieceLengthStr); err != nil {
		return nil, fmt.Errorf("invalid piece-length: %w", err)
	} else {
		info.PieceLength = pieceLength
	}
	if options.Private {
		private := true
		info.Private = &private
	}
	if !options.All {
		options.Excludes = append(options.Excludes, constants.DefaultIgnorePatterns...)
	}
	log.Infof("Creating torrent for %q", options.ContentPath)
	if err := infoBuildFromFilePath(info, options.ContentPath, options.Excludes,
		options.AllowRestrictedCharInFilename, options.AllowLongName); err != nil {
		return nil, fmt.Errorf("failed to build info from content-path: %w", err)
	}
	if len(info.Files) == 0 {
		return nil, fmt.Errorf("no files found in content-path")
	}
	if options.MinSize > 0 {
		size := int64(0)
		for _, file := range info.Files {
			size += file.Length
		}
		if size < options.MinSize {
			return nil, ErrSmall
		}
	}
	if options.InfoName != "" {
		info.Name = options.InfoName
	}
	if mi.InfoBytes, err = bencode.Marshal(info); err != nil {
		return nil, fmt.Errorf("failed to marshal info: %w", err)
	}
	if options.Output == "" {
		if info.Name != "" && info.Name != metainfo.NoName {
			options.Output = info.Name + ".torrent"
		} else {
			log.Warnf("The created torrent has NO root folder, use it's info-hash as output file name")
			options.Output = mi.HashInfoBytes().String() + ".torrent"
		}
	}
	log.Infof("Output to %q", options.Output)
	if options.Output == "-" {
		if term.IsTerminal(int(os.Stdout.Fd())) {
			err = fmt.Errorf(constants.HELP_TIP_TTY_BINARY_OUTPUT)
		} else {
			err = mi.Write(os.Stdout)
		}
	} else {
		if !options.Force && util.FileExists(options.Output) {
			err = fmt.Errorf(`output file %q already exists. use "--force" to overwrite`, options.Output)
		} else {
			var outputFile *os.File
			if outputFile, err = os.OpenFile(options.Output, os.O_TRUNC|os.O_CREATE|os.O_WRONLY, constants.PERM); err == nil {
				defer outputFile.Close()
				err = mi.Write(outputFile)
			}
		}
	}
	if err != nil {
		return nil, err
	}
	tinfo, err = FromMetaInfo(mi, info)
	if err != nil {
		return nil, err
	}
	return tinfo, err
}

// Adapted from metainfo.BuildFromFilePath.
// excludes: gitignore style exclude-file-patterns.
func infoBuildFromFilePath(info *metainfo.Info, root string, excludes []string,
	allowAnyCharInName bool, allowLongName bool) (err error) {
	info.Name = func() string {
		b := filepath.Base(root)
		switch b {
		case ".", "..", string(filepath.Separator):
			return metainfo.NoName
		default:
			return b
		}
	}()
	info.Files = nil
	err = filepath.Walk(root, func(path string, fi os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if len(excludes) > 0 {
			if relativePath, err := filepath.Rel(root, path); err == nil && relativePath != "." {
				if ignore, _ := pathspec.GitIgnore(excludes, relativePath); ignore {
					log.Tracef("Ignore %s", relativePath)
					if fi.IsDir() {
						return filepath.SkipDir
					} else {
						return nil
					}
				}
			}
		}
		if !allowLongName && len(fi.Name()) > constants.TORRENT_CONTENT_FILENAME_LENGTH_LIMIT {
			return fmt.Errorf("filename %q is too long (%d bytes in UTF-8). Consider truncate it to %q", fi.Name(),
				len(fi.Name()), util.StringPrefixInBytes(fi.Name(), constants.TORRENT_CONTENT_FILENAME_LENGTH_LIMIT))
		}
		if fi.IsDir() {
			// Directories are implicit in torrent files.
			return nil
		} else if path == root {
			// The root is a file.
			info.Length = fi.Size()
			return nil
		}
		relPath, err := filepath.Rel(root, path)
		if err != nil {
			return fmt.Errorf("error getting relative path: %s", err)
		}
		if !allowAnyCharInName && constants.FilepathInvalidCharsRegex.MatchString(relPath) {
			return fmt.Errorf("invalid content file path %q: contains restrictive chars", relPath)
		}
		info.Files = append(info.Files, metainfo.FileInfo{
			Path:   strings.Split(relPath, string(filepath.Separator)),
			Length: fi.Size(),
		})
		return nil
	})
	if err != nil {
		return
	}
	slices.SortStableFunc(info.Files, func(l, r metainfo.FileInfo) int {
		lp := strings.Join(l.Path, "/")
		rp := strings.Join(r.Path, "/")
		if lp < rp {
			return -1
		} else if lp > rp {
			return 1
		}
		return 0
	})
	if info.PieceLength == 0 {
		info.PieceLength = metainfo.ChoosePieceLength(info.TotalLength())
	}
	err = info.GeneratePieces(func(fi metainfo.FileInfo) (io.ReadCloser, error) {
		return os.Open(filepath.Join(root, strings.Join(fi.Path, string(filepath.Separator))))
	})
	if err != nil {
		err = fmt.Errorf("error generating pieces: %s", err)
	}
	return
}
