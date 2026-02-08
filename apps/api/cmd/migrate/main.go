package main

import (
	"fmt"
	"os"
	"strconv"

	"kuberan/internal/config"
	"kuberan/internal/logger"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
)

func main() {
	logger.Init(os.Getenv("ENV"))
	defer logger.Sync()

	if err := run(); err != nil {
		logger.Get().Fatalf("Migration error: %v", err)
	}
}

func run() error {
	if len(os.Args) < 2 {
		return fmt.Errorf("usage: migrate <up|down|version> [N]")
	}

	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	dsn := fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=%s",
		cfg.DBUser, cfg.DBPassword, cfg.DBHost, cfg.DBPort, cfg.DBName, cfg.DBSSLMode)

	m, err := migrate.New("file://migrations", dsn)
	if err != nil {
		return fmt.Errorf("failed to create migrate instance: %w", err)
	}
	defer func() {
		srcErr, dbErr := m.Close()
		if srcErr != nil {
			logger.Get().Warnf("migrate source close error: %v", srcErr)
		}
		if dbErr != nil {
			logger.Get().Warnf("migrate database close error: %v", dbErr)
		}
	}()

	command := os.Args[1]

	switch command {
	case "up":
		if err := m.Up(); err != nil && err != migrate.ErrNoChange {
			return fmt.Errorf("migration up failed: %w", err)
		}
		logger.Get().Info("Migrations applied successfully")

	case "down":
		steps := 1
		if len(os.Args) > 2 {
			steps, err = strconv.Atoi(os.Args[2])
			if err != nil {
				return fmt.Errorf("invalid step count: %w", err)
			}
		}
		if err := m.Steps(-steps); err != nil && err != migrate.ErrNoChange {
			return fmt.Errorf("migration down failed: %w", err)
		}
		logger.Get().Infof("Rolled back %d migration(s)", steps)

	case "version":
		version, dirty, err := m.Version()
		if err != nil {
			return fmt.Errorf("failed to get version: %w", err)
		}
		logger.Get().Infof("Version: %d, Dirty: %v", version, dirty)

	default:
		return fmt.Errorf("unknown command: %s (use up, down, or version)", command)
	}

	return nil
}
