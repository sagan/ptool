package match

import (
	"github.com/c-bata/go-prompt"

	"github.com/sagan/ptool/cmd"
	"github.com/sagan/ptool/cmd/shell/suggest"
)

func init() {
	cmd.AddShellCompletion("reseed.match", func(document *prompt.Document) []prompt.Suggest {
		info := suggest.Parse(document)
		if info.LastArgIndex < 1 {
			return nil
		}
		if info.LastArgIsFlag {
			switch info.LastArgFlag {
			case "download-dir":
				return suggest.DirArg(info.MatchingPrefix)
			default:
				return nil
			}
		}
		return suggest.DirArg(info.MatchingPrefix)
	})
}
