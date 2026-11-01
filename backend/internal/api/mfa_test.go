package api

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"database/sql/driver"
	"encoding/base64"
	"encoding/binary"
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
	"github.com/takayoshi-u/polaris/backend/internal/mfa"
	"github.com/takayoshi-u/polaris/backend/internal/pkg/crypto"
	"github.com/takayoshi-u/polaris/backend/internal/store"
)

func TestMFA_Unauthenticated(t *testing.T) {
	srv := NewServer(nil, "http://localhost:3000", GoogleOAuthConfig{})

	endpoints := []struct {
		method string
		path   string
	}{
		{http.MethodGet, "/api/v1/auth/mfa/status"},
		{http.MethodPost, "/api/v1/auth/mfa/totp/setup"},
		{http.MethodPost, "/api/v1/auth/mfa/totp/enable"},
		{http.MethodPost, "/api/v1/auth/mfa/totp/disable"},
		{http.MethodPost, "/api/v1/auth/mfa/backup-codes/regenerate"},
		{http.MethodPost, "/api/v1/auth/mfa/webauthn/register-options"},
		{http.MethodPost, "/api/v1/auth/mfa/webauthn/register-verify"},
		{http.MethodDelete, "/api/v1/auth/mfa/webauthn/keys/key-1"},
		{http.MethodPut, "/api/v1/system/users/usr-1/mfa-enforcement"},
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

// =========================================================================
// MFA Mock Database & State for API Verification
// =========================================================================

type mockWebAuthnCred struct {
	ID              string
	UserID          string
	Name            string
	CredentialID    []byte
	PublicKey       []byte
	AttestationType string
	AAGUID          []byte
	SignCount       int64
	CreatedAt       time.Time
	LastUsedAt      *time.Time
}

type mfaMockState struct {
	mu           sync.Mutex
	user         *domain.User
	passwordHash string
	totpSecret   string
	backupCodes  []domain.BackupCodeEntry
	credentials  []*mockWebAuthnCred
	challenges   map[string]*mockChallenge // tempTokenHash -> challenge
	sessions     map[string]string         // tokenHash -> userID
	settings     *domain.SystemSettings
	sysPerms     map[string][]string
}

type mockChallenge struct {
	id                 string
	userID             string
	tempTokenHash      string
	webauthnChallenge  string
	totpSetupSecret    string
	failedAttempts     int
	expiresAt          time.Time
}

var mfaState = &mfaMockState{
	challenges:  make(map[string]*mockChallenge),
	sessions:    make(map[string]string),
	sysPerms:    make(map[string][]string),
	credentials: make([]*mockWebAuthnCred, 0),
	settings: &domain.SystemSettings{
		ID:                 "settings-1",
		DefaultHoursPerDay: 8.0,
		RequireMFA:         false,
		UpdatedAt:          time.Now().UTC(),
	},
}

func init() {
	sql.Register("polaris_mfa_mock", &mfaMockDriver{})
}

type mfaMockDriver struct{}

func (d *mfaMockDriver) Open(name string) (driver.Conn, error) {
	return &mfaMockConn{state: mfaState}, nil
}

type mfaMockConn struct {
	state *mfaMockState
}

func (c *mfaMockConn) Prepare(query string) (driver.Stmt, error) {
	return &mfaMockStmt{conn: c, query: query}, nil
}

func (c *mfaMockConn) Close() error { return nil }
func (c *mfaMockConn) Begin() (driver.Tx, error) {
	return &mfaMockTx{}, nil
}

type mfaMockTx struct{}

func (tx *mfaMockTx) Commit() error   { return nil }
func (tx *mfaMockTx) Rollback() error { return nil }

type mfaMockStmt struct {
	conn  *mfaMockConn
	query string
}

func (s *mfaMockStmt) Close() error  { return nil }
func (s *mfaMockStmt) NumInput() int { return -1 }

func (s *mfaMockStmt) Exec(args []driver.Value) (driver.Result, error) {
	q := strings.TrimSpace(s.query)
	st := s.conn.state
	st.mu.Lock()
	defer st.mu.Unlock()

	switch {
	case strings.Contains(q, "INSERT INTO mfa_challenges"):
		id := fmt.Sprintf("%v", args[0])
		uid := fmt.Sprintf("%v", args[1])
		tokenHash := fmt.Sprintf("%v", args[2])
		mc := &mockChallenge{
			id:            id,
			userID:        uid,
			tempTokenHash: tokenHash,
			expiresAt:     time.Now().UTC().Add(5 * time.Minute),
		}
		if strings.Contains(q, "webauthn_challenge") {
			mc.webauthnChallenge = fmt.Sprintf("%v", args[3])
			if e, ok := args[4].(time.Time); ok {
				mc.expiresAt = e
			}
		} else {
			if e, ok := args[3].(time.Time); ok {
				mc.expiresAt = e
			}
		}
		st.challenges[tokenHash] = mc
		return driver.RowsAffected(1), nil

	case strings.Contains(q, "UPDATE mfa_challenges") && strings.Contains(q, "webauthn_challenge"):
		chall := fmt.Sprintf("%v", args[0])
		tHash := fmt.Sprintf("%v", args[1])
		if c, ok := st.challenges[tHash]; ok {
			c.webauthnChallenge = chall
			return driver.RowsAffected(1), nil
		}
		return driver.RowsAffected(0), nil

	case strings.Contains(q, "DELETE FROM mfa_challenges WHERE temp_token_hash = $1"):
		tHash := fmt.Sprintf("%v", args[0])
		delete(st.challenges, tHash)
		return driver.RowsAffected(1), nil

	case strings.Contains(q, "INSERT INTO user_sessions"):
		tokHash := fmt.Sprintf("%v", args[2])
		uid := fmt.Sprintf("%v", args[1])
		st.sessions[tokHash] = uid
		return driver.RowsAffected(1), nil

	case strings.Contains(q, "UPDATE users") && strings.Contains(q, "totp_secret = $1"):
		sec := fmt.Sprintf("%v", args[0])
		if b, ok := args[1].([]byte); ok {
			_ = json.Unmarshal(b, &st.backupCodes)
		} else {
			_ = json.Unmarshal([]byte(fmt.Sprintf("%v", args[1])), &st.backupCodes)
		}
		st.totpSecret = sec
		st.user.MFAEnabled = true
		return driver.RowsAffected(1), nil

	case strings.Contains(q, "UPDATE users") && strings.Contains(q, "totp_secret = NULL"):
		mfaEnabled, _ := args[0].(bool)
		st.totpSecret = ""
		st.user.MFAEnabled = mfaEnabled
		return driver.RowsAffected(1), nil

	case strings.Contains(q, "UPDATE users SET mfa_backup_codes = $1"):
		if b, ok := args[0].([]byte); ok {
			_ = json.Unmarshal(b, &st.backupCodes)
		} else {
			_ = json.Unmarshal([]byte(fmt.Sprintf("%v", args[0])), &st.backupCodes)
		}
		return driver.RowsAffected(1), nil

	case strings.Contains(q, "UPDATE users SET mfa_enforced = $1"):
		enforced, _ := args[0].(bool)
		st.user.MFAEnforced = enforced
		return driver.RowsAffected(1), nil

	case strings.Contains(q, "INSERT INTO webauthn_credentials"):
		// id, user_id, name, credential_id, public_key, attestation_type, aaguid, sign_count, created_at
		cID := fmt.Sprintf("%v", args[0])
		uID := fmt.Sprintf("%v", args[1])
		name := fmt.Sprintf("%v", args[2])
		credBytes, _ := args[3].([]byte)
		pubKey, _ := args[4].([]byte)
		attType := fmt.Sprintf("%v", args[5])
		aaguid, _ := args[6].([]byte)
		signCount, _ := args[7].(uint32)
		now := time.Now().UTC()

		cred := &mockWebAuthnCred{
			ID:              cID,
			UserID:          uID,
			Name:            name,
			CredentialID:    credBytes,
			PublicKey:       pubKey,
			AttestationType: attType,
			AAGUID:          aaguid,
			SignCount:       int64(signCount),
			CreatedAt:       now,
		}
		st.credentials = append(st.credentials, cred)
		st.user.MFAEnabled = true
		return driver.RowsAffected(1), nil

	case strings.Contains(q, "UPDATE webauthn_credentials") && strings.Contains(q, "SET sign_count = $1"):
		sc, _ := args[0].(uint32)
		cUUID := fmt.Sprintf("%v", args[2])
		for _, c := range st.credentials {
			if c.ID == cUUID {
				c.SignCount = int64(sc)
				now := time.Now().UTC()
				c.LastUsedAt = &now
			}
		}
		return driver.RowsAffected(1), nil

	case strings.Contains(q, "DELETE FROM webauthn_credentials WHERE id = $1"):
		cUUID := fmt.Sprintf("%v", args[0])
		var remaining []*mockWebAuthnCred
		found := false
		for _, c := range st.credentials {
			if c.ID == cUUID {
				found = true
			} else {
				remaining = append(remaining, c)
			}
		}
		st.credentials = remaining
		if len(st.credentials) == 0 && st.totpSecret == "" {
			st.user.MFAEnabled = false
		}
		if found {
			return driver.RowsAffected(1), nil
		}
		return driver.RowsAffected(0), nil

	case strings.Contains(q, "UPDATE system_settings") && strings.Contains(q, "require_mfa"):
		reqMFA, _ := args[3].(*bool)
		if reqMFA != nil {
			st.settings.RequireMFA = *reqMFA
		}
		return driver.RowsAffected(1), nil
	}

	return driver.RowsAffected(1), nil
}

func (s *mfaMockStmt) Query(args []driver.Value) (driver.Rows, error) {
	q := strings.TrimSpace(s.query)
	st := s.conn.state
	st.mu.Lock()
	defer st.mu.Unlock()

	switch {
	case strings.Contains(q, "UPDATE mfa_challenges") && strings.Contains(q, "failed_attempts = failed_attempts + 1"):
		tHash := fmt.Sprintf("%v", args[0])
		attempts := 1
		if c, ok := st.challenges[tHash]; ok {
			c.failedAttempts++
			attempts = c.failedAttempts
		}
		cols := []string{"failed_attempts"}
		return &mockRows{cols: cols, rows: [][]driver.Value{{attempts}}}, nil

	case strings.Contains(q, "FROM mfa_challenges") && strings.Contains(q, "temp_token_hash = $1"):
		tHash := fmt.Sprintf("%v", args[0])
		c, ok := st.challenges[tHash]
		if strings.Contains(q, "user_id") {
			cols := []string{"user_id", "webauthn_challenge", "expires_at", "failed_attempts"}
			if !ok {
				return &mockRows{cols: cols, rows: nil}, nil
			}
			var wChall any = nil
			if c.webauthnChallenge != "" {
				wChall = c.webauthnChallenge
			}
			return &mockRows{cols: cols, rows: [][]driver.Value{{c.userID, wChall, c.expiresAt, c.failedAttempts}}}, nil
		}
		cols := []string{"webauthn_challenge", "expires_at"}
		if !ok {
			return &mockRows{cols: cols, rows: nil}, nil
		}
		return &mockRows{cols: cols, rows: [][]driver.Value{{c.webauthnChallenge, c.expiresAt}}}, nil

	case strings.Contains(q, "DELETE FROM mfa_challenges") && strings.Contains(q, "RETURNING user_id"):
		tHash := fmt.Sprintf("%v", args[0])
		c, ok := st.challenges[tHash]
		cols := []string{"user_id", "expires_at"}
		if !ok {
			return &mockRows{cols: cols, rows: nil}, nil
		}
		delete(st.challenges, tHash)
		return &mockRows{cols: cols, rows: [][]driver.Value{{c.userID, c.expiresAt}}}, nil

	case strings.Contains(q, "FROM user_sessions s") && strings.Contains(q, "WHERE s.token_hash = $1"):
		tokHash := fmt.Sprintf("%v", args[0])
		uid, ok := st.sessions[tokHash]
		cols := []string{"user_id", "expires_at"}
		if !ok || st.user == nil || st.user.ID != uid {
			return &mockRows{cols: cols, rows: nil}, nil
		}
		values := [][]driver.Value{{uid, time.Now().UTC().Add(time.Hour)}}
		return &mockRows{cols: cols, rows: values}, nil

	case strings.Contains(q, "FROM users WHERE id = $1") && strings.Contains(q, "mfa_backup_codes"):
		if strings.Contains(q, "mfa_enabled") && strings.Contains(q, "totp_secret") {
			// GetMFAStatus
			cols := []string{"mfa_enabled", "mfa_enforced", "totp_secret", "mfa_backup_codes"}
			bJSON, _ := json.Marshal(st.backupCodes)
			var totpVal any = nil
			if st.totpSecret != "" {
				totpVal = st.totpSecret
			}
			return &mockRows{cols: cols, rows: [][]driver.Value{{st.user.MFAEnabled, st.user.MFAEnforced, totpVal, bJSON}}}, nil
		}
		if strings.Contains(q, "totp_secret") {
			// VerifyMFA
			cols := []string{"totp_secret", "mfa_backup_codes"}
			bJSON, _ := json.Marshal(st.backupCodes)
			var totpVal any = nil
			if st.totpSecret != "" {
				totpVal = st.totpSecret
			}
			return &mockRows{cols: cols, rows: [][]driver.Value{{totpVal, bJSON}}}, nil
		}
		if strings.Contains(q, "mfa_enabled") {
			// SaveWebAuthnCredential
			cols := []string{"mfa_enabled", "mfa_backup_codes"}
			bJSON, _ := json.Marshal(st.backupCodes)
			return &mockRows{cols: cols, rows: [][]driver.Value{{st.user.MFAEnabled, bJSON}}}, nil
		}
		// EnableTOTP
		cols := []string{"mfa_backup_codes"}
		bJSON, _ := json.Marshal(st.backupCodes)
		return &mockRows{cols: cols, rows: [][]driver.Value{{bJSON}}}, nil

	case strings.Contains(q, "FROM users WHERE LOWER(email) = $1"):
		cols := []string{"id", "password_hash", "status", "mfa_enabled", "totp_secret"}
		var totpVal any = nil
		if st.totpSecret != "" {
			totpVal = st.totpSecret
		}
		return &mockRows{cols: cols, rows: [][]driver.Value{{
			st.user.ID, st.passwordHash, st.user.Status, st.user.MFAEnabled, totpVal,
		}}}, nil

	case strings.Contains(q, "FROM users WHERE id = $1") && strings.Contains(q, "SELECT id, email, password_hash"):
		cols := []string{"id", "email", "password_hash", "google_sub", "display_name", "avatar_url", "status", "mfa_enabled", "mfa_enforced", "must_change_password", "created_at", "updated_at"}
		var gSub any = nil
		if st.user.GoogleSub != nil {
			gSub = *st.user.GoogleSub
		}
		return &mockRows{cols: cols, rows: [][]driver.Value{{
			st.user.ID, st.user.Email, st.passwordHash, gSub, st.user.DisplayName, st.user.AvatarURL, st.user.Status, st.user.MFAEnabled, st.user.MFAEnforced, st.user.MustChangePassword, st.user.CreatedAt, st.user.UpdatedAt,
		}}}, nil

	case strings.Contains(q, "SELECT mfa_enforced, totp_secret") && strings.Contains(q, "FROM users WHERE id = $1"):
		cols := []string{"mfa_enforced", "totp_secret", "count"}
		var totpVal any = nil
		if st.totpSecret != "" {
			totpVal = st.totpSecret
		}
		return &mockRows{cols: cols, rows: [][]driver.Value{{st.user.MFAEnforced, totpVal, len(st.credentials)}}}, nil

	case strings.Contains(q, "FROM webauthn_credentials") && strings.Contains(q, "user_id = $1"):
		cols := []string{"id", "user_id", "name", "credential_id", "attestation_type", "aaguid", "sign_count", "created_at", "last_used_at"}
		var values [][]driver.Value
		for _, c := range st.credentials {
			var lastUsed any = nil
			if c.LastUsedAt != nil {
				lastUsed = *c.LastUsedAt
			}
			values = append(values, []driver.Value{
				c.ID, c.UserID, c.Name, c.CredentialID, c.AttestationType, c.AAGUID, c.SignCount, c.CreatedAt, lastUsed,
			})
		}
		return &mockRows{cols: cols, rows: values}, nil

	case strings.Contains(q, "FROM webauthn_credentials") && strings.Contains(q, "credential_id = $1"):
		credIDArg, _ := args[0].([]byte)
		cols := []string{"id", "public_key", "sign_count", "user_id"}
		for _, c := range st.credentials {
			if bytes.Equal(c.CredentialID, credIDArg) {
				return &mockRows{cols: cols, rows: [][]driver.Value{{c.ID, c.PublicKey, c.SignCount, c.UserID}}}, nil
			}
		}
		return &mockRows{cols: cols, rows: nil}, nil

	case strings.Contains(q, "FROM system_settings"):
		cols := []string{"id", "default_hours_per_day", "default_issue_template_id", "default_task_template_id", "require_mfa", "updated_at"}
		values := [][]driver.Value{{
			st.settings.ID, st.settings.DefaultHoursPerDay, nil, nil, st.settings.RequireMFA, st.settings.UpdatedAt,
		}}
		return &mockRows{cols: cols, rows: values}, nil

	case strings.Contains(q, "SELECT DISTINCT sp.permission"):
		uid := fmt.Sprintf("%v", args[0])
		perms := st.sysPerms[uid]
		cols := []string{"permission"}
		var values [][]driver.Value
		for _, p := range perms {
			values = append(values, []driver.Value{p})
		}
		return &mockRows{cols: cols, rows: values}, nil

	case strings.Contains(q, "FROM users WHERE id = $1") && strings.Contains(q, "password_hash"):
		cols := []string{"password_hash", "mfa_enforced"}
		return &mockRows{cols: cols, rows: [][]driver.Value{{st.passwordHash, st.user.MFAEnforced}}}, nil

	case strings.Contains(q, "FROM system_allowed_domains"):
		return &mockRows{cols: []string{"domain"}, rows: nil}, nil

	case strings.Contains(q, "SELECT id, email, display_name, avatar_url, status, mfa_enabled, mfa_enforced, created_at, updated_at FROM users"):
		cols := []string{"id", "email", "display_name", "avatar_url", "status", "mfa_enabled", "mfa_enforced", "created_at", "updated_at"}
		values := [][]driver.Value{{
			st.user.ID, st.user.Email, st.user.DisplayName, st.user.AvatarURL, st.user.Status, st.user.MFAEnabled, st.user.MFAEnforced, st.user.CreatedAt, st.user.UpdatedAt,
		}}
		return &mockRows{cols: cols, rows: values}, nil
	}

	return &mockRows{cols: []string{"id"}, rows: nil}, nil
}

func setupMFATestServer() (*Server, *store.Store, string, *domain.User) {
	hashedPassword, _ := crypto.HashPassword("PolarisSecret123!")
	now := time.Now().UTC()

	mfaState.user = &domain.User{
		ID:          "user-mfa-1",
		Email:       "mfa_tester@example.com",
		DisplayName: "MFA Tester",
		Status:      domain.UserStatusActive,
		HasPassword: true,
		MFAEnabled:  false,
		MFAEnforced: false,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	mfaState.passwordHash = hashedPassword
	mfaState.totpSecret = ""
	mfaState.backupCodes = nil
	mfaState.credentials = nil
	mfaState.challenges = make(map[string]*mockChallenge)
	mfaState.sessions = make(map[string]string)
	mfaState.sysPerms = map[string][]string{
		"user-mfa-1": {domain.PermManageSystem},
	}
	mfaState.settings.RequireMFA = false

	db, _ := sql.Open("polaris_mfa_mock", "")
	mailer := mail.NewSMTPSender()
	fcmClient := fcm.NewClient(context.Background(), "")
	st := store.New(db, "0123456789abcdef0123456789abcdef", "http://localhost:3000", mailer, fcmClient)

	// Create valid active session for user-mfa-1
	rawTok, tokHash, _ := crypto.GenerateRandomToken(32)
	mfaState.sessions[tokHash] = mfaState.user.ID

	srv := NewServer(st, "http://localhost:3000", GoogleOAuthConfig{})
	return srv, st, rawTok, mfaState.user
}

// -------------------------------------------------------------------------
// Tests
// -------------------------------------------------------------------------

func TestMFA_TOTP_LifecycleAndLogin(t *testing.T) {
	srv, _, sessionTok, _ := setupMFATestServer()

	// 1. Initial login without MFA -> direct login success
	loginReq := map[string]string{
		"email":    "mfa_tester@example.com",
		"password": "PolarisSecret123!",
	}
	b, _ := json.Marshal(loginReq)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewReader(b))
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 on login, got %d", rec.Code)
	}
	var loginRes map[string]any
	_ = json.NewDecoder(rec.Body).Decode(&loginRes)
	if loginRes["mfa_required"] == true {
		t.Fatalf("expected mfa_required=false or omitted, got %v", loginRes["mfa_required"])
	}

	// 2. Setup TOTP: GET/POST /api/v1/auth/mfa/totp/setup
	req = httptest.NewRequest(http.MethodPost, "/api/v1/auth/mfa/totp/setup", nil)
	req.AddCookie(&http.Cookie{Name: SessionCookieName, Value: sessionTok})
	rec = httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("setup TOTP failed: %d: %s", rec.Code, rec.Body.String())
	}
	var setupRes struct {
		Secret     string `json:"secret"`
		OTPAuthURL string `json:"otpauth_url"`
	}
	_ = json.NewDecoder(rec.Body).Decode(&setupRes)
	if setupRes.Secret == "" || !strings.HasPrefix(setupRes.OTPAuthURL, "otpauth://totp/") {
		t.Fatalf("invalid setup response: %+v", setupRes)
	}

	// 3. Enable TOTP with invalid code -> should fail 400
	enableReq := map[string]string{
		"secret": setupRes.Secret,
		"code":   "999999",
	}
	b, _ = json.Marshal(enableReq)
	req = httptest.NewRequest(http.MethodPost, "/api/v1/auth/mfa/totp/enable", bytes.NewReader(b))
	req.AddCookie(&http.Cookie{Name: SessionCookieName, Value: sessionTok})
	rec = httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid TOTP code, got %d", rec.Code)
	}

	// 4. Enable TOTP with valid code -> success, returns 8 backup codes
	validCode, err := mfa.GenerateTOTP(setupRes.Secret, time.Now())
	if err != nil {
		t.Fatalf("generate TOTP failed: %v", err)
	}
	enableReq["code"] = validCode
	b, _ = json.Marshal(enableReq)
	req = httptest.NewRequest(http.MethodPost, "/api/v1/auth/mfa/totp/enable", bytes.NewReader(b))
	req.AddCookie(&http.Cookie{Name: SessionCookieName, Value: sessionTok})
	rec = httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("enable TOTP failed: %d: %s", rec.Code, rec.Body.String())
	}
	var enableRes struct {
		Status      string   `json:"status"`
		BackupCodes []string `json:"backup_codes"`
	}
	_ = json.NewDecoder(rec.Body).Decode(&enableRes)
	if len(enableRes.BackupCodes) != 8 {
		t.Fatalf("expected 8 backup codes, got %d", len(enableRes.BackupCodes))
	}

	// 5. Check MFA status: GET /api/v1/auth/mfa/status
	req = httptest.NewRequest(http.MethodGet, "/api/v1/auth/mfa/status", nil)
	req.AddCookie(&http.Cookie{Name: SessionCookieName, Value: sessionTok})
	rec = httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	var statusRes domain.MFAStatus
	_ = json.NewDecoder(rec.Body).Decode(&statusRes)
	if !statusRes.MFAEnabled || !statusRes.HasTOTP || statusRes.RemainingBackupCodes != 8 {
		t.Fatalf("unexpected MFA status: %+v", statusRes)
	}

	// 6. Login now requires MFA: POST /api/v1/auth/login
	b, _ = json.Marshal(loginReq)
	req = httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewReader(b))
	rec = httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 on login, got %d", rec.Code)
	}
	var challengeRes struct {
		MFARequired    bool     `json:"mfa_required"`
		TempToken      string   `json:"temp_token"`
		AllowedMethods []string `json:"allowed_methods"`
		HasTOTP        bool     `json:"has_totp"`
	}
	_ = json.NewDecoder(rec.Body).Decode(&challengeRes)
	if !challengeRes.MFARequired || challengeRes.TempToken == "" || !challengeRes.HasTOTP {
		t.Fatalf("expected MFA challenge on login, got %+v", challengeRes)
	}

	// Verify no session cookie set
	for _, c := range rec.Result().Cookies() {
		if c.Name == SessionCookieName && c.Value != "" {
			t.Fatalf("session cookie should not be set before MFA verification")
		}
	}

	// 7. Verify MFA with invalid code -> 401
	verifyReq := map[string]string{
		"temp_token": challengeRes.TempToken,
		"method":     "totp",
		"code":       "123456",
	}
	b, _ = json.Marshal(verifyReq)
	req = httptest.NewRequest(http.MethodPost, "/api/v1/auth/mfa/verify", bytes.NewReader(b))
	rec = httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for bad TOTP, got %d", rec.Code)
	}

	// 8. Verify MFA with valid TOTP code -> 200 OK, sets session cookie
	loginTOTP, _ := mfa.GenerateTOTP(setupRes.Secret, time.Now())
	verifyReq["code"] = loginTOTP
	b, _ = json.Marshal(verifyReq)
	req = httptest.NewRequest(http.MethodPost, "/api/v1/auth/mfa/verify", bytes.NewReader(b))
	rec = httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("MFA verify failed: %d: %s", rec.Code, rec.Body.String())
	}
	hasCookie := false
	for _, c := range rec.Result().Cookies() {
		if c.Name == SessionCookieName && c.Value != "" {
			hasCookie = true
		}
	}
	if !hasCookie {
		t.Fatalf("expected session cookie after successful MFA verification")
	}

	// 9. Login and verify with Backup Code
	b, _ = json.Marshal(loginReq)
	req = httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewReader(b))
	rec = httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	_ = json.NewDecoder(rec.Body).Decode(&challengeRes)

	usedBackupCode := enableRes.BackupCodes[0]
	verifyReq = map[string]string{
		"temp_token": challengeRes.TempToken,
		"method":     "backup_code",
		"code":       usedBackupCode,
	}
	b, _ = json.Marshal(verifyReq)
	req = httptest.NewRequest(http.MethodPost, "/api/v1/auth/mfa/verify", bytes.NewReader(b))
	rec = httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("backup code verification failed: %d: %s", rec.Code, rec.Body.String())
	}

	// Reusing the same backup code must fail
	b, _ = json.Marshal(loginReq)
	req = httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewReader(b))
	rec = httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	_ = json.NewDecoder(rec.Body).Decode(&challengeRes)

	verifyReq["temp_token"] = challengeRes.TempToken
	b, _ = json.Marshal(verifyReq)
	req = httptest.NewRequest(http.MethodPost, "/api/v1/auth/mfa/verify", bytes.NewReader(b))
	rec = httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("reused backup code should fail with 401, got %d", rec.Code)
	}
}

