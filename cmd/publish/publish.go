package publish

import (
	"bytes"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"github.com/sagan/ptool/cmd"
	"github.com/sagan/ptool/constants"
	"github.com/sagan/ptool/site"
	"github.com/sagan/ptool/util"
	"github.com/sagan/ptool/util/torrentutil"
)

const EXISTS_FLAG_FILE_PREFIX = ".existing-"
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
	ErrExists              = fmt.Errorf("same contents torrent exists in site")
)

var (
	checkExisting = false
	showJson      = false
	sitename      = ""
	clientname    = ""
	contentPath   = ""
	savePath      = ""
	fields        []string
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
	command.Flags().StringArrayVarP(&fields, "field", "", nil,
		`Additional field values when uploading torrent to site. E.g. "type=42"`)
	command.MarkFlagRequired("site")
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

	siteInstance, err := site.CreateSite(sitename)
	if err != nil {
		return fmt.Errorf("failed to create site: %w", err)
	}

	if contentPath != "" {
		return publicTorrent(siteInstance, contentPath, fieldValues, false, checkExisting)
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
		err = publicTorrent(siteInstance, contentPath, fieldValues, true, checkExisting)
		if err == nil {
			fmt.Printf("✓ %q: published\n", entry.Name())
		} else if err == ErrNoMetadataFile {
			fmt.Printf("- %q: no metadata file, skip it\n", entry.Name())
		} else if err == ErrExists {
			fmt.Printf("- %q: already exists in site\n", entry.Name())
		} else {
			fmt.Printf("X %q: failed to publish: %v\n", entry.Name(), err)
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

func publicTorrent(siteInstance site.Site, contentPath string,
	otherFields url.Values, mustMetadataFile bool, checkExisting bool) error {
	sitename := siteInstance.GetName()
	metadata := url.Values{}
	if metadataFile := filepath.Join(contentPath, constants.METADATA_FILE); util.FileExists(metadataFile) {
		log.Debugf("Parse medadata file %s", metadataFile)
		if metadataFileValues, err := parseMetadataFile(metadataFile); err == nil {
			for key, value := range metadataFileValues {
				metadata.Set(key, value)
			}
		} else {
			return ErrInvalidMetadataFile
		}
	} else if mustMetadataFile {
		return ErrNoMetadataFile
	}
	for name, value := range otherFields {
		metadata[name] = value
	}
	if metadata.Get("title") == "" {
		return fmt.Errorf("the following meta fields must has value: title")
	}

	existsFlagFile := filepath.Join(contentPath, EXISTS_FLAG_FILE_PREFIX+sitename)
	if util.FileExists(existsFlagFile) {
		return ErrExists
	}
	if number := metadata.Get("number"); number != "" && checkExisting {
		existsTorrentId := ""
		torrents, err := siteInstance.SearchTorrents(number, "")
		if err != nil {
			return fmt.Errorf("failed to search site torrents to check existing: %w", err)
		}
		regexp := regexp.MustCompile(`\b` + regexp.QuoteMeta(number) + `\b`)
		for _, torrent := range torrents {
			if regexp.MatchString(torrent.Name) || regexp.MatchString(torrent.Description) {
				existsTorrentId = torrent.Id
				break
			}
		}
		if existsTorrentId != "" {
			os.WriteFile(existsFlagFile, []byte(existsTorrentId), 0600)
			return ErrExists
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
			return fmt.Errorf("failed to make torrent: %w", err)
		}
	} else {
		log.Debugf("torrent file %q exists, use it", torrent)
	}
	torrentContents, err := os.ReadFile(torrent)
	if err != nil {
		return fmt.Errorf("failed to read torrent: %w", err)
	}
	tinfo, err := torrentutil.ParseTorrent(torrentContents)
	if err != nil {
		return fmt.Errorf("failed to parse torrent: %w", err)
	}
	if result := tinfo.Verify("", contentPath, 0); result != nil {
		return fmt.Errorf("content-path is NOT consistent with existing .torrent file")
	}
	coverImage := util.ExistsFileWithAnySuffix(filepath.Join(contentPath, COVER), constants.ImgExts)
	if coverImage != "" {
		metadata.Set("_cover", coverImage)
	}
	id, err := siteInstance.PublishTorrent(torrentContents, metadata)
	if err == nil {
		if id != "" {
			os.WriteFile(existsFlagFile, []byte(id), 0600)
		} else {
			util.TouchFile(existsFlagFile)
		}
	}
	return err
}
