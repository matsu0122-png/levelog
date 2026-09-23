package service

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"regexp"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"

	"levelog/backend/internal/apperror"
	"levelog/backend/internal/model"
)

const (
	sessionTokenBytes = 32
	sessionTTL        = 30 * 24 * time.Hour
	minPasswordLength = 8
	defaultTimezone   = "Asia/Tokyo"
)

var emailPattern = regexp.MustCompile(`^[^\s@]+@[^\s@]+\.[^\s@]+$`)

type AuthService struct {
	users    UserRepository
	sessions SessionRepository
	clock    Clock
}

func NewAuthService(users UserRepository, sessions SessionRepository, clock Clock) *AuthService {
	if clock == nil {
		clock = RealClock
	}
	return &AuthService{users: users, sessions: sessions, clock: clock}
}

// Register creates a new user account with a bcrypt-hashed password.
func (s *AuthService) Register(ctx context.Context, email, password, timezone string) (*model.User, error) {
	email = strings.TrimSpace(strings.ToLower(email))
	if !emailPattern.MatchString(email) {
		return nil, apperror.BadRequest("有効なメールアドレスを入力してください")
	}
	if len(password) < minPasswordLength {
		return nil, apperror.BadRequest("パスワードは8文字以上で入力してください")
	}
	if timezone == "" {
		timezone = defaultTimezone
	}
	if _, err := time.LoadLocation(timezone); err != nil {
		return nil, apperror.BadRequest("不正なタイムゾーンです")
	}

	if existing, err := s.users.GetByEmail(ctx, email); err != nil {
		return nil, err
	} else if existing != nil {
		return nil, apperror.Conflict("このメールアドレスは既に使用されています")
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, apperror.Internal("failed to hash password")
	}

	now := s.clock.Now()
	user := &model.User{
		Email:        email,
		PasswordHash: string(hash),
		Timezone:     timezone,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	if err := s.users.Create(ctx, user); err != nil {
		return nil, err
	}
	return user, nil
}

// Login verifies credentials and creates a new session, returning the raw
// session token (to be set as an httpOnly cookie by the handler) and the
// authenticated user.
func (s *AuthService) Login(ctx context.Context, email, password string) (token string, user *model.User, err error) {
	email = strings.TrimSpace(strings.ToLower(email))
	user, err = s.users.GetByEmail(ctx, email)
	if err != nil {
		return "", nil, err
	}
	if user == nil {
		return "", nil, apperror.Unauthorized("メールアドレスまたはパスワードが正しくありません")
	}
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
		return "", nil, apperror.Unauthorized("メールアドレスまたはパスワードが正しくありません")
	}

	rawToken, hash, genErr := generateSessionToken()
	if genErr != nil {
		return "", nil, apperror.Internal("failed to create session")
	}
	now := s.clock.Now()
	session := &model.Session{
		UserID:    user.ID,
		TokenHash: hash,
		ExpiresAt: now.Add(sessionTTL),
		CreatedAt: now,
	}
	if err := s.sessions.Create(ctx, session); err != nil {
		return "", nil, err
	}
	return rawToken, user, nil
}

// Logout deletes the session identified by the raw token.
func (s *AuthService) Logout(ctx context.Context, rawToken string) error {
	if rawToken == "" {
		return nil
	}
	return s.sessions.DeleteByTokenHash(ctx, hashToken(rawToken))
}

// ValidateSession resolves a raw bearer token into the authenticated user,
// or apperror.Unauthorized if the token is missing, unknown, or expired.
func (s *AuthService) ValidateSession(ctx context.Context, rawToken string) (*model.User, error) {
	if rawToken == "" {
		return nil, apperror.Unauthorized("認証が必要です")
	}
	session, err := s.sessions.GetByTokenHash(ctx, hashToken(rawToken))
	if err != nil {
		return nil, err
	}
	if session == nil || session.ExpiresAt.Before(s.clock.Now()) {
		return nil, apperror.Unauthorized("セッションが無効です。再度ログインしてください")
	}
	user, err := s.users.GetByID(ctx, session.UserID)
	if err != nil {
		return nil, err
	}
	if user == nil {
		return nil, apperror.Unauthorized("セッションが無効です。再度ログインしてください")
	}
	return user, nil
}

// CleanupExpiredSessions removes every session that has already expired,
// returning how many were removed. Nothing else prunes the sessions table
// (a session row otherwise lives forever even past its own expires_at —
// ValidateSession just stops accepting it), so this is meant to be called
// periodically — see RunSessionCleanupLoop.
func (s *AuthService) CleanupExpiredSessions(ctx context.Context) (int64, error) {
	return s.sessions.DeleteExpired(ctx, s.clock.Now())
}

func generateSessionToken() (raw string, hash string, err error) {
	buf := make([]byte, sessionTokenBytes)
	if _, err := rand.Read(buf); err != nil {
		return "", "", err
	}
	raw = hex.EncodeToString(buf)
	return raw, hashToken(raw), nil
}

func hashToken(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}
