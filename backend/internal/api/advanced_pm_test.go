package api

import (
	"bytes"
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

	// Update sprint to active
	upSpBody, _ := json.Marshal(map[string]any{
		"name":       "Sprint 1 (Active)",
		"goal":       "Ship MVP Now",
		"start_date": spStart,
		"end_date":   spEnd,
		"status":     "active",
	})
	req = httptest.NewRequest(http.MethodPut, fmt.Sprintf("/api/v1/sprints/%s", sp.ID), bytes.NewReader(upSpBody))
	req.AddCookie(cookie)
	rec = httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("failed to update sprint (%d): %s", rec.Code, rec.Body.String())
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

	// Test triggering the public webhook receiver with that token
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

	// ==========================================
	// Feature F: CSV Export & Import
	// ==========================================
	// Export CSV
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

	_ = ctx
}
