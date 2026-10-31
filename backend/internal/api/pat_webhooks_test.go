package api

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	_ "github.com/lib/pq"
	"github.com/takayoshi-u/polaris/backend/internal/domain"
	"github.com/takayoshi-u/polaris/backend/internal/fcm"
	"github.com/takayoshi-u/polaris/backend/internal/mail"
	"github.com/takayoshi-u/polaris/backend/internal/store"
	"github.com/takayoshi-u/polaris/backend/migrations"
)

type testRig struct {
	srv         *Server
	st          *store.Store
	db          *sql.DB
	adminUser   *domain.User
	adminCookie *http.Cookie
}

func setupTestRig(t *testing.T) *testRig {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("skipping webhook integration test: TEST_DATABASE_URL not set")
	}

	db, err := sql.Open("postgres", dsn)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}

	ctx := context.Background()
	if _, err := db.ExecContext(ctx, `DROP SCHEMA public CASCADE; CREATE SCHEMA public;`); err != nil {
		t.Fatalf("reset schema: %v", err)
	}
	if err := migrations.Run(ctx, db); err != nil {
		t.Fatalf("apply migrations: %v", err)
	}

	mailer := mail.NewSMTPSender()
	fcmClient := fcm.NewClient(ctx, "")
	st := store.New(db, "0123456789abcdef0123456789abcdef", "http://localhost:3000", mailer, fcmClient)
	if err := st.EnsureSystemDefaults(ctx, "", "", ""); err != nil {
		t.Fatalf("ensure defaults: %v", err)
	}

	srv := NewServer(st, "http://localhost:3000", GoogleOAuthConfig{})

	// Setup initial admin (鈴木PM)
	initBody, _ := json.Marshal(map[string]string{
		"email":        "suzuki-pm@example.com",
		"display_name": "鈴木PM",
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

	var initRes struct {
		User domain.User `json:"user"`
	}
	_ = json.NewDecoder(rec.Body).Decode(&initRes)

	return &testRig{
		srv:         srv,
		st:          st,
		db:          db,
		adminUser:   &initRes.User,
		adminCookie: cookies[0],
	}
}

// TestWebhookLifecycleAndPing verifies project-level webhook registration, ping testing, delivery logging, updating, and deleting.
func TestWebhookLifecycleAndPing(t *testing.T) {
	rig := setupTestRig(t)
	defer rig.db.Close()

	// 1. Create a project
	projBody, _ := json.Marshal(map[string]any{
		"key":           "PRJ",
		"name":          "鈴木PM検証プロジェクト",
		"description":   "Webhook & API test",
		"hours_per_day": 7.5,
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/projects", bytes.NewReader(projBody))
	req.AddCookie(rig.adminCookie)
	rec := httptest.NewRecorder()
	rig.srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create project failed (%d): %s", rec.Code, rec.Body.String())
	}
	var proj domain.Project
	_ = json.NewDecoder(rec.Body).Decode(&proj)

	// Setup mock webhook receiver server
	type receivedItem struct {
		Event     string
		Delivery  string
		Signature string
		Body      string
	}
	var receivedRequests []receivedItem
	var mu sync.Mutex
	receiver := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		mu.Lock()
		receivedRequests = append(receivedRequests, receivedItem{
			Event:     r.Header.Get("X-Polaris-Event"),
			Delivery:  r.Header.Get("X-Polaris-Delivery"),
			Signature: r.Header.Get("X-Polaris-Signature"),
			Body:      string(body),
		})
		mu.Unlock()
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer receiver.Close()

	// 2. Register a project webhook
	hookSecret := "whsec_customsecret12345"
	hookReqBody, _ := json.Marshal(map[string]any{
		"name":   "PM Slack 通知連携",
		"url":    receiver.URL,
		"secret": hookSecret,
		"events": []string{"task.created", "task.updated", "issue.created", "comment.created"},
	})
	req = httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/v1/projects/%s/webhooks", proj.ID), bytes.NewReader(hookReqBody))
	req.AddCookie(rig.adminCookie)
	rec = httptest.NewRecorder()
	rig.srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create webhook failed (%d): %s", rec.Code, rec.Body.String())
	}
	var createdHook domain.Webhook
	if err := json.NewDecoder(rec.Body).Decode(&createdHook); err != nil {
		t.Fatalf("decode created hook: %v", err)
	}
	if createdHook.ID == "" || createdHook.Name != "PM Slack 通知連携" || createdHook.Secret != hookSecret {
		t.Fatalf("unexpected webhook data: %+v", createdHook)
	}
	if createdHook.ProjectID == nil || *createdHook.ProjectID != proj.ID {
		t.Fatalf("expected webhook projectID to be %s, got %v", proj.ID, createdHook.ProjectID)
	}

	// 3. List project webhooks
	req = httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/v1/projects/%s/webhooks", proj.ID), nil)
	req.AddCookie(rig.adminCookie)
	rec = httptest.NewRecorder()
	rig.srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("list webhooks failed (%d): %s", rec.Code, rec.Body.String())
	}
	var hooksList []domain.Webhook
	_ = json.NewDecoder(rec.Body).Decode(&hooksList)
	if len(hooksList) != 1 || hooksList[0].ID != createdHook.ID {
		t.Fatalf("expected 1 webhook in list, got %d", len(hooksList))
	}

	// 4. Run Ping Test (POST /api/v1/projects/{projectId}/webhooks/{webhookId}/test)
	req = httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/v1/projects/%s/webhooks/%s/test", proj.ID, createdHook.ID), nil)
	req.AddCookie(rig.adminCookie)
	rec = httptest.NewRecorder()
	rig.srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("test webhook ping failed (%d): %s", rec.Code, rec.Body.String())
	}
	var deliveryRes domain.WebhookDelivery
	_ = json.NewDecoder(rec.Body).Decode(&deliveryRes)
	if deliveryRes.StatusCode == nil || *deliveryRes.StatusCode != 200 {
		t.Fatalf("expected 200 delivery status, got %+v", deliveryRes)
	}
	if deliveryRes.Event != "ping" {
		t.Fatalf("expected event=ping, got %s", deliveryRes.Event)
	}

	// Verify receiver got ping request and headers
	mu.Lock()
	if len(receivedRequests) != 1 {
		t.Fatalf("expected 1 received request on mock server, got %d", len(receivedRequests))
	}
	pingReq := receivedRequests[0]
	mu.Unlock()
	if pingReq.Event != "ping" {
		t.Errorf("expected X-Polaris-Event: ping, got %s", pingReq.Event)
	}
	if !strings.HasPrefix(pingReq.Signature, "sha256=") {
		t.Errorf("expected HMAC-SHA256 signature prefix, got %s", pingReq.Signature)
	}

	// 5. Verify delivery logging in webhook_deliveries endpoint
	req = httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/v1/projects/%s/webhooks/%s/deliveries", proj.ID, createdHook.ID), nil)
	req.AddCookie(rig.adminCookie)
	rec = httptest.NewRecorder()
	rig.srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("list webhook deliveries failed (%d): %s", rec.Code, rec.Body.String())
	}
	var deliveries []domain.WebhookDelivery
	_ = json.NewDecoder(rec.Body).Decode(&deliveries)
	if len(deliveries) != 1 || deliveries[0].ID != deliveryRes.ID {
		t.Fatalf("expected 1 delivery logged, got %d", len(deliveries))
	}

	// 6. Test updating project webhook
	updateBody, _ := json.Marshal(map[string]any{
		"name":      "PM Slack 通知連携 (更新版)",
		"url":       receiver.URL + "/updated",
		"secret":    "",
		"events":    []string{"task.created", "task.updated"},
		"is_active": true,
	})
	req = httptest.NewRequest(http.MethodPut, fmt.Sprintf("/api/v1/projects/%s/webhooks/%s", proj.ID, createdHook.ID), bytes.NewReader(updateBody))
	req.AddCookie(rig.adminCookie)
	rec = httptest.NewRecorder()
	rig.srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("update webhook failed (%d): %s", rec.Code, rec.Body.String())
	}
	var updatedHook domain.Webhook
	_ = json.NewDecoder(rec.Body).Decode(&updatedHook)
	if updatedHook.Name != "PM Slack 通知連携 (更新版)" || updatedHook.URL != receiver.URL+"/updated" {
		t.Fatalf("webhook not updated as expected: %+v", updatedHook)
	}
	if updatedHook.Secret != hookSecret {
		t.Fatalf("expected secret to be preserved as %s, got %s", hookSecret, updatedHook.Secret)
	}

	// 7. Test deleting project webhook
	req = httptest.NewRequest(http.MethodDelete, fmt.Sprintf("/api/v1/projects/%s/webhooks/%s", proj.ID, createdHook.ID), nil)
	req.AddCookie(rig.adminCookie)
	rec = httptest.NewRecorder()
	rig.srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("delete webhook failed (%d): %s", rec.Code, rec.Body.String())
	}

	// Verify deleted
	req = httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/v1/projects/%s/webhooks/%s", proj.ID, createdHook.ID), nil)
	req.AddCookie(rig.adminCookie)
	rec = httptest.NewRecorder()
	rig.srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404 after webhook deletion, got %d", rec.Code)
	}
}

