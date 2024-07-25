package publish

import (
	"bytes"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/natefinch/atomic"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"github.com/sagan/ptool/client"
	"github.com/sagan/ptool/cmd"
	"github.com/sagan/ptool/cmd/common"
	"github.com/sagan/ptool/config"
	"github.com/sagan/ptool/constants"
	"github.com/sagan/ptool/site"
	"github.com/sagan/ptool/util"
	"github.com/sagan/ptool/util/helper"
	"github.com/sagan/ptool/util/torrentutil"
)

const EXISTING_FLAG_FILE = ".existing-%s" // %s: sitename
const PUBLISHED_FLAG_FILE = ".published-%s"
const PUBLISHED_TORRENT_FILENAME = ".%s.torrent"
const COVER = "cover"

var command = &cobra.Command{
	Use:   "publish --site {site} { --content-path {content-path} | --save-path {save-path} } --client {client}",
	Short: "Publish (upload) torrent to site.",
	Long:  `Publish (upload) torrent to site.`,
	Args:  cobra.MatchAll(cobra.ExactArgs(0), cobra.OnlyValidArgs),
	RunE:  publish,
}

var (
	ErrInvalidMetadataFile = fmt.Errorf("invalid metadata file")
	ErrNoMetadataFile      = fmt.Errorf("no metadata file")
	ErrExisting            = fmt.Errorf("same contents torrent exists in site")
	ErrMayExisting         = fmt.Errorf("same contents torrent may exists in site")
	ErrAlreadyPublished    = fmt.Errorf("already published")
	ErrSmall               = fmt.Errorf("torrent contents is too small")
)

var (
	addPaused             = false
	noCover               = false
	doCheck               = false
	skipCheck             = false
	dryRun                = false
	checkExisting         = false
	showJson              = false
	maxTorrents           = int64(0)
	maxPublishingTorrents = int64(0)
	minTorrentSizeStr     = ""
	sitename              = ""
	clientname            = ""
	contentPath           = ""
	savePath              = ""
	comment               = ""
	commentFile           = ""
	moveOkTo              = ""
	moveFailTo            = ""
	mustTag               = ""
	metaArrayKeysStr      = ""
	maxTotalSizeStr       = ""
	addCategory           = ""
	addTagsStr            = ""
	imageFiles            []string
	fields                []string
	mapSavePaths          []string
)

