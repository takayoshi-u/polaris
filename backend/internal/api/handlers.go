package api

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/takayoshi-u/polaris/backend/internal/domain"
	"github.com/takayoshi-u/polaris/backend/internal/store"
)

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"status": "ok"})
}

func (s *Server) handleSetupStatus(w http.ResponseWriter, r *http.Request) {
	req, err := s.store.IsSetupRequired(r.Context())
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"setup_required": req, "needs_setup": req})
}

func (s *Server) handleSetupInit(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Email       string  `json:"email"`
		Password    string  `json:"password"`
		DisplayName string  `json:"display_name"`
		GoogleSub   *string `json:"google_sub"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid JSON body"})
		return
	}
	u, tok, err := s.store.SetupInitialAdmin(r.Context(), body.Email, body.Password, body.DisplayName, body.GoogleSub)
	if err != nil {
		writeError(w, err)
		return
	}
	setSessionCookie(w, tok)
	writeJSON(w, http.StatusCreated, map[string]any{"user": u, "session_token": tok})
}

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid JSON body"})
		return
	}
	res, err := s.store.AuthenticateWithPassword(r.Context(), body.Email, body.Password)
	if err != nil {
		writeError(w, err)
		return
	}
	if res.MFARequired {
		writeJSON(w, http.StatusOK, map[string]any{
			"mfa_required":    true,
			"temp_token":      res.TempToken,
			"allowed_methods": res.AllowedMethods,
			"has_totp":        res.HasTOTP,
			"has_webauthn":    res.HasWebAuthn,
		})
		return
	}
	setSessionCookie(w, res.SessionToken)
	writeJSON(w, http.StatusOK, map[string]any{"user": res.User, "session_token": res.SessionToken})
}

func (s *Server) handleGoogleAuthURL(w http.ResponseWriter, r *http.Request) {
	if s.oauthCfg == nil {
		writeJSON(w, http.StatusOK, map[string]any{"url": "", "enabled": false})
		return
	}
	state := r.URL.Query().Get("state")
	if state == "" {
		state = "polaris-oauth"
	}
	url := s.oauthCfg.AuthCodeURL(state)
	writeJSON(w, http.StatusOK, map[string]any{"url": url, "enabled": true})
}

func (s *Server) handleGoogleCallback(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Code        string `json:"code"`
		Email       string `json:"email"`
		GoogleSub   string `json:"google_sub"`
		DisplayName string `json:"display_name"`
		AvatarURL   string `json:"avatar_url"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid JSON body"})
		return
	}

	email := body.Email
	googleSub := body.GoogleSub
	displayName := body.DisplayName
	avatarURL := body.AvatarURL

	if body.Code != "" && s.oauthCfg != nil {
		token, err := s.oauthCfg.Exchange(r.Context(), body.Code)
		if err != nil {
			writeJSON(w, http.StatusUnauthorized, map[string]any{"error": "failed to exchange Google OAuth code"})
			return
		}
		client := s.oauthCfg.Client(r.Context(), token)
		resp, err := client.Get("https://www.googleapis.com/oauth2/v3/userinfo")
		if err != nil || resp.StatusCode != http.StatusOK {
			writeJSON(w, http.StatusUnauthorized, map[string]any{"error": "failed to fetch Google userinfo"})
			return
		}
		defer resp.Body.Close()
		var info struct {
			Sub     string `json:"sub"`
			Email   string `json:"email"`
			Name    string `json:"name"`
			Picture string `json:"picture"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&info); err != nil {
			writeJSON(w, http.StatusUnauthorized, map[string]any{"error": "invalid userinfo response"})
			return
		}
		email = info.Email
		googleSub = info.Sub
		displayName = info.Name
		avatarURL = info.Picture
	}

	u, tok, err := s.store.LoginWithGoogle(r.Context(), email, googleSub, displayName, avatarURL)
	if err != nil {
		writeError(w, err)
		return
	}
	if u.MFAEnabled {
		tempToken, err := s.store.CreateMFAChallenge(r.Context(), u.ID)
		if err != nil {
			writeError(w, err)
			return
		}
		status, err := s.store.GetMFAStatus(r.Context(), u.ID)
		hasTOTP := true
		hasWebAuthn := false
		if err == nil && status != nil {
			hasTOTP = status.HasTOTP
			hasWebAuthn = status.HasWebAuthn
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"mfa_required":    true,
			"temp_token":      tempToken,
			"allowed_methods": []string{"totp", "webauthn", "backup_code"},
			"has_totp":        hasTOTP,
			"has_webauthn":    hasWebAuthn,
		})
		return
	}
	setSessionCookie(w, tok)
	writeJSON(w, http.StatusOK, map[string]any{"user": u, "session_token": tok})
}

func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	tok := extractSessionToken(r)
	_ = s.store.RevokeSession(r.Context(), tok)
	clearSessionCookie(w)
	writeJSON(w, http.StatusOK, map[string]any{"status": "logged_out"})
}

func (s *Server) handleAcceptInvite(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Token       string `json:"token"`
		Password    string `json:"password"`
		DisplayName string `json:"display_name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid JSON body"})
		return
	}
	u, tok, err := s.store.AcceptInvite(r.Context(), body.Token, body.Password, body.DisplayName)
	if err != nil {
		writeError(w, err)
		return
	}
	setSessionCookie(w, tok)
	writeJSON(w, http.StatusOK, map[string]any{"user": u, "session_token": tok})
}

func (s *Server) handleRequestPasswordReset(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Email string `json:"email"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid JSON body"})
		return
	}
	_, err := s.store.RequestPasswordReset(r.Context(), body.Email)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"status": "sent"})
}

func (s *Server) handleConfirmPasswordReset(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Token       string `json:"token"`
		NewPassword string `json:"new_password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid JSON body"})
		return
	}
	if err := s.store.ConfirmPasswordReset(r.Context(), body.Token, body.NewPassword); err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"status": "password_reset"})
}

func (s *Server) handleConfirmEmailChange(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Token string `json:"token"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid JSON body"})
		return
	}
	u, err := s.store.ConfirmEmailChange(r.Context(), body.Token)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"user": u})
}

func (s *Server) handleGetMe(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, currentUser(r))
}

func (s *Server) handleUpdateMyProfile(w http.ResponseWriter, r *http.Request) {
	var body struct {
		DisplayName string `json:"display_name"`
		AvatarURL   string `json:"avatar_url"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid JSON body"})
		return
	}
	u, err := s.store.UpdateUserProfile(r.Context(), currentUser(r).ID, body.DisplayName, body.AvatarURL)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, u)
}

