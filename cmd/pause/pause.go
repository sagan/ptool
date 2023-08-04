package pause

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/sagan/ptool/client"
	"github.com/sagan/ptool/cmd"
)

var command = &cobra.Command{
	Use:         "pause {client} [--category category] [--tag tag] [--filter filter] [infoHash]...",
	Annotations: map[string](string){"cobra-prompt-dynamic-suggestions": "pause"},
	Aliases:     []string{"stop"},
	Short:       "Pause torrents of client.",
	Long: `Pause torrents of client.
[infoHash]...: infoHash list of torrents. It's possible to use state filter to target multiple torrents:
_all, _active, _done, _undone, _downloading, _seeding, _paused, _completed, _error.`,
	Args: cobra.MatchAll(cobra.MinimumNArgs(1), cobra.OnlyValidArgs),
	RunE: pause,
}

var (
	category = ""
	tag      = ""
	filter   = ""
)

func init() {
	command.Flags().StringVarP(&filter, "filter", "", "", "Filter torrents by name")
	command.Flags().StringVarP(&category, "category", "", "", "Filter torrents by category")
	command.Flags().StringVarP(&tag, "tag", "", "", "Filter torrents by tag. Comma-separated string list. Torrent which tags contain any one in the list will match")
	cmd.RootCmd.AddCommand(command)
}

func pause(cmd *cobra.Command, args []string) error {
	clientName := args[0]
	args = args[1:]
	if category == "" && tag == "" && filter == "" && len(args) == 0 {
		return fmt.Errorf("you must provide at least a condition flag or hashFilter")
	}
	clientInstance, err := client.CreateClient(clientName)
	if err != nil {
		return fmt.Errorf("failed to create client: %v", err)
	}

	infoHashes, err := client.SelectTorrents(clientInstance, category, tag, filter, args...)
	if err != nil {
		return err
	}
	if infoHashes == nil {
		err = clientInstance.PauseAllTorrents()
		if err != nil {
			return nil
		}
	} else if len(infoHashes) > 0 {
		err = clientInstance.PauseTorrents(infoHashes)
		if err != nil {
			return nil
		}
	}
	return nil
}
