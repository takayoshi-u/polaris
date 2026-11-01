package api

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/takayoshi-u/polaris/backend/internal/domain"
	"github.com/takayoshi-u/polaris/backend/internal/store"
)

func TestAdvancedPM_Unauthorized(t *testing.T) {
	srv := NewServer(nil, "http://localhost:3000", GoogleOAuthConfig{})

	endpoints := []struct {
		method string
		path   string
	}{
		{http.MethodGet, "/api/v1/projects/proj-1/dependencies"},
		{http.MethodPost, "/api/v1/projects/proj-1/dependencies"},
		{http.MethodDelete, "/api/v1/projects/proj-1/dependencies/dep-1"},
		{http.MethodGet, "/api/v1/projects/proj-1/sprints"},
		{http.MethodPost, "/api/v1/projects/proj-1/sprints"},
		{http.MethodGet, "/api/v1/sprints/sp-1"},
		{http.MethodPut, "/api/v1/sprints/sp-1"},
		{http.MethodDelete, "/api/v1/sprints/sp-1"},
		{http.MethodGet, "/api/v1/sprints/sp-1/burndown"},
		{http.MethodGet, "/api/v1/projects/proj-1/activity"},
		{http.MethodGet, "/api/v1/projects/proj-1/wiki"},
		{http.MethodPost, "/api/v1/projects/proj-1/wiki"},
		{http.MethodGet, "/api/v1/wiki/w-1"},
		{http.MethodPut, "/api/v1/wiki/w-1"},
		{http.MethodDelete, "/api/v1/wiki/w-1"},
		{http.MethodGet, "/api/v1/projects/proj-1/inbound-webhooks"},
		{http.MethodPost, "/api/v1/projects/proj-1/inbound-webhooks"},
		{http.MethodDelete, "/api/v1/inbound-webhooks/h-1"},
		{http.MethodGet, "/api/v1/projects/proj-1/export/csv"},
		{http.MethodPost, "/api/v1/projects/proj-1/import/csv"},
	}

	for _, ep := range endpoints {
		req := httptest.NewRequest(ep.method, ep.path, bytes.NewReader([]byte("{}")))
		rec := httptest.NewRecorder()
		srv.ServeHTTP(rec, req)
		if rec.Code != http.StatusUnauthorized {
			t.Errorf("expected 401 for %s %s, got %d", ep.method, ep.path, rec.Code)
		}
	}
}

func TestAdvancedPM_InboundWebhookPublicEndpointRequiresValidJSON(t *testing.T) {
	srv := NewServer(nil, "http://localhost:3000", GoogleOAuthConfig{})

	// Public endpoint without auth: sending invalid JSON gives 400 (not 401)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/inbound-webhooks/some-token", bytes.NewReader([]byte("invalid json")))
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid JSON on public webhook endpoint, got %d", rec.Code)
	}
}