func init() {
	command.Flags().BoolVarP(&addPaused, "add-paused", "", false,
		`Used with "--client {client}". Add published torrents to client in paused state`)
	command.Flags().BoolVarP(&noCover, "no-cover", "", false,
		`Do not use and upload "cover.<webp/png/jpg>" file to site images server`)
	command.Flags().BoolVarP(&doCheck, "check", "", false,
		"Do full torrent hashing check with disk contents if .torrent already exists")
	command.Flags().BoolVarP(&skipCheck, "skip-check", "", false,
		"Skip checking torrent hash with disk contents if .torrent already exists")
	command.Flags().BoolVarP(&dryRun, "dry-run", "d", false, "Dry run. Do NOT actually upload torrent to site")
	command.Flags().BoolVarP(&checkExisting, "check-existing", "", false,
		"Check whether same contents torrent already exists in site before publishing")
	command.Flags().BoolVarP(&showJson, "json", "", false, "Show output in json format")
	command.Flags().Int64VarP(&maxTorrents, "max-torrents", "", -1,
		"Number limit of (successfully) published torrents. -1 == no limit")
	command.Flags().Int64VarP(&maxPublishingTorrents, "max-publishing-torrents", "", -1,
		"Number limit of publishing torrents. -1 == no limit")
	command.Flags().StringVarP(&addCategory, "add-category", "", config.SEEDING_CAT,
		`Used with "--client {client}". Set category of published torrents in BitTorrent client`)
	command.Flags().StringVarP(&addTagsStr, "add-tags", "", "",
		`Used with "--client {client}". Add tags to published torrent in BitTorrent client (comma-separated)`)
	command.Flags().StringVarP(&maxTotalSizeStr, "max-total-size", "", "-1",
		"Will at most publish torrents with total contents size of this value. -1 == no limit")
	command.Flags().StringVarP(&mustTag, "must-tag", "", "", "Comma-separated tag list. "+
		`If set, only content folders which tags contains any one in the list will be published`)
	command.Flags().StringVarP(&minTorrentSizeStr, "min-torrent-size", "", "1MiB",
		"Do not publish torrent which contents size is smaller than (<) this value. -1 == no limit")
	command.Flags().StringVarP(&sitename, "site", "", "", "Publish site")
	command.Flags().StringVarP(&clientname, "client", "", "",
		"Local client name. Add torrent to it to start seeding it after published the torrent")
	command.Flags().StringVarP(&contentPath, "content-path", "", "", "Content path to publish")
	command.Flags().StringVarP(&savePath, "save-path", "", "", "Save path of contents to publish")
	command.Flags().StringVarP(&comment, "comment", "", "", `Publish comment. Equivalent to '--meta "comment=..."'`)
	command.Flags().StringVarP(&commentFile, "comment-file", "", "", `Read comment from file`)
	command.Flags().StringVarP(&moveOkTo, "move-ok-to", "", "",
		"Move successfully processed content folders to this new save path. Note it applies even in dry run mode")
	command.Flags().StringVarP(&moveFailTo, "move-fail-to", "", "",
		"Move content folders that failed to publish to this path. It doesn't count content folders that "+
			"do not have a metainfo.nfo file or fail to publish due to network or other temporary problems")
	command.Flags().StringVarP(&metaArrayKeysStr, "meta-array-keys", "", config.METADATA_ARRAY_KEYS,
		"Array type meta data keys. Comma-separated list. "+
			"Variables of theses names in meta data will be treated and rendered as array. If a such variable in "+
			constants.METADATA_FILE+" is string type, it's will be splitted to array as CSV (comma-separated values)")
	command.Flags().StringArrayVarP(&imageFiles, "image", "", nil,
		`Extra image (in addition to "cover.*") file names that will also be used in meta of uploaded torrent. `+
			`Filename or filename pattern relative to content path of torrent. E.g. "screenshot-*.png". `+
			`Non-existent file names or names without a valid image extension are ignored`)
	command.Flags().StringArrayVarP(&fields, "meta", "", nil,
		`Manually set meta values of torrent(s) to publish. Url query string format. E.g. "title=foo&author=bar"`)
	command.Flags().StringArrayVarP(&mapSavePaths, "map-save-path", "", nil,
		`Used with "--use-comment-meta". Map save path from local file system to the file system of BitTorrent client. `+
			`Format: "local_path|client_path". `+constants.HELP_ARG_PATH_MAPPERS)
	command.MarkFlagRequired("site")
	cmd.RootCmd.AddCommand(command)
}

