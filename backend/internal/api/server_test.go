package api

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	_ "github.com/lib/pq"
	"github.com/takayoshi-u/polaris/backend/internal/domain"
	"github.com/takayoshi-u/polaris/backend/internal/fcm"
	"github.com/takayoshi-u/polaris/backend/internal/mail"
	"github.com/takayoshi-u/polaris/backend/internal/store"
	"github.com/takayoshi-u/polaris/backend/migrations"
)

func TestServerHTTPBasic(t *testing.T) {
	srv := NewServer(nil, "http://localhost:3000", GoogleOAuthConfig{
		ClientID:     "test-client-id",
		ClientSecret: "test-secret",
		RedirectURL:  "http://localhost:3000/auth/google/callback",
	})

	// 1. OPTIONS preflight CORS check
	req := httptest.NewRequest(http.MethodOptions, "/api/v1/health", nil)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected 204 for OPTIONS, got %d", rec.Code)
	}
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "http://localhost:3000" {
		t.Fatalf("expected CORS origin http://localhost:3000, got %s", got)
	}

	// 2. GET /api/v1/health
	req = httptest.NewRequest(http.MethodGet, "/api/v1/health", nil)
	rec = httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 for /api/v1/health, got %d", rec.Code)
	}

	// 3. GET /api/v1/auth/google/url
	req = httptest.NewRequest(http.MethodGet, "/api/v1/auth/google/url", nil)
	rec = httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 for /api/v1/auth/google/url, got %d", rec.Code)
	}
	var urlRes map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&urlRes); err != nil {
		t.Fatalf("decode response failed: %v", err)
	}
	if urlRes["url"] == "" || urlRes["enabled"] != true {
		t.Fatalf("expected non-empty Google OAuth URL and enabled=true, got %v", urlRes)
	}

	// 4. Unauthorized check on protected route without cookie/token
	req = httptest.NewRequest(http.MethodGet, "/api/v1/auth/me", nil)
	rec = httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for unauthenticated /api/v1/auth/me, got %d", rec.Code)
	}
}

// TestFullIntegration runs end-to-end API & Store tests when TEST_DATABASE_URL is provided.
func TestFullIntegration(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("skipping database integration test: TEST_DATABASE_URL not set")
	}

	db, err := sql.Open("postgres", dsn)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()

	ctx := context.Background()
	if _, err := db.ExecContext(ctx, `DROP SCHEMA public CASCADE; CREATE SCHEMA public;`); err != nil {
		t.Fatalf("reset schema: %v", err)
	}
	if err := migrations.Run(ctx, db); err != nil {
		t.Fatalf("apply migrations: %v", err)
	}

	mailer := mail.NewSMTPSender()
	var sentMails int
	mailer.CustomSend = func(cfg mail.Config, msg mail.Message) error {
		sentMails++
		return nil
	}
	fcmClient := fcm.NewClient(ctx, "")

	st := store.New(db, "0123456789abcdef0123456789abcdef", "http://localhost:3000", mailer, fcmClient)
	if err := st.EnsureSystemDefaults(ctx, "", "", ""); err != nil {
		t.Fatalf("ensure defaults: %v", err)
	}

	srv := NewServer(st, "http://localhost:3000", GoogleOAuthConfig{})

	// 1. Setup initial admin
	initBody, _ := json.Marshal(map[string]string{
		"email":        "admin@example.com",
		"display_name": "System Admin",
		"password":     "Password123!",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/setup/init", bytes.NewReader(initBody))
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("setup init failed (%d): %s", rec.Code, rec.Body.String())
	}
	cookies := rec.Result().Cookies()
	if len(cookies) == 0 {
		t.Fatalf("expected session cookie after setup init")
	}
	authCookie := cookies[0]

	// 2. Create project
	projBody, _ := json.Marshal(map[string]any{
		"key":           "POL",
		"name":          "Polaris Core",
		"description":   "Core development project",
		"hours_per_day": 8.0,
	})
	req = httptest.NewRequest(http.MethodPost, "/api/v1/projects", bytes.NewReader(projBody))
	req.AddCookie(authCookie)
	rec = httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create project failed (%d): %s", rec.Code, rec.Body.String())
	}
	var proj domain.Project
	_ = json.NewDecoder(rec.Body).Decode(&proj)

	// 3. Create issue with create_initial_task=true
	issueBody, _ := json.Marshal(map[string]any{
		"title":               "Implement Backend",
		"description":         "Build Go API",
		"create_initial_task": true,
	})
	req = httptest.NewRequest(http.MethodPost, "/api/v1/projects/"+proj.ID+"/issues", bytes.NewReader(issueBody))
	req.AddCookie(authCookie)
	rec = httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create issue failed (%d): %s", rec.Code, rec.Body.String())
	}
	var issue domain.Issue
	_ = json.NewDecoder(rec.Body).Decode(&issue)
	if len(issue.Tasks) != 1 {
		t.Fatalf("expected 1 auto-created initial task, got %d", len(issue.Tasks))
	}
}
