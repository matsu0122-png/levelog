// Package reqid generates and threads per-request correlation IDs through
// context.Context so logs and error responses can be tied to one request.
package reqid

import (
	"context"
	"crypto/rand"
	"encoding/hex"
)

type ctxKey struct{}

// New generates a random 16-byte hex-encoded request ID.
func New() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "unknown"
	}
	return hex.EncodeToString(b)
}

func WithContext(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, ctxKey{}, id)
}

// FromContext returns the request ID stored by middleware.RequestID, or ""
// if none is present (e.g. outside of an HTTP request).
func FromContext(ctx context.Context) string {
	id, _ := ctx.Value(ctxKey{}).(string)
	return id
}