func publish(cmd *cobra.Command, args []string) (err error) {
	if util.CountNonZeroVariables(contentPath, savePath) != 1 {
		return fmt.Errorf("exact one of the --content-path and --save-path flsgs must be set")
	}
	if doCheck && skipCheck {
		return fmt.Errorf("--check and --skip-check flags are NOT compatible")
	}
	maxTotalSize, _ := util.RAMInBytes(maxTotalSizeStr)
	if comment != "" && commentFile != "" {
		return fmt.Errorf("--comment and --comment-file flags are NOT compatible")
	}
	if commentFile != "" {
		contents, err := os.ReadFile(commentFile)
		if err != nil {
			return fmt.Errorf("failed to read comment file: %w", err)
		}
		comment = string(contents)
	}
	addTags := util.SplitCsv(addTagsStr)
	comment = strings.TrimSpace(comment)
	minTorrentSize, _ := util.RAMInBytes(minTorrentSizeStr)
	metaValues := url.Values{}
	for _, field := range fields {
		values, err := url.ParseQuery(field)
		if err != nil {
			return fmt.Errorf("inalid field value: %w", err)
		}
		for name, value := range values {
			metaValues[name] = value
		}
	}
	if comment != "" {
		metaValues.Set("comment", comment)
	}
	if noCover {
		metaValues.Set(constants.METADATA_KEY_NO_COVER, "1")
	}
	mustTags := util.SplitCsv(mustTag)
	metaArrayKeys := util.SplitCsv(metaArrayKeysStr)

	var savePathMapper *common.PathMapper
	if len(mapSavePaths) > 0 {
		savePathMapper, err = common.NewPathMapper(mapSavePaths)
		if err != nil {
			return fmt.Errorf("invalid map-save-path(s): %w", err)
		}
	}
	siteInstance, err := site.CreateSite(sitename)
	if err != nil {
		return fmt.Errorf("failed to create site: %w", err)
	}
	if httpClient, _, err := site.CreateSiteHttpClient(siteInstance.GetSiteConfig(), config.Get()); err == nil {
		// workaround: force increase site http client timeout.
		if httpClient.TimeOut < time.Second*30 {
			httpClient.SetTimeout(time.Second * 30)
		}
	}
	if _, err := siteInstance.GetStatus(); err != nil {
		return fmt.Errorf("failed to get site status: %w", err)
	}
	var clientInstance client.Client
	if clientname != "" {
		clientInstance, err = client.CreateClient(clientname)
		if err != nil {
			return fmt.Errorf("failed to create client: %w", err)
		}
		if _, err := clientInstance.GetStatus(); err != nil {
			return fmt.Errorf("client status is not ok: %w", err)
		}
	}
	contentPathes := []string{} // abs pathes
	if savePath != "" {
		entries, err := os.ReadDir(savePath)
		if err != nil {
			return fmt.Errorf("failed to read save path: %w", err)
		}
		for _, entry := range entries {
			if !entry.IsDir() || strings.HasPrefix(entry.Name(), ".") {
				continue
			}
			contentPathes = append(contentPathes, filepath.Join(savePath, entry.Name()))
		}
	} else {
		contentPathes = append(contentPathes, contentPath)
	}
	if moveOkTo != "" && !util.DirExists(moveOkTo) {
		return fmt.Errorf("move-ok-to dir %q does not exist or is not dir", moveOkTo)
	}
	if moveFailTo != "" && !util.DirExists(moveFailTo) {
		return fmt.Errorf("move-fail-to dir %q does not exist or is not dir", moveFailTo)
	}

	errorCnt := int64(0)
	cntHandled := int64(0)
	cntPublished := int64(0)
	sizePublished := int64(0)
	for i, contentPath := range contentPathes {
		fmt.Printf("(%d/%d) ", i+1, len(contentPathes))
		id, tinfo, err := publicTorrent(siteInstance, clientInstance, contentPath, metaValues, true, checkExisting,
			savePathMapper, minTorrentSize, imageFiles, moveOkTo, dryRun, mustTags, metaArrayKeys, addCategory, addTags)
		ok, published := printResult(contentPath, id, err, sitename, clientname)
		if !ok {
			if moveFailTo != "" && (err == ErrExisting || err == ErrSmall || err == ErrInvalidMetadataFile) {
				targetpath := filepath.Join(moveFailTo, filepath.Base(contentPath))
				if util.FileExists(targetpath) {
					log.Errorf("move-fail-to target %q already exists", targetpath)
				} else if err := atomic.ReplaceFile(contentPath, targetpath); err != nil {
					log.Errorf("failed to move to move-fail-to target %q: %v", targetpath, err)
				}
			}
			errorCnt++
		}
		if !ok || published {
			cntHandled++
		}
		if err == nil {
			sizePublished += tinfo.Size
			cntPublished++
		}
		if maxTorrents > 0 && cntPublished >= maxTorrents ||
			maxPublishingTorrents > 0 && cntHandled >= maxPublishingTorrents ||
			maxTotalSize > 0 && sizePublished >= maxTotalSize {
			break
		}
	}
	if errorCnt > 0 {
		return fmt.Errorf("%d errors", errorCnt)
	}
	return nil
}

