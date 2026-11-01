package api

import (
	"bytes"
	"context"
	"database/sql"
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/takayoshi-u/polaris/backend/internal/domain"
	"github.com/takayoshi-u/polaris/backend/internal/fcm"
	"github.com/takayoshi-u/polaris/backend/internal/mail"
	"github.com/takayoshi-u/polaris/backend/internal/pkg/crypto"
	"github.com/takayoshi-u/polaris/backend/internal/store"
)

// =========================================================================
// In-Memory Mock SQL Driver for Project Admin Guard Testing
// =========================================================================

type guardMockState struct {
	mu             sync.Mutex
	users          map[string]*domain.User
	projects       map[string]*domain.Project
	projectMembers map[string]*domain.ProjectMember // id -> member
	sysPerms       map[string][]string             // userID -> permissions
	sessions       map[string]string               // tokenHash -> userID
}

var globalGuardState = &guardMockState{
	users:          make(map[string]*domain.User),
	projects:       make(map[string]*domain.Project),
	projectMembers: make(map[string]*domain.ProjectMember),
	sysPerms:       make(map[string][]string),
	sessions:       make(map[string]string),
}

func init() {
	sql.Register("project_guard_mock", &guardMockDriver{})
}

type guardMockDriver struct{}

func (d *guardMockDriver) Open(name string) (driver.Conn, error) {
	return &guardMockConn{state: globalGuardState}, nil
}

type guardMockConn struct {
	state *guardMockState
}

func (c *guardMockConn) Prepare(query string) (driver.Stmt, error) {
	return &guardMockStmt{conn: c, query: query}, nil
}

func (c *guardMockConn) Close() error { return nil }
func (c *guardMockConn) Begin() (driver.Tx, error) {
	return &guardMockTx{}, nil
}

type guardMockTx struct{}

func (t *guardMockTx) Commit() error   { return nil }
func (t *guardMockTx) Rollback() error { return nil }

type guardMockStmt struct {
	conn  *guardMockConn
	query string
}

func (s *guardMockStmt) Close() error  { return nil }
func (s *guardMockStmt) NumInput() int { return -1 }

func (s *guardMockStmt) Exec(args []driver.Value) (driver.Result, error) {
	q := strings.TrimSpace(s.query)
	st := s.conn.state
	st.mu.Lock()
	defer st.mu.Unlock()

	switch {
	case strings.Contains(q, "INSERT INTO project_members"):
		// id, project_id, subject_type, subject_id, role, created_at
		memID := fmt.Sprintf("%v", args[0])
		projID := fmt.Sprintf("%v", args[1])
		sType := fmt.Sprintf("%v", args[2])
		sID := fmt.Sprintf("%v", args[3])
		role := domain.ProjectRole(fmt.Sprintf("%v", args[4]))
		cAt, _ := args[5].(time.Time)

		// Check for existing by (project_id, subject_type, subject_id)
		var foundID string
		for id, m := range st.projectMembers {
			if m.ProjectID == projID && m.SubjectType == sType && m.SubjectID == sID {
				foundID = id
				break
			}
		}
		if foundID != "" {
			st.projectMembers[foundID].Role = role
		} else {
			st.projectMembers[memID] = &domain.ProjectMember{
				ID:          memID,
				ProjectID:   projID,
				SubjectType: sType,
				SubjectID:   sID,
				Role:        role,
				CreatedAt:   cAt,
			}
		}
		return driver.RowsAffected(1), nil

	case strings.Contains(q, "DELETE FROM project_members WHERE id = $1 AND project_id = $2"):
		memID := fmt.Sprintf("%v", args[0])
		projID := fmt.Sprintf("%v", args[1])
		if m, ok := st.projectMembers[memID]; ok && m.ProjectID == projID {
			delete(st.projectMembers, memID)
			return driver.RowsAffected(1), nil
		}
		return driver.RowsAffected(0), nil

	case strings.Contains(q, "UPDATE projects"):
		// UPDATE projects SET name = $1, description = $2, hours_per_day = $3 ... WHERE id = $7
		if len(args) >= 7 {
			projID := fmt.Sprintf("%v", args[6])
			if p, ok := st.projects[projID]; ok {
				p.Name = fmt.Sprintf("%v", args[0])
				p.Description = fmt.Sprintf("%v", args[1])
				return driver.RowsAffected(1), nil
			}
		}
		return driver.RowsAffected(1), nil
	}

	return driver.RowsAffected(1), nil
}

