package movesavepath

import (
	"github.com/c-bata/go-prompt"

	"github.com/sagan/ptool/cmd"
	"github.com/sagan/ptool/cmd/shell/suggest"
)

func init() {
	cmd.AddShellCompletion("movesavepath", func(document *prompt.Document) []prompt.Suggest {
		info := suggest.Parse(document)
		if info.LastArgIndex < 1 {
			return nil
		}
		if info.LastArgIsFlag {
			switch info.LastArgFlag {
			case "client":
				return suggest.ClientArg(info.MatchingPrefix)
			default:
				return nil
			}
		}
		if info.LastArgIndex == 1 || info.LastArgIndex == 2 {
			return suggest.FileArg("", "", true)
		}
		return nil
	})
}
