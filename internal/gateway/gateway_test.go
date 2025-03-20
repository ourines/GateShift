package gateway

import (
	"fmt"
	"net"
	"os"
	"os/exec"
	"runtime"
	"testing"
)

// Mock command executor for testing
type mockCmd struct {
	output string
	err    error
}

func (m *mockCmd) Output() ([]byte, error) {
	if m.err != nil {
		return nil, m.err
	}
	return []byte(m.output), nil
}

// Mock exec.Command
func mockExecCommand(command string, args ...string) *exec.Cmd {
	cs := []string{"-test.run=TestHelperProcess", "--", command}
	cs = append(cs, args...)
	cmd := exec.Command(os.Args[0], cs...)
	cmd.Env = []string{"GO_WANT_HELPER_PROCESS=1"}
	return cmd
}

// TestHelperProcess is not a real test, it's used to mock exec.Command
func TestHelperProcess(t *testing.T) {
	if os.Getenv("GO_WANT_HELPER_PROCESS") != "1" {
		return
	}
	defer os.Exit(0)

	args := os.Args
	for len(args) > 0 {
		if args[0] == "--" {
			args = args[1:]
			break
		}
		args = args[1:]
	}
	if len(args) == 0 {
		fmt.Fprintf(os.Stderr, "No command\n")
		os.Exit(2)
	}

	cmd, args := args[0], args[1:]
	switch cmd {
	case "route":
		fmt.Printf("interface: en0\n")
	case "networksetup":
		if args[0] == "-listallhardwareports" {
			fmt.Printf("Hardware Port: Wi-Fi\nDevice: en0\n")
		}
	case "ifconfig":
		fmt.Printf("inet 192.168.1.100 netmask 0xffffff00\n")
	case "netstat":
		fmt.Printf("default 192.168.1.1 en0\n")
	case "ip":
		if args[0] == "route" {
			if args[1] == "show" {
				fmt.Printf("default via 192.168.1.1 dev eth0\n")
			}
		}
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n", cmd)
		os.Exit(2)
	}
}

func TestCheckInternetConnectivity(t *testing.T) {
	result := CheckInternetConnectivity()
	// 这里我们只能测试函数是否正常执行，因为实际的连接状态取决于网络环境
	t.Logf("Internet connectivity check result: %v", result)
}

func TestNetworkInterface_String(t *testing.T) {
	iface := &NetworkInterface{
		Name:        "en0",
		ServiceName: "Wi-Fi",
		IP:          "192.168.1.100",
		Subnet:      "255.255.255.0",
		Gateway:     "192.168.1.1",
	}

	expected := "Interface: en0 (Wi-Fi)\nIP: 192.168.1.100\nSubnet: 255.255.255.0\nGateway: 192.168.1.1"
	if got := iface.String(); got != expected {
		t.Errorf("NetworkInterface.String() = %v, want %v", got, expected)
	}
}

func TestGetActiveInterface(t *testing.T) {
	// Save original exec.Command and restore it after the test
	oldExec := execCommand
	defer func() { execCommand = oldExec }()

	// Replace exec.Command with our mock
	execCommand = mockExecCommand

	iface, err := GetActiveInterface()
	if err != nil {
		t.Fatalf("GetActiveInterface() error = %v", err)
	}

	// Verify the interface details based on the mock data
	switch runtime.GOOS {
	case "darwin":
		if iface.Name != "en0" || iface.ServiceName != "Wi-Fi" {
			t.Errorf("GetActiveInterface() got = %v, want en0/Wi-Fi", iface)
		}
	case "linux":
		if iface.Name != "eth0" {
			t.Errorf("GetActiveInterface() got = %v, want eth0", iface)
		}
	}
}

func TestSwitchGateway(t *testing.T) {
	// Create a test interface
	iface := &NetworkInterface{
		Name:        "test0",
		ServiceName: "Test Interface",
		IP:          "192.168.1.100",
		Subnet:      "255.255.255.0",
		Gateway:     "192.168.1.1",
	}

	// Test switching to a new gateway
	err := SwitchGateway(iface, "192.168.1.2")
	if err != nil {
		// 在测试环境中，我们期望会有错误，因为没有实际的网络接口
		t.Logf("Expected error in test environment: %v", err)
	}
}

