package modifytorrent

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/sagan/ptool/client"
	"github.com/sagan/ptool/cmd"
	"github.com/sagan/ptool/constants"
	"github.com/sagan/ptool/util"
	"github.com/sagan/ptool/util/helper"
)

var command = &cobra.Command{
	Use: `modifytorrent {client} [...modifying_flags] ` +
		`[--category category] [--tag tag] [--filter filter] [infoHash]...`,
	Aliases:     []string{"modify", "modifytorrents", "setsharelimits"}, // integrate former "setsharelimits" cmd
	Annotations: map[string]string{"cobra-prompt-dynamic-suggestions": "modifytorrent"},
	Short:       "Modify torrents in client.",
	Long: fmt.Sprintf(`Modify torrents in client.
Change their category / save-path / tags, etc...

Note: this command is NOT about editing ".torrent" (metainfo) files in disk.
To do that, use "edittorrent" command instead.

%s.

Available "modifying" flags (at least one of them must be set):
* --set-category
* --set-save-path
* --add-tags
* --remove-tags.
* --ratio-limit : Set torrent ratio share limit. qb ratioLimit.
  For now, -2 means the global limit should be used, -1 means no limit.
* --seeding-time-limit : Set torrent seeding time share limit. qb seedingTimeLimit (but in seconds instead of minutes).
  For now, -2 means the global limit should be used, -1 means no limit.`, constants.HELP_INFOHASH_ARGS),
	Args: cobra.MatchAll(cobra.MinimumNArgs(1), cobra.OnlyValidArgs),
	RunE: modifytorrent,
}

var (
	seedingTimeLimit = int64(0)
	ratioLimit       = float64(0)
	category         = ""
	tag              = ""
	filter           = ""
	setCategory      = ""
	setSavePath      = ""
	addTags          = ""
	removeTags       = ""
)

func init() {
	command.Flags().Int64VarP(&seedingTimeLimit, "seeding-time-limit", "", 0,
		`(qBittorrent only) If != 0, set seeding time share limit of torrents. `+
			`Positive value: the max amount of time (seconds) the torrent should be seeded. `+
			`Negative value has special meaning`)
	command.Flags().Float64VarP(&ratioLimit, "ratio-limit", "", 0,
		`(qBittorrent only) If != 0, set ratio share limit of torrents. `+
			`Positive value: the max ratio (Up/Dl) the torrent should be seeded until. Negative value has special meaning`)
	command.Flags().StringVarP(&filter, "filter", "", "", constants.HELP_ARG_FILTER_TORRENT)
	command.Flags().StringVarP(&category, "category", "", "", constants.HELP_ARG_CATEGORY)
	command.Flags().StringVarP(&tag, "tag", "", "", constants.HELP_ARG_TAG)
	command.Flags().StringVarP(&setCategory, "set-category", "", "", `Modify category of torrents. `+
		`To make torrents become uncategoried, set it to "`+constants.NONE+`"`)
	command.Flags().StringVarP(&setSavePath, "set-save-path", "", "", "Modify save path of torrents")
	command.Flags().StringVarP(&addTags, "add-tags", "", "", "Add tags to torrent (comma-separated)")
	command.Flags().StringVarP(&removeTags, "remove-tags", "", "", "Remove tags from torrent (comma-separated)")
	cmd.RootCmd.AddCommand(command)
}

func modifytorrent(cmd *cobra.Command, args []string) error {
	if util.CountNonZeroVariables(setCategory, setSavePath, addTags, removeTags, seedingTimeLimit, ratioLimit) == 0 {
		return fmt.Errorf(`at least one modifying flag must be provided`)
	}
	clientName := args[0]
	infoHashes := args[1:]
	if category == "" && tag == "" && filter == "" {
		if _infoHashes, err := helper.ParseInfoHashesFromArgs(infoHashes); err != nil {
			return err
		} else {
			infoHashes = _infoHashes
		}
	}
	addTagsList := util.SplitCsv(addTags)
	removeTagsList := util.SplitCsv(removeTags)
	clientInstance, err := client.CreateClient(clientName)
	if err != nil {
		return fmt.Errorf("failed to create client: %w", err)
	}

	infoHashes, err = client.SelectTorrents(clientInstance, category, tag, filter, infoHashes...)
	if err != nil {
		return err
	}

	if setCategory != "" {
		if infoHashes == nil {
			err = clientInstance.SetAllTorrentsCatetory(setCategory)
			if err != nil {
				return err
			}
		} else if len(infoHashes) > 0 {
			err = clientInstance.SetTorrentsCatetory(infoHashes, setCategory)
			if err != nil {
				return err
			}
		}
	}

	if setSavePath != "" {
		if infoHashes == nil {
			err = clientInstance.SetAllTorrentsSavePath(setSavePath)
			if err != nil {
				return err
			}
		} else if len(infoHashes) > 0 {
			err = clientInstance.SetTorrentsSavePath(infoHashes, setSavePath)
			if err != nil {
				return err
			}
		}
	}

	if len(addTagsList) > 0 {
		if infoHashes == nil {
			err = clientInstance.AddTagsToAllTorrents(addTagsList)
			if err != nil {
				return err
			}
		} else if len(infoHashes) > 0 {
			err = clientInstance.AddTagsToTorrents(infoHashes, addTagsList)
			if err != nil {
				return err
			}
		}
	}

	if len(removeTagsList) > 0 {
		if infoHashes == nil {
			err = clientInstance.RemoveTagsFromAllTorrents(removeTagsList)
			if err != nil {
				return err
			}
		} else if len(infoHashes) > 0 {
			err = clientInstance.RemoveTagsFromTorrents(infoHashes, removeTagsList)
			if err != nil {
				return err
			}
		}
	}

	if seedingTimeLimit != 0 || ratioLimit != 0 {
		if infoHashes == nil {
			err = clientInstance.SetAllTorrentsShareLimits(ratioLimit, seedingTimeLimit)
			if err != nil {
				return err
			}
		} else if len(infoHashes) > 0 {
			err = clientInstance.SetTorrentsShareLimits(infoHashes, ratioLimit, seedingTimeLimit)
			if err != nil {
				return err
			}
		}
	}

	return nil
}
