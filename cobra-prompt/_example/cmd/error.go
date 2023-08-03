package cmd

import (
	"errors"

	"github.com/spf13/cobra"
)

var errorCmd = &cobra.Command{
	Use:   "error",
	Short: "Returns error",
	RunE: func(cmd *cobra.Command, args []string) error {
		return errors.New("when an error accure you can decide if you want to continue or not by handling error with OnErrorFunc")
	},
}

func init() {
	RootCmd.AddCommand(errorCmd)
}
