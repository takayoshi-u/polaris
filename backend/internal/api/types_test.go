package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/takayoshi-u/polaris/backend/internal/domain"
)

func TestTypes_Unauthorized(t *testing.T) {
	srv := NewServer(nil, "http://localhost:3000", GoogleOAuthConfig{})

	endpoints := []struct {
		method string
		path   string
	}{
		{http.MethodGet, "/api/v1/system/types"},
		{http.MethodPost, "/api/v1/system/types"},
		{http.MethodPut, "/api/v1/system/types/type-1"},
		{http.MethodDelete, "/api/v1/system/types/type-1"},
		{http.MethodGet, "/api/v1/projects/proj-1/types"},
		{http.MethodPost, "/api/v1/projects/proj-1/types"},
		{http.MethodPut, "/api/v1/projects/proj-1/types/type-1"},
		{http.MethodDelete, "/api/v1/projects/proj-1/types/type-1"},
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

func TestTypes_Integration(t *testing.T) {
	rig := setupTestRig(t)
	defer rig.db.Close()

	srv := rig.srv
	cookie := rig.adminCookie

	// 1. Get initial system-wide types (seeded via migration: タスク, バグ, 機能要望, 改善)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/system/types", nil)
	req.AddCookie(cookie)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 for GET /api/v1/system/types, got %d: %s", rec.Code, rec.Body.String())
	}
	var systemTypes []domain.ItemType
	if err := json.NewDecoder(rec.Body).Decode(&systemTypes); err != nil {
		t.Fatalf("decode system types failed: %v", err)
	}
	if len(systemTypes) < 4 {
		t.Fatalf("expected at least 4 default system types, got %d", len(systemTypes))
	}

	defaultTaskTypeID := systemTypes[0].ID

	// 2. Create custom system-wide type
	sysTypeBody, _ := json.Marshal(map[string]any{
		"name":          "リサーチ",
		"color":         "#8b5cf6",
		"display_order": 5,
	})
	req = httptest.NewRequest(http.MethodPost, "/api/v1/system/types", bytes.NewReader(sysTypeBody))
	req.AddCookie(cookie)
	rec = httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("failed to create system type (%d): %s", rec.Code, rec.Body.String())
	}
	var createdSysType domain.ItemType
	_ = json.NewDecoder(rec.Body).Decode(&createdSysType)
	if createdSysType.Name != "リサーチ" || createdSysType.ProjectID != nil {
		t.Fatalf("unexpected created system type: %+v", createdSysType)
	}

	// 3. Update custom system-wide type
	updateSysTypeBody, _ := json.Marshal(map[string]any{
		"name":  "調査・分析",
		"color": "#7c3aed",
	})
	req = httptest.NewRequest(http.MethodPut, "/api/v1/system/types/"+createdSysType.ID, bytes.NewReader(updateSysTypeBody))
	req.AddCookie(cookie)
	rec = httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("failed to update system type (%d): %s", rec.Code, rec.Body.String())
	}

	// 4. Create a Project
	projBody, _ := json.Marshal(map[string]any{
		"name":        "Type Test Project",
		"description": "Project for testing types",
	})
	req = httptest.NewRequest(http.MethodPost, "/api/v1/projects", bytes.NewReader(projBody))
	req.AddCookie(cookie)
	rec = httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("failed to create project (%d): %s", rec.Code, rec.Body.String())
	}
	var proj domain.Project
	_ = json.NewDecoder(rec.Body).Decode(&proj)

	// 5. List project types (should include system types)
	req = httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/v1/projects/%s/types", proj.ID), nil)
	req.AddCookie(cookie)
	rec = httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("failed to list project types (%d): %s", rec.Code, rec.Body.String())
	}
	var projTypes []domain.ItemType
	_ = json.NewDecoder(rec.Body).Decode(&projTypes)
	if len(projTypes) < 5 {
		t.Fatalf("expected at least 5 types for project, got %d", len(projTypes))
	}

	// 6. Create project-scoped type
	projTypeBody, _ := json.Marshal(map[string]any{
		"name":          "リリース作業",
		"color":         "#ec4899",
		"display_order": 10,
	})
	req = httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/v1/projects/%s/types", proj.ID), bytes.NewReader(projTypeBody))
	req.AddCookie(cookie)
	rec = httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("failed to create project type (%d): %s", rec.Code, rec.Body.String())
	}
	var createdProjType domain.ItemType
	_ = json.NewDecoder(rec.Body).Decode(&createdProjType)
	if createdProjType.ProjectID == nil || *createdProjType.ProjectID != proj.ID {
		t.Fatalf("expected project type to belong to project %s, got %v", proj.ID, createdProjType.ProjectID)
	}

	// 7. Update project-scoped type
	updateProjTypeBody, _ := json.Marshal(map[string]any{
		"name":  "デプロイ作業",
		"color": "#db2777",
	})
	req = httptest.NewRequest(http.MethodPut, fmt.Sprintf("/api/v1/projects/%s/types/%s", proj.ID, createdProjType.ID), bytes.NewReader(updateProjTypeBody))
	req.AddCookie(cookie)
	rec = httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("failed to update project type (%d): %s", rec.Code, rec.Body.String())
	}

	// 8. IDOR Prevention checks
	// Attempt to update system type via project endpoint -> 403 Forbidden
	req = httptest.NewRequest(http.MethodPut, fmt.Sprintf("/api/v1/projects/%s/types/%s", proj.ID, defaultTaskTypeID), bytes.NewReader(updateProjTypeBody))
	req.AddCookie(cookie)
	rec = httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 when updating system type via project endpoint, got %d", rec.Code)
	}

	// Attempt to update project type via system endpoint -> 400 Bad Request
	req = httptest.NewRequest(http.MethodPut, fmt.Sprintf("/api/v1/system/types/%s", createdProjType.ID), bytes.NewReader(updateSysTypeBody))
	req.AddCookie(cookie)
	rec = httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 when updating project type via system endpoint, got %d", rec.Code)
	}

	// 9. Create issue with type_id
	issueBody, _ := json.Marshal(map[string]any{
		"title":       "Issue With Type",
		"description": "Issue description",
		"type_id":     createdProjType.ID,
	})
	req = httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/v1/projects/%s/issues", proj.ID), bytes.NewReader(issueBody))
	req.AddCookie(cookie)
	rec = httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("failed to create issue with type (%d): %s", rec.Code, rec.Body.String())
	}
	var createdIssue domain.Issue
	_ = json.NewDecoder(rec.Body).Decode(&createdIssue)
	if createdIssue.TypeID == nil || *createdIssue.TypeID != createdProjType.ID {
		t.Fatalf("expected issue type_id %s, got %v", createdProjType.ID, createdIssue.TypeID)
	}
	if createdIssue.Type == nil || createdIssue.Type.Name != "デプロイ作業" {
		t.Fatalf("expected populated Type object on issue, got %v", createdIssue.Type)
	}

	// 10. Update issue: clear type_id
	clearIssueTypeBody, _ := json.Marshal(map[string]any{
		"type_id": nil,
	})
	req = httptest.NewRequest(http.MethodPut, fmt.Sprintf("/api/v1/projects/%s/issues/%s", proj.ID, createdIssue.ID), bytes.NewReader(clearIssueTypeBody))
	req.AddCookie(cookie)
	rec = httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("failed to update issue to clear type (%d): %s", rec.Code, rec.Body.String())
	}
	var updatedIssue domain.Issue
	_ = json.NewDecoder(rec.Body).Decode(&updatedIssue)
	if updatedIssue.TypeID != nil {
		t.Fatalf("expected cleared issue type_id, got %v", updatedIssue.TypeID)
	}

	// 11. Create task with type_id
	taskBody, _ := json.Marshal(map[string]any{
		"issue_id":    createdIssue.ID,
		"title":       "Task With Type",
		"description": "Task description",
		"type_id":     defaultTaskTypeID,
	})
	req = httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/v1/projects/%s/tasks", proj.ID), bytes.NewReader(taskBody))
	req.AddCookie(cookie)
	rec = httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("failed to create task with type (%d): %s", rec.Code, rec.Body.String())
	}
	var createdTask domain.Task
	_ = json.NewDecoder(rec.Body).Decode(&createdTask)
	if createdTask.TypeID == nil || *createdTask.TypeID != defaultTaskTypeID {
		t.Fatalf("expected task type_id %s, got %v", defaultTaskTypeID, createdTask.TypeID)
	}
	if createdTask.Type == nil {
		t.Fatalf("expected populated Type object on task, got nil")
	}

	// 12. Update task: change type_id to project type
	updateTaskTypeBody, _ := json.Marshal(map[string]any{
		"type_id": createdProjType.ID,
	})
	req = httptest.NewRequest(http.MethodPut, fmt.Sprintf("/api/v1/projects/%s/tasks/%s", proj.ID, createdTask.ID), bytes.NewReader(updateTaskTypeBody))
	req.AddCookie(cookie)
	rec = httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("failed to update task type (%d): %s", rec.Code, rec.Body.String())
	}
	var updatedTask domain.Task
	_ = json.NewDecoder(rec.Body).Decode(&updatedTask)
	if updatedTask.TypeID == nil || *updatedTask.TypeID != createdProjType.ID {
		t.Fatalf("expected updated task type_id %s, got %v", createdProjType.ID, updatedTask.TypeID)
	}

	// 13. Delete project type
	req = httptest.NewRequest(http.MethodDelete, fmt.Sprintf("/api/v1/projects/%s/types/%s", proj.ID, createdProjType.ID), nil)
	req.AddCookie(cookie)
	rec = httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("failed to delete project type (%d): %s", rec.Code, rec.Body.String())
	}

	// 14. Delete custom system type
	req = httptest.NewRequest(http.MethodDelete, fmt.Sprintf("/api/v1/system/types/%s", createdSysType.ID), nil)
	req.AddCookie(cookie)
	rec = httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("failed to delete system type (%d): %s", rec.Code, rec.Body.String())
	}
}