func TestParseIPv4(t *testing.T) {
	tests := []struct {
		name    string
		ip      string
		want    net.IP
		wantErr bool
	}{
		{
			name:    "valid IPv4",
			ip:      "192.168.1.1",
			want:    net.ParseIP("192.168.1.1"),
			wantErr: false,
		},
		{
			name:    "invalid IP",
			ip:      "invalid",
			want:    nil,
			wantErr: true,
		},
		{
			name:    "empty IP",
			ip:      "",
			want:    nil,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := net.ParseIP(tt.ip)
			if (got == nil) != tt.wantErr {
				t.Errorf("ParseIP() error = %v, wantErr %v", got == nil, tt.wantErr)
				return
			}
			if !tt.wantErr && !got.Equal(tt.want) {
				t.Errorf("ParseIP() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIsPrivateIP(t *testing.T) {
	tests := []struct {
		name     string
		ip       string
		expected bool
	}{
		{
			name:     "private IPv4 10.x.x.x",
			ip:       "10.0.0.1",
			expected: true,
		},
		{
			name:     "private IPv4 192.168.x.x",
			ip:       "192.168.1.1",
			expected: true,
		},
		{
			name:     "private IPv4 172.16.x.x",
			ip:       "172.16.0.1",
			expected: true,
		},
		{
			name:     "public IPv4",
			ip:       "8.8.8.8",
			expected: false,
		},
		{
			name:     "invalid IP",
			ip:       "invalid",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ip := net.ParseIP(tt.ip)
			if ip == nil && tt.ip != "invalid" {
				t.Fatalf("failed to parse IP: %s", tt.ip)
			}
			if got := IsPrivateIP(ip); got != tt.expected {
				t.Errorf("IsPrivateIP() = %v, want %v", got, tt.expected)
			}
		})
	}
}

// Test OS-specific implementations
func TestOSSpecificImplementations(t *testing.T) {
	// Save original exec.Command and restore it after the test
	oldExec := execCommand
	defer func() { execCommand = oldExec }()

	// Replace exec.Command with our mock
	execCommand = mockExecCommand

	t.Run("macOS", func(t *testing.T) {
		if runtime.GOOS == "darwin" {
			iface, err := getActiveMacInterface()
			if err != nil {
				t.Errorf("getActiveMacInterface() error = %v", err)
				return
			}
			if iface.Name != "en0" || iface.ServiceName != "Wi-Fi" {
				t.Errorf("getActiveMacInterface() got = %v, want en0/Wi-Fi", iface)
			}

			err = switchMacGateway(iface, "192.168.1.2")
			if err != nil {
				t.Logf("Expected error in test environment: %v", err)
			}
		}
	})

	t.Run("Linux", func(t *testing.T) {
		if runtime.GOOS == "linux" {
			iface, err := getActiveLinuxInterface()
			if err != nil {
				t.Errorf("getActiveLinuxInterface() error = %v", err)
				return
			}
			if iface.Name != "eth0" {
				t.Errorf("getActiveLinuxInterface() got = %v, want eth0", iface)
			}

			err = switchLinuxGateway(iface, "192.168.1.2")
			if err != nil {
				t.Logf("Expected error in test environment: %v", err)
			}
		}
	})

	t.Run("Windows", func(t *testing.T) {
		if runtime.GOOS == "windows" {
			iface, err := getActiveWindowsInterface()
			if err != nil {
				t.Errorf("getActiveWindowsInterface() error = %v", err)
				return
			}
			if iface.Name == "" {
				t.Error("getActiveWindowsInterface() got empty interface name")
			}

			err = switchWindowsGateway(iface, "192.168.1.2")
			if err != nil {
				t.Logf("Expected error in test environment: %v", err)
			}
		}
	})
}