// Read a yaml front matter style metafile from metadataFile. E.g.:
//
// ---
// title: foo
// author: bar
// ---
//
// any text...
//
// The YAML front matter must reside in the beginning of the metadataFile file.
// The file must be UTF-8 without BOM + \n line breaks.
// Spaces inside meta key are converted to "_".
// Meta variables of arrayKeys are treated as slice;
// If a value of such variable is string type, splitted it to slice as csv.
func parseMetadataFile(metadataFile string, arrayKeys []string) (metadata url.Values, err error) {
	contents, err := os.ReadFile(metadataFile)
	if err != nil {
		return nil, err
	}
	deli := []byte("---\n")
	if len(contents) < 10 || !bytes.Equal(contents[:len(deli)], deli) {
		return nil, fmt.Errorf("no yaml front matter")
	}
	contents = contents[len(deli):]
	deli = []byte("\n---\n")
	index := bytes.Index(contents, deli)
	if index < 3 {
		return nil, fmt.Errorf("no yaml front matter")
	}
	text := strings.TrimSpace(string(contents[index+len(deli):]))
	contents = contents[:index]
	var rawMetadata map[string]any
	if err = yaml.Unmarshal(contents, &rawMetadata); err != nil {
		return nil, err
	}
	metadata = url.Values{}
	for key, value := range rawMetadata {
		if strings.ContainsAny(key, " \t") {
			key = strings.ReplaceAll(key, " ", "_")
			key = strings.ReplaceAll(key, "\t", "_")
		}
		switch v := value.(type) {
		case string:
			if slices.Contains(arrayKeys, key) {
				metadata[key] = util.SplitCsv(v)
			} else {
				metadata.Set(key, v)
			}
		case []string:
			metadata[key] = v
		case []any:
			for _, vi := range v {
				metadata.Add(key, fmt.Sprint(vi))
			}
		default:
			metadata.Set(key, fmt.Sprint(v))
		}
	}
	metadata.Set("_text", text)
	metadata.Set("_meta", string(contents))
	return metadata, nil
}

