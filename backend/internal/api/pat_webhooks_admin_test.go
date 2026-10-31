package api

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"database/sql"
	"database/sql/driver"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
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
// In-Memory Mock SQL Driver for Administrator Integration Testing
// =========================================================================

type mockDBState struct {
	mu          sync.Mutex
	users       map[string]*mockUser
	pats        map[string]*mockPAT
	webhooks    map[string]*mockWebhook
	deliveries  []*mockDelivery
	sysPerms    map[string][]string // userID -> permissions
	settings    *domain.SystemSettings
	statuses    []domain.Status
}

type mockUser struct {
	ID          string
	Email       string
	DisplayName string
	AvatarURL   string
	Status      string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

type mockPAT struct {
	ID          string
	UserID      string
	Name        string
	TokenHash   string
	TokenPrefix string
	Scopes      []string
	ExpiresAt   *time.Time
	LastUsedAt  *time.Time
	RevokedAt   *time.Time
	CreatedAt   time.Time
}

type mockWebhook struct {
	ID        string
	ProjectID *string
	Name      string
	URL       string
	Secret    string
	Events    []string
	IsActive  bool
	CreatedBy string
	CreatedAt time.Time
	UpdatedAt time.Time
}

type mockDelivery struct {
	ID           string
	WebhookID    string
	Event        string
	StatusCode   *int
	DurationMs   int
	RequestBody  string
	ResponseBody *string
	ErrorMessage *string
	CreatedAt    time.Time
}

var globalMockState = &mockDBState{
	users:      make(map[string]*mockUser),
	pats:       make(map[string]*mockPAT),
	webhooks:   make(map[string]*mockWebhook),
	deliveries: make([]*mockDelivery, 0),
	sysPerms:   make(map[string][]string),
	settings: &domain.SystemSettings{
		ID:                 "sys-setting-1",
		DefaultHoursPerDay: 8.0,
		UpdatedAt:          time.Now().UTC(),
	},
	statuses: []domain.Status{
		{ID: "st-1", Name: "未着手", DisplayOrder: 1, Color: "#3b82f6"},
		{ID: "st-2", Name: "進行中", DisplayOrder: 2, Color: "#f59e0b"},
		{ID: "st-3", Name: "レビュー", DisplayOrder: 3, Color: "#8b5cf6"},
		{ID: "st-4", Name: "完了", DisplayOrder: 4, Color: "#10b981"},
	},
}

func init() {
	sql.Register("polaris_mock", &mockDriver{})
}

type mockDriver struct{}

func (d *mockDriver) Open(name string) (driver.Conn, error) {
	return &mockConn{state: globalMockState}, nil
}

type mockConn struct {
	state *mockDBState
}

func (c *mockConn) Prepare(query string) (driver.Stmt, error) {
	return &mockStmt{conn: c, query: query}, nil
}

func (c *mockConn) Close() error { return nil }
func (c *mockConn) Begin() (driver.Tx, error) {
	return &mockTx{}, nil
}

type mockTx struct{}

func (t *mockTx) Commit() error   { return nil }
func (t *mockTx) Rollback() error { return nil }

type mockStmt struct {
	conn  *mockConn
	query string
}

func (s *mockStmt) Close() error  { return nil }
func (s *mockStmt) NumInput() int { return -1 }

func (s *mockStmt) Exec(args []driver.Value) (driver.Result, error) {
	q := strings.TrimSpace(s.query)
	st := s.conn.state
	st.mu.Lock()
	defer st.mu.Unlock()

	switch {
	case strings.Contains(q, "INSERT INTO personal_access_tokens"):
		// id, user_id, name, token_hash, token_prefix, scopes, expires_at, created_at
		patID := fmt.Sprintf("%v", args[0])
		userID := fmt.Sprintf("%v", args[1])
		name := fmt.Sprintf("%v", args[2])
		tokenHash := fmt.Sprintf("%v", args[3])
		tokenPrefix := fmt.Sprintf("%v", args[4])
		var scopes []string
		switch b := args[5].(type) {
		case []byte:
			_ = json.Unmarshal(b, &scopes)
		case string:
			_ = json.Unmarshal([]byte(b), &scopes)
		default:
			_ = json.Unmarshal([]byte(fmt.Sprintf("%v", args[5])), &scopes)
		}
		var exp *time.Time
		if args[6] != nil {
			tVal, ok := args[6].(time.Time)
			if ok {
				exp = &tVal
			}
		}
		cAt, _ := args[7].(time.Time)

		st.pats[patID] = &mockPAT{
			ID:          patID,
			UserID:      userID,
			Name:        name,
			TokenHash:   tokenHash,
			TokenPrefix: tokenPrefix,
			Scopes:      scopes,
			ExpiresAt:   exp,
			CreatedAt:   cAt,
		}
		return driver.RowsAffected(1), nil

	case strings.Contains(q, "UPDATE personal_access_tokens") && strings.Contains(q, "SET revoked_at"):
		patID := fmt.Sprintf("%v", args[0])
		userID := fmt.Sprintf("%v", args[1])
		pat, exists := st.pats[patID]
		if !exists || pat.UserID != userID || pat.RevokedAt != nil {
			return driver.RowsAffected(0), nil
		}
		now := time.Now().UTC()
		pat.RevokedAt = &now
		return driver.RowsAffected(1), nil

	case strings.Contains(q, "UPDATE personal_access_tokens SET last_used_at = NOW()"):
		patID := fmt.Sprintf("%v", args[0])
		if p, ok := st.pats[patID]; ok {
			now := time.Now().UTC()
			p.LastUsedAt = &now
		}
		return driver.RowsAffected(1), nil

	case strings.Contains(q, "INSERT INTO webhooks"):
		// id, project_id, name, url, secret, events, is_active, created_by, created_at, updated_at
		whID := fmt.Sprintf("%v", args[0])
		var pid *string
		if args[1] != nil {
			sVal := fmt.Sprintf("%v", args[1])
			pid = &sVal
		}
		name := fmt.Sprintf("%v", args[2])
		url := fmt.Sprintf("%v", args[3])
		secret := fmt.Sprintf("%v", args[4])
		var events []string
		_ = json.Unmarshal([]byte(fmt.Sprintf("%v", args[5])), &events)
		cBy := fmt.Sprintf("%v", args[6])
		cAt, _ := args[7].(time.Time)

		st.webhooks[whID] = &mockWebhook{
			ID:        whID,
			ProjectID: pid,
			Name:      name,
			URL:       url,
			Secret:    secret,
			Events:    events,
			IsActive:  true,
			CreatedBy: cBy,
			CreatedAt: cAt,
			UpdatedAt: cAt,
		}
		return driver.RowsAffected(1), nil

	case strings.Contains(q, "DELETE FROM webhooks WHERE id = $1"):
		whID := fmt.Sprintf("%v", args[0])
		if _, exists := st.webhooks[whID]; exists {
			delete(st.webhooks, whID)
			return driver.RowsAffected(1), nil
		}
		return driver.RowsAffected(0), nil

	case strings.Contains(q, "INSERT INTO webhook_deliveries"):
		// id, webhook_id, event, status_code, duration_ms, request_body, response_body, error_message, created_at
		delID := fmt.Sprintf("%v", args[0])
		whID := fmt.Sprintf("%v", args[1])
		event := fmt.Sprintf("%v", args[2])
		var sc *int
		if args[3] != nil {
			if v, ok := args[3].(int64); ok {
				iv := int(v)
				sc = &iv
			} else if v, ok := args[3].(int); ok {
				sc = &v
			}
		}
		var dur int
		if v, ok := args[4].(int64); ok {
			dur = int(v)
		} else if v, ok := args[4].(int); ok {
			dur = v
		}
		reqBody := fmt.Sprintf("%v", args[5])
		var respBody *string
		if args[6] != nil {
			sVal := fmt.Sprintf("%v", args[6])
			respBody = &sVal
		}
		var errMsg *string
		if args[7] != nil {
			sVal := fmt.Sprintf("%v", args[7])
			errMsg = &sVal
		}
		cAt, _ := args[8].(time.Time)

		del := &mockDelivery{
			ID:           delID,
			WebhookID:    whID,
			Event:        event,
			StatusCode:   sc,
			DurationMs:   dur,
			RequestBody:  reqBody,
			ResponseBody: respBody,
			ErrorMessage: errMsg,
			CreatedAt:    cAt,
		}
		st.deliveries = append(st.deliveries, del)
		return driver.RowsAffected(1), nil
	}

	return driver.RowsAffected(1), nil
}

func (s *mockStmt) Query(args []driver.Value) (driver.Rows, error) {
	q := strings.TrimSpace(s.query)
	st := s.conn.state
	st.mu.Lock()
	defer st.mu.Unlock()

	switch {
	case strings.Contains(q, "FROM personal_access_tokens pat") && strings.Contains(q, "WHERE pat.token_hash = $1"):
		// VerifyPAT query
		hash := fmt.Sprintf("%v", args[0])
		for _, pat := range st.pats {
			if pat.TokenHash == hash {
				u := st.users[pat.UserID]
				if u == nil {
					continue
				}
				scopesJSON, _ := json.Marshal(pat.Scopes)
				cols := []string{"id", "scopes", "expires_at", "revoked_at", "u_id", "email", "display_name", "avatar_url", "status", "created_at", "updated_at"}
				values := [][]driver.Value{{
					pat.ID,
					scopesJSON,
					pat.ExpiresAt,
					pat.RevokedAt,
					u.ID,
					u.Email,
					u.DisplayName,
					u.AvatarURL,
					u.Status,
					u.CreatedAt,
					u.UpdatedAt,
				}}
				return &mockRows{cols: cols, rows: values}, nil
			}
		}
		return &mockRows{cols: []string{"id"}, rows: nil}, nil

	case strings.Contains(q, "SELECT id, user_id, name, token_prefix, scopes") && strings.Contains(q, "FROM personal_access_tokens"):
		// ListPATs
		userID := fmt.Sprintf("%v", args[0])
		cols := []string{"id", "user_id", "name", "token_prefix", "scopes", "expires_at", "last_used_at", "revoked_at", "created_at"}
		var values [][]driver.Value
		for _, pat := range st.pats {
			if pat.UserID == userID {
				scopesJSON, _ := json.Marshal(pat.Scopes)
				values = append(values, []driver.Value{
					pat.ID, pat.UserID, pat.Name, pat.TokenPrefix, scopesJSON, pat.ExpiresAt, pat.LastUsedAt, pat.RevokedAt, pat.CreatedAt,
				})
			}
		}
		return &mockRows{cols: cols, rows: values}, nil

	case strings.Contains(q, "SELECT DISTINCT sp.permission"):
		// GetUserSystemPermissions
		userID := fmt.Sprintf("%v", args[0])
		perms := st.sysPerms[userID]
		cols := []string{"permission"}
		var values [][]driver.Value
		for _, p := range perms {
			values = append(values, []driver.Value{p})
		}
		return &mockRows{cols: cols, rows: values}, nil

	case strings.Contains(q, "SELECT id, subject_type, subject_id, permission, created_at") && strings.Contains(q, "FROM system_permissions"):
		// ListSystemPermissions
		cols := []string{"id", "subject_type", "subject_id", "permission", "created_at"}
		var values [][]driver.Value
		for uid, perms := range st.sysPerms {
			for _, p := range perms {
				values = append(values, []driver.Value{
					"perm-" + uid + "-" + p, "user", uid, p, time.Now().UTC(),
				})
			}
		}
		return &mockRows{cols: cols, rows: values}, nil

	case strings.Contains(q, "FROM webhooks") && strings.Contains(q, "WHERE id = $1"):
		// GetWebhook
		whID := fmt.Sprintf("%v", args[0])
		wh, exists := st.webhooks[whID]
		cols := []string{"id", "project_id", "name", "url", "secret", "events", "is_active", "created_by", "created_at", "updated_at"}
		if !exists {
			return &mockRows{cols: cols, rows: nil}, nil
		}
		evJSON, _ := json.Marshal(wh.Events)
		var pidVal any = nil
		if wh.ProjectID != nil {
			pidVal = *wh.ProjectID
		}
		values := [][]driver.Value{{
			wh.ID, pidVal, wh.Name, wh.URL, wh.Secret, evJSON, wh.IsActive, wh.CreatedBy, wh.CreatedAt, wh.UpdatedAt,
		}}
		return &mockRows{cols: cols, rows: values}, nil

	case strings.Contains(q, "FROM webhooks") && strings.Contains(q, "WHERE project_id IS NULL"):
		// ListWebhooks (System-wide)
		cols := []string{"id", "project_id", "name", "url", "secret", "events", "is_active", "created_by", "created_at", "updated_at"}
		var values [][]driver.Value
		for _, wh := range st.webhooks {
			if wh.ProjectID == nil {
				evJSON, _ := json.Marshal(wh.Events)
				values = append(values, []driver.Value{
					wh.ID, nil, wh.Name, wh.URL, wh.Secret, evJSON, wh.IsActive, wh.CreatedBy, wh.CreatedAt, wh.UpdatedAt,
				})
			}
		}
		return &mockRows{cols: cols, rows: values}, nil

	case strings.Contains(q, "FROM webhook_deliveries") && strings.Contains(q, "WHERE webhook_id = $1"):
		// ListWebhookDeliveries
		whID := fmt.Sprintf("%v", args[0])
		cols := []string{"id", "webhook_id", "event", "status_code", "duration_ms", "request_body", "response_body", "error_message", "created_at"}
		var values [][]driver.Value
		for _, d := range st.deliveries {
			if d.WebhookID == whID {
				values = append(values, []driver.Value{
					d.ID, d.WebhookID, d.Event, d.StatusCode, d.DurationMs, []byte(d.RequestBody), d.ResponseBody, d.ErrorMessage, d.CreatedAt,
				})
			}
		}
		return &mockRows{cols: cols, rows: values}, nil

	case strings.Contains(q, "FROM system_settings"):
		// GetSystemSettings
		cols := []string{"id", "default_hours_per_day", "default_issue_template_id", "default_task_template_id", "updated_at"}
		values := [][]driver.Value{{
			st.settings.ID, st.settings.DefaultHoursPerDay, nil, nil, st.settings.UpdatedAt,
		}}
		return &mockRows{cols: cols, rows: values}, nil

	case strings.Contains(q, "FROM system_allowed_domains"):
		cols := []string{"domain"}
		return &mockRows{cols: cols, rows: nil}, nil

	case strings.Contains(q, "FROM statuses") && strings.Contains(q, "WHERE project_id IS NULL"):
		// ListStatuses: SELECT id, project_id, display_order, name, color, created_at FROM statuses
		cols := []string{"id", "project_id", "display_order", "name", "color", "created_at"}
		var values [][]driver.Value
		now := time.Now().UTC()
		for _, s := range st.statuses {
			values = append(values, []driver.Value{
				s.ID, nil, s.DisplayOrder, s.Name, s.Color, now,
			})
		}
		return &mockRows{cols: cols, rows: values}, nil
	}

	return &mockRows{cols: []string{"id"}, rows: nil}, nil
}

type mockRows struct {
	cols []string
	rows [][]driver.Value
	idx  int
}

func (r *mockRows) Columns() []string { return r.cols }
func (r *mockRows) Close() error      { return nil }
func (r *mockRows) Next(dest []driver.Value) error {
	if r.idx >= len(r.rows) {
		return io.EOF
	}
	for i, v := range r.rows[r.idx] {
		dest[i] = v
	}
	r.idx++
	return nil
}

// =========================================================================
// TEST SUITE: System Administrator Hands-on Verification
// =========================================================================

func setupAdminTestServer(t *testing.T) (*Server, *store.Store, *mockDBState, string, string) {
	db, err := sql.Open("polaris_mock", "")
	if err != nil {
		t.Fatalf("failed to open mock db: %v", err)
	}

	mailer := mail.NewSMTPSender()
	fcmClient := fcm.NewClient(context.Background(), "")
	st := store.New(db, "0123456789abcdef0123456789abcdef", "http://localhost:3000", mailer, fcmClient)
	srv := NewServer(st, "http://localhost:3000", GoogleOAuthConfig{})

	// Setup Admin User: 佐藤管理者 (Sato Admin)
	adminID := "usr_admin_sato"
	globalMockState.mu.Lock()
	globalMockState.users[adminID] = &mockUser{
		ID:          adminID,
		Email:       "sato-admin@example.com",
		DisplayName: "佐藤管理者",
		Status:      domain.UserStatusActive,
		CreatedAt:   time.Now().UTC(),
		UpdatedAt:   time.Now().UTC(),
	}
	// Grant PermManageSystem to 佐藤管理者
	globalMockState.sysPerms[adminID] = []string{domain.PermManageSystem}

	// Setup Regular User: 田中一般 (Tanaka User)
	regularID := "usr_tanaka_regular"
	globalMockState.users[regularID] = &mockUser{
		ID:          regularID,
		Email:       "tanaka-dev@example.com",
		DisplayName: "田中一般",
		Status:      domain.UserStatusActive,
		CreatedAt:   time.Now().UTC(),
		UpdatedAt:   time.Now().UTC(),
	}
	globalMockState.sysPerms[regularID] = []string{} // No system permissions
	globalMockState.mu.Unlock()

	return srv, st, globalMockState, adminID, regularID
}

// TestPATLifecycleAndAdminAPIAccess verifies:
// 1. Issue a PAT with `admin:system` scope
// 2. Use `Authorization: Bearer plr_pat_...` to invoke administrative APIs (GET /api/v1/system/settings, GET /api/v1/system/statuses)
// 3. Test that expired or revoked PATs return 401 Unauthorized
// 4. Test that token hashing ensures plain tokens are never exposed in DB or subsequent GET requests
func TestPATLifecycleAndAdminAPIAccess(t *testing.T) {
	srv, st, state, adminID, _ := setupAdminTestServer(t)
	ctx := context.Background()

	// -------------------------------------------------------------
	// 1. Issue a PAT with `admin:system` scope for 佐藤管理者
	// -------------------------------------------------------------
	expiresAt := time.Now().UTC().Add(30 * 24 * time.Hour)
	plainToken, pat, err := st.CreatePAT(ctx, adminID, "Sato Admin Automation PAT", []string{domain.ScopeAdminSystem}, &expiresAt)
	if err != nil {
		t.Fatalf("failed to create PAT: %v", err)
	}

	if !strings.HasPrefix(plainToken, "plr_pat_") {
		t.Fatalf("expected plain token to start with 'plr_pat_', got: %s", plainToken)
	}
	if pat.TokenPrefix != plainToken[:14] {
		t.Fatalf("expected token_prefix to match first 14 chars, got %s vs %s", pat.TokenPrefix, plainToken[:14])
	}

	// -------------------------------------------------------------
	// 4. Token Hashing Verification: Plain token is NEVER exposed in DB or subsequent GET requests
	// -------------------------------------------------------------
	state.mu.Lock()
	dbRecord, exists := state.pats[pat.ID]
	state.mu.Unlock()
	if !exists {
		t.Fatalf("pat record not found in mock DB")
	}

	// Verify plaintext token is NOT stored in DB
	if dbRecord.TokenHash == plainToken {
		t.Fatalf("SECURITY VIOLATION: plain token was stored directly in database instead of hash!")
	}
	expectedHash := crypto.HashToken(plainToken)
	if dbRecord.TokenHash != expectedHash {
		t.Fatalf("expected SHA-256 token hash %s, got %s", expectedHash, dbRecord.TokenHash)
	}

	// List PATs via store (equivalent to GET /api/v1/user/tokens)
	pats, err := st.ListPATs(ctx, adminID)
	if err != nil {
		t.Fatalf("failed to list PATs: %v", err)
	}
	if len(pats) == 0 {
		t.Fatalf("expected at least 1 PAT")
	}
	found := false
	for _, p := range pats {
		if p.ID == pat.ID {
			found = true
			if strings.Contains(p.TokenPrefix, plainToken) && len(p.TokenPrefix) == len(plainToken) {
				t.Fatalf("SECURITY VIOLATION: Full plain token exposed in ListPATs response!")
			}
			if len(p.TokenPrefix) > 16 {
				t.Fatalf("TokenPrefix length %d exceeds prefix boundary: %s", len(p.TokenPrefix), p.TokenPrefix)
			}
		}
	}
	if !found {
		t.Fatalf("created PAT not found in ListPATs")
	}

	// -------------------------------------------------------------
	// 2. Use `Authorization: Bearer plr_pat_...` to invoke Administrative APIs
	// -------------------------------------------------------------
	// (a) GET /api/v1/system/settings
	req := httptest.NewRequest(http.MethodGet, "/api/v1/system/settings", nil)
	req.Header.Set("Authorization", "Bearer "+plainToken)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("GET /api/v1/system/settings with admin PAT failed (HTTP %d): %s", rec.Code, rec.Body.String())
	}
	var settings domain.SystemSettings
	if err := json.NewDecoder(rec.Body).Decode(&settings); err != nil {
		t.Fatalf("failed to decode system settings: %v", err)
	}
	if settings.DefaultHoursPerDay != 8.0 {
		t.Fatalf("expected default hours 8.0, got %f", settings.DefaultHoursPerDay)
	}

	// (b) GET /api/v1/system/statuses
	req = httptest.NewRequest(http.MethodGet, "/api/v1/system/statuses", nil)
	req.Header.Set("Authorization", "Bearer "+plainToken)
	rec = httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("GET /api/v1/system/statuses with admin PAT failed (HTTP %d): %s", rec.Code, rec.Body.String())
	}
	var statuses []domain.Status
	if err := json.NewDecoder(rec.Body).Decode(&statuses); err != nil {
		t.Fatalf("failed to decode system statuses: %v", err)
	}
	if len(statuses) != 4 {
		t.Fatalf("expected 4 system statuses, got %d", len(statuses))
	}

	// (c) GET /api/v1/system/permissions (requires requireSystemPerm: domain.PermManageSystem)
	req = httptest.NewRequest(http.MethodGet, "/api/v1/system/permissions", nil)
	req.Header.Set("Authorization", "Bearer "+plainToken)
	rec = httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("GET /api/v1/system/permissions with admin PAT failed (HTTP %d): %s", rec.Code, rec.Body.String())
	}

	// -------------------------------------------------------------
	// 3. Test that Expired or Revoked PATs return 401 Unauthorized
	// -------------------------------------------------------------

	// (a) Test Revoked PAT
	err = st.RevokePAT(ctx, adminID, pat.ID)
	if err != nil {
		t.Fatalf("failed to revoke PAT: %v", err)
	}

	req = httptest.NewRequest(http.MethodGet, "/api/v1/system/settings", nil)
	req.Header.Set("Authorization", "Bearer "+plainToken)
	rec = httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 Unauthorized for revoked PAT, got %d: %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "token has been revoked") {
		t.Fatalf("expected 'token has been revoked' in response error, got: %s", rec.Body.String())
	}

	// (b) Test Expired PAT
	past := time.Now().UTC().Add(-24 * time.Hour)
	expiredPlainToken, _, err := st.CreatePAT(ctx, adminID, "Expired Admin PAT", []string{domain.ScopeAdminSystem}, &past)
	if err != nil {
		t.Fatalf("failed to create expired PAT: %v", err)
	}

	req = httptest.NewRequest(http.MethodGet, "/api/v1/system/settings", nil)
	req.Header.Set("Authorization", "Bearer "+expiredPlainToken)
	rec = httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 Unauthorized for expired PAT, got %d: %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "token has expired") {
		t.Fatalf("expected 'token has expired' in response error, got: %s", rec.Body.String())
	}

	// (c) Test Non-existent / Random PAT
	dummyToken := "plr_pat_0123456789abcdef0123456789abcdef0123456789abcdef"
	req = httptest.NewRequest(http.MethodGet, "/api/v1/system/settings", nil)
	req.Header.Set("Authorization", "Bearer "+dummyToken)
	rec = httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 Unauthorized for non-existent PAT, got %d", rec.Code)
	}
}

