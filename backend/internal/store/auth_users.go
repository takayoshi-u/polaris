package store

import (
	"context"
	"crypto/rand"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/takayoshi-u/polaris/backend/internal/domain"
	"github.com/takayoshi-u/polaris/backend/internal/mail"
	"github.com/takayoshi-u/polaris/backend/internal/pkg/crypto"
	"github.com/takayoshi-u/polaris/backend/internal/pkg/id"
)

const (
	SessionTTL           = 7 * 24 * time.Hour
	InviteTokenTTL       = 72 * time.Hour
	PasswordResetTTL     = 1 * time.Hour
	EmailChangeVerifyTTL = 1 * time.Hour
)

// IsSetupRequired returns true if there are 0 users in the database.
func (s *Store) IsSetupRequired(ctx context.Context) (bool, error) {
	var count int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM users`).Scan(&count); err != nil {
		return false, err
	}
	return count == 0, nil
}

// SetupInitialAdmin creates the first administrator user when no users exist.
// Once 1 user exists, this returns ErrForbidden.
func (s *Store) SetupInitialAdmin(ctx context.Context, email, password, displayName string, googleSub *string) (*domain.User, string, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	if email == "" || !strings.Contains(email, "@") {
		return nil, "", fmt.Errorf("%w: valid email is required", ErrBadRequest)
	}
	if displayName == "" {
		displayName = strings.Split(email, "@")[0]
	}

	var pwHash *string
	if password != "" {
		h, err := crypto.HashPassword(password)
		if err != nil {
			return nil, "", fmt.Errorf("%w: %v", ErrBadRequest, err)
		}
		pwHash = &h
	} else if googleSub == nil || *googleSub == "" {
		return nil, "", fmt.Errorf("%w: password or google authentication is required", ErrBadRequest)
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, "", err
	}
	defer tx.Rollback()

	var count int
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM users`).Scan(&count); err != nil {
		return nil, "", err
	}
	if count > 0 {
		return nil, "", fmt.Errorf("%w: initial setup is already completed", ErrForbidden)
	}

	userID := id.NewV7String()
	now := time.Now().UTC()
	_, err = tx.ExecContext(ctx, `
		INSERT INTO users (id, email, password_hash, google_sub, display_name, status, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $7)
	`, userID, email, pwHash, googleSub, displayName, domain.UserStatusActive, now)
	if err != nil {
		return nil, "", err
	}

	perms := []string{domain.PermManageSystem, domain.PermCreateProject, domain.PermDeleteProject}
	for _, perm := range perms {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO system_permissions (id, subject_type, subject_id, permission, created_at)
			VALUES ($1, $2, $3, $4, $5)
		`, id.NewV7String(), domain.SubjectUser, userID, perm, now); err != nil {
			return nil, "", err
		}
	}

	rawSessionToken, tokenHash, err := crypto.GenerateRandomToken(32)
	if err != nil {
		return nil, "", err
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO user_sessions (id, user_id, token_hash, expires_at, created_at)
		VALUES ($1, $2, $3, $4, $5)
	`, id.NewV7String(), userID, tokenHash, now.Add(SessionTTL), now); err != nil {
		return nil, "", err
	}

	if err := tx.Commit(); err != nil {
		return nil, "", err
	}

	u, err := s.GetUserByID(ctx, userID)
	return u, rawSessionToken, err
}

type LoginResult struct {
	User           *domain.User `json:"user,omitempty"`
	SessionToken   string       `json:"session_token,omitempty"`
	MFARequired    bool         `json:"mfa_required,omitempty"`
	TempToken      string       `json:"temp_token,omitempty"`
	AllowedMethods []string     `json:"allowed_methods,omitempty"`
	HasTOTP        bool         `json:"has_totp,omitempty"`
	HasWebAuthn    bool         `json:"has_webauthn,omitempty"`
}

// AuthenticateWithPassword authenticates an active user with email and password, checking for MFA.
func (s *Store) AuthenticateWithPassword(ctx context.Context, email, password string) (*LoginResult, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	var userID, status string
	var pwHash, totpSecret sql.NullString
	var mfaEnabled bool
	err := s.db.QueryRowContext(ctx, `
		SELECT id, password_hash, status, mfa_enabled, totp_secret
		FROM users WHERE LOWER(email) = $1 AND status != 'disabled'
	`, email).Scan(&userID, &pwHash, &status, &mfaEnabled, &totpSecret)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrUnauthorized
	}
	if err != nil {
		return nil, err
	}
	if status != domain.UserStatusActive {
		return nil, fmt.Errorf("%w: user account is %s", ErrForbidden, status)
	}
	if !pwHash.Valid || !crypto.CheckPassword(password, pwHash.String) {
		return nil, ErrUnauthorized
	}

	if mfaEnabled {
		tempToken, err := s.CreateMFAChallenge(ctx, userID)
		if err != nil {
			return nil, err
		}
		creds, _ := s.ListWebAuthnCredentials(ctx, userID)
		hasTOTP := totpSecret.Valid && strings.TrimSpace(totpSecret.String) != ""
		hasWebAuthn := len(creds) > 0

		return &LoginResult{
			MFARequired:    true,
			TempToken:      tempToken,
			AllowedMethods: []string{"totp", "webauthn", "backup_code"},
			HasTOTP:        hasTOTP,
			HasWebAuthn:    hasWebAuthn,
		}, nil
	}

	token, err := s.CreateSession(ctx, userID)
	if err != nil {
		return nil, err
	}
	u, err := s.GetUserByID(ctx, userID)
	if err != nil {
		return nil, err
	}
	return &LoginResult{
		User:         u,
		SessionToken: token,
	}, nil
}

// LoginWithPassword authenticates an active user with email and password.
func (s *Store) LoginWithPassword(ctx context.Context, email, password string) (*domain.User, string, error) {
	res, err := s.AuthenticateWithPassword(ctx, email, password)
	if err != nil {
		return nil, "", err
	}
	if res.MFARequired {
		return nil, res.TempToken, nil
	}
	return res.User, res.SessionToken, nil
}

