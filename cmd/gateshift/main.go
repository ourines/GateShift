package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/ourines/GateShift/internal/gateway"
	"github.com/ourines/GateShift/internal/utils"
	"github.com/ourines/GateShift/pkg/config"
)

var (
	cfgFile string
	rootCmd = &cobra.Command{
		Use:   "gateshift",
		Short: "A tool to switch between gateway configurations",
		Long: `GateShift is a cross-platform tool for switching between gateway configurations.
It allows you to easily switch between your default gateway and a proxy gateway.`,
	}
)

func init() {
	// Add commands
	rootCmd.AddCommand(proxyCmd())
	rootCmd.AddCommand(defaultCmd())
	rootCmd.AddCommand(configCmd())
	rootCmd.AddCommand(statusCmd())
	rootCmd.AddCommand(versionCmd())
	rootCmd.AddCommand(installCmd())
	rootCmd.AddCommand(uninstallCmd())
	rootCmd.AddCommand(upgradeCmd())

	// Add flags
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.gateshift/config.yaml)")
}

func proxyCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "proxy",
		Short: "Switch to the proxy gateway",
		Long:  `Switch the current active network interface to use the configured proxy gateway.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.LoadConfig()
			if err != nil {
				return err
			}

			return switchGateway(cfg.ProxyGateway)
		},
	}
}

func defaultCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "default",
		Short: "Switch to the default gateway",
		Long:  `Switch the current active network interface to use the default gateway.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.LoadConfig()
			if err != nil {
				return err
			}

			return switchGateway(cfg.DefaultGateway)
		},
	}
}

func configCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "Configure gateway settings",
		Long:  `Configure the proxy and default gateway settings.`,
		Run: func(cmd *cobra.Command, args []string) {
			cmd.Help()
		},
	}

	setProxy := &cobra.Command{
		Use:   "set-proxy [gateway-ip]",
		Short: "Set the proxy gateway IP address",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.LoadConfig()
			if err != nil {
				return err
			}

			cfg.ProxyGateway = args[0]
			if err := config.SaveConfig(cfg); err != nil {
				return err
			}

			fmt.Printf("Proxy gateway set to: %s\n", args[0])
			return nil
		},
	}

	setDefault := &cobra.Command{
		Use:   "set-default [gateway-ip]",
		Short: "Set the default gateway IP address",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.LoadConfig()
			if err != nil {
				return err
			}

			cfg.DefaultGateway = args[0]
			if err := config.SaveConfig(cfg); err != nil {
				return err
			}

			fmt.Printf("Default gateway set to: %s\n", args[0])
			return nil
		},
	}

	show := &cobra.Command{
		Use:   "show",
		Short: "Show the current configuration",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.LoadConfig()
			if err != nil {
				return err
			}

			fmt.Printf("Proxy Gateway: %s\n", cfg.ProxyGateway)
			fmt.Printf("Default Gateway: %s\n", cfg.DefaultGateway)
			return nil
		},
	}

	cmd.AddCommand(setProxy, setDefault, show)
	return cmd
}

func statusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show the current network status",
		Long:  `Display information about the current network interface and gateway.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Get the active interface
			iface, err := gateway.GetActiveInterface()
			if err != nil {
				return fmt.Errorf("failed to get active interface: %w", err)
			}

			// Check internet connectivity
			hasInternet := gateway.CheckInternetConnectivity()

			// Print status information
			fmt.Printf("Active Network Interface: %s\n", iface.Name)
			fmt.Printf("Service Name: %s\n", iface.ServiceName)
			fmt.Printf("IP Address: %s\n", iface.IP)
			fmt.Printf("Subnet Mask: %s\n", iface.Subnet)
			fmt.Printf("Current Gateway: %s\n", iface.Gateway)
			fmt.Printf("Internet Connectivity: %v\n", hasInternet)

			return nil
		},
	}
}

func versionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Show the version information",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Printf("Proxy Gateway Switcher v%s\n", strings.TrimPrefix(Version, "v"))
			fmt.Printf("Build time: %s\n", BuildTime)
		},
	}
}

func installCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "install",
		Short: "Install GateShift to system",
		RunE: func(cmd *cobra.Command, args []string) error {
			// 获取当前二进制文件的路径
			ex, err := os.Executable()
			if err != nil {
				return fmt.Errorf("failed to get executable path: %w", err)
			}

			// 检查目标路径是否已存在
			targetPath := "/usr/local/bin/gateshift"
			if _, err := os.Stat(targetPath); err == nil {
				// 如果文件已存在，检查是否是相同文件
				if isSameFile(ex, targetPath) {
					fmt.Println("GateShift is already installed at /usr/local/bin/gateshift")
					fmt.Println("If you want to reinstall, please run 'sudo rm /usr/local/bin/gateshift' first")
					return nil
				}
			}

			fmt.Printf("Installing GateShift to %s\n", targetPath)

			// 复制文件
			if err := copyFile(ex, targetPath); err != nil {
				return fmt.Errorf("failed to copy binary: %w", err)
			}

			// 设置执行权限
			if err := os.Chmod(targetPath, 0755); err != nil {
				return fmt.Errorf("failed to set executable permission: %w", err)
			}

			fmt.Println("Installation completed successfully!")

			// 获取当前工作目录
			cwd, err := os.Getwd()
			if err == nil {
				// 检查当前目录下是否存在二进制文件，如果存在则删除
				binPath := filepath.Join(cwd, filepath.Base(ex))
				if binPath != ex { // 确保不会删除正在运行的文件
					if err := os.Remove(binPath); err == nil {
						fmt.Println("Cleaned up downloaded binary file")
					}
				}
			}

			fmt.Println("\nRequesting elevated privileges for network configuration...")
			return nil
		},
	}
}

// 检查两个文件是否相同
func isSameFile(file1, file2 string) bool {
	info1, err1 := os.Stat(file1)
	info2, err2 := os.Stat(file2)
	if err1 != nil || err2 != nil {
		return false
	}
	return info1.Size() == info2.Size() && info1.ModTime().Equal(info2.ModTime())
}

// 复制文件
func copyFile(src, dst string) error {
	sourceFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer sourceFile.Close()

	destFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer destFile.Close()

	_, err = io.Copy(destFile, sourceFile)
	return err
}

func uninstallCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "uninstall",
		Short: "Uninstall GateShift from the system",
		Long:  `Remove GateShift from system-wide installation.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Initialize sudo session with 15-minute timeout
			sudoSession := utils.NewSudoSession(15 * time.Minute)

			// Get GOPATH
			gopath := os.Getenv("GOPATH")
			if gopath == "" {
				gopath = filepath.Join(os.Getenv("HOME"), "go")
			}
			gopathBin := filepath.Join(gopath, "bin")

			// Define the installation path based on OS
			var installPath string
			var installDir string
			var gopathVersion string

			switch runtime.GOOS {
			case "darwin", "linux":
				installPath = "/usr/local/bin/gateshift"
				gopathVersion = filepath.Join(gopathBin, "gateshift")
			case "windows":
				// On Windows, check Program Files
				installDir = os.Getenv("ProgramFiles")
				if installDir == "" {
					installDir = "C:\\Program Files"
				}
				installDir = filepath.Join(installDir, "GateShift")
				installPath = filepath.Join(installDir, "gateshift.exe")
				gopathVersion = filepath.Join(gopathBin, "gateshift.exe")
			default:
				return fmt.Errorf("unsupported operating system: %s", runtime.GOOS)
			}

			// Check both system and GOPATH installations
			systemInstalled := false
			gopathInstalled := false

			if _, err := os.Stat(installPath); err == nil {
				systemInstalled = true
			}
			if _, err := os.Stat(gopathVersion); err == nil {
				gopathInstalled = true
			}

			if !systemInstalled && !gopathInstalled {
				return fmt.Errorf("GateShift is not installed at %s or %s", installPath, gopathVersion)
			}

			// Remove system installation if exists
			if systemInstalled {
				fmt.Printf("Uninstalling GateShift from %s\n", installPath)

				switch runtime.GOOS {
				case "darwin", "linux":
					if err := sudoSession.RunWithPrivileges("rm", installPath); err != nil {
						return fmt.Errorf("failed to remove binary: %w", err)
					}
				case "windows":
					if err := os.Remove(installPath); err != nil {
						return fmt.Errorf("failed to remove binary: %w", err)
					}

					// Try to remove the directory if it's empty
					if err := os.Remove(installDir); err != nil {
						fmt.Printf("Note: Could not remove directory %s. It may not be empty.\n", installDir)
					}

					// Remove from PATH using PowerShell
					removeFromPathCmd := fmt.Sprintf(
						"$currentPath = [Environment]::GetEnvironmentVariable('Path', 'Machine'); "+
							"if ($currentPath -like '*%s*') { "+
							"$newPath = $currentPath -replace '%s;', '' -replace ';%s', '' -replace '%s'; "+
							"[Environment]::SetEnvironmentVariable('Path', $newPath, 'Machine') "+
							"}", installDir, installDir, installDir, installDir)

					psCmd := exec.Command("powershell", "-Command", removeFromPathCmd)
					if err := sudoSession.RunWithPrivileges(psCmd.Path, psCmd.Args[1:]...); err != nil {
						fmt.Println("Warning: Failed to remove from PATH automatically.")
						fmt.Printf("Please remove %s from your PATH manually if needed.\n", installDir)
					}
				}

				fmt.Println("System-wide installation removed successfully!")
			}

			// Remove GOPATH installation if exists
			if gopathInstalled {
				fmt.Printf("Removing GOPATH installation from %s\n", gopathVersion)
				if err := os.Remove(gopathVersion); err != nil {
					return fmt.Errorf("failed to remove GOPATH binary: %w", err)
				}
				fmt.Println("GOPATH installation removed successfully!")
			}

			if runtime.GOOS == "windows" {
				fmt.Println("You may need to restart your terminal or system for the PATH changes to take effect.")
			}

			return nil
		},
	}
}