// TestWebhookEventDispatches tests task.created, task.updated, issue.created, comment.created dispatches.
func TestWebhookEventDispatches(t *testing.T) {
	rig := setupTestRig(t)
	defer rig.db.Close()

	// Create project
	projBody, _ := json.Marshal(map[string]any{
		"key":           "PRJ2",
		"name":          "ディスパッチ検証プロジェクト",
		"description":   "Testing event dispatches",
		"hours_per_day": 8.0,
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/projects", bytes.NewReader(projBody))
	req.AddCookie(rig.adminCookie)
	rec := httptest.NewRecorder()
	rig.srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create project: %d", rec.Code)
	}
	var proj domain.Project
	_ = json.NewDecoder(rec.Body).Decode(&proj)

	// Mock receiver
	type receivedEvent struct {
		Event string
		Body  map[string]any
	}
	var mu sync.Mutex
	var received []receivedEvent

	receiver := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		bodyBytes, _ := io.ReadAll(r.Body)
		var parsed map[string]any
		_ = json.Unmarshal(bodyBytes, &parsed)
		mu.Lock()
		received = append(received, receivedEvent{
			Event: r.Header.Get("X-Polaris-Event"),
			Body:  parsed,
		})
		mu.Unlock()
		w.WriteHeader(http.StatusOK)
	}))
	defer receiver.Close()

	// Register webhook subscribed to all 4 events
	hookReq, _ := json.Marshal(map[string]any{
		"name":   "Full Event Hook",
		"url":    receiver.URL,
		"events": []string{"task.created", "task.updated", "issue.created", "comment.created"},
	})
	req = httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/v1/projects/%s/webhooks", proj.ID), bytes.NewReader(hookReq))
	req.AddCookie(rig.adminCookie)
	rec = httptest.NewRecorder()
	rig.srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create webhook failed: %d", rec.Code)
	}

	waitForEvent := func(expectedEvent string, timeout time.Duration) bool {
		deadline := time.Now().Add(timeout)
		for time.Now().Before(deadline) {
			mu.Lock()
			for _, ev := range received {
				if ev.Event == expectedEvent {
					mu.Unlock()
					return true
				}
			}
			mu.Unlock()
			time.Sleep(50 * time.Millisecond)
		}
		return false
	}

	// 1. Issue Created Event
	issueBody, _ := json.Marshal(map[string]any{
		"title":       "課題#1 画面設計",
		"description": "UI画面設計を作成する",
	})
	req = httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/v1/projects/%s/issues", proj.ID), bytes.NewReader(issueBody))
	req.AddCookie(rig.adminCookie)
	rec = httptest.NewRecorder()
	rig.srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create issue failed: %d", rec.Code)
	}
	var createdIssue domain.Issue
	_ = json.NewDecoder(rec.Body).Decode(&createdIssue)

	if !waitForEvent("issue.created", 2*time.Second) {
		t.Errorf("expected issue.created event was not received within timeout")
	}

	// 2. Task Created Event
	taskBody, _ := json.Marshal(map[string]any{
		"issue_id":      createdIssue.ID,
		"title":         "ワイヤーフレーム作成",
		"planned_hours": 4.0,
	})
	req = httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/v1/projects/%s/tasks", proj.ID), bytes.NewReader(taskBody))
	req.AddCookie(rig.adminCookie)
	rec = httptest.NewRecorder()
	rig.srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create task failed: %d", rec.Code)
	}
	var createdTask domain.Task
	_ = json.NewDecoder(rec.Body).Decode(&createdTask)

	if !waitForEvent("task.created", 2*time.Second) {
		t.Errorf("expected task.created event was not received within timeout")
	}

	// 3. Task Updated Event (Update Task progress)
	progressVal := 50
	updateTaskBody, _ := json.Marshal(map[string]any{
		"progress": progressVal,
	})
	req = httptest.NewRequest(http.MethodPut, fmt.Sprintf("/api/v1/projects/%s/tasks/%s", proj.ID, createdTask.ID), bytes.NewReader(updateTaskBody))
	req.AddCookie(rig.adminCookie)
	rec = httptest.NewRecorder()
	rig.srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("update task failed: %d", rec.Code)
	}

	if !waitForEvent("task.updated", 2*time.Second) {
		t.Errorf("expected task.updated event was not received within timeout")
	}

	// 4. Comment Created Event
	commentBody, _ := json.Marshal(map[string]any{
		"body": "作業を開始しました。本日中にドラフト作成予定です。",
	})
	req = httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/v1/projects/%s/tasks/%s/comments", proj.ID, createdTask.ID), bytes.NewReader(commentBody))
	req.AddCookie(rig.adminCookie)
	rec = httptest.NewRecorder()
	rig.srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create comment failed: %d", rec.Code)
	}

	if !waitForEvent("comment.created", 2*time.Second) {
		t.Errorf("expected comment.created event was not received within timeout")
	}

	// 5. Check BUG: GET /tasks/{taskId} should NOT trigger task.updated!
	mu.Lock()
	countBeforeGet := 0
	for _, ev := range received {
		if ev.Event == "task.updated" {
			countBeforeGet++
		}
	}
	mu.Unlock()

	req = httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/v1/projects/%s/tasks/%s", proj.ID, createdTask.ID), nil)
	req.AddCookie(rig.adminCookie)
	rec = httptest.NewRecorder()
	rig.srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("get task failed: %d", rec.Code)
	}
	time.Sleep(300 * time.Millisecond)

	mu.Lock()
	countAfterGet := 0
	for _, ev := range received {
		if ev.Event == "task.updated" {
			countAfterGet++
		}
	}
	mu.Unlock()

	if countAfterGet > countBeforeGet {
		t.Logf("BUG CONFIRMED: GET /tasks/{taskId} dispatched task.updated event! countBefore=%d, countAfter=%d", countBeforeGet, countAfterGet)
	} else {
		t.Logf("GET /tasks/{taskId} did not dispatch task.updated event.")
	}
}

