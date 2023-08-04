//go:build !linux || !amd64
// +build !linux !amd64

package osutil

import (
	"runtime"

	log "github.com/sirupsen/logrus"
)

// Dummy (placeholder)
// The real implentation (on Linux) will fork the child process and exit
func Fork(removeArg string) {
	log.Fatalf("Fork mode is NOT supported on current platform %s", runtime.GOOS)
}
