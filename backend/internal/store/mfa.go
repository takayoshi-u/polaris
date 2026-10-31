package store

import (
	"context"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/takayoshi-u/polaris/backend/internal/domain"
	"github.com/takayoshi-u/polaris/backend/internal/mfa"
	"github.com/takayoshi-u/polaris/backend/internal/pkg/crypto"
	"github.com/takayoshi-u/polaris/backend/internal/pkg/id"
)

const MFAChallengeTTL = 5 * time.Minute

// CreateMFAChallenge creates a temporary 5-minute challenge token for 2FA verification.
func (s *Store) CreateMFAChallenge(ctx context.Context, userID string) (string, error) {
	rawToken, tokenHash, err := crypto.GenerateRandomToken(32)
	if err != nil {
		return "", err
	}
	challengeID := id.NewV7String()
	now := time.Now().UTC()
	expiresAt := now.Add(MFAChallengeTTL)

	_, err = s.db.ExecContext(ctx, `
		INSERT INTO mfa_challenges (id, user_id, temp_token_hash, expires_at, created_at)
		VALUES ($1, $2, $3, $4, $5)
	`, challengeID, userID, tokenHash, expiresAt, now)
	if err != nil {
		return "", err
	}
	return rawToken, nil
}

// GetMFAChallenge retrieves the user ID and webauthn challenge from a valid temp token.
func (s *Store) GetMFAChallenge(ctx context.Context, tempToken string) (string, string, error) {
	tokenHash := crypto.HashToken(tempToken)
	var userID string
	var webauthnChallenge sql.NullString
	var expiresAt time.Time

	err := s.db.QueryRowContext(ctx, `
		SELECT user_id, webauthn_challenge, expires_at
		FROM mfa_challenges
		WHERE temp_token_hash = $1
	`, tokenHash).Scan(&userID, &webauthnChallenge, &expiresAt)
	if errors.Is(err, sql.ErrNoRows) {
		return "", "", fmt.Errorf("%w: invalid or expired temp token", ErrUnauthorized)
	}
	if err != nil {
		return "", "", err
	}
	if time.Now().UTC().After(expiresAt) {
		_, _ = s.db.ExecContext(ctx, `DELETE FROM mfa_challenges WHERE temp_token_hash = $1`, tokenHash)
		return "", "", fmt.Errorf("%w: temp token expired", ErrUnauthorized)
	}
	return userID, webauthnChallenge.String, nil
}

// SetMFAChallengeWebAuthn sets the WebAuthn challenge for an active temp_token.
func (s *Store) SetMFAChallengeWebAuthn(ctx context.Context, tempToken, challenge string) error {
	tokenHash := crypto.HashToken(tempToken)
	res, err := s.db.ExecContext(ctx, `
		UPDATE mfa_challenges
		SET webauthn_challenge = $1
		WHERE temp_token_hash = $2 AND expires_at > NOW()
	`, challenge, tokenHash)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("%w: invalid or expired temp token", ErrUnauthorized)
	}
	return nil
}

// SetUserWebAuthnRegisterChallenge stores the registration challenge for a logged-in user.
func (s *Store) SetUserWebAuthnRegisterChallenge(ctx context.Context, userID, challenge string) error {
	tokenHash := "reg_" + userID
	challengeID := id.NewV7String()
	now := time.Now().UTC()
	expiresAt := now.Add(MFAChallengeTTL)

	_, err := s.db.ExecContext(ctx, `
		INSERT INTO mfa_challenges (id, user_id, temp_token_hash, webauthn_challenge, expires_at, created_at)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (temp_token_hash) DO UPDATE
		SET webauthn_challenge = EXCLUDED.webauthn_challenge, expires_at = EXCLUDED.expires_at
	`, challengeID, userID, tokenHash, challenge, expiresAt, now)
	return err
}

