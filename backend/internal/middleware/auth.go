package middleware

import (
	"context"
	"net/http"

	"levelog/backend/internal/apperror"
	"levelog/backend/internal/httpx"
	"levelog/backend/internal/model"
	"levelog/backend/internal/service"
)

const SessionCookieName = "levelog_session"

type contextKey string

const userContextKey contextKey = "levelog_user"

// RequireAuth reads the session cookie, validates it, and stores the
// authenticated user in the request context. It rejects the request with
// 401 if the cookie is missing or the session is invalid/expired.
func RequireAuth(auth *service.AuthService) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			cookie, err := r.Cookie(SessionCookieName)
			if err != nil {
				httpx.WriteError(w, apperror.Unauthorized("認証が必要です"))
				return
			}

			user, err := auth.ValidateSession(r.Context(), cookie.Value)
			if err != nil {
				httpx.WriteError(w, err)
				return
			}

			ctx := context.WithValue(r.Context(), userContextKey, user)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// UserFromContext retrieves the authenticated user stored by RequireAuth.
func UserFromContext(ctx context.Context) *model.User {
	u, _ := ctx.Value(userContextKey).(*model.User)
	return u
}
