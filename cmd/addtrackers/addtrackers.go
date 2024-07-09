package addtrackers

import (
	"fmt"
	"os"
	"strings"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/sagan/ptool/client"
	"github.com/sagan/ptool/cmd"
	"github.com/sagan/ptool/constants"
	"github.com/sagan/ptool/util"
	"github.com/sagan/ptool/util/helper"
)

var command = &cobra.Command{
	Use:         "addtrackers {client} [--category category] [--tag tag] [--filter filter] [infoHash]...",
	Aliases:     []string{"addtracker"},
	Annotations: map[string]string{"cobra-prompt-dynamic-suggestions": "addtrackers"},
	Short:       "Add new trackers to torrents of client.",
	Long: fmt.Sprintf(`Add new trackers to torrents of client.
%s.

Example:
  ptool addtrackers <client> <infoHashes...> --tracker "https://..."
The --tracker flag can be set many times.

It will ask for confirmation, unless --force flag is set.

If "--remove-existing" flag is set, torrents' all existing trackers will be removed,
making the new added tracker(s) become their only (sole) tracker(s).`, constants.HELP_INFOHASH_ARGS),
	Args: cobra.MatchAll(cobra.MinimumNArgs(1), cobra.OnlyValidArgs),
	RunE: addtrackers,
}

var (
	removeExisting = false
	force          = false
	category       = ""
	tag            = ""
	filter         = ""
	oldTracker     = ""
	trackers       = []string{}
)

func init() {
	command.Flags().BoolVarP(&removeExisting, "remove-existing", "", false,
		`Remove all existing trackers. The added tracker(s) will become the only (sole) tracker(s) of torrent`)
	command.Flags().BoolVarP(&force, "force", "", false, "Force updating trackers. Do NOT prompt for confirm")
	command.Flags().StringVarP(&filter, "filter", "", "", constants.HELP_ARG_FILTER_TORRENT)
	command.Flags().StringVarP(&category, "category", "", "", constants.HELP_ARG_CATEGORY)
	command.Flags().StringVarP(&tag, "tag", "", "", constants.HELP_ARG_TAG)
	command.Flags().StringVarP(&oldTracker, "old-tracker", "", "",
		"The existing tracker domain or url. If set, only torrents that already have this tracker will get new tracker")
	command.Flags().StringArrayVarP(&trackers, "tracker", "", nil, "Set the tracker to add. Can be set multiple times")
	command.MarkFlagRequired("tracker")
	cmd.RootCmd.AddCommand(command)
}

func addtrackers(cmd *cobra.Command, args []string) error {
	clientName := args[0]
	infoHashes := args[1:]
	if category == "" && tag == "" && filter == "" {
		if _infoHashes, err := helper.ParseInfoHashesFromArgs(infoHashes); err != nil {
			return err
		} else {
			infoHashes = _infoHashes
		}
	}
	if len(trackers) == 0 {
		return fmt.Errorf("at least one --tracker flag must be set")
	}
	for _, tracker := range trackers {
		if !util.IsUrl(tracker) {
			return fmt.Errorf("the provided tracker %s is not a valid URL", tracker)
		}
	}
	clientInstance, err := client.CreateClient(clientName)
	if err != nil {
		return fmt.Errorf("failed to create client: %w", err)
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
		condition := ""
		if oldTracker != "" {
			condition += fmt.Sprintf("If they currently have this tracker (domain or url): %q", oldTracker)
		}
		if !helper.AskYesNoConfirm(fmt.Sprintf(
			`Will update above %d torrents, add the following trackers to them (Remove their existing trackers: %t):
-----
%s
-----
%s`, len(torrents), removeExisting, strings.Join(trackers, "\n"), condition)) {
			return fmt.Errorf("abort")
		}
	}
	errorCnt := int64(0)
	for i, torrent := range torrents {
		fmt.Printf("(%d/%d) ", i+1, len(torrents))
		fmt.Printf("Add trackers to torrent %s (%s)\n", torrent.InfoHash, torrent.Name)
		err := clientInstance.AddTorrentTrackers(torrent.InfoHash, trackers, oldTracker, removeExisting)
		if err != nil {
			log.Errorf("Failed to add trackers: %v\n", err)
			errorCnt++
		}
	}
	if errorCnt > 0 {
		return fmt.Errorf("%d errors", errorCnt)
	}
	return nil
}
