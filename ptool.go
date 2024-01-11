package main

import (
	"os"
	"runtime"
	_ "time/tzdata"

	_ "github.com/sagan/ptool/client/all"
	"github.com/sagan/ptool/cmd"
	_ "github.com/sagan/ptool/cmd/all"
	_ "github.com/sagan/ptool/site/all"
)

func main() {
	if runtime.GOOS == "windows" {
		// https://github.com/golang/go/issues/43947
		os.Setenv("NoDefaultCurrentDirectoryInExePath", "1")
	}
	cmd.Execute()
}
