package common

import (
	"slices"

	"github.com/sagan/ptool/cmd"
)

// "name", "size", "speed", "state", "time", "tracker", "none"
var ClientTorrentSortFlag = &cmd.EnumFlag{
	Description: "Sort field of client torrents",
	Options: [][2]string{
		{"name", ""},
		{"size", ""},
		{"speed", ""},
		{"state", ""},
		{"time", ""},
		{"activity-time", ""},
		{"tracker", ""},
		{"none", ""},
	},
}

// size|time|name|seeders|leechers|snatched|none
var SiteTorrentSortFlag = &cmd.EnumFlag{
	Description: "Sort field of site torrents",
	Options: [][2]string{
		{"size", ""},
		{"time", ""},
		{"name", ""},
		{"seeders", ""},
		{"leechers", ""},
		{"snatched", ""},
		{"none", ""},
	},
}

// asc|desc
var OrderFlag = &cmd.EnumFlag{
	Description: "Sort order",
	Options: [][2]string{
		{"asc", ""},
		{"desc", ""},
	},
}

func YesNoAutoFlag(desc string) *cmd.EnumFlag {
	return &cmd.EnumFlag{
		Description: desc,
		Options: [][2]string{
			{"auto", ""},
			{"yes", ""},
			{"no", ""},
		},
	}
}

// pure flag: bool or counter flag. It does not have a value.
// all single-letter name (shorthand) flags are always considered as pure (for now),
// so they are not included in the list.
// none-pure flag: a flag which has a value. e.g.: "--name=value", "--name value".
// This list is manually maintenanced for now.
var pureFlags = []string{
	"add-category-auto",
	"add-paused",
	"add-respect-noadd",
	"all",
	"append",
	"backup",
	"break",
	"check",
	"check-quick",
	"clients",
	"delete-added",
	"dense",
	"download-skip-existing",
	"dry-run",
	"export-skip-existing",
	"force",
	"force-local",
	"fork",
	"free",
	"help",
	"include-downloaded",
	"json",
	"largest",
	"lock-or-exit",
	"newest",
	"no-hr",
	"no-neutral",
	"no-paid",
	"parameters",
	"partial",
	"preserve",
	"raw",
	"rename-added",
	"rename-fail",
	"rename-ok",
	"one-page",
	"original-order",
	"save-append",
	"sequential-download",
	"show-files",
	"show-id-only",
	"show-info-hash-only",
	"show-names-only",
	"show-trackers",
	"show-values-only",
	"sites",
	"skip-check",
	"slow",
	"strict",
	"sum",
	"use-comment-meta",
	"verbose",
}

func IsPureFlag(name string) bool {
	return slices.Index(pureFlags, name) != -1
}
