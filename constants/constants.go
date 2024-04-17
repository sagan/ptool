// golang do NOT allow non-primitive constants so some values are defined as variables.
// However, all variables in this package never change in runtime.
package constants

import (
	"regexp"
	"time"
)

const FILENAME_INVALID_CHARS_REGEX = `[<>:"/\|\?\*]+`
const PERM = 0600 // 程序创建的所有文件的 PERM

const FILENAME_SUFFIX_ADDED = ".added"
const FILENAME_SUFFIX_OK = ".ok"
const FILENAME_SUFFIX_FAIL = ".fail"
const FILENAME_SUFFIX_BACKUP = ".bak"

// Some funcs require a (positive) timeout parameter. Use a very long value to emulate infinite. (Seconds)
const INFINITE_TIMEOUT = 86400 * 365 * 100

const BIG_FILE_SIZE = 10 * 1024 * 1024 // 10MiB
const FILE_HEADER_CHUNK_SIZE = 512

var FilenameInvalidCharsRegex = regexp.MustCompile(FILENAME_INVALID_CHARS_REGEX)

// .torrent file magic number.
// See: https://en.wikipedia.org/wiki/Torrent_file , https://en.wikipedia.org/wiki/Bencode .
// 大部分 .torrent 文件第一个字段是 announce，
// 个别种子没有 announce / announce-list 字段，第一个字段是 created by / creation date 等，
// 这类种子可以通过 DHT 下载成功。
// values: ["d8:announce", "d10:created by", "d13:creation date"]
var TorrentFileMagicNumbers = []string{"d8:announce", "d10:created by", "d13:creation date"}

// Some ptool cmds could add a suffix to processed (torrent) filenames.
// Current Values: [".added", ".ok", ".fail", ".bak"].
var ProcessedFilenameSuffixes = []string{
	FILENAME_SUFFIX_ADDED,
	FILENAME_SUFFIX_OK,
	FILENAME_SUFFIX_FAIL,
	FILENAME_SUFFIX_BACKUP,
}

// Unix zero time (00:00:00 UTC on 1 January 1970)
var UnixEpoch, _ = time.Parse("2006-01-02", "1970-01-01")
