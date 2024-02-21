package configcmd

// @todo (maybe): implement an interactive config mode (like "rclone config")

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/sagan/ptool/client"
	"github.com/sagan/ptool/cmd"
	"github.com/sagan/ptool/config"
	"github.com/sagan/ptool/site"
	"github.com/sagan/ptool/site/tpl"
	"github.com/sagan/ptool/util"
)

var Command = &cobra.Command{
	Use:   "config",
	Short: "Display or manage config file contents.",
	Long:  `Display or manage config file contents.`,
	Args:  cobra.MatchAll(cobra.ExactArgs(0), cobra.OnlyValidArgs),
	RunE:  configcmd,
}

var (
	filter = ""
)

func init() {
	Command.Flags().StringVarP(&filter, "filter", "", "", "Only show config item which name or note contains this")
	cmd.RootCmd.AddCommand(Command)
}

func configcmd(cmd *cobra.Command, args []string) error {
	fmt.Printf("Config file: %s%c%s\n", config.ConfigDir, filepath.Separator, config.ConfigFile)
	if _, err := os.Stat(path.Join(config.ConfigDir, config.ConfigFile)); err != nil {
		if os.IsNotExist(err) {
			fmt.Printf("<config file not exists>\n")
		} else {
			fmt.Printf("<config file can not be accessed: %v>\n", err)
		}
		return nil
	}
	clients := util.CopySlice(config.Get().Clients)
	sites := util.CopySlice(config.Get().Sites)
	groups := util.CopySlice(config.Get().Groups)
	aliases := util.CopySlice(config.Get().Aliases)
	cookieclouds := util.CopySlice(config.Get().Cookieclouds)
	sort.Slice(clients, func(i, j int) bool {
		return clients[i].Name < clients[j].Name
	})
	sort.Slice(sites, func(i, j int) bool {
		return sites[i].Name < sites[j].Name
	})
	sort.Slice(groups, func(i, j int) bool {
		return groups[i].Name < groups[j].Name
	})
	sort.Slice(aliases, func(i, j int) bool {
		return aliases[i].Name < aliases[j].Name
	})
	sort.Slice(cookieclouds, func(i, j int) bool {
		if cookieclouds[i].Name != cookieclouds[j].Name {
			return cookieclouds[i].Name < cookieclouds[j].Name
		}
		if cookieclouds[i].Server != cookieclouds[j].Server {
			return cookieclouds[i].Server < cookieclouds[j].Server
		}
		return cookieclouds[i].Uuid < cookieclouds[j].Uuid
	})
	fmt.Printf("All: %d clients, %d sites, %d groups, %d cookiecloud profiles, %d aliases\n",
		len(clients), len(sites), len(groups), len(cookieclouds), len(aliases))
	emptyListPlaceholder := "<none found>\n"
	emptyFlag := false
	if filter != "" {
		emptyListPlaceholder = "<none matched found>\n"
		fmt.Printf("Applying filter '%s'\n", filter)
	} else {
		fmt.Printf(`To filter, use "--filter string" flag` + "\n")
	}
	fmt.Printf("\n")

	fmt.Printf(`Clients: (test: "ptool status <name> -t")` + "\n")
	fmt.Printf("%-15s  %-15s  %-s\n", "Name", "Type", "Note")
	emptyFlag = true
	for _, clientConfig := range clients {
		note := ""
		if filter != "" && !clientConfig.MatchFilter(filter) {
			continue
		}
		emptyFlag = false
		if clientInstance, err := client.CreateClient(clientConfig.Name); err != nil {
			note = fmt.Sprintf("<error>: %v", err)
		} else {
			note = clientInstance.GetClientConfig().Url
		}
		fmt.Printf("%-15s  %-15s  %-s\n", clientConfig.Name, clientConfig.Type, note)
	}
	if emptyFlag {
		fmt.Print(emptyListPlaceholder)
	}
	fmt.Printf("\n")

	fmt.Printf(`Sites: (internal: *) (test: "ptool status <name> -t")` + "\n")
	fmt.Printf("%-15s  %-15s  %-15s  %-s\n", "Name", "Type", "Flags", "Note")
	emptyFlag = true
	for _, siteConfig := range sites {
		flags := []string{}
		note := ""
		show := filter == "" || siteConfig.MatchFilter(filter)
		for id := range tpl.SITES {
			if id == siteConfig.Type {
				flags = append(flags, "*")
				break
			}
		}
		if siteInstance, err := site.CreateSite(siteConfig.GetName()); err != nil {
			flags = append(flags, "<error>")
			note = fmt.Sprintf("%v", err)
		} else {
			note = siteInstance.GetSiteConfig().Url
			if util.ContainsI(note, filter) {
				show = true
			}
		}
		if !show {
			continue
		}
		emptyFlag = false
		fmt.Printf("%-15s  %-15s  %-15s  %-s\n", siteConfig.GetName(), siteConfig.Type, strings.Join(flags, ", "), note)
	}
	if emptyFlag {
		fmt.Print(emptyListPlaceholder)
	}
	fmt.Printf("\n")

	fmt.Printf("Groups:\n")
	fmt.Printf("%-15s  %-s\n", "Name", "Sites")
	emptyFlag = true
	for _, groupConfig := range groups {
		if filter != "" && !groupConfig.MatchFilter(filter) {
			continue
		}
		emptyFlag = false
		fmt.Printf("%-15s  %-s\n", groupConfig.Name, strings.Join(groupConfig.Sites, ", "))
	}
	if emptyFlag {
		fmt.Print(emptyListPlaceholder)
	}
	fmt.Printf("\n")

	fmt.Printf(`Cookiecloud profiles: (test: "ptool cookiecloud status")` + "\n")
	fmt.Printf("%-15s  %-15s  %-15s  %s\n", "Name", "Server", "Uuid", "Sites")
	emptyFlag = true
	for _, cookiecloud := range cookieclouds {
		if filter != "" && !cookiecloud.MatchFilter(filter) {
			continue
		}
		emptyFlag = false
		fmt.Printf("%-15s  %-15s  %-15s  %s\n", cookiecloud.Name, util.GetUrlDomain(cookiecloud.Server),
			cookiecloud.Uuid, strings.Join(cookiecloud.Sites, ", "))
	}
	if emptyFlag {
		fmt.Print(emptyListPlaceholder)
	}
	fmt.Printf("\n")

	fmt.Printf("Aliases:\n")
	fmt.Printf("%-15s  %-s\n", "Name", "Cmd")
	emptyFlag = true
	for _, aliasConfig := range aliases {
		if filter != "" && !aliasConfig.MatchFilter(filter) {
			continue
		}
		emptyFlag = false
		fmt.Printf("%-15s  %-s\n", aliasConfig.Name, aliasConfig.Cmd)
	}
	if emptyFlag {
		fmt.Print(emptyListPlaceholder)
	}
	fmt.Printf("\n")

	return nil
}
