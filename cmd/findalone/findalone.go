package findalone

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"golang.org/x/exp/slices"

	"github.com/sagan/ptool/client"
	"github.com/sagan/ptool/cmd"
	"github.com/sagan/ptool/util"
)

type File struct {
	Path  string
	Count int64
}

var command = &cobra.Command{
	Use:         "findalone {client} {save-path}...",
	Annotations: map[string]string{"cobra-prompt-dynamic-suggestions": "findalone"},
	Short:       "Find alone files (no matched torrent exists in client) in save path(s).",
	Long: `Find alone files (no matched torrent exists in client) in save path(s).
It will read the file list of provided save path(s) in local file system,
find the files that does not belong to any torrent in BitTorrent client.
Only the top-level files of save path(s) will be read, it doesn't scan the dir recursively.

If ptool and the BitTorrent client use different file system (e.g. the client runs in Docker),
then you may want to set the mapper rule of "ptool save path" to "client save path",
which can be done using "--map-save-path-prefix" flag. The flag can be set multiple times.

If --all flag is set, it will list all files in save pathes instead of only "alone" files,
and display each file's count of belonged torrents in client.

It prints found "alone" files or dirs to stdout.`,
	Args: cobra.MatchAll(cobra.MinimumNArgs(2), cobra.OnlyValidArgs),
	RunE: findalone,
}

var (
	showAll            = false
	originalOrder      = false
	mapSavePathPrefixs []string
)

func init() {
	command.Flags().BoolVarP(&showAll, "all", "a", false,
		"Show the list of all files in save pathes with the count of each file's belonged torrents in client")
	command.Flags().BoolVarP(&originalOrder, "original-order", "", false,
		`Used with "--all". Display the list in original (filename asc) order instead of count desc order`)
	command.Flags().StringArrayVarP(&mapSavePathPrefixs, "map-save-path-prefix", "", nil,
		`Map save path that ptool sees to the one that the BitTorrent client sees. `+
			`Format: "original_save_path|client_save_path". E.g. `+
			`"/root/Downloads:/var/Downloads" will map "/root/Downloads" or "/root/Downloads/..." save path to `+
			`"/var/Downloads" or "/var/Downloads/..."`)
	cmd.RootCmd.AddCommand(command)
}

func findalone(cmd *cobra.Command, args []string) error {
	if !showAll && originalOrder {
		return fmt.Errorf("--original-order must be used with --all flag")
	}
	clientName := args[0]
	savePathes := util.Map(args[1:], func(p string) string {
		return path.Clean(filepath.ToSlash(p))
	})
	clientInstance, err := client.CreateClient(clientName)
	if err != nil {
		return fmt.Errorf("failed to create client: %v", err)
	}
	savePathMapper := map[string]string{}
	for _, mapSavePathPrefix := range mapSavePathPrefixs {
		before, after, found := strings.Cut(mapSavePathPrefix, ":")
		if !found || before == "" || after == "" {
			return fmt.Errorf("invalid map-save-path-prefix %q", mapSavePathPrefix)
		}
		before = path.Clean(filepath.ToSlash(before))
		after = path.Clean(filepath.ToSlash(after))
		savePathMapper[before] = after
	}

	contentRootFiles := map[string]int64{}
	torrents, err := clientInstance.GetTorrents("", "", true)
	if err != nil {
		return fmt.Errorf("failed to get client torrents: %v", err)
	}
	for _, torrent := range torrents {
		contentPath := filepath.ToSlash(torrent.ContentPath)
		for before, after := range savePathMapper {
			if strings.HasPrefix(contentPath, after+"/") {
				contentPath = before + strings.TrimPrefix(contentPath, after)
				break
			}
		}
		contentRootFiles[contentPath]++
	}

	var files []File
	errorCnt := int64(0)
	for _, savePath := range savePathes {
		entries, err := os.ReadDir(savePath)
		if err != nil {
			log.Errorf("Failed to read save-path %s: %v", savePath, err)
			errorCnt++
			continue
		}
		for _, entry := range entries {
			fullpath := path.Join(savePath, entry.Name())
			if slices.Contains(savePathes, fullpath) {
				continue
			}
			if showAll {
				files = append(files, File{filepath.Clean(fullpath), contentRootFiles[fullpath]})
			} else if contentRootFiles[fullpath] == 0 {
				fmt.Printf("%s\n", filepath.Clean(fullpath)) // output in host sep
			}
		}
	}
	if showAll {
		if !originalOrder {
			slices.SortStableFunc(files, func(a File, b File) int { return int(b.Count - a.Count) })
		}
		for _, file := range files {
			fmt.Printf("%-3d  %s\n", file.Count, file.Path)
		}
	}
	if errorCnt > 0 {
		return fmt.Errorf("%d errors", errorCnt)
	}
	return nil
}
