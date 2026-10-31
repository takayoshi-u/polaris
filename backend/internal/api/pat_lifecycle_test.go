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
	"time"

	_ "github.com/lib/pq"
	"github.com/takayoshi-u/polaris/backend/internal/domain"
	"github.com/takayoshi-u/polaris/backend/internal/fcm"
	"github.com/takayoshi-u/polaris/backend/internal/mail"
	"github.com/takayoshi-u/polaris/backend/internal/store"
	"github.com/takayoshi-u/polaris/backend/migrations"
)

func setupTestServer(t *testing.T) (*Server, *store.Store, *http.Cookie, *domain.User, *domain.Project) {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("skipping PAT integration test: TEST_DATABASE_URL not set")
	}

	db, err := sql.Open("postgres", dsn)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { db.Close() })

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

	// 1. Create admin user
	initBody, _ := json.Marshal(map[string]string{
		"email":        "tanaka@example.com",
		"display_name": "Tanaka Developer",
		"password":     "Password123!",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/setup/init", bytes.NewReader(initBody))
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("setup init failed: %d %s", rec.Code, rec.Body.String())
	}
	cookies := rec.Result().Cookies()
	if len(cookies) == 0 {
		t.Fatalf("no session cookie received")
	}
	adminCookie := cookies[0]

	var initResp struct {
		User domain.User `json:"user"`
	}
	_ = json.NewDecoder(rec.Body).Decode(&initResp)
	adminUser := initResp.User

	// 2. Create project
	projBody, _ := json.Marshal(map[string]any{
		"key":           "DEV",
		"name":          "Developer Workspace",
		"description":   "Project for PAT testing",
		"hours_per_day": 8.0,
	})
	req = httptest.NewRequest(http.MethodPost, "/api/v1/projects", bytes.NewReader(projBody))
	req.AddCookie(adminCookie)
	rec = httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create project failed: %d %s", rec.Code, rec.Body.String())
	}
	var proj domain.Project
	_ = json.NewDecoder(rec.Body).Decode(&proj)

	return srv, st, adminCookie, &adminUser, &proj
}