// LoginWithGoogle authenticates a pre-registered/active user by matching email (or google_sub).
// If the user is in 'invited' status and logs in with matching Google email, activates them automatically.
func (s *Store) LoginWithGoogle(ctx context.Context, email, googleSub, displayName, avatarURL string) (*domain.User, string, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	if email == "" || googleSub == "" {
		return nil, "", fmt.Errorf("%w: email and google_sub are required", ErrBadRequest)
	}

	// Check if setup is required (0 users) -> auto setup first admin via Google
	setupReq, err := s.IsSetupRequired(ctx)
	if err != nil {
		return nil, "", err
	}
	if setupReq {
		return s.SetupInitialAdmin(ctx, email, "", displayName, &googleSub)
	}

	// Match by email so that if email was changed, matching the new email allows Google login
	var userID, status string
	err = s.db.QueryRowContext(ctx, `
		SELECT id, status FROM users WHERE LOWER(email) = $1 AND status != 'disabled'
	`, email).Scan(&userID, &status)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, "", fmt.Errorf("%w: user is not pre-registered or invited", ErrForbidden)
	}
	if err != nil {
		return nil, "", err
	}
	if status == domain.UserStatusDisabled {
		return nil, "", fmt.Errorf("%w: user account is disabled", ErrForbidden)
	}

	// In case a previously disabled user had this google_sub, unlink it so the active user can link it
	_, _ = s.db.ExecContext(ctx, `UPDATE users SET google_sub = NULL WHERE google_sub = $1 AND status = 'disabled'`, googleSub)

	now := time.Now().UTC()
	_, err = s.db.ExecContext(ctx, `
		UPDATE users
		SET google_sub = $1,
		    status = $2,
		    avatar_url = CASE WHEN avatar_url = '' THEN $3 ELSE avatar_url END,
		    updated_at = $4
		WHERE id = $5
	`, googleSub, domain.UserStatusActive, avatarURL, now, userID)
	if err != nil {
		return nil, "", err
	}

	token, err := s.CreateSession(ctx, userID)
	if err != nil {
		return nil, "", err
	}
	u, err := s.GetUserByID(ctx, userID)
	return u, token, err
}

func (s *Store) CreateSession(ctx context.Context, userID string) (string, error) {
	rawToken, tokenHash, err := crypto.GenerateRandomToken(32)
	if err != nil {
		return "", err
	}
	now := time.Now().UTC()
	_, err = s.db.ExecContext(ctx, `
		INSERT INTO user_sessions (id, user_id, token_hash, expires_at, created_at)
		VALUES ($1, $2, $3, $4, $5)
	`, id.NewV7String(), userID, tokenHash, now.Add(SessionTTL), now)
	if err != nil {
		return "", err
	}
	return rawToken, nil
}

func (s *Store) GetUserBySessionToken(ctx context.Context, rawToken string) (*domain.User, error) {
	if rawToken == "" {
		return nil, ErrUnauthorized
	}
	tokenHash := crypto.HashToken(rawToken)
	var userID string
	var expiresAt time.Time
	err := s.db.QueryRowContext(ctx, `
		SELECT s.user_id, s.expires_at
		FROM user_sessions s
		JOIN users u ON u.id = s.user_id
		WHERE s.token_hash = $1 AND u.status = 'active'
	`, tokenHash).Scan(&userID, &expiresAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrUnauthorized
	}
	if err != nil {
		return nil, err
	}
	if time.Now().UTC().After(expiresAt) {
		_, _ = s.db.ExecContext(ctx, `DELETE FROM user_sessions WHERE token_hash = $1`, tokenHash)
		return nil, ErrUnauthorized
	}
	return s.GetUserByID(ctx, userID)
}

func (s *Store) RevokeSession(ctx context.Context, rawToken string) error {
	if rawToken == "" {
		return nil
	}
	tokenHash := crypto.HashToken(rawToken)
	_, err := s.db.ExecContext(ctx, `DELETE FROM user_sessions WHERE token_hash = $1`, tokenHash)
	return err
}