// TestSystemWideOutboundWebhooks verifies:
// 1. Register a system-wide webhook via POST /api/v1/system/webhooks
// 2. Execute Ping test via POST /api/v1/system/webhooks/{webhookId}/test
// 3. Verify that X-Polaris-Signature is computed with HMAC-SHA256
// 4. Check webhook deliveries log via GET /api/v1/system/webhooks/{webhookId}/deliveries
// 5. Test deleting a system webhook
func TestSystemWideOutboundWebhooks(t *testing.T) {
	srv, st, _, adminID, _ := setupAdminTestServer(t)
	ctx := context.Background()

	// Issue admin PAT for authentication
	adminToken, _, err := st.CreatePAT(ctx, adminID, "Admin Webhook PAT", []string{domain.ScopeAdminSystem}, nil)
	if err != nil {
		t.Fatalf("create admin PAT: %v", err)
	}

	// -------------------------------------------------------------
	// Setup Mock Receiver HTTP Server to catch webhook Ping & verify HMAC
	// -------------------------------------------------------------
	var receivedHeaders http.Header
	var receivedBody []byte
	var deliveryCount int
	var mu sync.Mutex

	customSecret := "whsec_supersecret_testkey_12345"

	receiverSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		deliveryCount++
		receivedHeaders = r.Header.Clone()
		b, _ := io.ReadAll(r.Body)
		receivedBody = b

		// Respond with 200 OK and json body
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"received": true, "status": "ok"}`))
	}))
	defer receiverSrv.Close()

	// -------------------------------------------------------------
	// 1. Register a system-wide webhook via POST /api/v1/system/webhooks
	// -------------------------------------------------------------
	createBody, _ := json.Marshal(map[string]any{
		"name":   "監査ログ SIEM 連携 Webhook",
		"url":    receiverSrv.URL,
		"secret": customSecret,
		"events": []string{"task.created", "issue.created", "task.status_changed"},
	})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/system/webhooks", bytes.NewReader(createBody))
	req.Header.Set("Authorization", "Bearer "+adminToken)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("POST /api/v1/system/webhooks failed (HTTP %d): %s", rec.Code, rec.Body.String())
	}

	var createdWebhook domain.Webhook
	if err := json.NewDecoder(rec.Body).Decode(&createdWebhook); err != nil {
		t.Fatalf("failed to decode created webhook: %v", err)
	}
	if createdWebhook.ID == "" {
		t.Fatalf("expected non-empty webhook ID")
	}
	if createdWebhook.ProjectID != nil {
		t.Fatalf("expected system webhook to have nil ProjectID, got %v", *createdWebhook.ProjectID)
	}
	if createdWebhook.Name != "監査ログ SIEM 連携 Webhook" {
		t.Fatalf("unexpected webhook name: %s", createdWebhook.Name)
	}
	if createdWebhook.Secret != customSecret {
		t.Fatalf("unexpected secret: %s", createdWebhook.Secret)
	}

	// Verify GET /api/v1/system/webhooks returns it
	req = httptest.NewRequest(http.MethodGet, "/api/v1/system/webhooks", nil)
	req.Header.Set("Authorization", "Bearer "+adminToken)
	rec = httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("GET /api/v1/system/webhooks failed (%d): %s", rec.Code, rec.Body.String())
	}
	var list []domain.Webhook
	_ = json.NewDecoder(rec.Body).Decode(&list)
	if len(list) != 1 || list[0].ID != createdWebhook.ID {
		t.Fatalf("expected 1 system webhook in list, got %v", list)
	}

	// -------------------------------------------------------------
	// 2. Execute Ping test via POST /api/v1/system/webhooks/{webhookId}/test
	// -------------------------------------------------------------
	req = httptest.NewRequest(http.MethodPost, "/api/v1/system/webhooks/"+createdWebhook.ID+"/test", nil)
	req.Header.Set("Authorization", "Bearer "+adminToken)
	rec = httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("POST /api/v1/system/webhooks/{webhookId}/test failed (HTTP %d): %s", rec.Code, rec.Body.String())
	}

	var testDelivery domain.WebhookDelivery
	if err := json.NewDecoder(rec.Body).Decode(&testDelivery); err != nil {
		t.Fatalf("failed to decode test delivery: %v", err)
	}
	if testDelivery.StatusCode == nil || *testDelivery.StatusCode != 200 {
		t.Fatalf("expected delivery status_code 200, got %v", testDelivery.StatusCode)
	}
	if testDelivery.Event != "ping" {
		t.Fatalf("expected delivery event 'ping', got %s", testDelivery.Event)
	}

	// -------------------------------------------------------------
	// 3. Verify that X-Polaris-Signature is computed with HMAC-SHA256
	// -------------------------------------------------------------
	mu.Lock()
	count := deliveryCount
	sigHeader := receivedHeaders.Get("X-Polaris-Signature")
	eventHeader := receivedHeaders.Get("X-Polaris-Event")
	deliveryIDHeader := receivedHeaders.Get("X-Polaris-Delivery")
	bodyBytes := receivedBody
	mu.Unlock()

	if count != 1 {
		t.Fatalf("expected exactly 1 delivery to receiver server, got %d", count)
	}
	if eventHeader != "ping" {
		t.Fatalf("expected X-Polaris-Event header 'ping', got '%s'", eventHeader)
	}
	if deliveryIDHeader == "" {
		t.Fatalf("expected non-empty X-Polaris-Delivery header")
	}
	if !strings.HasPrefix(sigHeader, "sha256=") {
		t.Fatalf("expected X-Polaris-Signature to start with 'sha256=', got: %s", sigHeader)
	}

	// Recompute expected HMAC-SHA256 signature using the secret and body bytes
	mac := hmac.New(sha256.New, []byte(customSecret))
	mac.Write(bodyBytes)
	expectedSig := "sha256=" + hex.EncodeToString(mac.Sum(nil))

	if sigHeader != expectedSig {
		t.Fatalf("HMAC-SHA256 MISMATCH! expected: %s, got: %s", expectedSig, sigHeader)
	}
	t.Logf("✓ HMAC-SHA256 Signature verified successfully: %s", sigHeader)

	// -------------------------------------------------------------
	// 4. Check webhook deliveries log via GET /api/v1/system/webhooks/{webhookId}/deliveries
	// -------------------------------------------------------------
	req = httptest.NewRequest(http.MethodGet, "/api/v1/system/webhooks/"+createdWebhook.ID+"/deliveries", nil)
	req.Header.Set("Authorization", "Bearer "+adminToken)
	rec = httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("GET deliveries failed (HTTP %d): %s", rec.Code, rec.Body.String())
	}
	var deliveries []domain.WebhookDelivery
	if err := json.NewDecoder(rec.Body).Decode(&deliveries); err != nil {
		t.Fatalf("decode deliveries: %v", err)
	}
	if len(deliveries) != 1 {
		t.Fatalf("expected 1 delivery record, got %d", len(deliveries))
	}
	if deliveries[0].WebhookID != createdWebhook.ID {
		t.Fatalf("expected webhook_id %s, got %s", createdWebhook.ID, deliveries[0].WebhookID)
	}
	if deliveries[0].ResponseBody == nil || !strings.Contains(*deliveries[0].ResponseBody, `"status": "ok"`) {
		t.Fatalf("expected response body recorded in delivery log, got %v", deliveries[0].ResponseBody)
	}

	// -------------------------------------------------------------
	// 5. Test deleting a system webhook
	// -------------------------------------------------------------
	req = httptest.NewRequest(http.MethodDelete, "/api/v1/system/webhooks/"+createdWebhook.ID, nil)
	req.Header.Set("Authorization", "Bearer "+adminToken)
	rec = httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("DELETE /api/v1/system/webhooks/{id} failed (%d): %s", rec.Code, rec.Body.String())
	}

	// Verify it's gone: GET /api/v1/system/webhooks/{id} returns 404
	req = httptest.NewRequest(http.MethodGet, "/api/v1/system/webhooks/"+createdWebhook.ID, nil)
	req.Header.Set("Authorization", "Bearer "+adminToken)
	rec = httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404 Not Found after deletion, got %d: %s", rec.Code, rec.Body.String())
	}
}

// TestSecurityEdgeCases explores vulnerability boundaries:
// 1. PAT Scopes Bypass: Can a PAT without `admin:system` scope still invoke system APIs?
// 2. Cookie vs Header Conflict: Does a stale cookie override an explicit Authorization: Bearer header?
// 3. Cross-Project Webhook IDOR: Can a project admin delete a system-wide webhook via project route?
func TestSecurityEdgeCases(t *testing.T) {
	srv, st, _, adminID, regularID := setupAdminTestServer(t)
	ctx := context.Background()

	// -------------------------------------------------------------
	// Edge Case 1: PAT Scope Enforcement Check
	// -------------------------------------------------------------
	t.Run("PAT Scope Enforcement", func(t *testing.T) {
		// Admin 佐藤管理者 creates a restricted token with ONLY `read:issues` scope (NO `admin:system` scope)
		restrictedToken, _, err := st.CreatePAT(ctx, adminID, "Restricted Issue-Only Token", []string{domain.ScopeReadIssues}, nil)
		if err != nil {
			t.Fatalf("create restricted PAT: %v", err)
		}

		// Attempt to access system administrative settings with this restricted token
		req := httptest.NewRequest(http.MethodGet, "/api/v1/system/settings", nil)
		req.Header.Set("Authorization", "Bearer "+restrictedToken)
		rec := httptest.NewRecorder()
		srv.ServeHTTP(rec, req)

		// Expected behavior of least-privilege scoping:
		// A token lacking `admin:system` should be rejected with 403 Forbidden.
		// If it succeeds with 200 OK, this documents the Architectural Finding: Token Scopes are Not Enforced.
		if rec.Code == http.StatusOK {
			t.Logf("⚠️ FINDING DETECTED: Token with scope ['read:issues'] was permitted to access /api/v1/system/settings! Scopes are ignored in middleware.")
		} else if rec.Code == http.StatusForbidden {
			t.Logf("✓ Token scopes are strictly enforced (HTTP 403 Forbidden)")
		}
	})

	// -------------------------------------------------------------
	// Edge Case 2: Regular User attempting to use `admin:system` scope
	// -------------------------------------------------------------
	t.Run("Non-Admin User with admin:system PAT", func(t *testing.T) {
		// 田中一般 (who has NO system permission) issues a token requesting `admin:system`
		rogueToken, _, err := st.CreatePAT(ctx, regularID, "Rogue Admin Token", []string{domain.ScopeAdminSystem}, nil)
		if err != nil {
			t.Fatalf("create rogue PAT: %v", err)
		}

		req := httptest.NewRequest(http.MethodGet, "/api/v1/system/settings", nil)
		req.Header.Set("Authorization", "Bearer "+rogueToken)
		rec := httptest.NewRecorder()
		srv.ServeHTTP(rec, req)

		// Since GET /api/v1/system/settings only uses requireAuth, any authenticated user can read settings.
		// But let's check a protected route: GET /api/v1/system/permissions (which requires domain.PermManageSystem)
		req = httptest.NewRequest(http.MethodGet, "/api/v1/system/permissions", nil)
		req.Header.Set("Authorization", "Bearer "+rogueToken)
		rec = httptest.NewRecorder()
		srv.ServeHTTP(rec, req)

		if rec.Code != http.StatusForbidden {
			t.Fatalf("expected 403 Forbidden for non-admin user even if PAT requests admin:system, got %d", rec.Code)
		}
		t.Logf("✓ Non-admin user cannot escalate permissions simply by claiming admin:system scope: HTTP %d", rec.Code)
	})

	// -------------------------------------------------------------
	// Edge Case 3: Cookie Precedence over Bearer Header
	// -------------------------------------------------------------
	t.Run("Cookie Precedence over Authorization Header", func(t *testing.T) {
		// Create admin PAT
		adminToken, _, err := st.CreatePAT(ctx, adminID, "Admin PAT", []string{domain.ScopeAdminSystem}, nil)
		if err != nil {
			t.Fatalf("create admin PAT: %v", err)
		}

		// Simulate client request containing a cookie value AND an Authorization: Bearer header
		req := httptest.NewRequest(http.MethodGet, "/api/v1/system/permissions", nil)
		req.AddCookie(&http.Cookie{
			Name:  SessionCookieName,
			Value: "some_stale_or_invalid_session_cookie",
		})
		req.Header.Set("Authorization", "Bearer "+adminToken)
		rec := httptest.NewRecorder()
		srv.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("Authorization: Bearer header must take precedence over cookie, expected 200, got: %d", rec.Code)
		}
	})

	// -------------------------------------------------------------
	// Edge Case 4: Webhook IDOR / Cross-scope access prevention
	// -------------------------------------------------------------
	t.Run("Cross-Scope Webhook Deletion via Project Route", func(t *testing.T) {
		adminToken, _, err := st.CreatePAT(ctx, adminID, "Admin Cross Scope PAT", []string{domain.ScopeAdminSystem}, nil)
		if err != nil {
			t.Fatalf("create admin PAT: %v", err)
		}
		// Create a system-wide webhook
		sysHook, err := st.CreateWebhook(ctx, nil, "System Audit Hook", "http://localhost:9999", "whsec_test", []string{"task.created"}, adminID)
		if err != nil {
			t.Fatalf("create system webhook: %v", err)
		}

		// A request to access or delete this system webhook via a project route must return 404 Not Found
		req := httptest.NewRequest(http.MethodGet, "/api/v1/projects/any_proj_id/webhooks/"+sysHook.ID, nil)
		req.Header.Set("Authorization", "Bearer "+adminToken)
		rec := httptest.NewRecorder()
		srv.ServeHTTP(rec, req)
		if rec.Code != http.StatusNotFound && rec.Code != http.StatusForbidden {
			t.Fatalf("expected 404 Not Found or 403 Forbidden when getting system webhook via project route, got %d", rec.Code)
		}

		req = httptest.NewRequest(http.MethodDelete, "/api/v1/projects/any_proj_id/webhooks/"+sysHook.ID, nil)
		req.Header.Set("Authorization", "Bearer "+adminToken)
		rec = httptest.NewRecorder()
		srv.ServeHTTP(rec, req)
		if rec.Code != http.StatusNotFound && rec.Code != http.StatusForbidden {
			t.Fatalf("expected 404 Not Found or 403 Forbidden when deleting system webhook via project route, got %d", rec.Code)
		}

		// Ensure system webhook was NOT deleted
		wh, err := st.GetWebhook(ctx, sysHook.ID)
		if err != nil {
			t.Fatalf("expected system webhook to remain intact, got %v", err)
		}
		if wh.ID != sysHook.ID {
			t.Fatalf("expected webhook ID %s, got %s", sysHook.ID, wh.ID)
		}

		// Now delete it via the legitimate system route
		req = httptest.NewRequest(http.MethodDelete, "/api/v1/system/webhooks/"+sysHook.ID, nil)
		req.Header.Set("Authorization", "Bearer "+adminToken)
		rec = httptest.NewRecorder()
		srv.ServeHTTP(rec, req)
		if rec.Code != http.StatusNoContent {
			t.Fatalf("expected 204 NoContent deleting via system route, got %d", rec.Code)
		}
	})

	// -------------------------------------------------------------
	// Edge Case 5: PAT Scope Enforcement
	// -------------------------------------------------------------
	t.Run("PAT Scope Enforcement", func(t *testing.T) {
		// Create PAT with only read:issues
		plainToken, _, err := st.CreatePAT(ctx, adminID, "Read Only Issues PAT", []string{"read:issues"}, nil)
		if err != nil {
			t.Fatalf("create read-only PAT: %v", err)
		}

		// Attempt to invoke system management API -> 403 Forbidden
		req := httptest.NewRequest(http.MethodGet, "/api/v1/system/permissions", nil)
		req.Header.Set("Authorization", "Bearer "+plainToken)
		rec := httptest.NewRecorder()
		srv.ServeHTTP(rec, req)
		if rec.Code != http.StatusForbidden {
			t.Fatalf("expected 403 Forbidden when calling admin API with read:issues PAT, got %d", rec.Code)
		}

		// Attempt to create a task (write:tasks) -> 403 Forbidden
		taskBody, _ := json.Marshal(map[string]any{"name": "scope-test-task", "issue_id": "iss_1"})
		req = httptest.NewRequest(http.MethodPost, "/api/v1/projects/any_project/tasks", bytes.NewReader(taskBody))
		req.Header.Set("Authorization", "Bearer "+plainToken)
		rec = httptest.NewRecorder()
		srv.ServeHTTP(rec, req)
		if rec.Code != http.StatusForbidden {
			t.Fatalf("expected 403 Forbidden when creating task with read:issues PAT, got %d", rec.Code)
		}
	})
}
