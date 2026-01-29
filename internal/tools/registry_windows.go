//go:build windows

package tools

import (
	"os"
	"os/exec"
	"syscall"
)

// setSysProcAttr sets platform-specific process attributes for background processes
func setSysProcAttr(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{
		CreationFlags: syscall.CREATE_NEW_PROCESS_GROUP,
	}
}

// signalTerminate terminates the process on Windows
func signalTerminate(pid int) error {
	process, err := os.FindProcess(pid)
	if err != nil {
		return err
	}
	return process.Kill()
}

// isProcessRunning checks if a process is still running on Windows
func isProcessRunning(pid int) bool {
	process, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	// On Windows, FindProcess always succeeds, so we try to query the process
	// by sending signal 0 which doesn't actually send a signal but checks existence
	err = process.Signal(syscall.Signal(0))
	return err == nil
}