func TestAdvancedPM_FullIntegration(t *testing.T) {
	rig := setupTestRig(t)
	defer rig.db.Close()

	ctx := t.Context()
	srv := rig.srv
	cookie := rig.adminCookie

	// 1. Create a Project
	projBody, _ := json.Marshal(map[string]any{
		"key":           "ADV",
		"name":          "Advanced PM Project",
		"description":   "Project for testing advanced features",
		"hours_per_day": 8.0,
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/projects", bytes.NewReader(projBody))
	req.AddCookie(cookie)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("failed to create project (%d): %s", rec.Code, rec.Body.String())
	}
	var proj domain.Project
	_ = json.NewDecoder(rec.Body).Decode(&proj)

	// Fetch default statuses
	req = httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/v1/projects/%s/statuses", proj.ID), nil)
	req.AddCookie(cookie)
	rec = httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	var statuses []domain.Status
	_ = json.NewDecoder(rec.Body).Decode(&statuses)
	if len(statuses) == 0 {
		t.Fatalf("expected default statuses for project")
	}
	defaultStatusID := statuses[0].ID

	// 2. Create an Issue
	issueBody, _ := json.Marshal(map[string]any{
		"title":       "Core Feature Issue",
		"description": "Issue containing tasks",
	})
	req = httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/v1/projects/%s/issues", proj.ID), bytes.NewReader(issueBody))
	req.AddCookie(cookie)
	rec = httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("failed to create issue (%d): %s", rec.Code, rec.Body.String())
	}
	var issue domain.Issue
	_ = json.NewDecoder(rec.Body).Decode(&issue)

	// 3. Create Two Tasks
	t1Body, _ := json.Marshal(map[string]any{
		"title":     "Task 1",
		"status_id": defaultStatusID,
	})
	req = httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/v1/issues/%s/tasks", issue.ID), bytes.NewReader(t1Body))
	req.AddCookie(cookie)
	rec = httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	var task1 domain.Task
	_ = json.NewDecoder(rec.Body).Decode(&task1)

	t2Body, _ := json.Marshal(map[string]any{
		"title":     "Task 2",
		"status_id": defaultStatusID,
	})
	req = httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/v1/issues/%s/tasks", issue.ID), bytes.NewReader(t2Body))
	req.AddCookie(cookie)
	rec = httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	var task2 domain.Task
	_ = json.NewDecoder(rec.Body).Decode(&task2)

	// ==========================================
	// Feature A: Task Dependencies & Cycle Prevention
	// ==========================================
	// A.1: Self dependency rejection
	selfDepBody, _ := json.Marshal(map[string]any{
		"predecessor_id":  task1.ID,
		"successor_id":    task1.ID,
		"dependency_type": domain.DependencyFS,
		"lag_days":        0,
	})
	req = httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/v1/projects/%s/dependencies", proj.ID), bytes.NewReader(selfDepBody))
	req.AddCookie(cookie)
	rec = httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for self-dependency, got %d", rec.Code)
	}

	// A.2: Create valid dependency (Task 1 -> Task 2)
	depBody, _ := json.Marshal(map[string]any{
		"predecessor_id":  task1.ID,
		"successor_id":    task2.ID,
		"dependency_type": domain.DependencyFS,
		"lag_days":        1,
	})
	req = httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/v1/projects/%s/dependencies", proj.ID), bytes.NewReader(depBody))
	req.AddCookie(cookie)
	rec = httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("failed to create valid dependency (%d): %s", rec.Code, rec.Body.String())
	}
	var createdDep domain.TaskDependency
	_ = json.NewDecoder(rec.Body).Decode(&createdDep)

	// A.3: Circular dependency rejection (Task 2 -> Task 1)
	cycleDepBody, _ := json.Marshal(map[string]any{
		"predecessor_id":  task2.ID,
		"successor_id":    task1.ID,
		"dependency_type": domain.DependencyFS,
		"lag_days":        0,
	})
	req = httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/v1/projects/%s/dependencies", proj.ID), bytes.NewReader(cycleDepBody))
	req.AddCookie(cookie)
	rec = httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for circular dependency, got %d", rec.Code)
	}

	// A.4: List dependencies
	req = httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/v1/projects/%s/dependencies", proj.ID), nil)
	req.AddCookie(cookie)
	rec = httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("failed to list dependencies (%d)", rec.Code)
	}
	var deps []domain.TaskDependency
	_ = json.NewDecoder(rec.Body).Decode(&deps)
	if len(deps) != 1 || deps[0].ID != createdDep.ID {
		t.Fatalf("expected 1 dependency, got %d", len(deps))
	}

	// A.5: Delete dependency
	req = httptest.NewRequest(http.MethodDelete, fmt.Sprintf("/api/v1/projects/%s/dependencies/%s", proj.ID, createdDep.ID), nil)
	req.AddCookie(cookie)
	rec = httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("failed to delete dependency (%d): %s", rec.Code, rec.Body.String())
	}

	// ==========================================
	// Feature B: Sprints & Burndown
	// ==========================================
	spStart := time.Now().Format("2006-01-02")
	spEnd := time.Now().AddDate(0, 0, 14).Format("2006-01-02")
	spBody, _ := json.Marshal(map[string]any{
		"name":       "Sprint 1",
		"goal":       "Release MVP",
		"start_date": spStart,
		"end_date":   spEnd,
		"status":     "planned",
	})
	req = httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/v1/projects/%s/sprints", proj.ID), bytes.NewReader(spBody))
	req.AddCookie(cookie)
	rec = httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("failed to create sprint (%d): %s", rec.Code, rec.Body.String())
	}
	var sp domain.Sprint
	_ = json.NewDecoder(rec.Body).Decode(&sp)
	if sp.StartDate == nil || *sp.StartDate != spStart {
		t.Fatalf("expected sprint start_date %s, got %v", spStart, sp.StartDate)
	}
	if sp.EndDate == nil || *sp.EndDate != spEnd {
		t.Fatalf("expected sprint end_date %s, got %v", spEnd, sp.EndDate)
	}

	// Update sprint to active
	newSpEnd := time.Now().AddDate(0, 0, 21).Format("2006-01-02")
	upSpBody, _ := json.Marshal(map[string]any{
		"name":       "Sprint 1 (Active)",
		"goal":       "Ship MVP Now",
		"start_date": spStart,
		"end_date":   newSpEnd,
		"status":     "active",
	})
	req = httptest.NewRequest(http.MethodPut, fmt.Sprintf("/api/v1/sprints/%s", sp.ID), bytes.NewReader(upSpBody))
	req.AddCookie(cookie)
	rec = httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("failed to update sprint (%d): %s", rec.Code, rec.Body.String())
	}
	var updatedSp domain.Sprint
	_ = json.NewDecoder(rec.Body).Decode(&updatedSp)
	if updatedSp.StartDate == nil || *updatedSp.StartDate != spStart {
		t.Fatalf("expected updated sprint start_date %s, got %v", spStart, updatedSp.StartDate)
	}
	if updatedSp.EndDate == nil || *updatedSp.EndDate != newSpEnd {
		t.Fatalf("expected updated sprint end_date %s, got %v", newSpEnd, updatedSp.EndDate)
	}

	// Verify updated dates are persisted via GET
	req = httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/v1/sprints/%s", sp.ID), nil)
	req.AddCookie(cookie)
	rec = httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("failed to get updated sprint (%d): %s", rec.Code, rec.Body.String())
	}
	var fetchedSp domain.Sprint
	_ = json.NewDecoder(rec.Body).Decode(&fetchedSp)
	if fetchedSp.StartDate == nil || *fetchedSp.StartDate != spStart {
		t.Fatalf("expected persisted sprint start_date %s, got %v", spStart, fetchedSp.StartDate)
	}
	if fetchedSp.EndDate == nil || *fetchedSp.EndDate != newSpEnd {
		t.Fatalf("expected persisted sprint end_date %s, got %v", newSpEnd, fetchedSp.EndDate)
	}

	// Assign task to sprint with story points
	pts := 5.0
	upTaskBody, _ := json.Marshal(map[string]any{
		"title":        task1.Title,
		"status_id":    task1.StatusID,
		"sprint_id":    sp.ID,
		"story_points": pts,
	})
	req = httptest.NewRequest(http.MethodPut, fmt.Sprintf("/api/v1/tasks/%s", task1.ID), bytes.NewReader(upTaskBody))
	req.AddCookie(cookie)
	rec = httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("failed to assign task to sprint (%d): %s", rec.Code, rec.Body.String())
	}
	var assignedTask domain.Task
	_ = json.NewDecoder(rec.Body).Decode(&assignedTask)
	if assignedTask.SprintID == nil || *assignedTask.SprintID != sp.ID {
		t.Fatalf("expected task sprint_id %s, got %v", sp.ID, assignedTask.SprintID)
	}
	if assignedTask.StoryPoints == nil || *assignedTask.StoryPoints != pts {
		t.Fatalf("expected task story_points %f, got %v", pts, assignedTask.StoryPoints)
	}

	// Burndown
	req = httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/v1/sprints/%s/burndown", sp.ID), nil)
	req.AddCookie(cookie)
	rec = httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("failed to get burndown (%d): %s", rec.Code, rec.Body.String())
	}
	var burndown domain.SprintBurndown
	_ = json.NewDecoder(rec.Body).Decode(&burndown)
	if burndown.TotalSP != 5.0 {
		t.Fatalf("expected 5.0 total story points, got %f", burndown.TotalSP)
	}
	if len(burndown.Points) == 0 {
		t.Fatalf("expected data points in burndown")
	}

	// ==========================================
	// Feature C: Activity Logs
	// ==========================================
	req = httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/v1/projects/%s/activity?limit=20", proj.ID), nil)
	req.AddCookie(cookie)
	rec = httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("failed to get activity logs (%d): %s", rec.Code, rec.Body.String())
	}
	var logs []domain.ActivityLog
	_ = json.NewDecoder(rec.Body).Decode(&logs)
	if len(logs) == 0 {
		t.Fatalf("expected activity log entries for created tasks and sprint changes")
	}

	// ==========================================
	// Feature D: Project Wiki
	// ==========================================
	wikiBody, _ := json.Marshal(map[string]any{
		"title":   "Project Architecture",
		"slug":    "architecture",
		"content": "# Polaris Architecture\n\nGo backend and Nuxt 3 frontend.",
	})
	req = httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/v1/projects/%s/wiki", proj.ID), bytes.NewReader(wikiBody))
	req.AddCookie(cookie)
	rec = httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("failed to create wiki page (%d): %s", rec.Code, rec.Body.String())
	}
	var wiki domain.WikiPage
	_ = json.NewDecoder(rec.Body).Decode(&wiki)

	// Update wiki page
	upWikiBody, _ := json.Marshal(map[string]any{
		"title":   "Project Architecture (Updated)",
		"slug":    "architecture",
		"content": "# Updated Polaris Architecture\n\nWith advanced PM tools.",
	})
	req = httptest.NewRequest(http.MethodPut, fmt.Sprintf("/api/v1/wiki/%s", wiki.ID), bytes.NewReader(upWikiBody))
	req.AddCookie(cookie)
	rec = httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("failed to update wiki (%d): %s", rec.Code, rec.Body.String())
	}

	// List wiki pages
	req = httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/v1/projects/%s/wiki", proj.ID), nil)
	req.AddCookie(cookie)
	rec = httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("failed to list wiki pages (%d)", rec.Code)
	}
	var wikiList []domain.WikiPage
	_ = json.NewDecoder(rec.Body).Decode(&wikiList)
	if len(wikiList) != 1 {
		t.Fatalf("expected 1 wiki page, got %d", len(wikiList))
	}

	// ==========================================
	// Feature E: Inbound Webhooks
	// ==========================================
	webhookCreateBody, _ := json.Marshal(map[string]any{
		"name":              "GitHub Issues Inbound",
		"default_issue_id":  issue.ID,
		"default_status_id": defaultStatusID,
	})
	req = httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/v1/projects/%s/inbound-webhooks", proj.ID), bytes.NewReader(webhookCreateBody))
	req.AddCookie(cookie)
	rec = httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("failed to create inbound webhook (%d): %s", rec.Code, rec.Body.String())
	}
	var inHook domain.InboundWebhook
	_ = json.NewDecoder(rec.Body).Decode(&inHook)
	if inHook.TokenPlain == "" {
		t.Fatalf("expected generated token for inbound webhook")
	}
	if inHook.Secret == "" {
		t.Fatalf("expected generated secret for inbound webhook on create")
	}

	// Verify secret masking in ListInboundWebhooks
	req = httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/v1/projects/%s/inbound-webhooks", proj.ID), nil)
	req.AddCookie(cookie)
	rec = httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("failed to list inbound webhooks (%d)", rec.Code)
	}
	var listHooks []domain.InboundWebhook
	_ = json.NewDecoder(rec.Body).Decode(&listHooks)
	if len(listHooks) != 1 || listHooks[0].Secret != "" {
		t.Fatalf("expected inbound webhook secret to be masked in list response, got %q", listHooks[0].Secret)
	}

	// Test triggering the public webhook receiver with valid token (no signature)
	hookPayload, _ := json.Marshal(map[string]any{
		"title":       "Bug reported via Inbound Webhook",
		"description": "Details from crash reporter",
		"priority":    "high",
	})
	req = httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/v1/inbound-webhooks/%s", inHook.TokenPlain), bytes.NewReader(hookPayload))
	rec = httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("failed to process inbound webhook (%d): %s", rec.Code, rec.Body.String())
	}
	var webhookCreatedTask domain.Task
	_ = json.NewDecoder(rec.Body).Decode(&webhookCreatedTask)
	if webhookCreatedTask.Title != "Bug reported via Inbound Webhook" {
		t.Fatalf("unexpected task title: %s", webhookCreatedTask.Title)
	}

	// Test HMAC-SHA256 signature verification (invalid signature -> 401)
	req = httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/v1/inbound-webhooks/%s", inHook.TokenPlain), bytes.NewReader(hookPayload))
	req.Header.Set("X-Polaris-Signature", "sha256=invalidhexsignature0000000000000000000000000000000000000000000000")
	rec = httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for invalid HMAC signature, got %d", rec.Code)
	}

	// Test HMAC-SHA256 signature verification (valid signature -> 201)
	mac := hmac.New(sha256.New, []byte(inHook.Secret))
	mac.Write(hookPayload)
	expectedSig := hex.EncodeToString(mac.Sum(nil))

	req = httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/v1/inbound-webhooks/%s", inHook.TokenPlain), bytes.NewReader(hookPayload))
	req.Header.Set("X-Polaris-Signature", "sha256="+expectedSig)
	rec = httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201 for valid HMAC signature, got %d: %s", rec.Code, rec.Body.String())
	}

	// Test HMAC-SHA256 signature verification with X-Signature-256 header
	req = httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/v1/inbound-webhooks/%s", inHook.TokenPlain), bytes.NewReader(hookPayload))
	req.Header.Set("X-Signature-256", expectedSig)
	rec = httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201 for valid X-Signature-256 header, got %d: %s", rec.Code, rec.Body.String())
	}

	// ==========================================
	// Feature F: IDOR & Project Boundary Validation
	// ==========================================
	// Create Project 2
	proj2Body, _ := json.Marshal(map[string]any{
		"key":         "PR2",
		"name":        "Project 2",
		"description": "Second project for boundary testing",
	})
	req = httptest.NewRequest(http.MethodPost, "/api/v1/projects", bytes.NewReader(proj2Body))
	req.AddCookie(cookie)
	rec = httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	var proj2 domain.Project
	_ = json.NewDecoder(rec.Body).Decode(&proj2)

	// Create issue in Project 2
	issue2Body, _ := json.Marshal(map[string]any{
		"title": "Project 2 Issue",
	})
	req = httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/v1/projects/%s/issues", proj2.ID), bytes.NewReader(issue2Body))
	req.AddCookie(cookie)
	rec = httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	var issue2 domain.Issue
	_ = json.NewDecoder(rec.Body).Decode(&issue2)

	// Create wiki in Project 2
	wiki2Body, _ := json.Marshal(map[string]any{
		"title":   "Project 2 Wiki",
		"slug":    "proj2-wiki",
		"content": "Secret doc in project 2",
	})
	req = httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/v1/projects/%s/wiki", proj2.ID), bytes.NewReader(wiki2Body))
	req.AddCookie(cookie)
	rec = httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	var wiki2 domain.WikiPage
	_ = json.NewDecoder(rec.Body).Decode(&wiki2)

	// Inbound webhook in Project 1 with default_issue_id from Project 2 -> reject
	crossHookBody, _ := json.Marshal(map[string]any{
		"name":              "Cross Project Webhook",
		"default_issue_id":  issue2.ID,
		"default_status_id": defaultStatusID,
	})
	req = httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/v1/projects/%s/inbound-webhooks", proj.ID), bytes.NewReader(crossHookBody))
	req.AddCookie(cookie)
	rec = httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for cross-project default_issue_id, got %d", rec.Code)
	}

	// Create Wiki in Project 1 with parent_id from Project 2 -> reject
	crossWikiBody, _ := json.Marshal(map[string]any{
		"title":     "Cross Project Child Wiki",
		"slug":      "cross-child",
		"content":   "test",
		"parent_id": wiki2.ID,
	})
	req = httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/v1/projects/%s/wiki", proj.ID), bytes.NewReader(crossWikiBody))
	req.AddCookie(cookie)
	rec = httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for cross-project wiki parent_id on create, got %d", rec.Code)
	}

	// Update Wiki in Project 1 with parent_id from Project 2 -> reject
	crossUpWikiBody, _ := json.Marshal(map[string]any{
		"parent_id": wiki2.ID,
	})
	req = httptest.NewRequest(http.MethodPut, fmt.Sprintf("/api/v1/wiki/%s", wiki.ID), bytes.NewReader(crossUpWikiBody))
	req.AddCookie(cookie)
	rec = httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for cross-project wiki parent_id on update, got %d", rec.Code)
	}

	// ==========================================
	// Feature G: CSV Export, Formula Injection & Import
	// ==========================================
	// Create task with potentially malicious formula characters
	formulaTaskBody, _ := json.Marshal(map[string]any{
		"title":     "=cmd|' /C calc'!A0",
		"status_id": defaultStatusID,
	})
	req = httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/v1/issues/%s/tasks", issue.ID), bytes.NewReader(formulaTaskBody))
	req.AddCookie(cookie)
	rec = httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("failed to create formula task: %s", rec.Body.String())
	}

	// Export CSV and verify formula sanitization
	req = httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/v1/projects/%s/export/csv", proj.ID), nil)
	req.AddCookie(cookie)
	rec = httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("failed to export CSV (%d): %s", rec.Code, rec.Body.String())
	}
	csvContent := rec.Body.String()
	if !bytes.Contains([]byte(csvContent), []byte("Task Title")) {
		t.Fatalf("expected Task Title in CSV export header, got: %s", csvContent)
	}
	// Verify sanitized cell with prepended single quote
	if !bytes.Contains([]byte(csvContent), []byte("'=cmd|' /C calc'!A0")) {
		t.Fatalf("expected CSV formula sanitization to prepend single quote, got: %s", csvContent)
	}

	// Import CSV with a new task
	csvToImport := "Issue Title,Task Title,Status,Planned Hours,Story Points\n" +
		fmt.Sprintf("%s,Imported Task via CSV,%s,4.0,3\n", issue.Title, statuses[0].Name)
	req = httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/v1/projects/%s/import/csv", proj.ID), bytes.NewReader([]byte(csvToImport)))
	req.Header.Set("Content-Type", "text/csv")
	req.AddCookie(cookie)
	rec = httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("failed to import CSV (%d): %s", rec.Code, rec.Body.String())
	}
	var importRes store.CSVImportResult
	_ = json.NewDecoder(rec.Body).Decode(&importRes)
	if importRes.ImportedTasks != 1 {
		t.Fatalf("expected 1 task imported, got %d (errors: %v)", importRes.ImportedTasks, importRes.Errors)
	}

	// ==========================================
	// Feature H: PAT Scope Enforcement
	// ==========================================
	// Create PAT with only read:projects scope
	patBody, _ := json.Marshal(map[string]any{
		"name":   "ReadOnly PAT",
		"scopes": []string{"read:projects"},
	})
	req = httptest.NewRequest(http.MethodPost, "/api/v1/user/tokens", bytes.NewReader(patBody))
	req.AddCookie(cookie)
	rec = httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("failed to create PAT (%d): %s", rec.Code, rec.Body.String())
	}
	var patRes struct {
		Token string `json:"token"`
	}
	_ = json.NewDecoder(rec.Body).Decode(&patRes)

	// Attempt PUT /sprints/{sprintId} with read-only PAT -> 403
	req = httptest.NewRequest(http.MethodPut, fmt.Sprintf("/api/v1/sprints/%s", sp.ID), bytes.NewReader(upSpBody))
	req.Header.Set("Authorization", "Bearer "+patRes.Token)
	rec = httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 Forbidden for PUT sprint with read-only PAT, got %d", rec.Code)
	}

	// Attempt PUT /wiki/{wikiId} with read-only PAT -> 403
	req = httptest.NewRequest(http.MethodPut, fmt.Sprintf("/api/v1/wiki/%s", wiki.ID), bytes.NewReader(upWikiBody))
	req.Header.Set("Authorization", "Bearer "+patRes.Token)
	rec = httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 Forbidden for PUT wiki with read-only PAT, got %d", rec.Code)
	}

	// Attempt DELETE /inbound-webhooks/{webhookId} with read-only PAT -> 403
	req = httptest.NewRequest(http.MethodDelete, fmt.Sprintf("/api/v1/inbound-webhooks/%s", inHook.ID), nil)
	req.Header.Set("Authorization", "Bearer "+patRes.Token)
	rec = httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 Forbidden for DELETE inbound webhook with read-only PAT, got %d", rec.Code)
	}

	_ = ctx
}