func (s *Server) handleUpdateMyPassword(w http.ResponseWriter, r *http.Request) {
	var body struct {
		CurrentPassword string `json:"current_password"`
		NewPassword     string `json:"new_password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid JSON body"})
		return
	}
	if err := s.store.SetOrChangePassword(r.Context(), currentUser(r).ID, body.CurrentPassword, body.NewPassword); err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"status": "password_updated"})
}

func (s *Server) handleRequestMyEmailChange(w http.ResponseWriter, r *http.Request) {
	var body struct {
		CurrentPassword string `json:"current_password"`
		NewEmail        string `json:"new_email"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid JSON body"})
		return
	}
	_, err := s.store.RequestSelfEmailChange(r.Context(), currentUser(r).ID, body.CurrentPassword, body.NewEmail)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"status": "verification_email_sent"})
}

// System Management
func (s *Server) handleGetSystemSettings(w http.ResponseWriter, r *http.Request) {
	st, err := s.store.GetSystemSettings(r.Context())
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, st)
}

func (s *Server) handleUpdateSystemSettings(w http.ResponseWriter, r *http.Request) {
	var body struct {
		DefaultHoursPerDay     float64   `json:"default_hours_per_day"`
		DefaultIssueTemplateID *string   `json:"default_issue_template_id"`
		DefaultTaskTemplateID  *string   `json:"default_task_template_id"`
		AllowedDomains         *[]string `json:"allowed_domains"`
		RequireMFA             *bool     `json:"require_mfa"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid JSON body"})
		return
	}
	st, err := s.store.UpdateSystemSettings(r.Context(), body.DefaultHoursPerDay, body.DefaultIssueTemplateID, body.DefaultTaskTemplateID, body.AllowedDomains, body.RequireMFA)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, st)
}

func (s *Server) handleGetSMTPSettings(w http.ResponseWriter, r *http.Request) {
	st, err := s.store.GetSMTPSettings(r.Context())
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, st)
}

func (s *Server) handleUpdateSMTPSettings(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Enabled   bool    `json:"enabled"`
		Host      string  `json:"host"`
		Port      int     `json:"port"`
		UseAuth   bool    `json:"use_auth"`
		Username  string  `json:"username"`
		Password  *string `json:"password"`
		FromEmail string  `json:"from_email"`
		FromName  string  `json:"from_name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid JSON body"})
		return
	}
	st, err := s.store.UpdateSMTPSettings(r.Context(), body.Enabled, body.Host, body.Port, body.UseAuth, body.Username, body.Password, body.FromEmail, body.FromName)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, st)
}

func (s *Server) handleSendTestEmail(w http.ResponseWriter, r *http.Request) {
	var body struct {
		RecipientEmail string `json:"recipient_email"`
		ToEmail        string `json:"to_email"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid JSON body"})
		return
	}
	recipient := body.RecipientEmail
	if recipient == "" {
		recipient = body.ToEmail
	}
	if err := s.store.SendTestEmail(r.Context(), recipient); err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"status": "sent"})
}

func (s *Server) handleListSystemPermissions(w http.ResponseWriter, r *http.Request) {
	list, err := s.store.ListSystemPermissions(r.Context())
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, list)
}

func (s *Server) handleGrantSystemPermission(w http.ResponseWriter, r *http.Request) {
	var body struct {
		SubjectType string `json:"subject_type"`
		SubjectID   string `json:"subject_id"`
		Permission  string `json:"permission"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid JSON body"})
		return
	}
	entry, err := s.store.GrantSystemPermission(r.Context(), body.SubjectType, body.SubjectID, body.Permission)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, entry)
}

func (s *Server) handleRevokeSystemPermission(w http.ResponseWriter, r *http.Request) {
	if err := s.store.RevokeSystemPermission(r.Context(), r.PathValue("id")); err != nil {
		writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// Users
func (s *Server) handleListUsers(w http.ResponseWriter, r *http.Request) {
	users, err := s.store.ListUsers(r.Context())
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, users)
}

func (s *Server) handleCreateUser(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Email       string   `json:"email"`
		DisplayName string   `json:"display_name"`
		Password    string   `json:"password"`
		GroupIDs    []string `json:"group_ids"`
		Permissions []string `json:"permissions"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid JSON body"})
		return
	}
	u, err := s.store.CreateUserDirect(r.Context(), body.Email, body.DisplayName, body.Password, body.GroupIDs, body.Permissions)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, u)
}

func (s *Server) handleInviteUser(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Email       string   `json:"email"`
		DisplayName string   `json:"display_name"`
		GroupIDs    []string `json:"group_ids"`
		Permissions []string `json:"permissions"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid JSON body"})
		return
	}
	u, _, err := s.store.InviteUser(r.Context(), body.Email, body.DisplayName, body.GroupIDs, body.Permissions)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, u)
}

func (s *Server) handleBulkInviteUsers(w http.ResponseWriter, r *http.Request) {
	var body domain.BulkInviteRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid JSON body"})
		return
	}
	if len(body.Invitations) == 0 {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invitations list cannot be empty"})
		return
	}

	result, err := s.store.BulkInviteUsers(r.Context(), body.Invitations, body.DefaultGroupIDs, body.DefaultPermissions)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (s *Server) handleResendInvite(w http.ResponseWriter, r *http.Request) {
	rawToken, err := s.store.ResendInvite(r.Context(), r.PathValue("userId"))
	if err != nil {
		writeError(w, err)
		return
	}
	inviteURL := fmt.Sprintf("%s/auth/invite?token=%s", strings.TrimRight(s.corsAllowedOrigin, "/"), rawToken)
	writeJSON(w, http.StatusOK, map[string]any{
		"status":     "invite_resent",
		"invite_url": inviteURL,
	})
}

