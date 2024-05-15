package constants

const HELP_TORRENT_ARGS = `Args list is a torrent list that each one could be
a local filename (e.g. "*.torrent" or "[M-TEAM]CLANNAD.torrent"),
site torrent id (e.g. "mteam.488424") or url (e.g. "https://kp.m-team.cc/details.php?id=488424").
Torrent url that does NOT belong to any site (e.g. a public site url), as well as "magnet:" link, is also supported.
Use a single "-" as args to read the list from stdin, delimited by blanks,
as a special case, it also supports directly reading .torrent file contents from stdin.
At least one (1) torrent arg must be provided, or it will throw an error; This also applies
when the args is "-", in which case the torrent list read from stdin must NOT be empty.`

const HELP_INFOHASH_ARGS = `Args list is an info-hash list of torrents.
It's possible to use the following state filters in the list to select multiple torrents:
  _all, _active, _done, _undone, _downloading, _seeding, _paused, _completed, _error.

If none of the filter flags (--category & --tag & --filter) is set, a single "-" can
be used as args list to read the list from stdin, delimited by blanks.
Also, at least one (1) info-hash arg must be provided, or it will throw an error; This also applies
when the args is "-", in which case the info-hash list read from stdin must NOT be empty.

If any filter flag is set, the args list can be empty. If the args list is not empty,
only torrents that match both the filter flags AND the args list will be selected.`

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
	`Supported formats: "yyyy-MM-dd HH:mm:ss", "yyyy-MM-dd", a unix timestamp integer (seconds), ` +
	`or a time duration (e.g. "5d") which references to a past time point from now`
const HELP_ARG_PATH_MAPPERS = `E.g. ` +
	`"/root/Downloads|/var/Downloads" will map "/root/Downloads" or "/root/Downloads/..." path to ` +
	`"/var/Downloads" or "/var/Downloads/...". You can also use ":" instead of "|" as the separator ` +
	`if both pathes do not contain ":" char.`
