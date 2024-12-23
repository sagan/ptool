package main

import (
	_ "time/tzdata"

	_ "github.com/sagan/ptool/client/all"
	"github.com/sagan/ptool/cmd"
	_ "github.com/sagan/ptool/cmd/all"
	_ "github.com/sagan/ptool/site/all"
	"github.com/sagan/ptool/util/osutil"
)

func main() {
	osutil.Init()
	cmd.Execute()
}
