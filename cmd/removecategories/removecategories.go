package removecategories

import (
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/sagan/ptool/client"
	"github.com/sagan/ptool/cmd"
)

var command = &cobra.Command{
	Use:   "removecategories <client> <categories>...",
	Short: "Remove categories from client.",
	Long:  `Remove categories from client.`,
	Args:  cobra.MatchAll(cobra.MinimumNArgs(2), cobra.OnlyValidArgs),
	Run:   removecategories,
}

func init() {
	cmd.RootCmd.AddCommand(command)
}

func removecategories(cmd *cobra.Command, args []string) {
	clientName := args[0]
	categories := args[1:]
	clientInstance, err := client.CreateClient(clientName)
	if err != nil {
		log.Fatal(err)
	}

	err = clientInstance.RemoveCategories(categories)
	clientInstance.Close()
	if err != nil {
		log.Fatal(err)
	}
}
