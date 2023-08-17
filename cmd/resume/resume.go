package resume

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/sagan/ptool/client"
	"github.com/sagan/ptool/cmd"
)

var command = &cobra.Command{
	Use:         "resume {client} [--category category] [--tag tag] [--filter filter] [infoHash]...",
	Annotations: map[string]string{"cobra-prompt-dynamic-suggestions": "resume"},
	Aliases:     []string{"start"},
	Short:       "Resume torrents of client.",
	Long: `Resume torrents of client.
[infoHash]...: infoHash list of torrents. It's possible to use state filter to target multiple torrents:
_all, _active, _done, _undone, _downloading, _seeding, _paused, _completed, _error.
`,
	Args: cobra.MatchAll(cobra.MinimumNArgs(1), cobra.OnlyValidArgs),
	RunE: resume,
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

func resume(cmd *cobra.Command, args []string) error {
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
		err = clientInstance.ResumeAllTorrents()
		if err != nil {
			return err
		}
	} else if len(infoHashes) > 0 {
		err = clientInstance.ResumeTorrents(infoHashes)
		if err != nil {
			return err
		}
	}
	return nil
}
