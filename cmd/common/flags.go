package common

import (
	"github.com/sagan/ptool/cmd"
	"golang.org/x/exp/slices"
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

// pure flag: bool or counter flag. It does not have a value
// all single-letter name (shorthand) flags are pure and not included in the list
// none-pure flag: a flag which has a value. eg. "--name=value", "--name value"
// This list is manually maintenanced for now
var pureFlags = []string{
	"add-category-auto",
	"add-paused",
	"add-respect-noadd",
	"break",
	"clients",
	"delete-added",
	"dense",
	"dry-run",
	"force",
	"fork",
	"help",
	"include-downloaded",
	"largest",
	"no-hr",
	"no-paid",
	"preserve",
	"raw",
	"rename-added",
	"one-page",
	"show-files",
	"show-info-hash-only",
	"show-parameters",
	"show-trackers",
	"show-value-only",
	"sites",
	"skip-check",
	"sum",
	"verbose",
}

func IsPureFlag(name string) bool {
	return len(name) == 1 || slices.Index(pureFlags, name) != -1
}
