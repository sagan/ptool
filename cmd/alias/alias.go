package versioncmd

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
	if len(args) == 0 {
		return fmt.Errorf("alias name must be provided as the first arg")
	}
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
		return fmt.Errorf("failed to parse cmd of alias '%s': %v", aliasName, err)
	}
	aliasArgs = append([]string{os.Args[0]}, aliasArgs...)
	os.Args = append(aliasArgs, args...)
	fmt.Printf("Run alias '%s': %v\n", aliasName, os.Args[1:])
	return cmd.RootCmd.Execute()
}
