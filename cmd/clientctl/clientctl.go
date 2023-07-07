package clientctl

import (
	"fmt"
	"os"
	"strings"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"golang.org/x/exp/slices"

	"github.com/sagan/ptool/client"
	"github.com/sagan/ptool/cmd"
	"github.com/sagan/ptool/utils"
)

type Option struct {
	Name        string
	Type        int64 // 0 - normal; 1 - Speed; 2 - Size
	Readonly    bool
	Auto        bool
	Description string
}

var command = &cobra.Command{
	Use:   "clientctl <client> [<variable>[=value] ...]",
	Short: "Get or set client config.",
	Long:  `Get or set client config.`,
	Run:   clientctl,
}

var (
	allOptions = []Option{
		{"global_download_speed_limit", 1, false, true, "Global download speed limit (/s)"},
		{"global_upload_speed_limit", 1, false, true, "Global upload speed limit (/s)"},
		{"global_download_speed", 1, true, false, "Current global download speed (/s)"},
		{"global_upload_speed", 1, true, false, "Current global upload speed (/s)"},
		{"free_disk_space", 2, true, false, "Current free disk space of default save path"},
		{"save_path", 0, false, false, "Default save path"},
		{"qb_*", 0, false, false, "The qBittorrent specific preferences. For full list see https://github.com/qbittorrent/qBittorrent/wiki/WebUI-API-(qBittorrent-4.1)#get-application-preferences . eg. qb_create_subfolder_enabled"},
		{"tr_*", 0, true, false, "The transmission specific preferences (read-only for now). For full list see https://github.com/transmission/transmission/blob/3.00/extras/rpc-spec.txt#L482 . Convert argument name to snake_case. eg. tr_config_dir"},
	}
	showRaw        = false
	showValueOnly  = false
	showParameters = false
)

func init() {
	command.Flags().BoolVarP(&showParameters, "show-parameters", "", false, "Print all parameters list and exit")
	command.Flags().BoolVarP(&showRaw, "raw", "", false, "Display config value data in raw format")
	command.Flags().BoolVarP(&showValueOnly, "show-value-only", "", false, "Show config value data only")
	cmd.RootCmd.AddCommand(command)
}

func clientctl(cmd *cobra.Command, args []string) {
	if showParameters {
		fmt.Printf("%-30s %-5s %-5s %s\n", "Name", "Type", "Auto", "Description")
		for _, option := range allOptions {
			permission := "rw"
			if option.Readonly {
				permission = "r"
			}
			auto := ""
			if option.Auto {
				auto = "✓"
			}
			fmt.Printf("%-30s %-5s %-5s %s\n", option.Name, permission, auto, option.Description)
		}
		os.Exit(0)
	}
	if len(args) < 1 {
		log.Fatalf("<client> not provided")
	}
	if showRaw && showValueOnly {
		log.Fatalf("--raw and --show-value-only flags are NOT compatible")
	}
	clientName := args[0]
	clientInstance, err := client.CreateClient(clientName)
	if err != nil {
		log.Fatal(err)
	}
	args = args[1:]
	errorCnt := int64(0)
	if len(args) == 0 {
		args = []string{}
		for _, option := range allOptions {
			if option.Auto {
				args = append(args, option.Name)
			}
		}
	}

	for _, variable := range args {
		s := strings.Split(variable, "=")
		name := s[0]
		value := ""
		var err error
		if (clientInstance.GetClientConfig().Type == "qbittorrent" && strings.HasPrefix(variable, "qb_") ||
			clientInstance.GetClientConfig().Type == "transmission" && strings.HasPrefix(variable, "tr_")) && len(variable) > 3 {
			if len(s) == 1 {
				value, err = clientInstance.GetConfig(name)
				if err != nil {
					log.Errorf("Error get %s: %v", name, err)
				}
			} else {
				value = s[1]
				err = clientInstance.SetConfig(name, value)
				if err != nil {
					log.Errorf("Error set %s: %v", name, err)
				}
			}
			if err == nil {
				if showValueOnly {
					fmt.Printf("%v\n", value)
				} else {
					fmt.Printf("%s=%v\n", name, value)
				}
			} else {
				errorCnt++
			}
			continue
		}
		index := slices.IndexFunc(allOptions, func(o Option) bool { return o.Name == name })
		if index == -1 {
			log.Fatal("Unrecognized parameter: " + name)
		}
		option := allOptions[index]
		if len(s) == 1 {
			value, err = clientInstance.GetConfig(name)
			if err != nil {
				log.Errorf("Error get client %s config %s: %v", clientInstance.GetName(), name, err)
				errorCnt++
			}
		} else {
			if option.Readonly {
				log.Errorf("Error set client %s config %s: read-only", clientInstance.GetName(), name)
				errorCnt++
				continue
			}
			value = s[1]
			if option.Type > 0 {
				v, _ := utils.RAMInBytes(value)
				err = clientInstance.SetConfig(name, fmt.Sprint(v))
			} else {
				err = clientInstance.SetConfig(name, value)
			}
			if err != nil {
				log.Errorf("Error set client %s config %s=%s: %v", clientInstance.GetName(), name, value, err)
				value = ""
				errorCnt++
			}
		}
		if showValueOnly {
			fmt.Printf("%v\n", value)
		} else {
			printOption(name, value, option, showRaw)
		}
	}
	clientInstance.Close()
	if errorCnt > 0 {
		os.Exit(1)
	}
}

func printOption(name string, value string, option Option, showRaw bool) {
	if value != "" && option.Type > 0 {
		ff, _ := utils.RAMInBytes(value)
		if !showRaw {
			if option.Type == 1 {
				fmt.Printf("%s=%s/s\n", name, utils.BytesSize(float64(ff)))
			} else {
				fmt.Printf("%s=%s\n", name, utils.BytesSize(float64(ff)))
			}
		} else {
			fmt.Printf("%s=%d\n", name, ff)
		}
	} else {
		fmt.Printf("%s=%s\n", name, value)
	}
}
