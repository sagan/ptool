// golang do NOT allow non-primitive constants so some values are defined as variables.
// However, all variables in this package never change in runtime.
package constants

import (
	"fmt"
	"regexp"
	"strings"
)

// 如果 ptool.toml 配置文件里字符串类型配置项值为空，使用系统默认值；使用 NONE 值显式设置该配置项为空值。
// 部分 flag 参数使用 NONE 值显式指定为空值。
const NONE = "none"

// a special proxy value to indicate force use proxy from HTTP(S)_PROXY env.
const ENV_PROXY = "env"

// Invalid characters of file name / file path in Windows + NTFS.
// Most of theses chars (except '/') are valid in Linux.
const FILENAME_INVALID_CHARS_REGEX = `[<>:"/\|\?\*]+`
const FILEPATH_INVALID_CHARS_REGEX = `[<>:"|\?\*]+`

// file created for testing dir permissions & accessibility
const TEST_FILE = ".ptool-test"

const PERM = 0600     // 程序创建的所有文件的 PERM
const PERM_DIR = 0700 // 程序创建的所有文件夹的 PERM

const FILENAME_SUFFIX_ADDED = ".added"
const FILENAME_SUFFIX_OK = ".ok"
const FILENAME_SUFFIX_FAIL = ".fail"
const FILENAME_SUFFIX_BACKUP = ".bak"

// Some funcs require a (positive) timeout parameter. Use a very long value to emulate infinite. (Seconds)
const INFINITE_TIMEOUT = 86400 * 365 * 100

const BIG_FILE_SIZE = 10 * 1024 * 1024 // 10MiB
const FILE_HEADER_CHUNK_SIZE = 512
const INFINITE_SIZE = 1024 * 1024 * 1024 * 1024 * 1024 * 1024 // 1EiB

const CLIENT_DEFAULT_DOWNLOADING_SPEED_LIMIT = 300 * 1024 * 1024 / 8 // BT客户端默认下载速度上限：300Mbps

// Longer names in torrent will be truncated by libtorrent / qBottorrent, which could cause problems.
// See: https://github.com/qbittorrent/qBittorrent/issues/7038 .
const TORRENT_CONTENT_FILENAME_LENGTH_LIMIT = 240
const TORRENT_DEFAULT_PIECE_LENGTH = "16MiB"
const META_TORRENT_FILE = ".torrent"
const METADATA_FILE = "metadata.nfo"
const METADATA_KEY_ARRAY_KEYS = "_array_keys"
const METADATA_KEY_DRY_RUN = "_dryrun"

// type, name, ↑info, ↓info, others
const STATUS_FMT = "%-6s  %-15s  %-27s  %-27s  %-s\n"

var FilenameInvalidCharsRegex = regexp.MustCompile(FILENAME_INVALID_CHARS_REGEX)

var FilepathInvalidCharsRegex = regexp.MustCompile(FILEPATH_INVALID_CHARS_REGEX)

// It's a subset of https://rclone.org/overview/#restricted-filenames-caveats .
// Only include invalid filename characters in Windows (NTFS).
var FilepathRestrictedCharacterReplacement = map[rune]rune{
	'*': '＊',
	':': '：',
	'<': '＜',
	'>': '＞',
	'|': '｜',
	'?': '？',
	'"': '＂',
}

var FilenameRestrictedCharacterReplacement = map[rune]rune{
	'/':  '／',
	'\\': '＼',
}

// Replace invalid Windows filename chars to alternatives. E.g. '/' => '／', 	'?' => '？'
var FilenameRestrictedCharacterReplacer *strings.Replacer

// Replace invalid Windows file path chars to alternatives.
// Similar to FilenameRestrictedCharacterReplacer, but do not replace '/' or '\'.
var FilepathRestrictedCharacterReplacer *strings.Replacer

// .torrent file magic number.
// See: https://en.wikipedia.org/wiki/Torrent_file , https://en.wikipedia.org/wiki/Bencode .
// 大部分 .torrent 文件第一个字段是 announce，
// 个别种子没有 announce / announce-list 字段，第一个字段是 created by / creation date 等，
// 这类种子可以通过 DHT 下载成功。
// values: ["d8:announce", "d10:created by", "d13:creation date"]
var TorrentFileMagicNumbers = []string{"d8:announce", "d13:announce-list", "d10:created by", "d13:creation date"}

// Some ptool cmds could add a suffix to processed (torrent) filenames.
// Current Values: [".added", ".ok", ".fail", ".bak"].
var ProcessedFilenameSuffixes = []string{
	FILENAME_SUFFIX_ADDED,
	FILENAME_SUFFIX_OK,
	FILENAME_SUFFIX_FAIL,
	FILENAME_SUFFIX_BACKUP,
}

var ImgExts = []string{".webp", ".png", ".jpg", ".jpeg"}

// Sources:
// https://github.com/nyaadevs/nyaa/blob/master/trackers.txt ,
// https://github.com/ngosang/trackerslist/blob/master/trackers_best.txt .
// Only include most popular & stable trackers in this list.
var OpenTrackers = []string{
	"udp://open.stealth.si:80/announce",
	"udp://tracker.opentrackr.org:1337/announce",
	"udp://exodus.desync.com:6969/announce",
	"udp://open.demonii.com:1337/announce",      // At least since 2014
	"udp://tracker.torrent.eu.org:451/announce", // Since 2016: https://github.com/ngosang/trackerslist/issues/26
	// Runned by Internet Archive.
	// According to https://help.archive.org/help/archive-bittorrents/,
	// they are not open ("they track our only own torrents").
	// "udp://bt1.archive.org:6969/announce",
	// "udp://bt1.archive.org:6969/announce",
	"http://sukebei.tracker.wf:8888/announce", // nyaa
	"http://nyaa.tracker.wf:7777/announce",
}

// Ignored file patterns in .gitignore style.
// Several cmds skip handling these files.
var DefaultIgnorePatterns = []string{
	".*",
	"$*",
	"~$*",     // Microsoft Office tmp files
	"*.aria2", // aria2 control files
	"*.bak",
	"*.lnk", // windows shortcuts
	"*.swp", // vim temp files
	"*.tmp",
	"*.temp",
	"*.dropbox",
	"*.torrent",
	"node_modules/",
	"lost+found/",
	"System Volume Information",
	"desktop.ini",
	"Thumbs.db",
}

// Returned if the action is not processed due to in dry run mode
var ErrDryRun = fmt.Errorf("dry run")

func init() {
	args := []string{}
	for old, new := range FilepathRestrictedCharacterReplacement {
		args = append(args, string(old), string(new))
	}
	FilepathRestrictedCharacterReplacer = strings.NewReplacer(args...)
	for old, new := range FilenameRestrictedCharacterReplacement {
		args = append(args, string(old), string(new))
	}
	FilenameRestrictedCharacterReplacer = strings.NewReplacer(args...)
}