func (s *Store) RevokeAllUserSessions(ctx context.Context, userID string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM user_sessions WHERE user_id = $1`, userID)
	return err
}

func (s *Store) GetUserByID(ctx context.Context, userID string) (*domain.User, error) {
	var u domain.User
	var pwHash, googleSub sql.NullString
	err := s.db.QueryRowContext(ctx, `
		SELECT id, email, password_hash, google_sub, display_name, avatar_url, status, mfa_enabled, mfa_enforced, must_change_password, created_at, updated_at
		FROM users WHERE id = $1
	`, userID).Scan(&u.ID, &u.Email, &pwHash, &googleSub, &u.DisplayName, &u.AvatarURL, &u.Status, &u.MFAEnabled, &u.MFAEnforced, &u.MustChangePassword, &u.CreatedAt, &u.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	u.HasPassword = pwHash.Valid && pwHash.String != ""
	if googleSub.Valid {
		u.GoogleSub = &googleSub.String
	}
	perms, err := s.GetUserSystemPermissions(ctx, userID)
	if err != nil {
		return nil, err
	}
	u.Permissions = perms
	return &u, nil
}

func (s *Store) ListUsers(ctx context.Context) ([]domain.User, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, email, password_hash, google_sub, display_name, avatar_url, status, mfa_enabled, mfa_enforced, must_change_password, created_at, updated_at
		FROM users ORDER BY id ASC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var users []domain.User
	for rows.Next() {
		var u domain.User
		var pwHash, googleSub sql.NullString
		if err := rows.Scan(&u.ID, &u.Email, &pwHash, &googleSub, &u.DisplayName, &u.AvatarURL, &u.Status, &u.MFAEnabled, &u.MFAEnforced, &u.MustChangePassword, &u.CreatedAt, &u.UpdatedAt); err != nil {
			return nil, err
		}
		u.HasPassword = pwHash.Valid && pwHash.String != ""
		if googleSub.Valid {
			u.GoogleSub = &googleSub.String
		}
		perms, _ := s.GetUserSystemPermissions(ctx, u.ID)
		u.Permissions = perms
		users = append(users, u)
	}
	if users == nil {
		users = []domain.User{}
	}
	return users, rows.Err()
}

// CreateUserDirect registers a new user account directly in 'active' status with an initial password.
func (s *Store) CreateUserDirect(ctx context.Context, email, displayName, password string, mustChangePassword bool, groupIDs []string, permissions []string) (*domain.User, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	allowedDomains, err := s.GetAllowedDomains(ctx)
	if err != nil {
		return nil, err
	}
	if err := domain.ValidateEmailAllowedDomain(email, allowedDomains); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrBadRequest, err)
	}
	displayName = strings.TrimSpace(displayName)
	if displayName == "" {
		displayName = strings.Split(email, "@")[0]
	}
	pwHash, err := crypto.HashPassword(password)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrBadRequest, err)
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	var existing int
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM users WHERE LOWER(email) = $1 AND status != 'disabled'`, email).Scan(&existing); err != nil {
		return nil, err
	}
	if existing > 0 {
		return nil, fmt.Errorf("%w: email is already registered", ErrConflict)
	}

	userID := id.NewV7String()
	now := time.Now().UTC()
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO users (id, email, password_hash, display_name, status, must_change_password, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $7)
	`, userID, email, pwHash, displayName, domain.UserStatusActive, mustChangePassword, now); err != nil {
		return nil, err
	}

	for _, gid := range groupIDs {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO group_members (id, group_id, user_id, created_at)
			VALUES ($1, $2, $3, $4)
			ON CONFLICT DO NOTHING
		`, id.NewV7String(), gid, userID, now); err != nil {
			return nil, err
		}
	}

	for _, perm := range permissions {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO system_permissions (id, subject_type, subject_id, permission, created_at)
			VALUES ($1, $2, $3, $4, $5)
			ON CONFLICT DO NOTHING
		`, id.NewV7String(), domain.SubjectUser, userID, perm, now); err != nil {
			return nil, err
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return s.GetUserByID(ctx, userID)
}

// InviteUser pre-registers a user (checking domain restrictions), creates a 72h invite token, and sends an invite email.
func (s *Store) InviteUser(ctx context.Context, email, displayName string, groupIDs []string, permissions []string) (*domain.User, string, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	allowedDomains, err := s.GetAllowedDomains(ctx)
	if err != nil {
		return nil, "", err
	}
	if err := domain.ValidateEmailAllowedDomain(email, allowedDomains); err != nil {
		return nil, "", fmt.Errorf("%w: %v", ErrBadRequest, err)
	}
	if displayName == "" {
		displayName = strings.Split(email, "@")[0]
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, "", err
	}
	defer tx.Rollback()

	var existing int
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM users WHERE LOWER(email) = $1 AND status != 'disabled'`, email).Scan(&existing); err != nil {
		return nil, "", err
	}
	if existing > 0 {
		return nil, "", fmt.Errorf("%w: email is already registered", ErrConflict)
	}

	userID := id.NewV7String()
	now := time.Now().UTC()
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO users (id, email, display_name, status, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $5)
	`, userID, email, displayName, domain.UserStatusInvited, now); err != nil {
		return nil, "", err
	}

	for _, gid := range groupIDs {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO group_members (id, group_id, user_id, created_at)
			VALUES ($1, $2, $3, $4)
			ON CONFLICT DO NOTHING
		`, id.NewV7String(), gid, userID, now); err != nil {
			return nil, "", err
		}
	}

	for _, perm := range permissions {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO system_permissions (id, subject_type, subject_id, permission, created_at)
			VALUES ($1, $2, $3, $4, $5)
			ON CONFLICT DO NOTHING
		`, id.NewV7String(), domain.SubjectUser, userID, perm, now); err != nil {
			return nil, "", err
		}
	}

	rawToken, tokenHash, err := crypto.GenerateRandomToken(32)
	if err != nil {
		return nil, "", err
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO email_verification_tokens (id, user_id, token_type, token_hash, expires_at, created_at)
		VALUES ($1, $2, 'invite', $3, $4, $5)
	`, id.NewV7String(), userID, tokenHash, now.Add(InviteTokenTTL), now); err != nil {
		return nil, "", err
	}

	if err := tx.Commit(); err != nil {
		return nil, "", err
	}

	inviteURL := fmt.Sprintf("%s/auth/invite?token=%s", strings.TrimRight(s.appBaseURL, "/"), rawToken)
	_ = s.sendSystemEmail(ctx, email, "[Polaris] 招待のご案内", fmt.Sprintf(
		"%s 様\n\nPolarisへ招待されました。以下のリンク（有効期限：72時間）から初期設定を行ってください。\n%s\n",
		displayName, inviteURL,
	))

	u, err := s.GetUserByID(ctx, userID)
	if u != nil {
		u.InviteURL = inviteURL
	}
	return u, rawToken, err
}

