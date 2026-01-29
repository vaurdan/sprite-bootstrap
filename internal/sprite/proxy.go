package sprite

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"

	"sprite-bootstrap/internal/config"
)

// StartProxy starts a sprite proxy in the background
func StartProxy(spriteName, orgName string, localPort, remotePort int) error {
	// Check if proxy is already running
	if IsProxyRunning(spriteName) {
		return nil // Already running
	}

	// Ensure pids directory exists
	if err := config.EnsurePidsDir(); err != nil {
		return fmt.Errorf("failed to create pids directory: %w", err)
	}

	args := []string{"proxy"}
	if orgName != "" {
		args = append(args, "-o", orgName)
	}
	if spriteName != "" {
		args = append(args, "-s", spriteName)
	}
	args = append(args, fmt.Sprintf("%d:%d", localPort, remotePort))

	cmd := exec.Command(findSpriteBinary(), args...)

	// Detach the process from the parent (platform-specific)
	cmd.SysProcAttr = getSysProcAttr()

	// Start the proxy
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start proxy: %w", err)
	}

	// Save the PID
	pidFile := config.PidFile(spriteName)
	if err := os.WriteFile(pidFile, []byte(strconv.Itoa(cmd.Process.Pid)), 0644); err != nil {
		// Kill the process if we can't save the PID
		cmd.Process.Kill()
		return fmt.Errorf("failed to save proxy PID: %w", err)
	}

	// Release the process so it continues running after we exit
	cmd.Process.Release()

	return nil
}

// StopProxy stops a running sprite proxy
func StopProxy(spriteName string) error {
	pidFile := config.PidFile(spriteName)

	data, err := os.ReadFile(pidFile)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // No proxy running
		}
		return fmt.Errorf("failed to read PID file: %w", err)
	}

	pid, err := strconv.Atoi(string(data))
	if err != nil {
		os.Remove(pidFile)
		return fmt.Errorf("invalid PID in file: %w", err)
	}

	// Find and kill the process
	process, err := os.FindProcess(pid)
	if err != nil {
		os.Remove(pidFile)
		return nil // Process doesn't exist
	}

	if err := killProcess(process); err != nil {
		// Process might already be dead
		os.Remove(pidFile)
		return nil
	}

	os.Remove(pidFile)
	return nil
}

// IsProxyRunning checks if a proxy is running for the given sprite
func IsProxyRunning(spriteName string) bool {
	pidFile := config.PidFile(spriteName)

	data, err := os.ReadFile(pidFile)
	if err != nil {
		return false
	}

	pid, err := strconv.Atoi(string(data))
	if err != nil {
		return false
	}

	return isProcessRunning(pid)
}

// GetProxyPid returns the PID of the running proxy, or 0 if not running
func GetProxyPid(spriteName string) int {
	pidFile := config.PidFile(spriteName)

	data, err := os.ReadFile(pidFile)
	if err != nil {
		return 0
	}

	pid, err := strconv.Atoi(string(data))
	if err != nil {
		return 0
	}

	if !isProcessRunning(pid) {
		return 0
	}

	return pid
}
