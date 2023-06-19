package getcategories

import (
	"fmt"
	"strings"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/sagan/ptool/client"
	"github.com/sagan/ptool/cmd"
)

var command = &cobra.Command{
	Use:   "getcategories <client>",
	Short: "Get all categories of client",
	Long:  `Get all categories of client`,
	Args:  cobra.MatchAll(cobra.ExactArgs(1), cobra.OnlyValidArgs),
	Run:   getcategories,
}

func init() {
	cmd.RootCmd.AddCommand(command)
}

func getcategories(cmd *cobra.Command, args []string) {
	clientInstance, err := client.CreateClient(args[0])
	if err != nil {
		log.Fatal(err)
	}

	cats, err := clientInstance.GetCategories()
	clientInstance.Close()
	if err != nil {
		log.Fatalf("Failed to get categories: %v", err)
	}
	fmt.Printf("%s\n", strings.Join(cats, ", "))
}