// BulkInviteUsers invites multiple users in batch, validating allowed domains and existing users.
func (s *Store) BulkInviteUsers(ctx context.Context, invitations []domain.UserInviteItem, defaultGroupIDs []string, defaultPermissions []string) (*domain.BulkInviteResult, error) {
	allowedDomains, err := s.GetAllowedDomains(ctx)
	if err != nil {
		return nil, err
	}

	result := &domain.BulkInviteResult{
		Total:        len(invitations),
		InvitedUsers: []*domain.User{},
		Failures:     []domain.BulkInviteFailure{},
	}

	seenInBatch := make(map[string]bool)

	for _, inv := range invitations {
		rawEmail := strings.TrimSpace(inv.Email)
		cleanEmail := strings.ToLower(rawEmail)

		if cleanEmail == "" || !strings.Contains(cleanEmail, "@") {
			result.Failures = append(result.Failures, domain.BulkInviteFailure{
				Email:  rawEmail,
				Reason: "メールアドレスの形式が不正です",
			})
			continue
		}

		// Validate domain restriction
		if err := domain.ValidateEmailAllowedDomain(cleanEmail, allowedDomains); err != nil {
			result.Failures = append(result.Failures, domain.BulkInviteFailure{
				Email:  rawEmail,
				Reason: "メールアドレスのドメインが許可されていません",
			})
			continue
		}

		// Duplicate in input batch
		if seenInBatch[cleanEmail] {
			result.Failures = append(result.Failures, domain.BulkInviteFailure{
				Email:  rawEmail,
				Reason: "既に登録されているユーザーです",
			})
			continue
		}
		seenInBatch[cleanEmail] = true

		// Check if already in DB
		var existingCount int
		if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM users WHERE LOWER(email) = $1 AND status != 'disabled'`, cleanEmail).Scan(&existingCount); err != nil {
			result.Failures = append(result.Failures, domain.BulkInviteFailure{
				Email:  rawEmail,
				Reason: "データベースエラーが発生しました",
			})
			continue
		}
		if existingCount > 0 {
			result.Failures = append(result.Failures, domain.BulkInviteFailure{
				Email:  rawEmail,
				Reason: "既に登録されているユーザーです",
			})
			continue
		}

		displayName := strings.TrimSpace(inv.DisplayName)
		if displayName == "" {
			displayName = strings.Split(cleanEmail, "@")[0]
		}

		// Transaction per user
		tx, err := s.db.BeginTx(ctx, nil)
		if err != nil {
			result.Failures = append(result.Failures, domain.BulkInviteFailure{
				Email:  rawEmail,
				Reason: "データベースエラーが発生しました",
			})
			continue
		}

		userID := id.NewV7String()
		now := time.Now().UTC()
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO users (id, email, display_name, status, created_at, updated_at)
			VALUES ($1, $2, $3, $4, $5, $5)
		`, userID, cleanEmail, displayName, domain.UserStatusInvited, now); err != nil {
			_ = tx.Rollback()
			result.Failures = append(result.Failures, domain.BulkInviteFailure{
				Email:  rawEmail,
				Reason: "ユーザー作成に失敗しました",
			})
			continue
		}

		for _, gid := range defaultGroupIDs {
			if gid != "" {
				_, _ = tx.ExecContext(ctx, `
					INSERT INTO group_members (id, group_id, user_id, created_at)
					VALUES ($1, $2, $3, $4)
					ON CONFLICT DO NOTHING
				`, id.NewV7String(), gid, userID, now)
			}
		}

		for _, perm := range defaultPermissions {
			if perm != "" {
				_, _ = tx.ExecContext(ctx, `
					INSERT INTO system_permissions (id, subject_type, subject_id, permission, created_at)
					VALUES ($1, $2, $3, $4, $5)
					ON CONFLICT DO NOTHING
				`, id.NewV7String(), domain.SubjectUser, userID, perm, now)
			}
		}

		rawToken, tokenHash, err := crypto.GenerateRandomToken(32)
		if err != nil {
			_ = tx.Rollback()
			result.Failures = append(result.Failures, domain.BulkInviteFailure{
				Email:  rawEmail,
				Reason: "トークン生成に失敗しました",
			})
			continue
		}

		if _, err := tx.ExecContext(ctx, `
			INSERT INTO email_verification_tokens (id, user_id, token_type, token_hash, expires_at, created_at)
			VALUES ($1, $2, 'invite', $3, $4, $5)
		`, id.NewV7String(), userID, tokenHash, now.Add(InviteTokenTTL), now); err != nil {
			_ = tx.Rollback()
			result.Failures = append(result.Failures, domain.BulkInviteFailure{
				Email:  rawEmail,
				Reason: "トークン保存に失敗しました",
			})
			continue
		}

		if err := tx.Commit(); err != nil {
			result.Failures = append(result.Failures, domain.BulkInviteFailure{
				Email:  rawEmail,
				Reason: "トランザクションコミットに失敗しました",
			})
			continue
		}

		inviteURL := fmt.Sprintf("%s/auth/invite?token=%s", strings.TrimRight(s.appBaseURL, "/"), rawToken)
		_ = s.sendSystemEmail(ctx, cleanEmail, "[Polaris] 招待のご案内", fmt.Sprintf(
			"%s 様\n\nPolarisへ招待されました。以下のリンク（有効期限：72時間）から初期設定を行ってください。\n%s\n",
			displayName, inviteURL,
		))

		result.InvitedUsers = append(result.InvitedUsers, &domain.User{
			ID:          userID,
			Email:       cleanEmail,
			DisplayName: displayName,
			Status:      domain.UserStatusInvited,
			InviteURL:   inviteURL,
			CreatedAt:   now,
			UpdatedAt:   now,
		})
	}

	result.InvitedCount = len(result.InvitedUsers)
	result.FailedCount = len(result.Failures)

	return result, nil
}

func generateSecureTempPassword() string {
	const charset = "ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnpqrstuvwxyz23456789!#$%"
	b := make([]byte, 12)
	if _, err := rand.Read(b); err != nil {
		tok, _, _ := crypto.GenerateRandomToken(9)
		if len(tok) >= 12 {
			return tok[:12]
		}
		return tok
	}
	for i := range b {
		b[i] = charset[int(b[i])%len(charset)]
	}
	return string(b)
}

