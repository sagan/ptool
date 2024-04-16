package constants

const HELP_TORRENT_ARGS = `Args list is a torrent list that each one could be
a local filename (e.g. "*.torrent" or "[M-TEAM]CLANNAD.torrent"),
site torrent id (e.g.: "mteam.488424") or url (e.g.: "https://kp.m-team.cc/details.php?id=488424").
Torrent url that does NOT belong to any site (e.g.: a public site url), as well as "magnet:" link, is also supported.
Use a single "-" as args to read torrent list from stdin, delimited by blanks,
as a special case, it also supports directly reading .torrent file contents from stdin`

const HELP_INFOHASH_ARGS = `Args list is a list of info-hashes of torrents to delete.
It's possible to use the following state filters in list to target multiple torrents:
_all, _active, _done, _undone, _downloading, _seeding, _paused, _completed, _error.
Specially, use a single "-" as args to read the list from stdin, delimited by blanks`
