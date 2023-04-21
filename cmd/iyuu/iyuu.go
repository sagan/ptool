package iyuu

import (
	"github.com/sagan/ptool/cmd"
	"github.com/spf13/cobra"
)

var Command = &cobra.Command{
	Use:   "iyuu",
	Short: "Cross seed automation tool using iyuu API.",
	Long:  `Cross seed automation tool using iyuu API.`,
}

func init() {
	cmd.RootCmd.AddCommand(Command)
}
