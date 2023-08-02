package suggest

import (
	"os"
	"strings"
	"unicode"

	"github.com/c-bata/go-prompt"
	"github.com/sagan/ptool/client"
	"github.com/sagan/ptool/cmd"
	"github.com/sagan/ptool/cmd/common"
	"github.com/sagan/ptool/config"
)

type InputFlag struct {
	Name  string
	Value string
	// a bare flag does not have value part (which means it's NOT in "--name=value" format)
	Bare bool
}

// parsed info of current inputing command
type InputingCommand struct {
	Args           []string
	MatchingPrefix string // the prefix of inputing arg or flag, could be an empty string
	// the index of last arg, in all args but excluding flags.
	// It does not matter whether last arg itself is a flag
	// eg: "ls -lh /root" => 2
	LastArgIndex  int64
	LastArgIsFlag bool   // if last arg is a flag
	LastArgFlag   string // the flag name of last arg
}

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

func FileArg(prefix string, extension string) []prompt.Suggest {
	files, err := os.ReadDir(".")
	if err != nil {
		return nil
	}
	suggestions := []prompt.Suggest{}
	for _, file := range files {
		if extension != "" && (file.IsDir() || !strings.HasSuffix(file.Name(), "."+extension)) {
			continue
		}
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
		if !client.Disabled && strings.HasPrefix(client.Name, prefix) {
			suggestions = append(suggestions, prompt.Suggest{Text: client.Name, Description: "<client>"})
		}
	}
	return suggestions
}

func SiteArg(prefix string) []prompt.Suggest {
	suggestions := []prompt.Suggest{}
	for _, site := range config.Get().Sites {
		if !site.Disabled && strings.HasPrefix(site.GetName(), prefix) {
			suggestions = append(suggestions, prompt.Suggest{Text: site.GetName(), Description: "<site>"})
		}
	}
	return suggestions
}

func ClientOrSiteArg(prefix string) []prompt.Suggest {
	suggestions := []prompt.Suggest{}
	for _, client := range config.Get().Clients {
		if !client.Disabled && strings.HasPrefix(client.Name, prefix) {
			suggestions = append(suggestions, prompt.Suggest{Text: client.Name, Description: "<client>"})
		}
	}
	for _, site := range config.Get().Sites {
		if !site.Disabled && strings.HasPrefix(site.GetName(), prefix) {
			suggestions = append(suggestions, prompt.Suggest{Text: site.GetName(), Description: "<site>"})
		}
	}
	return suggestions
}

