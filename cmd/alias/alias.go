package alias

import (
	"fmt"
	"os"
	"strings"

	"github.com/google/shlex"
	"github.com/spf13/cobra"

	"github.com/sagan/ptool/cmd"
	"github.com/sagan/ptool/config"
)

var command = &cobra.Command{
	Use:                "alias",
	Short:              "Run alias.",
	Long:               `Run alias.`,
	DisableFlagParsing: true,
	Args:               cobra.MatchAll(cobra.MinimumNArgs(1), cobra.OnlyValidArgs),
	RunE:               aliascmd,
}

var (
	inAlias = false
)

func init() {
	cmd.RootCmd.AddCommand(command)
}

func aliascmd(_ *cobra.Command, args []string) error {
	if inAlias {
		return fmt.Errorf("recursive alias definition is NOT supported")
	}
	inAlias = true
	defer func() {
		inAlias = false
	}()
	aliasName := args[0]
	args = args[1:]

	aliasConfig := config.GetAliasConfig(aliasName)
	if aliasConfig == nil {
		return fmt.Errorf("command or alias '%s' not found. Run 'ptool --help' for usage", aliasName)
	}
	argsCmd := strings.TrimSpace(aliasConfig.Cmd)
	if argsCmd == "" {
		return fmt.Errorf("alias '%s' does have cmd", aliasName)
	}
	aliasArgs, err := shlex.Split(argsCmd)
	if err != nil {
		return fmt.Errorf("failed to parse alias %s cmdline '%s': %w", aliasName, argsCmd, err)
	}
	if len(args) < int(aliasConfig.MinArgs) {
		return fmt.Errorf("alias '%s' requires at least %d arg(s), only received %d",
			aliasName, aliasConfig.MinArgs, len(args))
	}
	aliasArgs = append([]string{os.Args[0]}, aliasArgs...)
	aliasArgs = append(aliasArgs, args...)
	if len(args) == int(aliasConfig.MinArgs) && aliasConfig.DefaultArgs != "" {
		if aliasDefaultArgs, err := shlex.Split(aliasConfig.DefaultArgs); err != nil {
			return fmt.Errorf("failed to parse alias '%s' defaultArgs '%s': %w", aliasName, aliasConfig.DefaultArgs, err)
		} else {
			aliasArgs = append(aliasArgs, aliasDefaultArgs...)
		}
	}
	os.Args = aliasArgs
	fmt.Fprintf(os.Stderr, "Run alias '%s': %v\n", aliasName, os.Args[1:])
	return cmd.RootCmd.Execute()
}
