package telemetry

import (
	"log/slog"
	"os"
	"strings"
)

// SetupLogger initializes and registers a structured JSON slog logger.
func SetupLogger(levelStr string, env string) *slog.Logger {
	var level slog.Level
	switch strings.ToLower(levelStr) {
	case "debug":
		level = slog.LevelDebug
	case "info":
		level = slog.LevelInfo
	case "warn", "warning":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	default:
		level = slog.LevelInfo
	}

	opts := &slog.HandlerOptions{
		Level:     level,
		AddSource: env == "development",
	}

	handler := slog.NewJSONHandler(os.Stdout, opts)
	logger := slog.New(handler).With(
		slog.String("service", "triage-api"),
		slog.String("env", env),
	)

	slog.SetDefault(logger)
	return logger
}
