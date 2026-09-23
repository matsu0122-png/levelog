package metrics

import (
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestObserve_CountsByMethodRouteStatus(t *testing.T) {
	r := NewRegistry()
	r.Observe("GET", "/api/missions", 200, 10*time.Millisecond)
	r.Observe("GET", "/api/missions", 200, 20*time.Millisecond)
	r.Observe("GET", "/api/missions", 500, 5*time.Millisecond)

	out := r.render()

	if !strings.Contains(out, `levelog_http_requests_total{method="GET",route="/api/missions",status="200"} 2`) {
		t.Errorf("expected 200 count of 2, got:\n%s", out)
	}
	if !strings.Contains(out, `levelog_http_requests_total{method="GET",route="/api/missions",status="500"} 1`) {
		t.Errorf("expected 500 count of 1, got:\n%s", out)
	}
}

func TestObserve_HistogramBucketsAreCumulative(t *testing.T) {
	r := NewRegistry()
	r.Observe("GET", "/api/missions", 200, 3*time.Millisecond)   // <= 0.005
	r.Observe("GET", "/api/missions", 200, 200*time.Millisecond) // <= 0.25, > 0.1

	out := r.render()

	// The 0.005s bucket only catches the first observation.
	if !strings.Contains(out, `levelog_http_request_duration_seconds_bucket{method="GET",route="/api/missions",le="0.005"} 1`) {
		t.Errorf("expected le=0.005 bucket count of 1, got:\n%s", out)
	}
	// The 0.25s bucket is cumulative and catches both.
	if !strings.Contains(out, `levelog_http_request_duration_seconds_bucket{method="GET",route="/api/missions",le="0.25"} 2`) {
		t.Errorf("expected le=0.25 bucket count of 2, got:\n%s", out)
	}
	if !strings.Contains(out, `levelog_http_request_duration_seconds_bucket{method="GET",route="/api/missions",le="+Inf"} 2`) {
		t.Errorf("expected +Inf bucket count of 2, got:\n%s", out)
	}
	if !strings.Contains(out, `levelog_http_request_duration_seconds_count{method="GET",route="/api/missions"} 2`) {
		t.Errorf("expected count of 2, got:\n%s", out)
	}
}

func TestHandler_ServesTextFormat(t *testing.T) {
	r := NewRegistry()
	r.Observe("GET", "/api/missions", 200, time.Millisecond)

	req := httptest.NewRequest("GET", "/metrics", nil)
	rec := httptest.NewRecorder()
	r.Handler().ServeHTTP(rec, req)

	if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/plain") {
		t.Errorf("Content-Type: got %q, want text/plain prefix", ct)
	}
	if !strings.Contains(rec.Body.String(), "levelog_http_requests_total") {
		t.Errorf("expected body to contain levelog_http_requests_total, got:\n%s", rec.Body.String())
	}
}
