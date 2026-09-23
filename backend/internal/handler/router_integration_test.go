// Integration tests exercise the full HTTP stack (router, middleware,
// handlers, services, repositories) against a real Postgres database — the
// unit tests in internal/service use in-memory fakes, which cannot verify
// real transaction/locking behavior (see TestCompleteDailyMission_Concurrent
// below) or the HTTP-layer concerns (cookies, status codes, ownership
// checks enforced across the handler+repository boundary).
//
// Set TEST_DATABASE_URL to run these against a scratch Postgres database,
// e.g. the dev docker-compose db:
//
//	TEST_DATABASE_URL="postgres://levelog:levelog@localhost:5432/levelog?sslmode=disable" \
//	  go test ./internal/handler/... -run Integration -v
//
// Without it, every test in this file skips.
package handler_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"os"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"levelog/backend/internal/config"
	"levelog/backend/internal/db"
	"levelog/backend/internal/handler"
	"levelog/backend/internal/logging"
	"levelog/backend/internal/metrics"
	"levelog/backend/internal/repository"
	"levelog/backend/internal/service"
)

var (
	testServer          *httptest.Server
	testMetricsRegistry *metrics.Registry
	skipReason          string
)

func TestMain(m *testing.M) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		skipReason = "TEST_DATABASE_URL not set; skipping integration tests"
		os.Exit(m.Run())
	}

	conn, err := db.Open(dsn, db.PoolOptions{MaxOpenConns: 10, MaxIdleConns: 5, ConnMaxLifetime: 30 * time.Minute})
	if err != nil {
		fmt.Println("open test db:", err)
		os.Exit(1)
	}
	if err := db.WaitForReady(context.Background(), conn, 10*time.Second); err != nil {
		fmt.Println("test db not ready:", err)
		os.Exit(1)
	}
	if err := db.Migrate(conn); err != nil {
		fmt.Println("migrate test db:", err)
		os.Exit(1)
	}

	userRepo := repository.NewUserRepo(conn)
	sessionRepo := repository.NewSessionRepo(conn)
	templateRepo := repository.NewTemplateRepo(conn)
	dailyRepo := repository.NewDailyMissionRepo(conn)
	xpRepo := repository.NewXPRepo(conn)

	authService := service.NewAuthService(userRepo, sessionRepo, nil)
	missionService := service.NewMissionService(userRepo, templateRepo, dailyRepo, xpRepo, nil)
	xpService := service.NewXPService(xpRepo)

	cfg := config.Config{
		FrontendOrigins: []string{"http://example.test"},
		CookieSecure:    false,
		RequestTimeout:  10 * time.Second,
	}
	logger := logging.New("error")

	router, reg := handler.NewRouter(handler.Services{
		Auth:     authService,
		Missions: missionService,
		XP:       xpService,
	}, cfg, conn, logger)
	testMetricsRegistry = reg

	testServer = httptest.NewServer(router)

	code := m.Run()

	testServer.Close()
	conn.Close()
	os.Exit(code)
}

func skipUnlessDB(t *testing.T) {
	t.Helper()
	if skipReason != "" {
		t.Skip(skipReason)
	}
}

// testClient is an HTTP client with its own cookie jar, standing in for one
// browser session.
func testClient(t *testing.T) *http.Client {
	t.Helper()
	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatalf("cookiejar: %v", err)
	}
	return &http.Client{Jar: jar}
}

func uniqueEmail(t *testing.T) string {
	t.Helper()
	return fmt.Sprintf("it-%d-%d@example.com", time.Now().UnixNano(), os.Getpid())
}

func doJSON(t *testing.T, client *http.Client, method, path string, body any, out any) *http.Response {
	t.Helper()
	var reqBody io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("marshal request body: %v", err)
		}
		reqBody = bytes.NewReader(b)
	}
	req, err := http.NewRequest(method, testServer.URL+path, reqBody)
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("%s %s: %v", method, path, err)
	}
	if out != nil {
		defer resp.Body.Close()
		if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
			t.Fatalf("decode response body for %s %s: %v", method, path, err)
		}
	} else {
		resp.Body.Close()
	}
	return resp
}

