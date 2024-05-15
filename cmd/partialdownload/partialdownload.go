package partialdownload

import (
	"fmt"
	"io"
	"os"
	"sort"

	"github.com/shibumi/go-pathspec"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/sagan/ptool/client"
	"github.com/sagan/ptool/cmd"
	"github.com/sagan/ptool/constants"
	"github.com/sagan/ptool/util"
)

type Chunk struct {
	Index int64
	Files int64
	Size  int64
}

type Summary struct {
	InfoHash           string
	ChunkSize          int64
	TotalSize          int64
	TotalFiles         int64
	SkippedSize        int64
	SkippedFiles       int64
	DownloadChunkIndex int64
	Chunks             []*Chunk
}

var command = &cobra.Command{
	Use:         "partialdownload {client} {infoHash} --chunk-size {size_str} {-a | --chunk-index index}",
	Aliases:     []string{"partialdl"},
	Annotations: map[string]string{"cobra-prompt-dynamic-suggestions": "partialdownload"},
	Short:       "Partially download a (large) torrent in client.",
	Long: `Partially download a (large) torrent in client.
Before running this command, you should add the target torrent to client in paused
state. You need to manually start the torrent task after running this command.

Examples:
  # View chunks info of the torrent
  ptool partialdownload local <info-hash> --chunk-size 500GiB -a

  # Download the first (0-indexed) chunk of the torrent in client (Mark files of other chunks as no-download)
  ptool partialdownload local <info-hash> --chunk-size 500GiB --chunk-index 0

  # Download the last (-1 index) chunk of the torrent
  ptool partialdownload local <info-hash> --chunk-size 500GiB --chunk-index -1

  # Exclude certain files of torrent from being downloaded
  ptool partialdownload <client> <infohash> --exclude "*.txt"

Without --strict flag, ptool will always split torrent contents to chunks.
The size of each chunk may be larger then chunk size. And there may be less
chunks than expected.

With --strict flag, ptool will ensure that the size of every chunk is strictly
less or equal than (<=) chunk size. There may be more chunks than expected. If
there is a single large file in torrent contents which is larger than (>) chunk
size, the command will fail.

Additional, it's possible to explicitly skip (ignore) certain files in torrent.
Skipped files will be excluded from being splitted to chunks and will also be marked as no-download.
To skip files, use any one (or more) of the following flags:
* --start-index index : The first file index of the first chunk. Skip prior files in torrent.
* --exclude pattern : Pattern of .gitignore style. Skip files which filename match any provided pattern.
* --include pattern : Pattern of .gitignore style. Skip all files which filename does NOT match any provided pattern.
If any of the above flags is set, the --chunk-size flag can be omitted, in which case
it's assumed to be 1EiB (effectively infinite), so all non-skipped files will be mark as download.

With --append flag, ptool will only mark files of current (index) chunk as download,
but will NOT mark other files as no-download (Leave their download / no-download marks unchanged).

Use case of this command: You have a cloud VPS / Server with limited disk space, and you want to use this
machine to download a large torrent. And then upload the downloaded torrent contents
to cloud drive using rclone, for example. The above task is trivial using this command.`,
	Args: cobra.MatchAll(cobra.ExactArgs(2), cobra.OnlyValidArgs),
	RunE: partialdownload,
}

var (
	chunkSizeStr  = ""
	chunkIndex    = int64(0)
	startIndex    = int64(0)
	showJson      = false
	showAll       = false
	appendMode    = false
	strict        = false
	originalOrder = false
	includes      []string
	excludes      []string
)

func init() {
	command.Flags().BoolVarP(&showJson, "json", "", false, "Show output in json format")
	command.Flags().BoolVarP(&showAll, "all", "a", false, "Show full chunks info and exit")
	command.Flags().BoolVarP(&appendMode, "append", "", false,
		"Append mode. Mark files of current chunk as download but do NOT mark files of other chunks as no-download")
	command.Flags().BoolVarP(&strict, "strict", "", false,
		"Set strict mode that the size of every chunk MUST be strictly <= chunk-size")
	command.Flags().BoolVarP(&originalOrder, "original-order", "", false,
		"Split torrent files to chunks by their original order instead of path order")
	command.Flags().Int64VarP(&chunkIndex, "chunk-index", "", 0, "Set the split chunk index (0-based) to download. "+
		"Negative value is related to the total chunks number, e.g. -1 means the last chunk. "+
		"Default value is 0 (the first chunk)")
	command.Flags().Int64VarP(&startIndex, "start-index", "", 0,
		"Set the index (0-based) of the first file in torrent to download. The prior files of torrent will be skipped. "+
			"Negative value is related to the total files number, e.g. -100 means skip all but the last 100 files. "+
			"Skipped files will be be excluded from being splitting into chunks")
	command.Flags().StringVarP(&chunkSizeStr, "chunk-size", "", "", "Set the split chunk size string. e.g. 500GiB")
	command.Flags().StringArrayVarP(&includes, "include", "", nil,
		`Specifiy patterns of files that only these files will be included. All other files will be skipped. `+
			`Use gitignore-style, checked against the file path in torrent. E.g. "*.txt". `+
			"Skipped files will be be excluded from being splitting into chunks")
	command.Flags().StringArrayVarP(&excludes, "exclude", "", nil,
		`Specifiy patterns of files that will be skipped. `+
			`Use gitignore-style, checked against the file path in torrent. E.g. "*.txt". `+
			"Skipped files will be be excluded from being splitting into chunks")
	cmd.RootCmd.AddCommand(command)
}

