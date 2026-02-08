// Package logger provides structured logging using Zap.
package logger

import (
	"sync"

	"go.uber.org/zap"
)

var (
	sugar *zap.SugaredLogger
	once  sync.Once
)

// Init initializes the global logger for the given environment.
// For "production", it uses a JSON encoder. For all other environments,
// it uses a human-readable console encoder.
func Init(env string) {
	once.Do(func() {
		var base *zap.Logger
		var err error

		if env == "production" {
			base, err = zap.NewProduction()
		} else {
			base, err = zap.NewDevelopment()
		}

		if err != nil {
			// Fallback to nop logger if initialization fails.
			base = zap.NewNop()
		}

		sugar = base.Sugar()
	})
}

// Get returns the global sugared logger.
// If Init has not been called, it initializes a development logger.
func Get() *zap.SugaredLogger {
	if sugar == nil {
		Init("development")
	}
	return sugar
}

// Sync flushes any buffered log entries. Call this before application exit.
func Sync() {
	if sugar != nil {
		_ = sugar.Sync()
	}
}
