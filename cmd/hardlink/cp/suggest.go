package hardlinkcp

import (
	"github.com/c-bata/go-prompt"

	"github.com/sagan/ptool/cmd"
	"github.com/sagan/ptool/cmd/shell/suggest"
)

func init() {
	cmd.AddShellCompletion("hardlinkcp", func(document *prompt.Document) []prompt.Suggest {
		info := suggest.Parse(document)
		if info.LastArgIsFlag {
			return nil
		}
		return suggest.FileArg(info.MatchingPrefix, "", false)
	})
}
