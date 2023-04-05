package status

import (
	"fmt"
	"log"

	"github.com/sagan/ptool/client"
	"github.com/sagan/ptool/cmd"
	"github.com/sagan/ptool/utils"
	"github.com/spf13/cobra"
)

var (
	filter   = ""
	category = ""
	showAll  = false
)

var command = &cobra.Command{
	Use:   "status <source>",
	Args:  cobra.MatchAll(cobra.ExactArgs(1), cobra.OnlyValidArgs),
	Short: "Show status",
	Long:  `A longer description`,
	Run:   status,
}

func init() {
	command.Flags().StringVar(&filter, "filter", "", "filter torrents by name")
	command.Flags().StringVar(&category, "category", "", "filter torrents by category")
	command.Flags().BoolVar(&showAll, "all", false, "show all torrents.")
	cmd.RootCmd.AddCommand(command)
}

func status(cmd *cobra.Command, args []string) {
	client, err := client.CreateClient(args[0])
	if err != nil {
		log.Fatal(err)
	}
	status, err := client.GetStatus()
	if err != nil {
		log.Fatalf("Cann't get client %s status: error=%v", client.GetName(), err)
	}
	fmt.Printf("Client %s Download Speed/Limit: %s/s / %s/s; Upload Speed/Limit: %s/s / %s/s; Free disk space: %s\n\n",
		client.GetName(),
		utils.HumanSize(float64(status.DownloadSpeed)),
		utils.HumanSize(float64(status.DownloadSpeedLimit)),
		utils.HumanSize(float64(status.UploadSpeed)),
		utils.HumanSize(float64(status.UploadSpeedLimit)),
		utils.HumanSize(float64(status.FreeSpaceOnDisk)),
	)

	torrents, err := client.GetTorrents("", category, showAll)
	if err != nil {
		log.Fatal("!!", err)
	}
	fmt.Printf("Name  InfoHash  Tracker  State  DlSpeed  UpSpeed  Meta\n")
	for _, torrent := range torrents {
		if filter != "" && !utils.ContainsI(torrent.Name, filter) && !utils.ContainsI(torrent.InfoHash, filter) {
			continue
		}
		fmt.Printf("%s  %s  %s  %s %s/s %s/s %v\n",
			torrent.Name,
			torrent.InfoHash,
			torrent.TrackerDomain,
			torrent.State,
			utils.HumanSize(float64(torrent.DownloadSpeed)),
			utils.HumanSize(float64(torrent.UploadSpeed)),
			torrent.Meta,
		)
	}
}