func (s *Server) handleAdminChangeUserEmail(w http.ResponseWriter, r *http.Request) {
	var body struct {
		NewEmail            string `json:"new_email"`
		SendVerification    *bool  `json:"send_verification"`
		RequireVerification *bool  `json:"require_verification"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid JSON body"})
		return
	}
	sendVerification := false
	if body.SendVerification != nil {
		sendVerification = *body.SendVerification
	} else if body.RequireVerification != nil {
		sendVerification = *body.RequireVerification
	}
	u, _, err := s.store.AdminChangeUserEmail(r.Context(), r.PathValue("userId"), body.NewEmail, sendVerification)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, u)
}

func (s *Server) handleUpdateUserStatus(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Status string `json:"status"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid JSON body"})
		return
	}
	u, err := s.store.UpdateUserStatus(r.Context(), r.PathValue("userId"), body.Status)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, u)
}

// System Default Statuses / Custom Fields / Templates
func (s *Server) handleListSystemStatuses(w http.ResponseWriter, r *http.Request) {
	list, err := s.store.ListStatuses(r.Context(), nil)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, list)
}

func (s *Server) handleCreateSystemStatus(w http.ResponseWriter, r *http.Request) {
	var body struct {
		DisplayOrder int    `json:"display_order"`
		Name         string `json:"name"`
		Color        string `json:"color"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid JSON body"})
		return
	}
	st, err := s.store.CreateStatus(r.Context(), nil, body.DisplayOrder, body.Name, body.Color)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, st)
}

func (s *Server) handleUpdateSystemStatus(w http.ResponseWriter, r *http.Request) {
	var body struct {
		DisplayOrder int    `json:"display_order"`
		Name         string `json:"name"`
		Color        string `json:"color"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid JSON body"})
		return
	}
	st, err := s.store.UpdateStatus(r.Context(), r.PathValue("statusId"), body.DisplayOrder, body.Name, body.Color)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, st)
}

func (s *Server) handleDeleteSystemStatus(w http.ResponseWriter, r *http.Request) {
	if err := s.store.DeleteStatus(r.Context(), nil, r.PathValue("statusId"), nil); err != nil {
		writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleResetSystemStatuses(w http.ResponseWriter, r *http.Request) {
	list, err := s.store.ResetSystemStatuses(r.Context())
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, list)
}

func (s *Server) handleListSystemCustomFields(w http.ResponseWriter, r *http.Request) {
	targetType := r.URL.Query().Get("target_type")
	list, err := s.store.ListCustomFields(r.Context(), nil, targetType)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, list)
}

func (s *Server) handleCreateSystemCustomField(w http.ResponseWriter, r *http.Request) {
	var body struct {
		TargetType string          `json:"target_type"`
		Name       string          `json:"name"`
		FieldType  string          `json:"field_type"`
		Options    json.RawMessage `json:"options"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid JSON body"})
		return
	}
	cf, err := s.store.CreateCustomField(r.Context(), nil, body.TargetType, body.Name, body.FieldType, body.Options)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, cf)
}

func (s *Server) handleUpdateSystemCustomField(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Name    string          `json:"name"`
		Options json.RawMessage `json:"options"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid JSON body"})
		return
	}
	cf, err := s.store.UpdateCustomField(r.Context(), r.PathValue("fieldId"), body.Name, body.Options)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, cf)
}

func (s *Server) handleDeleteSystemCustomField(w http.ResponseWriter, r *http.Request) {
	if err := s.store.DeleteCustomField(r.Context(), r.PathValue("fieldId")); err != nil {
		writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleListSystemTemplates(w http.ResponseWriter, r *http.Request) {
	targetType := r.URL.Query().Get("target_type")
	list, err := s.store.ListTemplates(r.Context(), nil, targetType)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, list)
}

func (s *Server) handleCreateSystemTemplate(w http.ResponseWriter, r *http.Request) {
	var body struct {
		TargetType string               `json:"target_type"`
		Name       string               `json:"name"`
		Rows       []domain.TemplateRow `json:"rows"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid JSON body"})
		return
	}
	tpl, err := s.store.CreateTemplate(r.Context(), nil, body.TargetType, body.Name, body.Rows)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, tpl)
}

func (s *Server) handleGetSystemTemplate(w http.ResponseWriter, r *http.Request) {
	tpl, err := s.store.GetTemplate(r.Context(), r.PathValue("templateId"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, tpl)
}

func (s *Server) handleUpdateSystemTemplate(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Name string               `json:"name"`
		Rows []domain.TemplateRow `json:"rows"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid JSON body"})
		return
	}
	tpl, err := s.store.UpdateTemplate(r.Context(), r.PathValue("templateId"), body.Name, body.Rows)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, tpl)
}

func (s *Server) handleDeleteSystemTemplate(w http.ResponseWriter, r *http.Request) {
	if err := s.store.DeleteTemplate(r.Context(), r.PathValue("templateId")); err != nil {
		writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// Global Groups & System Labels
func (s *Server) handleListGlobalGroups(w http.ResponseWriter, r *http.Request) {
	list, err := s.store.ListGroups(r.Context(), nil)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, list)
}

func (s *Server) handleCreateGlobalGroup(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Name      string   `json:"name"`
		MemberIDs []string `json:"member_ids"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid JSON body"})
		return
	}
	g, err := s.store.CreateGroup(r.Context(), nil, body.Name, body.MemberIDs)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, g)
}

func (s *Server) handleUpdateGlobalGroup(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Name      string   `json:"name"`
		MemberIDs []string `json:"member_ids"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid JSON body"})
		return
	}
	g, err := s.store.UpdateGroup(r.Context(), r.PathValue("groupId"), body.Name, body.MemberIDs)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, g)
}

func (s *Server) handleDeleteGlobalGroup(w http.ResponseWriter, r *http.Request) {
	if err := s.store.DeleteGroup(r.Context(), r.PathValue("groupId")); err != nil {
		writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleListSystemLabels(w http.ResponseWriter, r *http.Request) {
	list, err := s.store.ListLabels(r.Context(), nil)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, list)
}

func (s *Server) handleCreateSystemLabel(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Name  string `json:"name"`
		Color string `json:"color"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid JSON body"})
		return
	}
	l, err := s.store.CreateLabel(r.Context(), nil, body.Name, body.Color)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, l)
}

func (s *Server) handleUpdateSystemLabel(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Name  string `json:"name"`
		Color string `json:"color"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid JSON body"})
		return
	}
	l, err := s.store.UpdateLabel(r.Context(), r.PathValue("labelId"), body.Name, body.Color)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, l)
}

