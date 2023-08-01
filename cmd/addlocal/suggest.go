package addlocal

import (
	"github.com/c-bata/go-prompt"

	"github.com/sagan/ptool/cmd"
	"github.com/sagan/ptool/cmd/shell/suggest"
)

func init() {
	cmd.AddShellCompletion("addlocal", func(document *prompt.Document) []prompt.Suggest {
		info := suggest.Parse(document)
		if info.LastArgIsFlag {
			return nil
		}
		if info.LastArgIndex > 1 {
			return suggest.FileArg(info.MatchingPrefix, "torrent")
		}
		return suggest.ClientArg(info.MatchingPrefix)
	})
}
