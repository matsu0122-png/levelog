package middleware

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"levelog/backend/internal/reqid"
)

func TestLogging_EmitsStructuredRequestLine(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, nil))

	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
	})
	handler := Logging(logger)(inner)

	req := httptest.NewRequest(http.MethodPost, "/api/missions", nil)
	req = req.WithContext(reqid.WithContext(req.Context(), "test-request-id"))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	line := strings.TrimSpace(buf.String())
	if line == "" {
		t.Fatal("expected a log line to be written")
	}
	var entry map[string]any
	if err := json.Unmarshal([]byte(line), &entry); err != nil {
		t.Fatalf("log line is not valid JSON: %v\nline: %s", err, line)
	}

	if entry["msg"] != "http_request" {
		t.Errorf("msg: got %v", entry["msg"])
	}
	if entry["method"] != http.MethodPost {
		t.Errorf("method: got %v", entry["method"])
	}
	if entry["path"] != "/api/missions" {
		t.Errorf("path: got %v", entry["path"])
	}
	if entry["request_id"] != "test-request-id" {
		t.Errorf("request_id: got %v", entry["request_id"])
	}
	if status, ok := entry["status"].(float64); !ok || int(status) != http.StatusCreated {
		t.Errorf("status: got %v", entry["status"])
	}
	// No request/response body, headers, cookies, or query params should ever
	// be logged — only the fixed set of structured fields above.
	for key := range entry {
		switch key {
		case "time", "level", "msg", "request_id", "method", "path", "status", "duration_ms":
		default:
			t.Errorf("unexpected field in log line: %q (value %v)", key, entry[key])
		}
	}
}

func TestLogging_DefaultsStatusTo200WhenHandlerNeverCallsWriteHeader(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, nil))

	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("ok")) // implicit 200, no explicit WriteHeader call
	})
	handler := Logging(logger)(inner)

	req := httptest.NewRequest(http.MethodGet, "/health/live", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	var entry map[string]any
	if err := json.Unmarshal(bytes.TrimSpace(buf.Bytes()), &entry); err != nil {
		t.Fatalf("log line is not valid JSON: %v", err)
	}
	if status, ok := entry["status"].(float64); !ok || int(status) != http.StatusOK {
		t.Errorf("status: got %v, want 200", entry["status"])
	}
}