func (s *Server) handleDeleteSystemLabel(w http.ResponseWriter, r *http.Request) {
	if err := s.store.DeleteLabel(r.Context(), r.PathValue("labelId")); err != nil {
		writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// Projects
func (s *Server) handleListProjects(w http.ResponseWriter, r *http.Request) {
	includeNonMember := r.URL.Query().Get("include_non_member") == "true"
	list, err := s.store.ListProjects(r.Context(), currentUser(r).ID, includeNonMember)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, list)
}

func (s *Server) handleCreateProject(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Name        string   `json:"name"`
		Description string   `json:"description"`
		HoursPerDay *float64 `json:"hours_per_day"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid JSON body"})
		return
	}
	p, err := s.store.CreateProject(r.Context(), currentUser(r).ID, body.Name, body.Description, body.HoursPerDay)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, p)
}

func (s *Server) handleGetProject(w http.ResponseWriter, r *http.Request) {
	u := currentUser(r)
	projectID := r.PathValue("projectId")
	p, err := s.store.GetProject(r.Context(), projectID, u.ID)
	if err != nil {
		writeError(w, err)
		return
	}
	canDelete, _ := s.store.HasUserSystemPermission(r.Context(), u.ID, domain.PermDeleteProject)
	isManageSystem, _ := s.store.HasUserSystemPermission(r.Context(), u.ID, domain.PermManageSystem)
	if isManageSystem {
		p.CurrentUserRole = domain.RoleAdmin
		p.IsMember = true
	} else if !p.IsMember && !canDelete {
		writeJSON(w, http.StatusForbidden, map[string]any{"error": "not a member of this project"})
		return
	}
	writeJSON(w, http.StatusOK, p)
}

func (s *Server) handleUpdateProject(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Name                   string  `json:"name"`
		Description            string  `json:"description"`
		HoursPerDay            float64 `json:"hours_per_day"`
		DefaultIssueTemplateID *string `json:"default_issue_template_id"`
		DefaultTaskTemplateID  *string `json:"default_task_template_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid JSON body"})
		return
	}
	p, err := s.store.UpdateProject(r.Context(), r.PathValue("projectId"), currentUser(r).ID, body.Name, body.Description, body.HoursPerDay, body.DefaultIssueTemplateID, body.DefaultTaskTemplateID)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, p)
}

func (s *Server) handleDeleteProject(w http.ResponseWriter, r *http.Request) {
	confirmNonMember := r.URL.Query().Get("confirm_non_member") == "true"
	if err := s.store.DeleteProject(r.Context(), r.PathValue("projectId"), currentUser(r).ID, confirmNonMember); err != nil {
		writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// Project Members
func (s *Server) handleListProjectMembers(w http.ResponseWriter, r *http.Request) {
	list, err := s.store.ListProjectMembers(r.Context(), r.PathValue("projectId"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, list)
}

func (s *Server) handleUpsertProjectMember(w http.ResponseWriter, r *http.Request) {
	var body struct {
		SubjectType string             `json:"subject_type"`
		SubjectID   string             `json:"subject_id"`
		Role        domain.ProjectRole `json:"role"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid JSON body"})
		return
	}
	m, err := s.store.UpsertProjectMember(r.Context(), r.PathValue("projectId"), body.SubjectType, body.SubjectID, body.Role)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, m)
}

func (s *Server) handleRemoveProjectMember(w http.ResponseWriter, r *http.Request) {
	if err := s.store.RemoveProjectMember(r.Context(), r.PathValue("projectId"), r.PathValue("memberId")); err != nil {
		writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// Project Groups, Labels, Statuses, Custom Fields, Templates
func (s *Server) handleListProjectGroups(w http.ResponseWriter, r *http.Request) {
	pid := r.PathValue("projectId")
	list, err := s.store.ListGroups(r.Context(), &pid)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, list)
}

func (s *Server) handleCreateProjectGroup(w http.ResponseWriter, r *http.Request) {
	pid := r.PathValue("projectId")
	var body struct {
		Name      string   `json:"name"`
		MemberIDs []string `json:"member_ids"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid JSON body"})
		return
	}
	g, err := s.store.CreateGroup(r.Context(), &pid, body.Name, body.MemberIDs)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, g)
}

func (s *Server) handleUpdateProjectGroup(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Name      string   `json:"name"`
		MemberIDs []string `json:"member_ids"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid JSON body"})
		return
	}
	g, err := s.store.UpdateGroup(r.Context(), r.PathValue("groupId"), body.Name, body.MemberIDs)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, g)
}

func (s *Server) handleDeleteProjectGroup(w http.ResponseWriter, r *http.Request) {
	if err := s.store.DeleteGroup(r.Context(), r.PathValue("groupId")); err != nil {
		writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleListProjectLabels(w http.ResponseWriter, r *http.Request) {
	pid := r.PathValue("projectId")
	list, err := s.store.ListLabels(r.Context(), &pid)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, list)
}

func (s *Server) handleCreateProjectLabel(w http.ResponseWriter, r *http.Request) {
	pid := r.PathValue("projectId")
	var body struct {
		Name  string `json:"name"`
		Color string `json:"color"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid JSON body"})
		return
	}
	l, err := s.store.CreateLabel(r.Context(), &pid, body.Name, body.Color)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, l)
}

func (s *Server) handleUpdateProjectLabel(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Name  string `json:"name"`
		Color string `json:"color"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid JSON body"})
		return
	}
	l, err := s.store.UpdateLabel(r.Context(), r.PathValue("labelId"), body.Name, body.Color)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, l)
}

func (s *Server) handleDeleteProjectLabel(w http.ResponseWriter, r *http.Request) {
	if err := s.store.DeleteLabel(r.Context(), r.PathValue("labelId")); err != nil {
		writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleListProjectStatuses(w http.ResponseWriter, r *http.Request) {
	pid := r.PathValue("projectId")
	list, err := s.store.ListStatuses(r.Context(), &pid)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, list)
}

func (s *Server) handleCreateProjectStatus(w http.ResponseWriter, r *http.Request) {
	pid := r.PathValue("projectId")
	var body struct {
		DisplayOrder int    `json:"display_order"`
		Name         string `json:"name"`
		Color        string `json:"color"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid JSON body"})
		return
	}
	st, err := s.store.CreateStatus(r.Context(), &pid, body.DisplayOrder, body.Name, body.Color)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, st)
}

