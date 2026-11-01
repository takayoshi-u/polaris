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
	"github.com/takayoshi-u/polaris/backend/internal/pkg/id"
	"github.com/takayoshi-u/polaris/backend/internal/store"
)

// =========================================================================
// In-Memory Mock Driver for Bulk Direct User Tests
// =========================================================================

type bulkMockUser struct {
	ID                 string
	Email              string
	PasswordHash       string
	DisplayName        string
	AvatarURL          string
	Status             string
	MFAEnabled         bool
	MFAEnforced        bool
	MustChangePassword bool
	CreatedAt          time.Time
	UpdatedAt          time.Time
}

type bulkMockState struct {
	mu       sync.Mutex
	users    map[string]*bulkMockUser
	sessions map[string]string   // tokenHash -> userID
	sysPerms map[string][]string // userID -> permissions
}

var globalBulkState = &bulkMockState{
	users:    make(map[string]*bulkMockUser),
	sessions: make(map[string]string),
	sysPerms: make(map[string][]string),
}

func init() {
	sql.Register("bulk_direct_mock", &bulkMockDriver{})
}

type bulkMockDriver struct{}

func (d *bulkMockDriver) Open(name string) (driver.Conn, error) {
	return &bulkMockConn{state: globalBulkState}, nil
}

type bulkMockConn struct {
	state *bulkMockState
}

func (c *bulkMockConn) Prepare(query string) (driver.Stmt, error) {
	return &bulkMockStmt{conn: c, query: query}, nil
}

func (c *bulkMockConn) Close() error { return nil }
func (c *bulkMockConn) Begin() (driver.Tx, error) {
	return &bulkMockTx{}, nil
}

type bulkMockTx struct{}

func (t *bulkMockTx) Commit() error   { return nil }
func (t *bulkMockTx) Rollback() error { return nil }

type bulkMockStmt struct {
	conn  *bulkMockConn
	query string
}

func (s *bulkMockStmt) Close() error  { return nil }
func (s *bulkMockStmt) NumInput() int { return -1 }

func (s *bulkMockStmt) Exec(args []driver.Value) (driver.Result, error) {
	q := strings.TrimSpace(s.query)
	st := s.conn.state
	st.mu.Lock()
	defer st.mu.Unlock()

	switch {
	case strings.Contains(q, "INSERT INTO users"):
		// id, email, password_hash, display_name, status, must_change_password, created_at, updated_at
		uID := fmt.Sprintf("%v", args[0])
		email := fmt.Sprintf("%v", args[1])
		pwHash := ""
		if args[2] != nil {
			pwHash = fmt.Sprintf("%v", args[2])
		}
		dName := fmt.Sprintf("%v", args[3])
		status := fmt.Sprintf("%v", args[4])
		mustChange := false
		if len(args) > 5 && args[5] != nil {
			if mc, ok := args[5].(bool); ok {
				mustChange = mc
			}
		}
		now := time.Now().UTC()
		st.users[uID] = &bulkMockUser{
			ID:                 uID,
			Email:              email,
			PasswordHash:       pwHash,
			DisplayName:        dName,
			Status:             status,
			MustChangePassword: mustChange,
			MFAEnabled:         false,
			MFAEnforced:        false,
			CreatedAt:          now,
			UpdatedAt:          now,
		}
		return driver.RowsAffected(1), nil

	case strings.Contains(q, "UPDATE users SET password_hash = $1"):
		// UPDATE users SET password_hash = $1, must_change_password = FALSE, updated_at = $2 WHERE id = $3
		newHash := fmt.Sprintf("%v", args[0])
		targetID := fmt.Sprintf("%v", args[2])
		if u, ok := st.users[targetID]; ok {
			u.PasswordHash = newHash
			u.MustChangePassword = false
			u.UpdatedAt = time.Now().UTC()
			return driver.RowsAffected(1), nil
		}
		return driver.RowsAffected(0), nil

	case strings.Contains(q, "INSERT INTO user_sessions"):
		// id, user_id, token_hash, expires_at, created_at
		uID := fmt.Sprintf("%v", args[1])
		tHash := fmt.Sprintf("%v", args[2])
		st.sessions[tHash] = uID
		return driver.RowsAffected(1), nil

	case strings.Contains(q, "INSERT INTO group_members"):
		return driver.RowsAffected(1), nil

	case strings.Contains(q, "INSERT INTO system_permissions"):
		uID := fmt.Sprintf("%v", args[2])
		perm := fmt.Sprintf("%v", args[3])
		st.sysPerms[uID] = append(st.sysPerms[uID], perm)
		return driver.RowsAffected(1), nil
	}

	return driver.RowsAffected(1), nil
}

