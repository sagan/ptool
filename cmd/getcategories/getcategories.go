package getcategories

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/sagan/ptool/client"
	"github.com/sagan/ptool/cmd"
	"github.com/sagan/ptool/util"
)

var command = &cobra.Command{
	Use:         "getcategories {client}",
	Annotations: map[string]string{"cobra-prompt-dynamic-suggestions": "getcategories"},
	Short:       "Get all categories of client.",
	Long:        `Get all categories of client.`,
	Args:        cobra.MatchAll(cobra.ExactArgs(1), cobra.OnlyValidArgs),
	RunE:        getcategories,
}

var (
	showNamesOnly = false
)

func init() {
	command.Flags().BoolVarP(&showNamesOnly, "show-names-only", "", false, "Show category names only")
	cmd.RootCmd.AddCommand(command)
}

func getcategories(cmd *cobra.Command, args []string) error {
	clientName := args[0]
	clientInstance, err := client.CreateClient(clientName)
	if err != nil {
		return fmt.Errorf("failed to create client: %v", err)
	}

	cats, err := clientInstance.GetCategories()
	if err != nil {
		return fmt.Errorf("failed to get categories: %v", err)
	}
	if showNamesOnly {
		fmt.Printf("%s\n", strings.Join(util.Map(cats, func(cat client.TorrentCategory) string {
			return cat.Name
		}), ", "))
	} else {
		fmt.Printf("%-20s  %s\n", "Name", "SavePath")
		for _, cat := range cats {
			fmt.Printf("%-20s  %s\n", cat.Name, cat.SavePath)
		}
	}
	return nil
}
