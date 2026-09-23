// Package logging configures the process-wide structured (JSON) logger.
package logging

import (
	"log/slog"
	"os"
)

// New builds a JSON slog.Logger writing to stdout at the given level
// ("debug", "info", "warn", "error"; unrecognized values fall back to info).
func New(level string) *slog.Logger {
	var lvl slog.Level
	switch level {
	case "debug":
		lvl = slog.LevelDebug
	case "warn":
		lvl = slog.LevelWarn
	case "error":
		lvl = slog.LevelError
	default:
		lvl = slog.LevelInfo
	}
	handler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: lvl})
	return slog.New(handler)
}
