package renametag

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/sagan/ptool/client"
	"github.com/sagan/ptool/cmd"
	"github.com/sagan/ptool/util"
)

var command = &cobra.Command{
	Use:         "renametag {client} {old-tag(s)} {new-tag}",
	Aliases:     []string{"renametags"},
	Annotations: map[string]string{"cobra-prompt-dynamic-suggestions": "renametag"},
	Short:       "Rename tag in client.",
	Long: `Rename tag in client.
Note currently it works by adding new tag to torrents and then deleting old tag(s) from client.
{old-tag(s)}: comma-separated list, all tags in list will be "renamed" to new tag`,
	Args: cobra.MatchAll(cobra.ExactArgs(3), cobra.OnlyValidArgs),
	RunE: renametag,
}

func init() {
	cmd.RootCmd.AddCommand(command)
}

func renametag(cmd *cobra.Command, args []string) error {
	clientName := args[0]
	oldTag := args[1]
	newTag := args[2]
	if oldTag == "" || newTag == "" {
		return fmt.Errorf("old-tag and new-tag can NOT be empty")
	}
	clientInstance, err := client.CreateClient(clientName)
	if err != nil {
		return fmt.Errorf("failed to create client: %v", err)
	}
	torrents, err := client.QueryTorrents(clientInstance, "", oldTag, "")
	if err != nil {
		return fmt.Errorf("failed to query client torrents of old-tag: %v", err)
	}
	if len(torrents) > 0 {
		infoHashes := util.Map(torrents, func(t client.Torrent) string { return t.InfoHash })
		err = clientInstance.AddTagsToTorrents(infoHashes, []string{newTag})
		if err != nil {
			return fmt.Errorf("failed to add new-tag to client torrents: %v", err)
		}
	} else {
		err = clientInstance.CreateTags(newTag)
		if err != nil {
			return fmt.Errorf("failed to create new-tag in client: %v", err)
		}
	}
	err = clientInstance.DeleteTags(util.SplitCsv(oldTag)...)
	if err != nil {
		return fmt.Errorf("failed to delete old-tag(s) from client: %v", err)
	}
	return nil
}
