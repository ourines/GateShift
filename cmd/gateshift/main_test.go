package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGetPublicIP(t *testing.T) {
	// 创建一个模拟的 HTTP 服务器
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ip=1.2.3.4\nts=1234567890\nloc=XX"))
	}))
	defer server.Close()

	// 保存原始的 URL
	originalURL := "https://1.1.1.1/cdn-cgi/trace"
	cloudflareURL = server.URL

	// 测试完成后恢复原始 URL
	defer func() {
		cloudflareURL = originalURL
	}()

	// 执行测试
	ip, err := getPublicIP()
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	if ip != "1.2.3.4" {
		t.Errorf("Expected IP '1.2.3.4', got '%s'", ip)
	}
}

func TestGetPublicIPv6(t *testing.T) {
	// 创建一个模拟的 HTTP 服务器
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ip=2001:db8::1\nts=1234567890\nloc=XX"))
	}))
	defer server.Close()

	// 保存原始的 URL
	originalURL := "https://[2606:4700:4700::1111]/cdn-cgi/trace"
	cloudflareIPv6URL = server.URL

	// 测试完成后恢复原始 URL
	defer func() {
		cloudflareIPv6URL = originalURL
	}()

	// 执行测试
	ip, err := getPublicIPv6()
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	if ip != "2001:db8::1" {
		t.Errorf("Expected IP '2001:db8::1', got '%s'", ip)
	}
}

func TestIsSameFile(t *testing.T) {
	tests := []struct {
		name     string
		file1    string
		file2    string
		expected bool
	}{
		{
			name:     "non-existent files",
			file1:    "nonexistent1",
			file2:    "nonexistent2",
			expected: false,
		},
		{
			name:     "same file",
			file1:    "testdata/file1",
			file2:    "testdata/file1",
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isSameFile(tt.file1, tt.file2)
			if result != tt.expected {
				t.Errorf("isSameFile() = %v, want %v", result, tt.expected)
			}
		})
	}
}