func publicTorrent(siteInstance site.Site, clientInstance client.Client, contentPath string, otherFields url.Values,
	mustMetadataFile bool, checkExisting bool, savePathMapper *common.PathMapper, minTorrentSize int64,
	imageFiles []string, moveOk string, dryRun bool, mustTags []string, metaArrayKeys []string,
	addCategory string, addTags []string) (
	id string, tinfo *torrentutil.TorrentMeta, err error) {
	targetContentPath := contentPath
	if moveOk != "" {
		targetContentPath = filepath.Join(moveOk, filepath.Base(contentPath))
		if util.FileExists(targetContentPath) {
			return "", nil, fmt.Errorf("target path in move-ok-to dir %q already exists", targetContentPath)
		}
	}
	if savePathMapper != nil {
		// check early if path mapper will work
		savePath := filepath.Dir(targetContentPath)
		_, match := savePathMapper.Before2After(savePath)
		if !match {
			return "", nil, fmt.Errorf("local path %q can not be mapped to client path", savePath)
		}
	}
	sitename := siteInstance.GetName()
	var metadata url.Values
	if metadataFile := filepath.Join(contentPath, constants.METADATA_FILE); util.FileExists(metadataFile) {
		log.Debugf("Parse medadata file %s", metadataFile)
		metadata, err = parseMetadataFile(metadataFile, metaArrayKeys)
		if err != nil {
			return "", nil, ErrInvalidMetadataFile
		}
	} else if mustMetadataFile {
		return "", nil, ErrNoMetadataFile
	} else {
		metadata = url.Values{}
	}
	for name, value := range otherFields {
		metadata[name] = value
	}
	metadata[constants.METADATA_KEY_ARRAY_KEYS] = metaArrayKeys
	if metadata.Get("title") == "" {
		return "", nil, fmt.Errorf("no title meta data found")
	}
	if mustTags != nil && !slices.ContainsFunc(mustTags, func(t string) bool {
		return slices.Contains(metadata["tags"], t)
	}) {
		return "", nil, fmt.Errorf("torrent metadata does not has any tag of %v", mustTags)
	}
	if len(imageFiles) > 0 {
		var images []string
		for _, imageFile := range imageFiles {
			imageFilePath := ""
			if filepath.IsAbs(imageFile) {
				imageFilePath = filepath.Clean(imageFile)
			} else {
				imageFilePath = filepath.Join(contentPath, imageFile)
			}
			files := helper.GetWildcardFilenames(imageFilePath)
			if files == nil {
				files = append(files, imageFilePath)
			}
			for _, file := range files {
				if !util.HasAnySuffix(file, constants.ImgExts...) || !util.FileExists(file) {
					continue
				}
				images = append(images, file)
			}
			images = util.UniqueSlice(images)
		}
		if len(images) > 0 {
			metadata[constants.METADATA_KEY_IMAGES] = images
		}
	}
	if dryRun {
		metadata.Set(constants.METADATA_KEY_DRY_RUN, "1")
	}

	publishedFlagFile := filepath.Join(contentPath, fmt.Sprintf(PUBLISHED_FLAG_FILE, sitename))
	if util.FileExists(publishedFlagFile) {
		if targetContentPath != contentPath {
			if err = atomic.ReplaceFile(contentPath, targetContentPath); err != nil {
				return "", nil, fmt.Errorf("torrent already published but failed to move content folder: %w", err)
			}
		}
		if err := downloadPublishedTorrent(siteInstance, clientInstance, targetContentPath,
			savePathMapper, addCategory, addTags); err != nil {
			return "", nil, fmt.Errorf("failed to download published torrent: %w", err)
		}
		return "", nil, ErrAlreadyPublished
	}
	existingFlagFile := filepath.Join(contentPath, fmt.Sprintf(EXISTING_FLAG_FILE, sitename))
	if util.FileExists(existingFlagFile) {
		return "", nil, ErrExisting
	}
	if number := metadata.Get("number"); number != "" && checkExisting {
		torrents, err := siteInstance.SearchTorrents(number, "")
		if err != nil {
			return "", nil, fmt.Errorf("failed to search site torrents to check existing: %w", err)
		}
		regexp := regexp.MustCompile(`\b` + regexp.QuoteMeta(number) + `\b`)
		slices.SortFunc(torrents, func(a, b *site.Torrent) int { return int(b.Time - a.Time) })
		var existingTorrent *site.Torrent
		for _, torrent := range torrents {
			if regexp.MatchString(torrent.Name) || regexp.MatchString(torrent.Description) {
				existingTorrent = torrent
				break
			}
		}
		if existingTorrent != nil {
			errorCheckExistingTorrent := false
			// workaround for https://github.com/xiaomlove/nexusphp/issues/264 .
			beforePublished := false
			id := existingTorrent.ID()
			(func() {
				if existingTorrent.Seeders > 0 {
					return
				}
				torrentContents, _, err := siteInstance.DownloadTorrentById(id)
				if err != nil {
					log.Debugf("failed to download site existing torrent %s: %v", id, err)
					errorCheckExistingTorrent = true
					return
				}
				tinfo, err := torrentutil.ParseTorrent(torrentContents)
				if err != nil {
					return
				}
				if _, err = tinfo.Verify("", contentPath, 1); err != nil {
					return
				}
				if err = atomic.WriteFile(publishedFlagFile, strings.NewReader(id)); err != nil {
					return
				}
				torrentFilename := filepath.Join(contentPath, fmt.Sprintf(PUBLISHED_TORRENT_FILENAME, sitename))
				atomic.WriteFile(torrentFilename, bytes.NewReader(torrentContents))
				beforePublished = true
			})()
			if beforePublished {
				if targetContentPath != contentPath {
					if err = atomic.ReplaceFile(contentPath, targetContentPath); err != nil {
						return id, nil, fmt.Errorf("torrent published (id: %s) but failed to move content folder: %w", id, err)
					}
				}
				if err := downloadPublishedTorrent(siteInstance, clientInstance,
					targetContentPath, savePathMapper, addCategory, addTags); err != nil {
					return "", nil, fmt.Errorf("failed to download published torrent: %w", err)
				}
				return "", nil, ErrAlreadyPublished
			}
			if errorCheckExistingTorrent {
				// 站点种子存在，但做种人数为0，并且由于下载种子失败，无法确定该种子与本地文件夹是否是相同种子。
				// 所以仅本次返回错误，但不写入 existing flag 标记文件。下次运行时，会再次尝试下载种子检查。
				return "", nil, ErrMayExisting
			}
			atomic.WriteFile(existingFlagFile, strings.NewReader(id))
			return "", nil, ErrExisting
		}
	}

	torrent := filepath.Join(contentPath, constants.META_TORRENT_FILE)
	makeTorrentOptions := &torrentutil.TorrentMakeOptions{
		Force:          true,
		ContentPath:    contentPath,
		Private:        true,
		PieceLengthStr: constants.TORRENT_DEFAULT_PIECE_LENGTH,
		Output:         torrent,
		MinSize:        minTorrentSize,
		Excludes:       []string{constants.METADATA_FILE},
	}
	if !util.FileExists(torrent) {
		log.Debugf("torrent file %q does not exists, make it", torrent)
		if _, err := torrentutil.MakeTorrent(makeTorrentOptions); err != nil {
			if err == torrentutil.ErrSmall {
				return "", nil, ErrSmall
			}
			return "", nil, fmt.Errorf("failed to make torrent: %w", err)
		}
	} else {
		log.Debugf("torrent file %q exists, try to use it", torrent)
	}
	torrentStat, err := os.Stat(torrent)
	if err != nil {
		return "", nil, fmt.Errorf("failed to read torrent stat: %w", err)
	}
	torrentContents, err := os.ReadFile(torrent)
	if err != nil {
		return "", nil, fmt.Errorf("failed to read torrent: %w", err)
	}
	tinfo, err = torrentutil.ParseTorrent(torrentContents)
	if err != nil {
		return "", nil, fmt.Errorf("failed to parse torrent: %w", err)
	}
	var ts int64
	checkHashMode := int64(1)
	if skipCheck {
		checkHashMode = 0
	} else if doCheck {
		checkHashMode = 2
	}
	if ts, err = tinfo.Verify("", contentPath, checkHashMode); err != nil || ts > torrentStat.ModTime().Unix() {
		log.Debugf(".torrent file is obsolete (verify err=%v, content_ts=%d, torrent_ts=%d), re-make torrent",
			err, ts, torrentStat.ModTime().Unix())
		if tinfo, err = torrentutil.MakeTorrent(makeTorrentOptions); err != nil {
			if err == torrentutil.ErrSmall {
				return "", nil, ErrSmall
			}
			return "", nil, fmt.Errorf("failed to make torrent: %w", err)
		}
		torrentContents, err = os.ReadFile(torrent)
		if err != nil {
			return "", nil, fmt.Errorf("failed to read torrent: %w", err)
		}
	}
	if minTorrentSize > 0 && tinfo.Size < minTorrentSize {
		return "", nil, ErrSmall
	}
	if metadata.Get(constants.METADATA_KEY_NO_COVER) != "1" {
		coverImage := util.ExistsFileWithAnySuffix(filepath.Join(contentPath, COVER), constants.ImgExts)
		if coverImage != "" {
			metadata.Set(constants.METADATA_KEY_COVER, coverImage)
		}
	}
	id, err = siteInstance.PublishTorrent(torrentContents, metadata)
	if err != nil {
		if err == constants.ErrDryRun {
			if targetContentPath != contentPath {
				atomic.ReplaceFile(contentPath, targetContentPath)
			}
			return "", nil, err
		}
		return "", nil, fmt.Errorf("failed to publish torrent: %w", err)
	}
	if id != "" {
		atomic.WriteFile(publishedFlagFile, strings.NewReader(id))
	} else {
		util.TouchFile(existingFlagFile)
	}
	if targetContentPath != contentPath {
		if err = atomic.ReplaceFile(contentPath, targetContentPath); err != nil {
			return id, nil, fmt.Errorf("torrent published (id: %s) but failed to move content folder: %w", id, err)
		}
	}
	err = downloadPublishedTorrent(siteInstance, clientInstance, targetContentPath, savePathMapper, addCategory, addTags)
	if err != nil {
		return id, tinfo, fmt.Errorf("failed to download published torrent: %w", err)
	}
	return id, tinfo, nil
}

