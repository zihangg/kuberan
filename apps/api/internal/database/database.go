package database

import (
	"fmt"

	"kuberan/internal/logger"
	"kuberan/internal/models"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// Manager handles database operations
type Manager struct {
	db *gorm.DB
}

// NewManager creates a new database manager
func NewManager(config *Config) (*Manager, error) {
	db, err := gorm.Open(postgres.Open(config.DSN()), &gorm.Config{})
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	return &Manager{db: db}, nil
}

// Migrate runs database migrations
func (m *Manager) Migrate() error {
	logger.Get().Info("Running database migrations...")

	// List all models to migrate
	models := []interface{}{
		&models.User{},
		&models.Account{},
		&models.Category{},
		&models.Transaction{},
		&models.Budget{},
		&models.Investment{},
		&models.InvestmentTransaction{},
	}

	// Run migrations
	for _, model := range models {
		if err := m.db.AutoMigrate(model); err != nil {
			return fmt.Errorf("failed to migrate %T: %w", model, err)
		}
	}

	logger.Get().Info("Database migrations completed successfully")
	return nil
}

// DB returns the underlying GORM database instance
func (m *Manager) DB() *gorm.DB {
	return m.db
} 