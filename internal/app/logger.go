package app

import (
	"log/slog"
	"os"
)

var logger *slog.Logger

// initLogger initializes the logger based on environment variable
func initLogger() {
	level := slog.LevelInfo
	if envLevel := os.Getenv("LOG_LEVEL"); envLevel != "" {
		switch envLevel {
		case "debug":
			level = slog.LevelDebug
		case "info":
			level = slog.LevelInfo
		case "warn":
			level = slog.LevelWarn
		case "error":
			level = slog.LevelError
		}
	}

	opts := &slog.HandlerOptions{
		Level: level,
	}

	handler := slog.NewTextHandler(os.Stdout, opts)
	logger = slog.New(handler)
}

// init is called when the package is imported
func init() {
	initLogger()
}