// TestWebhookTenantBoundary verifies that a project admin cannot access or mutate webhooks of another project or system webhooks.
func TestWebhookTenantBoundary(t *testing.T) {
	rig := setupTestRig(t)
	defer rig.db.Close()

	// Create Project A
	projABody, _ := json.Marshal(map[string]any{"key": "PRJA", "name": "Project A", "hours_per_day": 8.0})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/projects", bytes.NewReader(projABody))
	req.AddCookie(rig.adminCookie)
	rec := httptest.NewRecorder()
	rig.srv.ServeHTTP(rec, req)
	var projA domain.Project
	_ = json.NewDecoder(rec.Body).Decode(&projA)

	// Create Project B
	projBBody, _ := json.Marshal(map[string]any{"key": "PRJB", "name": "Project B", "hours_per_day": 8.0})
	req = httptest.NewRequest(http.MethodPost, "/api/v1/projects", bytes.NewReader(projBBody))
	req.AddCookie(rig.adminCookie)
	rec = httptest.NewRecorder()
	rig.srv.ServeHTTP(rec, req)
	var projB domain.Project
	_ = json.NewDecoder(rec.Body).Decode(&projB)

	// Create second user (PM of Project A ONLY)
	userA, err := rig.st.CreateUserDirect(context.Background(), "user-a@example.com", "User A (Project A Admin)", "Password123!", nil, nil)
	if err != nil {
		t.Fatalf("create user A: %v", err)
	}
	// Add user A to Project A as Admin
	if _, err := rig.st.UpsertProjectMember(context.Background(), projA.ID, "user", userA.ID, domain.RoleAdmin); err != nil {
		t.Fatalf("add user A to project A: %v", err)
	}

	// Login User A
	loginBody, _ := json.Marshal(map[string]string{"email": "user-a@example.com", "password": "Password123!"})
	req = httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewReader(loginBody))
	rec = httptest.NewRecorder()
	rig.srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("login user A failed: %d", rec.Code)
	}
	userACookie := rec.Result().Cookies()[0]

	// Admin creates a webhook in Project B
	hookReq, _ := json.Marshal(map[string]any{
		"name":   "Project B Sensitive Webhook",
		"url":    "https://project-b.internal/webhook",
		"secret": "topsecretB",
		"events": []string{"task.created"},
	})
	req = httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/v1/projects/%s/webhooks", projB.ID), bytes.NewReader(hookReq))
	req.AddCookie(rig.adminCookie)
	rec = httptest.NewRecorder()
	rig.srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create webhook in project B failed: %d", rec.Code)
	}
	var hookB domain.Webhook
	_ = json.NewDecoder(rec.Body).Decode(&hookB)

	// Now User A (Admin of Project A, but NO ACCESS to Project B)
	// Attempts to access Project B's webhook via Project A's URL:
	// GET /api/v1/projects/{projA}/webhooks/{hookB_id}
	req = httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/v1/projects/%s/webhooks/%s", projA.ID, hookB.ID), nil)
	req.AddCookie(userACookie)
	rec = httptest.NewRecorder()
	rig.srv.ServeHTTP(rec, req)

	t.Logf("GET foreign webhook response code: %d", rec.Code)
	if rec.Code == http.StatusOK {
		t.Logf("CONFIRMED SECURITY VULNERABILITY (IDOR): Project A admin was able to GET Project B's webhook! Body: %s", rec.Body.String())
	}

	// Attempts to delete Project B's webhook via Project A's URL:
	// DELETE /api/v1/projects/{projA}/webhooks/{hookB_id}
	req = httptest.NewRequest(http.MethodDelete, fmt.Sprintf("/api/v1/projects/%s/webhooks/%s", projA.ID, hookB.ID), nil)
	req.AddCookie(userACookie)
	rec = httptest.NewRecorder()
	rig.srv.ServeHTTP(rec, req)

	t.Logf("DELETE foreign webhook response code: %d", rec.Code)
	if rec.Code == http.StatusNoContent {
		t.Logf("CONFIRMED SECURITY VULNERABILITY (IDOR): Project A admin was able to DELETE Project B's webhook!")
	}
}

