package add

import (
	"fmt"
	"net/url"
	"os"
	"strings"
	"text/template"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/sagan/ptool/client"
	"github.com/sagan/ptool/cmd"
	"github.com/sagan/ptool/cmd/common"
	"github.com/sagan/ptool/config"
	"github.com/sagan/ptool/constants"
	"github.com/sagan/ptool/site/public"
	"github.com/sagan/ptool/site/tpl"
	"github.com/sagan/ptool/util"
	"github.com/sagan/ptool/util/helper"
	"github.com/sagan/ptool/util/torrentutil"
)

var command = &cobra.Command{
	Use:         "add {client} {torrentFilename | torrentId | torrentUrl}...",
	Aliases:     []string{"addlocal"},
	Annotations: map[string]string{"cobra-prompt-dynamic-suggestions": "add"},
	Short:       "Add torrents to client.",
	Long: fmt.Sprintf(`Add torrents to client.
First arg is client. The following args is the args list.
%s.
It also supports "magnet:" urls.

By default, if the arg is a http(s) url, it will try to parse it as a site torrent url,
download the .torrent file to local and verify it's a valid metainfo file,
then adding the downloaded .torrent file contents to the BitTorrent client.
If "--raw" flag is set, it skips above procedures and directly submits the url to the BitTorrent client.

To set the name of added torrent in client, use "--rename string" flag,
The template supports the following variables:
* size : Torrent contents size string (e.g. "42GiB")
* id :  Torrent id in site
* site : Torrent site name
* filename : Original torrent filename without ".torrent" extension
* filename128 : The prefix of filename which is at max 128 bytes
* name : Torrent name
* name128 : The prefix of torrent name which is at max 128 bytes
* torrentInfo : The parsed "TorrentMeta" struct of torrent. See help of "parsetorrent" cmd
E.g. '--rename "{{.site}}.{{.id}} {{.name}}"'

Flags:
* --ratio-limit & --seeding-time-limit : See help of "modifytorrent" cmd for more info

If --use-comment-meta flag is set, ptool will extract torrent's category & tags & savePath meta info
from the 'comment' field of .torrent file (parsed in json '{tags, category, save_path, comment}' format).
The "ptool export" command has the same flag that saves meta info to 'comment' field when exporting torrents.`,
		constants.HELP_TORRENT_ARGS),
	Args: cobra.MatchAll(cobra.MinimumNArgs(2), cobra.OnlyValidArgs),
	RunE: add,
}

var (
	addRawUrl          = false
	slowMode           = false
	useCommentMeta     = false
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
	mapSavePaths       []string
)

func init() {
	command.Flags().BoolVarP(&addRawUrl, "raw", "", false,
		`Directly submit http(s) url arg to BitTorrent client (do not try to parse url and download .torrent file)`)
	command.Flags().BoolVarP(&slowMode, "slow", "", false, "Slow mode. wait after adding each torrent")
	command.Flags().BoolVarP(&useCommentMeta, "use-comment-meta", "", false,
		`Use "comment" field of .torrent file to extract category, tags, savePath and other meta info and apply them`)
	command.Flags().BoolVarP(&skipCheck, "skip-check", "", false, "Skip hash checking when adding torrents")
	command.Flags().BoolVarP(&addPaused, "add-paused", "", false, "Add torrents to client in paused state")
	command.Flags().BoolVarP(&addCategoryAuto, "add-category-auto", "", false,
		"Automatically set category of added torrent to corresponding sitename")
	command.Flags().BoolVarP(&sequentialDownload, "sequential-download", "", false,
		"(qbittorrent only) Enable sequential download")
	command.Flags().BoolVarP(&renameAdded, "rename-added", "", false,
		"Rename successfully added .torrent file to *"+constants.FILENAME_SUFFIX_ADDED+
			" unless it's name already has that suffix")
	command.Flags().BoolVarP(&deleteAdded, "delete-added", "", false, "Delete successfully added *.torrent file")
	command.Flags().BoolVarP(&forceLocal, "force-local", "", false, "Force treat all arg as local torrent filename")
	command.Flags().Int64VarP(&seedingTimeLimit, "seeding-time-limit", "", 0,
		"If != 0, the max amount of time (seconds) the torrent should be seeded. Negative value has special meaning")
	command.Flags().Float64VarP(&ratioLimit, "ratio-limit", "", 0,
		"If != 0, the max ratio (Up/Dl) the torrent should be seeded until. Negative value has special meaning")
	command.Flags().StringVarP(&rename, "rename", "", "", `Rename added torrents. `+
		`Available variable placeholders: {{.site}}, {{.id}} and more. `+constants.HELP_ARG_TEMPLATE)
	command.Flags().StringVarP(&addCategory, "add-category", "", "", "Set category of added torrents")
	command.Flags().StringVarP(&savePath, "add-save-path", "", "", "Set save path of added torrents")
	command.Flags().StringVarP(&defaultSite, "site", "", "", "Set default site of added torrents")
	command.Flags().StringVarP(&addTags, "add-tags", "", "", "Add tags to added torrent (comma-separated)")
	command.Flags().StringArrayVarP(&mapSavePaths, "map-save-path", "", nil,
		`Used with "--use-comment-meta". Map save path from torrent comment to the file system of BitTorrent client. `+
			`Format: "comment_save_path|client_save_path". `+constants.HELP_ARG_PATH_MAPPERS)
	cmd.RootCmd.AddCommand(command)
}

