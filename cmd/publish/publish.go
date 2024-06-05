package publish

import (
	"bytes"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
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
	"github.com/sagan/ptool/util/torrentutil"
)

const EXISTING_FLAG_FILE = ".existing-%s" // %s: sitename
const PUBLISHED_FLAG_FILE = ".published-%s"
const PUBLISHED_TORRENT_FILENAME = ".%s.torrent"
const COVER = "cover"

var command = &cobra.Command{
	Use:   "publish --site {site} {--content-path {content-path} | --save-path } --client {client}",
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
	ErrFs                  = fmt.Errorf("file system read error")
)

var (
	checkExisting = false
	showJson      = false
	sitename      = ""
	clientname    = ""
	contentPath   = ""
	savePath      = ""
	fields        []string
	mapSavePaths  []string
)

func init() {
	command.Flags().BoolVarP(&checkExisting, "check-existing", "", false,
		"Check whether same contents torrent already exists in site before publishing")
	command.Flags().BoolVarP(&showJson, "json", "", false, "Show output in json format")
	command.Flags().StringVarP(&sitename, "site", "", "", "Publish site")
	command.Flags().StringVarP(&clientname, "client", "", "",
		"Local client name. Add torrent to it to start seeding it after published the torrent")
	command.Flags().StringVarP(&contentPath, "content-path", "", "", "Content path to publish")
	command.Flags().StringVarP(&savePath, "save-path", "", "", "Save path of contents to publish")
	command.Flags().StringArrayVarP(&fields, "meta", "", nil,
		`Additional meta info values when uploading torrent to site. E.g. "type=42&subtype=12"`)
	command.MarkFlagRequired("site")
	command.Flags().StringArrayVarP(&mapSavePaths, "map-save-path", "", nil,
		`Used with "--use-comment-meta". Map save path from local file system to the file system of BitTorrent client. `+
			`Format: "local_path|client_path". `+constants.HELP_ARG_PATH_MAPPERS)
	cmd.RootCmd.AddCommand(command)
}

func publish(cmd *cobra.Command, args []string) (err error) {
	if util.CountNonZeroVariables(contentPath, savePath) != 1 {
		return fmt.Errorf("exact one of the --content-path and --save-path flsgs must be set")
	}
	fieldValues := url.Values{}
	for _, field := range fields {
		values, err := url.ParseQuery(field)
		if err != nil {
			return fmt.Errorf("inalid field value: %w", err)
		}
		for name, value := range values {
			fieldValues[name] = value
		}
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

	if contentPath != "" {
		id, err := publicTorrent(siteInstance, clientInstance,
			contentPath, fieldValues, false, checkExisting, savePathMapper)
		ok := printResult(contentPath, id, err, clientname)
		if !ok {
			return err
		} else {
			return nil
		}
	}

	errorCnt := int64(0)
	entries, err := os.ReadDir(savePath)
	if err != nil {
		return fmt.Errorf("failed to read dir: %w", err)
	}
	for _, entry := range entries {
		if !entry.IsDir() || strings.HasPrefix(entry.Name(), ".") {
			continue
		}
		contentPath := filepath.Join(savePath, entry.Name())
		id, err := publicTorrent(siteInstance, clientInstance,
			contentPath, fieldValues, true, checkExisting, savePathMapper)
		ok := printResult(contentPath, id, err, clientname)
		if !ok {
			errorCnt++
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
func parseMetadataFile(metadataFile string) (metadata map[string]string, err error) {
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
	text := strings.TrimSpace(string(contents[index+len(deli):]))
	contents = contents[:index]
	if err = yaml.Unmarshal(contents, &metadata); err != nil {
		return nil, err
	}
	metadata["text"] = text
	return metadata, nil
}

func publicTorrent(siteInstance site.Site, clientInstance client.Client, contentPath string, otherFields url.Values,
	mustMetadataFile bool, checkExisting bool, savePathMapper *common.PathMapper) (id string, err error) {
	sitename := siteInstance.GetName()
	metadata := url.Values{}
	if metadataFile := filepath.Join(contentPath, constants.METADATA_FILE); util.FileExists(metadataFile) {
		log.Debugf("Parse medadata file %s", metadataFile)
		if metadataFileValues, err := parseMetadataFile(metadataFile); err == nil {
			for key, value := range metadataFileValues {
				metadata.Set(key, value)
			}
		} else {
			return "", ErrInvalidMetadataFile
		}
	} else if mustMetadataFile {
		return "", ErrNoMetadataFile
	}
	for name, value := range otherFields {
		metadata[name] = value
	}
	if metadata.Get("title") == "" {
		return "", fmt.Errorf("the following meta fields must has value: title")
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
			Excludes:       []string{constants.METADATA_FILE},
		})
		if err != nil {
			return "", fmt.Errorf("failed to make torrent: %w", err)
		}
	} else {
		log.Debugf("torrent file %q exists, use it", torrent)
	}
	torrentContents, err := os.ReadFile(torrent)
	if err != nil {
		return "", fmt.Errorf("failed to read torrent: %w", err)
	}
	tinfo, err := torrentutil.ParseTorrent(torrentContents)
	if err != nil {
		return "", fmt.Errorf("failed to parse torrent: %w", err)
	}
	if result := tinfo.Verify("", contentPath, 0); result != nil {
		return "", fmt.Errorf("content-path is NOT consistent with existing .torrent file")
	}
	coverImage := util.ExistsFileWithAnySuffix(filepath.Join(contentPath, COVER), constants.ImgExts)
	if coverImage != "" {
		metadata.Set("_cover", coverImage)
	}
	id, err = siteInstance.PublishTorrent(torrentContents, metadata)
	if err != nil {
		return "", err
	}
	if id != "" {
		atomic.WriteFile(publishedFlagFile, strings.NewReader(id))
	} else {
		util.TouchFile(existingFlagFile)
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
	savePath := filepath.Base(contentPath)
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
// If result should be reported as en error, return false. Otherwise return true.
func printResult(path string, id string, err error, clientname string) (ok bool) {
	if err == nil {
		if clientname != "" {
			fmt.Printf("✓ %q: published as %s\n", path, id)
		} else {
			fmt.Printf("✓ %q: published as %s; added to client\n", path, id)
		}
		return true
	} else if err == ErrNoMetadataFile {
		fmt.Printf("- %q: no metadata file, skip it\n", path)
		return true
	} else if err == ErrAlreadyPublished {
		fmt.Printf("- %q: already published to site before\n", path)
		return true
	} else if err == ErrExisting {
		fmt.Printf("- %q: already exists in site\n", path)
		return true
	} else {
		fmt.Printf("X %q: failed to publish: %v\n", path, err)
		return false
	}
}
