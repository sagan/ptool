package main

import (
	_ "time/tzdata"

	"github.com/sagan/ptool/cmd"

	_ "github.com/sagan/ptool/client/all"
	_ "github.com/sagan/ptool/cmd/all"
	_ "github.com/sagan/ptool/site/all"
)

func main() {
	cmd.Execute()
}
