package middleware

import (
	"net/http"

	"levelog/backend/internal/reqid"
)

// RequestIDHeader is the header carrying the request's correlation ID, both
// inbound (reused if already set by an upstream proxy) and outbound.
const RequestIDHeader = "X-Request-Id"

// RequestID assigns every request a correlation ID (reusing one supplied by
// an upstream proxy/load balancer if present), stores it in the request
// context, and echoes it back in the response headers.
func RequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.Header.Get(RequestIDHeader)
		if id == "" {
			id = reqid.New()
		}
		w.Header().Set(RequestIDHeader, id)
		next.ServeHTTP(w, r.WithContext(reqid.WithContext(r.Context(), id)))
	})
}