// GetUserWebAuthnRegisterChallenge retrieves and clears the user's registration challenge.
func (s *Store) GetUserWebAuthnRegisterChallenge(ctx context.Context, userID string) (string, error) {
	tokenHash := "reg_" + userID
	var challenge string
	var expiresAt time.Time

	err := s.db.QueryRowContext(ctx, `
		SELECT webauthn_challenge, expires_at
		FROM mfa_challenges
		WHERE temp_token_hash = $1
	`, tokenHash).Scan(&challenge, &expiresAt)
	if errors.Is(err, sql.ErrNoRows) || time.Now().UTC().After(expiresAt) {
		return "", fmt.Errorf("%w: webauthn registration challenge expired or not found", ErrBadRequest)
	}
	if err != nil {
		return "", err
	}
	_, _ = s.db.ExecContext(ctx, `DELETE FROM mfa_challenges WHERE temp_token_hash = $1`, tokenHash)
	return challenge, nil
}

// ConsumeMFAChallenge deletes the temporary challenge and returns the user ID.
func (s *Store) ConsumeMFAChallenge(ctx context.Context, tempToken string) (string, error) {
	tokenHash := crypto.HashToken(tempToken)
	var userID string
	var expiresAt time.Time

	err := s.db.QueryRowContext(ctx, `
		DELETE FROM mfa_challenges
		WHERE temp_token_hash = $1
		RETURNING user_id, expires_at
	`, tokenHash).Scan(&userID, &expiresAt)
	if errors.Is(err, sql.ErrNoRows) {
		return "", fmt.Errorf("%w: invalid or expired temp token", ErrUnauthorized)
	}
	if err != nil {
		return "", err
	}
	if time.Now().UTC().After(expiresAt) {
		return "", fmt.Errorf("%w: temp token expired", ErrUnauthorized)
	}
	return userID, nil
}

// GetMFAStatus returns the user's full MFA status including keys and policy info.
func (s *Store) GetMFAStatus(ctx context.Context, userID string) (*domain.MFAStatus, error) {
	var mfaEnabled, mfaEnforced bool
	var totpSecret sql.NullString
	var backupJSON []byte

	err := s.db.QueryRowContext(ctx, `
		SELECT mfa_enabled, mfa_enforced, totp_secret, mfa_backup_codes
		FROM users WHERE id = $1
	`, userID).Scan(&mfaEnabled, &mfaEnforced, &totpSecret, &backupJSON)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}

	sysSettings, err := s.GetSystemSettings(ctx)
	systemMFARequired := false
	if err == nil && sysSettings != nil {
		systemMFARequired = sysSettings.RequireMFA
	}

	creds, err := s.ListWebAuthnCredentials(ctx, userID)
	if err != nil {
		return nil, err
	}

	var backupEntries []domain.BackupCodeEntry
	if len(backupJSON) > 0 {
		_ = json.Unmarshal(backupJSON, &backupEntries)
	}
	remainingCodes := 0
	for _, entry := range backupEntries {
		if !entry.Used {
			remainingCodes++
		}
	}

	hasTOTP := totpSecret.Valid && strings.TrimSpace(totpSecret.String) != ""

	return &domain.MFAStatus{
		MFAEnabled:           mfaEnabled,
		MFAEnforced:          mfaEnforced,
		SystemMFARequired:    systemMFARequired,
		HasTOTP:              hasTOTP,
		HasWebAuthn:          len(creds) > 0,
		WebAuthnCredentials:  creds,
		RemainingBackupCodes: remainingCodes,
	}, nil
}

