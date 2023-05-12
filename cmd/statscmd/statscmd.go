package statscmd

import (
	"fmt"
	"log"
	"os"

	"github.com/spf13/cobra"

	"github.com/sagan/ptool/cmd"
	"github.com/sagan/ptool/config"
	"github.com/sagan/ptool/stats"
)

var command = &cobra.Command{
	Use:   "stats [clients...] [flags]",
	Short: "Show client statistics",
	Long: `Show client brushing traffic statistics
Only torrents added by ptool (of this machine) will be counted.
The traffic info of a torrent will ONLY be recorded when it's been DELETED from the client.
To use this command, you must manually enable the statistics feature by add the "brushEnableStats = true" line to ptool.toml config file.
`,
	Run: statscmd,
}

var (
	statsFilename = ""
)

func init() {
	command.Flags().StringVarP(&statsFilename, "stats-file", "", "",
		"Manually specify stats file ("+config.STATS_FILENAME+") path")
	cmd.RootCmd.AddCommand(command)
}

func statscmd(cmd *cobra.Command, args []string) {
	if !config.Get().BrushEnableStats {
		fmt.Printf(`Statistics feature is NOT enabled currently. To enable it, add the "brushEnableStats = true" line to the top of ptool.toml config file.
It will use the "ptool_stats.txt" (in the same dir of ptool.toml file) as the statistics data file.
`)
		os.Exit(1)
	}
	if statsFilename == "" {
		statsFilename = config.ConfigDir + "/" + config.STATS_FILENAME
	}
	statDb, err := stats.NewDb(statsFilename)
	if err != nil {
		log.Fatalf("Failed to create stats db: %v", err)
	}
	clientnames := args
	if len(clientnames) == 0 {
		statDb.ShowTrafficStats("")
		return
	}

	doneFlag := map[string](bool){}
	for i, clientname := range clientnames {
		if clientname == "_" || doneFlag[clientname] {
			continue
		}
		doneFlag[clientname] = true
		if i > 0 {
			fmt.Printf("\n")
		}
		statDb.ShowTrafficStats(clientname)
	}
}
