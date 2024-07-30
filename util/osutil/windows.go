//go:build windows
// +build windows

package osutil

import (
	"os"

	"golang.org/x/sys/windows"
)

func init() {
	SetupConsole()
}

// Setup windows console: disable quick edit mode.
// From https://stackoverflow.com/questions/71690354/disable-quick-edit-in-golang .
// See https://stackoverflow.com/questions/33883530/why-is-my-command-prompt-freezing-on-windows-10 .
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
