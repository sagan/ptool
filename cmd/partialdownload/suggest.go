package partialdownload

import (
	"github.com/c-bata/go-prompt"

	"github.com/sagan/ptool/cmd"
	"github.com/sagan/ptool/cmd/shell/suggest"
)

func init() {
	cmd.AddShellCompletion("partialdownload", func(document *prompt.Document) []prompt.Suggest {
		info := suggest.Parse(document)
		if info.LastArgIndex < 1 {
			return nil
		}
		if info.LastArgIsFlag {
			return nil
		}
		if info.LastArgIndex == 1 {
			return suggest.ClientArg(info.MatchingPrefix)
		} else if info.LastArgIndex == 2 {
			return suggest.InfoHashArg(info.MatchingPrefix, info.Args[1])
		}
		return nil
	})
}
