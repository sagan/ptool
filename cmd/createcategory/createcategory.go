package createcategory

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/sagan/ptool/client"
	"github.com/sagan/ptool/cmd"
)

var command = &cobra.Command{
	Use:         "createcategory {client} {category} [--save-path path]",
	Annotations: map[string]string{"cobra-prompt-dynamic-suggestions": "createcategory"},
	Short:       "Create or edit category in client.",
	Long: `Create category in client. If category already exists, edit it.
Use --save-path to set (or modify) the save path of the category.`,
	Args: cobra.MatchAll(cobra.ExactArgs(2), cobra.OnlyValidArgs),
	RunE: createcategory,
}

var (
	savePath = ""
)

func init() {
	command.Flags().StringVarP(&savePath, "save-path", "", "", "Set the save path of the category. Can use \"none\" to set it back to default empty value")
	cmd.RootCmd.AddCommand(command)
}

func createcategory(cmd *cobra.Command, args []string) error {
	clientName := args[0]
	category := args[1]
	clientInstance, err := client.CreateClient(clientName)
	if err != nil {
		return fmt.Errorf("failed to create client: %v", err)
	}

	err = clientInstance.MakeCategory(category, savePath)
	if err != nil {
		return fmt.Errorf("failed to create category: %v", err)
	}
	return nil
}