// EnableTOTP activates TOTP for the user, sets mfa_enabled = true, and generates backup codes if needed.
func (s *Store) EnableTOTP(ctx context.Context, userID, secret string) ([]string, error) {
	secret = strings.TrimSpace(secret)
	if secret == "" {
		return nil, fmt.Errorf("%w: secret is required", ErrBadRequest)
	}

	var existingBackup []byte
	err := s.db.QueryRowContext(ctx, `SELECT mfa_backup_codes FROM users WHERE id = $1`, userID).Scan(&existingBackup)
	if err != nil {
		return nil, err
	}

	var backupEntries []domain.BackupCodeEntry
	if len(existingBackup) > 0 {
		_ = json.Unmarshal(existingBackup, &backupEntries)
	}

	var plainCodes []string
	if len(backupEntries) == 0 {
		plainCodes, backupEntries, err = mfa.GenerateBackupCodes(8)
		if err != nil {
			return nil, err
		}
	}

	rawBackup, _ := json.Marshal(backupEntries)

	now := time.Now().UTC()
	_, err = s.db.ExecContext(ctx, `
		UPDATE users
		SET totp_secret = $1, mfa_enabled = TRUE, mfa_backup_codes = $2, updated_at = $3
		WHERE id = $4
	`, secret, rawBackup, now, userID)
	if err != nil {
		return nil, err
	}

	return plainCodes, nil
}

// DisableTOTP deactivates TOTP after verifying user's password.
// If MFA is enforced by system or user setting and user has no WebAuthn keys, returns an error.
func (s *Store) DisableTOTP(ctx context.Context, userID, password string) error {
	var pwHash sql.NullString
	var mfaEnforced bool
	err := s.db.QueryRowContext(ctx, `
		SELECT password_hash, mfa_enforced
		FROM users WHERE id = $1
	`, userID).Scan(&pwHash, &mfaEnforced)
	if err != nil {
		return err
	}

	if pwHash.Valid && pwHash.String != "" {
		if !crypto.CheckPassword(password, pwHash.String) {
			return fmt.Errorf("%w: invalid password", ErrUnauthorized)
		}
	}

	// Policy check
	sysSettings, _ := s.GetSystemSettings(ctx)
	systemEnforced := sysSettings != nil && sysSettings.RequireMFA
	if mfaEnforced || systemEnforced {
		creds, _ := s.ListWebAuthnCredentials(ctx, userID)
		if len(creds) == 0 {
			return fmt.Errorf("%w: MFA cannot be disabled because it is required by policy", ErrBadRequest)
		}
	}

	creds, _ := s.ListWebAuthnCredentials(ctx, userID)
	hasWebAuthn := len(creds) > 0

	now := time.Now().UTC()
	_, err = s.db.ExecContext(ctx, `
		UPDATE users
		SET totp_secret = NULL, mfa_enabled = $1, updated_at = $2
		WHERE id = $3
	`, hasWebAuthn, now, userID)
	return err
}

// VerifyMFA validates the submitted TOTP or backup code against the temp_token challenge.
func (s *Store) VerifyMFA(ctx context.Context, tempToken, method, code string) (*domain.User, string, error) {
	code = strings.TrimSpace(code)
	if code == "" {
		return nil, "", fmt.Errorf("%w: verification code is required", ErrBadRequest)
	}

	userID, _, err := s.GetMFAChallenge(ctx, tempToken)
	if err != nil {
		return nil, "", err
	}

	var totpSecret sql.NullString
	var backupJSON []byte
	err = s.db.QueryRowContext(ctx, `
		SELECT totp_secret, mfa_backup_codes
		FROM users WHERE id = $1
	`, userID).Scan(&totpSecret, &backupJSON)
	if err != nil {
		return nil, "", err
	}

	switch method {
	case "totp":
		if !totpSecret.Valid || totpSecret.String == "" {
			return nil, "", fmt.Errorf("%w: TOTP is not configured for this user", ErrBadRequest)
		}
		if !mfa.ValidateTOTP(totpSecret.String, code, time.Now()) {
			return nil, "", fmt.Errorf("%w: invalid TOTP passcode", ErrUnauthorized)
		}

	case "backup_code":
		var entries []domain.BackupCodeEntry
		if len(backupJSON) > 0 {
			_ = json.Unmarshal(backupJSON, &entries)
		}
		matched, updated, err := mfa.VerifyAndUseBackupCode(entries, code)
		if err != nil || !matched {
			return nil, "", fmt.Errorf("%w: invalid or already used backup code", ErrUnauthorized)
		}
		newJSON, _ := json.Marshal(updated)
		_, err = s.db.ExecContext(ctx, `UPDATE users SET mfa_backup_codes = $1 WHERE id = $2`, newJSON, userID)
		if err != nil {
			return nil, "", err
		}

	default:
		return nil, "", fmt.Errorf("%w: unsupported MFA method %q", ErrBadRequest, method)
	}

	// Consume challenge
	_, _ = s.ConsumeMFAChallenge(ctx, tempToken)

	// Issue full session
	sessionToken, err := s.CreateSession(ctx, userID)
	if err != nil {
		return nil, "", err
	}
	u, err := s.GetUserByID(ctx, userID)
	return u, sessionToken, err
}

