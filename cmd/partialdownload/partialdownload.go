package partialdownload

import (
	"fmt"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/sagan/ptool/client"
	"github.com/sagan/ptool/cmd"
	"github.com/sagan/ptool/utils"
)

var command = &cobra.Command{
	Use:   "partialdownload <client> <infoHash>",
	Short: "Partial download a (large) torrent in client.",
	Long:  `Partial download a (large) torrent in client.`,
	Args:  cobra.MatchAll(cobra.ExactArgs(2), cobra.OnlyValidArgs),
	Run:   partialdownload,
}

var (
	chunkSizeStr = ""
	chunkIndex   = int64(0)
	showAll      = false
)

func init() {
	command.Flags().BoolVarP(&showAll, "all", "a", false, "show full comparison result")
	command.Flags().Int64VarP(&chunkIndex, "chunk-index", "", 0, "Set The split chunk index (0-indexed) to download")
	command.Flags().StringVarP(&chunkSizeStr, "chunk-size", "", "", "Set the split chunk size string. eg. 500GiB")
	command.MarkFlagRequired("chunk-size")
	cmd.RootCmd.AddCommand(command)
}

func partialdownload(cmd *cobra.Command, args []string) {
	chunkSize, _ := utils.RAMInBytes(chunkSizeStr)
	clientName := args[0]
	infoHash := args[1]
	if chunkSize <= 0 {
		log.Fatalf("Invalid chunk size %d", chunkSize)
	}
	if chunkIndex < 0 {
		log.Fatalf("Invalid chunk index %d", chunkIndex)
	}

	clientInstance, err := client.CreateClient(clientName)
	if err != nil {
		clientInstance.Close()
		log.Fatalf("Failed to create client: %v", err)
	}
	torrentFiles, err := clientInstance.GetTorrentContents(infoHash)
	if err != nil {
		clientInstance.Close()
		log.Fatalf("Failed to get client files: %v", err)
	}

	// scan all files in index order and download a (index) sequential files
	// a chunk contains at least 1 file. Chunk ends when all it's files size >= chunk size
	currentChunkIndex := int64(0)
	currentChunkSize := int64(0)
	chunkActualSize := int64(0)
	downloadFileIndexes := []int64{}
	noDownloadFileIndexes := []int64{}

	allSize := int64(0)
	for _, file := range torrentFiles {
		allSize += file.Size
		if currentChunkSize >= chunkSize {
			if currentChunkIndex == chunkIndex {
				chunkActualSize = currentChunkSize
			}
			currentChunkIndex++
			currentChunkSize = 0
		}
		currentChunkSize += file.Size
		if currentChunkIndex == chunkIndex {
			downloadFileIndexes = append(downloadFileIndexes, file.Index)
		} else {
			noDownloadFileIndexes = append(noDownloadFileIndexes, file.Index)
		}
	}
	if currentChunkIndex < chunkIndex {
		clientInstance.Close()
		log.Fatalf("Invalid chunkIndex %d. Torrent has %d chunks", chunkIndex, currentChunkIndex+1)
	} else if currentChunkIndex == chunkIndex { // last chunk
		chunkActualSize = currentChunkSize
	}

	err = clientInstance.SetFilePriority(infoHash, downloadFileIndexes, 7)
	if err != nil {
		clientInstance.Close()
		log.Fatalf("Failed to set download files: %v", err)
	}
	err = clientInstance.SetFilePriority(infoHash, noDownloadFileIndexes, 0)
	if err != nil {
		clientInstance.Close()
		log.Fatalf("Failed to set no download files: %v", err)
	}
	fmt.Printf("Torrent Size: %s / Chunks: %d; DownloadChunkIndex: %d; DownloadChunkSize: %s",
		utils.BytesSize(float64(allSize)), currentChunkIndex+1,
		chunkIndex, utils.BytesSize(float64(chunkActualSize)),
	)
	clientInstance.Close()
}
