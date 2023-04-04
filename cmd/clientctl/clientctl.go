package clientctl

import (
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"

	"github.com/sagan/ptool/client"
	"github.com/sagan/ptool/cmd"
	"github.com/sagan/ptool/utils"
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
	allOptions   = []string{"global_download_limit", "global_upload_limit"}
	speedOptions = []string{"global_download_limit", "global_upload_limit"}
	showRaw      = false
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
				exit = 1
			}
		} else {
			value = s[1]
			err = client.SetConfig(name, value)
			if err != nil {
				value = ""
				exit = 1
			}
		}
		printOption(name, value)
	}
	os.Exit(exit)
}

func printOption(name string, value string) {
	if !showRaw && slices.Contains(speedOptions, name) {
		ff, _ := strconv.ParseFloat(value, 64)
		fmt.Printf("%s=%s/s\n", name, utils.HumanSize(ff))
	} else {
		fmt.Printf("%s=%s\n", name, value)
	}
}
