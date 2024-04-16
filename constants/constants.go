// golang do NOT allow non-primitive constants so some values are defined as variables.
// However, all variables in this package never change in runtime.
package constants

import (
	"regexp"
	"time"
)

// .torrent file magic number.
// See: https://en.wikipedia.org/wiki/Torrent_file , https://en.wikipedia.org/wiki/Bencode .
const TORRENT_FILE_MAGIC_NUMBER = "d8:announce"

// 个别种子没有 announce / announce-list 字段，第一个字段是 creation date。这类种子可以通过 DHT 下载成功。
const TORRENT_FILE_MAGIC_NUMBER2 = "d13:creation date"

const FILENAME_INVALID_CHARS_REGEX = `[<>:"/\|\?\*]+`
const PERM = 0600 // 程序创建的所有文件的 PERM

const FILENAME_SUFFIX_ADDED = ".added"
const FILENAME_SUFFIX_OK = ".ok"
const FILENAME_SUFFIX_FAIL = ".fail"

// Some funcs require a (positive) timeout parameter. Use a very long value to emulate infinite. (Seconds)
const INFINITE_TIMEOUT = 86400 * 365 * 100

var FilenameInvalidCharsRegex = regexp.MustCompile(FILENAME_INVALID_CHARS_REGEX)

// Some ptool cmds could add a suffix to processed (torrent) filenames.
// Current Values: [".added", ".ok", ".fail"].
var ProcessedFilenameSuffixes = []string{
	FILENAME_SUFFIX_ADDED,
	FILENAME_SUFFIX_OK,
	FILENAME_SUFFIX_FAIL,
}

// Unix zero time (00:00:00 UTC on 1 January 1970)
var UnixEpoch, _ = time.Parse("2006-01-02", "1970-01-01")