// WebAuthn Credentials Management
func (s *Store) ListWebAuthnCredentials(ctx context.Context, userID string) ([]*domain.WebAuthnCredential, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, user_id, name, credential_id, attestation_type, aaguid, sign_count, created_at, last_used_at
		FROM webauthn_credentials
		WHERE user_id = $1
		ORDER BY created_at ASC
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var creds []*domain.WebAuthnCredential
	for rows.Next() {
		var c domain.WebAuthnCredential
		var rawCredID, rawAAGUID []byte
		var lastUsed sql.NullTime
		if err := rows.Scan(&c.ID, &c.UserID, &c.Name, &rawCredID, &c.AttestationType, &rawAAGUID, &c.SignCount, &c.CreatedAt, &lastUsed); err != nil {
			return nil, err
		}
		c.CredentialID = base64.RawURLEncoding.EncodeToString(rawCredID)
		if len(rawAAGUID) > 0 {
			c.AAGUID = hex.EncodeToString(rawAAGUID)
		}
		if lastUsed.Valid {
			c.LastUsedAt = &lastUsed.Time
		}
		creds = append(creds, &c)
	}
	if creds == nil {
		creds = []*domain.WebAuthnCredential{}
	}
	return creds, rows.Err()
}

// SaveWebAuthnCredential stores a newly registered security key and activates MFA if not already enabled.
func (s *Store) SaveWebAuthnCredential(ctx context.Context, userID, name string, credID, pubKey, aaguid []byte, attType string, signCount uint32) ([]string, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		name = "Security Key"
	}

	credUUID := id.NewV7String()
	now := time.Now().UTC()

	var existingBackup []byte
	var mfaEnabled bool
	err := s.db.QueryRowContext(ctx, `SELECT mfa_enabled, mfa_backup_codes FROM users WHERE id = $1`, userID).Scan(&mfaEnabled, &existingBackup)
	if err != nil {
		return nil, err
	}

	var backupEntries []domain.BackupCodeEntry
	if len(existingBackup) > 0 {
		_ = json.Unmarshal(existingBackup, &backupEntries)
	}

	var plainCodes []string
	if len(backupEntries) == 0 {
		plainCodes, backupEntries, err = mfa.GenerateBackupCodes(8)
		if err != nil {
			return nil, err
		}
	}
	rawBackup, _ := json.Marshal(backupEntries)

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	_, err = tx.ExecContext(ctx, `
		INSERT INTO webauthn_credentials (id, user_id, name, credential_id, public_key, attestation_type, aaguid, sign_count, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	`, credUUID, userID, name, credID, pubKey, attType, aaguid, signCount, now)
	if err != nil {
		return nil, err
	}

	_, err = tx.ExecContext(ctx, `
		UPDATE users
		SET mfa_enabled = TRUE, mfa_backup_codes = $1, updated_at = $2
		WHERE id = $3
	`, rawBackup, now, userID)
	if err != nil {
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}

	return plainCodes, nil
}

