package delete

import (
	"github.com/c-bata/go-prompt"

	"github.com/sagan/ptool/cmd"
	"github.com/sagan/ptool/cmd/shell/suggest"
)

func init() {
	cmd.AddShellCompletion("delete", func(document *prompt.Document) []prompt.Suggest {
		info := suggest.Parse(document)
		if info.LastArgIndex < 1 {
			return nil
		}
		if info.LastArgIsFlag {
			return nil
		}
		if info.LastArgIndex == 1 {
			return suggest.ClientArg(info.MatchingPrefix)
		}
		return suggest.InfoHashArg(info.MatchingPrefix, info.Args[1])
	})
}
