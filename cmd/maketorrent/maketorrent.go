// Adopted from https://github.com/anacrolix/torrent/blob/master/cmd/torrent/create.go .
package maketorrent

import (
	"fmt"
	"os"
	"strings"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/sagan/ptool/cmd"
	"github.com/sagan/ptool/constants"
	"github.com/sagan/ptool/util/torrentutil"
)

var command = &cobra.Command{
	Use:         "maketorrent {content-path}",
	Aliases:     []string{"createtorrent", "mktorrent", "make"},
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
  # --exclude : Prevent *.txt files from being indexed in created torrent
  ptool maketorrent ./MyVideos --public --exclude "*.txt"

By default, certain patterns files inside content-path will be ignored and NOT indexed in created .torrent file:
  %s
If "--all" flag is set, the default exclude-patterns will be disabled and ALL files will in indexed.
It's possible to provide your own customized exclude-pattern(s) using "--exclude" flag.

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
	command.Flags().StringVarP(&pieceLengthStr, "piece-length", "", constants.TORRENT_DEFAULT_PIECE_LENGTH,
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
	command.Flags().StringArrayVarP(&excludes, "exclude", "", nil,
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
	optoins := &torrentutil.TorrentMakeOptions{
		ContentPath:    contentPath,
		Output:         output,
		Public:         public,
		Private:        private,
		All:            all,
		Force:          force,
		PieceLengthStr: pieceLengthStr,
		Comment:        comment,
		InfoName:       infoName,
		UrlList:        urlList,
		Trackers:       trackers,
		Excludes:       excludes,
		CreatedBy:      createdBy,
		CreationDate:   creationDate,
	}
	if len(optoins.Trackers) == 0 && !optoins.Public {
		log.Warnf(`Warning: the created .torrent file will NOT have any trackers. ` +
			`Use "--tracker" flag to add a tracker; ` +
			`For public (non-private) torrent, use "--public" to add pre-defined open trackers`)
	}
	tinfo, err := torrentutil.MakeTorrent(optoins)
	if err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "\nSuccessfully created torrent file:\n")
	tinfo.Fprint(os.Stderr, output, true)
	tinfo.FprintFiles(os.Stderr, true, false)
	return nil
}
