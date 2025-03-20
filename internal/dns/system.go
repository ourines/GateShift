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
	// If using a non-standard port, we need to set up a local resolver
	if port != 53 {
		log.Printf("Warning: Using non-standard DNS port %d on macOS", port)
		log.Printf("The DNS server will be set to %s, but applications will need to be configured to use port %d", dnsServer, port)
		log.Printf("Try using the standard port 53 by running with sudo or setting the port to 53 with 'gateshift dns set-port 53'")
	}

	// Use networksetup to change DNS servers
	cmd := exec.Command("networksetup", "-setdnsservers", iface.ServiceName, dnsServer)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to set DNS servers: %w, output: %s", err, string(output))
	}

	log.Printf("DNS configured to use %s on %s", dnsServer, iface.ServiceName)
	log.Printf("If DNS is not working, try setting port to 53 with 'sudo gateshift dns set-port 53'")
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
