package show

import (
	"github.com/c-bata/go-prompt"

	"github.com/sagan/ptool/cmd"
	"github.com/sagan/ptool/cmd/common"
	"github.com/sagan/ptool/cmd/shell/suggest"
)

func init() {
	cmd.AddShellCompletion("show", func(document *prompt.Document) []prompt.Suggest {
		info := suggest.Parse(document)
		if info.LastArgIndex < 1 {
			return nil
		}
		if info.LastArgIsFlag {
			switch info.LastArgFlag {
			case "order":
				return suggest.EnumFlagArg(info.MatchingPrefix, common.OrderFlag)
			case "sort":
				return suggest.EnumFlagArg(info.MatchingPrefix, common.ClientTorrentSortFlag)
			default:
				return nil
			}
		}
		if info.LastArgIndex == 1 {
			return suggest.ClientArg(info.MatchingPrefix)
		}
		return suggest.InfoHashOrFilterArg(info.MatchingPrefix, info.Args[1])
	})
}
