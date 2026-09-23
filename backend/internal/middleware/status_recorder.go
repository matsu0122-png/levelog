package middleware

import "net/http"

// StatusRecorder wraps an http.ResponseWriter to capture the status code
// written, defaulting to 200 if WriteHeader is never called explicitly
// (mirrors net/http's own default). Shared between Logging and Metrics so
// both observe the same status for a given request.
type StatusRecorder struct {
	http.ResponseWriter
	Status int
}

func (r *StatusRecorder) WriteHeader(status int) {
	r.Status = status
	r.ResponseWriter.WriteHeader(status)
}
