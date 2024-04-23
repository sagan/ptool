package run

import (
	"fmt"
	"os"

	"github.com/google/shlex"
	"github.com/spf13/cobra"

	"github.com/sagan/ptool/cmd"
)

var command = &cobra.Command{
	Use:                "run {cmdline}",
	Short:              `Run cmdline. Accept the whole cmdline as single arg, e.g. 'ptool run "status local -t"'.`,
	Long:               `Run cmdline. Accept the whole cmdline as single arg, e.g. 'ptool run "status local -t"'.`,
	DisableFlagParsing: true,
	Args:               cobra.MatchAll(cobra.ExactArgs(1), cobra.OnlyValidArgs),
	RunE:               run,
}

func init() {
	cmd.RootCmd.AddCommand(command)
}

func run(_ *cobra.Command, args []string) error {
	cmdline := args[0]
	cmdlineArgs, err := shlex.Split(cmdline)
	if err != nil {
		return fmt.Errorf("failed to parse cmdline '%s': %v", cmdline, err)
	}
	os.Args = append([]string{os.Args[0]}, cmdlineArgs...)
	fmt.Fprintf(os.Stderr, "Run cmdline: %v\n", os.Args[1:])
	return cmd.RootCmd.Execute()
}
