package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
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

	// PID file paths
	DNSPIDFile string
)

func init() {
	// 获取用户主目录
	homeDir, err := os.UserHomeDir()
	if err == nil {
		// 创建程序数据目录
		dataDir := filepath.Join(homeDir, ".gateshift")
		if _, err := os.Stat(dataDir); os.IsNotExist(err) {
			os.MkdirAll(dataDir, 0755)
		}

		// 设置PID文件路径
		DNSPIDFile = filepath.Join(dataDir, "dns.pid")
	}

	// Cobra 初始化前检查是否有配置文件路径参数
	for i, arg := range os.Args {
		if arg == "--config" && i+1 < len(os.Args) {
			cfgFile = os.Args[i+1]
			break
		}
	}

	// Add commands
	rootCmd.AddCommand(proxyCmd())
	rootCmd.AddCommand(defaultCmd())
	rootCmd.AddCommand(configCmd())
	rootCmd.AddCommand(statusCmd())
	rootCmd.AddCommand(versionCmd())
	rootCmd.AddCommand(installCmd())
	rootCmd.AddCommand(uninstallCmd())
	rootCmd.AddCommand(upgradeCmd())
	rootCmd.AddCommand(dnsCmd)

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
					fmt.Printf("  Listen Address: %s\n", cfg.DNS.ListenAddr)
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

// dnsCmd represents the dns command
var dnsCmd = &cobra.Command{
	Use:   "dns",
	Short: "Manage DNS proxy service",
	Long:  `This command is used to manage the DNS proxy service, which can intercept DNS requests and forward them to upstream DNS servers.`,
	Run: func(cmd *cobra.Command, args []string) {
		cmd.Help()
	},
}

func init() {
	rootCmd.AddCommand(dnsCmd)

	// show command
	var showCmd = &cobra.Command{
		Use:   "show",
		Short: "Show DNS proxy configuration",
		Long:  `Show the current configuration of the DNS proxy service.`,
		Run: func(cmd *cobra.Command, args []string) {
			// Display DNS configuration
			cfg, err := config.LoadConfig()
			if err != nil {
				fmt.Println("Error loading config:", err)
				return
			}

			fmt.Printf("Listen Address: %s\n", cfg.DNS.ListenAddr)
			fmt.Printf("Upstream DNS Servers: %v\n", cfg.DNS.UpstreamDNS)

			// Check if DNS proxy is running
			if pid := getPID(DNSPIDFile); pid > 0 {
				fmt.Println("Status: Running")
			} else {
				fmt.Println("Status: Stopped")
			}
		},
	}
	dnsCmd.AddCommand(showCmd)

	// start command
	var startForeground bool
	var startCmd = &cobra.Command{
		Use:   "start",
		Short: "Start the DNS proxy service",
		Long:  `Start the DNS proxy service.`,
		Run: func(cmd *cobra.Command, args []string) {
			// Check if DNS proxy is already running
			if pid := getPID(DNSPIDFile); pid > 0 {
				fmt.Println("DNS service is already running")
				return
			}

			// Load configuration
			cfg, err := config.LoadConfig()
			if err != nil {
				fmt.Println("Error loading config:", err)
				return
			}

			if startForeground {
				fmt.Println("Starting DNS service in foreground mode. Press Ctrl+C to stop...")
				startDNSForeground(cfg)
			} else {
				fmt.Println("Starting DNS service in the background...")
				if err := startDNSBackground(cfg); err != nil {
					fmt.Println("Error starting DNS service:", err)
					return
				}
				fmt.Println("DNS service started successfully in the background")
			}
		},
	}
	startCmd.Flags().BoolVarP(&startForeground, "foreground", "f", false, "Run the DNS proxy in the foreground")
	dnsCmd.AddCommand(startCmd)

	// stop command
	var stopCmd = &cobra.Command{
		Use:   "stop",
		Short: "Stop the DNS proxy service",
		Long:  `Stop the DNS proxy service.`,
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Println("Stopping DNS service...")
			if err := stopDNS(); err != nil {
				fmt.Println("Error stopping DNS service:", err)
				return
			}
		},
	}
	dnsCmd.AddCommand(stopCmd)

	// restart command
	var restartCmd = &cobra.Command{
		Use:   "restart",
		Short: "Restart the DNS proxy service",
		Long:  `Restart the DNS proxy service.`,
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Println("Restarting DNS service...")
			// Stop the DNS service first
			if err := stopDNS(); err != nil {
				fmt.Println("Error stopping DNS service:", err)
				return
			}

			// Load configuration
			cfg, err := config.LoadConfig()
			if err != nil {
				fmt.Println("Error loading config:", err)
				return
			}

			// Start the DNS service
			fmt.Println("Starting DNS service...")
			if err := startDNSBackground(cfg); err != nil {
				fmt.Println("Error starting DNS service:", err)
				return
			}
			fmt.Println("DNS service restarted successfully")
		},
	}
	dnsCmd.AddCommand(restartCmd)

	// add-server command
	var addServerCmd = &cobra.Command{
		Use:   "add-server [server]",
		Short: "Add an upstream DNS server",
		Long:  `Add an upstream DNS server to the DNS proxy configuration.`,
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			server := args[0]
			// Ensure the server has a port (default to 53 if not specified)
			if _, _, err := net.SplitHostPort(server); err != nil {
				server = net.JoinHostPort(server, "53")
			}

			// Load configuration
			cfg, err := config.LoadConfig()
			if err != nil {
				fmt.Println("Error loading config:", err)
				return
			}

			// Check if the server already exists
			for _, s := range cfg.DNS.UpstreamDNS {
				if s == server {
					fmt.Printf("Upstream DNS server %s already exists\n", server)
					return
				}
			}

			// Add the server
			cfg.DNS.UpstreamDNS = append(cfg.DNS.UpstreamDNS, server)

			// Save configuration
			if err := config.SaveConfig(cfg); err != nil {
				fmt.Println("Error saving config:", err)
				return
			}

			fmt.Printf("Upstream DNS server %s added successfully\n", server)
			fmt.Println("Restart the DNS service to apply changes: gateshift dns restart")
		},
	}
	dnsCmd.AddCommand(addServerCmd)

	// remove-server command
	var removeServerCmd = &cobra.Command{
		Use:   "remove-server [server]",
		Short: "Remove an upstream DNS server",
		Long:  `Remove an upstream DNS server from the DNS proxy configuration.`,
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			server := args[0]
			// Ensure the server has a port (default to 53 if not specified)
			if _, _, err := net.SplitHostPort(server); err != nil {
				server = net.JoinHostPort(server, "53")
			}

			// Load configuration
			cfg, err := config.LoadConfig()
			if err != nil {
				fmt.Println("Error loading config:", err)
				return
			}

			// Find and remove the server
			found := false
			for i, s := range cfg.DNS.UpstreamDNS {
				if s == server {
					cfg.DNS.UpstreamDNS = append(cfg.DNS.UpstreamDNS[:i], cfg.DNS.UpstreamDNS[i+1:]...)
					found = true
					break
				}
			}

			if !found {
				fmt.Printf("Upstream DNS server %s not found\n", server)
				return
			}

			// Save configuration
			if err := config.SaveConfig(cfg); err != nil {
				fmt.Println("Error saving config:", err)
				return
			}

			fmt.Printf("Upstream DNS server %s removed successfully\n", server)
			fmt.Println("Restart the DNS service to apply changes: gateshift dns restart")
		},
	}
	dnsCmd.AddCommand(removeServerCmd)

	// list-servers command
	var listServersCmd = &cobra.Command{
		Use:   "list-servers",
		Short: "List all upstream DNS servers",
		Long:  `List all upstream DNS servers configured for the DNS proxy service.`,
		Run: func(cmd *cobra.Command, args []string) {
			// Load configuration
			cfg, err := config.LoadConfig()
			if err != nil {
				fmt.Println("Error loading config:", err)
				return
			}

			if len(cfg.DNS.UpstreamDNS) == 0 {
				fmt.Println("No upstream DNS servers configured")
				return
			}

			fmt.Println("Upstream DNS servers:")
			for _, server := range cfg.DNS.UpstreamDNS {
				fmt.Printf("- %s\n", server)
			}
		},
	}
	dnsCmd.AddCommand(listServersCmd)
}

