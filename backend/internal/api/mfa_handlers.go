package api

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/takayoshi-u/polaris/backend/internal/mfa"
)

func getRPIDFromRequest(r *http.Request) string {
	host := r.Host
	if strings.Contains(host, ":") {
		host = strings.Split(host, ":")[0]
	}
	if host == "" {
		return "localhost"
	}
	return host
}

func decodeBase64Flexible(s string) ([]byte, error) {
	s = strings.TrimSpace(s)
	if b, err := base64.RawURLEncoding.DecodeString(s); err == nil {
		return b, nil
	}
	if b, err := base64.URLEncoding.DecodeString(s); err == nil {
		return b, nil
	}
	if b, err := base64.RawStdEncoding.DecodeString(s); err == nil {
		return b, nil
	}
	return base64.StdEncoding.DecodeString(s)
}

// POST /api/v1/auth/mfa/verify
func (s *Server) handleMFAVerify(w http.ResponseWriter, r *http.Request) {
	var body struct {
		TempToken string `json:"temp_token"`
		Method    string `json:"method"`
		Code      string `json:"code"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid JSON body"})
		return
	}

	u, sessionToken, err := s.store.VerifyMFA(r.Context(), body.TempToken, body.Method, body.Code)
	if err != nil {
		writeError(w, err)
		return
	}

	setSessionCookie(w, sessionToken)
	writeJSON(w, http.StatusOK, map[string]any{"user": u, "session_token": sessionToken})
}

// GET /api/v1/auth/mfa/status
func (s *Server) handleMFAStatus(w http.ResponseWriter, r *http.Request) {
	u := currentUser(r)
	status, err := s.store.GetMFAStatus(r.Context(), u.ID)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, status)
}

// POST /api/v1/auth/mfa/totp/setup
func (s *Server) handleTOTPSetup(w http.ResponseWriter, r *http.Request) {
	u := currentUser(r)
	secret, err := mfa.GenerateTOTPSecret()
	if err != nil {
		writeError(w, err)
		return
	}

	otpURL := mfa.GenerateTOTPURI(secret, u.Email, "Polaris")
	writeJSON(w, http.StatusOK, map[string]any{
		"secret":      secret,
		"otpauth_url": otpURL,
	})
}

// POST /api/v1/auth/mfa/totp/enable
func (s *Server) handleTOTPEnable(w http.ResponseWriter, r *http.Request) {
	u := currentUser(r)
	var body struct {
		Secret string `json:"secret"`
		Code   string `json:"code"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid JSON body"})
		return
	}

	body.Secret = strings.TrimSpace(body.Secret)
	body.Code = strings.TrimSpace(body.Code)
	if body.Secret == "" || body.Code == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "secret and code are required"})
		return
	}

	if !mfa.ValidateTOTP(body.Secret, body.Code, time.Now()) {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid TOTP passcode"})
		return
	}

	backupCodes, err := s.store.EnableTOTP(r.Context(), u.ID, body.Secret)
	if err != nil {
		writeError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"status":       "enabled",
		"backup_codes": backupCodes,
	})
}

// POST /api/v1/auth/mfa/totp/disable
func (s *Server) handleTOTPDisable(w http.ResponseWriter, r *http.Request) {
	u := currentUser(r)
	var body struct {
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid JSON body"})
		return
	}

	if err := s.store.DisableTOTP(r.Context(), u.ID, body.Password); err != nil {
		writeError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"status": "disabled"})
}

// POST /api/v1/auth/mfa/backup-codes/regenerate
func (s *Server) handleRegenerateBackupCodes(w http.ResponseWriter, r *http.Request) {
	u := currentUser(r)
	var body struct {
		Password string `json:"password"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)

	codes, err := s.store.RegenerateBackupCodes(r.Context(), u.ID, body.Password)
	if err != nil {
		writeError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"backup_codes": codes,
	})
}

// POST /api/v1/auth/mfa/webauthn/register-options
func (s *Server) handleWebAuthnRegisterOptions(w http.ResponseWriter, r *http.Request) {
	u := currentUser(r)
	rpID := getRPIDFromRequest(r)

	opts, challenge, err := mfa.GenerateRegistrationOptions(u.ID, u.Email, u.DisplayName, rpID, "Polaris")
	if err != nil {
		writeError(w, err)
		return
	}

	if err := s.store.SetUserWebAuthnRegisterChallenge(r.Context(), u.ID, challenge); err != nil {
		writeError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, opts)
}

// POST /api/v1/auth/mfa/webauthn/register-verify
func (s *Server) handleWebAuthnRegisterVerify(w http.ResponseWriter, r *http.Request) {
	u := currentUser(r)
	var body struct {
		Name              string `json:"name"`
		ClientDataJSON    string `json:"client_data_json"`
		AttestationObject string `json:"attestation_object"`
		Response          *struct {
			ClientDataJSON    string `json:"clientDataJSON"`
			AttestationObject string `json:"attestationObject"`
		} `json:"response"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid JSON body"})
		return
	}

	clientDataStr := body.ClientDataJSON
	attestationStr := body.AttestationObject
	if body.Response != nil {
		if body.Response.ClientDataJSON != "" {
			clientDataStr = body.Response.ClientDataJSON
		}
		if body.Response.AttestationObject != "" {
			attestationStr = body.Response.AttestationObject
		}
	}

	clientDataBytes, err := decodeBase64Flexible(clientDataStr)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid client_data_json encoding"})
		return
	}
	attestationBytes, err := decodeBase64Flexible(attestationStr)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid attestation_object encoding"})
		return
	}

	challenge, err := s.store.GetUserWebAuthnRegisterChallenge(r.Context(), u.ID)
	if err != nil {
		writeError(w, err)
		return
	}

	parsed, err := mfa.ParseAttestation(clientDataBytes, attestationBytes, challenge)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
		return
	}

	backupCodes, err := s.store.SaveWebAuthnCredential(
		r.Context(),
		u.ID,
		body.Name,
		parsed.CredentialID,
		parsed.PublicKey,
		parsed.AAGUID,
		parsed.AttestationType,
		parsed.SignCount,
	)
	if err != nil {
		writeError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"status":       "registered",
		"backup_codes": backupCodes,
	})
}

