package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/c-bata/go-prompt"
	"github.com/gofrs/flock"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/sagan/ptool/client"
	"github.com/sagan/ptool/config"
	"github.com/sagan/ptool/site"
	"github.com/sagan/ptool/utils/osutil"
)

// Root represents the base command when called without any subcommands
var RootCmd = &cobra.Command{
	Use:   "ptool",
	Short: "ptool is a command-line program which facilitate the use of private tracker sites.",
	Long:  `ptool is a command-line program which facilitate the use of private tracker sites.`,
	// Run: func(cmd *cobra.Command, args []string) { },
	// SilenceErrors: true,
	SilenceUsage: true,
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		if config.InShell && config.Get().ShellMaxHistory != 0 {
			in := strings.Join(os.Args[1:], " ")
			ShellHistory.Write(in)
		}
	},
}

var (
	shellCompletions = map[string](func(document *prompt.Document) []prompt.Suggest){}
	ShellHistory     *ShellHistoryStruct
)

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	cobra.OnInitialize(func() {
		// cobra-prompt will start a new cobra context each time executing a command
		if config.Initialized {
			return
		}
		config.Initialized = true
		if config.Fork {
			osutil.Fork("--fork")
		}
		// level: panic(0), fatal(1), error(2), warn(3), info(4), debug(5), trace(6). Default level = warning(3)
		config.ConfigDir = filepath.Dir(config.ConfigFile)
		config.ConfigFile = filepath.Base(config.ConfigFile)
		configExt := filepath.Ext(config.ConfigFile)
		config.ConfigName = config.ConfigFile[:len(config.ConfigFile)-len(configExt)]
		if configExt != "" {
			config.ConfigType = configExt[1:]
		}
		logLevel := 3 + config.VerboseLevel
		log.SetLevel(log.Level(logLevel))
		log.Debugf("ptool start: %v", os.Args)
		log.Infof("config file: %s/%s", config.ConfigDir, config.ConfigFile)
		if config.LockFile != "" {
			log.Debugf("Locking file: %s", config.LockFile)
			err := flock.New(config.LockFile).Lock()
			if err != nil {
				log.Fatalf("Unable to lock file %s: %v", config.LockFile, err)
			}
			log.Infof("Lock acquired")
		}
		ShellHistory = &ShellHistoryStruct{filename: filepath.Join(config.ConfigDir, config.HISTORY_FILENAME)}
	})
	// see https://github.com/spf13/cobra/issues/914
	// Must use RunE to capture error
	err := RootCmd.Execute()
	if err != nil {
		Exit(1)
	} else {
		Exit(0)
	}
}

func init() {
	UserHomeDir, _ := os.UserHomeDir()
	configFile := "ptool.toml"
	configFiles := []string{
		filepath.Join(UserHomeDir, ".config/ptool/ptool.toml"),
		filepath.Join(UserHomeDir, ".config/ptool/ptool.yaml"),
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
	RootCmd.PersistentFlags().BoolVarP(&config.Fork, "fork", "", false, "Enables a daemon mode that runs the ptool process in the background (detached from current terminal). The current stdout / stderr will still be used so you may want to redirect them to files using pipe. It only works on Linux platform")
	RootCmd.PersistentFlags().StringVarP(&config.ConfigFile, "config", "", configFile, "Config file ([ptool.toml])")
	RootCmd.PersistentFlags().StringVarP(&config.LockFile, "lock", "", "", "Lock filename. If set, ptool will acquire the lock on the file before executing command. It is intended to be used to prevent multiple invocations of ptool process at the same time. If the lock file does not exist, it will be created automatically. However, it will NOT be deleted after ptool process exits")
	RootCmd.PersistentFlags().CountVarP(&config.VerboseLevel, "verbose", "v", "verbose (-v, -vv, -vvv)")
}

// clean all resources created during this session and exit
func Exit(code int) {
	log.Tracef("Exit. Closing resources")
	var resourcesWaitGroup sync.WaitGroup
	resourcesWaitGroup.Add(2)
	go func() {
		defer resourcesWaitGroup.Done()
		client.Exit()
	}()
	go func() {
		defer resourcesWaitGroup.Done()
		site.Exit()
	}()
	resourcesWaitGroup.Wait()
	if config.InShell && config.Get().ShellMaxHistory > 0 {
		ShellHistory.Truncate(int(config.Get().ShellMaxHistory))
	}
	os.Exit(code)
}

// cobra-prompt dynamic suggestions
func AddShellCompletion(name string, f func(document *prompt.Document) []prompt.Suggest) {
	shellCompletions[name] = f
}
func ShellDynamicSuggestionsFunc(name string, document *prompt.Document) []prompt.Suggest {
	f := shellCompletions[name]
	if f == nil {
		return nil
	}
	return f(document)
}
