package bind

import (
	"fmt"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/sagan/ptool/cmd/iyuu"
	"github.com/sagan/ptool/config"
)

var command = &cobra.Command{
	Use:   "bind",
	Short: "Bind (authenticate) iyuu service using PT site passkey.",
	Long:  `Bind (authenticate) iyuu service using PT site passkey.`,
	Run:   bind,
}

var (
	site    = ""
	uid     = int64(0)
	passkey = ""
)

func init() {
	command.Flags().StringVar(&site, "site", "", "Site")
	command.Flags().Int64Var(&uid, "uid", 0, "Uid")
	command.Flags().StringVar(&passkey, "passkey", "", "Passkey")
	iyuu.Command.AddCommand(command)
}

func bind(cmd *cobra.Command, args []string) {
	log.Tracef("iyuu token: %s", config.Get().IyuuToken)
	if config.Get().IyuuToken == "" {
		log.Fatalf("You must config iyuuToken in ptool.yaml to use iyuu functions")
	}

	if config.Get().IyuuToken == "" {
		log.Fatalf("iyuuToken not found in config file.")
	}
	if site == "" || uid == 0 || passkey == "" {
		log.Fatalf("You must provide site, uid, passkey to bind iyuu")
	}

	data, err := iyuu.IyuuApiBind(config.Get().IyuuToken, site, uid, passkey)
	fmt.Printf("Iyuu api status: error=%v, user=%v", err, data)
}
