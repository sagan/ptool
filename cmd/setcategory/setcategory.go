package setcategory

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/sagan/ptool/client"
	"github.com/sagan/ptool/cmd"
)

var command = &cobra.Command{
	Use:         "setcategory {client} {category} [--category category] [--tag tag] [--filter filter] [infoHash]...",
	Annotations: map[string]string{"cobra-prompt-dynamic-suggestions": "setcategory"},
	Short:       "Set category of torrents in client.",
	Long: `Set category of torrents in client.
[infoHash]...: infoHash list of torrents. It's possible to use state filter to target multiple torrents:
_all, _active, _done, _undone, _downloading, _seeding, _paused, _completed, _error.`,
	Args: cobra.MatchAll(cobra.MinimumNArgs(2), cobra.OnlyValidArgs),
	RunE: createtags,
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

func createtags(cmd *cobra.Command, args []string) error {
	clientName := args[0]
	cat := args[1]
	args = args[2:]
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
		err = clientInstance.SetAllTorrentsCatetory(cat)
		if err != nil {
			return err
		}
	} else if len(infoHashes) > 0 {
		err = clientInstance.SetTorrentsCatetory(infoHashes, cat)
		if err != nil {
			return err
		}
	}
	return nil
}