func (s *Server) handleUpdateProjectStatus(w http.ResponseWriter, r *http.Request) {
	var body struct {
		DisplayOrder int    `json:"display_order"`
		Name         string `json:"name"`
		Color        string `json:"color"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid JSON body"})
		return
	}
	st, err := s.store.UpdateStatus(r.Context(), r.PathValue("statusId"), body.DisplayOrder, body.Name, body.Color)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, st)
}

func (s *Server) handleDeleteProjectStatus(w http.ResponseWriter, r *http.Request) {
	pid := r.PathValue("projectId")
	var migrateTo *string
	if m := r.URL.Query().Get("migrate_to_status_id"); m != "" {
		migrateTo = &m
	}
	if err := s.store.DeleteStatus(r.Context(), &pid, r.PathValue("statusId"), migrateTo); err != nil {
		writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleListProjectCustomFields(w http.ResponseWriter, r *http.Request) {
	pid := r.PathValue("projectId")
	targetType := r.URL.Query().Get("target_type")
	list, err := s.store.ListCustomFields(r.Context(), &pid, targetType)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, list)
}

func (s *Server) handleCreateProjectCustomField(w http.ResponseWriter, r *http.Request) {
	pid := r.PathValue("projectId")
	var body struct {
		TargetType string          `json:"target_type"`
		Name       string          `json:"name"`
		FieldType  string          `json:"field_type"`
		Options    json.RawMessage `json:"options"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid JSON body"})
		return
	}
	cf, err := s.store.CreateCustomField(r.Context(), &pid, body.TargetType, body.Name, body.FieldType, body.Options)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, cf)
}

func (s *Server) handleUpdateProjectCustomField(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Name    string          `json:"name"`
		Options json.RawMessage `json:"options"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid JSON body"})
		return
	}
	cf, err := s.store.UpdateCustomField(r.Context(), r.PathValue("fieldId"), body.Name, body.Options)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, cf)
}

func (s *Server) handleDeleteProjectCustomField(w http.ResponseWriter, r *http.Request) {
	if err := s.store.DeleteCustomField(r.Context(), r.PathValue("fieldId")); err != nil {
		writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleListProjectTemplates(w http.ResponseWriter, r *http.Request) {
	pid := r.PathValue("projectId")
	targetType := r.URL.Query().Get("target_type")
	list, err := s.store.ListTemplates(r.Context(), &pid, targetType)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, list)
}

func (s *Server) handleCreateProjectTemplate(w http.ResponseWriter, r *http.Request) {
	pid := r.PathValue("projectId")
	var body struct {
		TargetType string               `json:"target_type"`
		Name       string               `json:"name"`
		Rows       []domain.TemplateRow `json:"rows"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid JSON body"})
		return
	}
	tpl, err := s.store.CreateTemplate(r.Context(), &pid, body.TargetType, body.Name, body.Rows)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, tpl)
}

