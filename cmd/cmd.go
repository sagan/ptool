package cmd

import (
	"log"
	"os"

	"github.com/sagan/ptool/config"
	"github.com/spf13/cobra"
)

// Root represents the base command when called without any subcommands
var RootCmd = &cobra.Command{
	Use:   "ptool",
	Short: "ptool command [flags]",
	Long:  `ptool.`,
	// Run: func(cmd *cobra.Command, args []string) { },
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	err := RootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

func init() {
	configFile, _ := os.UserHomeDir()
	configFile += "/.config/ptool/ptool.yaml"
	_, err := os.Stat(configFile)
	if err != nil {
		if _, err = os.Stat("ptool.yaml"); err == nil {
			configFile = "ptool.yaml"
		}
	}
	log.Printf("Default config file: %s\n", configFile)

	// global flags
	RootCmd.PersistentFlags().StringVar(&config.ConfigFile, "config", configFile, "config file ([ptool.yaml])")

	// local flags
	RootCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}
