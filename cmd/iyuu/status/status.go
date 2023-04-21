package status

import (
	"fmt"

	log "github.com/sirupsen/logrus"

	"github.com/sagan/ptool/cmd/iyuu"
	"github.com/sagan/ptool/config"
	"github.com/spf13/cobra"
)

var command = &cobra.Command{
	Use:   "status",
	Short: "Show iyuu user status",
	Long:  `Show iyuu user status`,
	Run:   status,
}

func init() {
	iyuu.Command.AddCommand(command)
}

func status(cmd *cobra.Command, args []string) {
	log.Print(config.ConfigFile, " ", args)
	log.Print("token", config.Get().IyuuToken)

	data, err := iyuu.IyuuApiGetUser(config.Get().IyuuToken)
	fmt.Printf("Iyuu status: error=%v, user=%v", err, data)
}
