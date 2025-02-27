package config

import (
	"fmt"
	"os"

	"github.com/spf13/viper"
)

// Config defines the structure of the configuration file
type Config struct {
	ServerPort      string `mapstructure:"server_port,omitempty"`
	NATSURL         string `mapstructure:"nats_url,omitempty"`
	NATSCredentials string `mapstructure:"nats_credentials,omitempty"`
}

// Validate ensures the configuration values are correct
func (c *Config) Validate() error {
	if c.NATSURL == "" {
		return fmt.Errorf("NATS URL cannot be empty")
	}
	return nil
}

// LoadConfig reads configuration from file or environment variables
func LoadConfig() (*Config, error) {
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath("./config/")
	viper.AddConfigPath("~/.config/slingboard/")

	fmt.Println("configfile", viper.GetViper().ConfigFileUsed())

	if err := viper.ReadInConfig(); err != nil {
		fmt.Println("Warning: No configuration file found, using defaults")
	}

	var config Config
	if err := viper.Unmarshal(&config); err != nil {
		return nil, fmt.Errorf("unable to decode config: %w", err)
	}

	// Validate loaded config
	if err := config.Validate(); err != nil {
		return nil, err
	}

	return &config, nil
}

// SaveConfig writes the config struct to the file
func SaveConfig(config *Config) error {
	viper.Set("nats_url", config.NATSURL)
	viper.Set("nats_credentials", config.NATSCredentials)

	if err := viper.WriteConfig(); err != nil {
		if os.IsNotExist(err) {
			return viper.SafeWriteConfig()
		}
		return err
	}
	return nil
}
