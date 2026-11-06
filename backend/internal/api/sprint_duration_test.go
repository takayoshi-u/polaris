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
)

func sprintDurDo(t *testing.T, rig *testRig, method, path string, body any) *httptest.ResponseRecorder {
	t.Helper()
	b, _ := json.Marshal(body)
	req := httptest.NewRequest(method, path, bytes.NewReader(b))
	req.AddCookie(rig.adminCookie)
	rec := httptest.NewRecorder()
	rig.srv.ServeHTTP(rec, req)
	return rec
}

func TestSprintDuration_CreateBoundariesAndDefault(t *testing.T) {
	rig := setupTestRig(t)
	defer rig.db.Close()

	for _, bad := range []int{0, 91} {
		rec := sprintDurDo(t, rig, http.MethodPost, "/api/v1/projects", map[string]any{"name": "bad", "sprint_duration_days": bad})
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("sprint_duration_days=%d: expected 400, got %d", bad, rec.Code)
		}
	}
	for _, ok := range []int{1, 90} {
		rec := sprintDurDo(t, rig, http.MethodPost, "/api/v1/projects", map[string]any{"name": fmt.Sprintf("ok%d", ok), "sprint_duration_days": ok})
		if rec.Code != http.StatusCreated {
			t.Fatalf("sprint_duration_days=%d: expected 201, got %d: %s", ok, rec.Code, rec.Body.String())
		}
		var p domain.Project
		_ = json.NewDecoder(rec.Body).Decode(&p)
		if p.SprintDurationDays != ok {
			t.Fatalf("expected %d, got %d", ok, p.SprintDurationDays)
		}
	}
	for name, body := range map[string]map[string]any{
		"null":    {"name": "p-null", "sprint_duration_days": nil},
		"omitted": {"name": "p-omitted"},
	} {
		rec := sprintDurDo(t, rig, http.MethodPost, "/api/v1/projects", body)
		if rec.Code != http.StatusCreated {
			t.Fatalf("%s: expected 201, got %d: %s", name, rec.Code, rec.Body.String())
		}
		var p domain.Project
		_ = json.NewDecoder(rec.Body).Decode(&p)
		if p.SprintDurationDays != 14 {
			t.Fatalf("%s: expected default 14, got %d", name, p.SprintDurationDays)
		}
	}
}

func TestSprintDuration_UpdateOmittedKeepsStoredValue(t *testing.T) {
	rig := setupTestRig(t)
	defer rig.db.Close()

	rec := sprintDurDo(t, rig, http.MethodPost, "/api/v1/projects", map[string]any{"name": "keep", "sprint_duration_days": 7})
	if rec.Code != http.StatusCreated {
		t.Fatalf("create: %d %s", rec.Code, rec.Body.String())
	}
	var p domain.Project
	_ = json.NewDecoder(rec.Body).Decode(&p)

	rec = sprintDurDo(t, rig, http.MethodPut, "/api/v1/projects/"+p.ID, map[string]any{"name": "keep-renamed", "description": p.Description, "hours_per_day": p.HoursPerDay})
	if rec.Code != http.StatusOK {
		t.Fatalf("update: %d %s", rec.Code, rec.Body.String())
	}
	var up domain.Project
	_ = json.NewDecoder(rec.Body).Decode(&up)
	if up.SprintDurationDays != 7 {
		t.Fatalf("expected stored 7 to be kept, got %d", up.SprintDurationDays)
	}
}

// All sprints have NULL end_date -> rollover auto-created sprint falls back to JST today.
func TestSprintDuration_RolloverFallsBackToJSTTodayWhenNoEndDates(t *testing.T) {
	rig := setupTestRig(t)
	defer rig.db.Close()

	rec := sprintDurDo(t, rig, http.MethodPost, "/api/v1/projects", map[string]any{"name": "nodates", "sprint_duration_days": 7})
	var p domain.Project
	_ = json.NewDecoder(rec.Body).Decode(&p)

	rec = sprintDurDo(t, rig, http.MethodGet, "/api/v1/projects/"+p.ID+"/statuses", nil)
	var statuses []domain.Status
	_ = json.NewDecoder(rec.Body).Decode(&statuses)
	if len(statuses) < 2 {
		t.Fatalf("expected statuses, got %d", len(statuses))
	}

	rec = sprintDurDo(t, rig, http.MethodPost, "/api/v1/projects/"+p.ID+"/sprints", map[string]any{"name": "Sprint 1", "status": "active"})
	if rec.Code != http.StatusCreated {
		t.Fatalf("create sprint: %d %s", rec.Code, rec.Body.String())
	}
	var sp domain.Sprint
	_ = json.NewDecoder(rec.Body).Decode(&sp)
	if sp.EndDate != nil {
		t.Fatalf("API CreateSprint must not default dates, got end_date %v", sp.EndDate)
	}

	rec = sprintDurDo(t, rig, http.MethodPost, "/api/v1/projects/"+p.ID+"/issues", map[string]any{"title": "I"})
	var iss domain.Issue
	_ = json.NewDecoder(rec.Body).Decode(&iss)
	rec = sprintDurDo(t, rig, http.MethodPost, "/api/v1/projects/"+p.ID+"/tasks", map[string]any{
		"issue_id": iss.ID, "title": "T", "status_id": statuses[0].ID, "sprint_id": sp.ID, "progress": 10,
	})
	var task domain.Task
	_ = json.NewDecoder(rec.Body).Decode(&task)

	rec = sprintDurDo(t, rig, http.MethodPut, "/api/v1/sprints/"+sp.ID, map[string]any{"status": "closed", "rollover_action": "next_sprint"})
	if rec.Code != http.StatusOK {
		t.Fatalf("close sprint: %d %s", rec.Code, rec.Body.String())
	}
	rec = sprintDurDo(t, rig, http.MethodGet, fmt.Sprintf("/api/v1/projects/%s/tasks/%s", p.ID, task.ID), nil)
	var moved domain.Task
	_ = json.NewDecoder(rec.Body).Decode(&moved)
	if moved.SprintID == nil || *moved.SprintID == sp.ID {
		t.Fatalf("task not moved: %v", moved.SprintID)
	}
	rec = sprintDurDo(t, rig, http.MethodGet, "/api/v1/sprints/"+*moved.SprintID, nil)
	var next domain.Sprint
	_ = json.NewDecoder(rec.Body).Decode(&next)

	jst := time.FixedZone("JST", 9*3600)
	// Tolerate a JST midnight crossing during the test.
	valid := map[string]bool{}
	for _, d := range []time.Time{time.Now().In(jst), time.Now().In(jst).Add(-time.Minute)} {
		valid[d.Format("2006-01-02")] = true
	}
	if next.StartDate == nil || !valid[*next.StartDate] {
		t.Fatalf("expected start_date = JST today, got %v", next.StartDate)
	}
	wantEnd := map[string]bool{}
	for s := range valid {
		d, _ := time.Parse("2006-01-02", s)
		wantEnd[d.AddDate(0, 0, 6).Format("2006-01-02")] = true
	}
	if next.EndDate == nil || !wantEnd[*next.EndDate] {
		t.Fatalf("expected end_date = start+6, got %v", next.EndDate)
	}
}
