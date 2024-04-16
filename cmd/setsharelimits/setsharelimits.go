package setsharelimits

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/sagan/ptool/client"
	"github.com/sagan/ptool/cmd"
	"github.com/sagan/ptool/constants"
	"github.com/sagan/ptool/util"
	"github.com/sagan/ptool/util/helper"
)

var command = &cobra.Command{
	Use: "setsharelimits {client} [--category category] [--tag tag] [--filter filter] " +
		"{--ratio-limit limit} {--seeding-time-limit limit} [infoHash]...",
	Annotations: map[string]string{"cobra-prompt-dynamic-suggestions": "setcategory"},
	Short:       "Set share limits of torrents in client.",
	Long: fmt.Sprintf(`Set share limits of torrents in client.
%s.

At least one of the following flags must be set:
* --ratio-limit : qb ratioLimit. For now, -2 means the global limit should be used, -1 means no limit.
* --seeding-time-limit : qb seedingTimeLimit (but in seconds instead of minutes).
  For now, -2 means the global limit should be used, -1 means no limit.`, constants.HELP_INFOHASH_ARGS),
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
		if _infoHashes, err := helper.ParseInfoHashesFromArgs(infoHashes); err != nil {
			return err
		} else {
			infoHashes = _infoHashes
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
