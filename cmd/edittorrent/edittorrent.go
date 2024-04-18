package edittorrent

import (
	"fmt"
	"os"
	"strings"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/sagan/ptool/cmd"
	"github.com/sagan/ptool/constants"
	"github.com/sagan/ptool/util"
	"github.com/sagan/ptool/util/helper"
	"github.com/sagan/ptool/util/torrentutil"
)

var command = &cobra.Command{
	Use:         "edittorrent {torrentFilename}...",
	Annotations: map[string]string{"cobra-prompt-dynamic-suggestions": "edittorrent"},
	Aliases:     []string{"edit"},
	Short:       "Edit local .torrent (metainfo) files.",
	Long: `Edit local .torrent (metainfo) files.
It will update local disk .torrent files in place.
It only supports editing / updating of fields that does NOT affect the info-hash of the torrent.
Args is the torrent filename list. Use a single "-" as args to read the list from stdin, delimited by blanks.

It will ask for confirm before updateing torrent files, unless --force flag is set.

Available "editing" flags (at least one of them must be set):
* --remove-tracker
* --add-tracker
* --add-public-trackers
* --update-tracker
* --update-created-by
* --update-creation-date
* --update-comment
* --replace-comment-meta-save-path-prefix (requires "--use-comment-meta")

If --use-comment-meta flag is set, ptool will parse the "comment" field of torrent
as meta info object in json '{tags, category, save_path, comment}' format,
and allow updating of the properties of the json object.
The "ptool export" and some other cmds have the same flag that would generate .torrent files
with meta info encoded in "comment" field in the above way.

The --use-comment-meta flag relative "editing" flags:
* --replace-comment-meta-save-path-prefix : Update "save_path", replace one prefix with another one.

If --backup flag is set, it will create a backup of original torrent file before updating it.`,
	Args: cobra.MatchAll(cobra.MinimumNArgs(1), cobra.OnlyValidArgs),
	RunE: edittorrent,
}

var (
	force                            = false
	doBackup                         = false
	useCommentMeta                   = false
	addPublicTrackers                = false
	removeTracker                    = ""
	addTracker                       = ""
	updateTracker                    = ""
	updateCreatedBy                  = ""
	updateCreationDate               = ""
	updateComment                    = ""
	replaceCommentMetaSavePathPrefix = ""
)

func init() {
	command.Flags().BoolVarP(&force, "force", "", false, "Do update torrent files without confirm")
	command.Flags().BoolVarP(&addPublicTrackers, "add-public-trackers", "", false,
		`Add common pre-defined open trackers to public (non-private) torrents. If a torrent is private, do nothing`)
	command.Flags().BoolVarP(&doBackup, "backup", "", false,
		"Backup original .torrent file to *"+constants.FILENAME_SUFFIX_BACKUP+
			" unless it's name already has that suffix. If the same name backup file already exists, it will be overwrited")
	command.Flags().BoolVarP(&useCommentMeta, "use-comment-meta", "", false,
		`Allow editing torrent save path and other infos that is encoded (as json) in "comment" field of torrents`)
	command.Flags().StringVarP(&removeTracker, "remove-tracker", "", "",
		"Remove tracker from torrents. If the tracker does NOT exists in the torrent, do nothing")
	command.Flags().StringVarP(&addTracker, "add-tracker", "", "",
		"Add new tracker to torrents. If the tracker already exists in the torrent, do nothing")
	command.Flags().StringVarP(&updateTracker, "update-tracker", "", "",
		"Set the tracker of torrents. It will become the sole tracker of torrents, all existing ones will be removed")
	command.Flags().StringVarP(&updateCreatedBy, "update-created-by", "", "", `Update "created by" field of torrents`)
	command.Flags().StringVarP(&updateCreationDate, "update-creation-date", "", "",
		`Update "creation date" field of torrents. E.g.: "2024-01-20 15:00:00" (local timezone), `+
			"or a unix timestamp integer (seconds)")
	command.Flags().StringVarP(&updateComment, "update-comment", "", "", `Update "comment" field of torrents`)
	command.Flags().StringVarP(&replaceCommentMetaSavePathPrefix, "replace-comment-meta-save-path-prefix", "", "",
		`Used with "--use-comment-meta". Update the prefix of 'save_path' property encoded in "comment" field `+
			`of torrents, replace old prefix with new one. Format: "old_path|new_path". E.g.: `+
			`"/root/Downloads|/var/Downloads" will change ""/root/Downloads" or "/root/Downloads/..." save path to `+
			`"/var/Downloads" or "/var/Downloads/..."`)
	cmd.RootCmd.AddCommand(command)
}

