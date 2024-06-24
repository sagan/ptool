package movesavepath

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/natefinch/atomic"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/sagan/ptool/client"
	"github.com/sagan/ptool/cmd"
	"github.com/sagan/ptool/cmd/common"
	"github.com/sagan/ptool/config"
	"github.com/sagan/ptool/constants"
	"github.com/sagan/ptool/util"
	"github.com/sagan/ptool/util/helper"
	"github.com/sagan/ptool/util/torrentutil"
)

var command = &cobra.Command{
	Use:         "movesavepath --client {client} {old-save-path} {new-save-path}",
	Aliases:     []string{"movesavepath"},
	Annotations: map[string]string{"cobra-prompt-dynamic-suggestions": "movesavepath"},
	Short:       "Move save path of client.",
	Long: `Move save path of client.
It moves contents of old-save-path folder to new-save-path folder. For all torrents in client with a save-path
inside old-save-path, it change the save path of torrent in client to corresponding new path inside new-save-path.
The {old-save-path} and {new-save-path} must be pathes of local file system of the the same disk and / or partition.
The {client} must be a client in local, or you will have to set the "--map-save-path local_path:client_path" flags.

If "--all" flag is not set, it will only move files that have any corresponding torrent in client
from old-save-path to new-save-path.
If "--all" flag is set, all contents of old-save-path will be moved to new-save-path,
after the action the old-save-path will become an empty dir.

It work in the following procedure:

1. Export all torrents in client which has a save-path inside old-save-path to "<config-dir>/msp-<client>/*.torrent",
with the old-save-path info encoded into the "comment" field of exported metainfo file.
All matched torrents in client must be downloaded completed.
2. Delete exported torrents from client (torrent content files in disk are NOT deleted).
3. Add back exported torrents to client, set their save-path to new locations inside new-save-path, and skip checking.
The exported torrent metainfo file that has successfully added back to client will be deleted.

If any error happens, you can safely re-run the same command to re-try and resume from last time.`,
	Args: cobra.MatchAll(cobra.ExactArgs(2), cobra.OnlyValidArgs),
	RunE: movesavepath,
}

var (
	all          = false
	force        = false
	clientname   = ""
	mapSavePaths []string
)

func init() {
	command.Flags().BoolVarP(&all, "all", "a", false, "Move all files of old-save-path to new-save-path. "+
		"If this flag is not set, only files that have corresponding torrents in client will be moved")
	command.Flags().BoolVarP(&force, "force", "", false, "Force action. Do NOT prompt for confirm")
	command.Flags().StringVarP(&clientname, "client", "", "", `Client name`)
	command.Flags().StringArrayVarP(&mapSavePaths, "map-save-path", "", nil,
		`Used with "--use-comment-meta". Map save path from local file file system to BitTorrent client. `+
			`Format: "local_save_path|client_save_path". `+constants.HELP_ARG_PATH_MAPPERS)
	command.MarkFlagRequired("client")
	cmd.RootCmd.AddCommand(command)
}

