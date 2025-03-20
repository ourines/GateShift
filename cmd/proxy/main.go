package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"time"

	"github.com/spf13/cobra"

	"github.com/user/proxy/internal/gateway"
	"github.com/user/proxy/internal/utils"
	"github.com/user/proxy/pkg/config"
)

var (
	cfgFile string
	rootCmd = &cobra.Command{
		Use:   "proxy",
		Short: "A tool to switch between gateway configurations",
		Long: `Proxy is a cross-platform tool for switching between gateway configurations.
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

	// Add flags
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.proxy/config.yaml)")
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
			fmt.Printf("Proxy Gateway Switcher v%s\n", Version)
			fmt.Printf("Build time: %s\n", BuildTime)
		},
	}
}

func installCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "install",
		Short: "Install the proxy tool system-wide",
		Long:  `Install the proxy tool to make it available system-wide for all users.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Initialize sudo session with 15-minute timeout
			sudoSession := utils.NewSudoSession(15 * time.Minute)

			// Get the executable path
			execPath, err := os.Executable()
			if err != nil {
				return fmt.Errorf("failed to get executable path: %w", err)
			}

			// Get absolute path
			absPath, err := filepath.Abs(execPath)
			if err != nil {
				return fmt.Errorf("failed to get absolute path: %w", err)
			}

			// Define the installation directory based on OS
			var installDir string
			var installPath string

			switch runtime.GOOS {
			case "darwin", "linux":
				installDir = "/usr/local/bin"
				installPath = filepath.Join(installDir, "proxy")
			case "windows":
				// On Windows, we'll use Program Files
				installDir = os.Getenv("ProgramFiles")
				if installDir == "" {
					installDir = "C:\\Program Files"
				}
				installDir = filepath.Join(installDir, "Proxy")
				installPath = filepath.Join(installDir, "proxy.exe")
			default:
				return fmt.Errorf("unsupported operating system: %s", runtime.GOOS)
			}

			fmt.Printf("Installing proxy to %s\n", installPath)

			// Create installation directory if it doesn't exist
			if err := os.MkdirAll(installDir, 0755); err != nil {
				return fmt.Errorf("failed to create installation directory: %w", err)
			}

			// Copy the binary to the installation directory
			switch runtime.GOOS {
			case "darwin", "linux":
				if err := sudoSession.RunWithPrivileges("cp", absPath, installPath); err != nil {
					return fmt.Errorf("failed to copy binary: %w", err)
				}

				// Make it executable
				if err := sudoSession.RunWithPrivileges("chmod", "+x", installPath); err != nil {
					return fmt.Errorf("failed to make binary executable: %w", err)
				}

				fmt.Println("Installation successful!")
				fmt.Println("You can now run 'proxy' from anywhere in your terminal.")
			case "windows":
				// On Windows, we'll create a copy and add to PATH
				if err := os.MkdirAll(installDir, 0755); err != nil {
					return fmt.Errorf("failed to create installation directory: %w", err)
				}

				// Copy the file
				input, err := os.ReadFile(absPath)
				if err != nil {
					return fmt.Errorf("failed to read binary: %w", err)
				}

				if err := os.WriteFile(installPath, input, 0755); err != nil {
					return fmt.Errorf("failed to write binary: %w", err)
				}

				// Add to PATH using PowerShell
				addToPathCmd := fmt.Sprintf(
					"$currentPath = [Environment]::GetEnvironmentVariable('Path', 'Machine'); "+
						"if ($currentPath -notlike '*%s*') { "+
						"[Environment]::SetEnvironmentVariable('Path', $currentPath + ';%s', 'Machine') "+
						"}", installDir, installDir)

				psCmd := exec.Command("powershell", "-Command", addToPathCmd)
				if err := sudoSession.RunWithPrivileges(psCmd.Path, psCmd.Args[1:]...); err != nil {
					fmt.Println("Warning: Failed to add to PATH automatically.")
					fmt.Printf("Please add %s to your PATH manually.\n", installDir)
				}

				fmt.Println("Installation successful!")
				fmt.Println("You may need to restart your terminal or system for the PATH changes to take effect.")
				fmt.Println("After that, you can run 'proxy' from anywhere in your terminal.")
			}

			return nil
		},
	}
}

func uninstallCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "uninstall",
		Short: "Uninstall the proxy tool from the system",
		Long:  `Remove the proxy tool from system-wide installation.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Initialize sudo session with 15-minute timeout
			sudoSession := utils.NewSudoSession(15 * time.Minute)

			// Define the installation path based on OS
			var installPath string
			var installDir string

			switch runtime.GOOS {
			case "darwin", "linux":
				installPath = "/usr/local/bin/proxy"
			case "windows":
				// On Windows, check Program Files
				installDir = os.Getenv("ProgramFiles")
				if installDir == "" {
					installDir = "C:\\Program Files"
				}
				installDir = filepath.Join(installDir, "Proxy")
				installPath = filepath.Join(installDir, "proxy.exe")
			default:
				return fmt.Errorf("unsupported operating system: %s", runtime.GOOS)
			}

			// Check if the binary exists in the installation path
			if _, err := os.Stat(installPath); os.IsNotExist(err) {
				return fmt.Errorf("proxy is not installed at %s", installPath)
			}

			fmt.Printf("Uninstalling proxy from %s\n", installPath)

			// Remove the binary
			switch runtime.GOOS {
			case "darwin", "linux":
				if err := sudoSession.RunWithPrivileges("rm", installPath); err != nil {
					return fmt.Errorf("failed to remove binary: %w", err)
				}

				fmt.Println("Uninstallation successful!")
			case "windows":
				// Remove the binary
				if err := os.Remove(installPath); err != nil {
					return fmt.Errorf("failed to remove binary: %w", err)
				}

				// Try to remove the directory if it's empty
				if err := os.Remove(installDir); err != nil {
					// It's okay if the directory can't be removed (might not be empty)
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

				fmt.Println("Uninstallation successful!")
				fmt.Println("You may need to restart your terminal or system for the PATH changes to take effect.")
			}

			return nil
		},
	}
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