func registerUser(t *testing.T, client *http.Client) (email string, userID string) {
	t.Helper()
	email = uniqueEmail(t)
	var out struct {
		ID string `json:"id"`
	}
	resp := doJSON(t, client, http.MethodPost, "/api/auth/register", map[string]string{
		"email":    email,
		"password": "password123",
		"timezone": "Asia/Tokyo",
	}, &out)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("register: expected 201, got %d", resp.StatusCode)
	}
	if out.ID == "" {
		t.Fatal("register: expected non-empty user id")
	}
	return email, out.ID
}

func allWeekdays() []string {
	return []string{"MONDAY", "TUESDAY", "WEDNESDAY", "THURSDAY", "FRIDAY", "SATURDAY", "SUNDAY"}
}

func createTemplate(t *testing.T, client *http.Client, title, difficulty string) string {
	t.Helper()
	var out struct {
		ID string `json:"id"`
	}
	resp := doJSON(t, client, http.MethodPost, "/api/missions", map[string]any{
		"title":       title,
		"description": "",
		"difficulty":  difficulty,
		"days":        allWeekdays(),
		"active":      true,
	}, &out)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create template: expected 201, got %d", resp.StatusCode)
	}
	return out.ID
}

func TestHealthEndpoints(t *testing.T) {
	skipUnlessDB(t)

	for _, path := range []string{"/health/live", "/health/ready", "/api/health"} {
		resp, err := http.Get(testServer.URL + path)
		if err != nil {
			t.Fatalf("GET %s: %v", path, err)
		}
		resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Errorf("GET %s: expected 200, got %d", path, resp.StatusCode)
		}
	}
}

func TestMetrics_RecordsRequestsByRoutePattern(t *testing.T) {
	skipUnlessDB(t)

	resp, err := http.Get(testServer.URL + "/health/live")
	if err != nil {
		t.Fatalf("GET /health/live: %v", err)
	}
	resp.Body.Close()

	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	rec := httptest.NewRecorder()
	testMetricsRegistry.Handler().ServeHTTP(rec, req)

	body := rec.Body.String()
	if !strings.Contains(body, `levelog_http_requests_total{method="GET",route="GET /health/live",status="200"}`) {
		t.Errorf("expected a recorded observation for GET /health/live, got:\n%s", body)
	}
}

// TestListMissions_EachTemplateGetsItsOwnScheduleDays guards against a
// regression of the connection-pool-starvation bug found and fixed in
// phase 17 (see docs/load-test-results.md): TemplateRepo.ListByUser used
// to run a query per template *while the outer query's rows cursor was
// still open*, which both duplicated work and, under concurrent load,
// could starve the connection pool. The fix batches all templates'
// schedule days into one follow-up query grouped by template ID — this
// test exercises that grouping directly: two templates with disjoint,
// non-trivial day sets must each get back exactly their own days, never
// the other's or a merged set.
func TestListMissions_EachTemplateGetsItsOwnScheduleDays(t *testing.T) {
	skipUnlessDB(t)
	client := testClient(t)
	registerUser(t, client)

	var weekdaysOut struct {
		ID string `json:"id"`
	}
	resp := doJSON(t, client, http.MethodPost, "/api/missions", map[string]any{
		"title":       "weekdays",
		"description": "",
		"difficulty":  "EASY",
		"days":        []string{"MONDAY", "TUESDAY", "WEDNESDAY", "THURSDAY", "FRIDAY"},
		"active":      true,
	}, &weekdaysOut)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create weekdays template: expected 201, got %d", resp.StatusCode)
	}

	var weekendOut struct {
		ID string `json:"id"`
	}
	resp = doJSON(t, client, http.MethodPost, "/api/missions", map[string]any{
		"title":       "weekend",
		"description": "",
		"difficulty":  "EASY",
		"days":        []string{"SATURDAY", "SUNDAY"},
		"active":      true,
	}, &weekendOut)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create weekend template: expected 201, got %d", resp.StatusCode)
	}

	var list []struct {
		ID   string   `json:"id"`
		Days []string `json:"days"`
	}
	resp = doJSON(t, client, http.MethodGet, "/api/missions", nil, &list)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET /api/missions: expected 200, got %d", resp.StatusCode)
	}
	if len(list) != 2 {
		t.Fatalf("expected 2 templates, got %d", len(list))
	}

	byID := map[string][]string{}
	for _, m := range list {
		byID[m.ID] = m.Days
	}

	wantWeekdays := []string{"MONDAY", "TUESDAY", "WEDNESDAY", "THURSDAY", "FRIDAY"}
	if got := byID[weekdaysOut.ID]; !slices.Equal(got, wantWeekdays) {
		t.Errorf("weekdays template days: got %v, want %v", got, wantWeekdays)
	}
	wantWeekend := []string{"SATURDAY", "SUNDAY"}
	if got := byID[weekendOut.ID]; !slices.Equal(got, wantWeekend) {
		t.Errorf("weekend template days: got %v, want %v", got, wantWeekend)
	}
}

