package cmd

import (
	"fmt"
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
	"github.com/sagan/ptool/util/osutil"
)

// Root represents the base command when called without any subcommands
var RootCmd = &cobra.Command{
	Use:   "ptool",
	Short: "ptool is a command-line program which facilitate the use of private tracker sites and BitTorrent clients.",
	Long: `ptool is a command-line program which facilitate the use of private tracker sites and BitTorrent clients.
It's a free and open-source software, visit https://github.com/sagan/ptool for more infomation.`,
	// Run: func(cmd *cobra.Command, args []string) { },
	SilenceErrors:      true,
	SilenceUsage:       true,
	DisableSuggestions: true,
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
			lock := flock.New(config.LockFile)
			if config.LockOrExit {
				ok, err := lock.TryLock()
				if err != nil {
					log.Fatalf("Unable to lock file %s: %v", config.LockFile, err)
				}
				if !ok {
					log.Fatalf("Unable to lock file %s: %v", config.LockFile, "acquired by other process")
				}
			} else {
				err := lock.Lock()
				if err != nil {
					log.Fatalf("Unable to lock file %s: %v", config.LockFile, err)
				}
			}
			log.Infof("Lock acquired")
		}
		ShellHistory = &ShellHistoryStruct{filename: filepath.Join(config.ConfigDir, config.HISTORY_FILENAME)}
	})
	// See https://github.com/spf13/cobra/issues/914 .
	// Must use RunE to capture error.
	// Returned errors:
	// Unknown command (specified direct subcommand not found), unknown shorthand flag,
	err := RootCmd.Execute()
	if err != nil {
		if strings.HasPrefix(err.Error(), "unknown command ") {
			log.Debugf("Unknown command. Try to parse input as alias: %v", os.Args)
			os.Args = append([]string{os.Args[0], "alias"}, os.Args[1:]...)
			err = RootCmd.Execute()
		}
		if err != nil {
			fmt.Printf("Error: %v.\n", err)
			Exit(1)
		}
	}
	Exit(0)
}

func init() {
	UserHomeDir, _ := os.UserHomeDir()
	configFile := filepath.Join(UserHomeDir, ".config/ptool/ptool.toml")
	configFiles := []string{
		configFile,
		filepath.Join(UserHomeDir, ".config/ptool/ptool.yaml"),
		filepath.Join(".", "ptool.toml"),
		filepath.Join(".", "ptool.yaml"),
	}
	for _, cf := range configFiles {
		_, err := os.Stat(cf)
		if err == nil || !os.IsNotExist(err) {
			configFile = cf
			break
		}
	}
	config.DefaultConfigFile = configFile

	// global flags
	RootCmd.PersistentFlags().BoolVarP(&config.Fork, "fork", "", false, "Enables a daemon mode that runs the ptool process in the background (detached from current terminal). The current stdout / stderr will still be used so you may want to redirect them to files using pipe. It only works on Linux platform")
	RootCmd.PersistentFlags().BoolVarP(&config.LockOrExit, "lock-or-exit", "", false, "Used with --lock flag. If failed to acquire lock, exit 1 immediately instead of waiting")
	RootCmd.PersistentFlags().StringVarP(&config.ConfigFile, "config", "", config.DefaultConfigFile, "Config file ([ptool.toml])")
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