// BulkCreateDirectUsers directly registers multiple user accounts in active status, assigning initial/temporary passwords.
func (s *Store) BulkCreateDirectUsers(ctx context.Context, req domain.BulkDirectUserRequest) (*domain.BulkDirectUserResult, error) {
	allowedDomains, err := s.GetAllowedDomains(ctx)
	if err != nil {
		return nil, err
	}

	result := &domain.BulkDirectUserResult{
		Total:        len(req.Users),
		CreatedUsers: []domain.BulkDirectUserCredential{},
		Failures:     []domain.BulkDirectUserFailure{},
	}

	seenInBatch := make(map[string]bool)

	for _, item := range req.Users {
		rawEmail := strings.TrimSpace(item.Email)
		cleanEmail := strings.ToLower(rawEmail)

		if cleanEmail == "" || !strings.Contains(cleanEmail, "@") {
			result.Failures = append(result.Failures, domain.BulkDirectUserFailure{
				Email:  rawEmail,
				Reason: "メールアドレスの形式が不正です",
			})
			continue
		}

		if err := domain.ValidateEmailAllowedDomain(cleanEmail, allowedDomains); err != nil {
			result.Failures = append(result.Failures, domain.BulkDirectUserFailure{
				Email:  rawEmail,
				Reason: "メールアドレスのドメインが許可されていません",
			})
			continue
		}

		if seenInBatch[cleanEmail] {
			result.Failures = append(result.Failures, domain.BulkDirectUserFailure{
				Email:  rawEmail,
				Reason: "既に登録されているユーザーです",
			})
			continue
		}
		seenInBatch[cleanEmail] = true

		var existingCount int
		if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM users WHERE LOWER(email) = $1 AND status != 'disabled'`, cleanEmail).Scan(&existingCount); err != nil {
			result.Failures = append(result.Failures, domain.BulkDirectUserFailure{
				Email:  rawEmail,
				Reason: "データベースエラーが発生しました",
			})
			continue
		}
		if existingCount > 0 {
			result.Failures = append(result.Failures, domain.BulkDirectUserFailure{
				Email:  rawEmail,
				Reason: "既に登録されているユーザーです",
			})
			continue
		}

		tempPassword := strings.TrimSpace(item.Password)
		if tempPassword == "" {
			tempPassword = strings.TrimSpace(req.DefaultPassword)
		}
		if tempPassword == "" {
			tempPassword = generateSecureTempPassword()
		}

		pwHash, err := crypto.HashPassword(tempPassword)
		if err != nil {
			result.Failures = append(result.Failures, domain.BulkDirectUserFailure{
				Email:  rawEmail,
				Reason: "パスワードのハッシュ化に失敗しました",
			})
			continue
		}

		displayName := strings.TrimSpace(item.DisplayName)
		if displayName == "" {
			displayName = strings.Split(cleanEmail, "@")[0]
		}

		groupMap := make(map[string]bool)
		for _, g := range req.DefaultGroupIDs {
			if strings.TrimSpace(g) != "" {
				groupMap[strings.TrimSpace(g)] = true
			}
		}
		for _, g := range item.GroupIDs {
			if strings.TrimSpace(g) != "" {
				groupMap[strings.TrimSpace(g)] = true
			}
		}

		permMap := make(map[string]bool)
		for _, p := range req.DefaultPermissions {
			if strings.TrimSpace(p) != "" {
				permMap[strings.TrimSpace(p)] = true
			}
		}
		for _, p := range item.Permissions {
			if strings.TrimSpace(p) != "" {
				permMap[strings.TrimSpace(p)] = true
			}
		}

		tx, err := s.db.BeginTx(ctx, nil)
		if err != nil {
			result.Failures = append(result.Failures, domain.BulkDirectUserFailure{
				Email:  rawEmail,
				Reason: "データベースエラーが発生しました",
			})
			continue
		}

		userID := id.NewV7String()
		now := time.Now().UTC()
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO users (id, email, password_hash, display_name, status, must_change_password, created_at, updated_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $7)
		`, userID, cleanEmail, pwHash, displayName, domain.UserStatusActive, req.MustChangePassword, now); err != nil {
			_ = tx.Rollback()
			result.Failures = append(result.Failures, domain.BulkDirectUserFailure{
				Email:  rawEmail,
				Reason: "ユーザー作成に失敗しました",
			})
			continue
		}

		for gID := range groupMap {
			if _, err := tx.ExecContext(ctx, `
				INSERT INTO group_members (id, group_id, user_id, created_at)
				VALUES ($1, $2, $3, $4)
				ON CONFLICT DO NOTHING
			`, id.NewV7String(), gID, userID, now); err != nil {
				_ = tx.Rollback()
				result.Failures = append(result.Failures, domain.BulkDirectUserFailure{
					Email:  rawEmail,
					Reason: "グループ所属の設定に失敗しました",
				})
				continue
			}
		}

		for perm := range permMap {
			if _, err := tx.ExecContext(ctx, `
				INSERT INTO system_permissions (id, subject_type, subject_id, permission, created_at)
				VALUES ($1, $2, $3, $4, $5)
				ON CONFLICT DO NOTHING
			`, id.NewV7String(), domain.SubjectUser, userID, perm, now); err != nil {
				_ = tx.Rollback()
				result.Failures = append(result.Failures, domain.BulkDirectUserFailure{
					Email:  rawEmail,
					Reason: "権限の設定に失敗しました",
				})
				continue
			}
		}

		if err := tx.Commit(); err != nil {
			result.Failures = append(result.Failures, domain.BulkDirectUserFailure{
				Email:  rawEmail,
				Reason: "トランザクションコミットに失敗しました",
			})
			continue
		}

		u, err := s.GetUserByID(ctx, userID)
		if err != nil {
			result.Failures = append(result.Failures, domain.BulkDirectUserFailure{
				Email:  rawEmail,
				Reason: "ユーザー情報の取得に失敗しました",
			})
			continue
		}

		result.CreatedUsers = append(result.CreatedUsers, domain.BulkDirectUserCredential{
			User:         u,
			TempPassword: tempPassword,
		})
	}

	result.CreatedCount = len(result.CreatedUsers)
	result.FailedCount = len(result.Failures)

	return result, nil
}


