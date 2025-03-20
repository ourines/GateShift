package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/viper"
)

func TestConfig_LoadAndSave(t *testing.T) {
	// 创建临时目录
	tmpDir, err := os.MkdirTemp("", "gateshift-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// 保存原始的 HOME 环境变量
	originalHome := os.Getenv("HOME")
	defer os.Setenv("HOME", originalHome)

	// 设置测试用的 HOME 环境变量
	os.Setenv("HOME", tmpDir)

	// 重置 viper 配置
	viper.Reset()

	// 创建测试配置
	testConfig := &Config{
		DefaultGateway: "192.168.1.1",
		ProxyGateway:   "192.168.1.2",
	}

	// 测试保存配置
	if err := SaveConfig(testConfig); err != nil {
		t.Errorf("Failed to save config: %v", err)
	}

	// 测试加载配置
	loadedConfig, err := LoadConfig()
	if err != nil {
		t.Errorf("Failed to load config: %v", err)
	}

	// 验证加载的配置是否正确
	if loadedConfig.DefaultGateway != testConfig.DefaultGateway {
		t.Errorf("DefaultGateway = %v, want %v", loadedConfig.DefaultGateway, testConfig.DefaultGateway)
	}
	if loadedConfig.ProxyGateway != testConfig.ProxyGateway {
		t.Errorf("ProxyGateway = %v, want %v", loadedConfig.ProxyGateway, testConfig.ProxyGateway)
	}
}

func TestConfig_Validate(t *testing.T) {
	tests := []struct {
		name    string
		config  *Config
		wantErr bool
	}{
		{
			name: "valid config",
			config: &Config{
				DefaultGateway: "192.168.1.1",
				ProxyGateway:   "192.168.1.2",
			},
			wantErr: false,
		},
		{
			name: "missing default gateway",
			config: &Config{
				ProxyGateway: "192.168.1.2",
			},
			wantErr: true,
		},
		{
			name: "missing proxy gateway",
			config: &Config{
				DefaultGateway: "192.168.1.1",
			},
			wantErr: true,
		},
		{
			name: "invalid default gateway",
			config: &Config{
				DefaultGateway: "invalid",
				ProxyGateway:   "192.168.1.2",
			},
			wantErr: true,
		},
		{
			name: "invalid proxy gateway",
			config: &Config{
				DefaultGateway: "192.168.1.1",
				ProxyGateway:   "invalid",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Config.Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestGetConfigDir(t *testing.T) {
	// 保存原始的 HOME 环境变量
	originalHome := os.Getenv("HOME")
	defer os.Setenv("HOME", originalHome)

	// 设置测试用的 HOME 环境变量
	testHome := "/tmp/test-home"
	os.Setenv("HOME", testHome)

	expected := filepath.Join(testHome, ".gateshift")
	if got := GetConfigDir(); got != expected {
		t.Errorf("GetConfigDir() = %v, want %v", got, expected)
	}
}

func TestGetDefaultConfigPath(t *testing.T) {
	// 保存原始的 HOME 环境变量
	originalHome := os.Getenv("HOME")
	defer os.Setenv("HOME", originalHome)

	// 设置测试用的 HOME 环境变量
	testHome := "/tmp/test-home"
	os.Setenv("HOME", testHome)

	expected := filepath.Join(testHome, ".gateshift", "config.yaml")
	if got := GetDefaultConfigPath(); got != expected {
		t.Errorf("GetDefaultConfigPath() = %v, want %v", got, expected)
	}
}
