package brush

import (
	"bytes"
	"log"
	"os"

	goTorrentParser "github.com/j-muller/go-torrent-parser"

	"github.com/sagan/ptool/client"
	"github.com/sagan/ptool/cmd"
	"github.com/sagan/ptool/site"
	"github.com/sagan/ptool/utils"
	"github.com/spf13/cobra"
)

const (
	CAT = "_brush"
)

var command = &cobra.Command{
	Use:   "brush client site",
	Short: "Brush site using client",
	Long:  `A longer description`,
	Args:  cobra.MatchAll(cobra.ExactArgs(2), cobra.OnlyValidArgs),
	Run:   brush,
}

var (
	dryRun = false
	paused = false
)

func init() {
	command.Flags().BoolVar(&dryRun, "dry-run", false, "Dry run. Do not actually controlling client")
	command.Flags().BoolVar(&paused, "paused", false, "Add torrents to client in paused state")
	cmd.RootCmd.AddCommand(command)
}

func brush(cmd *cobra.Command, args []string) {
	clientInstance, err := client.CreateClient(args[0])
	if err != nil {
		log.Fatal(err)
	}
	siteInstance, err := site.CreateSite(args[1])
	if err != nil {
		log.Fatal(err)
	}
	status, err := clientInstance.GetStatus()
	if err != nil {
		log.Fatal(err)
	}
	var siteTorrents []site.SiteTorrent
	if status.UploadSpeedLimit > 0 && status.UploadSpeedLimit < 100*1024 ||
		status.UploadSpeed > 0 &&
			(float64(status.UploadSpeed)/float64(status.UploadSpeedLimit)) >= 0.8 {
		log.Printf(
			"Client %s upload bandwidth is already full (Upload speed/limit: %s/s/%s/s). Do not fetch site new torrents\n",
			clientInstance.GetName(),
			utils.HumanSize(float64(status.UploadSpeed)),
			utils.HumanSize(float64(status.UploadSpeedLimit)),
		)
	} else {
		url := ""
		if siteInstance.GetSiteConfig().BrushUrl != "" {
			url = siteInstance.GetSiteConfig().BrushUrl
		}
		siteTorrents, _ = siteInstance.GetLatestTorrents(url)
		log.Printf("Get %d torrents\n", len(siteTorrents))
	}

	clientTorrents, err := clientInstance.GetTorrents("", CAT, true)
	if err != nil {
		log.Fatal(err)
	}
	result := Decide(status, clientTorrents, siteTorrents)
	log.Printf(
		`Current client %s torrents: %d; Download speed / limit: %s/s / %s/s; Upload speed / limit: %s/s / %s/s;Free disk space: %s;\n`,
		clientInstance.GetName(),
		len(clientTorrents),
		utils.HumanSize(float64(status.DownloadSpeed)),
		utils.HumanSize(float64(status.DownloadSpeedLimit)),
		utils.HumanSize(float64(status.UploadSpeed)),
		utils.HumanSize(float64(status.UploadSpeedLimit)),
		utils.HumanSize(float64(status.FreeSpaceOnDisk)),
	)
	log.Printf(
		`Fetched site %s torrents: %d; Client add / modify / stall / delete torrents: %d / %d / %d / %d. Msg: %s\n`,
		siteInstance.GetName(),
		len(siteTorrents),
		len(result.AddTorrents),
		len(result.ModifyTorrents),
		len(result.StallTorrents),
		len(result.DeleteTorrents),
		result.Msg,
	)
	tmpdir, _ := os.MkdirTemp(os.TempDir(), "ptool")
	for _, torrent := range result.AddTorrents {
		log.Printf("Add site %s torrent to client %s: %s / %s / %v", siteInstance.GetName(), clientInstance.GetName(), torrent.Name, torrent.Msg, torrent.Meta)
		if dryRun {
			continue
		}
		torrentdata, err := siteInstance.DownloadTorrent(torrent.DownloadUrl)
		if err != nil {
			log.Printf("Failed to download: %s. Skip \n", err)
			continue
		}
		tinfo, err := goTorrentParser.Parse(bytes.NewReader(torrentdata))
		if err != nil {
			continue
		}
		pClientTorrent := utils.FindInSlice(clientTorrents, func(ts client.Torrent) bool {
			return ts.InfoHash == tinfo.InfoHash
		})
		if pClientTorrent != nil {
			log.Printf("Already existing in client. skip\n")
			continue
		}
		log.Printf("torrent info: %s\n", tinfo.InfoHash)
		torrentOption := &client.TorrentOption{
			Name:     torrent.Name,
			Paused:   paused,
			Category: CAT,
		}
		// torrentname := fmt.Sprint(torrent.Name, "_", i, ".torrent")
		// os.WriteFile(tmpdir+"/"+torrentname, torrentdata, 0777)
		if !dryRun {
			err = clientInstance.AddTorrent(torrentdata, torrentOption, torrent.Meta)
			log.Printf("Add torrent result: error=%v", err)
		}
	}

	for _, torrent := range result.ModifyTorrents {
		log.Printf("Modify client %s torrent: %v / %v / %v / %v ", clientInstance.GetName(), torrent.Name, torrent.InfoHash, torrent.Msg, torrent.Meta)
		if dryRun {
			continue
		}
		err := clientInstance.ModifyTorrent(torrent.InfoHash, nil, torrent.Meta)
		log.Printf("Modify torrent result: error=%v", err)
	}

	for _, torrent := range result.StallTorrents {
		log.Printf("Stall client %s torrent: %v / %v / %v", clientInstance.GetName(), torrent.Name, torrent.InfoHash, torrent.Msg)
		if dryRun {
			continue
		}
		err := clientInstance.ModifyTorrent(torrent.InfoHash, &client.TorrentOption{
			DownloadSpeedLimit: 1,
		}, nil)
		log.Printf("Stall torrent result: error=%v", err)
	}

	for _, torrent := range result.DeleteTorrents {
		clientTorrent := utils.FindInSlice(clientTorrents, func(t client.Torrent) bool {
			return t.InfoHash == torrent.InfoHash
		})
		log.Printf("Delete client %s torrent: %v / %v / %v.", clientInstance.GetName(), torrent.Name, torrent.InfoHash, torrent.Msg)
		log.Printf("Total downloads / uploads: %s / %s;",
			utils.HumanSize(float64(clientTorrent.Downloaded)),
			utils.HumanSize(float64(clientTorrent.Uploaded)),
		)
		if dryRun {
			continue
		}
		err := clientInstance.DeleteTorrents([]string{torrent.InfoHash})
		log.Printf("Delete torrent result: error=%v", err)
	}

	os.RemoveAll(tmpdir)
}
