package bind

import (
	"fmt"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/sagan/ptool/cmd/iyuu"
	"github.com/sagan/ptool/config"
)

var command = &cobra.Command{
	Use:         "bind --site {iyuu_site_name} --uid {site_uid} --passkey {site_passkey}",
	Annotations: map[string](string){"cobra-prompt-dynamic-suggestions": "iyuu.bind"},
	Short:       "Bind (authenticate) iyuu service using PT site passkey.",
	Long: `Bind (authenticate) iyuu service using PT site passkey.

Example:
ptool iyuu bind --site zhuque --uid 123456 --passkey 0123456789abcdef

* --site {iyuu_site_name} : iyuu site name. Use "ptool iyuu sites -b" to list all bindable sites.
* --uid {site_uid} : the uid (user id) of corresponding site
* --passkey {site_passkey} : the passkey (or equivalent) of corresponding site`,
	RunE: bind,
}

var (
	site    = ""
	uid     = int64(0)
	passkey = ""
)

func init() {
	command.Flags().StringVarP(&site, "site", "", "", "(Required) Iyuu sitename used for binding. eg. zhuque")
	command.Flags().Int64VarP(&uid, "uid", "", 0, "(Required) Site uid")
	command.Flags().StringVarP(&passkey, "passkey", "", "", "(Required) Site passkey (or equivalent key)")
	command.MarkFlagRequired("site")
	command.MarkFlagRequired("uid")
	command.MarkFlagRequired("passkey")
	iyuu.Command.AddCommand(command)
}

func bind(cmd *cobra.Command, args []string) error {
	log.Tracef("iyuu token: %s", config.Get().IyuuToken)
	if config.Get().IyuuToken == "" {
		return fmt.Errorf("you must config iyuuToken in ptool.toml to use iyuu functions")
	}

	data, err := iyuu.IyuuApiBind(config.Get().IyuuToken, site, uid, passkey)
	fmt.Printf("Iyuu api status: error=%v, user=%v", err, data)
	return err
}
