package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/takayoshi-u/polaris/backend/internal/domain"
)

func TestBulkInvite_Unauthorized(t *testing.T) {
	srv := NewServer(nil, "http://localhost:3000", GoogleOAuthConfig{})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/users/bulk-invite", bytes.NewReader([]byte("{}")))
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for unauthorized bulk invite, got %d", rec.Code)
	}
}

func TestBulkInvite_EmptyList(t *testing.T) {
	rig := setupTestRig(t)
	defer rig.db.Close()

	body, _ := json.Marshal(domain.BulkInviteRequest{
		Invitations: []domain.UserInviteItem{},
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/users/bulk-invite", bytes.NewReader(body))
	req.AddCookie(rig.adminCookie)
	rec := httptest.NewRecorder()
	rig.srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for empty invitations list, got %d", rec.Code)
	}
}

func TestBulkInvite_FullIntegration(t *testing.T) {
	rig := setupTestRig(t)
	defer rig.db.Close()

	ctx := t.Context()
	srv := rig.srv
	cookie := rig.adminCookie

	// 1. Set allowed domains to ["example.com"]
	settingsBody, _ := json.Marshal(map[string]any{
		"allowed_domains":       []string{"example.com"},
		"default_hours_per_day": 8.0,
	})
	req := httptest.NewRequest(http.MethodPut, "/api/v1/system/settings", bytes.NewReader(settingsBody))
	req.AddCookie(cookie)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("failed to update system settings (%d): %s", rec.Code, rec.Body.String())
	}

	// 2. Pre-create an existing user
	existingBody, _ := json.Marshal(map[string]any{
		"email":        "existing@example.com",
		"display_name": "Existing User",
		"password":     "Password123!",
	})
	req = httptest.NewRequest(http.MethodPost, "/api/v1/users", bytes.NewReader(existingBody))
	req.AddCookie(cookie)
	rec = httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("failed to create existing user (%d): %s", rec.Code, rec.Body.String())
	}

	// 3. Perform bulk invite
	bulkReq := domain.BulkInviteRequest{
		Invitations: []domain.UserInviteItem{
			{Email: "alice@example.com", DisplayName: "Alice"},
			{Email: "bob@example.com", DisplayName: "Bob"},
			{Email: "bad@unknown.com", DisplayName: "Bad Domain User"},
			{Email: "existing@example.com", DisplayName: "Already Exists"},
			{Email: "alice@example.com", DisplayName: "Alice Duplicate"},
			{Email: "not-an-email", DisplayName: "Invalid Format"},
		},
		DefaultPermissions: []string{domain.PermCreateProject},
	}
	bulkBody, _ := json.Marshal(bulkReq)

	req = httptest.NewRequest(http.MethodPost, "/api/v1/users/bulk-invite", bytes.NewReader(bulkBody))
	req.AddCookie(cookie)
	rec = httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("bulk invite failed (%d): %s", rec.Code, rec.Body.String())
	}

	var res domain.BulkInviteResult
	if err := json.NewDecoder(rec.Body).Decode(&res); err != nil {
		t.Fatalf("failed to decode bulk invite response: %v", err)
	}

	if res.Total != 6 {
		t.Fatalf("expected total 6, got %d", res.Total)
	}
	if res.InvitedCount != 2 {
		t.Fatalf("expected invited_count 2, got %d", res.InvitedCount)
	}
	if res.FailedCount != 4 {
		t.Fatalf("expected failed_count 4, got %d", res.FailedCount)
	}
	if len(res.InvitedUsers) != 2 {
		t.Fatalf("expected 2 invited users, got %d", len(res.InvitedUsers))
	}

	// Verify successful users
	invitedEmails := map[string]bool{}
	for _, u := range res.InvitedUsers {
		invitedEmails[u.Email] = true
		if u.Status != domain.UserStatusInvited {
			t.Errorf("expected status 'invited' for %s, got %s", u.Email, u.Status)
		}
		if u.InviteURL == "" {
			t.Errorf("expected non-empty invite_url for %s", u.Email)
		}
	}
	if !invitedEmails["alice@example.com"] || !invitedEmails["bob@example.com"] {
		t.Errorf("missing alice or bob in invited_users: %v", invitedEmails)
	}

	// Verify failures
	failureReasons := map[string]string{}
	for _, f := range res.Failures {
		failureReasons[f.Email] = f.Reason
	}

	if reason, ok := failureReasons["bad@unknown.com"]; !ok || reason != "メールアドレスのドメインが許可されていません" {
		t.Errorf("expected domain rejection for bad@unknown.com, got reason: %s", reason)
	}
	if reason, ok := failureReasons["existing@example.com"]; !ok || reason != "既に登録されているユーザーです" {
		t.Errorf("expected already registered for existing@example.com, got reason: %s", reason)
	}
	if reason, ok := failureReasons["alice@example.com"]; !ok || reason != "既に登録されているユーザーです" {
		t.Errorf("expected duplicate in batch for alice@example.com, got reason: %s", reason)
	}
	if reason, ok := failureReasons["not-an-email"]; !ok || reason != "メールアドレスの形式が不正です" {
		t.Errorf("expected invalid format for not-an-email, got reason: %s", reason)
	}

	_ = ctx
}
