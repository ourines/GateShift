package config

import (
	"fmt"
	"net"
	"os"
	"path/filepath"

	"github.com/spf13/viper"
)

// Config holds all configuration for the application
type Config struct {
	ProxyGateway   string `mapstructure:"proxy_gateway"`
	DefaultGateway string `mapstructure:"default_gateway"`
}

// Validate checks if the configuration is valid
func (c *Config) Validate() error {
	if c.DefaultGateway == "" {
		return fmt.Errorf("default gateway is required")
	}
	if c.ProxyGateway == "" {
		return fmt.Errorf("proxy gateway is required")
	}

	// 验证 IP 地址格式
	if net.ParseIP(c.DefaultGateway) == nil {
		return fmt.Errorf("invalid default gateway IP address: %s", c.DefaultGateway)
	}
	if net.ParseIP(c.ProxyGateway) == nil {
		return fmt.Errorf("invalid proxy gateway IP address: %s", c.ProxyGateway)
	}

	return nil
}

// GetConfigDir returns the path to the configuration directory
func GetConfigDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		home = os.Getenv("HOME")
	}
	return filepath.Join(home, ".gateshift")
}

// GetDefaultConfigPath returns the path to the default configuration file
func GetDefaultConfigPath() string {
	return filepath.Join(GetConfigDir(), "config.yaml")
}

// LoadConfig loads the configuration from file or creates default one if it doesn't exist
func LoadConfig() (*Config, error) {
	configDir := GetConfigDir()
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return nil, fmt.Errorf("could not create config directory: %w", err)
	}

	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath(configDir)

	// Set defaults
	viper.SetDefault("proxy_gateway", "192.168.31.100")
	viper.SetDefault("default_gateway", "192.168.31.1")

	// Try to read config file
	if err := viper.ReadInConfig(); err != nil {
		// If config file doesn't exist, create it with defaults
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			configFile := filepath.Join(configDir, "config.yaml")
			if err := viper.SafeWriteConfigAs(configFile); err != nil {
				return nil, fmt.Errorf("could not write default config: %w", err)
			}
		} else {
			return nil, fmt.Errorf("could not read config: %w", err)
		}
	}

	var config Config
	if err := viper.Unmarshal(&config); err != nil {
		return nil, fmt.Errorf("could not unmarshal config: %w", err)
	}

	return &config, nil
}

// SaveConfig saves the configuration to file
func SaveConfig(config *Config) error {
	// 验证配置
	if err := config.Validate(); err != nil {
		return fmt.Errorf("invalid configuration: %w", err)
	}

	// 确保配置目录存在
	configDir := GetConfigDir()
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return fmt.Errorf("could not create config directory: %w", err)
	}

	viper.Set("proxy_gateway", config.ProxyGateway)
	viper.Set("default_gateway", config.DefaultGateway)

	// 如果配置文件不存在，使用 SafeWriteConfigAs
	configFile := viper.ConfigFileUsed()
	if configFile == "" {
		configFile = filepath.Join(configDir, "config.yaml")
		return viper.SafeWriteConfigAs(configFile)
	}

	return viper.WriteConfig()
}