// ResendInvite regenerates a 72h invitation token and resends the email.
func (s *Store) ResendInvite(ctx context.Context, userID string) (string, error) {
	u, err := s.GetUserByID(ctx, userID)
	if err != nil {
		return "", err
	}
	if u.Status != domain.UserStatusInvited {
		return "", fmt.Errorf("%w: user is already active or disabled", ErrBadRequest)
	}
	rawToken, tokenHash, err := crypto.GenerateRandomToken(32)
	if err != nil {
		return "", err
	}
	now := time.Now().UTC()
	_, _ = s.db.ExecContext(ctx, `DELETE FROM email_verification_tokens WHERE user_id = $1 AND token_type = 'invite'`, userID)
	if _, err := s.db.ExecContext(ctx, `
		INSERT INTO email_verification_tokens (id, user_id, token_type, token_hash, expires_at, created_at)
		VALUES ($1, $2, 'invite', $3, $4, $5)
	`, id.NewV7String(), userID, tokenHash, now.Add(InviteTokenTTL), now); err != nil {
		return "", err
	}

	inviteURL := fmt.Sprintf("%s/auth/invite?token=%s", strings.TrimRight(s.appBaseURL, "/"), rawToken)
	_ = s.sendSystemEmail(ctx, u.Email, "[Polaris] 招待のご案内（再送）", fmt.Sprintf(
		"%s 様\n\nPolarisへの招待リンク（有効期限：72時間）を再送いたします。\n%s\n",
		u.DisplayName, inviteURL,
	))
	return rawToken, nil
}

// AcceptInvite completes an invitation using the 72h token, setting a password (or activating for Google login).
func (s *Store) AcceptInvite(ctx context.Context, rawToken, password, displayName string) (*domain.User, string, error) {
	if rawToken == "" {
		return nil, "", fmt.Errorf("%w: token is required", ErrBadRequest)
	}
	tokenHash := crypto.HashToken(rawToken)
	var tokenID, userID string
	var expiresAt time.Time
	err := s.db.QueryRowContext(ctx, `
		SELECT id, user_id, expires_at
		FROM email_verification_tokens
		WHERE token_hash = $1 AND token_type = 'invite'
	`, tokenHash).Scan(&tokenID, &userID, &expiresAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, "", fmt.Errorf("%w: invalid invitation token", ErrBadRequest)
	}
	if err != nil {
		return nil, "", err
	}
	if time.Now().UTC().After(expiresAt) {
		return nil, "", fmt.Errorf("%w: invitation token has expired", ErrBadRequest)
	}

	var pwHash *string
	if password != "" {
		h, err := crypto.HashPassword(password)
		if err != nil {
			return nil, "", fmt.Errorf("%w: %v", ErrBadRequest, err)
		}
		pwHash = &h
	}

	now := time.Now().UTC()
	if displayName != "" {
		_, err = s.db.ExecContext(ctx, `
			UPDATE users SET password_hash = COALESCE($1, password_hash), display_name = $2, status = $3, updated_at = $4 WHERE id = $5
		`, pwHash, displayName, domain.UserStatusActive, now, userID)
	} else {
		_, err = s.db.ExecContext(ctx, `
			UPDATE users SET password_hash = COALESCE($1, password_hash), status = $2, updated_at = $3 WHERE id = $4
		`, pwHash, domain.UserStatusActive, now, userID)
	}
	if err != nil {
		return nil, "", err
	}
	_, _ = s.db.ExecContext(ctx, `DELETE FROM email_verification_tokens WHERE id = $1`, tokenID)

	sessionToken, err := s.CreateSession(ctx, userID)
	if err != nil {
		return nil, "", err
	}
	u, err := s.GetUserByID(ctx, userID)
	return u, sessionToken, err
}

// SetOrChangePassword allows a user to set a password for the first time (if Google-only) or change their existing password.
func (s *Store) SetOrChangePassword(ctx context.Context, userID, currentPassword, newPassword string) error {
	var pwHash sql.NullString
	err := s.db.QueryRowContext(ctx, `SELECT password_hash FROM users WHERE id = $1`, userID).Scan(&pwHash)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return err
	}
	// If password is already set, require currentPassword verification
	if pwHash.Valid && pwHash.String != "" {
		if !crypto.CheckPassword(currentPassword, pwHash.String) {
			return fmt.Errorf("%w: current password is incorrect", ErrForbidden)
		}
	}
	newHash, err := crypto.HashPassword(newPassword)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrBadRequest, err)
	}
	_, err = s.db.ExecContext(ctx, `UPDATE users SET password_hash = $1, must_change_password = FALSE, updated_at = $2 WHERE id = $3`, newHash, time.Now().UTC(), userID)
	return err
}

// RequestPasswordReset generates a 1h password reset token and emails the reset link.
func (s *Store) RequestPasswordReset(ctx context.Context, email string) (string, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	var userID, displayName, status string
	err := s.db.QueryRowContext(ctx, `SELECT id, display_name, status FROM users WHERE LOWER(email) = $1 AND status != 'disabled'`, email).Scan(&userID, &displayName, &status)
	if errors.Is(err, sql.ErrNoRows) || status != domain.UserStatusActive {
		// Return nil error to prevent email enumeration
		return "", nil
	}
	if err != nil {
		return "", err
	}

	rawToken, tokenHash, err := crypto.GenerateRandomToken(32)
	if err != nil {
		return "", err
	}
	now := time.Now().UTC()
	_, _ = s.db.ExecContext(ctx, `DELETE FROM email_verification_tokens WHERE user_id = $1 AND token_type = 'password_reset'`, userID)
	if _, err := s.db.ExecContext(ctx, `
		INSERT INTO email_verification_tokens (id, user_id, token_type, token_hash, expires_at, created_at)
		VALUES ($1, $2, 'password_reset', $3, $4, $5)
	`, id.NewV7String(), userID, tokenHash, now.Add(PasswordResetTTL), now); err != nil {
		return "", err
	}

	resetURL := fmt.Sprintf("%s/auth/reset-password?token=%s", strings.TrimRight(s.appBaseURL, "/"), rawToken)
	_ = s.sendSystemEmail(ctx, email, "[Polaris] パスワード再設定のご案内", fmt.Sprintf(
		"%s 様\n\nパスワード再設定のリクエストを受け付けました。以下のリンク（有効期限：1時間）から新しいパスワードを設定してください。\n%s\n",
		displayName, resetURL,
	))
	return rawToken, nil
}

