package rclone

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/sagan/ptool/constants"
)

// The format of each item that "rclone lsjson <root_path>" output.
// See https://rclone.org/commands/rclone_lsjson/ .
// Some fields that rclone outputs are commented out as ptool does NOT use them.
// *LsjsonItem implements: fs.DirEntry, fs.FileInfo, fs.ReadDirFS, fs.File.
// The fs.File implementation only supports Stat() action.
// Some json field names (e.g. "Size") are used by DirEntry / FileInfo /... interfaces,
// so they are renamed to with "Item" prefix.
type LsjsonItem struct {
	ItemIsDir   bool                   `json:"IsDir,omitempty"`
	ItemModTime string                 `json:"ModTime,omitempty"` // "2017-05-31T16:15:57.034468261+01:00"
	ItemName    string                 `json:"Name,omitempty"`    // "file.txt"
	Path        string                 `json:"Path,omitempty"`    //"full/path/file.txt". Relative to <root_path>
	ItemSize    int64                  `json:"Size,omitempty"`
	children    map[string]*LsjsonItem // set by ptool

	// ID            string            `json:"ID,omitempty"`
	// MimeType      string            `json:"MimeType,omitempty"`
	// hashes        map[string]string `json:"Hashes,omitempty"` // key: "SHA-1", "MD5".... value: hex string
	// isBucket      bool              `json:"IsBucket,omitempty"`
	// encrypted     string            `json:"Encrypted,omitempty"`     // encrypted Name
	// encryptedPath string            `json:"EncryptedPath,omitempty"` // encrypted Path
	// tier          string            `json:"Tier,omitempty"`          // "hot"
}

// Close implements fs.File.
func (l *LsjsonItem) Close() error {
	return nil
}

// Read implements fs.File.
func (l *LsjsonItem) Read([]byte) (int, error) {
	return 0, ErrNotImplemented
}

// Stat implements fs.File.
func (l *LsjsonItem) Stat() (fs.FileInfo, error) {
	return l, nil
}

var (
	ErrNotImplemented = errors.New("not implemented")
	timeFormats       = []string{"2006-01-02T15:04:05Z", "2006-01-02T15:04:05-07:00"}
)

// ModTime implements fs.FileInfo.
func (l *LsjsonItem) ModTime() time.Time {
	for _, format := range timeFormats {
		if t, err := time.Parse(format, l.ItemModTime); err == nil {
			return t
		}
	}
	return time.Unix(0, 0)
}

// Mode implements fs.FileInfo.
func (l *LsjsonItem) Mode() (fm fs.FileMode) {
	if l.ItemIsDir {
		fm |= fs.ModeDir
	}
	fm |= constants.PERM
	return fm
}

// Size implements fs.FileInfo.
func (l *LsjsonItem) Size() int64 {
	return l.ItemSize
}

// Sys implements fs.FileInfo.
func (l *LsjsonItem) Sys() any {
	return nil
}

// Info implements fs.DirEntry.
func (l *LsjsonItem) Info() (fs.FileInfo, error) {
	return l, nil
}

// IsDir implements fs.DirEntry.
func (l *LsjsonItem) IsDir() bool {
	return l.ItemIsDir
}

// Name implements fs.DirEntry.
func (l *LsjsonItem) Name() string {
	return l.ItemName
}

// Type implements fs.DirEntry.
func (l *LsjsonItem) Type() fs.FileMode {
	return l.Mode()
}

// Open implements fs.ReadDirFS.
func (l *LsjsonItem) Open(name string) (fs.File, error) {
	if item, err := l.getItemByPath(name); err != nil {
		return nil, err
	} else {
		return item, nil
	}
}

// Get item by relative path. name is the full relative path (e.g. "foo/bar.txt").
// If relativePath is empty, return self item.
func (l *LsjsonItem) getItemByPath(relativePath string) (*LsjsonItem, error) {
	item := l
	if relativePath == "" {
		return item, nil
	}
	pathes := strings.Split(relativePath, "/")
	for _, path := range pathes {
		if item.children == nil || item.children[path] == nil {
			return nil, fs.ErrNotExist
		}
		item = item.children[path]
	}
	return item, nil
}

// ReadDir implements fs.ReadDirFS.
func (l *LsjsonItem) ReadDir(name string) (entries []fs.DirEntry, err error) {
	if dirItem, err := l.getItemByPath(name); err != nil {
		return nil, err
	} else {
		for _, child := range dirItem.children {
			entries = append(entries, child)
		}
		return entries, nil
	}
}

// Get a (virtual) fs.ReadDirFS from the json file that is outputed by
// "rclone lsjson" (https://rclone.org/commands/rclone_lsjson/).
// It's expected to be the output of "rclone lsjson --recursive remote:<root_path>".
// "rclone lsjson" output a flatten list of all items in specified path, possibly recursivly,
// in no order (however, parent dir item are guaranteed to be before child items).
func GetFsFromRcloneLsjsonResult(lsjsonOutput []byte) (fs.ReadDirFS, error) {
	var items []*LsjsonItem
	if err := json.Unmarshal(lsjsonOutput, &items); err != nil {
		return nil, fmt.Errorf("failed to parse json: %w", err)
	}
	rootItem := &LsjsonItem{}
	for i, item := range items {
		if item == nil {
			return nil, fmt.Errorf("invalid lsjson output: index %d is null", i)
		}
		relativePath := ""
		if i := strings.LastIndex(item.Path, "/"); i != -1 {
			relativePath = item.Path[:i]
		}
		parentItem, err := rootItem.getItemByPath(relativePath)
		if err != nil {
			return nil, fmt.Errorf("invalid lsjson output: failed to get parent of index %d item: %w", i, err)
		}
		if parentItem.children == nil {
			parentItem.children = map[string]*LsjsonItem{}
		}
		parentItem.children[item.ItemName] = item
	}
	log.Tracef("GetFsFromRcloneLsjsonResult root item has %d childlen", len(rootItem.children))
	return rootItem, nil
}

var _ fs.ReadDirFS = (*LsjsonItem)(nil)
var _ fs.DirEntry = (*LsjsonItem)(nil)
var _ fs.FileInfo = (*LsjsonItem)(nil)
var _ fs.File = (*LsjsonItem)(nil)
