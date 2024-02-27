package suggest

import (
	"os"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/c-bata/go-prompt"
	"github.com/google/shlex"

	"github.com/sagan/ptool/client"
	"github.com/sagan/ptool/cmd"
	"github.com/sagan/ptool/cmd/common"
	"github.com/sagan/ptool/config"
)

type InputFlag struct {
	Name  string
	Value string
	Short bool // -a or -abc style
	// a bare flag does not have value part (which means it's NOT in "--name=value" format)
	Bare bool
}

// parsed info of current inputing ptool command
type InputingCommand struct {
	// normal (positional) args of the inputing command
	Args           []string
	MatchingPrefix string // the prefix of inputing arg or flag, could be an empty string.
	// count of positional args, excluding the last arg if it self is positional flag.
	// eg: "brush --dry-run --max-sites 10 local mteam" => 2,
	// as "brush" and "local" are positional args. last arg "mteam" also is, but is not counted.
	// "--dry-run" and "--max-sites 10" are flags.
	LastArgIndex  int64
	LastArgIsFlag bool   // if last arg is a flag
	LastArgFlag   string // the flag name of last arg
}

func DirArg(prefix string) []prompt.Suggest {
	return FileArg(prefix, "", true)
}

// any dir is included as it may be an intermediate dir
func FileArg(prefix string, suffix string, dirOnly bool) []prompt.Suggest {
	dirprefix := ""
	dir := "."
	// cann't use filepath.Dir here as it will discard the leading ./ or ../
	if index := strings.LastIndex(prefix, "/"); index != -1 {
		dir = prefix[:index]
		dirprefix = dir + "/"
		prefix = prefix[index+1:]
	}
	files, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	suggestions := []prompt.Suggest{}
	for _, file := range files {
		if dirOnly && !file.IsDir() {
			continue
		}
		if !file.IsDir() && suffix != "" && !strings.HasSuffix(file.Name(), suffix) {
			continue
		}
		if file.IsDir() || strings.HasPrefix(file.Name(), prefix) {
			desc := ""
			if file.IsDir() {
				desc = "<dir>"
			} else {
				desc = "<file>"
			}
			suggestions = append(suggestions, prompt.Suggest{Text: dirprefix + file.Name(), Description: desc})
		}
	}
	return suggestions
}

func ClientArg(prefix string) []prompt.Suggest {
	suggestions := []prompt.Suggest{}
	for _, client := range config.Get().ClientsEnabled {
		if strings.HasPrefix(client.Name, prefix) {
			suggestions = append(suggestions, prompt.Suggest{Text: client.Name, Description: "<client>"})
		}
	}
	return suggestions
}

func SiteArg(prefix string) []prompt.Suggest {
	suggestions := []prompt.Suggest{}
	for _, site := range config.Get().SitesEnabled {
		if strings.HasPrefix(site.GetName(), prefix) {
			suggestions = append(suggestions, prompt.Suggest{Text: site.GetName(), Description: "<site>"})
		}
	}
	return suggestions
}

func ClientOrSiteArg(prefix string) []prompt.Suggest {
	suggestions := []prompt.Suggest{}
	for _, client := range config.Get().ClientsEnabled {
		if strings.HasPrefix(client.Name, prefix) {
			suggestions = append(suggestions, prompt.Suggest{Text: client.Name, Description: "<client>"})
		}
	}
	for _, site := range config.Get().SitesEnabled {
		if strings.HasPrefix(site.GetName(), prefix) {
			suggestions = append(suggestions, prompt.Suggest{Text: site.GetName(), Description: "<site>"})
		}
	}
	return suggestions
}

