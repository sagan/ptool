package batchdl

import (
	"github.com/sagan/ptool/cmd"
)

var ActionEnumFlag = &cmd.EnumFlag{
	Description: "Choose action for found torrents",
	Options: [][2]string{
		{"show", "print torrent details"},
		{"download", "download torrent"},
		{"add", "add torrent to client"},
		{"printid", "print torrent id to stdout or file"},
		{"export", "export torrents info [csv] to stdout or file"},
	},
}
