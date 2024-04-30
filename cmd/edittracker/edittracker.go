package edittracker

import (
	"fmt"
	"os"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/sagan/ptool/client"
	"github.com/sagan/ptool/cmd"
	"github.com/sagan/ptool/constants"
	"github.com/sagan/ptool/util"
	"github.com/sagan/ptool/util/helper"
)

var command = &cobra.Command{
	Use: "edittracker <client> [--category category] [--tag tag] [--filter filter] [infoHash]... " +
		"--old-tracker {url} --new-tracker {url} [--replace-host]",
	Aliases:     []string{"replacetracker"},
	Annotations: map[string]string{"cobra-prompt-dynamic-suggestions": "edittracker"},
	Short:       "Edit tracker of torrents in client.",
	Long: fmt.Sprintf(`Edit tracker of torrents in client, replace the old tracker url with the new one.
%s.

A torrent will not be updated if old tracker does NOT exist in it's trackers list.
It may return an error in such case or not, depending on specific client implementation.

Examples:
  ptool edittracker <client> _all --old-tracker "https://..." --new-tracker "https://..."
  ptool edittracker <client> _all --old-tracker old-tracker.com --new-tracker new-tracker.com --replace-host
  ptool edittracker <client> _all --old-tracker old-tracker.com --new-tracker "https://..." --replace-host

It will ask for confirmation, unless --force flag is set.`, constants.HELP_INFOHASH_ARGS),
	Args: cobra.MatchAll(cobra.MinimumNArgs(1), cobra.OnlyValidArgs),
	RunE: edittracker,
}

var (
	force       = false
	replaceHost = false
	category    = ""
	tag         = ""
	filter      = ""
	oldTracker  = ""
	newTracker  = ""
)

func init() {
	command.Flags().BoolVarP(&force, "force", "", false, "Force updating trackers. Do NOT prompt for confirm")
	command.Flags().BoolVarP(&replaceHost, "replace-host", "", false,
		"Replace host mode. If set, --old-tracker should be the old host (hostname[:port]) instead of url, "+
			"the --new-tracker can either be a host or url")
	command.Flags().StringVarP(&filter, "filter", "", "", constants.HELP_ARG_FILTER_TORRENT)
	command.Flags().StringVarP(&category, "category", "", "", constants.HELP_ARG_CATEGORY)
	command.Flags().StringVarP(&tag, "tag", "", "", constants.HELP_ARG_TAG)
	command.Flags().StringVarP(&oldTracker, "old-tracker", "", "", "Set the old tracker")
	command.Flags().StringVarP(&newTracker, "new-tracker", "", "", "Set the new tracker")
	command.MarkFlagRequired("old-tracker")
	command.MarkFlagRequired("new-tracker")
	cmd.RootCmd.AddCommand(command)
}

func edittracker(cmd *cobra.Command, args []string) error {
	clientName := args[0]
	infoHashes := args[1:]
	if !replaceHost && (!util.IsUrl(oldTracker) || !util.IsUrl(newTracker)) {
		return fmt.Errorf("both --old-tracker and --new-tracker MUST be valid URL ( 'http(s)://...' )")
	}
	if category == "" && tag == "" && filter == "" {
		if _infoHashes, err := helper.ParseInfoHashesFromArgs(infoHashes); err != nil {
			return err
		} else {
			infoHashes = _infoHashes
		}
	}
	clientInstance, err := client.CreateClient(clientName)
	if err != nil {
		return fmt.Errorf("failed to create client: %v", err)
	}

	torrents, err := client.QueryTorrents(clientInstance, category, tag, filter, infoHashes...)
	if err != nil {
		return err
	}
	if len(torrents) == 0 {
		log.Infof("No matched torrents found")
		return nil
	}
	if !force {
		client.PrintTorrents(os.Stdout, torrents, "", 1, false)
		fmt.Printf("\n")
		if !helper.AskYesNoConfirm(fmt.Sprintf(
			`Will update above %d torrents, replace old tracker with new tracker:
-----
Old tracker: %s
New tracker: %s
ReplaceHost: %t
-----
If a torrent does NOT has the old tracker, it will NOT be updated`,
			len(torrents), oldTracker, newTracker, replaceHost)) {
			return fmt.Errorf("abort")
		}
	}
	errorCnt := int64(0)
	for _, torrent := range torrents {
		fmt.Printf("Edit torrent %s (%s) tracker\n", torrent.InfoHash, torrent.Name)
		err := clientInstance.EditTorrentTracker(torrent.InfoHash, oldTracker, newTracker, replaceHost)
		if err != nil {
			log.Errorf("Failed to edit tracker: %v\n", err)
			errorCnt++
		}
	}
	if errorCnt > 0 {
		return fmt.Errorf("%d errors", errorCnt)
	}
	return nil
}