func (s *guardMockStmt) Query(args []driver.Value) (driver.Rows, error) {
	q := strings.TrimSpace(s.query)
	st := s.conn.state
	st.mu.Lock()
	defer st.mu.Unlock()

	switch {
	case strings.Contains(q, "user_sessions") && strings.Contains(q, "token_hash"):
		// GetUserBySessionToken
		tokenHash := fmt.Sprintf("%v", args[0])
		userID, exists := st.sessions[tokenHash]
		if !exists {
			return &mockRows{cols: []string{"user_id", "expires_at"}, rows: nil}, nil
		}
		u := st.users[userID]
		if u == nil {
			return &mockRows{cols: []string{"user_id", "expires_at"}, rows: nil}, nil
		}
		cols := []string{"user_id", "expires_at"}
		values := [][]driver.Value{{
			userID, time.Now().Add(24 * time.Hour),
		}}
		return &mockRows{cols: cols, rows: values}, nil

	case strings.Contains(q, "FROM users") && strings.Contains(q, "WHERE id = $1"):
		// GetUserByID
		userID := fmt.Sprintf("%v", args[0])
		u, exists := st.users[userID]
		cols := []string{"id", "email", "password_hash", "google_sub", "display_name", "avatar_url", "status", "mfa_enabled", "mfa_enforced", "created_at", "updated_at"}
		if !exists || u == nil {
			return &mockRows{cols: cols, rows: nil}, nil
		}
		values := [][]driver.Value{{
			u.ID, u.Email, "hashed-pass", nil, u.DisplayName, u.AvatarURL, u.Status, u.MFAEnabled, u.MFAEnforced, u.CreatedAt, u.UpdatedAt,
		}}
		return &mockRows{cols: cols, rows: values}, nil

	case strings.Contains(q, "SELECT DISTINCT sp.permission"):
		// HasUserSystemPermission
		userID := fmt.Sprintf("%v", args[0])
		perms := st.sysPerms[userID]
		cols := []string{"permission"}
		var values [][]driver.Value
		for _, p := range perms {
			values = append(values, []driver.Value{p})
		}
		return &mockRows{cols: cols, rows: values}, nil

	case strings.Contains(q, "FROM project_members") && strings.Contains(q, "subject_type = 'user'") && strings.Contains(q, "subject_id = $2"):
		// GetEffectiveProjectRole direct
		projID := fmt.Sprintf("%v", args[0])
		userID := fmt.Sprintf("%v", args[1])
		cols := []string{"role"}
		for _, m := range st.projectMembers {
			if m.ProjectID == projID && m.SubjectType == domain.SubjectUser && m.SubjectID == userID {
				return &mockRows{cols: cols, rows: [][]driver.Value{{string(m.Role)}}}, nil
			}
		}
		return &mockRows{cols: cols, rows: nil}, nil

	case strings.Contains(q, "group_members"):
		// No groups in this test
		return &mockRows{cols: []string{"group_id"}, rows: nil}, nil

	case strings.Contains(q, "FROM projects") && strings.Contains(q, "WHERE id = $1"):
		// GetProject query
		projID := fmt.Sprintf("%v", args[0])
		p, exists := st.projects[projID]
		cols := []string{"id", "name", "description", "hours_per_day", "default_issue_template_id", "default_task_template_id", "created_by", "created_at", "updated_at"}
		if !exists {
			return &mockRows{cols: cols, rows: nil}, nil
		}
		var createdByVal any = nil
		if p.CreatedBy != nil {
			createdByVal = *p.CreatedBy
		}
		values := [][]driver.Value{{
			p.ID, p.Name, p.Description, p.HoursPerDay, nil, nil, createdByVal, p.CreatedAt, p.UpdatedAt,
		}}
		return &mockRows{cols: cols, rows: values}, nil

	case strings.Contains(q, "SELECT COALESCE(SUM(planned_cost), 0), COALESCE(SUM(actual_cost), 0)"):
		cols := []string{"planned_cost", "actual_cost"}
		values := [][]driver.Value{{0.0, 0.0}}
		return &mockRows{cols: cols, rows: values}, nil

	case strings.Contains(q, "SELECT role FROM project_members") && strings.Contains(q, "subject_type = $2"):
		// Upsert check current role
		projID := fmt.Sprintf("%v", args[0])
		sType := fmt.Sprintf("%v", args[1])
		sID := fmt.Sprintf("%v", args[2])
		cols := []string{"role"}
		for _, m := range st.projectMembers {
			if m.ProjectID == projID && m.SubjectType == sType && m.SubjectID == sID {
				return &mockRows{cols: cols, rows: [][]driver.Value{{string(m.Role)}}}, nil
			}
		}
		return &mockRows{cols: cols, rows: nil}, nil

	case strings.Contains(q, "SELECT COUNT(*)") && strings.Contains(q, "NOT (subject_type = $2"):
		// Upsert check other admins
		projID := fmt.Sprintf("%v", args[0])
		sType := fmt.Sprintf("%v", args[1])
		sID := fmt.Sprintf("%v", args[2])
		count := 0
		for _, m := range st.projectMembers {
			if m.ProjectID == projID && m.Role == domain.RoleAdmin {
				if !(m.SubjectType == sType && m.SubjectID == sID) {
					count++
				}
			}
		}
		return &mockRows{cols: []string{"count"}, rows: [][]driver.Value{{int64(count)}}}, nil

	case strings.Contains(q, "SELECT role FROM project_members WHERE id = $1"):
		// RemoveProjectMember check current role
		memID := fmt.Sprintf("%v", args[0])
		projID := fmt.Sprintf("%v", args[1])
		cols := []string{"role"}
		if m, ok := st.projectMembers[memID]; ok && m.ProjectID == projID {
			return &mockRows{cols: cols, rows: [][]driver.Value{{string(m.Role)}}}, nil
		}
		return &mockRows{cols: cols, rows: nil}, nil

	case strings.Contains(q, "SELECT COUNT(*)") && strings.Contains(q, "id != $2"):
		// RemoveProjectMember check other admins
		projID := fmt.Sprintf("%v", args[0])
		memID := fmt.Sprintf("%v", args[1])
		count := 0
		for _, m := range st.projectMembers {
			if m.ProjectID == projID && m.Role == domain.RoleAdmin && m.ID != memID {
				count++
			}
		}
		return &mockRows{cols: []string{"count"}, rows: [][]driver.Value{{int64(count)}}}, nil

	case strings.Contains(q, "FROM project_members") && strings.Contains(q, "WHERE project_id = $1 AND subject_type = $2 AND subject_id = $3"):
		// After upsert select
		projID := fmt.Sprintf("%v", args[0])
		sType := fmt.Sprintf("%v", args[1])
		sID := fmt.Sprintf("%v", args[2])
		cols := []string{"id", "project_id", "subject_type", "subject_id", "role", "created_at"}
		for _, m := range st.projectMembers {
			if m.ProjectID == projID && m.SubjectType == sType && m.SubjectID == sID {
				return &mockRows{cols: cols, rows: [][]driver.Value{{
					m.ID, m.ProjectID, m.SubjectType, m.SubjectID, string(m.Role), m.CreatedAt,
				}}}, nil
			}
		}
		return &mockRows{cols: cols, rows: nil}, nil

	case strings.Contains(q, "FROM project_members WHERE project_id = $1"):
		// ListProjectMembers
		projID := fmt.Sprintf("%v", args[0])
		cols := []string{"id", "project_id", "subject_type", "subject_id", "role", "created_at"}
		var values [][]driver.Value
		for _, m := range st.projectMembers {
			if m.ProjectID == projID {
				values = append(values, []driver.Value{
					m.ID, m.ProjectID, m.SubjectType, m.SubjectID, string(m.Role), m.CreatedAt,
				})
			}
		}
		return &mockRows{cols: cols, rows: values}, nil
	}

	return &mockRows{cols: []string{"dummy"}, rows: nil}, nil
}