// ConfirmPasswordReset verifies a 1h password reset token, updates the password, and revokes existing sessions.
func (s *Store) ConfirmPasswordReset(ctx context.Context, rawToken, newPassword string) error {
	if rawToken == "" {
		return fmt.Errorf("%w: token is required", ErrBadRequest)
	}
	newHash, err := crypto.HashPassword(newPassword)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrBadRequest, err)
	}
	tokenHash := crypto.HashToken(rawToken)
	var tokenID, userID string
	var expiresAt time.Time
	err = s.db.QueryRowContext(ctx, `
		SELECT id, user_id, expires_at
		FROM email_verification_tokens
		WHERE token_hash = $1 AND token_type = 'password_reset'
	`, tokenHash).Scan(&tokenID, &userID, &expiresAt)
	if errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("%w: invalid password reset token", ErrBadRequest)
	}
	if err != nil {
		return err
	}
	if time.Now().UTC().After(expiresAt) {
		return fmt.Errorf("%w: password reset token has expired", ErrBadRequest)
	}

	now := time.Now().UTC()
	if _, err := s.db.ExecContext(ctx, `UPDATE users SET password_hash = $1, must_change_password = FALSE, updated_at = $2 WHERE id = $3`, newHash, now, userID); err != nil {
		return err
	}
	_, _ = s.db.ExecContext(ctx, `DELETE FROM email_verification_tokens WHERE id = $1`, tokenID)
	return s.RevokeAllUserSessions(ctx, userID)
}

// RequestSelfEmailChange verifies the user's current password, validates domain restrictions,
// and sends a 1h verification link to the new email address.
func (s *Store) RequestSelfEmailChange(ctx context.Context, userID, currentPassword, newEmail string) (string, error) {
	newEmail = strings.ToLower(strings.TrimSpace(newEmail))
	allowedDomains, err := s.GetAllowedDomains(ctx)
	if err != nil {
		return "", err
	}
	if err := domain.ValidateEmailAllowedDomain(newEmail, allowedDomains); err != nil {
		return "", fmt.Errorf("%w: %v", ErrBadRequest, err)
	}

	var pwHash sql.NullString
	var displayName string
	err = s.db.QueryRowContext(ctx, `SELECT password_hash, display_name FROM users WHERE id = $1`, userID).Scan(&pwHash, &displayName)
	if errors.Is(err, sql.ErrNoRows) {
		return "", ErrNotFound
	}
	if err != nil {
		return "", err
	}
	if !pwHash.Valid || !crypto.CheckPassword(currentPassword, pwHash.String) {
		return "", fmt.Errorf("%w: current password verification failed", ErrForbidden)
	}

	var existing int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM users WHERE LOWER(email) = $1 AND id <> $2 AND status != 'disabled'`, newEmail, userID).Scan(&existing); err != nil {
		return "", err
	}
	if existing > 0 {
		return "", fmt.Errorf("%w: email is already in use", ErrConflict)
	}

	rawToken, tokenHash, err := crypto.GenerateRandomToken(32)
	if err != nil {
		return "", err
	}
	now := time.Now().UTC()
	_, _ = s.db.ExecContext(ctx, `DELETE FROM email_verification_tokens WHERE user_id = $1 AND token_type = 'email_change'`, userID)
	if _, err := s.db.ExecContext(ctx, `
		INSERT INTO email_verification_tokens (id, user_id, token_type, token_hash, new_email, expires_at, created_at)
		VALUES ($1, $2, 'email_change', $3, $4, $5, $6)
	`, id.NewV7String(), userID, tokenHash, newEmail, now.Add(EmailChangeVerifyTTL), now); err != nil {
		return "", err
	}

	verifyURL := fmt.Sprintf("%s/auth/verify-email?token=%s", strings.TrimRight(s.appBaseURL, "/"), rawToken)
	_ = s.sendSystemEmail(ctx, newEmail, "[Polaris] メールアドレス変更の確認", fmt.Sprintf(
		"%s 様\n\n新しいメールアドレスへの変更を完了するには、以下のリンク（有効期限：1時間）をクリックしてください。\n%s\n",
		displayName, verifyURL,
	))
	return rawToken, nil
}

// ConfirmEmailChange completes an email change using the 1h verification token.
func (s *Store) ConfirmEmailChange(ctx context.Context, rawToken string) (*domain.User, error) {
	if rawToken == "" {
		return nil, fmt.Errorf("%w: token is required", ErrBadRequest)
	}
	tokenHash := crypto.HashToken(rawToken)
	var tokenID, userID string
	var newEmail sql.NullString
	var expiresAt time.Time
	err := s.db.QueryRowContext(ctx, `
		SELECT id, user_id, new_email, expires_at
		FROM email_verification_tokens
		WHERE token_hash = $1 AND token_type = 'email_change'
	`, tokenHash).Scan(&tokenID, &userID, &newEmail, &expiresAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("%w: invalid email change token", ErrBadRequest)
	}
	if err != nil {
		return nil, err
	}
	if time.Now().UTC().After(expiresAt) || !newEmail.Valid || newEmail.String == "" {
		return nil, fmt.Errorf("%w: email change token has expired", ErrBadRequest)
	}

	allowedDomains, err := s.GetAllowedDomains(ctx)
	if err != nil {
		return nil, err
	}
	if err := domain.ValidateEmailAllowedDomain(newEmail.String, allowedDomains); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrBadRequest, err)
	}

	now := time.Now().UTC()
	// Clear google_sub so that Google OAuth links with the new matching email address
	if _, err := s.db.ExecContext(ctx, `
		UPDATE users SET email = $1, google_sub = NULL, updated_at = $2 WHERE id = $3
	`, newEmail.String, now, userID); err != nil {
		return nil, err
	}
	_, _ = s.db.ExecContext(ctx, `DELETE FROM email_verification_tokens WHERE id = $1`, tokenID)
	_ = s.RevokeAllUserSessions(ctx, userID)
	return s.GetUserByID(ctx, userID)
}

// AdminChangeUserEmail allows a system administrator to change a user's email without password confirmation.
func (s *Store) AdminChangeUserEmail(ctx context.Context, targetUserID, newEmail string, sendVerification bool) (*domain.User, string, error) {
	newEmail = strings.ToLower(strings.TrimSpace(newEmail))
	allowedDomains, err := s.GetAllowedDomains(ctx)
	if err != nil {
		return nil, "", err
	}
	if err := domain.ValidateEmailAllowedDomain(newEmail, allowedDomains); err != nil {
		return nil, "", fmt.Errorf("%w: %v", ErrBadRequest, err)
	}

	var existing int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM users WHERE LOWER(email) = $1 AND id <> $2 AND status != 'disabled'`, newEmail, targetUserID).Scan(&existing); err != nil {
		return nil, "", err
	}
	if existing > 0 {
		return nil, "", fmt.Errorf("%w: email is already in use", ErrConflict)
	}

	if sendVerification {
		rawToken, tokenHash, err := crypto.GenerateRandomToken(32)
		if err != nil {
			return nil, "", err
		}
		now := time.Now().UTC()
		_, _ = s.db.ExecContext(ctx, `DELETE FROM email_verification_tokens WHERE user_id = $1 AND token_type = 'email_change'`, targetUserID)
		if _, err := s.db.ExecContext(ctx, `
			INSERT INTO email_verification_tokens (id, user_id, token_type, token_hash, new_email, expires_at, created_at)
			VALUES ($1, $2, 'email_change', $3, $4, $5, $6)
		`, id.NewV7String(), targetUserID, tokenHash, newEmail, now.Add(EmailChangeVerifyTTL), now); err != nil {
			return nil, "", err
		}
		verifyURL := fmt.Sprintf("%s/auth/verify-email?token=%s", strings.TrimRight(s.appBaseURL, "/"), rawToken)
		_ = s.sendSystemEmail(ctx, newEmail, "[Polaris] メールアドレス変更の確認", fmt.Sprintf(
			"管理者によりメールアドレス変更が申請されました。以下のリンク（有効期限：1時間）をクリックして完了してください。\n%s\n",
			verifyURL,
		))
		u, err := s.GetUserByID(ctx, targetUserID)
		return u, rawToken, err
	}

	now := time.Now().UTC()
	if _, err := s.db.ExecContext(ctx, `
		UPDATE users SET email = $1, google_sub = NULL, updated_at = $2 WHERE id = $3
	`, newEmail, now, targetUserID); err != nil {
		return nil, "", err
	}
	_ = s.RevokeAllUserSessions(ctx, targetUserID)
	u, err := s.GetUserByID(ctx, targetUserID)
	return u, "", err
}

