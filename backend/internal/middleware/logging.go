package middleware

import (
	"log/slog"
	"net/http"
	"time"

	"levelog/backend/internal/reqid"
)

// Logging logs one structured JSON line per request via slog (method, path,
// status, duration, request ID), correlating with the ID set by RequestID
// and with any error logged through httpx.WriteError for the same request.
func Logging(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			rec := &StatusRecorder{ResponseWriter: w, Status: http.StatusOK}
			next.ServeHTTP(rec, r)
			logger.LogAttrs(r.Context(), slog.LevelInfo, "http_request",
				slog.String("request_id", reqid.FromContext(r.Context())),
				slog.String("method", r.Method),
				slog.String("path", r.URL.Path),
				slog.Int("status", rec.Status),
				slog.Int64("duration_ms", time.Since(start).Milliseconds()),
			)
		})
	}
}