// =========================================================================
// Tests
// =========================================================================

func TestProjectAdminGuard_Unauthorized(t *testing.T) {
	srv := NewServer(nil, "http://localhost:3000", GoogleOAuthConfig{})

	endpoints := []struct {
		method string
		path   string
	}{
		{http.MethodGet, "/api/v1/projects/proj-1/members"},
		{http.MethodPost, "/api/v1/projects/proj-1/members"},
		{http.MethodDelete, "/api/v1/projects/proj-1/members/mem-1"},
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

// TestProjectAdminGuard_UnitTests tests demotion/removal protection and system admin rescue using the mock driver.
func TestProjectAdminGuard_UnitTests(t *testing.T) {
	mockDB, err := sql.Open("project_guard_mock", "")
	if err != nil {
		t.Fatalf("open mock db: %v", err)
	}
	defer mockDB.Close()

	st := store.New(mockDB, "0123456789abcdef0123456789abcdef", "http://localhost:3000", mail.NewSMTPSender(), fcm.NewClient(context.Background(), ""))
	srv := NewServer(st, "http://localhost:3000", GoogleOAuthConfig{})

	// 1. Setup mock users and sessions
	now := time.Now().UTC()
	globalGuardState.mu.Lock()
	globalGuardState.users = map[string]*domain.User{
		"user-admin":    {ID: "user-admin", Email: "admin@example.com", DisplayName: "Admin User", Status: domain.UserStatusActive, MFAEnabled: true, CreatedAt: now, UpdatedAt: now},
		"user-member":   {ID: "user-member", Email: "member@example.com", DisplayName: "Member User", Status: domain.UserStatusActive, MFAEnabled: true, CreatedAt: now, UpdatedAt: now},
		"user-sysadmin": {ID: "user-sysadmin", Email: "sysadmin@example.com", DisplayName: "System Admin", Status: domain.UserStatusActive, MFAEnabled: true, CreatedAt: now, UpdatedAt: now},
		"user-outsider": {ID: "user-outsider", Email: "outsider@example.com", DisplayName: "Outsider User", Status: domain.UserStatusActive, MFAEnabled: true, CreatedAt: now, UpdatedAt: now},
	}
	globalGuardState.sysPerms = map[string][]string{
		"user-sysadmin": {domain.PermManageSystem},
	}
	// Setup sessions
	adminToken := "session-admin-token"
	memberToken := "session-member-token"
	sysadminToken := "session-sysadmin-token"
	outsiderToken := "session-outsider-token"

	globalGuardState.sessions = map[string]string{
		crypto.HashToken(adminToken):    "user-admin",
		crypto.HashToken(memberToken):   "user-member",
		crypto.HashToken(sysadminToken): "user-sysadmin",
		crypto.HashToken(outsiderToken): "user-outsider",
	}

	// Setup project
	projID := "proj-guard-1"
	globalGuardState.projects = map[string]*domain.Project{
		projID: {
			ID:          projID,
			Name:        "Guard Test Project",
			Description: "Testing last admin demotion and removal block",
			HoursPerDay: 8.0,
			CreatedAt:   now,
			UpdatedAt:   now,
		},
	}

	// Initially, user-admin is the sole admin member
	adminMemberID := "mem-admin-1"
	globalGuardState.projectMembers = map[string]*domain.ProjectMember{
		adminMemberID: {
			ID:          adminMemberID,
			ProjectID:   projID,
			SubjectType: domain.SubjectUser,
			SubjectID:   "user-admin",
			Role:        domain.RoleAdmin,
			CreatedAt:   now,
		},
	}
	globalGuardState.mu.Unlock()

	makeCookie := func(token string) *http.Cookie {
		return &http.Cookie{
			Name:  SessionCookieName,
			Value: token,
		}
	}

	// -------------------------------------------------------------
	// Test 1: Block demoting the last project admin
	// -------------------------------------------------------------
	demoteBody, _ := json.Marshal(map[string]any{
		"subject_type": domain.SubjectUser,
		"subject_id":   "user-admin",
		"role":         domain.RoleEditor,
	})
	req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/v1/projects/%s/members", projID), bytes.NewReader(demoteBody))
	req.AddCookie(makeCookie(adminToken))
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 when demoting last admin, got %d: %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "cannot demote the last project admin") {
		t.Fatalf("expected 'cannot demote the last project admin' in error, got: %s", rec.Body.String())
	}

	// -------------------------------------------------------------
	// Test 2: Block removing the last project admin
	// -------------------------------------------------------------
	req = httptest.NewRequest(http.MethodDelete, fmt.Sprintf("/api/v1/projects/%s/members/%s", projID, adminMemberID), nil)
	req.AddCookie(makeCookie(adminToken))
	rec = httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 when removing last admin, got %d: %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "cannot remove the last project admin") {
		t.Fatalf("expected 'cannot remove the last project admin' in error, got: %s", rec.Body.String())
	}

	// -------------------------------------------------------------
	// Test 3: Add a second admin -> then demotion & removal succeed
	// -------------------------------------------------------------
	addAdmin2Body, _ := json.Marshal(map[string]any{
		"subject_type": domain.SubjectUser,
		"subject_id":   "user-member",
		"role":         domain.RoleAdmin,
	})
	req = httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/v1/projects/%s/members", projID), bytes.NewReader(addAdmin2Body))
	req.AddCookie(makeCookie(adminToken))
	rec = httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 when adding second admin, got %d: %s", rec.Code, rec.Body.String())
	}

	// Demoting user-admin now succeeds because user-member is also admin
	req = httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/v1/projects/%s/members", projID), bytes.NewReader(demoteBody))
	req.AddCookie(makeCookie(adminToken))
	rec = httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 when demoting user-admin with second admin present, got %d: %s", rec.Code, rec.Body.String())
	}

	// Removing user-admin now succeeds
	req = httptest.NewRequest(http.MethodDelete, fmt.Sprintf("/api/v1/projects/%s/members/%s", projID, adminMemberID), nil)
	req.AddCookie(makeCookie(memberToken))
	rec = httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected 204 when removing user-admin, got %d: %s", rec.Code, rec.Body.String())
	}

	// Now user-member is the ONLY admin. Demoting user-member should be blocked.
	demoteMemberBody, _ := json.Marshal(map[string]any{
		"subject_type": domain.SubjectUser,
		"subject_id":   "user-member",
		"role":         domain.RoleViewer,
	})
	req = httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/v1/projects/%s/members", projID), bytes.NewReader(demoteMemberBody))
	req.AddCookie(makeCookie(memberToken))
	rec = httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 when demoting newly sole admin user-member, got %d: %s", rec.Code, rec.Body.String())
	}

	// -------------------------------------------------------------
	// Test 4: System Administrator (manage_system) Emergency Access
	// -------------------------------------------------------------
	// user-sysadmin has domain.PermManageSystem but is NOT in project_members.

	// 4a. GET /api/v1/projects/{projectId} -> returns CurrentUserRole=admin and IsMember=true
	req = httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/v1/projects/%s", projID), nil)
	req.AddCookie(makeCookie(sysadminToken))
	rec = httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 for system admin emergency access, got %d: %s", rec.Code, rec.Body.String())
	}
	var fetchedProj domain.Project
	_ = json.NewDecoder(rec.Body).Decode(&fetchedProj)
	if fetchedProj.CurrentUserRole != domain.RoleAdmin || !fetchedProj.IsMember {
		t.Fatalf("expected CurrentUserRole=admin and IsMember=true for system admin, got role=%s, is_member=%v", fetchedProj.CurrentUserRole, fetchedProj.IsMember)
	}

	// 4b. requireProjectRole bypass: user-sysadmin can update project settings (requires RoleAdmin)
	updateProjBody, _ := json.Marshal(map[string]any{
		"name":          "Rescued by SysAdmin",
		"description":   "Emergency update",
		"hours_per_day": 8.0,
	})
	req = httptest.NewRequest(http.MethodPut, fmt.Sprintf("/api/v1/projects/%s", projID), bytes.NewReader(updateProjBody))
	req.AddCookie(makeCookie(sysadminToken))
	rec = httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 for system admin PUT project, got %d: %s", rec.Code, rec.Body.String())
	}

	// 4c. requireProjectRole bypass: user-sysadmin can manage members (requires RoleAdmin)
	addMemberBody, _ := json.Marshal(map[string]any{
		"subject_type": domain.SubjectUser,
		"subject_id":   "user-admin",
		"role":         domain.RoleAdmin,
	})
	req = httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/v1/projects/%s/members", projID), bytes.NewReader(addMemberBody))
	req.AddCookie(makeCookie(sysadminToken))
	rec = httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 for system admin adding admin member, got %d: %s", rec.Code, rec.Body.String())
	}

	// -------------------------------------------------------------
	// Test 5: Verify outsider (non-member without manage_system) is rejected
	// -------------------------------------------------------------
	req = httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/v1/projects/%s", projID), nil)
	req.AddCookie(makeCookie(outsiderToken))
	rec = httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for outsider GET project, got %d: %s", rec.Code, rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodPut, fmt.Sprintf("/api/v1/projects/%s", projID), bytes.NewReader(updateProjBody))
	req.AddCookie(makeCookie(outsiderToken))
	rec = httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for outsider PUT project, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestProjectAdminGuard_FullIntegration(t *testing.T) {
	rig := setupTestRig(t)
	defer rig.db.Close()

	srv := rig.srv
	adminCookie := rig.adminCookie
	adminUser := rig.adminUser

	// 1. Create a regular user (member1)
	createUserBody, _ := json.Marshal(map[string]any{
		"email":        "member1@example.com",
		"display_name": "Project Member 1",
		"password":     "Password123!",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/users", bytes.NewReader(createUserBody))
	req.AddCookie(adminCookie)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("failed to create member1 (%d): %s", rec.Code, rec.Body.String())
	}
	var member1 domain.User
	_ = json.NewDecoder(rec.Body).Decode(&member1)

	// Login as member1
	loginBody, _ := json.Marshal(map[string]string{
		"email":    "member1@example.com",
		"password": "Password123!",
	})
	req = httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewReader(loginBody))
	rec = httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("failed to login as member1 (%d): %s", rec.Code, rec.Body.String())
	}
	member1Cookies := rec.Result().Cookies()
	if len(member1Cookies) == 0 {
		t.Fatalf("expected cookie for member1")
	}
	member1Cookie := member1Cookies[0]

	// 2. Create another regular user (outsider, non-member)
	createOutsiderBody, _ := json.Marshal(map[string]any{
		"email":        "outsider@example.com",
		"display_name": "Outsider User",
		"password":     "Password123!",
	})
	req = httptest.NewRequest(http.MethodPost, "/api/v1/users", bytes.NewReader(createOutsiderBody))
	req.AddCookie(adminCookie)
	rec = httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("failed to create outsider (%d): %s", rec.Code, rec.Body.String())
	}

	loginOutsiderBody, _ := json.Marshal(map[string]string{
		"email":    "outsider@example.com",
		"password": "Password123!",
	})
	req = httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewReader(loginOutsiderBody))
	rec = httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	outsiderCookies := rec.Result().Cookies()
	if len(outsiderCookies) == 0 {
		t.Fatalf("expected cookie for outsider")
	}
	outsiderCookie := outsiderCookies[0]

	// 3. Admin creates a project (Admin becomes initial admin member)
	projBody, _ := json.Marshal(map[string]any{
		"name":        "Admin Guard Project",
		"description": "Testing admin demotion and removal block",
	})
	req = httptest.NewRequest(http.MethodPost, "/api/v1/projects", bytes.NewReader(projBody))
	req.AddCookie(adminCookie)
	rec = httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("failed to create project (%d): %s", rec.Code, rec.Body.String())
	}
	var proj domain.Project
	_ = json.NewDecoder(rec.Body).Decode(&proj)

	// Fetch members: adminUser should be the sole admin
	req = httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/v1/projects/%s/members", proj.ID), nil)
	req.AddCookie(adminCookie)
	rec = httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("failed to list project members (%d): %s", rec.Code, rec.Body.String())
	}
	var members []domain.ProjectMember
	_ = json.NewDecoder(rec.Body).Decode(&members)
	if len(members) != 1 || members[0].Role != domain.RoleAdmin {
		t.Fatalf("expected sole admin member in project, got %+v", members)
	}
	adminMemberEntryID := members[0].ID

	// Add member1 as editor
	addMember1Body, _ := json.Marshal(map[string]any{
		"subject_type": domain.SubjectUser,
		"subject_id":   member1.ID,
		"role":         domain.RoleEditor,
	})
	req = httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/v1/projects/%s/members", proj.ID), bytes.NewReader(addMember1Body))
	req.AddCookie(adminCookie)
	rec = httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("failed to add member1 as editor (%d): %s", rec.Code, rec.Body.String())
	}

	// 4. Test 1: Block demotion of the last project admin
	demoteAdminBody, _ := json.Marshal(map[string]any{
		"subject_type": domain.SubjectUser,
		"subject_id":   adminUser.ID,
		"role":         domain.RoleEditor,
	})
	req = httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/v1/projects/%s/members", proj.ID), bytes.NewReader(demoteAdminBody))
	req.AddCookie(adminCookie)
	rec = httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 when demoting the last project admin, got %d: %s", rec.Code, rec.Body.String())
	}

	// 5. Test 2: Block removal of the last project admin
	req = httptest.NewRequest(http.MethodDelete, fmt.Sprintf("/api/v1/projects/%s/members/%s", proj.ID, adminMemberEntryID), nil)
	req.AddCookie(adminCookie)
	rec = httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 when removing the last project admin, got %d: %s", rec.Code, rec.Body.String())
	}

	// 6. Promote member1 to admin -> now there are two admins
	promoteMember1Body, _ := json.Marshal(map[string]any{
		"subject_type": domain.SubjectUser,
		"subject_id":   member1.ID,
		"role":         domain.RoleAdmin,
	})
	req = httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/v1/projects/%s/members", proj.ID), bytes.NewReader(promoteMember1Body))
	req.AddCookie(adminCookie)
	rec = httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("failed to promote member1 to admin (%d): %s", rec.Code, rec.Body.String())
	}

	// 7. Now demoting adminUser should succeed because member1 is also admin
	req = httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/v1/projects/%s/members", proj.ID), bytes.NewReader(demoteAdminBody))
	req.AddCookie(adminCookie)
	rec = httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 when demoting adminUser with another admin existing, got %d: %s", rec.Code, rec.Body.String())
	}

	// 8. Remove adminUser from project -> adminUser is no longer a project member
	req = httptest.NewRequest(http.MethodDelete, fmt.Sprintf("/api/v1/projects/%s/members/%s", proj.ID, adminMemberEntryID), nil)
	req.AddCookie(member1Cookie)
	rec = httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("failed to remove adminUser from project (%d): %s", rec.Code, rec.Body.String())
	}

	// 9. Now member1 is the sole admin. Attempting to demote or remove member1 should fail
	demoteMember1Body, _ := json.Marshal(map[string]any{
		"subject_type": domain.SubjectUser,
		"subject_id":   member1.ID,
		"role":         domain.RoleViewer,
	})
	req = httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/v1/projects/%s/members", proj.ID), bytes.NewReader(demoteMember1Body))
	req.AddCookie(member1Cookie)
	rec = httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 when demoting member1 who is now the last admin, got %d: %s", rec.Code, rec.Body.String())
	}

	// Get member1's memberEntryID
	req = httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/v1/projects/%s/members", proj.ID), nil)
	req.AddCookie(member1Cookie)
	rec = httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	_ = json.NewDecoder(rec.Body).Decode(&members)
	if len(members) != 1 {
		t.Fatalf("expected 1 member left in project, got %d", len(members))
	}
	member1EntryID := members[0].ID

	req = httptest.NewRequest(http.MethodDelete, fmt.Sprintf("/api/v1/projects/%s/members/%s", proj.ID, member1EntryID), nil)
	req.AddCookie(member1Cookie)
	rec = httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 when removing member1 who is now the last admin, got %d: %s", rec.Code, rec.Body.String())
	}

	// 10. Test 3: System Administrator (manage_system) Emergency Access
	// adminUser is NOT a member of proj, but has domain.PermManageSystem.

	// Check GET /api/v1/projects/{projectId} as adminUser:
	req = httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/v1/projects/%s", proj.ID), nil)
	req.AddCookie(adminCookie)
	rec = httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 for system admin emergency GET project, got %d: %s", rec.Code, rec.Body.String())
	}
	var fetchedProj domain.Project
	_ = json.NewDecoder(rec.Body).Decode(&fetchedProj)
	if fetchedProj.CurrentUserRole != domain.RoleAdmin || !fetchedProj.IsMember {
		t.Fatalf("expected CurrentUserRole=admin and IsMember=true for system admin, got role=%s, is_member=%v", fetchedProj.CurrentUserRole, fetchedProj.IsMember)
	}

	// System admin should be able to update project details via requireProjectRole(RoleAdmin)
	updateProjBody, _ := json.Marshal(map[string]any{
		"name":          "Rescued Project Name",
		"description":   "Updated by superuser",
		"hours_per_day": 7.5,
	})
	req = httptest.NewRequest(http.MethodPut, fmt.Sprintf("/api/v1/projects/%s", proj.ID), bytes.NewReader(updateProjBody))
	req.AddCookie(adminCookie)
	rec = httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 for system admin emergency PUT project, got %d: %s", rec.Code, rec.Body.String())
	}

	// System admin should be able to add themselves back as project admin
	reAddAdminBody, _ := json.Marshal(map[string]any{
		"subject_type": domain.SubjectUser,
		"subject_id":   adminUser.ID,
		"role":         domain.RoleAdmin,
	})
	req = httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/v1/projects/%s/members", proj.ID), bytes.NewReader(reAddAdminBody))
	req.AddCookie(adminCookie)
	rec = httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 for system admin adding admin member, got %d: %s", rec.Code, rec.Body.String())
	}

	// 11. Verify that outsider (non-member, no manage_system) is rejected
	req = httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/v1/projects/%s", proj.ID), nil)
	req.AddCookie(outsiderCookie)
	rec = httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for outsider GET project, got %d: %s", rec.Code, rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodPut, fmt.Sprintf("/api/v1/projects/%s", proj.ID), bytes.NewReader(updateProjBody))
	req.AddCookie(outsiderCookie)
	rec = httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for outsider PUT project, got %d: %s", rec.Code, rec.Body.String())
	}
}
