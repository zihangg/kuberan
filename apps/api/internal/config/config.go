package config

import (
	"log"
	"os"
	"time"

	"github.com/joho/godotenv"
)

// Config holds application configuration
type Config struct {
	// Server
	Port string

	// Database
	DBHost     string
	DBPort     string
	DBUser     string
	DBPassword string
	DBName     string
	DBSSLMode  string

	// JWT
	JWTSecret        string
	JWTExpirationDur time.Duration
}

var appConfig *Config

// Load loads configuration from environment variables
func Load() (*Config, error) {
	// Load .env file if not already loaded
	if err := godotenv.Load(); err != nil {
		log.Println("Warning: .env file not found")
	}

	// Get values from environment variables with defaults
	config := &Config{
		// Server
		Port: getEnv("PORT", "8080"),

		// Database
		DBHost:     getEnv("DB_HOST", "localhost"),
		DBPort:     getEnv("DB_PORT", "5432"),
		DBUser:     getEnv("DB_USER", "kuberan"),
		DBPassword: getEnv("DB_PASSWORD", "kuberan"),
		DBName:     getEnv("DB_NAME", "kuberan"),
		DBSSLMode:  getEnv("DB_SSLMODE", "disable"),

		// JWT
		JWTSecret: getEnv("JWT_SECRET", "fallback-secret-key-for-dev-only"),
	}

	// Parse JWT expiration duration
	expStr := getEnv("JWT_EXPIRES_IN", "24h")
	expDur, err := time.ParseDuration(expStr)
	if err != nil {
		log.Printf("Warning: invalid JWT_EXPIRES_IN value '%s', falling back to 24h\n", expStr)
		expDur = 24 * time.Hour
	}
	config.JWTExpirationDur = expDur

	appConfig = config
	return config, nil
}

// Get returns the application configuration
func Get() *Config {
	if appConfig == nil {
		var err error
		appConfig, err = Load()
		if err != nil {
			log.Fatalf("Failed to load configuration: %v", err)
		}
	}
	return appConfig
}

// getEnv retrieves an environment variable or returns a default value
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
} 