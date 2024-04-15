package add

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/google/shlex"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/sagan/ptool/client"
	"github.com/sagan/ptool/cmd"
	"github.com/sagan/ptool/config"
	"github.com/sagan/ptool/constants"
	"github.com/sagan/ptool/util"
	"github.com/sagan/ptool/util/helper"
	"github.com/sagan/ptool/util/torrentutil"
)

var command = &cobra.Command{
	Use:         "add {client} {torrentFilename | torrentId | torrentUrl}...",
	Aliases:     []string{"addlocal"},
	Annotations: map[string]string{"cobra-prompt-dynamic-suggestions": "add"},
	Short:       "Add torrents to client.",
	Long: `Add torrents to client.
Args is torrent list that each one could be a local filename (e.g. "*.torrent" or "[M-TEAM]CLANNAD.torrent"),
site torrent id (e.g.: "mteam.488424") or url (e.g.: "https://kp.m-team.cc/details.php?id=488424").
Torrent url that does NOT belong to any site (e.g.: a public site url), as well as "magnet:" link, is also supported.
Use a single "-" as args to read torrent list from stdin, delimited by blanks,
as a special case, it also supports directly reading .torrent file contents from stdin.

To set the name of added torrent in client, use --rename <name> flag,
which supports the following variable placeholders:
* [size] : Torrent size
* [id] :  Torrent id in site
* [site] : Torrent site
* [filename] : Original torrent filename without ".torrent" extension
* [filename128] : The prefix of [filename] which is at max 128 bytes
* [name] : Torrent name
* [name128] : The prefix of torrent name which is at max 128 bytes

Flags:
* --ratio-limit & --seeding-time-limit : See help of "ptool setsharelimits" cmd for more info

If --use-comment-meta flag is set, ptool will extract torrent's category & tags & savePath meta info
from the 'comment' field of .torrent file (parsed in json format: '{tags, category, save_path}').
The "ptool export" command has the same flag that saves meta info to 'comment' field when exporting torrents.`,
	Args: cobra.MatchAll(cobra.MinimumNArgs(2), cobra.OnlyValidArgs),
	RunE: add,
}

var (
	slowMode           = false
	useComment         = false
	addCategoryAuto    = false
	addPaused          = false
	skipCheck          = false
	sequentialDownload = false
	renameAdded        = false
	deleteAdded        = false
	forceLocal         = false
	ratioLimit         = float64(0)
	seedingTimeLimit   = int64(0)
	rename             = ""
	addCategory        = ""
	defaultSite        = ""
	addTags            = ""
	savePath           = ""
)

func init() {
	command.Flags().BoolVarP(&slowMode, "slow", "", false, "Slow mode. wait after adding each torrent")
	command.Flags().BoolVarP(&useComment, "use-comment-meta", "", false,
		"Use 'comment' field of .torrent file to extract category, tags, savePath and other meta info and apply them")
	command.Flags().BoolVarP(&skipCheck, "skip-check", "", false, "Skip hash checking when adding torrents")
	command.Flags().BoolVarP(&addPaused, "add-paused", "", false, "Add torrents to client in paused state")
	command.Flags().BoolVarP(&addCategoryAuto, "add-category-auto", "", false,
		"Automatically set category of added torrent to corresponding sitename")
	command.Flags().BoolVarP(&sequentialDownload, "sequential-download", "", false,
		"(qbittorrent only) Enable sequential download")
	command.Flags().BoolVarP(&renameAdded, "rename-added", "", false,
		"Rename successfully added torrent file to *"+constants.FILENAME_SUFFIX_ADDED)
	command.Flags().BoolVarP(&deleteAdded, "delete-added", "", false, "Delete successfully added *.torrent file")
	command.Flags().BoolVarP(&forceLocal, "force-local", "", false, "Force treat all arg as local torrent filename")
	command.Flags().Int64VarP(&seedingTimeLimit, "seeding-time-limit", "", 0,
		"If != 0, the max amount of time (seconds) the torrent should be seeded. Negative value has special meaning")
	command.Flags().Float64VarP(&ratioLimit, "ratio-limit", "", 0,
		"If != 0, the max ratio (Up/Dl) the torrent should be seeded until. Negative value has special meaning")
	command.Flags().StringVarP(&rename, "rename", "", "", "Rename added torrents (supports variables)")
	command.Flags().StringVarP(&addCategory, "add-category", "", "", "Set category of added torrents")
	command.Flags().StringVarP(&savePath, "add-save-path", "", "", "Set save path of added torrents")
	command.Flags().StringVarP(&defaultSite, "site", "", "", "Set default site of added torrents")
	command.Flags().StringVarP(&addTags, "add-tags", "", "", "Add tags to added torrent (comma-separated)")
	cmd.RootCmd.AddCommand(command)
}

