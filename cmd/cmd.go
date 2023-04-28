package cmd

import (
	"os"
	"path/filepath"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/sagan/ptool/config"
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
	cobra.OnInitialize(func() {
		// level: panic(0), fatal(1), error(2), warn(3), info(4), debug(5), trace(6). Default level = warning(3)
		config.ConfigDir = filepath.Dir(config.ConfigFile)
		logLevel := 3 + config.VerboseLevel
		log.SetLevel(log.Level(logLevel))
		log.Debugf("ptool start: %s", os.Args)
		log.Infof("config file: %s", config.ConfigFile)
	})
	err := RootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

func init() {
	UserHomeDir, _ := os.UserHomeDir()
	configFile := "ptool.toml"
	configFiles := []string{
		UserHomeDir + "/.config/ptool/ptool.toml",
		UserHomeDir + "/.config/ptool/ptool.yaml",
		"ptool.toml",
		"ptool.yaml",
	}
	for _, cf := range configFiles {
		_, err := os.Stat(cf)
		if err == nil {
			configFile = cf
			break
		}
	}

	// global flags
	RootCmd.PersistentFlags().StringVar(&config.ConfigFile, "config", configFile, "config file ([ptool.toml])")
	RootCmd.PersistentFlags().CountVarP(&config.VerboseLevel, "verbose", "v", "verbose (-v, -vv, -vvv)")
}
