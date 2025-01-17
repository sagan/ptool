package markinvalidtracker

import (
	"fmt"
	"strings"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/sagan/ptool/client"
	"github.com/sagan/ptool/client/transmission"
	"github.com/sagan/ptool/cmd"
	"github.com/sagan/ptool/config"
	"github.com/sagan/ptool/constants"
	"github.com/sagan/ptool/util"
	"github.com/sagan/ptool/util/helper"
)

var command = &cobra.Command{
	Use:         "markinvalidtracker {client} [--category category] [--tag tag] [--filter filter] [infoHash]...",
	Annotations: map[string]string{"cobra-prompt-dynamic-suggestions": "markinvalidtracker"},
	Aliases:     []string{"markinvalid"},
	Short: fmt.Sprintf(`Mark torrents in client which has invalid tracker with "%s*" tags.`,
		config.INVALID_TRACKER_TAG_PREFIX),
	Long: fmt.Sprintf(`Mark torrents in client which has invalid tracker with "%s*" tags.
%s.

It will check tracker status of torrents in client, mark those torrents which trackers status
is invalid with %q* tags. The tags prefix can be changed via "--tag-prefix" flag.

It detects various known invalidity tracker reasons,
assigns the following tags for torrents of different reasons:
%s

Note a torrent's trackers status is NOT treated as invalid if the tracker(s)
is currently inaccessible due to network problem or site server error.

Note it will first reset %q* tags, removing all torrents from which, before adding torrents to them`,
		config.INVALID_TRACKER_TAG_PREFIX, constants.HELP_INFOHASH_ARGS, config.INVALID_TRACKER_TAG_PREFIX,
		strings.Join(util.Map(client.TrackerValidityInfos[1:], func(i *client.TrackerValidityInfoStruct) string {
			return fmt.Sprintf("- %s%s : %s", config.INVALID_TRACKER_TAG_PREFIX, i.Name, i.Desc)
		}), "\n"),
		config.INVALID_TRACKER_TAG_PREFIX),
	Args: cobra.MatchAll(cobra.MinimumNArgs(1), cobra.OnlyValidArgs),
	RunE: markinvalidtracker,
}

var (
	noClean   = false
	category  = ""
	tag       = ""
	filter    = ""
	tagPrefix = ""
)

func init() {
	command.Flags().StringVarP(&filter, "filter", "", "", constants.HELP_ARG_FILTER_TORRENT)
	command.Flags().StringVarP(&tagPrefix, "tag-prefix", "", config.INVALID_TRACKER_TAG_PREFIX,
		"Mark found invalid tracker with tags of this prefix")
	command.Flags().StringVarP(&category, "category", "", "", constants.HELP_ARG_CATEGORY)
	command.Flags().BoolVarP(&noClean, "no-clean", "", false, `Do not clean existing torrents of "`+
		config.INVALID_TRACKER_TAG_PREFIX+`*" tags`)
	command.Flags().StringVarP(&tag, "tag", "", "", constants.HELP_ARG_TAG)
	cmd.RootCmd.AddCommand(command)
}

func markinvalidtracker(cmd *cobra.Command, args []string) error {
	clientName := args[0]
	infoHashes := args[1:]
	if category == "" && tag == "" && filter == "" {
		if _infoHashes, err := helper.ParseInfoHashesFromArgs(infoHashes); err != nil {
			return err
		} else {
			infoHashes = _infoHashes
		}
	}
	clientInstance, err := client.CreateClient(clientName)
	if err != nil {
		return fmt.Errorf("failed to create client: %w", err)
	}

	invalidTorrents := map[client.TrackerValidity][]string{}
	// A workaround for transmission performance boost. tr can get all infos in batch
	if trClient, ok := clientInstance.(*transmission.Client); ok {
		trClient.Sync(true)
	}
	torrents, err := client.QueryTorrents(clientInstance, category, tag, filter, infoHashes...)
	if err != nil {
		return fmt.Errorf("failed to query client torrents: %w", err)
	}
	errorCnt := int64(0)
	infoHashes = nil
	for _, torrent := range torrents {
		log.Debugf("Check %s (%s) trackers status...", torrent.InfoHash, torrent.Name)
		trackers, err := clientInstance.GetTorrentTrackers(torrent.InfoHash)
		if err != nil {
			log.Errorf("Failed to get torrent %s trackers: %v", torrent.InfoHash, err)
			errorCnt++
			continue
		}
		validity := trackers.SpeculateTrackerValidity()
		if validity == 0 {
			continue
		}
		log.Warnf("torrent %s (%s)'s trackers seems invalid (%s): %v\n",
			torrent.InfoHash, torrent.Name, client.TrackerValidityInfos[validity].Name, trackers)
		invalidTorrents[validity] = append(invalidTorrents[validity], torrent.InfoHash)
	}

	tags := []string{}
	for _, info := range client.TrackerValidityInfos[1:] {
		tags = append(tags, tagPrefix+info.Name)
	}
	if !noClean {
		if err = clientInstance.DeleteTags(tags...); err != nil {
			return fmt.Errorf("failed to clean mark tags: %w", err)
		}
	}

	for _, validityInfo := range client.TrackerValidityInfos[1:] {
		infoHashes := invalidTorrents[validityInfo.Value]
		if len(infoHashes) == 0 {
			continue
		}
		log.Warnf("Marking %d torrents as invalid tracker - %s", len(infoHashes), validityInfo.Name)
		tag := tagPrefix + validityInfo.Name
		if err = clientInstance.AddTagsToTorrents(infoHashes, []string{tag}); err != nil {
			return fmt.Errorf("failed to mark invalid tracker torrents: %w", err)
		}
		fmt.Printf("Found %d torrents with invalid tracker, marked them with %q tag\n", len(infoHashes), tag)
	}

	if errorCnt > 0 {
		return fmt.Errorf("%d errors", errorCnt)
	}
	return nil
}
