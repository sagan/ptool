package setsharelimits

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/sagan/ptool/client"
	"github.com/sagan/ptool/cmd"
	"github.com/sagan/ptool/util"
	"github.com/sagan/ptool/util/helper"
)

var command = &cobra.Command{
	Use: "setsharelimits {client} [--category category] [--tag tag] [--filter filter] " +
		"{--ratio-limit limit} {--seeding-time-limit limit} [infoHash]...",
	Annotations: map[string]string{"cobra-prompt-dynamic-suggestions": "setcategory"},
	Short:       "Set share limits of torrents in client.",
	Long: `Set share limits of torrents in client.
[infoHash]...: infoHash list of torrents. It's possible to use state filter to target multiple torrents:
_all, _active, _done, _undone, _downloading, _seeding, _paused, _completed, _error.
Specially, use a single "-" as args to read infoHash list from stdin, delimited by blanks.

At least one of the following flags must be set:
* --ratio-limit : qb ratioLimit. For now, -2 means the global limit should be used, -1 means no limit.
* --seeding-time-limit : qb seedingTimeLimit (but in seconds instead of minutes).
  For now, -2 means the global limit should be used, -1 means no limit.`,
	Args: cobra.MatchAll(cobra.MinimumNArgs(2), cobra.OnlyValidArgs),
	RunE: setsharelimits,
}

var (
	seedingTimeLimit = int64(0)
	ratioLimit       = float64(0)
	category         = ""
	tag              = ""
	filter           = ""
)

func init() {
	command.Flags().Int64VarP(&seedingTimeLimit, "seeding-time-limit", "", 0,
		"If != 0, the max amount of time (seconds) the torrent should be seeded. Negative value has special meaning")
	command.Flags().Float64VarP(&ratioLimit, "ratio-limit", "", 0,
		"If != 0, the max ratio (Up/Dl) the torrent should be seeded until. Negative value has special meaning")
	command.Flags().StringVarP(&filter, "filter", "", "", "Filter torrents by name")
	command.Flags().StringVarP(&category, "category", "", "", "Filter torrents by category")
	command.Flags().StringVarP(&tag, "tag", "", "",
		"Filter torrents by tag. Comma-separated list. Torrent which tags contain any one in the list matches")
	cmd.RootCmd.AddCommand(command)
}

func setsharelimits(cmd *cobra.Command, args []string) error {
	clientName := args[0]
	infoHashes := args[1:]
	if util.CountNonZeroVariables(ratioLimit, seedingTimeLimit) == 0 {
		return fmt.Errorf(`at least one of --ratio-limit and --seeding-time-limit flags must be set`)
	}
	if category == "" && tag == "" && filter == "" {
		if len(infoHashes) == 0 {
			return fmt.Errorf("you must provide at least a condition flag or hashFilter")
		}
		if len(infoHashes) == 1 && infoHashes[0] == "-" {
			if data, err := helper.ReadArgsFromStdin(); err != nil {
				return fmt.Errorf("failed to parse stdin to info hashes: %v", err)
			} else if len(data) == 0 {
				return nil
			} else {
				infoHashes = data
			}
		}
	}
	clientInstance, err := client.CreateClient(clientName)
	if err != nil {
		return fmt.Errorf("failed to create client: %v", err)
	}

	infoHashes, err = client.SelectTorrents(clientInstance, category, tag, filter, infoHashes...)
	if err != nil {
		return err
	}
	if infoHashes == nil {
		err = clientInstance.SetAllTorrentsShareLimits(ratioLimit, seedingTimeLimit)
		if err != nil {
			return err
		}
	} else if len(infoHashes) > 0 {
		err = clientInstance.SetTorrentsShareLimits(infoHashes, ratioLimit, seedingTimeLimit)
		if err != nil {
			return err
		}
	}
	return nil
}
