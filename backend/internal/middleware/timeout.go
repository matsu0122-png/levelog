package middleware

import (
	"context"
	"net/http"
	"time"
)

// Timeout bounds each request's context with a deadline, so downstream work
// that honors ctx cancellation (database queries in particular — every
// repository call in this codebase is issued with *Context variants) is
// aborted instead of holding a connection indefinitely on a stalled
// dependency.
func Timeout(d time.Duration) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx, cancel := context.WithTimeout(r.Context(), d)
			defer cancel()
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