// UpdateUserStatus updates a user's status ('active' or 'disabled').
// Disabling a user revokes all their active sessions while preserving their comment/history records.
func (s *Store) UpdateUserStatus(ctx context.Context, targetUserID, status string) (*domain.User, error) {
	if status != domain.UserStatusActive && status != domain.UserStatusDisabled {
		return nil, fmt.Errorf("%w: status must be 'active' or 'disabled'", ErrBadRequest)
	}
	if status == domain.UserStatusDisabled {
		perms, err := s.GetUserSystemPermissions(ctx, targetUserID)
		if err != nil {
			return nil, err
		}
		hasManage := false
		for _, p := range perms {
			if p == domain.PermManageSystem {
				hasManage = true
				break
			}
		}
		if hasManage {
			var otherActiveAdmins int
			err = s.db.QueryRowContext(ctx, `
				SELECT COUNT(DISTINCT u.id)
				FROM users u
				WHERE u.status = 'active' AND u.id != $1
				  AND (
				    EXISTS (
				      SELECT 1 FROM system_permissions sp
				      WHERE sp.subject_type = 'user' AND sp.subject_id = u.id AND sp.permission = 'manage_system'
				    )
				    OR EXISTS (
				      SELECT 1 FROM system_permissions sp
				      JOIN group_members gm ON gm.group_id = sp.subject_id
				      JOIN groups g ON g.id = gm.group_id
				      WHERE sp.subject_type = 'group' AND g.project_id IS NULL AND gm.user_id = u.id AND sp.permission = 'manage_system'
				    )
				  )
			`, targetUserID).Scan(&otherActiveAdmins)
			if err != nil {
				return nil, err
			}
			if otherActiveAdmins == 0 {
				return nil, fmt.Errorf("%w: cannot deactivate the last active administrator", ErrBadRequest)
			}
		}
	}
	res, err := s.db.ExecContext(ctx, `
		UPDATE users SET status = $1, updated_at = $2 WHERE id = $3
	`, status, time.Now().UTC(), targetUserID)
	if err != nil {
		return nil, err
	}
	affected, _ := res.RowsAffected()
	if affected == 0 {
		return nil, ErrNotFound
	}
	if status == domain.UserStatusDisabled {
		_ = s.RevokeAllUserSessions(ctx, targetUserID)
	}
	return s.GetUserByID(ctx, targetUserID)
}

func (s *Store) UpdateUserProfile(ctx context.Context, userID, displayName, avatarURL string) (*domain.User, error) {
	if strings.TrimSpace(displayName) == "" {
		return nil, fmt.Errorf("%w: display_name is required", ErrBadRequest)
	}
	_, err := s.db.ExecContext(ctx, `
		UPDATE users SET display_name = $1, avatar_url = $2, updated_at = $3 WHERE id = $4
	`, strings.TrimSpace(displayName), strings.TrimSpace(avatarURL), time.Now().UTC(), userID)
	if err != nil {
		return nil, err
	}
	return s.GetUserByID(ctx, userID)
}

func (s *Store) sendSystemEmail(ctx context.Context, to, subject, body string) error {
	if s.mailer == nil {
		return nil
	}
	cfg, err := s.getMailConfig(ctx)
	if err != nil {
		return err
	}
	msg := mail.Message{
		To:      to,
		Subject: subject,
		Body:    body,
	}
	if smtpSender, ok := s.mailer.(*mail.SMTPSender); ok && smtpSender.CustomSend != nil {
		return s.mailer.Send(cfg, msg)
	}
	go func(c mail.Config, m mail.Message) {
		_ = s.mailer.Send(c, m)
	}(cfg, msg)
	return nil
}

