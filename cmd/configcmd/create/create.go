package create

import (
	"fmt"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/sagan/ptool/cmd/configcmd"
	"github.com/sagan/ptool/config"
)

var command = &cobra.Command{
	Use:   "create",
	Short: "Create initial pool config file.",
	Long:  `Create initial pool config file.`,
	Args:  cobra.MatchAll(cobra.ExactArgs(0), cobra.OnlyValidArgs),
	RunE:  create,
}

func init() {
	configcmd.Command.AddCommand(command)
}

func create(cmd *cobra.Command, args []string) error {
	fmt.Printf("Creating config file %s%c%s\n", config.ConfigDir, filepath.Separator, config.ConfigFile)
	return config.CreateDefaultConfig()
}