func upgradeCmd() *cobra.Command {
	var autoApprove bool

	cmd := &cobra.Command{
		Use:   "upgrade",
		Short: "Check for updates and upgrade GateShift",
		Long: `Check for new versions of GateShift and upgrade if available.
If a new version is found, it will be downloaded and installed automatically.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Remove 'v' prefix from current version if present
			currentVersion := strings.TrimPrefix(Version, "v")
			fmt.Printf("Current version: v%s\n", currentVersion)
			fmt.Println("Checking for updates...")

			// Get latest release info from GitHub
			latestVersion, downloadURL, err := getLatestRelease()
			if err != nil {
				return fmt.Errorf("failed to check for updates: %w", err)
			}

			// Compare versions without 'v' prefix
			if latestVersion == currentVersion {
				fmt.Println("You are already running the latest version!")
				return nil
			}

			fmt.Printf("New version available: v%s\n", latestVersion)

			// Ask for confirmation unless auto-approve is set
			if !autoApprove {
				fmt.Print("Do you want to upgrade? [y/N] ")
				var response string
				fmt.Scanln(&response)
				if response != "y" && response != "Y" {
					fmt.Println("Upgrade cancelled")
					return nil
				}
			}

			// Create temporary directory for download
			tmpDir, err := os.MkdirTemp("", "gateshift-upgrade")
			if err != nil {
				return fmt.Errorf("failed to create temporary directory: %w", err)
			}
			defer os.RemoveAll(tmpDir)

			// Download the new version
			fmt.Println("Downloading new version...")
			binaryPath := filepath.Join(tmpDir, "gateshift")
			if runtime.GOOS == "windows" {
				binaryPath += ".exe"
			}

			if err := downloadFile(downloadURL, binaryPath); err != nil {
				return fmt.Errorf("failed to download new version: %w", err)
			}

			// Make the downloaded file executable
			if runtime.GOOS != "windows" {
				if err := os.Chmod(binaryPath, 0755); err != nil {
					return fmt.Errorf("failed to make binary executable: %w", err)
				}
			}

			// Get current executable path
			execPath, err := os.Executable()
			if err != nil {
				return fmt.Errorf("failed to get executable path: %w", err)
			}

			// Initialize sudo session
			sudoSession := utils.NewSudoSession(15 * time.Minute)

			// Replace the current binary
			fmt.Println("Installing new version...")
			if runtime.GOOS == "windows" {
				// On Windows, we need to rename instead of direct replacement
				backupPath := execPath + ".old"
				if err := os.Rename(execPath, backupPath); err != nil {
					return fmt.Errorf("failed to backup current binary: %w", err)
				}
				if err := os.Rename(binaryPath, execPath); err != nil {
					// Try to restore backup
					os.Rename(backupPath, execPath)
					return fmt.Errorf("failed to install new version: %w", err)
				}
				os.Remove(backupPath)
			} else {
				if err := sudoSession.RunWithPrivileges("cp", binaryPath, execPath); err != nil {
					return fmt.Errorf("failed to install new version: %w", err)
				}
			}

			fmt.Printf("Successfully upgraded to v%s!\n", latestVersion)
			fmt.Println("Please restart GateShift to use the new version.")
			return nil
		},
	}

	cmd.Flags().BoolVarP(&autoApprove, "yes", "y", false, "Automatically approve upgrade without confirmation")
	return cmd
}

func getLatestRelease() (version string, downloadURL string, err error) {
	// GitHub API URL for latest release
	apiURL := "https://api.github.com/repos/ourines/GateShift/releases/latest"

	// Create HTTP client with timeout
	client := &http.Client{Timeout: 10 * time.Second}

	// Make request
	resp, err := client.Get(apiURL)
	if err != nil {
		return "", "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", "", fmt.Errorf("GitHub API returned status %d", resp.StatusCode)
	}

	// Parse response
	var release struct {
		TagName string `json:"tag_name"`
		Assets  []struct {
			Name               string `json:"name"`
			BrowserDownloadURL string `json:"browser_download_url"`
		} `json:"assets"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return "", "", err
	}

	// Remove 'v' prefix from version if present
	version = strings.TrimPrefix(release.TagName, "v")

	// Find the appropriate asset for current platform
	var assetName string
	switch runtime.GOOS {
	case "darwin":
		if runtime.GOARCH == "arm64" {
			assetName = fmt.Sprintf("gateshift-darwin-arm64")
		} else {
			assetName = fmt.Sprintf("gateshift-darwin-amd64")
		}
	case "linux":
		if runtime.GOARCH == "arm64" {
			assetName = fmt.Sprintf("gateshift-linux-arm64")
		} else {
			assetName = fmt.Sprintf("gateshift-linux-amd64")
		}
	case "windows":
		assetName = fmt.Sprintf("gateshift-windows-amd64.exe")
	default:
		return "", "", fmt.Errorf("unsupported platform: %s", runtime.GOOS)
	}

	// Find download URL for the appropriate asset
	for _, asset := range release.Assets {
		if asset.Name == assetName {
			return version, asset.BrowserDownloadURL, nil
		}
	}

	return "", "", fmt.Errorf("no suitable binary found for platform %s/%s", runtime.GOOS, runtime.GOARCH)
}