func add(cmd *cobra.Command, args []string) error {
	clientName := args[0]
	if renameAdded && deleteAdded {
		return fmt.Errorf("--rename-added and --delete-added flags are NOT compatible")
	}
	if !useCommentMeta && len(mapSavePaths) > 0 {
		return fmt.Errorf("--map-save-path must be used with --use-comment-meta flag")
	}
	torrents, stdinTorrentContents, err := helper.ParseTorrentsFromArgs(args[1:])
	if err != nil {
		return err
	}
	clientInstance, err := client.CreateClient(clientName)
	if err != nil {
		return fmt.Errorf("failed to create client: %w", err)
	}
	option := &client.TorrentOption{
		Pause:              addPaused,
		SkipChecking:       skipCheck,
		SequentialDownload: sequentialDownload,
		RatioLimit:         ratioLimit,
		SeedingTimeLimit:   seedingTimeLimit,
	}
	fixedTags := util.SplitCsv(addTags)
	var savePathMapper *common.PathMapper
	if len(mapSavePaths) > 0 {
		savePathMapper, err = common.NewPathMapper(mapSavePaths)
		if err != nil {
			return fmt.Errorf("invalid map-save-path(s): %w", err)
		}
	}
	var renameTemplate *template.Template
	if rename != "" {
		if renameTemplate, err = helper.GetTemplate(rename); err != nil {
			return fmt.Errorf("invalid rename template: %v", err)
		}
	}

	errorCnt := int64(0)
	cntAdded := int64(0)
	sizeAdded := int64(0)

	for i, torrent := range torrents {
		if i > 0 && slowMode {
			util.Sleep(3)
		}
		fmt.Printf("(%d/%d) ", i+1, len(torrents))
		option.Category = ""
		option.Tags = nil
		option.SavePath = ""
		// handle as a special case
		if util.IsPureTorrentUrl(torrent) || (addRawUrl && util.IsUrl(torrent)) {
			option.Category = addCategory
			option.Tags = fixedTags
			option.SavePath = savePath
			isPublicTorrentUrl := false
			if util.IsPureTorrentUrl(torrent) {
				// All magnet: link should be public torrent.
				isPublicTorrentUrl = true
			} else if urlObj, err := url.Parse(torrent); err == nil {
				if sitename, _ := tpl.GuessSiteByDomain(urlObj.Host, ""); sitename != "" {
					option.Tags = append(option.Tags, client.GenerateTorrentTagFromSite(sitename))
				} else if publicSite := public.GetSiteByDomain(urlObj.Host); publicSite != nil {
					isPublicTorrentUrl = true
					option.Tags = append(option.Tags, client.GenerateTorrentTagFromSite(publicSite.Name))
				}
			}
			if isPublicTorrentUrl {
				option.Tags = append(option.Tags, config.PUBLIC_TAG)
				if ratioLimit == 0 {
					option.RatioLimit = config.Get().PublicTorrentRatioLimit
				}
			} else {
				option.Tags = append(option.Tags, config.PRIVATE_TAG)
			}
			if err = clientInstance.AddTorrent([]byte(torrent), option, nil); err != nil {
				fmt.Printf("✕ %s: failed to add to client: %v\n", torrent, err)
				errorCnt++
			} else {
				fmt.Printf("✓ %s\n", torrent)
			}
			continue
		}
		// Note: tinfo coule be nil here. It's a workaround as anacrolix/torrent failed to parse some torrents.
		content, tinfo, siteInstance, sitename, filename, id, isLocal, err :=
			helper.GetTorrentContent(torrent, defaultSite, forceLocal, false, stdinTorrentContents, true, nil)
		if err != nil {
			fmt.Printf("✕ %s: %v\n", torrent, err)
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
		if useCommentMeta {
			if tinfo == nil {
				fmt.Printf("✕ %s: can NOT parse comment meta (invalid torrent)\n", torrent)
				errorCnt++
				continue
			} else if commentMeta := tinfo.DecodeComment(); commentMeta == nil {
				fmt.Printf("✕ %s: failed to parse comment meta\n", torrent)
				errorCnt++
				continue
			} else {
				log.Debugf("Found and use torrent %s comment meta %v", torrent, commentMeta)
				option.Category = commentMeta.Category
				option.Tags = commentMeta.Tags
				option.SavePath = commentMeta.SavePath
				if option.SavePath != "" && savePathMapper != nil {
					if _savePath, match := savePathMapper.Before2After(option.SavePath); !match {
						fmt.Printf("✕ %s: failed to map comment meta save path %q\n", torrent, option.SavePath)
						errorCnt++
						continue
					} else {
						option.SavePath = _savePath
					}
				}
				tinfo.SetComment(commentMeta.Comment)
				if data, err := tinfo.ToBytes(); err == nil {
					content = data
				}
			}
		}
		// it category & tags & savePath options are not set by comment-meta, set them with flag values
		if option.Category == "" {
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
		}
		if option.Tags == nil {
			if tinfo != nil {
				if tinfo.IsPrivate() {
					option.Tags = append(option.Tags, config.PRIVATE_TAG)
				} else {
					option.Tags = append(option.Tags, config.PUBLIC_TAG)
				}
			}
			if sitename != "" {
				option.Tags = append(option.Tags, client.GenerateTorrentTagFromSite(sitename))
			}
			if hr {
				option.Tags = append(option.Tags, config.HR_TAG)
			}
			option.Tags = append(option.Tags, fixedTags...)
			if renameTemplate != nil {
				name, err := torrentutil.RenameTorrent(renameTemplate, sitename, id, filename, tinfo, false)
				if err == nil {
					option.Name = name
				} else {
					log.Errorf("torrent %s rename template render failed and is not renamed: %v", torrent, err)
				}
			}
		}
		if ratioLimit == 0 {
			if tinfo == nil || tinfo.IsPrivate() {
				option.RatioLimit = 0
			} else {
				option.RatioLimit = config.Get().PublicTorrentRatioLimit
			}
		}
		if option.SavePath == "" {
			option.SavePath = savePath
		}
		err = clientInstance.AddTorrent(content, option, nil)
		if err != nil {
			fmt.Printf("✕ %s (site=%s): failed to add torrent to client: %v // %s (%s)\n",
				torrent, sitename, err, contentPath, util.BytesSize(float64(size)))
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
		fmt.Printf("✓ %s (site=%s). infoHash=%s // %s (%s)\n",
			torrent, sitename, infoHash, contentPath, util.BytesSize(float64(size)))
	}
	fmt.Fprintf(os.Stderr, "\n// Done. Added torrent (Size/Cnt): %s / %d; ErrorCnt: %d\n",
		util.BytesSize(float64(sizeAdded)), cntAdded, errorCnt)
	if errorCnt > 0 {
		return fmt.Errorf("%d errors", errorCnt)
	}
	return nil
}
