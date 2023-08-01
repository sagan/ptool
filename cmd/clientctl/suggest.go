package clientctl

import (
	"strings"

	"github.com/c-bata/go-prompt"

	"github.com/sagan/ptool/cmd"
	"github.com/sagan/ptool/cmd/shell/suggest"
)

func init() {
	cmd.AddShellCompletion("clientctl", func(document *prompt.Document) []prompt.Suggest {
		info := suggest.Parse(document)
		if info.LastArgIsFlag {
			return nil
		}
		if info.LastArgIndex == 1 {
			return suggest.ClientArg(info.MatchingPrefix)
		}
		if strings.HasPrefix(info.MatchingPrefix, "=") {
			return nil
		}
		commpletions := [][2]string{}
		for _, option := range allOptions {
			commpletions = append(commpletions, [2]string{option.Name, option.Description})
		}
		return suggest.EnumArg(info.MatchingPrefix, commpletions)
	})
}
