package database

import (
	"fmt"

	"kuberan/internal/config"
)

// Config holds database configuration
type Config struct {
	Host     string
	Port     string
	User     string
	Password string
	DBName   string
	SSLMode  string
}

// NewConfig creates a new database configuration
func NewConfig() (*Config, error) {
	// Get application configuration
	appConfig := config.Get()

	return &Config{
		Host:     appConfig.DBHost,
		Port:     appConfig.DBPort,
		User:     appConfig.DBUser,
		Password: appConfig.DBPassword,
		DBName:   appConfig.DBName,
		SSLMode:  appConfig.DBSSLMode,
	}, nil
}

// DSN returns the PostgreSQL connection string
func (c *Config) DSN() string {
	return fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
		c.Host, c.Port, c.User, c.Password, c.DBName, c.SSLMode)
}
