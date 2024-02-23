package show

import (
	"fmt"

	toml "github.com/pelletier/go-toml/v2"
	"github.com/spf13/cobra"

	"github.com/sagan/ptool/cmd/sites"
	"github.com/sagan/ptool/site/tpl"
	"github.com/sagan/ptool/util"
)

var command = &cobra.Command{
	Use:   "show {site}...",
	Short: "Show detailed configuration of internal supported PT site.",
	Long: `Show detailed configuration of internal supported PT site.
It prints output in toml format. All configurations of any site can be overrided in ptool.toml.`,
	Args: cobra.MatchAll(cobra.MinimumNArgs(1), cobra.OnlyValidArgs),
	RunE: show,
}

func init() {
	sites.Command.AddCommand(command)
}

func show(cmd *cobra.Command, args []string) error {
	sitenames := args

	for _, sitename := range sitenames {
		tplconfig := tpl.SITES[sitename]
		if tplconfig == nil {
			fmt.Printf("# %s : is NOT a internal supported site\n", sitename)
			continue
		}
		str, err := toml.Marshal(util.StructToMap(*tplconfig, true, true))
		if err != nil {
			fmt.Printf("# %s : failed to get detailed configuration: %v\n", sitename, err)
			continue
		}
		fmt.Printf("# %s\n[[sites]]\n%s\n", sitename, str)
	}
	return nil
}
