//go:build !linux && !windows
// +build !linux,!windows

package osutil

import (
	"runtime"

	log "github.com/sirupsen/logrus"
)

// Non Linux/Window platforms setup / initialization.
func Init() {
}

// Dummy (placeholder).
// The real implentation (on Linux) will fork the child process and exit.
func Fork(removeArg string) {
	log.Fatalf("Fork mode is NOT supported on current platform %s", runtime.GOOS)
}
