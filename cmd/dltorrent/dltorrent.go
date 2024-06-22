package dltorrent

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/natefinch/atomic"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/sagan/ptool/cmd"
	"github.com/sagan/ptool/constants"
	"github.com/sagan/ptool/util"
	"github.com/sagan/ptool/util/helper"
	"github.com/sagan/ptool/util/torrentutil"
)

var command = &cobra.Command{
	Use:         "dltorrent {torrentId | torrentUrl}... [--dir dir]",
	Annotations: map[string]string{"cobra-prompt-dynamic-suggestions": "dltorrent"},
	Short:       "Download site torrents to local.",
	Long: `Download site torrents to local.
Args is torrent list that each one could be a site torrent id (e.g. "mteam.488424")
or url (e.g. "https://kp.m-team.cc/details.php?id=488424").
Torrent url that does NOT belong to any site (e.g. a public site url) is also supported.
Use a single "-" as args to read torrent (id or url) list from stdin, delimited by blanks.

To set the filename of downloaded torrent, use --rename <name> flag,
which supports the following variable placeholders:
* [size] : Torrent size
* [id] :  Torrent id in site
* [site] : Torrent site
* [filename] : Original torrent filename without ".torrent" extension
* [filename128] : The prefix of [filename] which is at max 128 bytes
* [name] : Torrent name
* [name128] : The prefix of torrent name which is at max 128 bytes`,
	Args: cobra.MatchAll(cobra.MinimumNArgs(1), cobra.OnlyValidArgs),
	RunE: dltorrent,
}

var (
	skipExisting    = false
	slowMode        = false
	downloadDir     = ""
	rename          = ""
	defaultSite     = ""
	errSkipExisting = errors.New("skip existing torrent")
)

func init() {
	command.Flags().BoolVarP(&skipExisting, "skip-existing", "", false,
		`Do NOT re-download torrent that same name file already exists in local dir. `+
			`If this flag is set, the download torrent filename ("--rename" flag) will be fixed to `+
			`"[site].[id].torrent" (e.g. "mteam.12345.torrent") format`)
	command.Flags().BoolVarP(&slowMode, "slow", "", false, "Slow mode. wait after downloading each torrent")
	command.Flags().StringVarP(&defaultSite, "site", "", "", "Set default site of torrents")
	command.Flags().StringVarP(&downloadDir, "download-dir", "", ".", `Set the dir of downloaded torrents. `+
		`Use "-" to directly output torrent content to stdout`)
	command.Flags().StringVarP(&rename, "rename", "", "", "Rename downloaded torrents (supports variables)")
	cmd.RootCmd.AddCommand(command)
}

// @todo: currently, --skip-existing flag will NOT work if (torrent) arg is a site torrent url,
// to fix it the site.Site interface must be changed to separate torrent url parsing from downloading.
func dltorrent(cmd *cobra.Command, args []string) error {
	errorCnt := int64(0)
	torrents := args
	if len(torrents) == 1 && torrents[0] == "-" {
		if data, err := helper.ReadArgsFromStdin(); err != nil {
			return fmt.Errorf("failed to parse stdin to info torrent ids: %w", err)
		} else if len(data) == 0 {
			return nil
		} else {
			torrents = data
		}
	}
	outputToStdout := false
	if downloadDir == "-" {
		if len(torrents) > 1 {
			return fmt.Errorf(`"--download-dir -" can only be used to download one torrent`)
		} else {
			outputToStdout = true
		}
	}
	var beforeDownload func(sitename string, id string) error
	if skipExisting {
		if rename != "" {
			return fmt.Errorf("--skip-existing and --rename flags are NOT compatible")
		}
		if outputToStdout {
			return fmt.Errorf(`--skip-existing can NOT be used with "--download-dir -"`)
		}
		beforeDownload = func(sitename, id string) error {
			if sitename != "" && id != "" {
				filename := fmt.Sprintf("%s.%s.torrent", sitename, id)
				if util.FileExistsWithOptionalSuffix(filepath.Join(downloadDir, filename),
					constants.ProcessedFilenameSuffixes...) {
					log.Debugf("Skip downloading local-existing torrent %s.%s", sitename, id)
					return errSkipExisting
				}
			}
			return nil
		}
	}
	for i, torrent := range torrents {
		if i > 0 && slowMode {
			util.Sleep(3)
		}
		content, tinfo, _, sitename, _filename, id, _, err :=
			helper.GetTorrentContent(torrent, defaultSite, false, true, nil, true, beforeDownload)
		if outputToStdout {
			if err != nil {
				errorCnt++
				fmt.Fprintf(os.Stderr, "Failed to download torrent: %v\n", err)
			} else if term.IsTerminal(int(os.Stdout.Fd())) {
				errorCnt++
				fmt.Fprintf(os.Stderr, "%s\n", constants.HELP_TIP_TTY_BINARY_OUTPUT)
			} else if _, err = os.Stdout.Write(content); err != nil {
				errorCnt++
				fmt.Fprintf(os.Stderr, "Failed to output torrent content to stdout: %v\n", err)
			}
			continue
		}
		if err != nil {
			if err == errSkipExisting {
				fmt.Printf("- %s (site=%s): skip due to exists in local dir (%s.%s.torrent)\n",
					torrent, sitename, sitename, id)
			} else {
				fmt.Printf("✕ %s (site=%s): %v\n", torrent, sitename, err)
				errorCnt++
			}
			continue
		}
		filename := ""
		if skipExisting && sitename != "" && id != "" {
			filename = fmt.Sprintf("%s.%s.torrent", sitename, strings.TrimPrefix(id, sitename+"."))
		} else if rename == "" {
			filename = _filename
		} else {
			filename = torrentutil.RenameTorrent(rename, sitename, id, _filename, tinfo)
		}
		err = atomic.WriteFile(filepath.Join(downloadDir, filename), bytes.NewReader(content))
		if err != nil {
			fmt.Printf("✕ %s (site=%s): failed to save to %s/: %v\n", filename, sitename, downloadDir, err)
			errorCnt++
		} else {
			fmt.Printf("✓ %s (site=%s): saved to %s/\n", filename, sitename, downloadDir)
		}
	}
	if errorCnt > 0 {
		return fmt.Errorf("%d errors", errorCnt)
	}
	return nil
}