// TestListMissions_Concurrent guards against the connection-pool-starvation
// regression itself (as opposed to
// TestListMissions_EachTemplateGetsItsOwnScheduleDays above, which guards
// the grouping logic's correctness but wouldn't have caught this — the old
// buggy code returned the right data, just occasionally not at all under
// load). The pre-fix TemplateRepo.ListByUser queried per-template schedule
// days while the outer query's rows cursor was still open, so each
// in-flight request held one pool connection and blocked acquiring a
// second; at concurrency meaningfully above the pool size (TestMain uses
// MaxOpenConns: 10) that starved the pool and produced 500/504s, confirmed
// with a real load test before the fix (docs/load-test-results.md).
// concurrency here (100) was chosen empirically: a smaller burst (25) did
// not reliably reproduce the failure even against the pre-fix code (a
// bounded, one-shot burst of concurrent goroutines drains through the
// pool differently than the sustained load a real `hey`/`wrk` run applies
// — see docs/load-test-results.md), but 100 reproduced 500s/504s
// consistently. Confirmed by temporarily reverting the fix and running
// this test alone: it failed against the old code and passes (in ~1s)
// against the fix.
func TestListMissions_Concurrent(t *testing.T) {
	skipUnlessDB(t)
	client := testClient(t)
	registerUser(t, client)
	createTemplate(t, client, "concurrent read test", "EASY")

	const concurrency = 100
	var wg sync.WaitGroup
	statuses := make([]int, concurrency)
	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			resp := doJSON(t, client, http.MethodGet, "/api/missions", nil, nil)
			statuses[i] = resp.StatusCode
		}(i)
	}
	wg.Wait()

	for i, status := range statuses {
		if status != http.StatusOK {
			t.Errorf("concurrent GET /api/missions[%d]: expected 200, got %d", i, status)
		}
	}
}

func TestUnknownRoute404(t *testing.T) {
	skipUnlessDB(t)

	resp, err := http.Get(testServer.URL + "/this-route-does-not-exist")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("expected 404, got %d", resp.StatusCode)
	}
}

func TestAuthFlow(t *testing.T) {
	skipUnlessDB(t)
	client := testClient(t)

	// Unauthenticated access is rejected.
	resp := doJSON(t, client, http.MethodGet, "/api/me", nil, nil)
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("GET /api/me unauthenticated: expected 401, got %d", resp.StatusCode)
	}

	email, userID := registerUser(t, client)

	// Registration also logs the user in (session cookie set).
	var me struct {
		ID    string `json:"id"`
		Email string `json:"email"`
	}
	resp = doJSON(t, client, http.MethodGet, "/api/me", nil, &me)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET /api/me after register: expected 200, got %d", resp.StatusCode)
	}
	if me.ID != userID || me.Email != email {
		t.Errorf("GET /api/me: got %+v, want id=%s email=%s", me, userID, email)
	}

	// Wrong password is rejected without revealing which field was wrong.
	resp = doJSON(t, testClient(t), http.MethodPost, "/api/auth/login", map[string]string{
		"email":    email,
		"password": "wrong-password",
	}, nil)
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("login with wrong password: expected 401, got %d", resp.StatusCode)
	}

	// Logout invalidates the session.
	resp = doJSON(t, client, http.MethodPost, "/api/auth/logout", nil, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("logout: expected 200, got %d", resp.StatusCode)
	}
	resp = doJSON(t, client, http.MethodGet, "/api/me", nil, nil)
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("GET /api/me after logout: expected 401, got %d", resp.StatusCode)
	}
}

