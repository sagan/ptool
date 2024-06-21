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
	Use:   "publish --site {site} {--content-path {content-path} | --save-path {save-path} } --client {client}",
	Short: "Publish (upload) torrent to site.",
	Long:  `Publish (upload) torrent to site.`,
	Args:  cobra.MatchAll(cobra.ExactArgs(0), cobra.OnlyValidArgs),
	RunE:  publish,
}

var (
	ErrInvalidMetadataFile = fmt.Errorf("invalid metadata file")
	ErrNoMetadataFile      = fmt.Errorf("no metadata file")
	ErrExisting            = fmt.Errorf("same contents torrent exists in site")
	ErrAlreadyPublished    = fmt.Errorf("already published")
	ErrSmall               = fmt.Errorf("torrent contents is too small")
	ErrFs                  = fmt.Errorf("file system read error")
)

var (
	dryRun            = false
	checkExisting     = false
	showJson          = false
	maxTorrents       = int64(0)
	minTorrentSizeStr = ""
	sitename          = ""
	clientname        = ""
	contentPath       = ""
	savePath          = ""
	comment           = ""
	commentFile       = ""
	moveOkTo          = ""
	mustTag           = ""
	imageFiles        []string
	fields            []string
	mapSavePaths      []string
)

func init() {
	command.Flags().BoolVarP(&dryRun, "dry-run", "d", false, "Dry run. Do NOT actually upload torrent to site")
	command.Flags().BoolVarP(&checkExisting, "check-existing", "", false,
		"Check whether same contents torrent already exists in site before publishing")
	command.Flags().BoolVarP(&showJson, "json", "", false, "Show output in json format")
	command.Flags().Int64VarP(&maxTorrents, "max-torrents", "", -1,
		"Number limit of publishing torrents. -1 == no limit")
	command.Flags().StringVarP(&mustTag, "must-tag", "", "", "Comma-separated tag list. "+
		`If set, only content folders which tags contains any one in the list will be published`)
	command.Flags().StringVarP(&minTorrentSizeStr, "min-torrent-size", "", "100MiB",
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
	var mustTags []string
	if mustTag != "" {
		mustTags = util.SplitCsv(mustTag)
	}

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
	contentPathes := []string{}
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
	if moveOkTo != "" {
		if err = os.MkdirAll(moveOkTo, constants.PERM_DIR); err != nil {
			return fmt.Errorf("move-ok-to dir %q does not exist and cann't be created: %w", moveOkTo, err)
		}
	}

	errorCnt := int64(0)
	cntHandled := int64(0)
	for _, contentPath := range contentPathes {
		id, err := publicTorrent(siteInstance, clientInstance, contentPath, metaValues, true,
			checkExisting, savePathMapper, minTorrentSize, imageFiles, moveOkTo, dryRun, mustTags)
		ok, published := printResult(contentPath, id, err, sitename, clientname)
		if !ok {
			errorCnt++
		}
		if !ok || published {
			cntHandled++
		}
		if maxTorrents > 0 && cntHandled >= maxTorrents {
			break
		}
	}
	if errorCnt > 0 {
		return fmt.Errorf("%d errors", errorCnt)
	}
	return nil
}

// Read a yaml front matter style metafile. E.g.:
//
// ---
// title: foo
// author: bar
// ---
//
// any text...
//
// Spaces inside meta key are converted to "_".
// Some keys are treated as slice: tags, narrator;
// If the value of these keys is string, splitted it to slice as csv.
func parseMetadataFile(metadataFile string) (metadata url.Values, err error) {
	contents, err := os.ReadFile(metadataFile)
	if err != nil {
		return nil, err
	}
	deli := []byte("---\n")
	if len(contents) < 10 || !bytes.Equal(contents[:len(deli)], deli) {
		return nil, ErrInvalidMetadataFile
	}
	contents = contents[len(deli):]
	index := bytes.Index(contents, deli)
	if index < 3 {
		return nil, ErrInvalidMetadataFile
	}
	metaTxt := strings.TrimSpace(string(contents[:index+len(deli)]))
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
			if slices.Contains(constants.MetadataArrayKeys, key) {
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
	metadata.Set("_meta", metaTxt)
	return metadata, nil
}

func publicTorrent(siteInstance site.Site, clientInstance client.Client, contentPath string, otherFields url.Values,
	mustMetadataFile bool, checkExisting bool, savePathMapper *common.PathMapper,
	minTorrentSize int64, imageFiles []string, moveOk string, dryRun bool, mustTags []string) (id string, err error) {
	targetpath := ""
	if moveOk != "" {
		targetpath = filepath.Join(moveOk, filepath.Base(contentPath))
		if util.FileExists(targetpath) {
			return "", fmt.Errorf("target path in move-ok-to dir %q already exists", targetpath)
		}
	}
	sitename := siteInstance.GetName()
	var metadata url.Values
	if metadataFile := filepath.Join(contentPath, constants.METADATA_FILE); util.FileExists(metadataFile) {
		log.Debugf("Parse medadata file %s", metadataFile)
		metadata, err = parseMetadataFile(metadataFile)
		if err != nil {
			return "", ErrInvalidMetadataFile
		}
	} else if mustMetadataFile {
		return "", ErrNoMetadataFile
	} else {
		metadata = url.Values{}
	}
	for name, value := range otherFields {
		metadata[name] = value
	}
	if metadata.Get("title") == "" {
		return "", fmt.Errorf("no title meta data found")
	}
	if mustTags != nil && !slices.ContainsFunc(mustTags, func(t string) bool {
		return slices.Contains(metadata["tags"], t)
	}) {
		return "", fmt.Errorf("torrent metadata does not has any tag of %v", mustTags)
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
			metadata["_images"] = images
		}
	}
	if dryRun {
		metadata.Set("_dryrun", "1")
	}

	publishedFlagFile := filepath.Join(contentPath, fmt.Sprintf(PUBLISHED_FLAG_FILE, sitename))
	if util.FileExists(publishedFlagFile) {
		if err := downloadPublishedTorrent(siteInstance, clientInstance, contentPath, savePathMapper); err != nil {
			return "", fmt.Errorf("failed to download published torrent: %w", err)
		}
		return "", ErrAlreadyPublished
	}
	existingFlagFile := filepath.Join(contentPath, fmt.Sprintf(EXISTING_FLAG_FILE, sitename))
	if util.FileExists(existingFlagFile) {
		return "", ErrExisting
	}
	if number := metadata.Get("number"); number != "" && checkExisting {
		existingTorrentId := ""
		torrents, err := siteInstance.SearchTorrents(number, "")
		if err != nil {
			return "", fmt.Errorf("failed to search site torrents to check existing: %w", err)
		}
		regexp := regexp.MustCompile(`\b` + regexp.QuoteMeta(number) + `\b`)
		for _, torrent := range torrents {
			if regexp.MatchString(torrent.Name) || regexp.MatchString(torrent.Description) {
				existingTorrentId = torrent.Id
				break
			}
		}
		if existingTorrentId != "" {
			atomic.WriteFile(existingFlagFile, strings.NewReader(existingTorrentId))
			return "", ErrExisting
		}
	}

	torrent := filepath.Join(contentPath, constants.META_TORRENT_FILE)
	if !util.FileExists(torrent) {
		log.Debugf("torrent file %q does not exists, make it", torrent)
		_, err := torrentutil.MakeTorrent(&torrentutil.TorrentMakeOptions{
			ContentPath:    contentPath,
			Private:        true,
			PieceLengthStr: constants.TORRENT_DEFAULT_PIECE_LENGTH,
			Output:         torrent,
			MinSize:        minTorrentSize,
			Excludes:       []string{constants.METADATA_FILE},
		})
		if err != nil {
			if err == torrentutil.ErrSmall {
				return "", ErrSmall
			}
			return "", fmt.Errorf("failed to make torrent: %w", err)
		}
	} else {
		log.Debugf("torrent file %q exists, use it", torrent)
	}
	torrentStat, err := os.Stat(torrent)
	if err != nil {
		return "", fmt.Errorf("failed to read torrent stat: %w", err)
	}
	torrentContents, err := os.ReadFile(torrent)
	if err != nil {
		return "", fmt.Errorf("failed to read torrent: %w", err)
	}
	tinfo, err := torrentutil.ParseTorrent(torrentContents)
	if err != nil {
		return "", fmt.Errorf("failed to parse torrent: %w", err)
	}
	if ts, err := tinfo.Verify("", contentPath, 0); err != nil {
		return "", fmt.Errorf("content-path is NOT consistent with existing .torrent file")
	} else if ts > torrentStat.ModTime().Unix() {
		return "", fmt.Errorf("content-path files modification time is newer than existing .torrent file")
	}
	coverImage := util.ExistsFileWithAnySuffix(filepath.Join(contentPath, COVER), constants.ImgExts)
	if coverImage != "" {
		metadata.Set("_cover", coverImage)
	}
	id, err = siteInstance.PublishTorrent(torrentContents, metadata)
	if err != nil {
		if err == constants.ErrDryRun && targetpath != "" {
			atomic.ReplaceFile(contentPath, targetpath)
		}
		return "", err
	}
	if id != "" {
		atomic.WriteFile(publishedFlagFile, strings.NewReader(id))
	} else {
		util.TouchFile(existingFlagFile)
	}
	if targetpath != "" {
		if err = atomic.ReplaceFile(contentPath, targetpath); err != nil {
			return id, fmt.Errorf("torrent published (id: %s) but failed to move content folder: %w", id, err)
		}
		contentPath = targetpath
	}
	err = downloadPublishedTorrent(siteInstance, clientInstance, contentPath, savePathMapper)
	if err != nil {
		return id, err
	}
	return id, nil
}

// Download published torrent to local, optionaly add it to local client to start seeding.
func downloadPublishedTorrent(siteInstance site.Site, clientInstance client.Client,
	contentPath string, savePathMapper *common.PathMapper) (err error) {
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
			return err
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
		if !match {
			return fmt.Errorf("local path %q can not be converted to client path", savePath)
		}
		savePath = newSavePath
	}
	err = clientInstance.AddTorrent(torrentContents, &client.TorrentOption{
		SkipChecking: true,
		SavePath:     savePath,
		Category:     config.SEEDING_CAT,
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
	case ErrNoMetadataFile, ErrExisting:
		fmt.Printf("- %q: %v\n", contentPath, err)
		ok = true
	case ErrSmall:
		fmt.Printf("! %q: %v\n", contentPath, err)
		ok = true
	default:
		fmt.Printf("X %q: %v\n", contentPath, err)
	}
	return
}
