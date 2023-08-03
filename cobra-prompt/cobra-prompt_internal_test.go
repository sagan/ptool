package cobraprompt

import (
	"testing"

	"github.com/c-bata/go-prompt"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
)

func newTestCommand(use string, short string) *cobra.Command {
	return &cobra.Command{
		Use:   use,
		Short: short,
		Run:   func(cmd *cobra.Command, args []string) {},
	}
}

var rootCmd = newTestCommand("root", "The root cmd")
var getCmd = newTestCommand("get", "Get something")
var getObjectCmd = newTestCommand("object", "Get the object")
var getThingCmd = newTestCommand("thing", "The thing")

func init() {
	rootCmd.AddCommand(getCmd)
	getCmd.AddCommand(getObjectCmd)
	getCmd.AddCommand(getThingCmd)
	getObjectCmd.Flags().BoolP("verbose", "v", false, "Verbose log")
}

func TestFindSuggestions(t *testing.T) {
	cp := &CobraPrompt{
		RootCmd: rootCmd,
	}
	buf := prompt.NewBuffer()

	buf.InsertText("", false, true)
	suggestions := findSuggestions(cp, buf.Document())
	hasLen := assert.Len(t, suggestions, 1, "Should find 1 suggestion")
	if hasLen {
		assert.Equal(t, getCmd.Name(), suggestions[0].Text, "Should find get command")
	}

	buf.InsertText("get ", false, true)
	suggestions = findSuggestions(cp, buf.Document())

	hasLen = assert.Len(t, suggestions, 2, "Should find 2 sub commands under get")
	if hasLen {
		assert.Equal(t, getObjectCmd.Name(), suggestions[0].Text, "Should find object command")
		assert.Equal(t, getThingCmd.Name(), suggestions[1].Text, "Should find thing command")
	}

	buf.InsertText("object -", false, true)
	suggestions = findSuggestions(cp, buf.Document())

	hasLen = assert.Len(t, suggestions, 1, "Should find verbose flag")
	if hasLen {
		assert.Equal(t, "-v", suggestions[0].Text, "Should find verbose short flag")
	}

	buf.InsertText("-", false, true)
	suggestions = findSuggestions(cp, buf.Document())

	hasLen = assert.Len(t, suggestions, 1, "Should find verbose flag")
	if hasLen {
		assert.Equal(t, "--verbose", suggestions[0].Text, "Should find verbose flag")
	}
}
