package addtags

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
	Use:         "addtags {client} {tags} [--category category] [--tag tag] [--filter filter] [infoHash]...",
	Aliases:     []string{"addtag"},
	Annotations: map[string]string{"cobra-prompt-dynamic-suggestions": "addtags"},
	Short:       "Add tags to torrents in client.",
	Long: fmt.Sprintf(`Add tags to torrents in client.
First arg ({tags}) is comma-seperated tags list. The following args is the args list.
%s.`, constants.HELP_INFOHASH_ARGS),
	Args: cobra.MatchAll(cobra.MinimumNArgs(2), cobra.OnlyValidArgs),
	RunE: addtags,
}

var (
	category = ""
	tag      = ""
	filter   = ""
)

func init() {
	command.Flags().StringVarP(&filter, "filter", "", "", constants.HELP_ARG_FILTER_TORRENT)
	command.Flags().StringVarP(&category, "category", "", "", constants.HELP_ARG_CATEGORY)
	command.Flags().StringVarP(&tag, "tag", "", "", constants.HELP_ARG_TAG)
	cmd.RootCmd.AddCommand(command)
}

func addtags(cmd *cobra.Command, args []string) error {
	clientName := args[0]
	tags := util.SplitCsv(args[1])
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
		return fmt.Errorf("failed to create client: %w", err)
	}

	infoHashes, err = client.SelectTorrents(clientInstance, category, tag, filter, infoHashes...)
	if err != nil {
		return err
	}
	if infoHashes == nil {
		err = clientInstance.AddTagsToAllTorrents(tags)
		if err != nil {
			return err
		}
	} else if len(infoHashes) > 0 {
		err = clientInstance.AddTagsToTorrents(infoHashes, tags)
		if err != nil {
			return err
		}
	}
	return nil
}
