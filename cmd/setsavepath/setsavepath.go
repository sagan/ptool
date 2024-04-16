package setsavepath

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/sagan/ptool/client"
	"github.com/sagan/ptool/cmd"
	"github.com/sagan/ptool/constants"
	"github.com/sagan/ptool/util/helper"
)

var command = &cobra.Command{
	Use:         "setsavepath {client} {savePath} [--category category] [--tag tag] [--filter filter] [infoHash]...",
	Annotations: map[string]string{"cobra-prompt-dynamic-suggestions": "setsavepath"},
	Short:       "Set the save path of torrents in client.",
	Long: fmt.Sprintf(`Set the save path of torrents in client.
%s.`, constants.HELP_INFOHASH_ARGS),
	Args: cobra.MatchAll(cobra.MinimumNArgs(2), cobra.OnlyValidArgs),
	RunE: setsavepath,
}

var (
	category = ""
	tag      = ""
	filter   = ""
)

func init() {
	command.Flags().StringVarP(&filter, "filter", "", "", "Filter torrents by name")
	command.Flags().StringVarP(&category, "category", "", "", "Filter torrents by category")
	command.Flags().StringVarP(&tag, "tag", "", "",
		"Filter torrents by tag. Comma-separated list. Torrent which tags contain any one in the list matches")
	cmd.RootCmd.AddCommand(command)
}

func setsavepath(cmd *cobra.Command, args []string) error {
	clientName := args[0]
	savePath := args[1]
	infoHashes := args[2:]
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
		err = clientInstance.SetAllTorrentsSavePath(savePath)
		if err != nil {
			return err
		}
	} else if len(infoHashes) > 0 {
		err = clientInstance.SetTorrentsSavePath(infoHashes, savePath)
		if err != nil {
			return err
		}
	}
	return nil
}
