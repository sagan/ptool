package status

import (
	"fmt"
	"log"
	"os"

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
	Use:   "status ...clients",
	Args:  cobra.MatchAll(cobra.MinimumNArgs(1), cobra.OnlyValidArgs),
	Short: "Show clients status",
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
	clientnames := args
	isSingle := len(clientnames) == 1
	hasError := false
	for i, clientname := range clientnames {
		if i > 0 {
			fmt.Printf("\n")
		}
		client, err := client.CreateClient(clientname)
		if err != nil {
			log.Printf("Failed to create client %s: %v", clientname, err)
			hasError = true
			continue
		}
		status, err := client.GetStatus()
		if err != nil {
			log.Printf("Cann't get client %s status: error=%v", client.GetName(), err)
			hasError = true
			continue
		}
		fmt.Printf("Client %s Download Speed/Limit: %s/s / %s/s; Upload Speed/Limit: %s/s / %s/s; Free disk space: %s\n",
			client.GetName(),
			utils.BytesSize(float64(status.DownloadSpeed)),
			utils.BytesSize(float64(status.DownloadSpeedLimit)),
			utils.BytesSize(float64(status.UploadSpeed)),
			utils.BytesSize(float64(status.UploadSpeedLimit)),
			utils.BytesSize(float64(status.FreeSpaceOnDisk)),
		)

		if isSingle || showAll {
			torrents, err := client.GetTorrents("", category, !isSingle && showAll)
			if err != nil {
				log.Printf("Cann't get client %s torrents: %v", clientname, err)
				hasError = true
				continue
			}
			fmt.Printf("\nName  InfoHash  Tracker  State  DlSpeed  UpSpeed  Meta\n")
			for _, torrent := range torrents {
				if filter != "" && !utils.ContainsI(torrent.Name, filter) && !utils.ContainsI(torrent.InfoHash, filter) {
					continue
				}
				fmt.Printf("%s  %s  %s  %s %s/s %s/s %v\n",
					torrent.Name,
					torrent.InfoHash,
					torrent.TrackerDomain,
					torrent.State,
					utils.BytesSize(float64(torrent.DownloadSpeed)),
					utils.BytesSize(float64(torrent.UploadSpeed)),
					torrent.Meta,
				)
			}
		}
	}
	if hasError {
		os.Exit(1)
	}
}