func edittorrent(cmd *cobra.Command, args []string) error {
	torrents, _, err := helper.ParseTorrentsFromArgs(args)
	if err != nil {
		return err
	}
	if len(torrents) == 0 {
		log.Infof("No torrents found")
		return nil
	}
	if len(torrents) == 1 && torrents[0] == "-" {
		return fmt.Errorf(`"-" as reading .torrent content from stdin is NOT supported here`)
	}
	if util.CountNonZeroVariables(removeTracker, addTracker, addPublicTrackers, updateTracker,
		updateCreatedBy, updateCreationDate, updateComment, replaceCommentMetaSavePathPrefix) == 0 {
		return fmt.Errorf(`at least one of "--add-*", "--remove-*", "--update-*", or "--replace-*" flags must be set`)
	}
	if updateTracker != "" && (util.CountNonZeroVariables(removeTracker, addTracker, addPublicTrackers) > 0) {
		return fmt.Errorf(`"--update-tracker" flag is NOT compatible with other tracker editing flags`)
	}
	if !useCommentMeta && (util.CountNonZeroVariables(replaceCommentMetaSavePathPrefix) > 0) {
		return fmt.Errorf(`editing of comment meta fields must be used with "--use-comment-meta" flag`)
	}
	var savePathReplaces []string
	if replaceCommentMetaSavePathPrefix != "" {
		savePathReplaces = strings.Split(replaceCommentMetaSavePathPrefix, "|")
		if len(savePathReplaces) != 2 || savePathReplaces[0] == "" {
			return fmt.Errorf("invalid --replace-comment-meta-save-path-prefix")
		}
	}
	errorCnt := int64(0)
	cntTorrents := int64(0)

	if !force {
		fmt.Printf("Will edit (update) the following .torrent files:")
		for _, torrent := range torrents {
			fmt.Printf("  %q", torrent)
		}
		fmt.Printf("\n\nApplying the below modifications:\n-----\n")
		if removeTracker != "" {
			fmt.Printf("Remove tracker: %q\n", removeTracker)
		}
		if addTracker != "" {
			fmt.Printf("Add tracker: %q\n", addTracker)
		}
		if addPublicTrackers {
			fmt.Printf("Add public trackers:\n  %s\n", strings.Join(constants.OpenTrackers, "\n  "))
		}
		if updateTracker != "" {
			fmt.Printf("Update tracker: %q\n", updateTracker)
		}
		if updateCreatedBy != "" {
			fmt.Printf(`Update "created_by" field: %q`+"\n", updateCreatedBy)
		}
		if updateCreationDate != "" {
			fmt.Printf(`Update "creation_date" field: %q`+"\n", updateCreationDate)
		}
		if updateComment != "" {
			fmt.Printf(`Update "comment" field: %q`+"\n", updateComment)
		}
		if replaceCommentMetaSavePathPrefix != "" {
			fmt.Printf(`Replace prefix of 'save_path' meta in "comment" field: %q => %q`+"\n",
				savePathReplaces[0], savePathReplaces[1])
		}
		fmt.Printf("-----\n\n")
		if !helper.AskYesNoConfirm("Will update torrent files") {
			return fmt.Errorf("abort")
		}
	}

	for _, torrent := range torrents {
		_, tinfo, _, _, _, _, _, err := helper.GetTorrentContent(torrent, "", true, false, nil, false, nil)
		if err != nil {
			log.Errorf("Failed to parse %s: %v", torrent, err)
			errorCnt++
			continue
		}
		var commentMeta *torrentutil.TorrentCommentMeta
		if useCommentMeta {
			commentMeta = tinfo.DecodeComment()
		}
		changed := false
		if removeTracker != "" {
			err = tinfo.RemoveTracker(removeTracker)
			switch err {
			case torrentutil.ErrNoChange:
				err = nil
			case nil:
				changed = true
			}
		}
		if err == nil && addTracker != "" {
			err = tinfo.AddTracker(addTracker, -1)
			switch err {
			case torrentutil.ErrNoChange:
				err = nil
			case nil:
				changed = true
			}
		}
		if err == nil && addPublicTrackers && !tinfo.IsPrivate() {
			for _, tracker := range constants.OpenTrackers {
				if tinfo.AddTracker(tracker, -1) == nil {
					changed = true
				}
			}
		}
		if err == nil && updateTracker != "" {
			err = tinfo.UpdateTracker(updateTracker)
			switch err {
			case torrentutil.ErrNoChange:
				err = nil
			case nil:
				changed = true
			}
		}
		if err == nil && updateCreatedBy != "" {
			err = tinfo.UpdateCreatedBy(updateCreatedBy)
			switch err {
			case torrentutil.ErrNoChange:
				err = nil
			case nil:
				changed = true
			}
		}
		if err == nil && updateCreationDate != "" {
			err = tinfo.UpdateCreationDate(updateCreationDate)
			switch err {
			case torrentutil.ErrNoChange:
				err = nil
			case nil:
				changed = true
			}
		}
		if err == nil && updateComment != "" {
			if useCommentMeta {
				if commentMeta == nil {
					commentMeta = &torrentutil.TorrentCommentMeta{}
				}
				if commentMeta.Comment != updateComment {
					commentMeta.Comment = updateComment
					changed = true
				}
			} else {
				err = tinfo.UpdateComment(updateComment)
				switch err {
				case torrentutil.ErrNoChange:
					err = nil
				case nil:
					changed = true
				}
			}
		}
		if err == nil && replaceCommentMetaSavePathPrefix != "" && commentMeta != nil {
			if commentMeta.SavePath == savePathReplaces[0] ||
				strings.HasPrefix(commentMeta.SavePath, savePathReplaces[0]+"/") ||
				strings.HasPrefix(commentMeta.SavePath, savePathReplaces[0]+`\`) {
				commentMeta.SavePath = savePathReplaces[1] + commentMeta.SavePath[len(savePathReplaces[0]):]
				changed = true
			}
		}
		if err != nil {
			fmt.Printf("✕ %s : failed to update torrent: %v\n", torrent, err)
			errorCnt++
			continue
		}
		if !changed {
			fmt.Printf("- %s : no change\n", torrent)
			continue
		}
		if commentMeta != nil {
			if err = tinfo.EncodeComment(commentMeta); err != nil {
				fmt.Printf("✕ %s : failed to encode comment meta: %v\n", torrent, err)
				errorCnt++
				continue
			}
		}
		if doBackup && !strings.HasSuffix(torrent, constants.FILENAME_SUFFIX_BACKUP) {
			if err := util.CopyFile(torrent, util.TrimAnySuffix(torrent,
				constants.ProcessedFilenameSuffixes...)+constants.FILENAME_SUFFIX_BACKUP); err != nil {
				fmt.Printf("✕ %s : abort updating file due to failed to create backup file: %v\n", torrent, err)
				errorCnt++
				continue
			}
		}
		if data, err := tinfo.ToBytes(); err != nil {
			fmt.Printf("✕ %s : failed to generate new contents: %v\n", torrent, err)
			errorCnt++
		} else if err := os.WriteFile(torrent, data, constants.PERM); err != nil {
			fmt.Printf("✕ %s : failed to write new contents: %v\n", torrent, err)
			errorCnt++
		} else {
			fmt.Printf("✓ %s : successfully updated\n", torrent)
			cntTorrents++
		}
	}
	fmt.Fprintf(os.Stderr, "\n")
	fmt.Fprintf(os.Stderr, "// Updated torrents: %d\n", cntTorrents)
	if errorCnt > 0 {
		return fmt.Errorf("%d errors", errorCnt)
	}
	return nil
}
