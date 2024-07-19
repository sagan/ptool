package common

import (
	"slices"

	"github.com/sagan/ptool/cmd"
	"github.com/sagan/ptool/constants"
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
		{constants.NONE, ""},
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
		{constants.NONE, ""},
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
// non-pure flag: a flag which has a value. e.g. "--name=value", "--name value".
// This list is manually maintenanced for now.
var pureFlags = []string{
	"add-category-auto",
	"add-paused",
	"add-public-trackers",
	"add-respect-noadd",
	"all",
	"allow-filename-restricted-characters",
	"allow-long-name",
	"append",
	"backup",
	"bindable",
	"break",
	"check",
	"check-quick",
	"clients",
	"data-order",
	"dedupe",
	"delete-added",
	"delete-alone",
	"delete-fail",
	"dense",
	"dry-run",
	"force",
	"force-local",
	"fork",
	"free",
	"help",
	"include-downloaded",
	"insecure",
	"json",
	"largest",
	"latest",
	"lock-or-exit",
	"newest",
	"no-clean",
	"no-cover",
	"no-hr",
	"no-neutral",
	"no-paid",
	"parameters",
	"partial",
	"preserve",
	"preserve-if-xseed-exist",
	"private",
	"public",
	"raw",
	"remove-existing",
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
	"skip-existing",
	"slow",
	"strict",
	"sum",
	"use-comment-meta",
	"verbose",
}

func IsPureFlag(name string) bool {
	return slices.Contains(pureFlags, name)
}
