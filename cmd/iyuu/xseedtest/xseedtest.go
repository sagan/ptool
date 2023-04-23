package xseedtest

import (
	log "github.com/sirupsen/logrus"

	"github.com/sagan/ptool/cmd/iyuu"
	"github.com/sagan/ptool/config"
	"github.com/spf13/cobra"
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
	log.Print(config.ConfigFile, " ", args)
	log.Print("token", config.Get().IyuuToken)

	if infoHash != "" {
		iyuu.IyuuApiHash(config.Get().IyuuToken, []string{infoHash})
	}
}
