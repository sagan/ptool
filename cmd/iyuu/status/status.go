package status

import (
	"fmt"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/sagan/ptool/cmd/iyuu"
	"github.com/sagan/ptool/config"
)

var command = &cobra.Command{
	Use:   "status",
	Short: "Show iyuu user status.",
	Long:  `Show iyuu user status.`,
	RunE:  status,
}

func init() {
	iyuu.Command.AddCommand(command)
}

func status(cmd *cobra.Command, args []string) error {
	log.Tracef("iyuu token: %s", config.Get().IyuuToken)
	if config.Get().IyuuToken == "" {
		return fmt.Errorf("you must config iyuuToken in ptool.toml to use iyuu functions")
	}

	data, err := iyuu.IyuuApiGetUser(config.Get().IyuuToken)
	fmt.Printf("Iyuu status: error=%v, user=%v", err, data)
	return nil
}
