package constants

const HELP_TORRENT_ARGS = `Args list is a torrent list that each one could be
a local filename (e.g. "*.torrent" or "[M-TEAM]CLANNAD.torrent"),
site torrent id (e.g. "mteam.488424") or url (e.g. "https://kp.m-team.cc/details.php?id=488424").
Torrent url that does NOT belong to any site (e.g. a public site url), as well as "magnet:" link, is also supported.
Use a single "-" as args to read the list from stdin, delimited by blanks,
as a special case, it also supports directly reading .torrent file contents from stdin`

const HELP_INFOHASH_ARGS = `Args list is an info-hash list of torrents.
It's possible to use the following state filters in list to target multiple torrents:
_all, _active, _done, _undone, _downloading, _seeding, _paused, _completed, _error.
Use a single "-" as args to read the list from stdin, delimited by blanks`

const HELP_TIP_TTY_BINARY_OUTPUT = "binary .torrent file will mess up the terminal. Use pipe to redirect stdout"

const HELP_ARG_TRACKER = `Filter torrents by tracker url or domain. Use "` +
	NONE + `" to select torrents without tracker`

const HELP_ARG_FILTER_TORRENT = "Filter torrents by name"

const HELP_ARG_CATEGORY = `Filter torrents by category. Use "` + NONE + `" to select uncategoried torrents`
const HELP_ARG_CATEGORY_XSEED = `Only xseed torrents that belongs to this category. Use "` +
	NONE + `" to select uncategoried torrents`

const HELP_ARG_TAG = `Filter torrents by tag. Comma-separated list. ` +
	`Torrent which tags contain any one in the list matches. Use "` + NONE + `" to select untagged torrents`
const HELP_ARG_TAG_XSEED = `Comma-separated tag list. Only xseed torrents which tags ` +
	`contain any one in the list. Use "` + NONE + `" to select untagged torrents`
const HELP_ARG_TIMES = `Time string (local timezone). ` +
	`Supported formats: "yyyy-MM-dd HH:mm:ss", a unix timestamp integer (seconds), ` +
	`or a time duration (e.g. "5d") which references to a past time point from now`
