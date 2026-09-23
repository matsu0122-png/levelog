package middleware

import (
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"golang.org/x/time/rate"

	"levelog/backend/internal/apperror"
	"levelog/backend/internal/httpx"
)

type limiterEntry struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

// RateLimit throttles requests per client IP with a token bucket (refill
// rate r, burst b), responding 429 once a key's bucket is empty. Meant to
// be applied to specific brute-force-sensitive routes (login/register) at
// registration time, not globally — see router.go. Each call returns an
// independent limiter (its own per-IP map), so e.g. wrapping login and
// register separately gives each its own budget.
//
// Limiters live in process memory, not a shared store (Redis etc.): on
// production's 2-app-server deployment, each instance enforces its own
// independent limit, so an attacker able to spread requests across both
// backends sees roughly double a single instance's ceiling. Accepted
// tradeoff for this project's scale — see docs/security-review.md for the
// reasoning — rather than standing up shared infrastructure just for this.
// Idle entries (a key with no requests for 10 minutes) are periodically
// swept so the map doesn't grow unbounded over the process's lifetime.
func RateLimit(r rate.Limit, b int) func(http.Handler) http.Handler {
	var mu sync.Mutex
	entries := make(map[string]*limiterEntry)

	go func() {
		ticker := time.NewTicker(5 * time.Minute)
		defer ticker.Stop()
		for range ticker.C {
			cutoff := time.Now().Add(-10 * time.Minute)
			mu.Lock()
			for key, e := range entries {
				if e.lastSeen.Before(cutoff) {
					delete(entries, key)
				}
			}
			mu.Unlock()
		}
	}()

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			key := clientIP(req)

			mu.Lock()
			e, ok := entries[key]
			if !ok {
				e = &limiterEntry{limiter: rate.NewLimiter(r, b)}
				entries[key] = e
			}
			e.lastSeen = time.Now()
			allowed := e.limiter.Allow()
			mu.Unlock()

			if !allowed {
				httpx.WriteError(w, req, apperror.TooManyRequests("試行回数が多すぎます。しばらくしてから再度お試しください"))
				return
			}
			next.ServeHTTP(w, req)
		})
	}
}

// clientIP prefers the leftmost address in X-Forwarded-For (the original
// client, per convention) over r.RemoteAddr. Trusting this header is safe
// specifically because of this deployment's topology: the api container is
// never reachable except through the web container's Nginx on the same
// docker network (docker-compose.prod.yml — api publishes no port to the
// host), and the Sakura Cloud load balancer in front of that is a plain
// TCP pass-through (docs/tls-design.md), so Nginx is the only process that
// ever sets this header — there's no untrusted hop able to forge it before
// it reaches this middleware.
func clientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		if i := strings.IndexByte(xff, ','); i >= 0 {
			xff = xff[:i]
		}
		if ip := strings.TrimSpace(xff); ip != "" {
			return ip
		}
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