func ClientOrSiteOrGroupArg(prefix string) []prompt.Suggest {
	suggestions := []prompt.Suggest{}
	for _, client := range config.Get().ClientsEnabled {
		if strings.HasPrefix(client.Name, prefix) {
			suggestions = append(suggestions, prompt.Suggest{Text: client.Name, Description: "<client>"})
		}
	}
	for _, site := range config.Get().SitesEnabled {
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

func SiteOrGroupArg(prefix string) []prompt.Suggest {
	suggestions := []prompt.Suggest{}
	for _, site := range config.Get().SitesEnabled {
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

// parse current inputing ptool command (from start to cursor).
// everything after cursor is ignored.
// this func requires external info to discriminate whether a flag is pure or not
func Parse(document *prompt.Document) *InputingCommand {
	noneFlagArgsCnt := int64(0)
	var previousFlag, lastArgFlag *InputFlag
	txt := document.CurrentLineBeforeCursor()
	lastToken := ""
	tokens, err := shlex.Split(txt)
	args := []string{}
	flagsTerminated := false
	if err == nil {
		for _, token := range tokens {
			var flag *InputFlag
			if !flagsTerminated {
				flag = parseFlag(token)
			}
			if flag == nil {
				if lastArgFlag == nil || !lastArgFlag.Bare || lastArgFlag.Short || lastArgFlag.Name == "" ||
					common.IsPureFlag(lastArgFlag.Name) {
					noneFlagArgsCnt++
					args = append(args, token)
				}
			} else if flag.Name == "" {
				// The argument -- terminates all options parsing
				// see https://www.gnu.org/software/libc/manual/html_node/Argument-Syntax.html
				flagsTerminated = true
			}
			previousFlag = lastArgFlag
			lastArgFlag = flag
		}
		// If the char before cursor is space, treat it as user just started inputing a new (still empty) arg
		// it's a workaround for now as shlex discard all info about spaces
		lastRune, _ := utf8.DecodeLastRuneInString(txt)
		if unicode.IsSpace(lastRune) {
			tokens = append(tokens, "")
			if lastArgFlag == nil || !lastArgFlag.Bare || lastArgFlag.Short || lastArgFlag.Name == "" ||
				common.IsPureFlag(lastArgFlag.Name) {
				noneFlagArgsCnt++
				args = append(args, "")
			}
			previousFlag = lastArgFlag
			lastArgFlag = nil
		}
		if len(tokens) > 0 {
			lastToken = tokens[len(tokens)-1]
		}
	}
	var inputingCommand InputingCommand
	inputingCommand.Args = args
	inputingCommand.LastArgIndex = noneFlagArgsCnt
	if lastArgFlag != nil {
		// input suffix: "... --name=va"
		inputingCommand.LastArgIsFlag = true
		if !lastArgFlag.Bare && !lastArgFlag.Short {
			inputingCommand.LastArgFlag = lastArgFlag.Name
			inputingCommand.MatchingPrefix = lastArgFlag.Value
		}
	} else if previousFlag != nil && previousFlag.Bare && !previousFlag.Short && previousFlag.Name != "" &&
		!common.IsPureFlag(previousFlag.Name) {
		// input suffix: "... --name va"
		inputingCommand.MatchingPrefix = lastToken
		inputingCommand.LastArgFlag = previousFlag.Name
		inputingCommand.LastArgIsFlag = true
	} else {
		// input suffix is a normal (positional) argument
		inputingCommand.MatchingPrefix = lastToken
		inputingCommand.LastArgIndex--
	}
	return &inputingCommand
}

func parseFlag(arg string) (flag *InputFlag) {
	// POSIX treat standalone "-" as NOT flag
	if strings.HasPrefix(arg, "-") && len(arg) > 1 {
		flag = &InputFlag{}
		if i := strings.Index(arg, "="); i == -1 {
			flag.Bare = true
		} else {
			flag.Value = arg[i+1:]
			arg = arg[:i]
		}
		if strings.HasPrefix(arg, "--") {
			arg = arg[2:]
		} else {
			flag.Short = true
			arg = arg[1:]
		}
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
	if clientName == "" {
		return nil
	}
	clientInstance, err := client.CreateClient(clientName)
	if err != nil {
		return nil
	}
	if !clientInstance.Cached() {
		return nil
	}
	torrents, err := clientInstance.GetTorrents("", "", len(prefix) >= 2)
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
