package dltorrent

import (
	"github.com/c-bata/go-prompt"

	"github.com/sagan/ptool/cmd"
	"github.com/sagan/ptool/cmd/shell/suggest"
)

func init() {
	cmd.AddShellCompletion("dltorrent", func(document *prompt.Document) []prompt.Suggest {
		info := suggest.Parse(document)
		if info.LastArgIndex < 1 {
			return nil
		}
		if info.LastArgIsFlag {
			switch info.LastArgFlag {
			case "site":
				return suggest.SiteArg(info.MatchingPrefix)
			default:
				return nil
			}
		}
		return nil
	})
}