func (s *Server) handleGetProjectTemplate(w http.ResponseWriter, r *http.Request) {
	tpl, err := s.store.GetTemplate(r.Context(), r.PathValue("templateId"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, tpl)
}

func (s *Server) handleUpdateProjectTemplate(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Name string               `json:"name"`
		Rows []domain.TemplateRow `json:"rows"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid JSON body"})
		return
	}
	tpl, err := s.store.UpdateTemplate(r.Context(), r.PathValue("templateId"), body.Name, body.Rows)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, tpl)
}

func (s *Server) handleDeleteProjectTemplate(w http.ResponseWriter, r *http.Request) {
	if err := s.store.DeleteTemplate(r.Context(), r.PathValue("templateId")); err != nil {
		writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// Issues & Tasks Request Structs

type CreateIssueRequest struct {
	TemplateID        *string                    `json:"template_id"`
	TypeID            *string                    `json:"type_id"`
	Title             string                     `json:"title"`
	Description       string                     `json:"description"`
	LabelIDs          []string                   `json:"label_ids"`
	CustomValues      map[string]json.RawMessage `json:"custom_values"`
	CreateInitialTask bool                       `json:"create_initial_task"`
}

type UpdateIssueRequest struct {
	TemplateID   *string                    `json:"template_id"`
	TypeID       *string                    `json:"type_id"`
	State        *string                    `json:"state"`
	Title        *string                    `json:"title"`
	Description  *string                    `json:"description"`
	LabelIDs     *[]string                  `json:"label_ids"`
	CustomValues map[string]json.RawMessage `json:"custom_values"`
	DisplayOrder *int                       `json:"display_order"`
}

type CreateTaskRequest struct {
	IssueID          string                     `json:"issue_id"`
	TemplateID       *string                    `json:"template_id"`
	TypeID           *string                    `json:"type_id"`
	StatusID         *string                    `json:"status_id"`
	Title            string                     `json:"title"`
	Description      string                     `json:"description"`
	Progress         int                        `json:"progress"`
	PlannedStartDate *string                    `json:"planned_start_date"`
	PlannedEndDate   *string                    `json:"planned_end_date"`
	ActualStartDate  *string                    `json:"actual_start_date"`
	ActualEndDate    *string                    `json:"actual_end_date"`
	PlannedHours     *float64                   `json:"planned_hours"`
	ActualHours      *float64                   `json:"actual_hours"`
	PlannedCost      *float64                   `json:"planned_cost"`
	ActualCost       *float64                   `json:"actual_cost"`
	StoryPoints      *float64                   `json:"story_points"`
	Assignees        []domain.SubjectRef        `json:"assignees"`
	LabelIDs         []string                   `json:"label_ids"`
	CustomValues     map[string]json.RawMessage `json:"custom_values"`
}

type UpdateTaskRequest struct {
	IssueID          *string                    `json:"issue_id"`
	TemplateID       *string                    `json:"template_id"`
	TypeID           *string                    `json:"type_id"`
	StatusID         *string                    `json:"status_id"`
	Title            *string                    `json:"title"`
	Description      *string                    `json:"description"`
	Progress         *int                       `json:"progress"`
	PlannedStartDate *string                    `json:"planned_start_date"`
	PlannedEndDate   *string                    `json:"planned_end_date"`
	ActualStartDate  *string                    `json:"actual_start_date"`
	ActualEndDate    *string                    `json:"actual_end_date"`
	Assignees        *[]domain.SubjectRef       `json:"assignees"`
	LabelIDs         *[]string                  `json:"label_ids"`
	CustomValues     map[string]json.RawMessage `json:"custom_values"`
	KanbanOrder      *int                       `json:"kanban_order"`
	GanttOrder       *int                       `json:"gantt_order"`
}

// Issues & Tasks
func (s *Server) handleListIssues(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	var labelIDs []string
	if rawLabels := strings.TrimSpace(q.Get("label_ids")); rawLabels != "" {
		for _, p := range strings.Split(rawLabels, ",") {
			if trimmed := strings.TrimSpace(p); trimmed != "" {
				labelIDs = append(labelIDs, trimmed)
			}
		}
	}
	issueState := q.Get("state")
	if issueState == "" {
		issueState = q.Get("issue_state")
	}
	filter := store.IssueTaskFilter{
		LabelIDs:   labelIDs,
		AssigneeID: q.Get("assignee_id"),
		StatusID:   q.Get("status_id"),
		TypeID:     q.Get("type_id"),
		IssueState: issueState,
		Keyword:    q.Get("keyword"),
	}
	issues, err := s.store.ListIssuesWithTasks(r.Context(), r.PathValue("projectId"), currentUser(r).ID, filter)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, issues)
}

func (s *Server) handleCreateIssue(w http.ResponseWriter, r *http.Request) {
	var body CreateIssueRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid JSON body"})
		return
	}
	iss, err := s.store.CreateIssue(r.Context(), r.PathValue("projectId"), currentUser(r).ID, store.IssueCreateInput{
		TemplateID:        body.TemplateID,
		TypeID:            body.TypeID,
		Title:             body.Title,
		Description:       body.Description,
		LabelIDs:          body.LabelIDs,
		CustomValues:      body.CustomValues,
		CreateInitialTask: body.CreateInitialTask,
	})
	if err != nil {
		writeError(w, err)
		return
	}
	pid := r.PathValue("projectId")
	s.store.DispatchWebhooks(r.Context(), &pid, domain.WebhookEventIssueCreated, iss)
	writeJSON(w, http.StatusCreated, iss)
}

func (s *Server) handleGetIssue(w http.ResponseWriter, r *http.Request) {
	iss, err := s.store.GetIssue(r.Context(), r.PathValue("projectId"), r.PathValue("issueId"), currentUser(r).ID)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, iss)
}

func (s *Server) handleUpdateIssue(w http.ResponseWriter, r *http.Request) {
	bodyBytes, err := io.ReadAll(r.Body)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "failed to read body"})
		return
	}
	var rawMap map[string]json.RawMessage
	if err := json.Unmarshal(bodyBytes, &rawMap); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid JSON body"})
		return
	}
	var body UpdateIssueRequest
	if err := json.Unmarshal(bodyBytes, &body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid JSON body"})
		return
	}
	in := store.IssueUpdateInput{
		TemplateID:   body.TemplateID,
		TypeID:       body.TypeID,
		State:        body.State,
		Title:        body.Title,
		Description:  body.Description,
		LabelIDs:     body.LabelIDs,
		CustomValues: body.CustomValues,
		DisplayOrder: body.DisplayOrder,
	}
	if raw, ok := rawMap["type_id"]; ok {
		s := strings.TrimSpace(string(raw))
		if s == "null" || s == `""` {
			in.ClearTypeID = true
			in.TypeID = nil
		}
	}
	iss, err := s.store.UpdateIssue(r.Context(), r.PathValue("projectId"), r.PathValue("issueId"), currentUser(r).ID, in)
	if err != nil {
		writeError(w, err)
		return
	}
	pid := r.PathValue("projectId")
	s.store.DispatchWebhooks(r.Context(), &pid, domain.WebhookEventIssueStateChanged, iss)
	writeJSON(w, http.StatusOK, iss)
}

func (s *Server) handleTransferIssueOwner(w http.ResponseWriter, r *http.Request) {
	var body struct {
		OwnerUserID string `json:"owner_user_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid JSON body"})
		return
	}
	iss, err := s.store.TransferIssueOwner(r.Context(), r.PathValue("projectId"), r.PathValue("issueId"), currentUser(r).ID, body.OwnerUserID)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, iss)
}

func (s *Server) handleDeleteIssue(w http.ResponseWriter, r *http.Request) {
	deletedTasks, err := s.store.DeleteIssue(r.Context(), r.PathValue("projectId"), r.PathValue("issueId"), currentUser(r).ID)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"deleted_tasks_count": deletedTasks})
}

func (s *Server) handleCreateTask(w http.ResponseWriter, r *http.Request) {
	var body CreateTaskRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid JSON body"})
		return
	}
	t, err := s.store.CreateTask(r.Context(), r.PathValue("projectId"), currentUser(r).ID, store.TaskCreateInput{
		IssueID:          body.IssueID,
		TemplateID:       body.TemplateID,
		TypeID:           body.TypeID,
		StatusID:         body.StatusID,
		Title:            body.Title,
		Description:      body.Description,
		Progress:         body.Progress,
		PlannedStartDate: body.PlannedStartDate,
		PlannedEndDate:   body.PlannedEndDate,
		ActualStartDate:  body.ActualStartDate,
		ActualEndDate:    body.ActualEndDate,
		PlannedHours:     body.PlannedHours,
		ActualHours:      body.ActualHours,
		PlannedCost:      body.PlannedCost,
		ActualCost:       body.ActualCost,
		StoryPoints:      body.StoryPoints,
		Assignees:        body.Assignees,
		LabelIDs:         body.LabelIDs,
		CustomValues:     body.CustomValues,
	})
	if err != nil {
		writeError(w, err)
		return
	}
	pid := r.PathValue("projectId")
	s.store.DispatchWebhooks(r.Context(), &pid, domain.WebhookEventTaskCreated, t)
	writeJSON(w, http.StatusCreated, t)
}

func (s *Server) handleGetTask(w http.ResponseWriter, r *http.Request) {
	t, err := s.store.GetTask(r.Context(), r.PathValue("projectId"), r.PathValue("taskId"), currentUser(r).ID)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, t)
}

func parseOptionalFloat(raw json.RawMessage) (*float64, bool, error) {
	s := strings.TrimSpace(string(raw))
	if s == "" || s == "null" || s == `""` {
		return nil, true, nil
	}
	var f float64
	if err := json.Unmarshal(raw, &f); err != nil {
		var strVal string
		if errStr := json.Unmarshal(raw, &strVal); errStr == nil {
			strVal = strings.TrimSpace(strVal)
			if strVal == "" {
				return nil, true, nil
			}
			if fParsed, errParse := strconv.ParseFloat(strVal, 64); errParse == nil {
				return &fParsed, false, nil
			}
		}
		return nil, false, err
	}
	return &f, false, nil
}

