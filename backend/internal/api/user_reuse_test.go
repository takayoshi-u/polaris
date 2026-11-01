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
	"os"
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
	"github.com/takayoshi-u/polaris/backend/migrations"
)

// =========================================================================
// In-Memory Mock Driver for User Reuse Tests
// =========================================================================

type reuseMockUser struct {
	ID           string
	Email        string
	PasswordHash string
	DisplayName  string
	AvatarURL    string
	Status       string
	MFAEnabled   bool
	MFAEnforced  bool
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

type reuseMockState struct {
	mu          sync.Mutex
	users       map[string]*reuseMockUser // id -> user
	sessions    map[string]string         // tokenHash -> userID
	sysPerms    map[string][]string       // userID -> permissions
	resetTokens map[string]string         // tokenHash -> userID
}

var globalReuseState = &reuseMockState{
	users:       make(map[string]*reuseMockUser),
	sessions:    make(map[string]string),
	sysPerms:    make(map[string][]string),
	resetTokens: make(map[string]string),
}

func init() {
	sql.Register("user_reuse_mock", &reuseMockDriver{})
}

type reuseMockDriver struct{}

func (d *reuseMockDriver) Open(name string) (driver.Conn, error) {
	return &reuseMockConn{state: globalReuseState}, nil
}

type reuseMockConn struct {
	state *reuseMockState
}

func (c *reuseMockConn) Prepare(query string) (driver.Stmt, error) {
	return &reuseMockStmt{conn: c, query: query}, nil
}

func (c *reuseMockConn) Close() error { return nil }
func (c *reuseMockConn) Begin() (driver.Tx, error) {
	return &reuseMockTx{}, nil
}

type reuseMockTx struct{}

func (t *reuseMockTx) Commit() error   { return nil }
func (t *reuseMockTx) Rollback() error { return nil }

type reuseMockStmt struct {
	conn  *reuseMockConn
	query string
}

func (s *reuseMockStmt) Close() error  { return nil }
func (s *reuseMockStmt) NumInput() int { return -1 }

func (s *reuseMockStmt) Exec(args []driver.Value) (driver.Result, error) {
	q := strings.TrimSpace(s.query)
	st := s.conn.state
	st.mu.Lock()
	defer st.mu.Unlock()

	switch {
	case strings.Contains(q, "INSERT INTO users"):
		uID := fmt.Sprintf("%v", args[0])
		email := fmt.Sprintf("%v", args[1])
		pwHash := ""
		if args[2] != nil {
			pwHash = fmt.Sprintf("%v", args[2])
		}
		dName := fmt.Sprintf("%v", args[3])
		status := fmt.Sprintf("%v", args[4])
		now := time.Now().UTC()
		st.users[uID] = &reuseMockUser{
			ID:           uID,
			Email:        email,
			PasswordHash: pwHash,
			DisplayName:  dName,
			Status:       status,
			CreatedAt:    now,
			UpdatedAt:    now,
		}
		return driver.RowsAffected(1), nil

	case strings.Contains(q, "UPDATE users SET status = $1"):
		// UPDATE users SET status = $1, updated_at = $2 WHERE id = $3
		newStatus := fmt.Sprintf("%v", args[0])
		targetID := fmt.Sprintf("%v", args[2])
		if u, ok := st.users[targetID]; ok {
			u.Status = newStatus
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

	case strings.Contains(q, "DELETE FROM user_sessions WHERE user_id = $1"):
		uID := fmt.Sprintf("%v", args[0])
		for h, id := range st.sessions {
			if id == uID {
				delete(st.sessions, h)
			}
		}
		return driver.RowsAffected(1), nil

	case strings.Contains(q, "INSERT INTO email_verification_tokens"):
		// id, user_id, token_type, token_hash, expires_at, created_at
		uID := fmt.Sprintf("%v", args[1])
		tHash := fmt.Sprintf("%v", args[3])
		st.resetTokens[tHash] = uID
		return driver.RowsAffected(1), nil

	case strings.Contains(q, "DELETE FROM email_verification_tokens"):
		return driver.RowsAffected(1), nil

	case strings.Contains(q, "UPDATE users SET google_sub = NULL"):
		return driver.RowsAffected(1), nil
	}

	return driver.RowsAffected(1), nil
}

func (s *reuseMockStmt) Query(args []driver.Value) (driver.Rows, error) {
	q := strings.TrimSpace(s.query)
	st := s.conn.state
	st.mu.Lock()
	defer st.mu.Unlock()

	switch {
	case strings.Contains(q, "SELECT COUNT(*)") && strings.Contains(q, "status != 'disabled'") && strings.Contains(q, "id <> $2"):
		// Email change duplicate check: SELECT COUNT(*) FROM users WHERE LOWER(email) = $1 AND id <> $2 AND status != 'disabled'
		email := strings.ToLower(fmt.Sprintf("%v", args[0]))
		excludeID := fmt.Sprintf("%v", args[1])
		count := int64(0)
		for _, u := range st.users {
			if strings.ToLower(u.Email) == email && u.ID != excludeID && u.Status != domain.UserStatusDisabled {
				count++
			}
		}
		return &mockRows{cols: []string{"count"}, rows: [][]driver.Value{{count}}}, nil

	case strings.Contains(q, "SELECT COUNT(*)") && strings.Contains(q, "status != 'disabled'"):
		// Duplicate check: SELECT COUNT(*) FROM users WHERE LOWER(email) = $1 AND status != 'disabled'
		email := strings.ToLower(fmt.Sprintf("%v", args[0]))
		count := int64(0)
		for _, u := range st.users {
			if strings.ToLower(u.Email) == email && u.Status != domain.UserStatusDisabled {
				count++
			}
		}
		return &mockRows{cols: []string{"count"}, rows: [][]driver.Value{{count}}}, nil

	case strings.Contains(q, "SELECT COUNT(*)") && strings.Contains(q, "FROM users"):
		// Setup initial check
		return &mockRows{cols: []string{"count"}, rows: [][]driver.Value{{int64(len(st.users))}}}, nil

	case strings.Contains(q, "FROM users") && strings.Contains(q, "status != 'disabled'") && strings.Contains(q, "password_hash"):
		// AuthenticateWithPassword: SELECT id, password_hash, status, mfa_enabled, totp_secret FROM users WHERE LOWER(email) = $1 AND status != 'disabled'
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

	case strings.Contains(q, "FROM users") && strings.Contains(q, "status != 'disabled'") && strings.Contains(q, "display_name"):
		// RequestPasswordReset: SELECT id, display_name, status FROM users WHERE LOWER(email) = $1 AND status != 'disabled'
		email := strings.ToLower(fmt.Sprintf("%v", args[0]))
		cols := []string{"id", "display_name", "status"}
		for _, u := range st.users {
			if strings.ToLower(u.Email) == email && u.Status != domain.UserStatusDisabled {
				return &mockRows{cols: cols, rows: [][]driver.Value{{
					u.ID, u.DisplayName, u.Status,
				}}}, nil
			}
		}
		return &mockRows{cols: cols, rows: nil}, nil

	case strings.Contains(q, "FROM users") && strings.Contains(q, "WHERE id = $1"):
		// GetUserByID
		uID := fmt.Sprintf("%v", args[0])
		u, ok := st.users[uID]
		cols := []string{"id", "email", "password_hash", "google_sub", "display_name", "avatar_url", "status", "mfa_enabled", "mfa_enforced", "created_at", "updated_at"}
		if !ok {
			return &mockRows{cols: cols, rows: nil}, nil
		}
		return &mockRows{cols: cols, rows: [][]driver.Value{{
			u.ID, u.Email, u.PasswordHash, nil, u.DisplayName, u.AvatarURL, u.Status, u.MFAEnabled, u.MFAEnforced, u.CreatedAt, u.UpdatedAt,
		}}}, nil

	case strings.Contains(q, "user_sessions") && strings.Contains(q, "token_hash"):
		// GetUserBySessionToken
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
		// GetUserSystemPermissions
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

	case strings.Contains(q, "SELECT COUNT(DISTINCT u.id)"):
		// otherActiveAdmins check
		return &mockRows{cols: []string{"count"}, rows: [][]driver.Value{{int64(2)}}}, nil
	}

	return &mockRows{cols: []string{"dummy"}, rows: nil}, nil
}

// =========================================================================
// Unit Test: Disabled Email Reuse Lifecycle
// =========================================================================

func TestUserEmailReuseLifecycle_UnitTest(t *testing.T) {
	mockDB, err := sql.Open("user_reuse_mock", "")
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
	globalReuseState.mu.Lock()
	globalReuseState.users = make(map[string]*reuseMockUser)
	globalReuseState.sessions = make(map[string]string)
	globalReuseState.sysPerms = make(map[string][]string)
	globalReuseState.resetTokens = make(map[string]string)

	now := time.Now().UTC()
	globalReuseState.users[adminID] = &reuseMockUser{
		ID:           adminID,
		Email:        "admin@example.com",
		DisplayName:  "Admin",
		Status:       domain.UserStatusActive,
		MFAEnabled:   true,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	globalReuseState.sessions[crypto.HashToken(adminToken)] = adminID
	globalReuseState.sysPerms[adminID] = []string{domain.PermManageSystem}
	globalReuseState.mu.Unlock()

	adminCookie := &http.Cookie{
		Name:  SessionCookieName,
		Value: adminToken,
	}

	targetEmail := "reuse.target@example.com"
	passUserA := "PassUserA123!"
	passUserB := "PassUserB456!"

	// -------------------------------------------------------------
	// 1. Register User A (Active)
	// -------------------------------------------------------------
	createBodyA, _ := json.Marshal(map[string]any{
		"email":        targetEmail,
		"display_name": "User A (Original)",
		"password":     passUserA,
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/users", bytes.NewReader(createBodyA))
	req.AddCookie(adminCookie)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("step 1: expected 201 for creating User A, got %d: %s", rec.Code, rec.Body.String())
	}
	var resUserA domain.User
	if err := json.NewDecoder(rec.Body).Decode(&resUserA); err != nil {
		t.Fatalf("step 1: decode user A failed: %v", err)
	}
	userAID := resUserA.ID
	if userAID == "" {
		t.Fatalf("step 1: expected non-empty ID for User A")
	}

	// -------------------------------------------------------------
	// 2. Duplicate registration while User A is active must fail (409 Conflict)
	// -------------------------------------------------------------
	createBodyDup, _ := json.Marshal(map[string]any{
		"email":        targetEmail,
		"display_name": "User A Duplicate",
		"password":     "AnotherPass!",
	})
	req = httptest.NewRequest(http.MethodPost, "/api/v1/users", bytes.NewReader(createBodyDup))
	req.AddCookie(adminCookie)
	rec = httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusConflict {
		t.Fatalf("step 2: expected 409 Conflict on duplicate registration, got %d: %s", rec.Code, rec.Body.String())
	}

	// -------------------------------------------------------------
	// 3. User A login verification
	// -------------------------------------------------------------
	loginBodyA, _ := json.Marshal(map[string]string{
		"email":    targetEmail,
		"password": passUserA,
	})
	req = httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewReader(loginBodyA))
	rec = httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("step 3: expected 200 for User A login, got %d: %s", rec.Code, rec.Body.String())
	}
	var loginResA map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&loginResA); err != nil {
		t.Fatalf("step 3: decode login res A failed: %v", err)
	}
	uMapA, _ := loginResA["user"].(map[string]any)
	if uMapA["id"] != userAID {
		t.Fatalf("step 3: expected login user ID %s, got %v", userAID, uMapA["id"])
	}

	// -------------------------------------------------------------
	// 4. Deactivate User A (status = 'disabled')
	// -------------------------------------------------------------
	statusBody, _ := json.Marshal(map[string]string{
		"status": domain.UserStatusDisabled,
	})
	req = httptest.NewRequest(http.MethodPut, fmt.Sprintf("/api/v1/users/%s/status", userAID), bytes.NewReader(statusBody))
	req.AddCookie(adminCookie)
	rec = httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("step 4: expected 200 deactivating User A, got %d: %s", rec.Code, rec.Body.String())
	}

	// -------------------------------------------------------------
	// 5. Register User B with the same email address (should SUCCEED)
	// -------------------------------------------------------------
	createBodyB, _ := json.Marshal(map[string]any{
		"email":        targetEmail,
		"display_name": "User B (New Person)",
		"password":     passUserB,
	})
	req = httptest.NewRequest(http.MethodPost, "/api/v1/users", bytes.NewReader(createBodyB))
	req.AddCookie(adminCookie)
	rec = httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("step 5: expected 201 for creating User B with same email as disabled User A, got %d: %s", rec.Code, rec.Body.String())
	}
	var resUserB domain.User
	if err := json.NewDecoder(rec.Body).Decode(&resUserB); err != nil {
		t.Fatalf("step 5: decode user B failed: %v", err)
	}
	userBID := resUserB.ID
	if userBID == "" {
		t.Fatalf("step 5: expected non-empty ID for User B")
	}
	if userBID == userAID {
		t.Fatalf("step 5: expected User B to have a distinct ID from User A, but both got %s", userAID)
	}

	// -------------------------------------------------------------
	// 6. User B login verification (Authenticates User B, not User A)
	// -------------------------------------------------------------
	// 6a: Login with User B's password -> must succeed and return User B
	loginBodyB, _ := json.Marshal(map[string]string{
		"email":    targetEmail,
		"password": passUserB,
	})
	req = httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewReader(loginBodyB))
	rec = httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("step 6a: expected 200 for User B login, got %d: %s", rec.Code, rec.Body.String())
	}
	var loginResB map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&loginResB); err != nil {
		t.Fatalf("step 6a: decode login res B failed: %v", err)
	}
	uMapB, _ := loginResB["user"].(map[string]any)
	if uMapB["id"] != userBID {
		t.Fatalf("step 6a: expected login user ID %s (User B), got %v", userBID, uMapB["id"])
	}

	// 6b: Login with User A's password -> must fail with 401 Unauthorized
	req = httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewReader(loginBodyA))
	rec = httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("step 6b: expected 401 Unauthorized for old User A password, got %d", rec.Code)
	}

	// -------------------------------------------------------------
	// 7. Duplicate registration while User B is active must fail (409 Conflict)
	// -------------------------------------------------------------
	createBodyC, _ := json.Marshal(map[string]any{
		"email":        targetEmail,
		"display_name": "User C Duplicate",
		"password":     "PassUserC789!",
	})
	req = httptest.NewRequest(http.MethodPost, "/api/v1/users", bytes.NewReader(createBodyC))
	req.AddCookie(adminCookie)
	rec = httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusConflict {
		t.Fatalf("step 7: expected 409 Conflict when User B is active, got %d: %s", rec.Code, rec.Body.String())
	}

	// -------------------------------------------------------------
	// 8. Password reset sends to currently active user (User B)
	// -------------------------------------------------------------
	pwResetBody, _ := json.Marshal(map[string]string{
		"email": targetEmail,
	})
	req = httptest.NewRequest(http.MethodPost, "/api/v1/auth/password-reset/request", bytes.NewReader(pwResetBody))
	rec = httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("step 8: expected 200 for password reset request, got %d: %s", rec.Code, rec.Body.String())
	}

	// -------------------------------------------------------------
	// 9. Store-level AdminChangeUserEmail conflict check
	// -------------------------------------------------------------
	// User C exists with another email
	userCID := "user-c-id"
	globalReuseState.mu.Lock()
	globalReuseState.users[userCID] = &reuseMockUser{
		ID:          userCID,
		Email:       "userc@example.com",
		DisplayName: "User C",
		Status:      domain.UserStatusActive,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	globalReuseState.mu.Unlock()

	// Changing User C's email to targetEmail (active User B) must return ErrConflict
	_, _, err = st.AdminChangeUserEmail(context.Background(), userCID, targetEmail, false)
	if err == nil || !strings.Contains(err.Error(), "already in use") {
		t.Fatalf("step 9: expected 'already in use' conflict error, got %v", err)
	}
}

