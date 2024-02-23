package recheck

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/sagan/ptool/client"
	"github.com/sagan/ptool/cmd"
	"github.com/sagan/ptool/util"
)

var command = &cobra.Command{
	Use:         "recheck {client} [--category category] [--tag tag] [--filter filter] [infoHash]...",
	Annotations: map[string]string{"cobra-prompt-dynamic-suggestions": "recheck"},
	Short:       "Recheck torrents of client.",
	Long: `Recheck torrents of client.
[infoHash]...: infoHash list of torrents. It's possible to use state filter to target multiple torrents:
_all, _active, _done, _undone, _downloading, _seeding, _paused, _completed, _error.
`,
	Args: cobra.MatchAll(cobra.MinimumNArgs(1), cobra.OnlyValidArgs),
	RunE: recheck,
}

var (
	category = ""
	tag      = ""
	filter   = ""
	doAction = false
)

func init() {
	command.Flags().StringVarP(&filter, "filter", "", "", "Filter torrents by name")
	command.Flags().StringVarP(&category, "category", "", "", "Filter torrents by category")
	command.Flags().StringVarP(&tag, "tag", "", "",
		"Filter torrents by tag. Comma-separated list. Torrent which tags contain any one in the list matches")
	command.Flags().BoolVarP(&doAction, "do", "", false, "Do recheck torrents without asking for confirm")
	cmd.RootCmd.AddCommand(command)
}

func recheck(cmd *cobra.Command, args []string) error {
	clientName := args[0]
	infoHashes := args[1:]
	if category == "" && tag == "" && filter == "" && len(infoHashes) == 0 {
		return fmt.Errorf("you must provide at least a condition flag or hashFilter")
	}
	clientInstance, err := client.CreateClient(clientName)
	if err != nil {
		return fmt.Errorf("failed to create client: %v", err)
	}
	quickMode := true
	if category != "" || tag != "" || filter != "" {
		quickMode = false
	} else {
		for _, infoHash := range infoHashes {
			if !client.IsValidInfoHash(infoHash) {
				quickMode = false
				break
			}
		}
	}

	torrents, err := client.QueryTorrents(clientInstance, category, tag, filter, infoHashes...)
	if err != nil {
		return fmt.Errorf("failed to fetch client torrents: %v", err)
	}
	if len(torrents) == 0 {
		return nil
	}
	if !quickMode && !doAction {
		size := int64(0)
		for _, torrent := range torrents {
			size += torrent.Size
		}
		if !util.AskYesNoConfirm(fmt.Sprintf(
			"Will recheck %d (%s) torrents. Note the checking process can NOT be stopped once started",
			len(torrents), util.BytesSizeAround(float64(size)))) {
			return fmt.Errorf("abort")
		}
	}
	infoHashes = util.Map(torrents, func(t client.Torrent) string { return t.InfoHash })
	return clientInstance.RecheckTorrents(infoHashes)
}
