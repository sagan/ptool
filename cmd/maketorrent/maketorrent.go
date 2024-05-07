// Adopted from https://github.com/anacrolix/torrent/blob/master/cmd/torrent/create.go .
package maketorrent

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/anacrolix/torrent/bencode"
	"github.com/anacrolix/torrent/metainfo"
	pathspec "github.com/shibumi/go-pathspec"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/sagan/ptool/cmd"
	"github.com/sagan/ptool/constants"
	"github.com/sagan/ptool/util"
	"github.com/sagan/ptool/util/torrentutil"
)

var command = &cobra.Command{
	Use:         "maketorrent {content-path}",
	Aliases:     []string{"createtorrent", "mktorrent"},
	Annotations: map[string]string{"cobra-prompt-dynamic-suggestions": "maketorrent"},
	Short:       "Make (create) a .torrent (metainfo) file from content folder or file in file system.",
	Long: fmt.Sprintf(`Make (create) a .torrent (metainfo) file from content folder or file in file system.
By default, it saves created torrent to "{content-name}.torrent" file,
where "{content-name}" is is folder or file name of "{content-path}".
To manually set the output .torrent filename, use "--output" flag; set it to "-" to directly output to stdout.
It creates BitTorrent v1 format torrent. V2 or hybrid format is NOT supported at this time.

Examples:
  ptool maketorrent ./MyVideos # output: ./MyVideos.torrent

  # --public : add common open trackers to created torrent
  # --excludes : Prevent *.txt files from being indexed in created torrent
  ptool maketorrent ./MyVideos --public --excludes "*.txt"

By default, certain patterns files inside content-path will be ignored and NOT indexed in created .torrent file:
  %s
If "--all" flag is set, the default exclude-patterns will be disabled and ALL files will in indexed.
It's possible to provide your own customized exclude-pattern(s) using "--excludes" flag.

If "--public" flag is set, ptool will add the following open trackers to created .torrent file:
  %s
To create a private torrent (for uploading to Private Trackers site), set "--private" flag.`,
		strings.Join(constants.DefaultIgnorePatterns, " ; "), strings.Join(constants.OpenTrackers, "\n  ")),
	Args: cobra.MatchAll(cobra.ExactArgs(1), cobra.OnlyValidArgs),
	RunE: maketorrent,
}

var (
	all            = false
	private        = false
	public         = false
	force          = false
	pieceLengthStr = ""
	infoName       = ""
	comment        = ""
	output         = ""
	createdBy      = ""
	creationDate   = ""
	trackers       []string
	urlList        []string
	excludes       []string
)

func init() {
	command.Flags().BoolVarP(&all, "all", "a", false, `Index all files of content folder to created torrent. `+
		`If not set, ptool will ignore files with certain patterns names inside content folder: `+
		strings.Join(constants.DefaultIgnorePatterns, " ; "))
	command.Flags().BoolVarP(&private, "private", "p", false, `Mark created torrent as private ("info.private" field)`)
	command.Flags().BoolVarP(&public, "public", "P", false, `Mark created torrent as public (non-private), `+
		`ptool will automatically add common pre-defined open trackers to it`)
	command.Flags().BoolVarP(&force, "force", "", false, "Force overwrite existing output .torrent file in disk")
	command.Flags().StringVarP(&pieceLengthStr, "piece-length", "", "16MiB",
		`Set the piece length ("info"."piece length" field) of created .torrent`)
	command.Flags().StringVarP(&output, "output", "", "", `Set the output .torrent filename. `+
		`Use "-" to output to stdout`)
	command.Flags().StringVarP(&infoName, "info-name", "", "", `Manually set the "info.name" field of created torrent`)
	command.Flags().StringVarP(&comment, "comment", "", "", `Set the "comment" field of created torrent`)
	command.Flags().StringVarP(&createdBy, "created-by", "", "",
		`Manually set the "created by" field of created torrent. To unset this field, set it to "`+constants.NONE+`"`)
	command.Flags().StringVarP(&creationDate, "creation-date", "", "",
		`Set the "creation date" field of torrent. E.g. "2024-01-20 15:00:00" (local timezone), `+
			`or a unix timestamp integer (seconds). Default to now; To unset this field, set it to "`+constants.NONE+`"`)
	command.Flags().StringArrayVarP(&trackers, "tracker", "", nil,
		`Set the trackers ("Announce" & "AnnounceList" field) of created torrent`)
	command.Flags().StringArrayVarP(&excludes, "excludes", "", nil,
		`Specifiy patterns of files that will NOT be included (indexed) to created torrent. `+
			`Use gitignore-style, checked against relative path of the file to the root folder. `+
			`E.g. "*.txt"`)
	command.Flags().StringArrayVarP(&urlList, "url-list", "", nil,
		`Set the "url list" field (BEP 19 WebSeeds) of created torrent`)
	cmd.RootCmd.AddCommand(command)
}

