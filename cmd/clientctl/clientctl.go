package clientctl

import (
	"fmt"
	"os"
	"strings"

	"github.com/sagan/ptool/client"
	"github.com/sagan/ptool/cmd"
	"github.com/sagan/ptool/utils"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"golang.org/x/exp/slices"
)

var command = &cobra.Command{
	Use: "clientctl [variable[=value] ...]",
	// Args:  cobra.MatchAll(cobra.ExactArgs(1), cobra.OnlyValidArgs),
	Short: "Get or set client config",
	Long:  `A longer description`,
	Run:   clientctl,
}

var (
	allOptions  = []string{"global_download_speed_limit", "global_upload_speed_limit"}
	sizeOptions = []string{"global_download_speed_limit", "global_upload_speed_limit"}
	showRaw     = false
)

func init() {
	command.Flags().BoolVar(&showRaw, "raw", false, "show raw config data")
	cmd.RootCmd.AddCommand(command)
}

func clientctl(cmd *cobra.Command, args []string) {
	client, err := client.CreateClient(args[0])
	if err != nil {
		log.Fatal(err)
	}
	args = args[1:]
	exit := 0

	if len(args) == 0 {
		args = allOptions
	}

	for _, variable := range args {
		s := strings.Split(variable, "=")
		name := s[0]
		value := ""
		var err error
		if !slices.Contains(allOptions, name) {
			log.Fatal("Unrecognized option " + name)
		}
		if len(s) == 1 {
			value, err = client.GetConfig(name)
			if err != nil {
				log.Printf("Error get client %s config %s: %v", client.GetName(), name, err)
				exit = 1
			}
		} else {
			value = s[1]
			if slices.Contains(sizeOptions, name) {
				v, _ := utils.RAMInBytes(value)
				err = client.SetConfig(name, fmt.Sprint(v))
			} else {
				err = client.SetConfig(name, value)
			}
			if err != nil {
				log.Printf("Error set client %s config %s=%s: %v", client.GetName(), name, value, err)
				value = ""
				exit = 1
			}
		}
		printOption(name, value)
	}
	os.Exit(exit)
}

func printOption(name string, value string) {
	if value != "" && slices.Contains(sizeOptions, name) {
		ff, _ := utils.RAMInBytes(value)
		if !showRaw {
			fmt.Printf("%s=%s/s\n", name, utils.BytesSize(float64(ff)))
		} else {
			fmt.Printf("%s=%d\n", name, ff)
		}
	} else {
		fmt.Printf("%s=%s\n", name, value)
	}
}
