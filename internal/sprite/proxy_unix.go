//go:build !windows

package sprite

import (
	"os"
	"syscall"
)

func getSysProcAttr() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{
		Setpgid: true,
	}
}

func killProcess(process *os.Process) error {
	return process.Signal(syscall.SIGTERM)
}

func isProcessRunning(pid int) bool {
	process, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	// On Unix, send signal 0 to check if process exists
	err = process.Signal(syscall.Signal(0))
	return err == nil
}