func (summary *Summary) printCommon(output io.Writer) {
	fmt.Fprintf(output, "Total files: %s (%d) / Chunks: %d; ChunkSize: %s;  Skipped files: %s (%d)\n",
		util.BytesSize(float64(summary.TotalSize)), summary.TotalFiles, len(summary.Chunks),
		util.BytesSize(float64(summary.ChunkSize)), util.BytesSize(float64(summary.SkippedSize)), summary.SkippedFiles,
	)
}

func (summary *Summary) PrintAll(output io.Writer) {
	summary.printCommon(output)
	fmt.Fprintf(output, "%-10s  %-5s  %s\n", "ChunkIndex", "Files", "Size")
	if summary.SkippedFiles > 0 {
		fmt.Fprintf(output, "%-10s  %-5d  %s\n",
			"<skip>", summary.SkippedFiles, util.BytesSize(float64(summary.SkippedSize)))
	}
	for _, chunk := range summary.Chunks {
		fmt.Fprintf(output, "%-10d  %-5d  %s\n",
			chunk.Index, chunk.Files, util.BytesSize(float64(chunk.Size)))
	}
}

func (summary *Summary) PrintSelf(output io.Writer) {
	summary.printCommon(output)
	fmt.Fprintf(output, "DownloadChunkIndex: %d; DownloadChunkSize: %s (%d)\n", summary.DownloadChunkIndex,
		util.BytesSize(float64(summary.Chunks[summary.DownloadChunkIndex].Size)),
		summary.Chunks[summary.DownloadChunkIndex].Files,
	)
}

func NewSummary(infoHash string, chunkSize int64) *Summary {
	return &Summary{
		InfoHash:           infoHash,
		ChunkSize:          chunkSize,
		DownloadChunkIndex: -1,
	}
}

