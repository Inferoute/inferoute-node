package config

import (
	"fmt"
	"strings"

	"github.com/spf13/viper"
)

// Config holds all configuration for the application in a flat structure.
type Config struct {
	Environment string `mapstructure:"environment"`
	ServerPort  int    `mapstructure:"server_port"`
	ServerHost  string `mapstructure:"server_host"`

	DatabaseHost     string `mapstructure:"database_host"`
	DatabasePort     int    `mapstructure:"database_port"`
	DatabaseUser     string `mapstructure:"database_user"`
	DatabasePassword string `mapstructure:"database_password"`
	DatabaseDBName   string `mapstructure:"database_dbname"`
	DatabaseSSLMode  string `mapstructure:"database_sslmode"`

	RabbitMQHost     string `mapstructure:"rabbitmq_host"`
	RabbitMQPort     int    `mapstructure:"rabbitmq_port"`
	RabbitMQUser     string `mapstructure:"rabbitmq_user"`
	RabbitMQPassword string `mapstructure:"rabbitmq_password"`
	RabbitMQVHost    string `mapstructure:"rabbitmq_vhost"`

	AuthHMACSecret    string `mapstructure:"auth_hmacsecret"`
	AuthTokenDuration int    `mapstructure:"auth_tokenduration"`
}

// LoadConfig reads configuration from the .env file and environment variables.
func LoadConfig(configPath string) (*Config, error) {
	v := viper.New()

	// Load .env file.
	v.SetConfigFile(".env")
	if err := v.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("error reading .env file: %v", err)
	}

	// Override with environment variables.
	v.AutomaticEnv()

	var config Config
	if err := v.Unmarshal(&config); err != nil {
		return nil, fmt.Errorf("error unmarshaling config: %v", err)
	}

	if err := validateConfig(&config); err != nil {
		return nil, err
	}
	return &config, nil
}

// validateConfig ensures required configuration fields are present.
func validateConfig(cfg *Config) error {
	if cfg.DatabaseHost == "" {
		return fmt.Errorf("database host is required")
	}
	if cfg.DatabasePort == 0 {
		return fmt.Errorf("database port is required")
	}
	if cfg.DatabaseDBName == "" {
		return fmt.Errorf("database name is required")
	}
	if cfg.ServerPort == 0 {
		return fmt.Errorf("server port is required")
	}
	return nil
}

// IsDevelopment returns true if environment is development.
func (c *Config) IsDevelopment() bool {
	return strings.ToLower(c.Environment) == "development"
}

// IsProduction returns true if environment is production.
func (c *Config) IsProduction() bool {
	return strings.ToLower(c.Environment) == "production"
}

// IsTest returns true if environment is test.
func (c *Config) IsTest() bool {
	return strings.ToLower(c.Environment) == "test"
}
