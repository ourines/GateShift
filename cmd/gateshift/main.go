package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	"github.com/ourines/GateShift/internal/dns"
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

	// Cloudflare URLs for IP lookup
	cloudflareURL     = "https://1.1.1.1/cdn-cgi/trace"
	cloudflareIPv6URL = "https://[2606:4700:4700::1111]/cdn-cgi/trace"

	// Global DNS proxy instance
	dnsProxy *dns.DNSProxy
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
	rootCmd.AddCommand(dnsCmd())

	// Add flags
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.gateshift/config.yaml)")
}

func proxyCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "proxy",
		Short: "Switch to the proxy gateway",
		Long:  `Switch the current active network interface to use the configured proxy gateway.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.LoadConfig()
			if err != nil {
				return err
			}

			err = switchGateway(cfg.ProxyGateway)
			if err != nil {
				return err
			}

			fmt.Println("Switched to proxy gateway successfully")
			fmt.Println("Note: For DNS leak protection, you may want to run: gateshift dns start")

			return nil
		},
	}

	return cmd
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

			err = switchGateway(cfg.DefaultGateway)
			if err != nil {
				return err
			}

			fmt.Println("Switched to default gateway successfully")
			fmt.Println("Note: If DNS proxy is running, you may want to stop it with: gateshift dns stop")

			return nil
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

	reset := &cobra.Command{
		Use:   "reset",
		Short: "Reset all configuration to default values",
		RunE: func(cmd *cobra.Command, args []string) error {
			confirm, _ := cmd.Flags().GetBool("yes")

			if !confirm {
				fmt.Print("Are you sure you want to reset all configuration to default values? [y/N] ")
				var response string
				fmt.Scanln(&response)
				if response != "y" && response != "Y" {
					fmt.Println("Reset cancelled")
					return nil
				}
			}

			cfg, err := config.ResetToDefaults()
			if err != nil {
				return err
			}

			fmt.Println("Configuration reset to default values:")
			fmt.Printf("Proxy Gateway: %s\n", cfg.ProxyGateway)
			fmt.Printf("Default Gateway: %s\n", cfg.DefaultGateway)
			fmt.Printf("DNS Listen Address: %s\n", cfg.DNS.ListenAddr)
			fmt.Printf("DNS Listen Port: %d\n", cfg.DNS.ListenPort)
			fmt.Printf("DNS Upstream Servers: %v\n", cfg.DNS.UpstreamDNS)

			// Stop DNS proxy if it's running
			if dnsProxy != nil && dnsProxy.IsRunning() {
				fmt.Println("Stopping DNS proxy...")
				if err := dnsProxy.Stop(); err != nil {
					fmt.Printf("Warning: Failed to stop DNS proxy: %v\n", err)
				}

				if err := dns.RestoreSystemDNS(); err != nil {
					fmt.Printf("Warning: Failed to restore system DNS: %v\n", err)
				}
			}

			return nil
		},
	}

	reset.Flags().BoolP("yes", "y", false, "Skip confirmation prompt")

	cmd.AddCommand(setProxy, setDefault, show, reset)
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

			// Get public IP address
			publicIP, err := getPublicIP()
			publicIPv6, err6 := getPublicIPv6()

			// Print status information
			fmt.Printf("Active Network Interface: %s\n", iface.Name)
			fmt.Printf("Service Name: %s\n", iface.ServiceName)
			fmt.Printf("IP Address: %s\n", iface.IP)
			fmt.Printf("Subnet Mask: %s\n", iface.Subnet)
			fmt.Printf("Current Gateway: %s\n", iface.Gateway)
			fmt.Printf("Internet Connectivity: %v\n", hasInternet)

			if err == nil {
				fmt.Printf("Public IPv4: %s\n", publicIP)
			} else {
				fmt.Printf("Public IPv4: Not available\n")
			}

			if err6 == nil {
				fmt.Printf("Public IPv6: %s\n", publicIPv6)
			} else {
				fmt.Printf("Public IPv6: Not available\n")
			}

			// DNS Proxy status
			cfg, err := config.LoadConfig()
			if err == nil {
				fmt.Printf("\nDNS Proxy Settings:\n")

				// 使用 isServiceRunning 函数检查服务是否在运行
				running := isServiceRunning()
				if running || (dnsProxy != nil && dnsProxy.IsRunning()) {
					fmt.Printf("  Status: Running\n")
					fmt.Printf("  Listen Address: %s:%d\n", cfg.DNS.ListenAddr, cfg.DNS.ListenPort)
					fmt.Printf("  Upstream DNS: %v\n", cfg.DNS.UpstreamDNS)
				} else {
					fmt.Printf("  Status: Stopped\n")
				}
			}

			return nil
		},
	}
}

// getPublicIP 通过 Cloudflare 获取公网 IPv4 地址
func getPublicIP() (string, error) {
	client := &http.Client{Timeout: 5 * time.Second}
	req, err := http.NewRequest("GET", cloudflareURL, nil)
	if err != nil {
		return "", err
	}

	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	lines := strings.Split(string(body), "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "ip=") {
			return strings.TrimPrefix(line, "ip="), nil
		}
	}

	return "", fmt.Errorf("IP not found in response")
}

// getPublicIPv6 通过 Cloudflare 获取公网 IPv6 地址
func getPublicIPv6() (string, error) {
	client := &http.Client{Timeout: 5 * time.Second}
	req, err := http.NewRequest("GET", cloudflareIPv6URL, nil)
	if err != nil {
		return "", err
	}

	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	lines := strings.Split(string(body), "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "ip=") {
			return strings.TrimPrefix(line, "ip="), nil
		}
	}

	return "", fmt.Errorf("IPv6 not found in response")
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

func dnsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "dns",
		Short: "Manage DNS settings",
		Long:  `Commands for viewing and configuring DNS settings.`,
	}

	// 启动DNS服务
	startDNS := &cobra.Command{
		Use:   "start",
		Short: "Start the DNS proxy service",
		Long:  `Start the DNS proxy service.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// 读取配置
			cfg, err := config.LoadConfig()
			if err != nil {
				return fmt.Errorf("failed to load configuration: %w", err)
			}

			// 检查服务是否已经在运行
			if isServiceRunning() {
				fmt.Println("DNS service is already running.")
				return nil
			}

			// 是否保持在前台运行
			keepForeground, _ := cmd.Flags().GetBool("foreground")

			// 如果需要在后台运行
			if !keepForeground {
				fmt.Println("Starting DNS service in the background...")

				// 获取当前二进制文件的路径
				ex, err := os.Executable()
				if err != nil {
					return fmt.Errorf("failed to get executable path: %w", err)
				}

				// 设置命令行参数
				args := []string{"dns", "start", "--foreground"}

				// 创建一个新的进程
				attr := &os.ProcAttr{
					Files: []*os.File{nil, nil, nil}, // 标准输入、输出和错误重定向到 /dev/null
				}

				// 启动新进程
				process, err := os.StartProcess(ex, append([]string{ex}, args...), attr)
				if err != nil {
					return fmt.Errorf("failed to start daemon process: %w", err)
				}

				// 进程独立运行
				err = process.Release()
				if err != nil {
					return fmt.Errorf("failed to release daemon process: %w", err)
				}

				fmt.Println("DNS service started successfully in the background")
				return nil
			}

			// 如果是前台运行，或者是从后台启动的子进程

			// 获取用户主目录，用于存放日志文件
			homeDir, err := os.UserHomeDir()
			if err != nil {
				return fmt.Errorf("failed to get home directory: %w", err)
			}

			// 创建日志目录
			logDir := filepath.Join(homeDir, ".gateshift", "logs")
			if err := os.MkdirAll(logDir, 0755); err != nil {
				return fmt.Errorf("failed to create log directory: %w", err)
			}

			// 日志文件路径
			logFilePath := filepath.Join(logDir, "gateshift-dns.log")

			// 创建日志文件
			logFile, err := os.OpenFile(logFilePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
			if err != nil {
				return fmt.Errorf("failed to open log file: %w", err)
			}
			defer logFile.Close()

			// 设置日志输出到文件（保持一份到终端）
			if keepForeground {
				// 如果是前台运行，同时输出到终端和日志文件
				log.SetOutput(io.MultiWriter(os.Stdout, logFile))
			} else {
				// 如果是后台运行，只输出到日志文件
				log.SetOutput(logFile)
			}

			log.Printf("DNS service started at %s", time.Now().Format(time.RFC3339))

			// 设置信号处理
			sigChan := make(chan os.Signal, 1)
			signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

			// 启动 DNS 代理
			log.Printf("Starting DNS proxy on %s:%d", cfg.DNS.ListenAddr, cfg.DNS.ListenPort)
			dnsProxy, err = dns.NewDNSProxy(cfg.DNS.ListenAddr, cfg.DNS.ListenPort, cfg.DNS.UpstreamDNS)
			if err != nil {
				log.Printf("Failed to create DNS proxy: %v", err)
				return fmt.Errorf("failed to create DNS proxy: %w", err)
			}

			// 启动 DNS 代理
			if err := dnsProxy.Start(); err != nil {
				log.Printf("Failed to start DNS proxy: %v", err)
				return fmt.Errorf("failed to start DNS proxy: %w", err)
			}

			// 配置系统 DNS
			log.Printf("Configuring system DNS to use %s:%d", cfg.DNS.ListenAddr, cfg.DNS.ListenPort)

			// 非标准端口的特别提示
			if cfg.DNS.ListenPort != 53 && runtime.GOOS == "darwin" {
				log.Printf("Warning: Using non-standard port %d on macOS", cfg.DNS.ListenPort)
				log.Printf("Some applications may not respect the port setting and will continue using port 53")
			}

			if err := dns.ConfigureSystemDNS(cfg.DNS.ListenAddr, cfg.DNS.ListenPort); err != nil {
				log.Printf("Warning: Failed to configure system DNS: %v", err)
			} else {
				log.Printf("DNS leak protection enabled")
			}

			if keepForeground {
				fmt.Println("DNS service running. Press Ctrl+C to stop.")
			}

			// 等待信号退出
			sig := <-sigChan
			log.Printf("Received signal: %v", sig)

			// 停止 DNS 代理
			if dnsProxy != nil && dnsProxy.IsRunning() {
				log.Printf("Stopping DNS proxy...")
				if err := dnsProxy.Stop(); err != nil {
					log.Printf("Warning: Failed to stop DNS proxy: %v", err)
				} else {
					log.Printf("DNS proxy stopped")
				}

				// 恢复系统 DNS
				if err := dns.RestoreSystemDNS(); err != nil {
					log.Printf("Warning: Failed to restore system DNS: %v", err)
				} else {
					log.Printf("System DNS restored")
				}
			}

			log.Printf("DNS service stopped at %s", time.Now().Format(time.RFC3339))
			return nil
		},
	}

	// 添加前台运行标志
	startDNS.Flags().BoolP("foreground", "f", false, "Run in the foreground (don't detach)")

	// 停止DNS服务
	stopDNS := &cobra.Command{
		Use:   "stop",
		Short: "Stop the running DNS service",
		Long:  `Stop the running DNS proxy service and restore system DNS settings.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// 找到所有gateshift进程
			fmt.Println("Stopping DNS service...")

			// 获取当前二进制文件的路径
			ex, err := os.Executable()
			if err != nil {
				return fmt.Errorf("failed to get executable path: %w", err)
			}

			// 获取所有gateshift进程
			var command string
			switch runtime.GOOS {
			case "darwin", "linux":
				command = "pgrep -f " + filepath.Base(ex)
			case "windows":
				command = "tasklist | findstr " + filepath.Base(ex)
			default:
				return fmt.Errorf("unsupported operating system: %s", runtime.GOOS)
			}

			execCmd := exec.Command("sh", "-c", command)
			output, err := execCmd.Output()
			if err != nil {
				// 没有找到进程，可能已经停止
				fmt.Println("No DNS service is running.")
				return nil
			}

			// 解析输出，获取进程ID
			var pids []string
			for _, line := range strings.Split(string(output), "\n") {
				if line == "" {
					continue
				}

				fields := strings.Fields(line)
				if len(fields) > 0 {
					// 检查是否为DNS服务进程（包含 'dns start' 字样）
					checkCmd := fmt.Sprintf("ps -p %s -o command= | grep 'dns start'", fields[0])
					checkOutput, _ := exec.Command("sh", "-c", checkCmd).Output()
					if len(checkOutput) > 0 {
						pids = append(pids, fields[0])
					}
				}
			}

			if len(pids) == 0 {
				fmt.Println("No DNS service found.")
				return nil
			}

			// 发送SIGTERM信号给每个守护进程
			for _, pid := range pids {
				fmt.Printf("Stopping DNS service (PID: %s)...\n", pid)

				// 根据操作系统执行相应的终止命令
				var err error
				switch runtime.GOOS {
				case "darwin", "linux":
					killExecCmd := exec.Command("sudo", "kill", "-SIGTERM", pid)
					err = killExecCmd.Run()
				case "windows":
					killExecCmd := exec.Command("taskkill", "/F", "/PID", pid)
					err = killExecCmd.Run()
				}

				if err != nil {
					return fmt.Errorf("failed to stop DNS service (PID: %s): %w", pid, err)
				}
			}

			fmt.Println("DNS service stopped successfully. System DNS settings restored.")
			return nil
		},
	}

	// 重启DNS服务
	restartDNS := &cobra.Command{
		Use:   "restart",
		Short: "Restart the DNS proxy service",
		Long:  `Restart the running DNS proxy service.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// 先停止服务
			stopCmd := exec.Command(os.Args[0], "dns", "stop")
			stopCmd.Stdout = os.Stdout
			stopCmd.Stderr = os.Stderr
			if err := stopCmd.Run(); err != nil {
				return fmt.Errorf("failed to stop DNS service: %w", err)
			}

			// 短暂等待以确保服务完全停止
			time.Sleep(1 * time.Second)

			// 再启动服务
			startCmd := exec.Command(os.Args[0], "dns", "start")
			startCmd.Stdout = os.Stdout
			startCmd.Stderr = os.Stderr
			if err := startCmd.Run(); err != nil {
				return fmt.Errorf("failed to start DNS service: %w", err)
			}

			return nil
		},
	}

	// 设置上游DNS服务器
	setUpstream := &cobra.Command{
		Use:   "set-upstream [dns-server-ips...]",
		Short: "Set the upstream DNS servers",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.LoadConfig()
			if err != nil {
				return fmt.Errorf("failed to load configuration: %w", err)
			}

			// 确保每个服务器地址都有端口号
			upstreamServers := make([]string, len(args))
			for i, server := range args {
				// 检查是否已包含端口号
				if !strings.Contains(server, ":") {
					// 默认添加端口53
					upstreamServers[i] = server + ":53"
				} else {
					upstreamServers[i] = server
				}
			}

			cfg.DNS.UpstreamDNS = upstreamServers
			if err := config.SaveConfig(cfg); err != nil {
				return fmt.Errorf("failed to save configuration: %w", err)
			}

			fmt.Printf("Upstream DNS servers set to: %v\n", upstreamServers)
			fmt.Println("Restart the DNS service to apply changes: gateshift dns restart")
			return nil
		},
	}

	// 设置DNS代理监听地址
	setListenAddr := &cobra.Command{
		Use:   "set-address [ip-address]",
		Short: "Set the DNS proxy listening address",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			// 验证输入的是有效的IP地址
			ip := net.ParseIP(args[0])
			if ip == nil {
				return fmt.Errorf("invalid IP address: %s", args[0])
			}

			cfg, err := config.LoadConfig()
			if err != nil {
				return fmt.Errorf("failed to load configuration: %w", err)
			}

			cfg.DNS.ListenAddr = args[0]
			if err := config.SaveConfig(cfg); err != nil {
				return fmt.Errorf("failed to save configuration: %w", err)
			}

			fmt.Printf("DNS proxy listen address set to: %s\n", args[0])
			fmt.Println("Restart the DNS service to apply changes: gateshift dns restart")
			return nil
		},
	}

	// 设置DNS代理监听端口
	setPort := &cobra.Command{
		Use:   "set-port [port-number]",
		Short: "Set the DNS proxy listening port",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			port, err := strconv.Atoi(args[0])
			if err != nil {
				return fmt.Errorf("invalid port number: %w", err)
			}

			if port < 1 || port > 65535 {
				return fmt.Errorf("port number must be between 1 and 65535")
			}

			cfg, err := config.LoadConfig()
			if err != nil {
				return fmt.Errorf("failed to load configuration: %w", err)
			}

			cfg.DNS.ListenPort = port
			if err := config.SaveConfig(cfg); err != nil {
				return fmt.Errorf("failed to save configuration: %w", err)
			}

			fmt.Printf("DNS proxy port set to: %d\n", port)
			fmt.Println("Restart the DNS service to apply changes: gateshift dns restart")
			return nil
		},
	}

	// 显示DNS配置
	showDNS := &cobra.Command{
		Use:   "show",
		Short: "Show the current DNS settings",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.LoadConfig()
			if err != nil {
				return fmt.Errorf("failed to load configuration: %w", err)
			}

			fmt.Printf("Listen Address: %s\n", cfg.DNS.ListenAddr)
			fmt.Printf("Listen Port: %d\n", cfg.DNS.ListenPort)
			fmt.Printf("Upstream DNS Servers: %v\n", cfg.DNS.UpstreamDNS)

			// 检查DNS服务是否在运行
			status := "Stopped"
			if isServiceRunning() {
				status = "Running"
			} else {
				// 使用系统命令检查端口是否在使用中
				var checkCmd *exec.Cmd
				switch runtime.GOOS {
				case "darwin", "linux":
					checkCmd = exec.Command("sh", "-c", fmt.Sprintf("sudo lsof -i UDP:%d", cfg.DNS.ListenPort))
				case "windows":
					checkCmd = exec.Command("cmd", "/c", fmt.Sprintf("netstat -ano | findstr %d", cfg.DNS.ListenPort))
				}

				if checkCmd != nil {
					output, _ := checkCmd.CombinedOutput()
					if len(output) > 0 && !strings.Contains(string(output), "not found") {
						status = "Running (detected via system check)"
					}
				}
			}

			fmt.Printf("Status: %s\n", status)
			if status == "Stopped" {
				fmt.Println("\nDNS service is not running.")
				fmt.Println("Try running 'gateshift dns start' to start it.")

				// 建议一些常见问题的解决方案
				fmt.Println("\nPossible issues:")
				fmt.Println("1. The DNS proxy might need elevated privileges to bind to port", cfg.DNS.ListenPort)
				fmt.Println("2. Another program might be using port", cfg.DNS.ListenPort)
				fmt.Println("3. The DNS proxy might have crashed")

				// 如果端口低于1024，提供额外建议
				if cfg.DNS.ListenPort < 1024 {
					fmt.Println("\nTip: Port numbers below 1024 require elevated privileges.")
					fmt.Println("Consider using a higher port number with 'gateshift dns set-port 10053'")
				}
			}

			return nil
		},
	}

	// 查看DNS日志
	showLogs := &cobra.Command{
		Use:   "logs",
		Short: "Show DNS proxy logs",
		Long:  `Display the DNS proxy logs to monitor DNS queries and responses.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// 获取用户主目录
			homeDir, err := os.UserHomeDir()
			if err != nil {
				return fmt.Errorf("failed to get home directory: %w", err)
			}

			// 日志文件路径
			logFile := filepath.Join(homeDir, ".gateshift", "logs", "gateshift-dns.log")

			// 检查日志文件是否存在
			if _, err := os.Stat(logFile); os.IsNotExist(err) {
				return fmt.Errorf("DNS log file not found. Make sure the DNS service is running first")
			}

			// 获取命令行参数
			lines, _ := cmd.Flags().GetInt("lines")
			follow, _ := cmd.Flags().GetBool("follow")
			filter, _ := cmd.Flags().GetString("filter")

			// 使用 grep 和 tail 命令查看日志
			var command string

			if follow {
				// 实时查看日志
				if filter != "" {
					// 使用 grep 过滤，添加 -i 参数使搜索不区分大小写
					command = fmt.Sprintf("tail -f -n %d %s | grep -i --line-buffered %s",
						lines, logFile, filter)
				} else {
					command = fmt.Sprintf("tail -f -n %d %s", lines, logFile)
				}

				fmt.Printf("Showing last %d lines of DNS logs", lines)
				if filter != "" {
					fmt.Printf(" (filtered by '%s', case-insensitive)", filter)
				}
				fmt.Println(". Press Ctrl+C to exit.")
			} else {
				// 查看指定行数
				if filter != "" {
					// 使用 grep 过滤，添加 -i 参数使搜索不区分大小写
					command = fmt.Sprintf("grep -i %s %s | tail -n %d",
						filter, logFile, lines)
				} else {
					command = fmt.Sprintf("tail -n %d %s", lines, logFile)
				}
			}

			// 创建命令
			execCmd := exec.Command("sh", "-c", command)
			execCmd.Stdout = os.Stdout
			execCmd.Stderr = os.Stderr

			return execCmd.Run()
		},
	}

	// 添加标志
	showLogs.Flags().IntP("lines", "n", 50, "Number of lines to show from the end of the log file")
	showLogs.Flags().BoolP("follow", "f", false, "Follow the log file (similar to 'tail -f')")
	showLogs.Flags().StringP("filter", "F", "", "Filter log entries (e.g., domain name, IP address, case-insensitive)")

	// 添加所有命令
	cmd.AddCommand(startDNS, stopDNS, restartDNS, setUpstream, setListenAddr, showDNS, setPort, showLogs)
	return cmd
}

// 帮助函数：检查DNS服务是否正在运行
func isServiceRunning() bool {
	// 获取当前二进制文件的路径
	ex, err := os.Executable()
	if err != nil {
		return false
	}

	// 构建命令用于查找包含"dns start"的进程
	var command string
	switch runtime.GOOS {
	case "darwin", "linux":
		command = fmt.Sprintf("pgrep -f '%s dns start'", filepath.Base(ex))
	case "windows":
		command = fmt.Sprintf("tasklist | findstr %s | findstr \"dns start\"", filepath.Base(ex))
	default:
		return false
	}

	// 执行命令
	cmd := exec.Command("sh", "-c", command)
	output, err := cmd.Output()

	// 如果命令执行成功且有输出，表示服务正在运行
	return err == nil && len(output) > 0
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