// Download published torrent to local, optionaly add it to local client to start seeding.
func downloadPublishedTorrent(siteInstance site.Site, clientInstance client.Client,
	contentPath string, savePathMapper *common.PathMapper, addCategory string, addTags []string) (err error) {
	sitename := siteInstance.GetName()
	torrentFilename := filepath.Join(contentPath, fmt.Sprintf(PUBLISHED_TORRENT_FILENAME, sitename))
	var torrentContents []byte
	if !util.FileExists(torrentFilename) {
		publishedFlagFile := filepath.Join(contentPath, fmt.Sprintf(PUBLISHED_FLAG_FILE, sitename))
		if !util.FileExists(publishedFlagFile) {
			return fmt.Errorf("id file not exists")
		}
		contents, err := os.ReadFile(publishedFlagFile)
		if err != nil {
			return fmt.Errorf("failed to read %s", publishedFlagFile)
		}
		id := string(contents)
		if id == "" {
			return fmt.Errorf("published torrent id file is empty")
		}
		torrentContents, _, err = siteInstance.DownloadTorrentById(id)
		if err != nil {
			return fmt.Errorf("failed to download prerviously published torrent: %w", err)
		}
		if err := atomic.WriteFile(torrentFilename, bytes.NewReader(torrentContents)); err != nil {
			return fmt.Errorf("failed to write downloaded torrent file: %w", err)
		}
	} else {
		torrentContents, err = os.ReadFile(torrentFilename)
		if err != nil {
			return fmt.Errorf("failed to read downloaded torrent file: %w", err)
		}
	}
	savePath := filepath.Dir(contentPath)
	if savePathMapper != nil {
		newSavePath, match := savePathMapper.Before2After(savePath)
		if !match { // Actually it's have been checked previously, so here match should always be true
			return fmt.Errorf("local path %q can not be converted to client path", savePath)
		}
		savePath = newSavePath
	}
	tags := []string{client.GenerateTorrentTagFromSite(sitename), config.PRIVATE_TAG}
	tags = append(tags, addTags...)
	err = clientInstance.AddTorrent(torrentContents, &client.TorrentOption{
		Pause:        addPaused,
		SkipChecking: true,
		SavePath:     savePath,
		Category:     addCategory,
		Tags:         tags,
	}, nil)
	if err != nil {
		return fmt.Errorf("failed to add torrent to client: %w", err)
	}
	return nil
}

