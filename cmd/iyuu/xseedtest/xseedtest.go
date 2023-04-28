package xseedtest

import (
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/sagan/ptool/cmd/iyuu"
	"github.com/sagan/ptool/config"
)

var command = &cobra.Command{
	Use:   "xseedtest",
	Short: "Cross seed",
	Long:  `Cross seed`,
	Run:   xseed,
}

var (
	infoHash = ""
)

func init() {
	command.Flags().StringVarP(&infoHash, "info-hash", "i", "", "Torrent info hash")
	iyuu.Command.AddCommand(command)
}

func xseed(cmd *cobra.Command, args []string) {
	log.Tracef("iyuu token: %s", config.Get().IyuuToken)
	if config.Get().IyuuToken == "" {
		log.Fatalf("You must config iyuuToken in ptool.yaml to use iyuu functions")
	}

	if infoHash != "" {
		iyuu.IyuuApiHash(config.Get().IyuuToken, []string{infoHash})
	}
}
