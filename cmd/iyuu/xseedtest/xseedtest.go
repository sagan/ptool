package xseedtest

import (
	"fmt"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/sagan/ptool/cmd/iyuu"
	"github.com/sagan/ptool/config"
)

var command = &cobra.Command{
	Use:         "xseedtest",
	Annotations: map[string](string){"cobra-prompt-dynamic-suggestions": "iyuu.xseedtest"},
	Short:       "Cross seed test.",
	Long:        `Cross seed test.`,
	RunE:        xseed,
}

var (
	infoHash = ""
)

func init() {
	command.Flags().StringVarP(&infoHash, "info-hash", "", "", "Torrent info hash")
	iyuu.Command.AddCommand(command)
}

func xseed(cmd *cobra.Command, args []string) error {
	log.Tracef("iyuu token: %s", config.Get().IyuuToken)
	if config.Get().IyuuToken == "" {
		return fmt.Errorf("you must config iyuuToken in ptool.toml to use iyuu functions")
	}

	if infoHash != "" {
		iyuu.IyuuApiHash(config.Get().IyuuToken, []string{infoHash})
	}
	return nil
}
