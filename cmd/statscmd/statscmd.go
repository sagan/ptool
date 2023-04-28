package statscmd

import (
	"fmt"
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
To use this command, you must manually enable the statistics feature by add the "brushEnableStats: true" line to your ptool.yaml config file.
`,
	Run: statscmd,
}

func init() {
	cmd.RootCmd.AddCommand(command)
}

func statscmd(cmd *cobra.Command, args []string) {
	if !config.Get().BrushEnableStats {
		fmt.Printf(`Statistics feature is NOT enabled currently. To enable it, add the "brushEnableStats: true" line to ptool.yaml config file.
It will use the "ptool_stats.txt" (in the same dir of ptool.yaml file) as the statistics data file.
`)
		os.Exit(1)
	}
	clientnames := args
	if len(clientnames) == 0 {
		stats.Db.ShowTrafficStats("")
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
		stats.Db.ShowTrafficStats(clientname)
	}
}