func TestMFA_EnforcementMiddleware(t *testing.T) {
	srv, _, sessionTok, user := setupMFATestServer()

	// 1. Initially without MFA enforcement, protected endpoint succeeds
	req := httptest.NewRequest(http.MethodGet, "/api/v1/users", nil)
	req.AddCookie(&http.Cookie{Name: SessionCookieName, Value: sessionTok})
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 before enforcement, got %d", rec.Code)
	}

	// 2. Set user-level enforcement: PUT /api/v1/system/users/{userId}/mfa-enforcement
	enforceReq := map[string]bool{"mfa_enforced": true}
	b, _ := json.Marshal(enforceReq)
	req = httptest.NewRequest(http.MethodPut, "/api/v1/system/users/"+user.ID+"/mfa-enforcement", bytes.NewReader(b))
	req.AddCookie(&http.Cookie{Name: SessionCookieName, Value: sessionTok})
	rec = httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("set MFA enforcement failed: %d: %s", rec.Code, rec.Body.String())
	}

	// 3. Centralized middleware should block /api/v1/users with 403 MFA_SETUP_REQUIRED
	req = httptest.NewRequest(http.MethodGet, "/api/v1/users", nil)
	req.AddCookie(&http.Cookie{Name: SessionCookieName, Value: sessionTok})
	rec = httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 Forbidden under MFA enforcement, got %d", rec.Code)
	}
	var errRes map[string]string
	_ = json.NewDecoder(rec.Body).Decode(&errRes)
	if errRes["error"] != "MFA_SETUP_REQUIRED" {
		t.Fatalf("expected error MFA_SETUP_REQUIRED, got %v", errRes["error"])
	}

	// 4. Exempt paths should still succeed
	exemptPaths := []string{
		"/api/v1/auth/me",
		"/api/v1/auth/mfa/status",
		"/api/v1/auth/mfa/totp/setup",
	}
	for _, p := range exemptPaths {
		method := http.MethodGet
		if strings.HasSuffix(p, "/setup") {
			method = http.MethodPost
		}
		r := httptest.NewRequest(method, p, nil)
		r.AddCookie(&http.Cookie{Name: SessionCookieName, Value: sessionTok})
		w := httptest.NewRecorder()
		srv.ServeHTTP(w, r)
		if w.Code != http.StatusOK {
			t.Errorf("expected exempt path %s to succeed, got %d: %s", p, w.Code, w.Body.String())
		}
	}

	// 5. Disable TOTP should be blocked by policy when MFA is enforced
	mfaState.user.MFAEnabled = true
	mfaState.totpSecret = "JBSWY3DPEHPK3PXP"
	disableReq := map[string]string{"password": "PolarisSecret123!"}
	b, _ = json.Marshal(disableReq)
	req = httptest.NewRequest(http.MethodPost, "/api/v1/auth/mfa/totp/disable", bytes.NewReader(b))
	req.AddCookie(&http.Cookie{Name: SessionCookieName, Value: sessionTok})
	rec = httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 Bad Request when disabling TOTP while MFA is enforced, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestMFA_WebAuthnRegistrationAndVerification(t *testing.T) {
	srv, _, sessionTok, _ := setupMFATestServer()

	// 1. Request registration options: POST /api/v1/auth/mfa/webauthn/register-options
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/mfa/webauthn/register-options", nil)
	req.AddCookie(&http.Cookie{Name: SessionCookieName, Value: sessionTok})
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("webauthn register-options failed: %d: %s", rec.Code, rec.Body.String())
	}
	var regOpts mfa.PublicKeyCredentialCreationOptions
	_ = json.NewDecoder(rec.Body).Decode(&regOpts)
	if regOpts.Challenge == "" || regOpts.RP.Name != "Polaris" {
		t.Fatalf("invalid register options: %+v", regOpts)
	}

	// 2. Mock authenticating browser: create ECDSA P-256 key and build attestationObject
	privKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate EC key: %v", err)
	}

	// Build COSE key CBOR bytes
	xBytes := privKey.PublicKey.X.Bytes()
	yBytes := privKey.PublicKey.Y.Bytes()
	// Pad to 32 bytes
	if len(xBytes) < 32 {
		xBytes = append(make([]byte, 32-len(xBytes)), xBytes...)
	}
	if len(yBytes) < 32 {
		yBytes = append(make([]byte, 32-len(yBytes)), yBytes...)
	}
	coseKeyMap := map[any]any{
		int64(1):  int64(2),  // kty: EC2
		int64(3):  int64(-7), // alg: ES256
		int64(-1): int64(1),  // crv: P-256
		int64(-2): xBytes,    // x
		int64(-3): yBytes,    // y
	}
	coseBytes, _ := mfa.MarshalCBOR(coseKeyMap)

	credID := []byte("mock-cred-12345678")
	var authData bytes.Buffer
	authData.Write(make([]byte, 32)) // RP ID hash
	authData.WriteByte(0x41)         // flags: UP (0x01) | AT (0x40)
	var scBuf [4]byte
	binary.BigEndian.PutUint32(scBuf[:], 1)
	authData.Write(scBuf[:])     // signCount = 1
	authData.Write(make([]byte, 16)) // AAGUID
	var credLenBuf [2]byte
	binary.BigEndian.PutUint16(credLenBuf[:], uint16(len(credID)))
	authData.Write(credLenBuf[:])
	authData.Write(credID)
	authData.Write(coseBytes)

	attObjMap := map[any]any{
		"fmt":      "none",
		"authData": authData.Bytes(),
		"attStmt":  map[any]any{},
	}
	attBytes, _ := mfa.MarshalCBOR(attObjMap)

	clientDataJSON := fmt.Sprintf(`{"type":"webauthn.create","challenge":%q,"origin":"http://localhost:3000"}`, regOpts.Challenge)

	// 3. Register verify: POST /api/v1/auth/mfa/webauthn/register-verify
	regVerifyBody := map[string]any{
		"name":               "My YubiKey",
		"client_data_json":   base64.RawURLEncoding.EncodeToString([]byte(clientDataJSON)),
		"attestation_object": base64.RawURLEncoding.EncodeToString(attBytes),
	}
	b, _ := json.Marshal(regVerifyBody)
	req = httptest.NewRequest(http.MethodPost, "/api/v1/auth/mfa/webauthn/register-verify", bytes.NewReader(b))
	req.AddCookie(&http.Cookie{Name: SessionCookieName, Value: sessionTok})
	rec = httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("register-verify failed (%d): %s", rec.Code, rec.Body.String())
	}
	var regVerifyRes struct {
		Status      string   `json:"status"`
		BackupCodes []string `json:"backup_codes"`
	}
	_ = json.NewDecoder(rec.Body).Decode(&regVerifyRes)
	if regVerifyRes.Status != "registered" || len(regVerifyRes.BackupCodes) != 8 {
		t.Fatalf("unexpected register-verify response: %+v", regVerifyRes)
	}

	// 4. WebAuthn Login Assertion Flow
	// (a) Login to get temp_token
	loginReq := map[string]string{
		"email":    "mfa_tester@example.com",
		"password": "PolarisSecret123!",
	}
	b, _ = json.Marshal(loginReq)
	req = httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewReader(b))
	rec = httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	var chal struct {
		TempToken   string `json:"temp_token"`
		HasWebAuthn bool   `json:"has_webauthn"`
	}
	_ = json.NewDecoder(rec.Body).Decode(&chal)
	if chal.TempToken == "" || !chal.HasWebAuthn {
		t.Fatalf("expected WebAuthn challenge on login: %+v", chal)
	}

	// (b) Request login options
	b, _ = json.Marshal(map[string]string{"temp_token": chal.TempToken})
	req = httptest.NewRequest(http.MethodPost, "/api/v1/auth/mfa/webauthn/login-options", bytes.NewReader(b))
	rec = httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("login-options failed: %d: %s", rec.Code, rec.Body.String())
	}
	var loginOpts mfa.PublicKeyCredentialRequestOptions
	_ = json.NewDecoder(rec.Body).Decode(&loginOpts)
	if loginOpts.Challenge == "" || len(loginOpts.AllowCredentials) == 0 {
		t.Fatalf("expected login options with challenge and allowed credentials, got: %+v", loginOpts)
	}

	// (c) Sign assertion
	assertionClientDataJSON := fmt.Sprintf(`{"type":"webauthn.get","challenge":%q,"origin":"http://localhost:3000"}`, loginOpts.Challenge)
	var assertionAuthData bytes.Buffer
	assertionAuthData.Write(make([]byte, 32))
	assertionAuthData.WriteByte(0x01) // UP = 1
	var scBuf2 [4]byte
	binary.BigEndian.PutUint32(scBuf2[:], 2)
	assertionAuthData.Write(scBuf2[:]) // signCount = 2

	cdHash := sha256.Sum256([]byte(assertionClientDataJSON))
	var signedMsg bytes.Buffer
	signedMsg.Write(assertionAuthData.Bytes())
	signedMsg.Write(cdHash[:])
	msgHash := sha256.Sum256(signedMsg.Bytes())

	sig, err := ecdsa.SignASN1(rand.Reader, privKey, msgHash[:])
	if err != nil {
		t.Fatalf("sign assertion: %v", err)
	}

	// (d) Verify login assertion: POST /api/v1/auth/mfa/webauthn/login-verify
	loginVerifyBody := map[string]any{
		"temp_token":         chal.TempToken,
		"credential_id":      base64.RawURLEncoding.EncodeToString(credID),
		"client_data_json":   base64.RawURLEncoding.EncodeToString([]byte(assertionClientDataJSON)),
		"authenticator_data": base64.RawURLEncoding.EncodeToString(assertionAuthData.Bytes()),
		"signature":          base64.RawURLEncoding.EncodeToString(sig),
	}
	b, _ = json.Marshal(loginVerifyBody)
	req = httptest.NewRequest(http.MethodPost, "/api/v1/auth/mfa/webauthn/login-verify", bytes.NewReader(b))
	rec = httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("login-verify failed (%d): %s", rec.Code, rec.Body.String())
	}
	var loginSuccess map[string]any
	_ = json.NewDecoder(rec.Body).Decode(&loginSuccess)
	if loginSuccess["session_token"] == "" {
		t.Fatalf("expected session_token in login-verify response")
	}

	// 5. Delete security key: DELETE /api/v1/auth/mfa/webauthn/keys/{id}
	credUUID := mfaState.credentials[0].ID
	req = httptest.NewRequest(http.MethodDelete, "/api/v1/auth/mfa/webauthn/keys/"+credUUID, nil)
	req.AddCookie(&http.Cookie{Name: SessionCookieName, Value: sessionTok})
	rec = httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("delete key failed (%d): %s", rec.Code, rec.Body.String())
	}
}