// POST /api/v1/auth/mfa/webauthn/login-options
func (s *Server) handleWebAuthnLoginOptions(w http.ResponseWriter, r *http.Request) {
	var body struct {
		TempToken string `json:"temp_token"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid JSON body"})
		return
	}

	userID, _, err := s.store.GetMFAChallenge(r.Context(), body.TempToken)
	if err != nil {
		writeError(w, err)
		return
	}

	creds, err := s.store.ListWebAuthnCredentials(r.Context(), userID)
	if err != nil {
		writeError(w, err)
		return
	}

	var allowedCredIDs [][]byte
	for _, c := range creds {
		rawID, err := decodeBase64Flexible(c.CredentialID)
		if err == nil {
			allowedCredIDs = append(allowedCredIDs, rawID)
		}
	}

	rpID := getRPIDFromRequest(r)
	opts, challenge, err := mfa.GenerateRequestOptions(allowedCredIDs, rpID)
	if err != nil {
		writeError(w, err)
		return
	}

	if err := s.store.SetMFAChallengeWebAuthn(r.Context(), body.TempToken, challenge); err != nil {
		writeError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, opts)
}

// POST /api/v1/auth/mfa/webauthn/login-verify
func (s *Server) handleWebAuthnLoginVerify(w http.ResponseWriter, r *http.Request) {
	var body struct {
		TempToken         string `json:"temp_token"`
		CredentialID      string `json:"credential_id"`
		ID                string `json:"id"`
		ClientDataJSON    string `json:"client_data_json"`
		AuthenticatorData string `json:"authenticator_data"`
		Signature         string `json:"signature"`
		Response          *struct {
			ClientDataJSON    string `json:"clientDataJSON"`
			AuthenticatorData string `json:"authenticatorData"`
			Signature         string `json:"signature"`
		} `json:"response"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid JSON body"})
		return
	}

	credIDStr := body.CredentialID
	if credIDStr == "" {
		credIDStr = body.ID
	}
	clientDataStr := body.ClientDataJSON
	authDataStr := body.AuthenticatorData
	sigStr := body.Signature
	if body.Response != nil {
		if body.Response.ClientDataJSON != "" {
			clientDataStr = body.Response.ClientDataJSON
		}
		if body.Response.AuthenticatorData != "" {
			authDataStr = body.Response.AuthenticatorData
		}
		if body.Response.Signature != "" {
			sigStr = body.Response.Signature
		}
	}

	userID, challenge, err := s.store.GetMFAChallenge(r.Context(), body.TempToken)
	if err != nil {
		writeError(w, err)
		return
	}
	if challenge == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "WebAuthn challenge not initialized; call login-options first"})
		return
	}

	rawCredID, err := decodeBase64Flexible(credIDStr)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid credential_id"})
		return
	}
	clientDataBytes, err := decodeBase64Flexible(clientDataStr)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid client_data_json"})
		return
	}
	authDataBytes, err := decodeBase64Flexible(authDataStr)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid authenticator_data"})
		return
	}
	sigBytes, err := decodeBase64Flexible(sigStr)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid signature"})
		return
	}

	credUUID, pubKey, signCount, credUserID, err := s.store.GetWebAuthnCredentialByID(r.Context(), rawCredID)
	if err != nil {
		writeError(w, err)
		return
	}
	if credUserID != userID {
		writeJSON(w, http.StatusForbidden, map[string]any{"error": "credential does not belong to this user"})
		return
	}

	newSignCount, err := mfa.VerifyAssertion(clientDataBytes, authDataBytes, sigBytes, pubKey, challenge, signCount)
	if err != nil {
		writeJSON(w, http.StatusUnauthorized, map[string]any{"error": err.Error()})
		return
	}

	_ = s.store.UpdateWebAuthnUsage(r.Context(), credUUID, newSignCount)
	_, _ = s.store.ConsumeMFAChallenge(r.Context(), body.TempToken)

	sessionToken, err := s.store.CreateSession(r.Context(), userID)
	if err != nil {
		writeError(w, err)
		return
	}

	u, err := s.store.GetUserByID(r.Context(), userID)
	if err != nil {
		writeError(w, err)
		return
	}

	setSessionCookie(w, sessionToken)
	writeJSON(w, http.StatusOK, map[string]any{
		"user":          u,
		"session_token": sessionToken,
	})
}

// DELETE /api/v1/auth/mfa/webauthn/keys/{id}
func (s *Server) handleDeleteWebAuthnKey(w http.ResponseWriter, r *http.Request) {
	u := currentUser(r)
	keyID := r.PathValue("id")
	if err := s.store.DeleteWebAuthnCredential(r.Context(), u.ID, keyID); err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"status": "deleted"})
}

// PUT /api/v1/system/users/{userId}/mfa-enforcement
func (s *Server) handleUpdateUserMFAEnforcement(w http.ResponseWriter, r *http.Request) {
	targetUserID := r.PathValue("userId")
	var body struct {
		MFAEnforced bool `json:"mfa_enforced"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid JSON body"})
		return
	}

	u, err := s.store.UpdateUserMFAEnforcement(r.Context(), targetUserID, body.MFAEnforced)
	if err != nil {
		writeError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, u)
}
