//go:build linux && !arm64
// +build linux,!arm64

package osutil

import (
	"os"
	"os/exec"
	"syscall"

	"github.com/sagan/ptool/util"
	log "github.com/sirupsen/logrus"
)

// Run ptool in the background (detached mode).
// Basicly it just forks a new ptool process with same args.
// from https://github.com/golang/go/issues/227 .
func Fork(removeArg string) {
	err := syscall.FcntlFlock(os.Stdout.Fd(), syscall.F_SETLKW, &syscall.Flock_t{
		Type: syscall.F_WRLCK, Whence: 0, Start: 0, Len: 0})
	if err != nil {
		log.Fatalln("Failed to lock stdout:", err)
	}
	if os.Getppid() != 1 {
		// I am the parent, spawn child to run as daemon
		binary, err := exec.LookPath(os.Args[0])
		if err != nil {
			log.Fatalln("Failed to lookup binary:", err)
		}
		args := util.Filter(os.Args, func(arg string) bool {
			return removeArg == "" || arg != removeArg
		})
		_, err = os.StartProcess(binary, args, &os.ProcAttr{Dir: "", Env: nil,
			Files: []*os.File{os.Stdin, os.Stdout, os.Stderr}, Sys: nil})
		if err != nil {
			log.Fatalln("Failed to start process:", err)
		}
		os.Exit(0)
	} else {
		// I am the child, i.e. the daemon, start new session and detach from terminal
		_, err := syscall.Setsid()
		if err != nil {
			log.Fatalln("Failed to create new session:", err)
		}
		file, err := os.OpenFile("/dev/null", os.O_RDWR, 0)
		if err != nil {
			log.Fatalln("Failed to open /dev/null:", err)
		}
		syscall.Dup2(int(file.Fd()), int(os.Stdin.Fd()))
		syscall.Dup2(int(file.Fd()), int(os.Stdout.Fd()))
		syscall.Dup2(int(file.Fd()), int(os.Stderr.Fd()))
		file.Close()
	}
}