// Print result of publishTorrent().
// If result should be reported as en error, return ok=false. Otherwise return ok=true.
func printResult(contentPath string, id string, err error,
	sitename string, clientname string) (ok bool, published bool) {
	switch err {
	case nil:
		torrentFilename := filepath.Join(contentPath, fmt.Sprintf(PUBLISHED_TORRENT_FILENAME, sitename))
		if clientname != "" {
			fmt.Printf("✓ %q: published as id %s (%s)\n", contentPath, id, torrentFilename)
		} else {
			fmt.Printf("✓ %q: published as id %s (%s); added to client\n", contentPath, id, torrentFilename)
		}
		ok = true
		published = true
	case constants.ErrDryRun:
		fmt.Printf("→ %q: Ready to publish to site (Dry Run)\n", contentPath)
		ok = true
		published = true
	case ErrAlreadyPublished:
		fmt.Printf("* %q: %v\n", contentPath, err)
		ok = true
	case ErrNoMetadataFile:
		fmt.Printf("- %q: %v\n", contentPath, err)
		ok = true
	case ErrSmall, ErrExisting:
		fmt.Printf("! %q: %v\n", contentPath, err)
		ok = false
	case ErrMayExisting:
		fmt.Printf("? %q: %v\n", contentPath, err)
		ok = false
	default:
		fmt.Printf("X %q: %v\n", contentPath, err)
	}
	return
}
