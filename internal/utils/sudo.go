package utils

import (
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"runtime"
	"time"
)

// SudoSession manages elevated privileges
type SudoSession struct {
	timeout time.Duration
	lastUse time.Time
}

var (
	// Global sudo session
	globalSession *SudoSession
)

// NewSudoSession creates a new sudo session with the specified timeout
func NewSudoSession(timeout time.Duration) *SudoSession {
	if globalSession == nil {
		globalSession = &SudoSession{
			timeout: timeout,
			lastUse: time.Now(),
		}
	}
	return globalSession
}

// RunWithPrivileges runs a command with elevated privileges
func (s *SudoSession) RunWithPrivileges(name string, args ...string) error {
	// Update last use time
	s.lastUse = time.Now()

	switch runtime.GOOS {
	case "darwin", "linux":
		return s.runUnixSudo(name, args...)
	case "windows":
		return s.runWindowsElevated(name, args...)
	default:
		return fmt.Errorf("unsupported operating system: %s", runtime.GOOS)
	}
}

// runUnixSudo runs a command with sudo on Unix-like systems
func (s *SudoSession) runUnixSudo(name string, args ...string) error {
	// Check if we're already running as root
	currentUser, err := user.Current()
	if err != nil {
		return fmt.Errorf("failed to get current user: %w", err)
	}

	// If we're already root, just run the command
	if currentUser.Username == "root" {
		cmd := exec.Command(name, args...)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		return cmd.Run()
	}

	// Create a temporary script to run the command with sudo
	tempDir := os.TempDir()
	scriptPath := filepath.Join(tempDir, fmt.Sprintf("proxy_sudo_%d.sh", time.Now().UnixNano()))

	// Create the script
	script := "#!/bin/bash\n"
	script += fmt.Sprintf("%s %s\n", name, QuoteArgs(args))

	// Write the script to a file
	if err := os.WriteFile(scriptPath, []byte(script), 0700); err != nil {
		return fmt.Errorf("failed to create temporary script: %w", err)
	}
	defer os.Remove(scriptPath) // Clean up

	// Run the script with sudo
	sudoCmd := exec.Command("sudo", "-n", scriptPath)
	sudoCmd.Stdout = os.Stdout
	sudoCmd.Stderr = os.Stderr

	// Try to run without password first (if sudo timeout is still valid)
	if err := sudoCmd.Run(); err == nil {
		return nil
	}

	// If sudo -n failed, we need to ask for a password
	fmt.Println("Requesting elevated privileges for network configuration...")
	sudoCmd = exec.Command("sudo", scriptPath)
	sudoCmd.Stdout = os.Stdout
	sudoCmd.Stderr = os.Stderr
	return sudoCmd.Run()
}

// runWindowsElevated runs a command with elevated privileges on Windows
func (s *SudoSession) runWindowsElevated(name string, args ...string) error {
	// On Windows, we'll use PowerShell's Start-Process with -Verb RunAs
	scriptPath := filepath.Join(os.TempDir(), fmt.Sprintf("proxy_elevated_%d.ps1", time.Now().UnixNano()))

	// Create the PowerShell script
	script := "Start-Process "
	script += fmt.Sprintf("-FilePath '%s' ", name)
	if len(args) > 0 {
		script += fmt.Sprintf("-ArgumentList '%s' ", QuoteArgs(args))
	}
	script += "-Verb RunAs -Wait"

	// Write the script to a file
	if err := os.WriteFile(scriptPath, []byte(script), 0700); err != nil {
		return fmt.Errorf("failed to create temporary script: %w", err)
	}
	defer os.Remove(scriptPath) // Clean up

	// Run the PowerShell script
	cmd := exec.Command("powershell", "-ExecutionPolicy", "Bypass", "-File", scriptPath)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// IsExpired checks if the sudo session has expired
func (s *SudoSession) IsExpired() bool {
	return time.Since(s.lastUse) > s.timeout
}

// QuoteArgs quotes command line arguments for use in scripts
func QuoteArgs(args []string) string {
	quoted := ""
	for i, arg := range args {
		if i > 0 {
			quoted += " "
		}
		// Quote the argument if it contains spaces
		if containsSpace(arg) {
			quoted += fmt.Sprintf("\"%s\"", arg)
		} else {
			quoted += arg
		}
	}
	return quoted
}

func containsSpace(s string) bool {
	for _, r := range s {
		if r == ' ' || r == '\t' || r == '\n' || r == '\r' {
			return true
		}
	}
	return false
}