func (s *bulkMockStmt) Query(args []driver.Value) (driver.Rows, error) {
	q := strings.TrimSpace(s.query)
	st := s.conn.state
	st.mu.Lock()
	defer st.mu.Unlock()

	switch {
	case strings.Contains(q, "SELECT COUNT(*)") && strings.Contains(q, "status != 'disabled'"):
		email := strings.ToLower(fmt.Sprintf("%v", args[0]))
		count := int64(0)
		for _, u := range st.users {
			if strings.ToLower(u.Email) == email && u.Status != domain.UserStatusDisabled {
				count++
			}
		}
		return &mockRows{cols: []string{"count"}, rows: [][]driver.Value{{count}}}, nil

	case strings.Contains(q, "SELECT COUNT(*)") && strings.Contains(q, "FROM users"):
		return &mockRows{cols: []string{"count"}, rows: [][]driver.Value{{int64(len(st.users))}}}, nil

	case strings.Contains(q, "FROM users") && strings.Contains(q, "status != 'disabled'") && strings.Contains(q, "password_hash"):
		// AuthenticateWithPassword
		email := strings.ToLower(fmt.Sprintf("%v", args[0]))
		cols := []string{"id", "password_hash", "status", "mfa_enabled", "totp_secret"}
		for _, u := range st.users {
			if strings.ToLower(u.Email) == email && u.Status != domain.UserStatusDisabled {
				return &mockRows{cols: cols, rows: [][]driver.Value{{
					u.ID, u.PasswordHash, u.Status, u.MFAEnabled, nil,
				}}}, nil
			}
		}
		return &mockRows{cols: cols, rows: nil}, nil

	case strings.Contains(q, "SELECT password_hash FROM users WHERE id = $1"):
		// SetOrChangePassword verification
		uID := fmt.Sprintf("%v", args[0])
		u, ok := st.users[uID]
		cols := []string{"password_hash"}
		if !ok {
			return &mockRows{cols: cols, rows: nil}, nil
		}
		return &mockRows{cols: cols, rows: [][]driver.Value{{u.PasswordHash}}}, nil

	case strings.Contains(q, "FROM users WHERE id = $1"):
		// GetUserByID
		uID := fmt.Sprintf("%v", args[0])
		u, ok := st.users[uID]
		cols := []string{"id", "email", "password_hash", "google_sub", "display_name", "avatar_url", "status", "mfa_enabled", "mfa_enforced", "must_change_password", "created_at", "updated_at"}
		if !ok {
			return &mockRows{cols: cols, rows: nil}, nil
		}
		return &mockRows{cols: cols, rows: [][]driver.Value{{
			u.ID, u.Email, u.PasswordHash, nil, u.DisplayName, u.AvatarURL, u.Status, u.MFAEnabled, u.MFAEnforced, u.MustChangePassword, u.CreatedAt, u.UpdatedAt,
		}}}, nil

	case strings.Contains(q, "FROM users ORDER BY id ASC"):
		// ListUsers
		cols := []string{"id", "email", "password_hash", "google_sub", "display_name", "avatar_url", "status", "mfa_enabled", "mfa_enforced", "must_change_password", "created_at", "updated_at"}
		var values [][]driver.Value
		for _, u := range st.users {
			values = append(values, []driver.Value{
				u.ID, u.Email, u.PasswordHash, nil, u.DisplayName, u.AvatarURL, u.Status, u.MFAEnabled, u.MFAEnforced, u.MustChangePassword, u.CreatedAt, u.UpdatedAt,
			})
		}
		return &mockRows{cols: cols, rows: values}, nil

	case strings.Contains(q, "user_sessions") && strings.Contains(q, "token_hash"):
		tHash := fmt.Sprintf("%v", args[0])
		uID, ok := st.sessions[tHash]
		cols := []string{"user_id", "expires_at"}
		if !ok {
			return &mockRows{cols: cols, rows: nil}, nil
		}
		return &mockRows{cols: cols, rows: [][]driver.Value{{
			uID, time.Now().Add(24 * time.Hour),
		}}}, nil

	case strings.Contains(q, "SELECT DISTINCT sp.permission"):
		uID := fmt.Sprintf("%v", args[0])
		perms := st.sysPerms[uID]
		cols := []string{"permission"}
		var values [][]driver.Value
		for _, p := range perms {
			values = append(values, []driver.Value{p})
		}
		return &mockRows{cols: cols, rows: values}, nil

	case strings.Contains(q, "FROM system_allowed_domains"):
		return &mockRows{cols: []string{"domain"}, rows: nil}, nil

	case strings.Contains(q, "FROM system_settings"):
		cols := []string{"id", "default_hours_per_day", "default_issue_template_id", "default_task_template_id", "require_mfa", "updated_at"}
		values := [][]driver.Value{{id.NewV7String(), 8.0, nil, nil, false, time.Now().UTC()}}
		return &mockRows{cols: cols, rows: values}, nil
	}

	return &mockRows{cols: []string{"dummy"}, rows: nil}, nil
}