// =========================================================================
// Real PostgreSQL Integration Test (when TEST_DATABASE_URL is set)
// =========================================================================

func TestUserEmailReuse_DatabaseIntegration(t *testing.T) {
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
	fcmClient := fcm.NewClient(ctx, "")
	st := store.New(db, "0123456789abcdef0123456789abcdef", "http://localhost:3000", mailer, fcmClient)

	targetEmail := "reusetest@example.com"
	passA := "UserPassA123!"
	passB := "UserPassB456!"

	// 1. Create User A directly
	userA, err := st.CreateUserDirect(ctx, targetEmail, "User A", passA, nil, nil)
	if err != nil {
		t.Fatalf("create user A failed: %v", err)
	}

	// 2. Duplicate while active -> must fail
	_, err = st.CreateUserDirect(ctx, targetEmail, "User A Dup", "PassDup123!", nil, nil)
	if err == nil {
		t.Fatalf("expected conflict error on duplicate active user, got nil")
	}

	// 3. Authenticate User A
	resA, err := st.AuthenticateWithPassword(ctx, targetEmail, passA)
	if err != nil {
		t.Fatalf("authenticate user A failed: %v", err)
	}
	if resA.User == nil || resA.User.ID != userA.ID {
		t.Fatalf("expected user A ID %s, got %v", userA.ID, resA.User)
	}

	// 4. Disable User A
	_, err = st.UpdateUserStatus(ctx, userA.ID, domain.UserStatusDisabled)
	if err != nil {
		t.Fatalf("disable user A failed: %v", err)
	}

	// 5. Create User B with same email -> must succeed with new ID
	userB, err := st.CreateUserDirect(ctx, targetEmail, "User B", passB, nil, nil)
	if err != nil {
		t.Fatalf("create user B with reused email failed: %v", err)
	}
	if userB.ID == userA.ID {
		t.Fatalf("user B received same ID as user A: %s", userB.ID)
	}

	// 6. Authenticate with reused email -> must authenticate User B
	resB, err := st.AuthenticateWithPassword(ctx, targetEmail, passB)
	if err != nil {
		t.Fatalf("authenticate user B failed: %v", err)
	}
	if resB.User == nil || resB.User.ID != userB.ID {
		t.Fatalf("expected user B ID %s, got %v", userB.ID, resB.User)
	}

	// Old password for User A must fail
	_, err = st.AuthenticateWithPassword(ctx, targetEmail, passA)
	if err == nil {
		t.Fatalf("expected authentication failure with old User A password")
	}
}
