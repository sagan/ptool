package dltorrent

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/sagan/ptool/cmd"
	"github.com/sagan/ptool/util/helper"
	"github.com/sagan/ptool/util/torrentutil"
)

var command = &cobra.Command{
	Use:         "dltorrent {torrentId | torrentUrl}... [--dir dir]",
	Annotations: map[string]string{"cobra-prompt-dynamic-suggestions": "dltorrent"},
	Short:       "Download site torrents to local.",
	Long: `Download site torrents to local.
Args is torrent list that each one could be a site torrent id (e.g.: "mteam.488424")
or url (e.g.: "https://kp.m-team.cc/details.php?id=488424").
Torrent url that does NOT belong to any site (e.g.: a public site url) is also supported.

--rename <name> flag supports the following variable placeholders:
* [size] : Torrent size
* [id] :  Torrent id in site
* [site] : Torrent site
* [filename] : Original torrent filename without ".torrent" extension
* [name] : Torrent name`,
	Args: cobra.MatchAll(cobra.MinimumNArgs(1), cobra.OnlyValidArgs),
	RunE: dltorrent,
}

var (
	downloadDir = ""
	rename      = ""
	defaultSite = ""
)

func init() {
	command.Flags().StringVarP(&defaultSite, "site", "", "", "Set default site of torrents")
	command.Flags().StringVarP(&downloadDir, "dir", "", ".", `Set the dir of downloaded torrents`)
	command.Flags().StringVarP(&rename, "rename", "", "", "Rename downloaded torrents (supports variables)")
	cmd.RootCmd.AddCommand(command)
}

func dltorrent(cmd *cobra.Command, args []string) error {
	errorCnt := int64(0)
	torrents := args

	for _, torrent := range torrents {
		content, tinfo, _, siteName, filename, id, err :=
			helper.GetTorrentContent(torrent, defaultSite, false, true, nil, true)
		if err != nil {
			fmt.Printf("✕ %s (site=%s): %v\n", torrent, siteName, err)
			errorCnt++
			continue
		}
		fileName := ""
		if rename == "" {
			fileName = filename
		} else {
			fileName = torrentutil.RenameTorrent(rename, siteName, id, filename, tinfo)
		}
		err = os.WriteFile(downloadDir+"/"+fileName, content, 0666)
		if err != nil {
			fmt.Printf("✕ %s (site=%s): failed to save to %s/: %v\n", fileName, siteName, downloadDir, err)
			errorCnt++
		} else {
			fmt.Printf("✓ %s (site=%s): saved to %s/\n", fileName, siteName, downloadDir)
		}
	}
	if errorCnt > 0 {
		return fmt.Errorf("%d errors", errorCnt)
	}
	return nil
}
