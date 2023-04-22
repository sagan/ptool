package delete

import (
	"github.com/sagan/ptool/client"
	"github.com/sagan/ptool/cmd"
	"github.com/spf13/cobra"

	log "github.com/sirupsen/logrus"
)

var command = &cobra.Command{
	Use:   "delete <client> <infoHash>...",
	Short: "Delete torrents from client.",
	Long:  `Delete torrents from client.`,
	Args:  cobra.MatchAll(cobra.MinimumNArgs(2), cobra.OnlyValidArgs),
	Run:   delete,
}

var (
	preserve = false
)

func init() {
	command.Flags().BoolVarP(&preserve, "preserve", "p", false, "Preserve (don't delete) torrent files on the hard disk.")
	cmd.RootCmd.AddCommand(command)
}

func delete(cmd *cobra.Command, args []string) {
	clientInstance, err := client.CreateClient(args[0])
	if err != nil {
		log.Fatal(err)
	}
	infoHashes := args[1:]
	err = clientInstance.DeleteTorrents(infoHashes, !preserve)
	if err != nil {
		log.Fatal(err)
	}
}
