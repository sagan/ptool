package reseed

import (
	log "github.com/sirupsen/logrus"

	"github.com/sagan/ptool/cmd"
	"github.com/sagan/ptool/config"
	"github.com/spf13/cobra"
)

var command = &cobra.Command{
	Use:   "reseed source",
	Short: "Reseed use iyuu API",
	Long:  `A longer description`,
	Run:   reseed,
}

func init() {
	cmd.RootCmd.AddCommand(command)
}

func reseed(cmd *cobra.Command, args []string) {
	log.Print(config.ConfigFile, " ", args)
	log.Print("token", config.Get().IyuuToken)
}