func TestTask_SprintAndStoryPointsRoundTrip(t *testing.T) {
	rig := setupTestRig(t)
	defer rig.db.Close()

	srv := rig.srv
	cookie := rig.adminCookie

	// 1. Create a Project
	projBody, _ := json.Marshal(map[string]any{
		"key":           "RND",
		"name":          "RoundTrip PM Project",
		"description":   "Project for round-trip testing",
		"hours_per_day": 8.0,
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/projects", bytes.NewReader(projBody))
	req.AddCookie(cookie)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("failed to create project (%d): %s", rec.Code, rec.Body.String())
	}
	var proj domain.Project
	_ = json.NewDecoder(rec.Body).Decode(&proj)

	// Fetch default status
	req = httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/v1/projects/%s/statuses", proj.ID), nil)
	req.AddCookie(cookie)
	rec = httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	var statuses []domain.Status
	_ = json.NewDecoder(rec.Body).Decode(&statuses)
	defaultStatusID := statuses[0].ID

	// 2. Create Issue
	issueBody, _ := json.Marshal(map[string]any{
		"title": "RoundTrip Issue",
	})
	req = httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/v1/projects/%s/issues", proj.ID), bytes.NewReader(issueBody))
	req.AddCookie(cookie)
	rec = httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("failed to create issue (%d): %s", rec.Code, rec.Body.String())
	}
	var issue domain.Issue
	_ = json.NewDecoder(rec.Body).Decode(&issue)

	// 3. Create two Sprints: sp1 and sp2
	sp1Body, _ := json.Marshal(map[string]any{
		"name":   "Sprint A",
		"status": "planned",
	})
	req = httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/v1/projects/%s/sprints", proj.ID), bytes.NewReader(sp1Body))
	req.AddCookie(cookie)
	rec = httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("failed to create sprint 1 (%d): %s", rec.Code, rec.Body.String())
	}
	var sp1 domain.Sprint
	_ = json.NewDecoder(rec.Body).Decode(&sp1)

	sp2Body, _ := json.Marshal(map[string]any{
		"name":   "Sprint B",
		"status": "planned",
	})
	req = httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/v1/projects/%s/sprints", proj.ID), bytes.NewReader(sp2Body))
	req.AddCookie(cookie)
	rec = httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("failed to create sprint 2 (%d): %s", rec.Code, rec.Body.String())
	}
	var sp2 domain.Sprint
	_ = json.NewDecoder(rec.Body).Decode(&sp2)

	// a. Create task with story_points & sprint_id -> assert values
	createPts := 3.5
	createTaskBody, _ := json.Marshal(map[string]any{
		"issue_id":     issue.ID,
		"status_id":    defaultStatusID,
		"title":        "Task with Sprint & SP",
		"sprint_id":    sp1.ID,
		"story_points": createPts,
	})
	req = httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/v1/projects/%s/tasks", proj.ID), bytes.NewReader(createTaskBody))
	req.AddCookie(cookie)
	rec = httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("failed to create task (%d): %s", rec.Code, rec.Body.String())
	}
	var createdTask domain.Task
	_ = json.NewDecoder(rec.Body).Decode(&createdTask)
	if createdTask.SprintID == nil || *createdTask.SprintID != sp1.ID {
		t.Fatalf("expected task sprint_id %s, got %v", sp1.ID, createdTask.SprintID)
	}
	if createdTask.StoryPoints == nil || *createdTask.StoryPoints != createPts {
		t.Fatalf("expected task story_points %f, got %v", createPts, createdTask.StoryPoints)
	}

	// Verify persistence via GET
	req = httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/v1/projects/%s/tasks/%s", proj.ID, createdTask.ID), nil)
	req.AddCookie(cookie)
	rec = httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("failed to get task (%d): %s", rec.Code, rec.Body.String())
	}
	var getTask domain.Task
	_ = json.NewDecoder(rec.Body).Decode(&getTask)
	if getTask.SprintID == nil || *getTask.SprintID != sp1.ID {
		t.Fatalf("expected persisted task sprint_id %s, got %v", sp1.ID, getTask.SprintID)
	}
	if getTask.StoryPoints == nil || *getTask.StoryPoints != createPts {
		t.Fatalf("expected persisted task story_points %f, got %v", createPts, getTask.StoryPoints)
	}

	// b. Update task with new story_points & sprint_id -> assert values
	newPts := 8.0
	updateTaskBody, _ := json.Marshal(map[string]any{
		"sprint_id":    sp2.ID,
		"story_points": newPts,
	})
	req = httptest.NewRequest(http.MethodPut, fmt.Sprintf("/api/v1/tasks/%s", createdTask.ID), bytes.NewReader(updateTaskBody))
	req.AddCookie(cookie)
	rec = httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("failed to update task (%d): %s", rec.Code, rec.Body.String())
	}
	var updatedTask domain.Task
	_ = json.NewDecoder(rec.Body).Decode(&updatedTask)
	if updatedTask.SprintID == nil || *updatedTask.SprintID != sp2.ID {
		t.Fatalf("expected updated task sprint_id %s, got %v", sp2.ID, updatedTask.SprintID)
	}
	if updatedTask.StoryPoints == nil || *updatedTask.StoryPoints != newPts {
		t.Fatalf("expected updated task story_points %f, got %v", newPts, updatedTask.StoryPoints)
	}

	// Verify update persistence via GET
	req = httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/v1/projects/%s/tasks/%s", proj.ID, createdTask.ID), nil)
	req.AddCookie(cookie)
	rec = httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("failed to get updated task (%d): %s", rec.Code, rec.Body.String())
	}
	_ = json.NewDecoder(rec.Body).Decode(&getTask)
	if getTask.SprintID == nil || *getTask.SprintID != sp2.ID {
		t.Fatalf("expected persisted updated task sprint_id %s, got %v", sp2.ID, getTask.SprintID)
	}
	if getTask.StoryPoints == nil || *getTask.StoryPoints != newPts {
		t.Fatalf("expected persisted updated task story_points %f, got %v", newPts, getTask.StoryPoints)
	}

	// c. Update task with null story_points & null sprint_id -> assert cleared to nil
	clearTaskBody, _ := json.Marshal(map[string]any{
		"sprint_id":    nil,
		"story_points": nil,
	})
	req = httptest.NewRequest(http.MethodPut, fmt.Sprintf("/api/v1/tasks/%s", createdTask.ID), bytes.NewReader(clearTaskBody))
	req.AddCookie(cookie)
	rec = httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("failed to clear task sprint and sp (%d): %s", rec.Code, rec.Body.String())
	}
	var clearedTask domain.Task
	_ = json.NewDecoder(rec.Body).Decode(&clearedTask)
	if clearedTask.SprintID != nil {
		t.Fatalf("expected cleared task sprint_id to be nil, got %v", *clearedTask.SprintID)
	}
	if clearedTask.StoryPoints != nil {
		t.Fatalf("expected cleared task story_points to be nil, got %v", *clearedTask.StoryPoints)
	}

	// Verify clear persistence via GET
	req = httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/v1/projects/%s/tasks/%s", proj.ID, createdTask.ID), nil)
	req.AddCookie(cookie)
	rec = httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("failed to get cleared task (%d): %s", rec.Code, rec.Body.String())
	}
	_ = json.NewDecoder(rec.Body).Decode(&getTask)
	if getTask.SprintID != nil {
		t.Fatalf("expected persisted cleared task sprint_id to be nil, got %v", *getTask.SprintID)
	}
	if getTask.StoryPoints != nil {
		t.Fatalf("expected persisted cleared task story_points to be nil, got %v", *getTask.StoryPoints)
	}
}