// isServiceRunning 检查DNS服务是否在运行
func isServiceRunning() bool {
	// 从PID文件获取进程ID
	pid := getPID(DNSPIDFile)
	if pid <= 0 {
		return false
	}

	// 检查进程是否存在
	process, err := os.FindProcess(pid)
	if err != nil {
		return false
	}

	// 在类Unix系统上，FindProcess总是成功的，我们需要发送一个信号0来检查进程是否真的存在
	if runtime.GOOS != "windows" {
		err = process.Signal(syscall.Signal(0))
		return err == nil
	}

	// Windows上，我们可以假设如果FindProcess成功，进程就存在
	return true
}

// getPID 从PID文件中读取进程ID
func getPID(pidFile string) int {
	if _, err := os.Stat(pidFile); os.IsNotExist(err) {
		return 0
	}

	data, err := os.ReadFile(pidFile)
	if err != nil {
		return 0
	}

	pid := 0
	fmt.Sscanf(string(data), "%d", &pid)
	return pid
}

// savePID 保存进程ID到PID文件
func savePID(pidFile string, pid int) error {
	return os.WriteFile(pidFile, []byte(fmt.Sprintf("%d", pid)), 0644)
}

// startDNSForeground 在前台启动DNS服务
func startDNSForeground(cfg *config.Config) {
	// 启动DNS代理
	var err error
	dnsProxy, err = dns.NewDNSProxy(cfg.DNS.ListenAddr, cfg.DNS.UpstreamDNS)
	if err != nil {
		fmt.Printf("Error creating DNS proxy: %v\n", err)
		return
	}

	if err := dnsProxy.Start(); err != nil {
		fmt.Printf("Error starting DNS proxy: %v\n", err)
		return
	}

	// 配置系统DNS
	if err := dns.ConfigureSystemDNS(cfg.DNS.ListenAddr); err != nil {
		fmt.Printf("Warning: Failed to configure system DNS: %v\n", err)
	}

	// 保存当前进程PID
	savePID(DNSPIDFile, os.Getpid())

	// 等待中断信号
	fmt.Println("DNS service running. Press Ctrl+C to stop.")
	select {}
}

