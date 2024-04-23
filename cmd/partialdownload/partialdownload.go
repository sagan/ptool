package partialdownload

import (
	"fmt"
	"sort"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/sagan/ptool/client"
	"github.com/sagan/ptool/cmd"
	"github.com/sagan/ptool/util"
)

type Chunk struct {
	Index    int64
	FilesCnt int64
	Size     int64
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

Without --strict flag, ptool will always split torrent contents to chunks.
The size of each chunk may be larger then chunk size. And there may be less
chunks than expected.

With --strict flag, ptool will ensure that the size of every chunk is strictly
less or equal than (<=) chunk size. There may be more chunks than expected. If
there is a single large file in torrent contents which is larger than (>) chunk
size, the command will fail.

With --append flag, ptool will only mark files of current (index) chunk as download,
but will NOT mark files of other chunks as no-download (Leave their download / no-download
marks unchanged).

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
	showAll       = false
	appendMode    = false
	strict        = false
	originalOrder = false
)

func init() {
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
	command.MarkFlagRequired("chunk-size")
	cmd.RootCmd.AddCommand(command)
}

func partialdownload(cmd *cobra.Command, args []string) error {
	chunkSize, _ := util.RAMInBytes(chunkSizeStr)
	clientName := args[0]
	infoHash := args[1]
	if chunkSize <= 0 {
		return fmt.Errorf("invalid chunk size %d", chunkSize)
	}

	clientInstance, err := client.CreateClient(clientName)
	if err != nil {
		return fmt.Errorf("failed to create client: %v", err)
	}
	torrentFiles, err := clientInstance.GetTorrentContents(infoHash)
	if err != nil {
		return fmt.Errorf("failed to get client files: %v", err)
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
	// scan all files in order and download a (index) sequential files
	// a chunk contains at least 1 file. Chunk ends when all it's files size >= chunk size
	chunks := []*Chunk{}
	currentChunkIndex := int64(0)
	currentChunkSize := int64(0)
	currentChunkFilesCnt := int64(0)
	downloadFileIndexes := []int64{}
	noDownloadFileIndexes := []int64{}
	allSize := int64(0)
	skippedSize := int64(0)
	// For negative chunk-index, scan once to get total chunks count
	if chunkIndex < 0 {
		for i, file := range torrentFiles {
			if int64(i) < startIndex {
				continue
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
	for i, file := range torrentFiles {
		if int64(i) < startIndex {
			skippedSize += file.Size
			noDownloadFileIndexes = append(noDownloadFileIndexes, file.Index)
			continue
		}
		allSize += file.Size
		if strict && file.Size > chunkSize {
			return fmt.Errorf("torrent can NOT be strictly splitted to %s chunks: file %s is too large (%s)",
				util.BytesSize(float64(chunkSize)), file.Path, util.BytesSize(float64(file.Size)))
		}
		if currentChunkSize >= chunkSize || (strict && (currentChunkSize+file.Size) > chunkSize) {
			chunks = append(chunks, &Chunk{currentChunkIndex, currentChunkFilesCnt, currentChunkSize})
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
	chunks = append(chunks, &Chunk{currentChunkIndex, currentChunkFilesCnt, currentChunkSize}) // last chunk
	if showAll {
		fmt.Printf("Torrent Size: %s (%d) / Chunk Size: %s; Skipped files: %d; All %d Chunks:\n",
			util.BytesSize(float64(allSize)), len(torrentFiles),
			util.BytesSize(float64(chunkSize)), startIndex, len(chunks))
		fmt.Printf("%-10s  %-15s  %-5s  %s\n", "ChunkIndex", "FileStartIndex", "Files", "Size")
		fileStartIndex := int64(0)
		if startIndex > 0 {
			fmt.Printf("%-10s  %-15d  %-5d  %s\n",
				"<skip>", fileStartIndex, startIndex, util.BytesSize(float64(skippedSize)))
			fileStartIndex += startIndex
		}
		for _, chunk := range chunks {
			fmt.Printf("%-10d  %-15d  %-5d  %s\n",
				chunk.Index, fileStartIndex, chunk.FilesCnt, util.BytesSize(float64(chunk.Size)))
			fileStartIndex += chunk.FilesCnt
		}
		return nil
	}
	if chunkIndex >= int64(len(chunks)) {
		return fmt.Errorf("invalid chunkIndex %d. Torrent has %d chunks", chunkIndex, len(chunks))
	}
	chunk := chunks[chunkIndex]
	err = clientInstance.SetFilePriority(infoHash, downloadFileIndexes, 1)
	if err != nil {
		return fmt.Errorf("failed to mark files as download: %v", err)
	}
	log.Infof("Marked %d files as download.", len(downloadFileIndexes))
	if !appendMode {
		util.Sleep(5)
		err = clientInstance.SetFilePriority(infoHash, noDownloadFileIndexes, 0)
		if err != nil {
			return fmt.Errorf("failed to mark files as no-download: %v", err)
		}
		log.Infof("Marked %d files as no-download.", len(noDownloadFileIndexes))
	}
	fmt.Printf("Torrent Size: %s (%d) / Chunks: %d; Skipped files: %d; "+
		"DownloadChunkIndex: %d; DownloadChunkSize: %s (%d)",
		util.BytesSize(float64(allSize)), len(torrentFiles), len(chunks), startIndex,
		chunkIndex, util.BytesSize(float64(chunk.Size)), chunk.FilesCnt,
	)
	return nil
}
