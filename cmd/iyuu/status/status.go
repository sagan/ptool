package status

import (
	"fmt"
	"os"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/sagan/ptool/cmd/iyuu"
	"github.com/sagan/ptool/config"
	"github.com/sagan/ptool/util"
)

var command = &cobra.Command{
	Use:         "status",
	Annotations: map[string]string{"cobra-prompt-dynamic-suggestions": "iyuu.status"},
	Short:       "Show iyuu user status.",
	Long: `Show iyuu user status.

The output is returned by iyuu server and printed as opaque data, ptool does not interpret it.
The command always exits with 0 as long as iyuu returns a http 200 response.`,
	RunE: status,
}

func init() {
	iyuu.Command.AddCommand(command)
}

func status(cmd *cobra.Command, args []string) error {
	log.Tracef("iyuu token: %s", config.Get().IyuuToken)
	if config.Get().IyuuToken == "" {
		return fmt.Errorf("you must config iyuuToken in ptool.toml to use iyuu functions")
	}

	data, err := iyuu.IyuuApiUsersProfile(config.Get().IyuuToken)
	fmt.Printf("Iyuu status: error=%v, data=", err)
	util.PrintJson(os.Stdout, data)
	return err
}