func (s *Server) handleUpdateTask(w http.ResponseWriter, r *http.Request) {
	bodyBytes, err := io.ReadAll(r.Body)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "failed to read body"})
		return
	}
	var rawMap map[string]json.RawMessage
	if err := json.Unmarshal(bodyBytes, &rawMap); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid JSON body"})
		return
	}

	var body UpdateTaskRequest
	if err := json.Unmarshal(bodyBytes, &body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid JSON body"})
		return
	}

	in := store.TaskUpdateInput{
		IssueID:          body.IssueID,
		TemplateID:       body.TemplateID,
		TypeID:           body.TypeID,
		StatusID:         body.StatusID,
		Title:            body.Title,
		Description:      body.Description,
		Progress:         body.Progress,
		PlannedStartDate: body.PlannedStartDate,
		PlannedEndDate:   body.PlannedEndDate,
		ActualStartDate:  body.ActualStartDate,
		ActualEndDate:    body.ActualEndDate,
		Assignees:        body.Assignees,
		LabelIDs:         body.LabelIDs,
		CustomValues:     body.CustomValues,
		KanbanOrder:      body.KanbanOrder,
		GanttOrder:       body.GanttOrder,
	}

	if raw, ok := rawMap["type_id"]; ok {
		s := strings.TrimSpace(string(raw))
		if s == "null" || s == `""` {
			in.ClearTypeID = true
			in.TypeID = nil
		}
	}

	if raw, ok := rawMap["planned_hours"]; ok {
		val, clear, err := parseOptionalFloat(raw)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid planned_hours"})
			return
		}
		in.PlannedHours = val
		in.ClearPlannedHours = clear
	}
	if raw, ok := rawMap["actual_hours"]; ok {
		val, clear, err := parseOptionalFloat(raw)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid actual_hours"})
			return
		}
		in.ActualHours = val
		in.ClearActualHours = clear
	}
	if raw, ok := rawMap["planned_cost"]; ok {
		val, clear, err := parseOptionalFloat(raw)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid planned_cost"})
			return
		}
		in.PlannedCost = val
		in.ClearPlannedCost = clear
	}
	if raw, ok := rawMap["actual_cost"]; ok {
		val, clear, err := parseOptionalFloat(raw)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid actual_cost"})
			return
		}
		in.ActualCost = val
		in.ClearActualCost = clear
	}

	t, err := s.store.UpdateTask(r.Context(), r.PathValue("projectId"), r.PathValue("taskId"), currentUser(r).ID, in)
	if err != nil {
		writeError(w, err)
		return
	}
	pid := r.PathValue("projectId")
	s.store.DispatchWebhooks(r.Context(), &pid, domain.WebhookEventTaskUpdated, t)
	if in.StatusID != nil {
		s.store.DispatchWebhooks(r.Context(), &pid, domain.WebhookEventTaskStatusChanged, t)
	}
	writeJSON(w, http.StatusOK, t)
}

func (s *Server) handleTransferTaskOwner(w http.ResponseWriter, r *http.Request) {
	var body struct {
		OwnerUserID string `json:"owner_user_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid JSON body"})
		return
	}
	t, err := s.store.TransferTaskOwner(r.Context(), r.PathValue("projectId"), r.PathValue("taskId"), currentUser(r).ID, body.OwnerUserID)
	if err != nil {
		writeError(w, err)
		return
	}
	pid := r.PathValue("projectId")
	s.store.DispatchWebhooks(r.Context(), &pid, domain.WebhookEventTaskUpdated, t)
	writeJSON(w, http.StatusOK, t)
}

func (s *Server) handleMoveTaskKanban(w http.ResponseWriter, r *http.Request) {
	var body struct {
		IssueID     string `json:"issue_id"`
		StatusID    string `json:"status_id"`
		KanbanOrder int    `json:"kanban_order"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid JSON body"})
		return
	}
	t, err := s.store.MoveTaskKanban(r.Context(), r.PathValue("projectId"), r.PathValue("taskId"), currentUser(r).ID, body.IssueID, body.StatusID, body.KanbanOrder)
	if err != nil {
		writeError(w, err)
		return
	}
	pid := r.PathValue("projectId")
	s.store.DispatchWebhooks(r.Context(), &pid, domain.WebhookEventTaskUpdated, t)
	s.store.DispatchWebhooks(r.Context(), &pid, domain.WebhookEventTaskStatusChanged, t)
	writeJSON(w, http.StatusOK, t)
}

func (s *Server) handleUpdateTaskGanttDates(w http.ResponseWriter, r *http.Request) {
	var body struct {
		PlannedStartDate *string `json:"planned_start_date"`
		PlannedEndDate   *string `json:"planned_end_date"`
		ActualStartDate  *string `json:"actual_start_date"`
		ActualEndDate    *string `json:"actual_end_date"`
		GanttOrder       *int    `json:"gantt_order"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid JSON body"})
		return
	}
	t, err := s.store.UpdateTaskGanttDates(r.Context(), r.PathValue("projectId"), r.PathValue("taskId"), currentUser(r).ID, body.PlannedStartDate, body.PlannedEndDate, body.ActualStartDate, body.ActualEndDate, body.GanttOrder)
	if err != nil {
		writeError(w, err)
		return
	}
	pid := r.PathValue("projectId")
	s.store.DispatchWebhooks(r.Context(), &pid, domain.WebhookEventTaskUpdated, t)
	writeJSON(w, http.StatusOK, t)
}

func (s *Server) handleDeleteTask(w http.ResponseWriter, r *http.Request) {
	if err := s.store.DeleteTask(r.Context(), r.PathValue("projectId"), r.PathValue("taskId"), currentUser(r).ID); err != nil {
		writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// Comments & Attachments
func normalizeTargetType(raw string) (string, error) {
	switch raw {
	case "issues", "issue":
		return domain.TargetIssue, nil
	case "tasks", "task":
		return domain.TargetTask, nil
	default:
		return "", fmt.Errorf("%w: targetType must be 'issues' or 'tasks'", store.ErrBadRequest)
	}
}

func (s *Server) handleListComments(w http.ResponseWriter, r *http.Request) {
	targetType, err := normalizeTargetType(r.PathValue("targetType"))
	if err != nil {
		writeError(w, err)
		return
	}
	list, err := s.store.ListComments(r.Context(), r.PathValue("projectId"), targetType, r.PathValue("targetId"), currentUser(r).ID)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, list)
}

func (s *Server) handleCreateComment(w http.ResponseWriter, r *http.Request) {
	targetType, err := normalizeTargetType(r.PathValue("targetType"))
	if err != nil {
		writeError(w, err)
		return
	}
	var body struct {
		Body     string              `json:"body"`
		Mentions []domain.SubjectRef `json:"mentions"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid JSON body"})
		return
	}
	c, err := s.store.CreateComment(r.Context(), r.PathValue("projectId"), targetType, r.PathValue("targetId"), currentUser(r).ID, body.Body, body.Mentions)
	if err != nil {
		writeError(w, err)
		return
	}
	pid := r.PathValue("projectId")
	s.store.DispatchWebhooks(r.Context(), &pid, domain.WebhookEventCommentCreated, c)
	writeJSON(w, http.StatusCreated, c)
}

