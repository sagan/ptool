package example

import (
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"

	"github.com/sagan/ptool/cmd/configcmd"
	"github.com/sagan/ptool/config"
)

var command = &cobra.Command{
	Use:   "example",
	Short: "Display example config file contents.",
	Long:  `Display example config file contents.`,
	Args:  cobra.MatchAll(cobra.ExactArgs(0), cobra.OnlyValidArgs),
	RunE:  example,
}

var (
	format = ""
)

func init() {
	command.Flags().StringVarP(&format, "format", "", "", `Select the format of example config file to display, `+
		`e.g. "toml", "yaml". By default it uses the format of current config file`)
	configcmd.Command.AddCommand(command)
}

func example(cmd *cobra.Command, args []string) error {
	if format == "" {
		format = config.ConfigType
	}
	if file, err := config.DefaultConfigFs.Open(config.EXAMPLE_CONFIG_FILE + "." + format); err != nil {
		return fmt.Errorf("unsupported config file type %q: %w", format, err)
	} else {
		fmt.Printf("# %s.%s\n\n", config.EXAMPLE_CONFIG_FILE, format)
		io.Copy(os.Stdout, file)
		return nil
	}
}
