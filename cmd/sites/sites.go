package sites

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/sagan/ptool/cmd"
	"github.com/sagan/ptool/site/tpl"
	"github.com/sagan/ptool/util"
)

var Command = &cobra.Command{
	Use:   "sites [--filter filter]",
	Short: "Show internal supported PT sites list which can be used with this software.",
	Long: `Show internal supported PT sites list which can be used with this software.
By default it does NOT display obsolete / legacy site that is currently / already dead,
unless --all flag is set.`,
	Args: cobra.MatchAll(cobra.ExactArgs(0), cobra.OnlyValidArgs),
	RunE: sites,
}

var (
	showJson = false
	showAll  = false
	filter   = ""
)

func init() {
	Command.Flags().BoolVarP(&showJson, "json", "", false, "Show output in json format")
	Command.Flags().BoolVarP(&showAll, "all", "a", false, "Show all sites (including dead / obsolete / legacy sites)")
	Command.Flags().StringVarP(&filter, "filter", "", "",
		"Filter sites. Only show sites which name / url / comment contain this string")
	cmd.RootCmd.AddCommand(Command)
}

func sites(cmd *cobra.Command, args []string) error {
	if showJson {
		siteDatas := []map[string]any{}
		for _, name := range tpl.SITENAMES {
			if filter != "" && !tpl.SITES[name].MatchFilter(filter) {
				continue
			}
			siteData := util.StructToMap(*tpl.SITES[name], true, false)
			siteData["name"] = name
			siteDatas = append(siteDatas, siteData)
		}
		util.PrintJson(os.Stdout, siteDatas)
		return nil
	}
	fmt.Printf("<internal supported sites by this program (dead: X; globalHnR: !)>\n")
	if filter == "" {
		fmt.Printf(`<to filter, use "--filter string" flag>` + "\n")
	} else {
		fmt.Printf("<applying filter '%s'>\n", filter)
	}
	fmt.Printf("%-15s  %-15s  %-30s  %13s  %-5s  %s\n", "Type", "Aliases", "Url", "Schema", "Flags", "Comment")
	for _, name := range tpl.SITENAMES {
		siteInfo := tpl.SITES[name]
		if siteInfo.Dead && !showAll {
			continue
		}
		if filter != "" && !siteInfo.MatchFilter(filter) {
			continue
		}
		flags := []string{}
		if siteInfo.Dead {
			flags = append(flags, "X")
		}
		if siteInfo.GlobalHnR {
			flags = append(flags, "!")
		}
		fmt.Printf("%-15s  %-15s  %-30s  %13s  %-5s  %s\n", name, strings.Join(siteInfo.Aliases, ","),
			siteInfo.Url, siteInfo.Type, strings.Join(flags, ""), siteInfo.Comment)
	}
	return nil
}