func ClientOrSiteOrGroupArg(prefix string) []prompt.Suggest {
	suggestions := []prompt.Suggest{}
	for _, client := range config.Get().Clients {
		if !client.Disabled && strings.HasPrefix(client.Name, prefix) {
			suggestions = append(suggestions, prompt.Suggest{Text: client.Name, Description: "<client>"})
		}
	}
	for _, site := range config.Get().Sites {
		if !site.Disabled && strings.HasPrefix(site.GetName(), prefix) {
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

func SiteOrGroupArg(prefix string) []prompt.Suggest {
	suggestions := []prompt.Suggest{}
	for _, site := range config.Get().Sites {
		if !site.Disabled && strings.HasPrefix(site.GetName(), prefix) {
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

// parse current inputing command (from start to cursor)
// everything after cursor is ignored
// it's far from complete, does not handle a lot of thing (eg. spaces, quotes) at all.
// this func requires external info to discriminate whether a flag is pure or not
func Parse(document *prompt.Document) *InputingCommand {
	args := []string{}
	noneFlagArgsCnt := int64(0)
	currentArg := ""
	var previousFlag *InputFlag
	for index, char := range document.Text {
		if index >= document.CursorPositionCol() {
			break
		}
		if unicode.IsSpace(char) {
			if currentArg != "" {
				flag, isFlag := ParseFlag(currentArg)
				if isFlag {
					previousFlag = flag
				} else {
					if previousFlag == nil || !previousFlag.Bare || common.IsPureFlag(previousFlag.Name) {
						noneFlagArgsCnt++
					}
					previousFlag = nil
				}
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
	var inputingCommand InputingCommand
	inputingCommand.Args = args
	inputingCommand.LastArgIndex = noneFlagArgsCnt
	flag, isFlag := ParseFlag(currentArg) // last arg flag
	if isFlag {
		inputingCommand.LastArgIsFlag = true
		// only parse last flag name when it is complete
		if !flag.Bare {
			inputingCommand.LastArgFlag = flag.Name
			inputingCommand.MatchingPrefix = flag.Value
		}
	} else {
		inputingCommand.MatchingPrefix = currentArg
		// if second last arg is a none-pure bare flag, treat it as the flag of lastArg
		// a none-pure flag should has a value part
		// So in the case of penultimate arg be none-pure bare flag, the last arg should be the value of that flag
		if previousFlag != nil && previousFlag.Bare && !common.IsPureFlag(previousFlag.Name) {
			inputingCommand.LastArgFlag = previousFlag.Name
			inputingCommand.LastArgIsFlag = true
		}
	}
	return &inputingCommand
}

func ParseFlag(arg string) (flag *InputFlag, isFlag bool) {
	if strings.HasPrefix(arg, "-") {
		isFlag = true
		flag = &InputFlag{}
		if i := strings.Index(arg, "="); i == -1 {
			flag.Bare = true
		} else {
			flag.Value = arg[i+1:]
			arg = arg[:i]
		}
		arg = strings.TrimPrefix(arg, "--")
		arg = strings.TrimPrefix(arg, "-")
		flag.Name = arg
	}
	return
}

func EnumArg(prefix string, options [][2]string) []prompt.Suggest {
	suggestions := []prompt.Suggest{}
	for _, option := range options {
		if strings.HasPrefix(option[0], prefix) {
			suggestions = append(suggestions, prompt.Suggest{Text: option[0], Description: option[1]})
		}
	}
	return suggestions
}

func EnumFlagArg(prefix string, enumFlag *cmd.EnumFlag) []prompt.Suggest {
	return EnumArg(prefix, enumFlag.Options)
}

// _all|_active|_done|_undone
var infoHashFilters = [][2]string{
	{"_all", ":: all torrents"},
	{"_active", ":: current active torrents"},
	{"_done", ":: completed downloaded torrents (_completed | _seeding)"},
	{"_undone", ":: incomplete downloaded torrents (_downloading | _paused)"},
	{"_seeding", "state: seeding"},
	{"_downloading", "state: downloading"},
	{"_completed", "state: completed"},
	{"_paused", "state: paused"},
	{"_checking", "state: checking"},
	{"_error", "state: error"},
	{"_unknown", "state: unknown"},
}

func InfoHashOrFilterArg(prefix string, clientName string) []prompt.Suggest {
	if strings.HasPrefix(prefix, "_") {
		return EnumArg(prefix, infoHashFilters)
	}
	return InfoHashArg(prefix, clientName)
}

func InfoHashArg(prefix string, clientName string) []prompt.Suggest {
	if len(prefix) < 2 || clientName == "" {
		return nil
	}
	clientInstance, err := client.CreateClient(clientName)
	if err != nil {
		return nil
	}
	if !clientInstance.Cached() {
		return nil
	}
	torrents, err := clientInstance.GetTorrents("", "", true)
	if err != nil {
		return nil
	}
	suggestions := []prompt.Suggest{}
	for _, torrent := range torrents {
		if !strings.HasPrefix(torrent.InfoHash, prefix) {
			continue
		}
		suggestions = append(suggestions, prompt.Suggest{
			Text:        torrent.InfoHash,
			Description: torrent.StateIconText() + " " + torrent.Name,
		})
	}
	return suggestions
}
