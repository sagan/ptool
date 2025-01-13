//go:build windows
// +build windows

package osutil

import (
	"os"
	"runtime"

	log "github.com/sirupsen/logrus"
	"golang.org/x/sys/windows"
)

// Windows platform setup / initialization.
func Init() {
	// https://github.com/golang/go/issues/43947
	os.Setenv("NoDefaultCurrentDirectoryInExePath", "1")
	SetupConsole()
}

// Setup windows console: disable quick edit mode.
// From https://stackoverflow.com/questions/71690354/disable-quick-edit-in-golang .
// See https://stackoverflow.com/questions/33883530/why-is-my-command-prompt-freezing-on-windows-10 .
// Known problem: Windows PowerShell 5.1 creates UTF-16 file when piping process's stdout / stderr;
// See: https://github.com/golang/go/issues/65157 .
func SetupConsole() {
	winConsole := windows.Handle(os.Stdin.Fd())
	var mode uint32
	err := windows.GetConsoleMode(winConsole, &mode)
	if err != nil {
		return
	}
	// Disable this mode
	mode &^= windows.ENABLE_QUICK_EDIT_MODE
	// Enable this mode
	mode |= windows.ENABLE_EXTENDED_FLAGS
	windows.SetConsoleMode(winConsole, mode)
}

// Dummy (placeholder).
// The real implentation (on Linux) will fork the child process and exit.
func Fork(removeArg string) {
	log.Fatalf("Fork mode is NOT supported on current platform %s", runtime.GOOS)
}
