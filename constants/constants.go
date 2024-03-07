package constants

import "regexp"

// .torrent file magic number.
// See: https://en.wikipedia.org/wiki/Torrent_file , https://en.wikipedia.org/wiki/Bencode .
const TORRENT_FILE_MAGIC_NUMBER = "d8:announce"
const FILENAME_INVALID_CHARS_REGEX = `[<>:"/\|\?\*]+`

var FilenameInvalidCharsRegex = regexp.MustCompile(FILENAME_INVALID_CHARS_REGEX)