func partialdownload(cmd *cobra.Command, args []string) (err error) {
	var chunkSize int64
	if chunkSizeStr != "" {
		if chunkSize, err = util.RAMInBytes(chunkSizeStr); err != nil {
			return fmt.Errorf("invalid chunk-size: %w", err)
		}
	}
	if chunkSize <= 0 {
		if len(includes) > 0 || len(excludes) > 0 || startIndex != 0 {
			chunkSize = constants.INFINITE_SIZE
		} else {
			return fmt.Errorf("either --chunk-size, or any of --start-index, --include, --exclude flags must be set")
		}
	}
	if chunkSize <= 0 {
		return fmt.Errorf("invalid chunk size %d", chunkSize)
	}
	clientName := args[0]
	infoHash := args[1]

	clientInstance, err := client.CreateClient(clientName)
	if err != nil {
		return fmt.Errorf("failed to create client: %w", err)
	}
	torrentFiles, err := clientInstance.GetTorrentContents(infoHash)
	if err != nil {
		return fmt.Errorf("failed to get client files: %w", err)
	}
	if len(torrentFiles) == 0 {
		return fmt.Errorf("target torrent has no files")
	}
	if startIndex < 0 && int64(len(torrentFiles))+startIndex < 0 || startIndex >= int64(len(torrentFiles)) {
		return fmt.Errorf("invalid start-index %d, torrent has %d files", startIndex, len(torrentFiles))
	}
	if startIndex < 0 {
		startIndex = int64(len(torrentFiles)) + startIndex
	}
	if !originalOrder {
		// while not necessory to use stable sort, we want absolutely consistent results
		sort.SliceStable(torrentFiles, func(i, j int) bool {
			return torrentFiles[i].Path < torrentFiles[j].Path
		})
	}
	summary := NewSummary(infoHash, chunkSize)
	currentChunkIndex := int64(0)
	currentChunkSize := int64(0)
	currentChunkFilesCnt := int64(0)
	downloadFileIndexes := []int64{}
	noDownloadFileIndexes := []int64{}
	// For negative chunk-index, scan once to get total chunks count
	if chunkIndex < 0 {
		for i, file := range torrentFiles {
			if int64(i) < startIndex {
				continue
			}
			if len(includes) > 0 {
				if ignore, err := pathspec.GitIgnore(includes, file.Path); err != nil {
					return fmt.Errorf("invalid includes: %w", err)
				} else if !ignore {
					continue
				}
			}
			if len(excludes) > 0 {
				if ignore, err := pathspec.GitIgnore(excludes, file.Path); err != nil {
					return fmt.Errorf("invalid excludes: %w", err)
				} else if ignore {
					continue
				}
			}
			if strict && file.Size > chunkSize {
				return fmt.Errorf("torrent can NOT be strictly splitted to %s chunks: file %s is too large (%s)",
					util.BytesSize(float64(chunkSize)), file.Path, util.BytesSize(float64(file.Size)))
			}
			if currentChunkSize >= chunkSize || (strict && (currentChunkSize+file.Size) > chunkSize) {
				currentChunkIndex++
				currentChunkSize = 0
			}
			currentChunkSize += file.Size
		}
		actualChunkIndex := currentChunkIndex + 1 + chunkIndex
		if actualChunkIndex < 0 {
			return fmt.Errorf("invalid chunkIndex %d. Torrent has %d chunks", chunkIndex, currentChunkIndex+1)
		}
		chunkIndex = actualChunkIndex
		currentChunkIndex = 0
		currentChunkSize = 0
	}
	// scan all files in order and download a (index) sequential files
	// a chunk contains at least 1 file. Chunk ends when all it's files size >= chunk size
	for i, file := range torrentFiles {
		skip := false
		if int64(i) < startIndex {
			skip = true
		}
		if !skip && len(includes) > 0 {
			if ignore, err := pathspec.GitIgnore(includes, file.Path); err != nil {
				return fmt.Errorf("invalid includes: %w", err)
			} else if !ignore {
				log.Debugf("Skip non-includes file %q", file.Path)
				skip = true
			}
		}
		if !skip && len(excludes) > 0 {
			if ignore, err := pathspec.GitIgnore(excludes, file.Path); err != nil {
				return fmt.Errorf("invalid excludes: %w", err)
			} else if ignore {
				log.Debugf("Skip excludes file %q", file.Path)
				skip = true
			}
		}
		if skip {
			summary.SkippedFiles++
			summary.SkippedSize += file.Size
			noDownloadFileIndexes = append(noDownloadFileIndexes, file.Index)
			continue
		}
		summary.TotalFiles++
		summary.TotalSize += file.Size
		if strict && file.Size > chunkSize {
			return fmt.Errorf("torrent can NOT be strictly splitted to %s chunks: file %s is too large (%s)",
				util.BytesSize(float64(chunkSize)), file.Path, util.BytesSize(float64(file.Size)))
		}
		if currentChunkSize >= chunkSize || (strict && (currentChunkSize+file.Size) > chunkSize) {
			summary.Chunks = append(summary.Chunks, &Chunk{currentChunkIndex, currentChunkFilesCnt, currentChunkSize})
			currentChunkIndex++
			currentChunkSize = 0
			currentChunkFilesCnt = 0
		}
		currentChunkSize += file.Size
		currentChunkFilesCnt++
		if currentChunkIndex == chunkIndex {
			downloadFileIndexes = append(downloadFileIndexes, file.Index)
		} else {
			noDownloadFileIndexes = append(noDownloadFileIndexes, file.Index)
		}
	}
	// last chunk
	summary.Chunks = append(summary.Chunks, &Chunk{currentChunkIndex, currentChunkFilesCnt, currentChunkSize})
	if showAll {
		if showJson {
			return util.PrintJson(os.Stdout, summary)
		}
		summary.PrintAll(os.Stdout)
		return nil
	}
	if chunkIndex >= int64(len(summary.Chunks)) {
		return fmt.Errorf("invalid chunkIndex %d. Torrent has %d chunks", chunkIndex, len(summary.Chunks))
	}
	summary.DownloadChunkIndex = chunkIndex
	// mark file as download
	if len(downloadFileIndexes) > 0 {
		err = clientInstance.SetFilePriority(infoHash, downloadFileIndexes, 1)
		if err != nil {
			return fmt.Errorf("failed to mark files as download: %w", err)
		}
		log.Infof("Marked %d files as download.", len(downloadFileIndexes))
	} else {
		log.Infof("No files are marked as download")
	}
	// mark file as non-download
	if !appendMode {
		if len(noDownloadFileIndexes) > 0 {
			err = clientInstance.SetFilePriority(infoHash, noDownloadFileIndexes, 0)
			if err != nil {
				return fmt.Errorf("failed to mark files as no-download: %w", err)
			}
			log.Infof("Marked %d files as no-download.", len(noDownloadFileIndexes))
		} else {
			log.Infof("No files are marked as non-download")
		}
	}
	if showJson {
		return util.PrintJson(os.Stdout, summary)
	}
	summary.PrintSelf(os.Stdout)
	return nil
}
