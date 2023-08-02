package batchdl

import (
	"github.com/c-bata/go-prompt"

	"github.com/sagan/ptool/cmd"
	"github.com/sagan/ptool/cmd/common"
	"github.com/sagan/ptool/cmd/shell/suggest"
)

func init() {
	cmd.AddShellCompletion("batchdl", func(document *prompt.Document) []prompt.Suggest {
		info := suggest.Parse(document)
		if info.LastArgIndex < 1 {
			return nil
		}
		if info.LastArgIsFlag {
			switch info.LastArgFlag {
			case "action":
				return suggest.EnumFlagArg(info.MatchingPrefix, ActionEnumFlag)
			case "add-client":
				return suggest.ClientArg(info.MatchingPrefix)
			case "order":
				return suggest.EnumFlagArg(info.MatchingPrefix, common.OrderFlag)
			case "sort":
				return suggest.EnumFlagArg(info.MatchingPrefix, common.SiteTorrentSortFlag)
			default:
				return nil
			}
		}
		if info.LastArgIndex != 1 {
			return nil
		}
		return suggest.SiteArg(info.MatchingPrefix)
	})
}