func downloadFile(url string, filepath string) error {
	// Create HTTP client with timeout
	client := &http.Client{Timeout: 5 * time.Minute}

	// Make request
	resp, err := client.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download failed with status %d", resp.StatusCode)
	}

	// Create output file
	out, err := os.Create(filepath)
	if err != nil {
		return err
	}
	defer out.Close()

	// Create progress bar
	size := resp.ContentLength
	progress := &ProgressWriter{
		Total:     size,
		Writer:    out,
		LastPrint: time.Now(),
	}

	// Copy with progress
	_, err = io.Copy(progress, resp.Body)
	fmt.Println() // New line after progress bar
	return err
}

type ProgressWriter struct {
	Total     int64
	Current   int64
	Writer    io.Writer
	LastPrint time.Time
}

func (pw *ProgressWriter) Write(p []byte) (int, error) {
	n, err := pw.Writer.Write(p)
	pw.Current += int64(n)

	// Update progress every 100ms
	if time.Since(pw.LastPrint) >= 100*time.Millisecond {
		percentage := float64(pw.Current) / float64(pw.Total) * 100
		fmt.Printf("\rDownloading... %.1f%% (%d/%d bytes)", percentage, pw.Current, pw.Total)
		pw.LastPrint = time.Now()
	}

	return n, err
}

func switchGateway(newGateway string) error {
	// Get the active interface
	iface, err := gateway.GetActiveInterface()
	if err != nil {
		return fmt.Errorf("failed to get active interface: %w", err)
	}

	// Check if already using the target gateway
	if iface.Gateway == newGateway {
		fmt.Printf("Already using gateway: %s\n", newGateway)
		return nil
	}

	// Switch to the new gateway
	fmt.Printf("Switching gateway from %s to %s...\n", iface.Gateway, newGateway)
	startTime := time.Now()

	if err := gateway.SwitchGateway(iface, newGateway); err != nil {
		return fmt.Errorf("failed to switch gateway: %w", err)
	}

	elapsed := time.Since(startTime)
	fmt.Printf("Gateway switched successfully (took %v)\n", elapsed.Round(time.Millisecond))

	// Verify internet connectivity
	if gateway.CheckInternetConnectivity() {
		fmt.Println("Internet connectivity confirmed")
	} else {
		fmt.Println("Warning: No internet connectivity detected")
	}

	return nil
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