func maketorrent(cmd *cobra.Command, args []string) (err error) {
	if private && public {
		return fmt.Errorf("--private and --public flags are NOT compatible")
	}
	contentPath := args[0]
	mi := &metainfo.MetaInfo{
		AnnounceList: make([][]string, 0),
		Comment:      comment,
		UrlList:      urlList,
	}
	if public {
		trackers = append(trackers, constants.OpenTrackers...)
		util.UniqueSlice(trackers)
	}
	for _, a := range trackers {
		mi.AnnounceList = append(mi.AnnounceList, []string{a})
	}
	if len(trackers) > 0 {
		mi.Announce = trackers[0]
	}
	mi.SetDefaults()
	if createdBy != "" {
		if createdBy == constants.NONE {
			mi.CreatedBy = ""
		} else {
			mi.CreatedBy = createdBy
		}
	}
	if creationDate != "" {
		if creationDate == constants.NONE {
			mi.CreationDate = 0
		} else {
			ts, err := util.ParseTime(creationDate, nil)
			if err != nil {
				return fmt.Errorf("invalid creation-date: %v", err)
			}
			mi.CreationDate = ts
		}
	}
	info := &metainfo.Info{}
	if pieceLength, err := util.RAMInBytes(pieceLengthStr); err != nil {
		return fmt.Errorf("invalid piece-length: %v", err)
	} else {
		info.PieceLength = pieceLength
	}
	if private {
		info.Private = &private
	}
	if !all {
		excludes = append(excludes, constants.DefaultIgnorePatterns...)
	}
	log.Infof("Creating torrent for %q", contentPath)
	if err := infoBuildFromFilePath(info, contentPath, excludes); err != nil {
		return fmt.Errorf("failed to build info from content-path: %v", err)
	}
	if len(info.Files) == 0 {
		return fmt.Errorf("no files found in content-path")
	}
	if infoName != "" {
		info.Name = infoName
	}
	if mi.InfoBytes, err = bencode.Marshal(info); err != nil {
		return fmt.Errorf("failed to marshal info: %v", err)
	}
	if output == "" {
		if info.Name != "" && info.Name != metainfo.NoName {
			output = info.Name + ".torrent"
		} else {
			log.Warnf("The created torrent has NO root folder, use it's info-hash as output file name")
			output = mi.HashInfoBytes().String() + ".torrent"
		}
	}
	log.Infof("Output to %q", output)
	if len(trackers) == 0 {
		log.Warnf(`Warning: the created .torrent file does NOT have any trackers. ` +
			`Use "--tracker" flag to add a tracker; ` +
			`For public (non-private) torrent, use "--public" to add pre-defined open trackers`)
	}
	if output == "-" {
		if term.IsTerminal(int(os.Stdout.Fd())) {
			err = fmt.Errorf(constants.HELP_TIP_TTY_BINARY_OUTPUT)
		} else {
			err = mi.Write(os.Stdout)
		}
	} else {
		if !force && util.FileExists(output) {
			err = fmt.Errorf(`output file %q already exists. use "--force" to overwrite`, output)
		} else {
			var outputFile *os.File
			if outputFile, err = os.OpenFile(output, os.O_TRUNC|os.O_CREATE|os.O_WRONLY, constants.PERM); err == nil {
				defer outputFile.Close()
				err = mi.Write(outputFile)
			}
		}
	}
	if err == nil {
		if tinfo, err := torrentutil.FromMetaInfo(mi, info); err != nil {
			log.Errorf("Failed to parsed created .torrent file contents: %v", err)
		} else {
			fmt.Fprintf(os.Stderr, "\nSuccessfully created torrent file:\n")
			tinfo.Fprint(os.Stderr, output, true)
			tinfo.FprintFiles(os.Stderr, true, false)
		}
	}
	return err
}

// Adapted from metainfo.BuildFromFilePath.
// excludes: gitignore style exclude-file-patterns.
func infoBuildFromFilePath(info *metainfo.Info, root string, excludes []string) (err error) {
	info.Name = func() string {
		b := filepath.Base(root)
		switch b {
		case ".", "..", string(filepath.Separator):
			return metainfo.NoName
		default:
			return b
		}
	}()
	info.Files = nil
	err = filepath.Walk(root, func(path string, fi os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if len(excludes) > 0 {
			if relativePath, err := filepath.Rel(root, path); err == nil && relativePath != "." {
				if ignore, _ := pathspec.GitIgnore(excludes, relativePath); ignore {
					log.Warnf("Ignore %s", relativePath)
					if fi.IsDir() {
						return filepath.SkipDir
					} else {
						return nil
					}
				}
			}
		}
		if fi.IsDir() {
			// Directories are implicit in torrent files.
			return nil
		} else if path == root {
			// The root is a file.
			info.Length = fi.Size()
			return nil
		}
		relPath, err := filepath.Rel(root, path)
		if err != nil {
			return fmt.Errorf("error getting relative path: %s", err)
		}
		info.Files = append(info.Files, metainfo.FileInfo{
			Path:   strings.Split(relPath, string(filepath.Separator)),
			Length: fi.Size(),
		})
		return nil
	})
	if err != nil {
		return
	}
	slices.SortStableFunc(info.Files, func(l, r metainfo.FileInfo) int {
		lp := strings.Join(l.Path, "/")
		rp := strings.Join(r.Path, "/")
		if lp < rp {
			return -1
		} else if lp > rp {
			return 1
		}
		return 0
	})
	if info.PieceLength == 0 {
		info.PieceLength = metainfo.ChoosePieceLength(info.TotalLength())
	}
	err = info.GeneratePieces(func(fi metainfo.FileInfo) (io.ReadCloser, error) {
		return os.Open(filepath.Join(root, strings.Join(fi.Path, string(filepath.Separator))))
	})
	if err != nil {
		err = fmt.Errorf("error generating pieces: %s", err)
	}
	return
}
