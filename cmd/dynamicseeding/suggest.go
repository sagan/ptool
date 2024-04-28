package dynamicseeding

import (
	"github.com/c-bata/go-prompt"

	"github.com/sagan/ptool/cmd"
	"github.com/sagan/ptool/cmd/shell/suggest"
)

func init() {
	cmd.AddShellCompletion("dynamicseeding", func(document *prompt.Document) []prompt.Suggest {
		info := suggest.Parse(document)
		if info.LastArgIndex < 1 {
			return nil
		}
		if info.LastArgIsFlag {
			return nil
		}
		switch info.LastArgIndex {
		case 1:
			return suggest.ClientArg(info.MatchingPrefix)
		case 2:
			return suggest.SiteArg(info.MatchingPrefix)
		default:
			return nil
		}
	})
}
