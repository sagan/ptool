package hardlinkcp

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/KarpelesLab/reflink"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/sagan/ptool/cmd/hardlink"
	"github.com/sagan/ptool/constants"
	"github.com/sagan/ptool/util"
)

var command = &cobra.Command{
	Use:         "cp {source} {dest}",
	Annotations: map[string]string{"cobra-prompt-dynamic-suggestions": "hardlinkcp"},
	Short:       "Create hardlinked duplicate of source folder or file.",
	Long: `Create hardlinked duplicate of source folder or file.
Similar to what "cp -rl SOURCE DEST" in Linux does. It works in every platform.

For small file (defined by --hardlink-min-size), it will create a copy instead of a hardlink.`,
	Args: cobra.MatchAll(cobra.ExactArgs(2), cobra.OnlyValidArgs),
	RunE: hardlinkcp,
}

var (
	setReadonly  = false
	useReflink   = false
	sizeLimitStr = ""
)

func init() {
	command.Flags().BoolVarP(&setReadonly, "set-readonly", "", false, `Set created hardlinks to read-only. `+
		`It doesn't get applied if copy or reflink is used instead of hardlink`)
	// @todo : support ReFS (Windows 11 "Dev Drive" feature) reflink on Windows.
	// See https://github.com/0xbadfca11/reflink .
	command.Flags().BoolVarP(&useReflink, "use-reflink", "", false, constants.HELP_ARG_USE_REF_LINK)
	command.Flags().StringVarP(&sizeLimitStr, "hardlink-min-size", "", "1MiB",
		"File with size smaller than (<) this value will be copied instead of hardlinked. -1 == always hardlink")
	hardlink.Command.AddCommand(command)
}

func hardlinkcp(cmd *cobra.Command, args []string) error {
	source := args[0]
	dest := args[1]
	sizeLimit, _ := util.RAMInBytes(sizeLimitStr)

	sourceIsDir := false
	destIsDir := false
	if source == "." || source == ".." || strings.HasSuffix(source, "/") || strings.HasSuffix(source, `\`) {
		sourceIsDir = true
	}
	if dest == "." || dest == ".." || strings.HasSuffix(dest, "/") || strings.HasSuffix(dest, `\`) {
		destIsDir = true
	}
	source = filepath.Clean(source)
	dest = filepath.Clean(dest)
	sourceStat, err := os.Stat(source)
	if err != nil {
		return fmt.Errorf("failed to access source %q: %w", source, err)
	}
	if sourceStat.IsDir() {
		sourceIsDir = true
	} else if sourceIsDir {
		return fmt.Errorf(`source specified as a dir (has a "/" or "\" prefix) but actually is NOT`)
	} else if !sourceStat.Mode().IsRegular() {
		return fmt.Errorf("source %q is NOT a dir or regular file", source)
	}
	if _, err := os.Stat(dest); err == nil {
		return fmt.Errorf("dest %q already exists", dest)
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("dest %q can NOT be accessed: %w", dest, err)
	}
	if !sourceIsDir && destIsDir {
		return fmt.Errorf(`dest specified as a dir (has a "/" or "\" prefix) but source is NOT a dir`)
	}

	if !sourceIsDir {
		if useReflink {
			return reflink.Always(source, dest)
		} else if sizeLimit >= 0 && sourceStat.Size() < sizeLimit {
			return util.CopyFile(source, dest)
		}
		err = os.Link(source, dest)
		if setReadonly {
			if err := os.Chmod(dest, constants.PERM_RO); err != nil {
				log.Warnf("Failed to set read-only on %q: %v", dest, err)
			}
		}
		return err
	}

	return util.LinkDir(source, dest, sizeLimit, useReflink, setReadonly)
}
