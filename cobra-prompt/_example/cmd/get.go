package cmd

import (
	"github.com/c-bata/go-prompt"
	"github.com/spf13/cobra"
	cobraprompt "github.com/stromland/cobra-prompt"
)

var getFoodDynamicAnnotationValue = "GetFood"

var GetFoodDynamic = func(annotationValue string) []prompt.Suggest {
	if annotationValue != getFoodDynamicAnnotationValue {
		return nil
	}

	return []prompt.Suggest{
		{Text: "apple", Description: "Green apple"},
		{Text: "tomato", Description: "Red tomato"},
	}
}

var getCmd = &cobra.Command{
	Use:     "get",
	Short:   "Get something",
	Aliases: []string{"eat"},
	Run: func(cmd *cobra.Command, args []string) {
		cmd.Usage()
	},
}

var getFoodCmd = &cobra.Command{
	Use:   "food",
	Short: "Get some food",
	Annotations: map[string]string{
		cobraprompt.DynamicSuggestionsAnnotation: getFoodDynamicAnnotationValue,
	},
	Run: func(cmd *cobra.Command, args []string) {
		verbose, _ := cmd.Flags().GetBool("verbose")
		for _, v := range args {
			if verbose {
				cmd.Println("Here you go, take this:", v)
			} else {
				cmd.Println(v)
			}
		}
	},
}

func init() {
	RootCmd.AddCommand(getCmd)
	getCmd.AddCommand(getFoodCmd)
	getCmd.PersistentFlags().BoolP("verbose", "v", false, "Verbose log")
}
