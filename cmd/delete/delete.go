package delete

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/sagan/ptool/client"
	"github.com/sagan/ptool/cmd"
)

var command = &cobra.Command{
	Use:     "delete {client} {infoHash}...",
	Aliases: []string{"rm"},
	Short:   "Delete torrents from client.",
	Long: `Delete torrents from client.
{infoHash}...: infoHash list of torrents.`,
	Args: cobra.MatchAll(cobra.MinimumNArgs(2), cobra.OnlyValidArgs),
	RunE: delete,
}

var (
	preserve = false
)

func init() {
	command.Flags().BoolVarP(&preserve, "preserve", "p", false, "Preserve (don't delete) torrent content files on the disk")
	cmd.RootCmd.AddCommand(command)
}

func delete(cmd *cobra.Command, args []string) error {
	clientName := args[0]
	infoHashes := args[1:]
	clientInstance, err := client.CreateClient(clientName)
	if err != nil {
		return fmt.Errorf("failed to create client: %v", err)
	}

	err = clientInstance.DeleteTorrents(infoHashes, !preserve)
	if err != nil {
		return fmt.Errorf("failed to delete torrent: %v", err)
	}
	return nil
}
