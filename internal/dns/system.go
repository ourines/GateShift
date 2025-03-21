package dns

import (
	"fmt"
	"log"
	"os/exec"
	"runtime"

	"github.com/ourines/GateShift/internal/gateway"
)

// ConfigureSystemDNS configures the system to use the DNS proxy
func ConfigureSystemDNS(proxyIP string, port int) error {
	switch runtime.GOOS {
	case "darwin":
		return configureDarwinDNS(proxyIP, port)
	case "windows":
		return configureWindowsDNS(proxyIP, port)
	case "linux":
		return configureLinuxDNS(proxyIP, port)
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
func configureDarwinDNS(dnsServer string, port int) error {
	iface, err := gateway.GetActiveInterface()
	if err != nil {
		return fmt.Errorf("failed to get active interface: %w", err)
	}

	// MacOS doesn't support specifying DNS ports through networksetup
	if port != 53 {
		// 当使用非标准端口时，构建完整的地址（IP:Port 格式）
		fullAddress := fmt.Sprintf("%s:%d", dnsServer, port)
		log.Printf("Warning: Using non-standard DNS port %d on macOS", port)
		log.Printf("MacOS does not natively support DNS ports through system settings")
		log.Printf("The following methods are recommended:")
		log.Printf("1. Set applications to use %s as DNS server", fullAddress)
		log.Printf("2. Use port 53 (requires root): sudo gateshift dns set-port 53")
		log.Printf("3. Configure a local resolver that forwards to %s", fullAddress)

		// 尝试设置系统 DNS，但警告用户可能无法使用非标准端口
		cmd := exec.Command("networksetup", "-setdnsservers", iface.ServiceName, dnsServer)
		output, err := cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("failed to set DNS servers: %w, output: %s", err, string(output))
		}

		log.Printf("DNS server address set to %s on %s, but port %d may not be used by all applications",
			dnsServer, iface.ServiceName, port)
		return nil
	}

	// 使用标准端口 53
	cmd := exec.Command("networksetup", "-setdnsservers", iface.ServiceName, dnsServer)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to set DNS servers: %w, output: %s", err, string(output))
	}

	log.Printf("DNS configured to use %s (port 53) on %s", dnsServer, iface.ServiceName)
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
func configureWindowsDNS(dnsServer string, port int) error {
	// Get the name of the active interface
	iface, err := gateway.GetActiveInterface()
	if err != nil {
		return fmt.Errorf("failed to get active interface: %w", err)
	}

	// Windows netsh doesn't support specifying port for DNS
	if port != 53 {
		log.Printf("Warning: Using non-standard DNS port %d on Windows", port)
		log.Printf("The DNS server will be set to %s, but applications will need to be configured to use port %d", dnsServer, port)
		log.Printf("Try using the standard port 53 by running as administrator or setting the port to 53 with 'gateshift dns set-port 53'")
	}

	// Use netsh to set DNS servers
	cmd := exec.Command("netsh", "interface", "ip", "set", "dns", fmt.Sprintf("name=\"%s\"", iface.Name), "static", dnsServer)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to set DNS servers: %w, output: %s", err, string(output))
	}

	log.Printf("DNS configured to use %s on %s", dnsServer, iface.Name)
	log.Printf("If DNS is not working, try setting port to 53 with 'gateshift dns set-port 53' and run as administrator")
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
func configureLinuxDNS(dnsServer string, port int) error {
	// On Linux, we'll update /etc/resolv.conf directly
	// This is a simplified implementation and might need adjustment for different distros

	// Linux resolv.conf doesn't support port specifications directly
	if port != 53 {
		log.Printf("Warning: Using non-standard DNS port %d on Linux", port)
		log.Printf("The DNS server will be set to %s, but applications will use standard port 53", dnsServer)
		log.Printf("Try using the standard port 53 by running with sudo or setting the port to 53 with 'gateshift dns set-port 53'")
	}

	cmd := exec.Command("sh", "-c", fmt.Sprintf("echo 'nameserver %s' > /etc/resolv.conf", dnsServer))
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to set DNS servers: %w, output: %s", err, string(output))
	}

	log.Printf("DNS configured to use %s", dnsServer)
	log.Printf("If DNS is not working, try setting port to 53 with 'sudo gateshift dns set-port 53'")
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
