package gateway

import (
	"net"
	"testing"
)

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
