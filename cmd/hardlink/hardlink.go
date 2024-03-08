package hardlink

import (
	"github.com/spf13/cobra"

	"github.com/sagan/ptool/cmd"
)

var Command = &cobra.Command{
	Use:   "hardlink",
	Short: "Hardlink utilities",
	Long:  `Hardlink utilities`,
	Args:  cobra.MatchAll(cobra.ExactArgs(0), cobra.OnlyValidArgs),
}

func init() {
	cmd.RootCmd.AddCommand(Command)
}
