//go:build windows

package sprite

import (
	"os"
	"syscall"
)

const DETACHED_PROCESS = 0x00000008

func getSysProcAttr() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{
		CreationFlags: DETACHED_PROCESS,
	}
}

func killProcess(process *os.Process) error {
	return process.Kill()
}

func isProcessRunning(pid int) bool {
	process, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	// On Windows, FindProcess always succeeds
	// We need to try to send signal 0 to check if process is running
	return process.Signal(syscall.Signal(0)) == nil
}
