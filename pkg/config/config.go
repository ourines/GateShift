package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/viper"
)

// Config holds all configuration for the application
type Config struct {
	ProxyGateway   string `mapstructure:"proxy_gateway"`
	DefaultGateway string `mapstructure:"default_gateway"`
}

// LoadConfig loads the configuration from file or creates default one if it doesn't exist
func LoadConfig() (*Config, error) {
	// Get home directory
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("could not get home directory: %w", err)
	}

	// Create config directory if it doesn't exist
	configDir := filepath.Join(home, ".gateshift")
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return nil, fmt.Errorf("could not create config directory: %w", err)
	}

	configFile := filepath.Join(configDir, "config")

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
			if err := viper.SafeWriteConfigAs(configFile + ".yaml"); err != nil {
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
	viper.Set("proxy_gateway", config.ProxyGateway)
	viper.Set("default_gateway", config.DefaultGateway)

	return viper.WriteConfig()
}