func TestMissionLifecycleAndXP(t *testing.T) {
	skipUnlessDB(t)
	client := testClient(t)
	registerUser(t, client)

	createTemplate(t, client, "水を飲む", "NORMAL") // +20 XP

	var today struct {
		Missions []struct {
			ID     string `json:"id"`
			Status string `json:"status"`
		} `json:"missions"`
		Progress struct {
			TotalXP int `json:"totalXp"`
		} `json:"progress"`
	}
	resp := doJSON(t, client, http.MethodGet, "/api/missions/today", nil, &today)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET /api/missions/today: expected 200, got %d", resp.StatusCode)
	}
	if len(today.Missions) != 1 {
		t.Fatalf("expected 1 daily mission generated for today, got %d", len(today.Missions))
	}
	if today.Progress.TotalXP != 0 {
		t.Fatalf("expected 0 XP before completing anything, got %d", today.Progress.TotalXP)
	}
	dailyID := today.Missions[0].ID

	// Complete: grants XP exactly once.
	var complete struct {
		XPGained  int  `json:"xpGained"`
		LeveledUp bool `json:"leveledUp"`
		Progress  struct {
			TotalXP int `json:"totalXp"`
		} `json:"progress"`
	}
	resp = doJSON(t, client, http.MethodPost, "/api/daily-missions/"+dailyID+"/complete", nil, &complete)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("complete: expected 200, got %d", resp.StatusCode)
	}
	if complete.XPGained != 20 || complete.Progress.TotalXP != 20 {
		t.Fatalf("complete: expected +20 XP (total 20), got gained=%d total=%d", complete.XPGained, complete.Progress.TotalXP)
	}

	// Double-complete (sequential, same session) must be a no-op, not a
	// second XP grant — this is the double-award guard from the handler's
	// perspective (internal/service has the equivalent fake-repo test).
	resp = doJSON(t, client, http.MethodPost, "/api/daily-missions/"+dailyID+"/complete", nil, &complete)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("double complete: expected 200, got %d", resp.StatusCode)
	}
	if complete.XPGained != 0 || complete.Progress.TotalXP != 20 {
		t.Fatalf("double complete: expected +0 XP (total still 20), got gained=%d total=%d", complete.XPGained, complete.Progress.TotalXP)
	}

	// Uncomplete reverses the XP exactly.
	var uncomplete struct {
		Progress struct {
			TotalXP int `json:"totalXp"`
		} `json:"progress"`
	}
	resp = doJSON(t, client, http.MethodPost, "/api/daily-missions/"+dailyID+"/uncomplete", nil, &uncomplete)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("uncomplete: expected 200, got %d", resp.StatusCode)
	}
	if uncomplete.Progress.TotalXP != 0 {
		t.Fatalf("uncomplete: expected total XP back to 0, got %d", uncomplete.Progress.TotalXP)
	}

	// Repeated uncomplete (already pending) must also be a no-op.
	resp = doJSON(t, client, http.MethodPost, "/api/daily-missions/"+dailyID+"/uncomplete", nil, &uncomplete)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("double uncomplete: expected 200, got %d", resp.StatusCode)
	}
	if uncomplete.Progress.TotalXP != 0 {
		t.Fatalf("double uncomplete: expected total XP to stay 0, got %d", uncomplete.Progress.TotalXP)
	}
}

