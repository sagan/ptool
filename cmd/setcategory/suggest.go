package setcategory

import (
	"github.com/c-bata/go-prompt"

	"github.com/sagan/ptool/cmd"
	"github.com/sagan/ptool/cmd/shell/suggest"
)

func init() {
	cmd.AddShellCompletion("setcategory", func(document *prompt.Document) []prompt.Suggest {
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
		if info.LastArgIndex >= 3 {
			return suggest.InfoHashOrFilterArg(info.MatchingPrefix, info.Args[1])
		}
		return nil
	})
}
