package middleware

import (
	"net/http"
	"time"
)

// Recorder is implemented by *metrics.Registry. Declared here as a
// single-method interface (rather than importing the metrics package)
// to keep this middleware decoupled from how observations are stored.
type Recorder interface {
	Observe(method, route string, status int, dur time.Duration)
}

// Metrics records one observation per request into rec, labeled by the
// route pattern mux would dispatch it to (mux.Handler's second return
// value) rather than the raw URL path — path parameters (user/mission
// IDs) would otherwise blow up the number of distinct label
// combinations. Must wrap mux directly, not another middleware, so
// mux.Handler(r) resolves the same pattern mux.ServeHTTP just dispatched.
func Metrics(rec Recorder, mux *http.ServeMux) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			sr := &StatusRecorder{ResponseWriter: w, Status: http.StatusOK}
			next.ServeHTTP(sr, r)

			_, pattern := mux.Handler(r)
			if pattern == "" {
				pattern = "unmatched"
			}
			rec.Observe(r.Method, pattern, sr.Status, time.Since(start))
		})
	}
}