// TestCompleteDailyMission_Concurrent fires many simultaneous completion
// requests for the same daily mission and asserts XP is granted exactly
// once. This is the scenario the fake in-memory repositories used by
// internal/service's unit tests cannot exercise: it depends on the real
// `SELECT ... FOR UPDATE` row lock in
// internal/repository/daily_mission_repo.go serializing the transactions.
func TestCompleteDailyMission_Concurrent(t *testing.T) {
	skipUnlessDB(t)
	client := testClient(t)
	registerUser(t, client)
	createTemplate(t, client, "腕立て伏せ", "HARD") // +40 XP

	var today struct {
		Missions []struct {
			ID string `json:"id"`
		} `json:"missions"`
	}
	resp := doJSON(t, client, http.MethodGet, "/api/missions/today", nil, &today)
	if resp.StatusCode != http.StatusOK || len(today.Missions) != 1 {
		t.Fatalf("setup: expected 1 daily mission, got status=%d missions=%d", resp.StatusCode, len(today.Missions))
	}
	dailyID := today.Missions[0].ID

	const concurrency = 15
	var wg sync.WaitGroup
	statuses := make([]int, concurrency)
	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			// Each goroutine shares the same authenticated session cookie
			// jar (safe for concurrent reads across goroutines) to hit the
			// endpoint as the same logged-in user simultaneously.
			resp := doJSON(t, client, http.MethodPost, "/api/daily-missions/"+dailyID+"/complete", nil, nil)
			statuses[i] = resp.StatusCode
		}(i)
	}
	wg.Wait()

	for i, status := range statuses {
		if status != http.StatusOK {
			t.Errorf("concurrent complete[%d]: expected 200, got %d", i, status)
		}
	}

	var stats struct {
		Progress struct {
			TotalXP int `json:"totalXp"`
		} `json:"progress"`
	}
	resp = doJSON(t, client, http.MethodGet, "/api/stats", nil, &stats)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET /api/stats: expected 200, got %d", resp.StatusCode)
	}
	if stats.Progress.TotalXP != 40 {
		t.Fatalf("expected exactly one +40 XP grant despite %d concurrent completes, got total XP %d", concurrency, stats.Progress.TotalXP)
	}
}

func TestCrossUserAuthorization(t *testing.T) {
	skipUnlessDB(t)
	clientA := testClient(t)
	registerUser(t, clientA)
	templateID := createTemplate(t, clientA, "ランニング", "EASY")

	var today struct {
		Missions []struct {
			ID string `json:"id"`
		} `json:"missions"`
	}
	resp := doJSON(t, clientA, http.MethodGet, "/api/missions/today", nil, &today)
	if resp.StatusCode != http.StatusOK || len(today.Missions) != 1 {
		t.Fatalf("setup: expected 1 daily mission for user A, got status=%d missions=%d", resp.StatusCode, len(today.Missions))
	}
	dailyID := today.Missions[0].ID

	clientB := testClient(t)
	registerUser(t, clientB)

	// User B must not see user A's templates.
	var listB []struct {
		ID string `json:"id"`
	}
	resp = doJSON(t, clientB, http.MethodGet, "/api/missions", nil, &listB)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET /api/missions as user B: expected 200, got %d", resp.StatusCode)
	}
	for _, tpl := range listB {
		if tpl.ID == templateID {
			t.Fatal("user B can see user A's mission template in their own list")
		}
	}

	// User B cannot edit user A's template.
	resp = doJSON(t, clientB, http.MethodPut, "/api/missions/"+templateID, map[string]any{
		"title": "乗っ取り", "difficulty": "EASY", "days": allWeekdays(), "active": true,
	}, nil)
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("user B editing user A's template: expected 404, got %d", resp.StatusCode)
	}

	// User B cannot delete user A's template.
	resp = doJSON(t, clientB, http.MethodDelete, "/api/missions/"+templateID, nil, nil)
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("user B deleting user A's template: expected 404, got %d", resp.StatusCode)
	}

	// User B cannot complete user A's daily mission.
	resp = doJSON(t, clientB, http.MethodPost, "/api/daily-missions/"+dailyID+"/complete", nil, nil)
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("user B completing user A's daily mission: expected 404, got %d", resp.StatusCode)
	}

	// User A's mission must remain untouched (still completable by A).
	var complete struct {
		XPGained int `json:"xpGained"`
	}
	resp = doJSON(t, clientA, http.MethodPost, "/api/daily-missions/"+dailyID+"/complete", nil, &complete)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("user A completing own mission after user B's attempts: expected 200, got %d", resp.StatusCode)
	}
	if complete.XPGained != 10 {
		t.Errorf("expected user A to still get the full +10 XP, got %d", complete.XPGained)
	}
}
