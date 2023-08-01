package xseedcheck

import (
	"github.com/c-bata/go-prompt"

	"github.com/sagan/ptool/cmd"
	"github.com/sagan/ptool/cmd/shell/suggest"
)

func init() {
	cmd.AddShellCompletion("xseedcheck", func(document *prompt.Document) []prompt.Suggest {
		info := suggest.Parse(document)
		if info.LastArgIsFlag {
			return nil
		}
		switch info.LastArgIndex {
		case 1:
			return suggest.ClientArg(info.MatchingPrefix)
		case 3:
			return suggest.FileArg(info.MatchingPrefix, "torrent")
		default:
			return nil
		}
	})
}
