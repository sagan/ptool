package cookiecloud

import (
	"github.com/spf13/cobra"

	"github.com/sagan/ptool/cmd"
)

var Command = &cobra.Command{
	Use:   "cookiecloud",
	Short: "Use cookiecloud to sync cookies or import sites.",
	Long: `Use cookiecloud to sync cookies or import sites.

See also:
* CookieCloud: https://github.com/easychen/CookieCloud`,
	Args: cobra.MatchAll(cobra.ExactArgs(0), cobra.OnlyValidArgs),
}

func init() {
	cmd.RootCmd.AddCommand(Command)
}
