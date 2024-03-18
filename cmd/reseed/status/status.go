package status

import (
	"fmt"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/sagan/ptool/cmd/reseed"
	"github.com/sagan/ptool/config"
)

var command = &cobra.Command{
	Use:         "status",
	Annotations: map[string]string{"cobra-prompt-dynamic-suggestions": "reseed.status"},
	Short:       "Show Reseed user status.",
	Long:        `Show Reseed user status.`,
	RunE:        status,
}

func init() {
	reseed.Command.AddCommand(command)
}

func status(cmd *cobra.Command, args []string) error {
	if config.Get().ReseedUsername == "" || config.Get().ReseedPassword == "" {
		return fmt.Errorf("you must config reseedUsername & reseedPassword in ptool.toml to use reseed functions")
	}
	log.Debugf("Login using reseed username=%s password=%s", config.Get().ReseedUsername, config.Get().ReseedPassword)
	token, err := reseed.Login(config.Get().ReseedUsername, config.Get().ReseedPassword)
	if err != nil {
		return err
	}
	fmt.Printf("✓ success logined with username %s, acquired token: %s\n", config.Get().ReseedUsername, token)
	return nil
}
