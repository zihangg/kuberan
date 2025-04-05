package database

import (
	"fmt"
	"os"

	"github.com/joho/godotenv"
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
	if err := godotenv.Load(); err != nil {
		// It's okay if .env doesn't exist, we'll use defaults or environment variables
		fmt.Println("Warning: .env file not found")
	}

	return &Config{
		Host:     getEnv("DB_HOST", "localhost"),
		Port:     getEnv("DB_PORT", "5432"),
		User:     getEnv("DB_USER", "kuberan"),
		Password: getEnv("DB_PASSWORD", "kuberan"),
		DBName:   getEnv("DB_NAME", "kuberan"),
		SSLMode:  getEnv("DB_SSLMODE", "disable"),
	}, nil
}

// DSN returns the PostgreSQL connection string
func (c *Config) DSN() string {
	return fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
		c.Host, c.Port, c.User, c.Password, c.DBName, c.SSLMode)
}

// getEnv retrieves an environment variable or returns a default value
func getEnv(key, defaultValue string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return defaultValue
} 