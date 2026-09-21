package handler

import (
	"net/http"
	"time"

	"levelog/backend/internal/config"
	"levelog/backend/internal/httpx"
	"levelog/backend/internal/middleware"
	"levelog/backend/internal/service"
)

type AuthHandler struct {
	auth *service.AuthService
	cfg  config.Config
}

func NewAuthHandler(auth *service.AuthService, cfg config.Config) *AuthHandler {
	return &AuthHandler{auth: auth, cfg: cfg}
}

type registerRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
	Timezone string `json:"timezone"`
}

type userResponse struct {
	ID       string `json:"id"`
	Email    string `json:"email"`
	Timezone string `json:"timezone"`
}

func (h *AuthHandler) setSessionCookie(w http.ResponseWriter, token string) {
	http.SetCookie(w, &http.Cookie{
		Name:     middleware.SessionCookieName,
		Value:    token,
		Path:     "/",
		Domain:   h.cfg.CookieDomain,
		HttpOnly: true,
		Secure:   h.cfg.CookieSecure,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   int((30 * 24 * time.Hour).Seconds()),
	})
}

func (h *AuthHandler) clearSessionCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     middleware.SessionCookieName,
		Value:    "",
		Path:     "/",
		Domain:   h.cfg.CookieDomain,
		HttpOnly: true,
		Secure:   h.cfg.CookieSecure,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   -1,
	})
}

func (h *AuthHandler) Register(w http.ResponseWriter, r *http.Request) {
	var req registerRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.WriteError(w, err)
		return
	}
	user, err := h.auth.Register(r.Context(), req.Email, req.Password, req.Timezone)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}

	token, _, err := h.auth.Login(r.Context(), req.Email, req.Password)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	h.setSessionCookie(w, token)
	httpx.WriteJSON(w, http.StatusCreated, userResponse{ID: user.ID, Email: user.Email, Timezone: user.Timezone})
}

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.WriteError(w, err)
		return
	}
	token, user, err := h.auth.Login(r.Context(), req.Email, req.Password)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	h.setSessionCookie(w, token)
	httpx.WriteJSON(w, http.StatusOK, userResponse{ID: user.ID, Email: user.Email, Timezone: user.Timezone})
}

func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	if cookie, err := r.Cookie(middleware.SessionCookieName); err == nil {
		_ = h.auth.Logout(r.Context(), cookie.Value)
	}
	h.clearSessionCookie(w)
	httpx.WriteJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (h *AuthHandler) Me(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromContext(r.Context())
	httpx.WriteJSON(w, http.StatusOK, userResponse{ID: user.ID, Email: user.Email, Timezone: user.Timezone})
}