// GetWebAuthnCredentialByID retrieves a specific credential by raw credential_id.
func (s *Store) GetWebAuthnCredentialByID(ctx context.Context, credID []byte) (string, []byte, int64, string, error) {
	var id, userID string
	var pubKey []byte
	var signCount int64

	err := s.db.QueryRowContext(ctx, `
		SELECT id, public_key, sign_count, user_id
		FROM webauthn_credentials
		WHERE credential_id = $1
	`, credID).Scan(&id, &pubKey, &signCount, &userID)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil, 0, "", fmt.Errorf("%w: credential not found", ErrNotFound)
	}
	if err != nil {
		return "", nil, 0, "", err
	}
	return id, pubKey, signCount, userID, nil
}

// UpdateWebAuthnUsage updates the credential's sign count and last used timestamp.
func (s *Store) UpdateWebAuthnUsage(ctx context.Context, credUUID string, signCount uint32) error {
	now := time.Now().UTC()
	_, err := s.db.ExecContext(ctx, `
		UPDATE webauthn_credentials
		SET sign_count = $1, last_used_at = $2
		WHERE id = $3
	`, signCount, now, credUUID)
	return err
}

// DeleteWebAuthnCredential removes a registered security key.
func (s *Store) DeleteWebAuthnCredential(ctx context.Context, userID, credUUID string) error {
	var count int
	var totpSecret sql.NullString
	var mfaEnforced bool

	err := s.db.QueryRowContext(ctx, `
		SELECT mfa_enforced, totp_secret,
		       (SELECT COUNT(*) FROM webauthn_credentials WHERE user_id = $1)
		FROM users WHERE id = $1
	`, userID).Scan(&mfaEnforced, &totpSecret, &count)
	if err != nil {
		return err
	}

	hasTOTP := totpSecret.Valid && totpSecret.String != ""

	// Policy check
	sysSettings, _ := s.GetSystemSettings(ctx)
	systemEnforced := sysSettings != nil && sysSettings.RequireMFA
	if (mfaEnforced || systemEnforced) && count <= 1 && !hasTOTP {
		return fmt.Errorf("%w: cannot delete last MFA method when MFA is required by policy", ErrBadRequest)
	}

	res, err := s.db.ExecContext(ctx, `
		DELETE FROM webauthn_credentials WHERE id = $1 AND user_id = $2
	`, credUUID, userID)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}

	// If no keys left and no TOTP, disable MFA
	if count <= 1 && !hasTOTP {
		now := time.Now().UTC()
		_, _ = s.db.ExecContext(ctx, `UPDATE users SET mfa_enabled = FALSE, updated_at = $1 WHERE id = $2`, now, userID)
	}

	return nil
}

// RegenerateBackupCodes creates a fresh set of 8 backup codes.
func (s *Store) RegenerateBackupCodes(ctx context.Context, userID, password string) ([]string, error) {
	if password != "" {
		var pwHash sql.NullString
		err := s.db.QueryRowContext(ctx, `SELECT password_hash FROM users WHERE id = $1`, userID).Scan(&pwHash)
		if err != nil {
			return nil, err
		}
		if pwHash.Valid && pwHash.String != "" && !crypto.CheckPassword(password, pwHash.String) {
			return nil, fmt.Errorf("%w: invalid password", ErrUnauthorized)
		}
	}

	plainCodes, entries, err := mfa.GenerateBackupCodes(8)
	if err != nil {
		return nil, err
	}
	rawJSON, _ := json.Marshal(entries)
	now := time.Now().UTC()

	_, err = s.db.ExecContext(ctx, `
		UPDATE users SET mfa_backup_codes = $1, updated_at = $2 WHERE id = $3
	`, rawJSON, now, userID)
	if err != nil {
		return nil, err
	}
	return plainCodes, nil
}

// UpdateUserMFAEnforcement sets the individual user's MFA enforcement flag.
func (s *Store) UpdateUserMFAEnforcement(ctx context.Context, userID string, enforced bool) (*domain.User, error) {
	now := time.Now().UTC()
	res, err := s.db.ExecContext(ctx, `
		UPDATE users SET mfa_enforced = $1, updated_at = $2 WHERE id = $3
	`, enforced, now, userID)
	if err != nil {
		return nil, err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return nil, ErrNotFound
	}
	return s.GetUserByID(ctx, userID)
}
