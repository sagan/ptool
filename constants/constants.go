package constants

import "regexp"

// .torrent file magic number.
// See: https://en.wikipedia.org/wiki/Torrent_file , https://en.wikipedia.org/wiki/Bencode .
const TORRENT_FILE_MAGIC_NUMBER = "d8:announce"

// 个别种子没有 announce / announce-list 字段，第一个字段是 creation。这类种子可以通过 DHT 下载成功。
const TORRENT_FILE_MAGIC_NUMBER2 = "d13:creation date"

const FILENAME_INVALID_CHARS_REGEX = `[<>:"/\|\?\*]+`
const PERM = 0600 // 程序创建的所有文件的 PERM

var FilenameInvalidCharsRegex = regexp.MustCompile(FILENAME_INVALID_CHARS_REGEX)
