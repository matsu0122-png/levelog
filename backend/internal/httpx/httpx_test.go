package httpx

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"levelog/backend/internal/apperror"
)

func TestWriteJSON(t *testing.T) {
	rec := httptest.NewRecorder()
	WriteJSON(rec, http.StatusCreated, map[string]string{"ok": "yes"})

	if rec.Code != http.StatusCreated {
		t.Errorf("status: got %d, want %d", rec.Code, http.StatusCreated)
	}
	if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
		t.Errorf("Content-Type: got %q", ct)
	}
	var body map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal body: %v", err)
	}
	if body["ok"] != "yes" {
		t.Errorf("body: got %v", body)
	}
}

func TestWriteJSON_NilBody(t *testing.T) {
	rec := httptest.NewRecorder()
	WriteJSON(rec, http.StatusNoContent, nil)

	if rec.Code != http.StatusNoContent {
		t.Errorf("status: got %d, want %d", rec.Code, http.StatusNoContent)
	}
	if rec.Body.Len() != 0 {
		t.Errorf("expected empty body, got %q", rec.Body.String())
	}
}

func TestWriteError_AppError(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/missions", nil)

	WriteError(rec, req, apperror.NotFound("ミッションが見つかりません"))

	if rec.Code != http.StatusNotFound {
		t.Errorf("status: got %d, want %d", rec.Code, http.StatusNotFound)
	}
	var body errorBody
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal body: %v", err)
	}
	if body.Error.Code != "NOT_FOUND" || body.Error.Message != "ミッションが見つかりません" {
		t.Errorf("body: got %+v", body)
	}
}

func TestWriteError_GenericErrorDoesNotLeakDetails(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/missions", nil)

	WriteError(rec, req, errors.New("pq: connection to db_internal_host refused, password wrong"))

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status: got %d, want %d", rec.Code, http.StatusInternalServerError)
	}
	var body errorBody
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal body: %v", err)
	}
	if body.Error.Code != "INTERNAL" {
		t.Errorf("code: got %q, want INTERNAL", body.Error.Code)
	}
	if strings.Contains(body.Error.Message, "db_internal_host") || strings.Contains(body.Error.Message, "password") {
		t.Errorf("response leaked internal error details: %q", body.Error.Message)
	}
}

func TestWriteError_ContextDeadlineExceeded(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/missions", nil)

	WriteError(rec, req, context.DeadlineExceeded)

	if rec.Code != http.StatusGatewayTimeout {
		t.Errorf("status: got %d, want %d", rec.Code, http.StatusGatewayTimeout)
	}
	var body errorBody
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal body: %v", err)
	}
	if body.Error.Code != "TIMEOUT" {
		t.Errorf("code: got %q, want TIMEOUT", body.Error.Code)
	}
}

func TestWriteError_WrappedContextDeadlineExceeded(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/missions", nil)

	// Repositories return wrapped errors (fmt.Errorf("...: %w", err)), so the
	// timeout classification must survive unwrapping via errors.Is.
	wrapped := &wrappedError{msg: "query users: context deadline exceeded", err: context.DeadlineExceeded}
	WriteError(rec, req, wrapped)

	if rec.Code != http.StatusGatewayTimeout {
		t.Errorf("status: got %d, want %d", rec.Code, http.StatusGatewayTimeout)
	}
}

type wrappedError struct {
	msg string
	err error
}

func (e *wrappedError) Error() string { return e.msg }
func (e *wrappedError) Unwrap() error { return e.err }

func TestDecodeJSON_Valid(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/auth/login", strings.NewReader(`{"email":"a@b.com","password":"secret123"}`))
	var out struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := DecodeJSON(req, &out); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out.Email != "a@b.com" || out.Password != "secret123" {
		t.Errorf("decoded: %+v", out)
	}
}

func TestDecodeJSON_InvalidJSON(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/auth/login", strings.NewReader(`{not json`))
	var out map[string]string
	err := DecodeJSON(req, &out)
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
	var aerr *apperror.Error
	if !errors.As(err, &aerr) || aerr.Status != http.StatusBadRequest {
		t.Errorf("expected BadRequest apperror, got %v", err)
	}
}

func TestDecodeJSON_UnknownFieldRejected(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/auth/login", strings.NewReader(`{"email":"a@b.com","totally_unknown_field":1}`))
	var out struct {
		Email string `json:"email"`
	}
	if err := DecodeJSON(req, &out); err == nil {
		t.Fatal("expected error for unknown field")
	}
}

// sanity check that the timeout classification doesn't accidentally treat
// context.DeadlineExceeded's near neighbor (a slow real timeout with a
// deadline set far in the future) as a timeout.
func TestWriteError_RealTimeoutContextIsUnaffectedWhenNotExceeded(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Hour)
	defer cancel()
	req := httptest.NewRequest(http.MethodGet, "/api/missions", nil).WithContext(ctx)
	rec := httptest.NewRecorder()

	WriteError(rec, req, apperror.BadRequest("bad"))

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status: got %d, want %d", rec.Code, http.StatusBadRequest)
	}
}