func (s *Server) handleUpdateComment(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Body     string              `json:"body"`
		Mentions []domain.SubjectRef `json:"mentions"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid JSON body"})
		return
	}
	c, err := s.store.UpdateComment(r.Context(), r.PathValue("projectId"), r.PathValue("commentId"), currentUser(r).ID, body.Body, body.Mentions)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, c)
}

func (s *Server) handleDeleteComment(w http.ResponseWriter, r *http.Request) {
	if err := s.store.DeleteComment(r.Context(), r.PathValue("projectId"), r.PathValue("commentId"), currentUser(r).ID); err != nil {
		writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleListAttachments(w http.ResponseWriter, r *http.Request) {
	targetType, err := normalizeTargetType(r.PathValue("targetType"))
	if err != nil {
		writeError(w, err)
		return
	}
	list, err := s.store.ListAttachments(r.Context(), r.PathValue("projectId"), targetType, r.PathValue("targetId"), currentUser(r).ID)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, list)
}

func (s *Server) handleUploadAttachment(w http.ResponseWriter, r *http.Request) {
	targetType, err := normalizeTargetType(r.PathValue("targetType"))
	if err != nil {
		writeError(w, err)
		return
	}

	ct := r.Header.Get("Content-Type")
	if strings.HasPrefix(ct, "multipart/form-data") {
		mr, err := r.MultipartReader()
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid multipart request"})
			return
		}
		for {
			part, err := mr.NextPart()
			if err != nil {
				writeJSON(w, http.StatusBadRequest, map[string]any{"error": "no file part found in multipart upload"})
				return
			}
			if part.FormName() == "file" {
				fileName := part.FileName()
				mimeType := part.Header.Get("Content-Type")
				att, upErr := s.store.UploadAttachment(r.Context(), r.PathValue("projectId"), targetType, r.PathValue("targetId"), currentUser(r).ID, fileName, mimeType, part)
				_ = part.Close()
				if upErr != nil {
					writeError(w, upErr)
					return
				}
				pid := r.PathValue("projectId")
				s.store.DispatchWebhooks(r.Context(), &pid, domain.WebhookEventAttachmentCreated, att)
				writeJSON(w, http.StatusCreated, att)
				return
			}
			_ = part.Close()
		}
	}

	fileName := r.URL.Query().Get("file_name")
	if fileName == "" {
		fileName = r.Header.Get("X-File-Name")
	}
	att, err := s.store.UploadAttachment(r.Context(), r.PathValue("projectId"), targetType, r.PathValue("targetId"), currentUser(r).ID, fileName, ct, r.Body)
	if err != nil {
		writeError(w, err)
		return
	}
	pid := r.PathValue("projectId")
	s.store.DispatchWebhooks(r.Context(), &pid, domain.WebhookEventAttachmentCreated, att)
	writeJSON(w, http.StatusCreated, att)
}

func (s *Server) handleDownloadAttachment(w http.ResponseWriter, r *http.Request) {
	// First verify metadata before streaming headers
	att, err := s.store.DownloadAttachment(r.Context(), r.PathValue("projectId"), r.PathValue("attachmentId"), currentUser(r).ID, nil)
	if err != nil {
		writeError(w, err)
		return
	}
	w.Header().Set("Content-Type", att.MimeType)
	w.Header().Set("Content-Length", fmt.Sprintf("%d", att.SizeBytes))
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", att.FileName))
	w.WriteHeader(http.StatusOK)
	_, _ = s.store.DownloadAttachment(r.Context(), r.PathValue("projectId"), r.PathValue("attachmentId"), currentUser(r).ID, w)
}

func (s *Server) handleDeleteAttachment(w http.ResponseWriter, r *http.Request) {
	if err := s.store.DeleteAttachment(r.Context(), r.PathValue("projectId"), r.PathValue("attachmentId"), currentUser(r).ID); err != nil {
		writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// Notifications & FCM
func (s *Server) handleListNotifications(w http.ResponseWriter, r *http.Request) {
	unreadOnly := r.URL.Query().Get("unread_only") == "true"
	list, err := s.store.ListNotifications(r.Context(), currentUser(r).ID, unreadOnly)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, list)
}

func (s *Server) handleMarkNotificationRead(w http.ResponseWriter, r *http.Request) {
	if err := s.store.MarkNotificationRead(r.Context(), currentUser(r).ID, r.PathValue("notificationId")); err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"status": "read"})
}

func (s *Server) handleMarkAllNotificationsRead(w http.ResponseWriter, r *http.Request) {
	if err := s.store.MarkAllNotificationsRead(r.Context(), currentUser(r).ID); err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"status": "all_read"})
}

func (s *Server) handleRegisterFCMToken(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Token      string `json:"token"`
		DeviceName string `json:"device_name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid JSON body"})
		return
	}
	if err := s.store.RegisterFCMToken(r.Context(), currentUser(r).ID, body.Token, body.DeviceName); err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"status": "registered"})
}

func (s *Server) handleRemoveFCMToken(w http.ResponseWriter, r *http.Request) {
	token := r.URL.Query().Get("token")
	if token == "" {
		var body struct {
			Token string `json:"token"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		token = body.Token
	}
	if err := s.store.RemoveFCMToken(r.Context(), currentUser(r).ID, token); err != nil {
		writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