func TestTypes_RequestStructsJSON(t *testing.T) {
	// Test CreateIssueRequest JSON parsing
	issueJSON := `{"title":"Issue Title","type_id":"018f0000-0000-7000-8000-000000000001"}`
	var cir CreateIssueRequest
	if err := json.Unmarshal([]byte(issueJSON), &cir); err != nil {
		t.Fatalf("unmarshal CreateIssueRequest: %v", err)
	}
	if cir.TypeID == nil || *cir.TypeID != "018f0000-0000-7000-8000-000000000001" {
		t.Fatalf("expected type_id on CreateIssueRequest, got %v", cir.TypeID)
	}

	// Test UpdateIssueRequest JSON parsing
	updateIssueJSON := `{"type_id":"018f0000-0000-7000-8000-000000000002"}`
	var uir UpdateIssueRequest
	if err := json.Unmarshal([]byte(updateIssueJSON), &uir); err != nil {
		t.Fatalf("unmarshal UpdateIssueRequest: %v", err)
	}
	if uir.TypeID == nil || *uir.TypeID != "018f0000-0000-7000-8000-000000000002" {
		t.Fatalf("expected type_id on UpdateIssueRequest, got %v", uir.TypeID)
	}

	// Test CreateTaskRequest JSON parsing
	taskJSON := `{"issue_id":"018f0000-0000-7000-8000-000000000000","title":"Task Title","type_id":"018f0000-0000-7000-8000-000000000001"}`
	var ctr CreateTaskRequest
	if err := json.Unmarshal([]byte(taskJSON), &ctr); err != nil {
		t.Fatalf("unmarshal CreateTaskRequest: %v", err)
	}
	if ctr.TypeID == nil || *ctr.TypeID != "018f0000-0000-7000-8000-000000000001" {
		t.Fatalf("expected type_id on CreateTaskRequest, got %v", ctr.TypeID)
	}

	// Test UpdateTaskRequest JSON parsing
	updateTaskJSON := `{"type_id":"018f0000-0000-7000-8000-000000000003"}`
	var utr UpdateTaskRequest
	if err := json.Unmarshal([]byte(updateTaskJSON), &utr); err != nil {
		t.Fatalf("unmarshal UpdateTaskRequest: %v", err)
	}
	if utr.TypeID == nil || *utr.TypeID != "018f0000-0000-7000-8000-000000000003" {
		t.Fatalf("expected type_id on UpdateTaskRequest, got %v", utr.TypeID)
	}
}