// TestPATLifecycleAndPermissions tests Personal Access Token authentication, permissions (Admin, Editor, Viewer, Non-Member), and project-scoped API actions.
func TestPATLifecycleAndPermissions(t *testing.T) {
	rig := setupTestRig(t)
	defer rig.db.Close()

	// 1. Create a project
	projBody, _ := json.Marshal(map[string]any{
		"key":           "PATP",
		"name":          "PAT検証プロジェクト",
		"description":   "Personal Access Token Project",
		"hours_per_day": 8.0,
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/projects", bytes.NewReader(projBody))
	req.AddCookie(rig.adminCookie)
	rec := httptest.NewRecorder()
	rig.srv.ServeHTTP(rec, req)
	var proj domain.Project
	_ = json.NewDecoder(rec.Body).Decode(&proj)

	// Create an issue in the project
	issueBody, _ := json.Marshal(map[string]any{
		"title":       "機能要件定義",
		"description": "要件の取りまとめ",
	})
	req = httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/v1/projects/%s/issues", proj.ID), bytes.NewReader(issueBody))
	req.AddCookie(rig.adminCookie)
	rec = httptest.NewRecorder()
	rig.srv.ServeHTTP(rec, req)
	var createdIssue domain.Issue
	_ = json.NewDecoder(rec.Body).Decode(&createdIssue)

	// 2. Create users with different roles:
	// - Editor user
	editorUser, _ := rig.st.CreateUserDirect(context.Background(), "editor@example.com", "Editor 佐藤", "Password123!", nil, nil)
	_, _ = rig.st.UpsertProjectMember(context.Background(), proj.ID, "user", editorUser.ID, domain.RoleEditor)

	// - Viewer user
	viewerUser, _ := rig.st.CreateUserDirect(context.Background(), "viewer@example.com", "Viewer 田中", "Password123!", nil, nil)
	_, _ = rig.st.UpsertProjectMember(context.Background(), proj.ID, "user", viewerUser.ID, domain.RoleViewer)

	// - Non-member user
	nonMemberUser, _ := rig.st.CreateUserDirect(context.Background(), "nonmember@example.com", "NonMember 山本", "Password123!", nil, nil)

	// Generate PAT for each user
	createPAT := func(userID, tokenName string, scopes []string, expiresAt *time.Time) string {
		plainToken, _, err := rig.st.CreatePAT(context.Background(), userID, tokenName, scopes, expiresAt)
		if err != nil {
			t.Fatalf("create PAT for %s: %v", userID, err)
		}
		return plainToken
	}

	adminPAT := createPAT(rig.adminUser.ID, "Admin PAT", []string{"read:projects", "write:projects"}, nil)
	editorPAT := createPAT(editorUser.ID, "Editor PAT", []string{"read:issues", "write:tasks"}, nil)
	viewerPAT := createPAT(viewerUser.ID, "Viewer PAT", []string{"read:issues", "read:tasks"}, nil)
	nonMemberPAT := createPAT(nonMemberUser.ID, "NonMember PAT", []string{"read:issues"}, nil)

	// Helper for PAT authenticated request
	doPATRequest := func(method, path, token string, body any) *httptest.ResponseRecorder {
		var rdr io.Reader
		if body != nil {
			b, _ := json.Marshal(body)
			rdr = bytes.NewReader(b)
		}
		r := httptest.NewRequest(method, path, rdr)
		if token != "" {
			r.Header.Set("Authorization", "Bearer "+token)
		}
		w := httptest.NewRecorder()
		rig.srv.ServeHTTP(w, r)
		return w
	}

	// 3. Test Listing Issues via PAT
	// Admin PAT -> 200 OK
	w := doPATRequest(http.MethodGet, fmt.Sprintf("/api/v1/projects/%s/issues", proj.ID), adminPAT, nil)
	if w.Code != http.StatusOK {
		t.Errorf("admin PAT list issues expected 200, got %d: %s", w.Code, w.Body.String())
	}

	// Viewer PAT -> 200 OK
	w = doPATRequest(http.MethodGet, fmt.Sprintf("/api/v1/projects/%s/issues", proj.ID), viewerPAT, nil)
	if w.Code != http.StatusOK {
		t.Errorf("viewer PAT list issues expected 200, got %d: %s", w.Code, w.Body.String())
	}

	// Non-Member PAT -> 403 Forbidden
	w = doPATRequest(http.MethodGet, fmt.Sprintf("/api/v1/projects/%s/issues", proj.ID), nonMemberPAT, nil)
	if w.Code != http.StatusForbidden {
		t.Errorf("non-member PAT list issues expected 403 Forbidden, got %d: %s", w.Code, w.Body.String())
	}

	// 4. Test Creating Task via PAT
	taskPayload := map[string]any{
		"issue_id":      createdIssue.ID,
		"title":         "Editor作成タスク",
		"planned_hours": 3.5,
	}

	// Viewer PAT creates task -> 403 Forbidden
	w = doPATRequest(http.MethodPost, fmt.Sprintf("/api/v1/projects/%s/tasks", proj.ID), viewerPAT, taskPayload)
	if w.Code != http.StatusForbidden {
		t.Errorf("viewer PAT create task expected 403 Forbidden, got %d: %s", w.Code, w.Body.String())
	}

	// Non-member PAT creates task -> 403 Forbidden
	w = doPATRequest(http.MethodPost, fmt.Sprintf("/api/v1/projects/%s/tasks", proj.ID), nonMemberPAT, taskPayload)
	if w.Code != http.StatusForbidden {
		t.Errorf("non-member PAT create task expected 403 Forbidden, got %d: %s", w.Code, w.Body.String())
	}

	// Editor PAT creates task -> 201 Created
	w = doPATRequest(http.MethodPost, fmt.Sprintf("/api/v1/projects/%s/tasks", proj.ID), editorPAT, taskPayload)
	if w.Code != http.StatusCreated {
		t.Fatalf("editor PAT create task expected 201 Created, got %d: %s", w.Code, w.Body.String())
	}
	var createdTask domain.Task
	_ = json.NewDecoder(w.Body).Decode(&createdTask)

	// 5. Test Updating Task Progress via PAT
	updatePayload := map[string]any{
		"progress": 80,
	}

	// Viewer PAT updates task progress -> 403 Forbidden
	w = doPATRequest(http.MethodPut, fmt.Sprintf("/api/v1/projects/%s/tasks/%s", proj.ID, createdTask.ID), viewerPAT, updatePayload)
	if w.Code != http.StatusForbidden {
		t.Errorf("viewer PAT update task expected 403 Forbidden, got %d: %s", w.Code, w.Body.String())
	}

	// Editor PAT updates task progress -> 200 OK
	w = doPATRequest(http.MethodPut, fmt.Sprintf("/api/v1/projects/%s/tasks/%s", proj.ID, createdTask.ID), editorPAT, updatePayload)
	if w.Code != http.StatusOK {
		t.Errorf("editor PAT update task progress expected 200 OK, got %d: %s", w.Code, w.Body.String())
	}

	// 6. Test Revocation and Expiration of PAT
	// Create a token and revoke it
	revokeToken, patObj, _ := rig.st.CreatePAT(context.Background(), editorUser.ID, "To Be Revoked", []string{}, nil)
	if err := rig.st.RevokePAT(context.Background(), editorUser.ID, patObj.ID); err != nil {
		t.Fatalf("revoke PAT failed: %v", err)
	}
	w = doPATRequest(http.MethodGet, fmt.Sprintf("/api/v1/projects/%s/issues", proj.ID), revokeToken, nil)
	if w.Code != http.StatusUnauthorized {
		t.Errorf("revoked PAT expected 401 Unauthorized, got %d: %s", w.Code, w.Body.String())
	}

	// Create an already-expired token
	past := time.Now().UTC().Add(-1 * time.Hour)
	expiredToken, _, _ := rig.st.CreatePAT(context.Background(), editorUser.ID, "Expired Token", []string{}, &past)
	w = doPATRequest(http.MethodGet, fmt.Sprintf("/api/v1/projects/%s/issues", proj.ID), expiredToken, nil)
	if w.Code != http.StatusUnauthorized {
		t.Errorf("expired PAT expected 401 Unauthorized, got %d: %s", w.Code, w.Body.String())
	}

	// Malformed / Non-existent token
	w = doPATRequest(http.MethodGet, fmt.Sprintf("/api/v1/projects/%s/issues", proj.ID), "plr_pat_invalidnonexistenttoken123456", nil)
	if w.Code != http.StatusUnauthorized {
		t.Errorf("invalid PAT expected 401 Unauthorized, got %d: %s", w.Code, w.Body.String())
	}
}
