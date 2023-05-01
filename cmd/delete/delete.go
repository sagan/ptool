package delete

import (
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/sagan/ptool/client"
	"github.com/sagan/ptool/cmd"
)

var command = &cobra.Command{
	Use:   "delete <client> <infoHash>...",
	Short: "Delete torrents from client",
	Long: `Delete torrents from client
<infoHash>...: infoHash list of torrents.`,
	Args: cobra.MatchAll(cobra.MinimumNArgs(2), cobra.OnlyValidArgs),
	Run:  delete,
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