// =========================================================================
// Test: Bulk Direct User Creation & Temporary Password Enforcement
// =========================================================================

func TestBulkDirectUserRegistration_TempPasswordEnforcement(t *testing.T) {
	mockDB, err := sql.Open("bulk_direct_mock", "")
	if err != nil {
		t.Fatalf("open mock db: %v", err)
	}
	defer mockDB.Close()

	mailer := mail.NewSMTPSender()
	fcmClient := fcm.NewClient(context.Background(), "")
	st := store.New(mockDB, "0123456789abcdef0123456789abcdef", "http://localhost:3000", mailer, fcmClient)
	srv := NewServer(st, "http://localhost:3000", GoogleOAuthConfig{})

	// Setup initial system admin
	adminID := "admin-user-id"
	adminToken := "admin-session-raw-token"
	globalBulkState.mu.Lock()
	globalBulkState.users = make(map[string]*bulkMockUser)
	globalBulkState.sessions = make(map[string]string)
	globalBulkState.sysPerms = make(map[string][]string)

	now := time.Now().UTC()
	globalBulkState.users[adminID] = &bulkMockUser{
		ID:                 adminID,
		Email:              "admin@example.com",
		DisplayName:        "Admin",
		Status:             domain.UserStatusActive,
		MFAEnabled:         true,
		MustChangePassword: false,
		CreatedAt:          now,
		UpdatedAt:          now,
	}
	globalBulkState.sessions[crypto.HashToken(adminToken)] = adminID
	globalBulkState.sysPerms[adminID] = []string{domain.PermManageSystem}
	globalBulkState.mu.Unlock()

	adminCookie := &http.Cookie{
		Name:  SessionCookieName,
		Value: adminToken,
	}

	// -------------------------------------------------------------
	// 1. Bulk Direct User Registration via POST /api/v1/users/bulk-direct
	// -------------------------------------------------------------
	reqBody := domain.BulkDirectUserRequest{
		Users: []domain.BulkDirectUserItem{
			{
				Email:       "taro@example.com",
				DisplayName: "Taro Yamada",
				Password:    "TaroCustomPass123!",
			},
			{
				Email:       "hanako@example.com",
				DisplayName: "", // auto display_name from email prefix
				Password:    "", // should use default or auto-generated temp password
			},
			{
				Email: "invalid-email-address",
			},
			{
				Email: "taro@example.com", // duplicate in batch
			},
		},
		DefaultPassword:    "",
		MustChangePassword: true,
	}

	jsonBytes, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/users/bulk-direct", bytes.NewReader(jsonBytes))
	req.AddCookie(adminCookie)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("step 1: expected 200 for bulk-direct, got %d: %s", rec.Code, rec.Body.String())
	}

	var res domain.BulkDirectUserResult
	if err := json.NewDecoder(rec.Body).Decode(&res); err != nil {
		t.Fatalf("step 1: decode response failed: %v", err)
	}

	if res.Total != 4 {
		t.Fatalf("step 1: expected total 4, got %d", res.Total)
	}
	if res.CreatedCount != 2 {
		t.Fatalf("step 1: expected created_count 2, got %d", res.CreatedCount)
	}
	if res.FailedCount != 2 {
		t.Fatalf("step 1: expected failed_count 2, got %d", res.FailedCount)
	}
	if res.Succeeded != 2 {
		t.Fatalf("step 1: expected succeeded 2, got %d", res.Succeeded)
	}
	if res.Failed != 2 {
		t.Fatalf("step 1: expected failed 2, got %d", res.Failed)
	}
	if len(res.CreatedUsers) != 2 {
		t.Fatalf("step 1: expected 2 CreatedUsers, got %d", len(res.CreatedUsers))
	}

	// Verify User 1 (Taro)
	user1Cred := res.CreatedUsers[0]
	if user1Cred.User.Email != "taro@example.com" {
		t.Errorf("step 1: expected user1 email taro@example.com, got %s", user1Cred.User.Email)
	}
	if user1Cred.TempPassword != "TaroCustomPass123!" {
		t.Errorf("step 1: expected user1 temp password TaroCustomPass123!, got %s", user1Cred.TempPassword)
	}
	if !user1Cred.User.MustChangePassword {
		t.Errorf("step 1: expected user1 must_change_password to be true")
	}

	// Verify User 2 (Hanako: auto generated 12-char password and display_name hanako)
	user2Cred := res.CreatedUsers[1]
	if user2Cred.User.Email != "hanako@example.com" {
		t.Errorf("step 1: expected user2 email hanako@example.com, got %s", user2Cred.User.Email)
	}
	if user2Cred.User.DisplayName != "hanako" {
		t.Errorf("step 1: expected user2 display_name hanako, got %s", user2Cred.User.DisplayName)
	}
	if len(user2Cred.TempPassword) != 12 {
		t.Errorf("step 1: expected 12-char generated password, got %s (len %d)", user2Cred.TempPassword, len(user2Cred.TempPassword))
	}
	if !user2Cred.User.MustChangePassword {
		t.Errorf("step 1: expected user2 must_change_password to be true")
	}

	// -------------------------------------------------------------
	// 2. Login as User 1 with Temp Password
	// -------------------------------------------------------------
	loginBody, _ := json.Marshal(map[string]string{
		"email":    "taro@example.com",
		"password": user1Cred.TempPassword,
	})
	req = httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewReader(loginBody))
	rec = httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("step 2: expected 200 for User 1 login with temp password, got %d: %s", rec.Code, rec.Body.String())
	}

	cookies := rec.Result().Cookies()
	var user1Cookie *http.Cookie
	for _, c := range cookies {
		if c.Name == SessionCookieName && c.Value != "" {
			user1Cookie = c
			break
		}
	}
	if user1Cookie == nil {
		t.Fatalf("step 2: expected session cookie for User 1 login")
	}

	// -------------------------------------------------------------
	// 3. Temporary Password Enforcement: Block access to regular endpoints (403)
	// -------------------------------------------------------------
	req = httptest.NewRequest(http.MethodGet, "/api/v1/users", nil)
	req.AddCookie(user1Cookie)
	rec = httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("step 3: expected 403 Forbidden for User 1 with temp password, got %d: %s", rec.Code, rec.Body.String())
	}
	var errMap map[string]any
	_ = json.NewDecoder(rec.Body).Decode(&errMap)
	if errMap["error"] != "PASSWORD_CHANGE_REQUIRED" {
		t.Fatalf("step 3: expected error code PASSWORD_CHANGE_REQUIRED, got %v", errMap["error"])
	}

	// -------------------------------------------------------------
	// 4. Allowed Endpoints while must_change_password is true
	// -------------------------------------------------------------
	// 4a: GET /api/v1/auth/me
	req = httptest.NewRequest(http.MethodGet, "/api/v1/auth/me", nil)
	req.AddCookie(user1Cookie)
	rec = httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("step 4a: expected 200 for GET /api/v1/auth/me, got %d: %s", rec.Code, rec.Body.String())
	}

	// 4b: GET /api/v1/users/me
	req = httptest.NewRequest(http.MethodGet, "/api/v1/users/me", nil)
	req.AddCookie(user1Cookie)
	rec = httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("step 4b: expected 200 for GET /api/v1/users/me, got %d: %s", rec.Code, rec.Body.String())
	}

	// -------------------------------------------------------------
	// 5. Change Password via PUT /api/v1/auth/me/password
	// -------------------------------------------------------------
	newPassword := "NewPermanentPass789!"
	changePassBody, _ := json.Marshal(map[string]string{
		"current_password": user1Cred.TempPassword,
		"new_password":     newPassword,
	})
	req = httptest.NewRequest(http.MethodPut, "/api/v1/auth/me/password", bytes.NewReader(changePassBody))
	req.AddCookie(user1Cookie)
	rec = httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("step 5: expected 200 for password change, got %d: %s", rec.Code, rec.Body.String())
	}

	// -------------------------------------------------------------
	// 6. Access Unblocked: User 1 can now access regular endpoints
	// -------------------------------------------------------------
	req = httptest.NewRequest(http.MethodGet, "/api/v1/users", nil)
	req.AddCookie(user1Cookie)
	rec = httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("step 6: expected 200 for GET /api/v1/users after changing password, got %d: %s", rec.Code, rec.Body.String())
	}

	// -------------------------------------------------------------
	// 7. Single Direct User Creation via POST /api/v1/users (default must_change_password: true)
	// -------------------------------------------------------------
	singleBody, _ := json.Marshal(map[string]any{
		"email":        "jiro@example.com",
		"display_name": "Jiro",
		"password":     "JiroPass123!",
	})
	req = httptest.NewRequest(http.MethodPost, "/api/v1/users", bytes.NewReader(singleBody))
	req.AddCookie(adminCookie)
	rec = httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("step 7: expected 201 for single user create, got %d: %s", rec.Code, rec.Body.String())
	}
	var resJiro domain.User
	_ = json.NewDecoder(rec.Body).Decode(&resJiro)
	if !resJiro.MustChangePassword {
		t.Errorf("step 7: expected must_change_password to default to true for direct creation")
	}
}
