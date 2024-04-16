package recheck

import (
	"fmt"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/sagan/ptool/client"
	"github.com/sagan/ptool/cmd"
	"github.com/sagan/ptool/constants"
	"github.com/sagan/ptool/util"
	"github.com/sagan/ptool/util/helper"
)

var command = &cobra.Command{
	Use:         "recheck {client} [--category category] [--tag tag] [--filter filter] [infoHash]...",
	Annotations: map[string]string{"cobra-prompt-dynamic-suggestions": "recheck"},
	Short:       "Recheck torrents of client.",
	Long: fmt.Sprintf(`Recheck torrents of client.
%s.`, constants.HELP_INFOHASH_ARGS),
	Args: cobra.MatchAll(cobra.MinimumNArgs(1), cobra.OnlyValidArgs),
	RunE: recheck,
}

var (
	category = ""
	tag      = ""
	filter   = ""
	force    = false
)

func init() {
	command.Flags().StringVarP(&filter, "filter", "", "", "Filter torrents by name")
	command.Flags().StringVarP(&category, "category", "", "", "Filter torrents by category")
	command.Flags().StringVarP(&tag, "tag", "", "",
		"Filter torrents by tag. Comma-separated list. Torrent which tags contain any one in the list matches")
	command.Flags().BoolVarP(&force, "force", "", false, "Do recheck torrents without asking for confirm")
	cmd.RootCmd.AddCommand(command)
}

func recheck(cmd *cobra.Command, args []string) error {
	clientName := args[0]
	infoHashes := args[1:]
	infohashesOnly := true
	if category != "" || tag != "" || filter != "" {
		infohashesOnly = false
	} else {
		if _infoHashes, err := helper.ParseInfoHashesFromArgs(infoHashes); err != nil {
			return err
		} else {
			infoHashes = _infoHashes
		}
		for _, infoHash := range infoHashes {
			if !client.IsValidInfoHash(infoHash) {
				infohashesOnly = false
				break
			}
		}
	}
	clientInstance, err := client.CreateClient(clientName)
	if err != nil {
		return fmt.Errorf("failed to create client: %v", err)
	}
	if infohashesOnly {
		if len(infoHashes) == 0 {
			return fmt.Errorf("no torrent to recheck")
		}
		if force {
			if err = clientInstance.RecheckTorrents(infoHashes); err != nil {
				return fmt.Errorf("failed to recheck torrents: %v", err)
			}
			return nil
		}
	}

	torrents, err := client.QueryTorrents(clientInstance, category, tag, filter, infoHashes...)
	if err != nil {
		return fmt.Errorf("failed to fetch client torrents: %v", err)
	}
	if len(torrents) == 0 {
		log.Infof("No matched torrents found")
		return nil
	}
	if !force {
		size := int64(0)
		for _, torrent := range torrents {
			size += torrent.Size
		}
		if !helper.AskYesNoConfirm(fmt.Sprintf(
			"Will recheck %d (%s) torrents. Note the checking process can NOT be stopped once started",
			len(torrents), util.BytesSizeAround(float64(size)))) {
			return fmt.Errorf("abort")
		}
	}
	infoHashes = util.Map(torrents, func(t client.Torrent) string { return t.InfoHash })
	return clientInstance.RecheckTorrents(infoHashes)
}
