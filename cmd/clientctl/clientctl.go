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
	Name string
	Type int64 // 0 - normal; 1 - Speed; 2 - Size
	Auto bool
}

var command = &cobra.Command{
	Use:   "clientctl <client> [<variable>[=value] ...]",
	Args:  cobra.MatchAll(cobra.MinimumNArgs(1), cobra.OnlyValidArgs),
	Short: "Get or set client config.",
	Long:  `Get or set client config.`,
	Run:   clientctl,
}

var (
	allOptions = []Option{
		{"global_download_speed_limit", 1, true},
		{"global_upload_speed_limit", 1, true},
		{"global_download_speed", 1, false},
		{"global_upload_speed", 1, false},
		{"free_disk_space", 2, false},
	}
	showRaw       = false
	showValueOnly = false
)

func init() {
	command.Flags().BoolVarP(&showRaw, "raw", "", false, "Display config value data in raw format")
	command.Flags().BoolVarP(&showValueOnly, "show-value-only", "", false, "show config value data only")
	cmd.RootCmd.AddCommand(command)
}

func clientctl(cmd *cobra.Command, args []string) {
	if showRaw && showValueOnly {
		log.Fatalf("--raw and --show-value-only flags are NOT compatible")
	}
	clientInstance, err := client.CreateClient(args[0])
	if err != nil {
		log.Fatal(err)
	}
	args = args[1:]
	cntError := int64(0)
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
		index := slices.IndexFunc(allOptions, func(o Option) bool { return o.Name == name })
		if index == -1 {
			log.Fatal("Unrecognized option " + name)
		}
		option := allOptions[index]
		if len(s) == 1 {
			value, err = clientInstance.GetConfig(name)
			if err != nil {
				log.Errorf("Error get client %s config %s: %v", clientInstance.GetName(), name, err)
				cntError++
			}
		} else {
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
				cntError++
			}
		}
		if showValueOnly {
			fmt.Printf("%v\n", value)
		} else {
			printOption(name, value, option, showRaw)
		}
	}
	clientInstance.Close()
	if cntError > 0 {
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
