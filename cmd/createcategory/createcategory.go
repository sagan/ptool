package createcategory

import (
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/sagan/ptool/client"
	"github.com/sagan/ptool/cmd"
)

var command = &cobra.Command{
	Use:   "createcategory <client> <category>",
	Short: "Create or edit category in client.",
	Long: `Create category in client. If category already exists, edit it.
Use --save-path to set (or modify) the save path of the category.`,
	Args: cobra.MatchAll(cobra.ExactArgs(2), cobra.OnlyValidArgs),
	Run:  createcategory,
}

var (
	savePath = ""
)

func init() {
	command.Flags().StringVarP(&savePath, "save-path", "", "", "Set the save path of the category. Can use \"none\" to set it back to default empty value")
	cmd.RootCmd.AddCommand(command)
}

func createcategory(cmd *cobra.Command, args []string) {
	clientName := args[0]
	category := args[1]
	clientInstance, err := client.CreateClient(clientName)
	if err != nil {
		log.Fatal(err)
	}

	err = clientInstance.MakeCategory(category, savePath)
	clientInstance.Close()
	if err != nil {
		log.Fatalf("Failed to create category: %v", err)
	}
}