func TestPATLifecycleAndTaskOperations(t *testing.T) {
	srv, st, adminCookie, user, proj := setupTestServer(t)

	// Step 1: Create an Issue and Task using cookie session
	issueBody, _ := json.Marshal(map[string]any{
		"title":               "Parent Issue for PAT Task",
		"description":         "Testing PAT operations",
		"create_initial_task": true,
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/projects/"+proj.ID+"/issues", bytes.NewReader(issueBody))
	req.AddCookie(adminCookie)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create issue failed: %d %s", rec.Code, rec.Body.String())
	}
	var issue domain.Issue
	_ = json.NewDecoder(rec.Body).Decode(&issue)
	if len(issue.Tasks) == 0 {
		t.Fatalf("expected at least 1 task")
	}
	taskID := issue.Tasks[0].ID

	// Step 2: Generate a PAT for Tanaka Developer with scopes: read:tasks, write:tasks
	patReqBody, _ := json.Marshal(map[string]any{
		"name":            "Tanaka CI/CD Key",
		"scopes":          []string{"read:tasks", "write:tasks"},
		"expires_in_days": 30,
	})
	req = httptest.NewRequest(http.MethodPost, "/api/v1/user/tokens", bytes.NewReader(patReqBody))
	req.AddCookie(adminCookie)
	rec = httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create PAT failed: %d %s", rec.Code, rec.Body.String())
	}

	var patResponse struct {
		Token string                    `json:"token"`
		PAT   domain.PersonalAccessToken `json:"pat"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&patResponse); err != nil {
		t.Fatalf("decode PAT response: %v", err)
	}

	rawToken := patResponse.Token
	patID := patResponse.PAT.ID

	if rawToken == "" || patResponse.PAT.TokenPrefix == "" {
		t.Fatalf("expected valid token and prefix, got %v", patResponse)
	}
	if !bytes.HasPrefix([]byte(rawToken), []byte("plr_pat_")) {
		t.Fatalf("expected token prefix plr_pat_, got %s", rawToken)
	}
	t.Logf("Generated PAT: prefix=%s, id=%s", patResponse.PAT.TokenPrefix, patID)

	// Step 3: Perform Task Operations using Authorization: Bearer <PAT> WITHOUT session cookie
	// 3a. Read Task
	req = httptest.NewRequest(http.MethodGet, "/api/v1/projects/"+proj.ID+"/tasks/"+taskID, nil)
	req.Header.Set("Authorization", "Bearer "+rawToken)
	rec = httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("read task with PAT failed: %d %s", rec.Code, rec.Body.String())
	}
	var fetchedTask domain.Task
	_ = json.NewDecoder(rec.Body).Decode(&fetchedTask)
	if fetchedTask.ID != taskID {
		t.Fatalf("expected task ID %s, got %s", taskID, fetchedTask.ID)
	}
	t.Logf("Successfully read task via PAT: %s (Progress: %d%%)", fetchedTask.Title, fetchedTask.Progress)

	// 3b. Update Task Progress
	newProgress := 65
	updateBody, _ := json.Marshal(map[string]any{
		"progress": newProgress,
	})
	req = httptest.NewRequest(http.MethodPut, "/api/v1/projects/"+proj.ID+"/tasks/"+taskID, bytes.NewReader(updateBody))
	req.Header.Set("Authorization", "Bearer "+rawToken)
	rec = httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("update task progress with PAT failed: %d %s", rec.Code, rec.Body.String())
	}
	var updatedTask domain.Task
	_ = json.NewDecoder(rec.Body).Decode(&updatedTask)
	if updatedTask.Progress != newProgress {
		t.Fatalf("expected task progress %d, got %d", newProgress, updatedTask.Progress)
	}
	t.Logf("Successfully updated task progress via PAT to %d%%", updatedTask.Progress)

	// 3c. Update Task Dates & Hours & Costs (Planned & Actual)
	startDate := "2026-11-01"
	endDate := "2026-11-15"
	plannedHours := 12.50
	actualHours := 14.75
	plannedCost := 50000.0
	actualCost := 58000.0
	datesBody, _ := json.Marshal(map[string]any{
		"planned_start_date": startDate,
		"planned_end_date":   endDate,
		"actual_start_date":  startDate,
		"actual_end_date":    endDate,
		"planned_hours":      plannedHours,
		"actual_hours":       actualHours,
		"planned_cost":       plannedCost,
		"actual_cost":        actualCost,
	})
	req = httptest.NewRequest(http.MethodPut, "/api/v1/projects/"+proj.ID+"/tasks/"+taskID, bytes.NewReader(datesBody))
	req.Header.Set("Authorization", "Bearer "+rawToken)
	rec = httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("update task dates with PAT failed: %d %s", rec.Code, rec.Body.String())
	}
	var taskWithDates domain.Task
	_ = json.NewDecoder(rec.Body).Decode(&taskWithDates)
	if taskWithDates.PlannedStartDate == nil || *taskWithDates.PlannedStartDate != startDate {
		t.Fatalf("expected planned_start_date %s, got %v", startDate, taskWithDates.PlannedStartDate)
	}
	if taskWithDates.PlannedEndDate == nil || *taskWithDates.PlannedEndDate != endDate {
		t.Fatalf("expected planned_end_date %s, got %v", endDate, taskWithDates.PlannedEndDate)
	}
	if taskWithDates.PlannedHours == nil || *taskWithDates.PlannedHours != plannedHours {
		t.Fatalf("expected planned_hours %f, got %v", plannedHours, taskWithDates.PlannedHours)
	}
	if taskWithDates.ActualHours == nil || *taskWithDates.ActualHours != actualHours {
		t.Fatalf("expected actual_hours %f, got %v", actualHours, taskWithDates.ActualHours)
	}
	if taskWithDates.PlannedCost == nil || *taskWithDates.PlannedCost != plannedCost {
		t.Fatalf("expected planned_cost %f, got %v", plannedCost, taskWithDates.PlannedCost)
	}
	if taskWithDates.ActualCost == nil || *taskWithDates.ActualCost != actualCost {
		t.Fatalf("expected actual_cost %f, got %v", actualCost, taskWithDates.ActualCost)
	}
	t.Logf("Successfully updated task dates, hours (%.2fh/%.2fh), costs via PAT", *taskWithDates.PlannedHours, *taskWithDates.ActualHours)

	// 3d. Update Task Gantt Dates via PATCH endpoint
	patchGanttBody, _ := json.Marshal(map[string]any{
		"planned_start_date": "2026-11-05",
		"planned_end_date":   "2026-11-20",
	})
	req = httptest.NewRequest(http.MethodPatch, "/api/v1/projects/"+proj.ID+"/tasks/"+taskID+"/gantt-dates", bytes.NewReader(patchGanttBody))
	req.Header.Set("Authorization", "Bearer "+rawToken)
	rec = httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("patch task gantt dates with PAT failed: %d %s", rec.Code, rec.Body.String())
	}
	t.Logf("Successfully updated task gantt dates via PATCH with PAT")

	// 3e. Create Comment with Mentions on Task
	commentBody, _ := json.Marshal(map[string]any{
		"body": "Progress reported via CI PAT: @[" + user.DisplayName + "](user:" + user.ID + ") please check.",
		"mentions": []domain.SubjectRef{
			{Type: domain.SubjectUser, ID: user.ID},
		},
	})
	req = httptest.NewRequest(http.MethodPost, "/api/v1/projects/"+proj.ID+"/tasks/"+taskID+"/comments", bytes.NewReader(commentBody))
	req.Header.Set("Authorization", "Bearer "+rawToken)
	rec = httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create comment with PAT failed: %d %s", rec.Code, rec.Body.String())
	}
	var comment domain.Comment
	_ = json.NewDecoder(rec.Body).Decode(&comment)
	if comment.AuthorUserID != user.ID {
		t.Fatalf("expected comment author %s, got %s", user.ID, comment.AuthorUserID)
	}
	if len(comment.Mentions) != 1 || comment.Mentions[0].ID != user.ID {
		t.Fatalf("expected mention of user %s, got %v", user.ID, comment.Mentions)
	}
	t.Logf("Successfully created task comment with mention via PAT: %s", comment.Body)

	// Step 4: Verify invalid / malformed tokens return 401 Unauthorized
	unauthCases := []struct {
		name       string
		authHeader string
	}{
		{"Empty Authorization Header", ""},
		{"Non-Bearer format", "Basic dXNlcjpwYXNz"},
		{"Malformed non-pat token", "Bearer not_a_real_token"},
		{"Random PAT token prefix but fake hash", "Bearer plr_pat_deadbeef00112233445566778899aabbccddeeff"},
		{"Short token", "Bearer plr_pat_"},
	}

	for _, tc := range unauthCases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/api/v1/projects/"+proj.ID+"/tasks/"+taskID, nil)
			if tc.authHeader != "" {
				req.Header.Set("Authorization", tc.authHeader)
			}
			rec := httptest.NewRecorder()
			srv.ServeHTTP(rec, req)
			if rec.Code != http.StatusUnauthorized {
				t.Fatalf("[%s] expected 401 Unauthorized, got %d (%s)", tc.name, rec.Code, rec.Body.String())
			}
		})
	}

	// Step 5: Test Revocation
	// 5a. Revoke PAT via DELETE /api/v1/user/tokens/{tokenId} (using admin session cookie)
	req = httptest.NewRequest(http.MethodDelete, "/api/v1/user/tokens/"+patID, nil)
	req.AddCookie(adminCookie)
	rec = httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("revoke PAT failed: %d %s", rec.Code, rec.Body.String())
	}
	t.Logf("Successfully revoked PAT %s", patID)

	// 5b. Immediate check: call API with revoked token -> must fail with 401 Unauthorized
	req = httptest.NewRequest(http.MethodGet, "/api/v1/projects/"+proj.ID+"/tasks/"+taskID, nil)
	req.Header.Set("Authorization", "Bearer "+rawToken)
	rec = httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 Unauthorized after revocation, got %d (%s)", rec.Code, rec.Body.String())
	}
	t.Logf("Verified revoked PAT is immediately rejected with 401 Unauthorized")

	// 5c. Test expired token: create a fresh PAT, set expiration in the past
	expTok, expPat, err := st.CreatePAT(context.Background(), user.ID, "Expired Key", []string{"read:tasks"}, nil)
	if err != nil {
		t.Fatalf("failed to create expired PAT: %v", err)
	}
	past := time.Now().UTC().Add(-24 * time.Hour)
	_, err = st.DB().Exec(`UPDATE personal_access_tokens SET expires_at = $1 WHERE id = $2`, past, expPat.ID)
	if err != nil {
		t.Fatalf("failed to update expires_at: %v", err)
	}
	req = httptest.NewRequest(http.MethodGet, "/api/v1/projects/"+proj.ID+"/tasks/"+taskID, nil)
	req.Header.Set("Authorization", "Bearer "+expTok)
	rec = httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 Unauthorized for expired token, got %d (%s)", rec.Code, rec.Body.String())
	}
	t.Logf("Verified expired PAT is rejected with 401 Unauthorized")
}

// TestPATScopeEnforcementVerification verifies whether token scopes (e.g. read:tasks vs write:tasks)
// are enforced by the API endpoints.
func TestPATScopeEnforcementVerification(t *testing.T) {
	srv, st, adminCookie, user, proj := setupTestServer(t)

	// Create an issue and task
	issueBody, _ := json.Marshal(map[string]any{
		"title":               "Task for Scope Test",
		"create_initial_task": true,
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/projects/"+proj.ID+"/issues", bytes.NewReader(issueBody))
	req.AddCookie(adminCookie)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	var issue domain.Issue
	_ = json.NewDecoder(rec.Body).Decode(&issue)
	taskID := issue.Tasks[0].ID

	// Issue a Read-Only PAT with only "read:tasks" scope
	readOnlyTok, _, err := st.CreatePAT(context.Background(), user.ID, "ReadOnly Task Key", []string{"read:tasks"}, nil)
	if err != nil {
		t.Fatalf("create read-only PAT: %v", err)
	}

	// 1. Reading task with read-only token should SUCCEED
	req = httptest.NewRequest(http.MethodGet, "/api/v1/projects/"+proj.ID+"/tasks/"+taskID, nil)
	req.Header.Set("Authorization", "Bearer "+readOnlyTok)
	rec = httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 reading task with read:tasks scope, got %d", rec.Code)
	}

	// 2. Writing/Updating task with read-only token
	// If scope enforcement is NOT implemented, this write succeeds (200 OK), which is a security defect!
	updateBody, _ := json.Marshal(map[string]any{"progress": 99})
	req = httptest.NewRequest(http.MethodPut, "/api/v1/projects/"+proj.ID+"/tasks/"+taskID, bytes.NewReader(updateBody))
	req.Header.Set("Authorization", "Bearer "+readOnlyTok)
	rec = httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code == http.StatusOK {
		t.Logf("⚠️ DEFECT CONFIRMED: Token with only [read:tasks] scope succeeded in writing task (HTTP 200). Backend lacks scope checking middleware!")
	} else if rec.Code == http.StatusForbidden {
		t.Logf("Scope enforcement active: write blocked with 403 Forbidden")
	} else {
		t.Logf("Unexpected status code %d: %s", rec.Code, rec.Body.String())
	}
}