func TestAgileEnhancements_PremortemPostmortemAndSprintFilter(t *testing.T) {
	rig := setupTestRig(t)
	defer rig.db.Close()

	srv := rig.srv
	cookie := rig.adminCookie

	// 1. Create Project
	projBody, _ := json.Marshal(map[string]any{
		"name":          "Agile Enhancements Project",
		"description":   "Testing premortem/postmortem, sprint filters, and story point rollups",
		"hours_per_day": 8.0,
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/projects", bytes.NewReader(projBody))
	req.AddCookie(cookie)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("failed to create project (%d): %s", rec.Code, rec.Body.String())
	}
	var proj domain.Project
	_ = json.NewDecoder(rec.Body).Decode(&proj)

	// Fetch statuses (first = todo, last = done)
	req = httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/v1/projects/%s/statuses", proj.ID), nil)
	req.AddCookie(cookie)
	rec = httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	var statuses []domain.Status
	_ = json.NewDecoder(rec.Body).Decode(&statuses)
	if len(statuses) < 2 {
		t.Fatalf("expected at least 2 statuses, got %d", len(statuses))
	}
	todoStatusID := statuses[0].ID
	doneStatusID := statuses[len(statuses)-1].ID

	// 2. Issue Premortem & Postmortem Round-Trip (Create -> Get -> Update -> Clear)
	createIssueBody, _ := json.Marshal(map[string]any{
		"title":       "Incident Prevention Epic",
		"description": "Epic description",
		"premortem":   "Risk: DB connection pool exhaustion",
		"postmortem":  "Initial postmortem note",
	})
	req = httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/v1/projects/%s/issues", proj.ID), bytes.NewReader(createIssueBody))
	req.AddCookie(cookie)
	rec = httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("failed to create issue (%d): %s", rec.Code, rec.Body.String())
	}
	var createdIssue domain.Issue
	_ = json.NewDecoder(rec.Body).Decode(&createdIssue)
	if createdIssue.Premortem != "Risk: DB connection pool exhaustion" {
		t.Fatalf("expected premortem set on create, got %q", createdIssue.Premortem)
	}
	if createdIssue.Postmortem != "Initial postmortem note" {
		t.Fatalf("expected postmortem set on create, got %q", createdIssue.Postmortem)
	}

	// Verify GET persistence
	req = httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/v1/projects/%s/issues/%s", proj.ID, createdIssue.ID), nil)
	req.AddCookie(cookie)
	rec = httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	var fetchedIssue domain.Issue
	_ = json.NewDecoder(rec.Body).Decode(&fetchedIssue)
	if fetchedIssue.Premortem != "Risk: DB connection pool exhaustion" || fetchedIssue.Postmortem != "Initial postmortem note" {
		t.Fatalf("expected persisted premortem/postmortem, got %q / %q", fetchedIssue.Premortem, fetchedIssue.Postmortem)
	}

	// Update premortem & postmortem
	updateIssueBody, _ := json.Marshal(map[string]any{
		"premortem":  "Updated Risk: Cache stampede",
		"postmortem": "Resolved with singleflight",
	})
	req = httptest.NewRequest(http.MethodPut, fmt.Sprintf("/api/v1/projects/%s/issues/%s", proj.ID, createdIssue.ID), bytes.NewReader(updateIssueBody))
	req.AddCookie(cookie)
	rec = httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("failed to update issue (%d): %s", rec.Code, rec.Body.String())
	}
	var updatedIssue domain.Issue
	_ = json.NewDecoder(rec.Body).Decode(&updatedIssue)
	if updatedIssue.Premortem != "Updated Risk: Cache stampede" || updatedIssue.Postmortem != "Resolved with singleflight" {
		t.Fatalf("expected updated premortem/postmortem, got %q / %q", updatedIssue.Premortem, updatedIssue.Postmortem)
	}

	// Clear premortem & postmortem with empty string
	clearIssueBody, _ := json.Marshal(map[string]any{
		"premortem":  "",
		"postmortem": "",
	})
	req = httptest.NewRequest(http.MethodPut, fmt.Sprintf("/api/v1/projects/%s/issues/%s", proj.ID, createdIssue.ID), bytes.NewReader(clearIssueBody))
	req.AddCookie(cookie)
	rec = httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("failed to clear issue premortem/postmortem (%d): %s", rec.Code, rec.Body.String())
	}
	var clearedIssue domain.Issue
	_ = json.NewDecoder(rec.Body).Decode(&clearedIssue)
	if clearedIssue.Premortem != "" || clearedIssue.Postmortem != "" {
		t.Fatalf("expected cleared premortem/postmortem, got %q / %q", clearedIssue.Premortem, clearedIssue.Postmortem)
	}

	// 3. Create Active Sprint & Planned Sprint
	spActiveBody, _ := json.Marshal(map[string]any{
		"name":   "Active Sprint 1",
		"status": "active",
	})
	req = httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/v1/projects/%s/sprints", proj.ID), bytes.NewReader(spActiveBody))
	req.AddCookie(cookie)
	rec = httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	var activeSprint domain.Sprint
	_ = json.NewDecoder(rec.Body).Decode(&activeSprint)

	// 4. Create 3 Tasks:
	// - Task A: in activeSprint, SP=3, progress=100 (todo status) -> completed via progress
	// - Task B: in activeSprint, SP=5, progress=20 (done status) -> completed via Done status
	// - Task C: unassigned sprint, SP=2, progress=0 (todo status) -> not completed
	for _, tc := range []map[string]any{
		{"issue_id": createdIssue.ID, "title": "Task A", "status_id": todoStatusID, "sprint_id": activeSprint.ID, "story_points": 3.0, "progress": 100},
		{"issue_id": createdIssue.ID, "title": "Task B", "status_id": doneStatusID, "sprint_id": activeSprint.ID, "story_points": 5.0, "progress": 20},
		{"issue_id": createdIssue.ID, "title": "Task C", "status_id": todoStatusID, "story_points": 2.0, "progress": 0},
	} {
		b, _ := json.Marshal(tc)
		req = httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/v1/projects/%s/tasks", proj.ID), bytes.NewReader(b))
		req.AddCookie(cookie)
		rec = httptest.NewRecorder()
		srv.ServeHTTP(rec, req)
		if rec.Code != http.StatusCreated {
			t.Fatalf("failed to create task (%d): %s", rec.Code, rec.Body.String())
		}
	}

	// 5. Verify Issue Rollup SP
	req = httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/v1/projects/%s/issues/%s", proj.ID, createdIssue.ID), nil)
	req.AddCookie(cookie)
	rec = httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	_ = json.NewDecoder(rec.Body).Decode(&fetchedIssue)
	if fetchedIssue.Rollup.TotalStoryPoints != 10.0 {
		t.Fatalf("expected issue TotalStoryPoints 10.0, got %v", fetchedIssue.Rollup.TotalStoryPoints)
	}
	if fetchedIssue.Rollup.CompletedStoryPoints != 8.0 {
		t.Fatalf("expected issue CompletedStoryPoints 8.0, got %v", fetchedIssue.Rollup.CompletedStoryPoints)
	}

	// 6. Verify Sprint CompletedTaskCount & CompletedStoryPoints in GET & List
	req = httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/v1/sprints/%s", activeSprint.ID), nil)
	req.AddCookie(cookie)
	rec = httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	var gotSprint domain.Sprint
	_ = json.NewDecoder(rec.Body).Decode(&gotSprint)
	if gotSprint.TaskCount != 2 || gotSprint.CompletedTaskCount != 2 {
		t.Fatalf("expected sprint TaskCount=2, CompletedTaskCount=2, got %d / %d", gotSprint.TaskCount, gotSprint.CompletedTaskCount)
	}
	if gotSprint.TotalStoryPoints != 8.0 || gotSprint.CompletedStoryPoints != 8.0 {
		t.Fatalf("expected sprint TotalStoryPoints=8.0, CompletedStoryPoints=8.0, got %v / %v", gotSprint.TotalStoryPoints, gotSprint.CompletedStoryPoints)
	}

	// 7. Verify GET /projects/{id}/issues?sprint_id=... filters
	// 7a: sprint_id=unassigned -> only Task C
	req = httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/v1/projects/%s/issues?sprint_id=unassigned", proj.ID), nil)
	req.AddCookie(cookie)
	rec = httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	var unassignedIssues []domain.Issue
	_ = json.NewDecoder(rec.Body).Decode(&unassignedIssues)
	if len(unassignedIssues) != 1 || len(unassignedIssues[0].Tasks) != 1 || unassignedIssues[0].Tasks[0].Title != "Task C" {
		t.Fatalf("expected 1 issue with Task C for sprint_id=unassigned, got %+v", unassignedIssues)
	}

	// 7b: sprint_id=active -> Task A & Task B
	req = httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/v1/projects/%s/issues?sprint_id=active", proj.ID), nil)
	req.AddCookie(cookie)
	rec = httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	var activeIssues []domain.Issue
	_ = json.NewDecoder(rec.Body).Decode(&activeIssues)
	if len(activeIssues) != 1 || len(activeIssues[0].Tasks) != 2 {
		t.Fatalf("expected 1 issue with 2 tasks for sprint_id=active, got %+v", activeIssues)
	}

	// 7c: sprint_id=<UUID> -> Task A & Task B
	req = httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/v1/projects/%s/issues?sprint_id=%s", proj.ID, activeSprint.ID), nil)
	req.AddCookie(cookie)
	rec = httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	var uuidIssues []domain.Issue
	_ = json.NewDecoder(rec.Body).Decode(&uuidIssues)
	if len(uuidIssues) != 1 || len(uuidIssues[0].Tasks) != 2 {
		t.Fatalf("expected 1 issue with 2 tasks for sprint_id=%s, got %+v", activeSprint.ID, uuidIssues)
	}
}
