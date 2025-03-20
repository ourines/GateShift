package gateway

import (
	"fmt"
	"net"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/ourines/GateShift/internal/utils"
)

// Mock command executor for testing
var execCommand = exec.Command

// NetworkInterface represents information about a network interface
type NetworkInterface struct {
	Name        string
	ServiceName string
	IP          string
	Subnet      string
	Gateway     string
}

// Initialize sudo session with 15-minute timeout
var sudoSession = utils.NewSudoSession(15 * time.Minute)

// GetActiveInterface returns the currently active network interface
func GetActiveInterface() (*NetworkInterface, error) {
	switch runtime.GOOS {
	case "darwin":
		return getActiveMacInterface()
	case "linux":
		return getActiveLinuxInterface()
	case "windows":
		return getActiveWindowsInterface()
	default:
		return nil, fmt.Errorf("unsupported operating system: %s", runtime.GOOS)
	}
}

// SwitchGateway changes the gateway for the active network interface
func SwitchGateway(iface *NetworkInterface, newGateway string) error {
	switch runtime.GOOS {
	case "darwin":
		return switchMacGateway(iface, newGateway)
	case "linux":
		return switchLinuxGateway(iface, newGateway)
	case "windows":
		return switchWindowsGateway(iface, newGateway)
	default:
		return fmt.Errorf("unsupported operating system: %s", runtime.GOOS)
	}
}

// macOS specific implementations
func getActiveMacInterface() (*NetworkInterface, error) {
	// Get active interface name
	cmd := exec.Command("route", "-n", "get", "default")
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to get default route: %w", err)
	}

	outputStr := string(output)
	interfaceLine := ""
	for _, line := range strings.Split(outputStr, "\n") {
		if strings.Contains(line, "interface:") {
			interfaceLine = line
			break
		}
	}

	if interfaceLine == "" {
		return nil, fmt.Errorf("could not find interface line in route output")
	}

	ifaceName := strings.TrimSpace(strings.Split(interfaceLine, ":")[1])

	// Get service name
	cmd = exec.Command("networksetup", "-listallhardwareports")
	output, err = cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to list hardware ports: %w", err)
	}

	outputStr = string(output)
	var serviceName string
	lines := strings.Split(outputStr, "\n")
	for i, line := range lines {
		if strings.Contains(line, "Device: "+ifaceName) && i > 0 {
			serviceLine := lines[i-1]
			if strings.HasPrefix(serviceLine, "Hardware Port:") {
				serviceName = strings.TrimSpace(strings.TrimPrefix(serviceLine, "Hardware Port:"))
				break
			}
		}
	}

	if serviceName == "" {
		return nil, fmt.Errorf("could not find service name for interface %s", ifaceName)
	}

	// Get IP, subnet, and gateway
	cmd = exec.Command("ifconfig", ifaceName)
	output, err = cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to get interface config: %w", err)
	}

	outputStr = string(output)
	var ip, subnet string
	for _, line := range strings.Split(outputStr, "\n") {
		if strings.Contains(line, "inet ") {
			parts := strings.Fields(line)
			if len(parts) >= 4 {
				ip = parts[1]
				// Convert netmask from hex to decimal
				netmaskHex := strings.TrimPrefix(parts[3], "0x")
				if len(netmaskHex) == 8 {
					a, _ := strconv.ParseInt(netmaskHex[0:2], 16, 64)
					b, _ := strconv.ParseInt(netmaskHex[2:4], 16, 64)
					c, _ := strconv.ParseInt(netmaskHex[4:6], 16, 64)
					d, _ := strconv.ParseInt(netmaskHex[6:8], 16, 64)
					subnet = fmt.Sprintf("%d.%d.%d.%d", a, b, c, d)
				}
				break
			}
		}
	}

	if ip == "" || subnet == "" {
		return nil, fmt.Errorf("could not find IP or subnet for interface %s", ifaceName)
	}

	// Get gateway
	cmd = exec.Command("netstat", "-nr")
	output, err = cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to get routing table: %w", err)
	}

	outputStr = string(output)
	var gateway string
	for _, line := range strings.Split(outputStr, "\n") {
		if strings.HasPrefix(line, "default") && strings.Contains(line, ifaceName) {
			parts := strings.Fields(line)
			if len(parts) >= 2 {
				gateway = parts[1]
				break
			}
		}
	}

	if gateway == "" {
		return nil, fmt.Errorf("could not find gateway for interface %s", ifaceName)
	}

	return &NetworkInterface{
		Name:        ifaceName,
		ServiceName: serviceName,
		IP:          ip,
		Subnet:      subnet,
		Gateway:     gateway,
	}, nil
}

func switchMacGateway(iface *NetworkInterface, newGateway string) error {
	// Use networksetup to change the gateway with sudo privileges
	return sudoSession.RunWithPrivileges("networksetup", "-setmanual", iface.ServiceName, iface.IP, iface.Subnet, newGateway)
}

