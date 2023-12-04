package version

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/sagan/ptool/cmd"
	"github.com/sagan/ptool/site/tpl"
	"github.com/sagan/ptool/util"
)

var command = &cobra.Command{
	Use:   "sites [--filter filter]",
	Short: "Show internal supported PT sites list which can be used with this software.",
	Long:  `Show internal supported PT sites list which can be used with this software.`,
	Args:  cobra.MatchAll(cobra.ExactArgs(0), cobra.OnlyValidArgs),
	RunE:  sites,
}

var (
	filter = ""
)

func init() {
	command.Flags().StringVarP(&filter, "filter", "", "", "Filter sites. Only show sites which name / url / comment contain this string")
	cmd.RootCmd.AddCommand(command)
}

func sites(cmd *cobra.Command, args []string) error {
	fmt.Printf("<internal supported sites by this program. use --filter flag to find a specific site>\n")
	fmt.Printf("%-15s  %-15s  %-30s  %10s  %s\n", "Type", "Aliases", "Url", "Schema", "Comment")
	for _, name := range tpl.SITENAMES {
		siteInfo := tpl.SITES[name]
		if filter != "" && (!util.ContainsI(siteInfo.GetName(), filter) &&
			!util.ContainsI(siteInfo.Url, filter) && !util.ContainsI(siteInfo.Comment, filter)) {
			continue
		}
		fmt.Printf("%-15s  %-15s  %-30s  %10s  %s\n", name, strings.Join(siteInfo.Aliases, ","), siteInfo.Url, siteInfo.Type, siteInfo.Comment)
	}
	return nil
}
