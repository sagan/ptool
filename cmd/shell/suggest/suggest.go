package suggest

import (
	"os"
	"strings"
	"unicode"

	"github.com/c-bata/go-prompt"
	"github.com/sagan/ptool/config"
)

func DirArg(prefix string) []prompt.Suggest {
	files, err := os.ReadDir(".")
	if err != nil {
		return nil
	}
	suggestions := []prompt.Suggest{}
	for _, file := range files {
		if file.IsDir() && strings.HasPrefix(file.Name(), prefix) {
			suggestions = append(suggestions, prompt.Suggest{Text: file.Name(), Description: "<dir>"})
		}
	}
	return suggestions
}

func FileArg(prefix string) []prompt.Suggest {
	files, err := os.ReadDir(".")
	if err != nil {
		return nil
	}
	suggestions := []prompt.Suggest{}
	for _, file := range files {
		if strings.HasPrefix(file.Name(), prefix) {
			desc := ""
			if file.IsDir() {
				desc = "<dir>"
			} else {
				desc = "<file>"
			}
			suggestions = append(suggestions, prompt.Suggest{Text: file.Name(), Description: desc})
		}
	}
	return suggestions
}

func ClientArg(prefix string) []prompt.Suggest {
	suggestions := []prompt.Suggest{}
	for _, client := range config.Get().Clients {
		if strings.HasPrefix(client.Name, prefix) {
			suggestions = append(suggestions, prompt.Suggest{Text: client.Name, Description: "<client>"})
		}
	}
	return suggestions
}

func ClientOrSiteArg(prefix string) []prompt.Suggest {
	suggestions := []prompt.Suggest{}
	for _, client := range config.Get().Clients {
		if strings.HasPrefix(client.Name, prefix) {
			suggestions = append(suggestions, prompt.Suggest{Text: client.Name, Description: "<client>"})
		}
	}
	for _, site := range config.Get().Sites {
		if strings.HasPrefix(site.GetName(), prefix) {
			suggestions = append(suggestions, prompt.Suggest{Text: site.GetName(), Description: "<site>"})
		}
	}
	return suggestions
}

func ClientOrSiteOrGroupArg(prefix string) []prompt.Suggest {
	suggestions := []prompt.Suggest{}
	for _, client := range config.Get().Clients {
		if strings.HasPrefix(client.Name, prefix) {
			suggestions = append(suggestions, prompt.Suggest{Text: client.Name, Description: "<client>"})
		}
	}
	for _, site := range config.Get().Sites {
		if strings.HasPrefix(site.GetName(), prefix) {
			suggestions = append(suggestions, prompt.Suggest{Text: site.GetName(), Description: "<site>"})
		}
	}
	for _, group := range config.Get().Groups {
		if strings.HasPrefix(group.Name, prefix) {
			suggestions = append(suggestions, prompt.Suggest{Text: group.Name, Description: "<group>"})
		}
	}
	return suggestions
}

// parse current inputing command (from start to cursor or end), return args
// the currentArg is where cursor is
func Parse(document *prompt.Document) []string {
	args := []string{}
	currentArg := ""
	for index, char := range document.Text {
		if index >= document.CursorPositionCol() {
			break
		}
		if unicode.IsSpace(char) {
			if currentArg != "" {
				args = append(args, currentArg)
				currentArg = ""
			}
			continue
		} else {
			currentArg += string(char)
		}
	}
	// last arg, even blank
	args = append(args, currentArg)
	return args
}