// Linux specific implementations
func getActiveLinuxInterface() (*NetworkInterface, error) {
	// Get active interface name
	interfaces, err := net.Interfaces()
	if err != nil {
		return nil, fmt.Errorf("failed to list interfaces: %w", err)
	}

	for _, iface := range interfaces {
		if iface.Flags&net.FlagUp != 0 && iface.Flags&net.FlagLoopback == 0 {
			addrs, err := iface.Addrs()
			if err != nil {
				continue
			}

			for _, addr := range addrs {
				if ipnet, ok := addr.(*net.IPNet); ok && !ipnet.IP.IsLoopback() && ipnet.IP.To4() != nil {
					// Found an active interface with IPv4
					// Now get the gateway
					cmd := exec.Command("ip", "route", "show", "default")
					output, err := cmd.Output()
					if err != nil {
						return nil, fmt.Errorf("failed to get default route: %w", err)
					}

					outputStr := string(output)
					var gateway string
					for _, line := range strings.Split(outputStr, "\n") {
						if strings.HasPrefix(line, "default via") && strings.Contains(line, iface.Name) {
							parts := strings.Fields(line)
							if len(parts) >= 3 {
								gateway = parts[2]
								break
							}
						}
					}

					if gateway == "" {
						continue // Try next interface if no gateway found
					}

					return &NetworkInterface{
						Name:        iface.Name,
						ServiceName: iface.Name, // Linux doesn't have separate service names
						IP:          ipnet.IP.String(),
						Subnet:      net.IP(ipnet.Mask).String(),
						Gateway:     gateway,
					}, nil
				}
			}
		}
	}

	return nil, fmt.Errorf("no active network interface found")
}

func switchLinuxGateway(iface *NetworkInterface, newGateway string) error {
	// First delete the existing default route with sudo
	if err := sudoSession.RunWithPrivileges("ip", "route", "del", "default"); err != nil {
		return fmt.Errorf("failed to delete default route: %w", err)
	}

	// Add the new default route with sudo
	return sudoSession.RunWithPrivileges("ip", "route", "add", "default", "via", newGateway, "dev", iface.Name)
}

// Windows specific implementations
func getActiveWindowsInterface() (*NetworkInterface, error) {
	// Get interface information
	cmd := exec.Command("netsh", "interface", "ip", "show", "config")
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to get interface config: %w", err)
	}

	outputStr := string(output)
	var (
		currentInterface    string
		ip, subnet, gateway string
	)

	lines := strings.Split(outputStr, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.Contains(line, "Configuration for interface") {
			currentInterface = strings.Trim(strings.Split(line, "\"")[1], "\"")
		}

		if currentInterface != "" {
			if strings.Contains(line, "IP Address:") {
				ip = strings.TrimSpace(strings.TrimPrefix(line, "IP Address:"))
			} else if strings.Contains(line, "Subnet Prefix:") {
				subnet = strings.TrimSpace(strings.Split(line, "(")[0])
				subnet = strings.TrimSpace(strings.TrimPrefix(subnet, "Subnet Prefix:"))
			} else if strings.Contains(line, "Default Gateway:") {
				gateway = strings.TrimSpace(strings.TrimPrefix(line, "Default Gateway:"))
			}

			// If we have all the information, check if the interface is active
			if ip != "" && subnet != "" && gateway != "" {
				// Ping test to verify the interface is active
				pingCmd := exec.Command("ping", "-n", "1", "-w", "1000", "8.8.8.8")
				if pingCmd.Run() == nil {
					return &NetworkInterface{
						Name:        currentInterface,
						ServiceName: currentInterface,
						IP:          ip,
						Subnet:      subnet,
						Gateway:     gateway,
					}, nil
				}

				// Reset for next interface
				currentInterface = ""
				ip, subnet, gateway = "", "", ""
			}
		}
	}

	return nil, fmt.Errorf("no active network interface found")
}

func switchWindowsGateway(iface *NetworkInterface, newGateway string) error {
	// Windows requires administrative privileges to change the gateway
	return sudoSession.RunWithPrivileges("netsh", "interface", "ip", "set", "address",
		fmt.Sprintf("name=\"%s\"", iface.Name), "gateway="+newGateway)
}

// CheckInternetConnectivity verifies if there's internet connectivity
func CheckInternetConnectivity() bool {
	// Ping Google's DNS to check internet connectivity
	cmd := exec.Command("ping", "-c", "1", "-W", "1", "8.8.8.8")
	return cmd.Run() == nil
}

// String returns a string representation of the NetworkInterface
func (n *NetworkInterface) String() string {
	return fmt.Sprintf("Interface: %s (%s)\nIP: %s\nSubnet: %s\nGateway: %s",
		n.Name, n.ServiceName, n.IP, n.Subnet, n.Gateway)
}

// IsPrivateIP checks if an IP address is private
func IsPrivateIP(ip net.IP) bool {
	if ip == nil {
		return false
	}

	// Check if IPv4
	if ip4 := ip.To4(); ip4 != nil {
		// Following RFC 1918
		// 10.0.0.0/8
		if ip4[0] == 10 {
			return true
		}
		// 172.16.0.0/12
		if ip4[0] == 172 && ip4[1] >= 16 && ip4[1] <= 31 {
			return true
		}
		// 192.168.0.0/16
		if ip4[0] == 192 && ip4[1] == 168 {
			return true
		}
	}

	return false
}
