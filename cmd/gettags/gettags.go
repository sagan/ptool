package gettags

import (
	"fmt"
	"strings"

	"github.com/sagan/ptool/client"
	"github.com/sagan/ptool/cmd"
	"github.com/spf13/cobra"

	log "github.com/sirupsen/logrus"
)

var command = &cobra.Command{
	Use:   "gettags <client>",
	Short: "Get all tags of client.",
	Long:  `Get all tags of client.`,
	Args:  cobra.MatchAll(cobra.ExactArgs(1), cobra.OnlyValidArgs),
	Run:   gettags,
}

func init() {
	cmd.RootCmd.AddCommand(command)
}

func gettags(cmd *cobra.Command, args []string) {
	clientInstance, err := client.CreateClient(args[0])
	if err != nil {
		log.Fatal(err)
	}

	tags, err := clientInstance.GetTags()
	if err != nil {
		log.Fatalf("Failed to get tags: %v", err)
	}
	fmt.Printf("%s\n", strings.Join(tags, ", "))
}