func movesavepath(cmd *cobra.Command, args []string) (err error) {
	oldSavePath := args[0]
	newSavePath := args[1]
	if absOldSavePath, err := filepath.Abs(oldSavePath); err != nil {
		return fmt.Errorf("failed to get abs path of old-save-path")
	} else {
		oldSavePath = absOldSavePath
	}
	if absNewSavePath, err := filepath.Abs(newSavePath); err != nil {
		return fmt.Errorf("failed to get abs path of new-save-path")
	} else {
		newSavePath = absNewSavePath
	}
	if oldSavePath == newSavePath ||
		strings.HasPrefix(oldSavePath, newSavePath+string(filepath.Separator)) ||
		strings.HasPrefix(newSavePath, oldSavePath+string(filepath.Separator)) {
		return fmt.Errorf("old-save-path and new-save-path must not overlap")
	}
	if oldSavePathStat, err := os.Stat(oldSavePath); err != nil || !oldSavePathStat.IsDir() {
		return fmt.Errorf("old-save-path does not exists or is not dir (err: %w)", err)
	}
	if newSavePathStat, err := os.Stat(newSavePath); err != nil || !newSavePathStat.IsDir() {
		return fmt.Errorf("new-save-path does not exists or is not dir (err: %w)", err)
	}
	var savePathMapper *common.PathMapper
	if len(mapSavePaths) > 0 {
		savePathMapper, err = common.NewPathMapper(mapSavePaths)
		if err != nil {
			return fmt.Errorf("invalid map-save-path(s): %w", err)
		}
		_, match := savePathMapper.Before2After(oldSavePath)
		if !match {
			return fmt.Errorf("invalid map-save-path(s): old-save-path can NOT be mapped to client path")
		}
		_, match = savePathMapper.Before2After(newSavePath)
		if !match {
			return fmt.Errorf("invalid map-save-path(s): new-save-path can NOT be mapped to client path")
		}
	}

	tmppath := filepath.Join(config.ConfigDir, "msp-"+clientname)
	if err = os.MkdirAll(tmppath, constants.PERM_DIR); err != nil {
		return fmt.Errorf("failed to create tmp dir %q: %w", tmppath, err)
	}
	log.Warnf("Use tmppath: %q", tmppath)
	var oldSavePathEntries []fs.DirEntry
	if oldSavePathEntries, err = os.ReadDir(oldSavePath); err != nil {
		return fmt.Errorf("failed to read oldpath: %w", err)
	} else {
		testfile := filepath.Join(oldSavePath, constants.TEST_FILE)
		testfiledst := filepath.Join(newSavePath, constants.TEST_FILE)
		if err = util.TouchFile(testfile); err != nil {
			return fmt.Errorf("failed to access oldpath: %w", err)
		}
		if err = atomic.ReplaceFile(testfile, testfiledst); err != nil {
			return fmt.Errorf("failed to move file from oldpath to newpath: %w", err)
		}
		os.Remove(testfiledst)
	}

	clientInstance, err := client.CreateClient(clientname)
	if err != nil {
		return fmt.Errorf("failed to create client: %w", err)
	}
	clientAllTorrents, err := clientInstance.GetTorrents("", "", true)
	if err != nil {
		return fmt.Errorf("failed to get client torrents: %w", err)
	}
	var clientTorrents []*client.Torrent
	for _, torrent := range clientAllTorrents {
		torrentSavePath := torrent.SavePath
		if savePathMapper != nil {
			localSavePath, match := savePathMapper.After2Before(torrent.SavePath)
			if !match {
				continue
			}
			torrentSavePath = filepath.Clean(localSavePath) // to local sep
		}
		if oldSavePath != torrentSavePath && !strings.HasPrefix(torrentSavePath, oldSavePath+string(filepath.Separator)) {
			continue
		}
		if torrent.State != "completed" && torrent.State != "seeding" {
			return fmt.Errorf("client torrent %s (%s) which is inside old-save-path is not completed",
				torrent.Name, torrent.ContentPath)
		}
		clientTorrents = append(clientTorrents, torrent)
	}

	if !force {
		client.PrintTorrents(os.Stderr, clientTorrents, "", 1, false)
		fmt.Fprintf(os.Stderr, "\n")
		oldPathStr := oldSavePath
		newPathStr := newSavePath
		if savePathMapper != nil {
			oldPathStr = fmt.Sprintf("%s (client: %s)", oldSavePath, util.First(savePathMapper.Before2After(oldSavePath)))
			newPathStr = fmt.Sprintf("%s (client: %s)", newSavePath, util.First(savePathMapper.Before2After(newSavePath)))
		}
		var tip string
		if all {
			tip = "All contents of the old-save-path will be moved."
		} else {
			tip = "Only contents of the old-save-path that have corresponding torrents in client will be moved."
		}
		fmt.Fprintf(os.Stderr, `Move contents of the old-save-path to new-save-path:
---
old-save-path: %s
new-save-path: %s
tmppath: %s
---

Above %d torrents in client have save path inside old-save-path now, they will be exported to tmppath/*.torrent,
temporarily deleted from client, and then added back later with save path inside new-save-path.

%s

If you have previously runned this command and failed in the middle,
the previous exported torrents in tmppath will also be added back to client.
If any error happens, you can safely re-run the same command to re-try and resume from last time.
`, oldPathStr, newPathStr, tmppath, len(clientTorrents), tip)
		if !helper.AskYesNoConfirm("") {
			return fmt.Errorf("abort")
		}
	}

	var clientTorrentInfoHashes []string
	for _, torrent := range clientTorrents {
		exportFilename := filepath.Join(tmppath, torrent.InfoHash+".torrent")
		if err = common.ExportClientTorrent(clientInstance, torrent, exportFilename, true); err != nil {
			return fmt.Errorf("failed to export client torrent %s: %w", torrent.Name, err)
		}
		clientTorrentInfoHashes = append(clientTorrentInfoHashes, torrent.InfoHash)
	}
	log.Warnf("Exported %d torrents from client to %q", len(clientTorrentInfoHashes), tmppath)

	tmppathEntries, err := os.ReadDir(tmppath)
	if err != nil {
		return fmt.Errorf("failed to read tmppath: %w", err)
	}
	var existFlags map[string]struct{}

	if !all {
		// In default mode, only move torrent files. So we must check once prior moving.
		existFlags = map[string]struct{}{}
		for _, entry := range tmppathEntries {
			if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".torrent") || strings.HasPrefix(entry.Name(), ".") {
				continue
			}
			entrypath := filepath.Join(tmppath, entry.Name())
			contents, err := os.ReadFile(entrypath)
			if err != nil {
				return fmt.Errorf("failed to read %s in tmppath: %w", entry.Name(), err)
			}
			tinfo, err := torrentutil.ParseTorrent(contents)
			if err != nil {
				return fmt.Errorf("failed to parse %s in tmppath: %w", entry.Name(), err)
			}
			commentMeta := tinfo.DecodeComment()
			if commentMeta == nil {
				return fmt.Errorf("failed to parse %s in tmppath: invalid comment-meta", entry.Name())
			}
			torrentSavePath := commentMeta.SavePath
			if savePathMapper != nil {
				clientSavePath, match := savePathMapper.After2Before(commentMeta.SavePath)
				if !match {
					return fmt.Errorf("tmppath/%s comment-meta save path %q can not be mapped to local path",
						entry.Name(), commentMeta.SavePath)
				}
				torrentSavePath = filepath.Clean(clientSavePath)
			}
			if torrentSavePath != oldSavePath && !strings.HasPrefix(torrentSavePath, oldSavePath+string(filepath.Separator)) {
				return fmt.Errorf("tmppath/%s comment meta local save path %q is not inside old-save-path",
					entry.Name(), torrentSavePath)
			}
			newTorrentSavePath := newSavePath + torrentSavePath[len(oldSavePath):]
			if savePathMapper != nil {
				_, match := savePathMapper.Before2After(newTorrentSavePath)
				if !match {
					return fmt.Errorf("tmppath/%s new save path %q can not be mapped to client path",
						entry.Name(), newTorrentSavePath)
				}
			}
			// E.g.: oldSavePath: /root/Downloads; newSavePath: /var/Downloads; torrentSavePath: /root/Downloads/Others .
			// In such case, the top folder "Others" should be moved.
			if strings.HasPrefix(torrentSavePath, oldSavePath+string(filepath.Separator)) {
				topname, _, _ := strings.Cut(torrentSavePath[len(oldSavePath)+1:], string(filepath.Separator))
				existFlags[topname] = struct{}{}
			} else {
				for _, rootFile := range tinfo.RootFiles() {
					existFlags[rootFile] = struct{}{}
				}
			}
		}
	}

	if len(clientTorrentInfoHashes) > 0 {
		if err = clientInstance.DeleteTorrents(clientTorrentInfoHashes, false); err != nil {
			return fmt.Errorf("failed to delete client torrents: %w", err)
		}
		log.Warnf("Temporarily deleted %d torrents from client", len(clientTorrentInfoHashes))
	}

	log.Warnf("Beginning moving files from old-save-path to new-save-path")
	movedEntriesCnt := int64(0)
	for _, entry := range oldSavePathEntries {
		if !all {
			if _, ok := existFlags[entry.Name()]; !ok {
				continue
			}
		}
		frompath := filepath.Join(oldSavePath, entry.Name())
		topath := filepath.Join(newSavePath, entry.Name())
		if util.FileExists(topath) {
			return fmt.Errorf("failed to move file %q from oldpath to newpath: target already exists", entry.Name())
		}
		if err = atomic.ReplaceFile(frompath, topath); err != nil {
			return fmt.Errorf("failed to move file %q fom oldpath to newpath: %v", entry.Name(), err)
		}
		movedEntriesCnt++
	}
	log.Warnf("Moved %d entries from old-save-path to new-save-path", movedEntriesCnt)

	for _, entry := range tmppathEntries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".torrent") || strings.HasPrefix(entry.Name(), ".") {
			continue
		}
		entrypath := filepath.Join(tmppath, entry.Name())
		contents, err := os.ReadFile(entrypath)
		if err != nil {
			return fmt.Errorf("failed to read %s in tmppath: %w", entry.Name(), err)
		}
		tinfo, err := torrentutil.ParseTorrent(contents)
		if err != nil {
			return fmt.Errorf("failed to parse %s in tmppath: %w", entry.Name(), err)
		}
		commentMeta := tinfo.DecodeComment()
		if commentMeta == nil {
			return fmt.Errorf("failed to parse %s in tmppath: invalid comment-meta", entry.Name())
		}
		torrentSavePath := commentMeta.SavePath
		if savePathMapper != nil {
			clientSavePath, match := savePathMapper.After2Before(commentMeta.SavePath)
			if !match {
				return fmt.Errorf("tmppath/%s comment-meta save path %q can not be mapped to local path",
					entry.Name(), commentMeta.SavePath)
			}
			torrentSavePath = filepath.Clean(clientSavePath)
		}
		if torrentSavePath != oldSavePath && !strings.HasPrefix(torrentSavePath, oldSavePath+string(filepath.Separator)) {
			return fmt.Errorf("tmppath/%s comment meta local save path %q is not inside old-save-path",
				entry.Name(), torrentSavePath)
		}

		// new save path
		newTorrentSavePath := newSavePath + torrentSavePath[len(oldSavePath):]
		clientTorrentSavePath := newTorrentSavePath
		if savePathMapper != nil {
			clientSavePath, match := savePathMapper.Before2After(newTorrentSavePath)
			if !match {
				return fmt.Errorf("tmppath/%s new save path %q can not be mapped to client path",
					entry.Name(), newTorrentSavePath)
			}
			clientTorrentSavePath = clientSavePath
		}
		tinfo.MetaInfo.Comment = commentMeta.Comment
		if data, err := tinfo.ToBytes(); err == nil {
			contents = data
		}
		err = clientInstance.AddTorrent(contents, &client.TorrentOption{
			SavePath:     clientTorrentSavePath,
			Tags:         commentMeta.Tags,
			Category:     commentMeta.Category,
			SkipChecking: true,
		}, nil)
		if err != nil {
			return fmt.Errorf("failed to add back torrent %s to client: %w", entry.Name(), err)
		}
		log.Warnf("Added back %s to client (client-save-path: %s)", entry.Name(), clientTorrentSavePath)
		os.Remove(entrypath)
	}

	log.Warnf("All done successfully")
	return nil
}