func add(cmd *cobra.Command, args []string) error {
	clientName := args[0]
	if renameAdded && deleteAdded {
		return fmt.Errorf("--rename-added and --delete-added flags are NOT compatible")
	}
	// directly read a torrent content from stdin.
	stdinTorrentContents := []byte{}
	torrents := util.ParseFilenameArgs(args[1:]...)
	if len(torrents) == 1 && torrents[0] == "-" {
		if config.InShell {
			return fmt.Errorf(`"-" arg can not be used in shell`)
		}
		if stdin, err := io.ReadAll(os.Stdin); err != nil {
			return fmt.Errorf("failed to read stdin: %v", err)
		} else if bytes.HasPrefix(stdin, []byte(constants.TORRENT_FILE_MAGIC_NUMBER)) ||
			bytes.HasPrefix(stdin, []byte(constants.TORRENT_FILE_MAGIC_NUMBER2)) {
			stdinTorrentContents = stdin
		} else if data, err := shlex.Split(string(stdin)); err != nil {
			return fmt.Errorf("failed to parse stdin to tokens: %v", err)
		} else {
			torrents = data
		}
	}
	clientInstance, err := client.CreateClient(clientName)
	if err != nil {
		return fmt.Errorf("failed to create client: %v", err)
	}
	option := &client.TorrentOption{
		Pause:              addPaused,
		SkipChecking:       skipCheck,
		SequentialDownload: sequentialDownload,
		RatioLimit:         ratioLimit,
		SeedingTimeLimit:   seedingTimeLimit,
	}
	var fixedTags []string
	if addTags != "" {
		fixedTags = util.SplitCsv(addTags)
	}
	errorCnt := int64(0)
	cntAdded := int64(0)
	sizeAdded := int64(0)
	cntAll := len(torrents)

	for i, torrent := range torrents {
		option.Category = ""
		option.Tags = nil
		option.SavePath = ""
		// handle as a special case
		if util.IsPureTorrentUrl(torrent) {
			option.Category = addCategory
			option.Tags = fixedTags
			option.SavePath = savePath
			if err = clientInstance.AddTorrent([]byte(torrent), option, nil); err != nil {
				fmt.Printf("✕ %s (%d/%d): failed to add to client: %v\n", torrent, i+1, cntAll, err)
				errorCnt++
			} else {
				fmt.Printf("✓ %s (%d/%d)\n", torrent, i+1, cntAll)
			}
			continue
		}
		if i > 0 && slowMode {
			util.Sleep(3)
		}
		content, tinfo, siteInstance, sitename, filename, id, isLocal, err :=
			helper.GetTorrentContent(torrent, defaultSite, forceLocal, false, stdinTorrentContents, true, nil)
		if err != nil {
			fmt.Printf("✕ %s (%d/%d): %v\n", torrent, i+1, cntAll, err)
			errorCnt++
			continue
		}
		size := int64(0)
		infoHash := ""
		contentPath := ""
		if tinfo != nil {
			size = tinfo.Size
			infoHash = tinfo.InfoHash
			contentPath = tinfo.ContentPath
		}
		hr := false
		if siteInstance != nil {
			hr = siteInstance.GetSiteConfig().GlobalHnR
		}
		if useComment {
			if commentMeta := tinfo.DecodeComment(); commentMeta == nil {
				fmt.Printf("✕ %s (%d/%d): failed to parse comment meta\n", torrent, i+1, cntAll)
				errorCnt++
				continue
			} else {
				log.Debugf("Found and use torrent %s comment meta %v", torrent, commentMeta)
				option.Category = commentMeta.Category
				option.Tags = commentMeta.Tags
				option.SavePath = commentMeta.SavePath
				tinfo.MetaInfo.Comment = commentMeta.Comment
				if data, err := tinfo.ToBytes(); err == nil {
					content = data
				}
			}
		} else {
			if addCategoryAuto {
				if sitename != "" {
					option.Category = sitename
				} else if addCategory != "" {
					option.Category = addCategory
				} else {
					option.Category = config.FALLBACK_CAT
				}
			} else {
				option.Category = addCategory
			}
			if tinfo.IsPrivate() {
				option.Tags = append(option.Tags, config.PRIVATE_TAG)
			}
			if sitename != "" {
				option.Tags = append(option.Tags, client.GenerateTorrentTagFromSite(sitename))
			}
			if hr {
				option.Tags = append(option.Tags, config.HR_TAG)
			}
			option.Tags = append(option.Tags, fixedTags...)
			if rename != "" {
				option.Name = torrentutil.RenameTorrent(rename, sitename, id, filename, tinfo)
			}
			option.SavePath = savePath
		}
		err = clientInstance.AddTorrent(content, option, nil)
		if err != nil {
			fmt.Printf("✕ %s (%d/%d) (site=%s): failed to add torrent to client: %v // %s\n",
				torrent, i+1, cntAll, sitename, err, contentPath)
			errorCnt++
			continue
		}
		if isLocal && torrent != "-" {
			if renameAdded && !strings.HasSuffix(torrent, constants.FILENAME_SUFFIX_ADDED) {
				if err := os.Rename(torrent, util.TrimAnySuffix(torrent,
					constants.ProcessedFilenameSuffixes...)+constants.FILENAME_SUFFIX_ADDED); err != nil {
					log.Debugf("Failed to rename %s to *%s: %v", torrent, constants.FILENAME_SUFFIX_ADDED, err)
				}
			} else if deleteAdded {
				if err := os.Remove(torrent); err != nil {
					log.Debugf("Failed to delete %s: %v // %s", torrent, err, contentPath)
				}
			}
		}
		cntAdded++
		sizeAdded += size
		fmt.Printf("✓ %s (%d/%d) (site=%s). infoHash=%s // %s\n", torrent, i+1, cntAll, sitename, infoHash, contentPath)
	}
	fmt.Fprintf(os.Stderr, "\n// Done. Added torrent (Size/Cnt): %s / %d; ErrorCnt: %d\n",
		util.BytesSize(float64(sizeAdded)), cntAdded, errorCnt)
	if errorCnt > 0 {
		return fmt.Errorf("%d errors", errorCnt)
	}
	return nil
}
