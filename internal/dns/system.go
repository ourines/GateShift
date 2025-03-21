package dns

import (
	"fmt"
	"log"
	"os/exec"
	"runtime"

	"github.com/ourines/GateShift/internal/gateway"
)

// ConfigureSystemDNS configures the system to use the DNS proxy
func ConfigureSystemDNS(proxyIP string) error {
	switch runtime.GOOS {
	case "darwin":
		return configureDarwinDNS(proxyIP)
	case "windows":
		return configureWindowsDNS(proxyIP)
	case "linux":
		return configureLinuxDNS(proxyIP)
	default:
		return fmt.Errorf("unsupported operating system: %s", runtime.GOOS)
	}
}

// RestoreSystemDNS restores the system's original DNS settings
func RestoreSystemDNS() error {
	switch runtime.GOOS {
	case "darwin":
		return restoreDarwinDNS()
	case "windows":
		return restoreWindowsDNS()
	case "linux":
		return restoreLinuxDNS()
	default:
		return fmt.Errorf("unsupported operating system: %s", runtime.GOOS)
	}
}

// macOS specific functions
func configureDarwinDNS(dnsServer string) error {
	iface, err := gateway.GetActiveInterface()
	if err != nil {
		return fmt.Errorf("failed to get active interface: %w", err)
	}

	// 注意: macOS的networksetup命令使用标准53端口
	cmd := exec.Command("networksetup", "-setdnsservers", iface.ServiceName, dnsServer)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to set DNS servers: %w, output: %s", err, string(output))
	}

	log.Printf("DNS服务器IP已设置为 %s 在网络接口 %s", dnsServer, iface.ServiceName)
	log.Printf("DNS已配置为使用 %s 在网络接口 %s", dnsServer, iface.ServiceName)

	return nil
}

func restoreDarwinDNS() error {
	iface, err := gateway.GetActiveInterface()
	if err != nil {
		return fmt.Errorf("failed to get active interface: %w", err)
	}

	// Restore DHCP DNS or use empty string to clear custom DNS
	cmd := exec.Command("networksetup", "-setdnsservers", iface.ServiceName, "empty")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to restore DNS servers: %w, output: %s", err, string(output))
	}

	log.Printf("DNS settings restored to default on %s", iface.ServiceName)
	return nil
}

// Windows specific functions
func configureWindowsDNS(dnsServer string) error {
	// Get the name of the active interface
	iface, err := gateway.GetActiveInterface()
	if err != nil {
		return fmt.Errorf("failed to get active interface: %w", err)
	}

	// 注意: Windows的netsh命令使用标准53端口
	cmd := exec.Command("netsh", "interface", "ip", "set", "dns", fmt.Sprintf("name=\"%s\"", iface.Name), "static", dnsServer)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to set DNS servers: %w, output: %s", err, string(output))
	}

	log.Printf("DNS服务器IP已设置为 %s 在网络接口 %s", dnsServer, iface.Name)
	log.Printf("DNS已配置为使用 %s 在网络接口 %s", dnsServer, iface.Name)

	return nil
}

func restoreWindowsDNS() error {
	// Get the name of the active interface
	iface, err := gateway.GetActiveInterface()
	if err != nil {
		return fmt.Errorf("failed to get active interface: %w", err)
	}

	// Use netsh to set DNS servers back to DHCP
	cmd := exec.Command("netsh", "interface", "ip", "set", "dns", fmt.Sprintf("name=\"%s\"", iface.Name), "dhcp")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to restore DNS servers: %w, output: %s", err, string(output))
	}

	log.Printf("DNS settings restored to DHCP on %s", iface.Name)
	return nil
}

// Linux specific functions
func configureLinuxDNS(dnsServer string) error {
	// 注意: Linux的resolv.conf使用标准53端口
	cmd := exec.Command("sh", "-c", fmt.Sprintf("echo 'nameserver %s' > /etc/resolv.conf", dnsServer))
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to set DNS servers: %w, output: %s", err, string(output))
	}

	log.Printf("DNS服务器IP已设置为 %s 在/etc/resolv.conf", dnsServer)
	log.Printf("DNS已配置为使用 %s 在/etc/resolv.conf", dnsServer)

	return nil
}

func restoreLinuxDNS() error {
	// This is a simplified implementation that restores a basic resolv.conf
	// A more robust solution would backup and restore the original file
	cmd := exec.Command("sh", "-c", "echo 'nameserver 8.8.8.8\nnameserver 8.8.4.4' > /etc/resolv.conf")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to restore DNS servers: %w, output: %s", err, string(output))
	}

	log.Printf("DNS settings restored to default")
	return nil
}
