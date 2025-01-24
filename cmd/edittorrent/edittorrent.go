package edittorrent

import (
	"bytes"
	"fmt"
	"os"
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
	Use:         "edittorrent {torrentFilename}...",
	Annotations: map[string]string{"cobra-prompt-dynamic-suggestions": "edittorrent"},
	Aliases:     []string{"edit", "edittorrents"},
	Short:       "Edit local .torrent (metainfo) files.",
	Long: `Edit local .torrent (metainfo) files.
Args is the torrent filename list. Use a single "-" as args to read the list from stdin, delimited by blanks.

Note: this command is NOT about modifying torrents in BitTorrent client.
To do that, use "modifytorrent" command instead.

It will update local disk .torrent files in place, unless "--output string" flag is set,
in which case the updated .torrent contents will be output to that file.

It will ask for confirm before updateing torrent files, unless --force flag is set.

Available "editing" flags (at least one of them must be set):
* --remove-tracker
* --add-tracker
* --add-public-trackers
* --update-tracker
* --update-created-by
* --update-creation-date
* --update-info-source
* --update-info-name
* --update-comment
* --set-private
* --set-public
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
	setPrivate                       = false
	setPublic                        = false
	removeTracker                    = ""
	addTracker                       = ""
	updateTracker                    = ""
	updateCreatedBy                  = ""
	updateCreationDate               = ""
	updateInfoSource                 = ""
	updateInfoName                   = ""
	updateComment                    = ""
	replaceCommentMetaSavePathPrefix = ""
	output                           = ""
)

func init() {
	command.Flags().BoolVarP(&force, "force", "", false, "Do update torrent files without confirm")
	command.Flags().BoolVarP(&setPrivate, "set-private", "", false,
		`Set "info.private" field to 1 to mark torrent as private. Warning: info-hash of torrents will change`)
	command.Flags().BoolVarP(&setPublic, "set-public", "", false,
		`Unset "info.private" field to mark torrent as public (non-private). Warning: info-hash of torrents will change`)
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
	command.Flags().StringVarP(&updateCreatedBy, "update-created-by", "", "",
		`Update "created by" field of torrents. To unset this field, set it to "`+constants.NONE+`"`)
	command.Flags().StringVarP(&updateCreationDate, "update-creation-date", "", "",
		`Update "creation date" field of torrents. E.g. "2024-01-20 15:00:00" (local timezone), `+
			`or a unix timestamp integer (seconds). To unset this field, set it to "`+constants.NONE+`"`)
	command.Flags().StringVarP(&updateInfoSource, "update-info-source", "", "",
		`Update "info.source" field of torrents. Warning: info-hash of torrents will change`)
	command.Flags().StringVarP(&updateInfoName, "update-info-name", "", "",
		`Update "info.name" field of torrents. Warning: info-hash of torrents will change`)
	command.Flags().StringVarP(&updateComment, "update-comment", "", "", `Update "comment" field of torrents`)
	command.Flags().StringVarP(&replaceCommentMetaSavePathPrefix, "replace-comment-meta-save-path-prefix", "", "",
		`Used with "--use-comment-meta". Update the prefix of 'save_path' property encoded in "comment" field `+
			`of torrents, replace old prefix with new one. Format: "old_path|new_path". E.g. `+
			`"/root/Downloads:/var/Downloads" will change ""/root/Downloads" or "/root/Downloads/..." save path to `+
			`"/var/Downloads" or "/var/Downloads/..."`)
	command.Flags().StringVarP(&output, "output", "", "", `Save updated .torrent file contents to this file, `+
		`instead of updating the original file in place. Can only be used with 1 (one) torrent arg. `+
		`Set to "-" to output to stdout`)
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
	if util.CountNonZeroVariables(output, doBackup) > 1 {
		return fmt.Errorf("--output and --backup flags are NOT compatible")
	}
	if util.CountNonZeroVariables(setPrivate, setPublic) > 1 {
		return fmt.Errorf("--set-private and --set-public flags are NOT compatible")
	}
	if output != "" {
		if len(torrents) > 1 {
			return fmt.Errorf("--output flag can only be used with 1 (one) torrent arg")
		}
		if output == "-" && term.IsTerminal(int(os.Stdout.Fd())) {
			return fmt.Errorf(constants.HELP_TIP_TTY_BINARY_OUTPUT)
		}
	}
	if util.CountNonZeroVariables(removeTracker, addTracker, addPublicTrackers, updateTracker,
		updateCreatedBy, setPrivate, setPublic, updateCreationDate, updateInfoSource, updateInfoName,
		updateComment, replaceCommentMetaSavePathPrefix) == 0 {
		return fmt.Errorf(`at least one of "--add/remove/update/set/replace-*" flags must be set`)
	}
	if updateTracker != "" && (util.CountNonZeroVariables(removeTracker, addTracker, addPublicTrackers) > 0) {
		return fmt.Errorf(`"--update-tracker" flag is NOT compatible with other tracker editing flags`)
	}
	if !useCommentMeta && (util.CountNonZeroVariables(replaceCommentMetaSavePathPrefix) > 0) {
		return fmt.Errorf(`editing of comment meta fields must be used with "--use-comment-meta" flag`)
	}
	var savePathReplaces []string
	if replaceCommentMetaSavePathPrefix != "" {
		savePathReplaces = strings.Split(replaceCommentMetaSavePathPrefix, ":")
		if len(savePathReplaces) != 2 || savePathReplaces[0] == "" {
			return fmt.Errorf("invalid --replace-comment-meta-save-path-prefix")
		}
	}
	createdBy := updateCreatedBy
	if createdBy == constants.NONE {
		createdBy = ""
	}
	creationDate := int64(0)
	if updateCreationDate != "" {
		if updateCreationDate == constants.NONE {
			creationDate = 0
		} else {
			ts, err := util.ParseTime(updateCreationDate, nil)
			if err != nil {
				return fmt.Errorf("invalid update-creation-date: %w", err)
			}
			creationDate = ts
		}
	}
	errorCnt := int64(0)
	cntTorrents := int64(0)

	if !force {
		fmt.Fprintf(os.Stderr, "Will edit (update) the following .torrent files:")
		for _, torrent := range torrents {
			fmt.Fprintf(os.Stderr, "  %q", torrent)
		}
		fmt.Fprintf(os.Stderr, "\n\nApplying the below modifications:\n-----\n")
		if removeTracker != "" {
			fmt.Fprintf(os.Stderr, "Remove tracker: %q\n", removeTracker)
		}
		if addTracker != "" {
			fmt.Fprintf(os.Stderr, "Add tracker: %q\n", addTracker)
		}
		if addPublicTrackers {
			fmt.Fprintf(os.Stderr, "Add public trackers:\n  %s\n", strings.Join(constants.OpenTrackers, "\n  "))
		}
		if updateTracker != "" {
			fmt.Fprintf(os.Stderr, "Update tracker: %q\n", updateTracker)
		}
		if updateCreatedBy != "" {
			fmt.Fprintf(os.Stderr, `Update "created_by" field: %q`+"\n", updateCreatedBy)
		}
		if updateCreationDate != "" {
			fmt.Fprintf(os.Stderr, `Update "creation_date" field: %q`+"\n", updateCreationDate)
		}
		if updateInfoSource != "" {
			fmt.Fprintf(os.Stderr, `Update "info.source" field: %q`+"\n", updateInfoSource)
		}
		if updateInfoName != "" {
			fmt.Fprintf(os.Stderr, `Update "info.name" field: %q`+"\n", updateInfoName)
		}
		if updateComment != "" {
			fmt.Fprintf(os.Stderr, `Update "comment" field: %q`+"\n", updateComment)
		}
		if setPrivate {
			fmt.Fprintf(os.Stderr, `Set "info.private" field = 1`+"\n")
		} else if setPublic {
			fmt.Fprintf(os.Stderr, `Unset "info.private" field`+"\n")
		}
		if replaceCommentMetaSavePathPrefix != "" {
			fmt.Fprintf(os.Stderr, `Replace prefix of 'save_path' meta in "comment" field: %q => %q`+"\n",
				savePathReplaces[0], savePathReplaces[1])
		}
		fmt.Fprintf(os.Stderr, "-----\n\n")
		if updateInfoSource != "" || updateInfoName != "" || setPrivate || setPublic {
			fmt.Fprintf(os.Stderr, "Warning: the info-hash of torrents will change.\n")
		}
		if output != "" && output != "-" && util.FileExists(output) {
			fmt.Fprintf(os.Stderr, "Warning: output %q already exists and will be overwritten.\n", output)
		}
		if output != "" {
			fmt.Fprintf(os.Stderr, "Updated torrent will be output to: %s\n", output)
		}
		if !helper.AskYesNoConfirm("Will update torrent files") {
			return fmt.Errorf("abort")
		}
	}

	for i, torrent := range torrents {
		fmt.Fprintf(os.Stderr, "(%d/%d) ", i+1, len(torrents))
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
			err = tinfo.UpdateCreatedBy(createdBy)
			switch err {
			case torrentutil.ErrNoChange:
				err = nil
			case nil:
				changed = true
			}
		}
		if err == nil && updateCreationDate != "" {
			err = tinfo.UpdateCreationDate(creationDate)
			switch err {
			case torrentutil.ErrNoChange:
				err = nil
			case nil:
				changed = true
			}
		}
		if err == nil && updateInfoSource != "" {
			err = tinfo.UpdateInfoSource(updateInfoSource)
			switch err {
			case torrentutil.ErrNoChange:
				err = nil
			case nil:
				changed = true
			}
		}
		if err == nil && updateInfoName != "" {
			err = tinfo.UpdateInfoName(updateInfoName)
			switch err {
			case torrentutil.ErrNoChange:
				err = nil
			case nil:
				changed = true
			}
		}
		if err == nil && (setPrivate || setPublic) {
			isPrivate := true
			if setPublic {
				isPrivate = false
			}
			err = tinfo.SetInfoPrivate(isPrivate)
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
			fmt.Fprintf(os.Stderr, "✕ %s : failed to update torrent: %v\n", torrent, err)
			errorCnt++
			continue
		}
		if !changed {
			fmt.Fprintf(os.Stderr, "- %s : no change\n", torrent)
			if output == "" {
				continue
			}
		}
		if commentMeta != nil {
			if err = tinfo.EncodeComment(commentMeta); err != nil {
				fmt.Fprintf(os.Stderr, "✕ %s : failed to encode comment meta: %v\n", torrent, err)
				errorCnt++
				continue
			}
		}
		if doBackup && !strings.HasSuffix(torrent, constants.FILENAME_SUFFIX_BACKUP) {
			if err := util.CopyFile(torrent, util.TrimAnySuffix(torrent,
				constants.ProcessedFilenameSuffixes...)+constants.FILENAME_SUFFIX_BACKUP); err != nil {
				fmt.Fprintf(os.Stderr, "✕ %s : abort updating file due to failed to create backup file: %v\n", torrent, err)
				errorCnt++
				continue
			}
		}
		if data, err := tinfo.ToBytes(); err != nil {
			fmt.Fprintf(os.Stderr, "✕ %s : failed to generate new contents: %v\n", torrent, err)
			errorCnt++
		} else {
			if output != "" {
				if output == "-" {
					_, err = os.Stdout.Write(data)
				} else {
					err = atomic.WriteFile(output, bytes.NewReader(data))
				}
			} else {
				err = atomic.WriteFile(torrent, bytes.NewReader(data))
			}
			if err != nil {
				fmt.Fprintf(os.Stderr, "✕ %s : failed to write new contents: %v\n", torrent, err)
				errorCnt++
			} else {
				if output != "" {
					fmt.Fprintf(os.Stderr, "✓ %s : successfully editted and outputted to %s\n", torrent, output)
				} else {
					fmt.Fprintf(os.Stderr, "✓ %s : successfully updated\n", torrent)
				}
				cntTorrents++
			}
		}
	}
	fmt.Fprintf(os.Stderr, "\n")
	fmt.Fprintf(os.Stderr, "// Updated torrents: %d\n", cntTorrents)
	if errorCnt > 0 {
		return fmt.Errorf("%d errors", errorCnt)
	}
	return nil
}