// startDNSBackground 在后台启动DNS服务
func startDNSBackground(cfg *config.Config) error {
	// 获取当前可执行文件路径
	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to get executable path: %w", err)
	}

	// 获取用户主目录，用于日志文件
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
	logFile := filepath.Join(logDir, "gateshift-dns.log")

	// 打开日志文件
	f, err := os.OpenFile(logFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return fmt.Errorf("failed to open log file: %w", err)
	}

	// 构建命令行参数
	args := []string{"dns", "start", "-f"}

	// 如果有配置文件路径，也传递给子进程
	if cfgFile != "" {
		args = append(args, "--config", cfgFile)
	}

	// 创建新进程
	cmd := exec.Command(exe, args...)
	cmd.Stdout = f
	cmd.Stderr = f

	// 设置新进程独立运行
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setpgid: true,
	}

	// 启动进程
	if err := cmd.Start(); err != nil {
		f.Close()
		return fmt.Errorf("failed to start DNS service: %w", err)
	}

	// 保存PID
	savePID(DNSPIDFile, cmd.Process.Pid)

	// 分离进程
	cmd.Process.Release()

	// 延迟关闭日志文件
	go func() {
		time.Sleep(1 * time.Second)
		f.Close()
	}()

	return nil
}

// stopDNS 停止DNS服务
func stopDNS() error {
	// 读取PID
	pid := getPID(DNSPIDFile)
	if pid <= 0 {
		fmt.Println("No DNS service is running.")
		return nil
	}

	fmt.Printf("Stopping DNS service (PID: %d)...\n", pid)

	// 发送终止信号
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin", "linux":
		cmd = exec.Command("sudo", "kill", fmt.Sprintf("%d", pid))
	case "windows":
		cmd = exec.Command("taskkill", "/F", "/PID", fmt.Sprintf("%d", pid))
	default:
		return fmt.Errorf("unsupported operating system: %s", runtime.GOOS)
	}

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to stop DNS service: %w", err)
	}

	// 删除PID文件
	os.Remove(DNSPIDFile)

	// 恢复系统DNS设置
	if err := dns.RestoreSystemDNS(); err != nil {
		return fmt.Errorf("failed to restore system DNS: %w", err)
	}

	fmt.Println("DNS service stopped successfully. System DNS settings restored.")
	return nil
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
