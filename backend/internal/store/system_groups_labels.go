package store

import (
	"context"
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

func (s *Store) GetAllowedDomains(ctx context.Context) ([]string, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT domain FROM system_allowed_domains ORDER BY created_at ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	domains := []string{}
	for rows.Next() {
		var d string
		if err := rows.Scan(&d); err != nil {
			return nil, err
		}
		domains = append(domains, d)
	}
	return domains, rows.Err()
}

func (s *Store) SetAllowedDomains(ctx context.Context, domains []string) ([]string, error) {
	cleaned, err := domain.ValidateAllowedDomainsList(domains)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrBadRequest, err)
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, `DELETE FROM system_allowed_domains`); err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	for _, d := range cleaned {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO system_allowed_domains (id, domain, created_at)
			VALUES ($1, $2, $3)
		`, id.NewV7String(), d, now); err != nil {
			return nil, err
		}
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return s.GetAllowedDomains(ctx)
}

func (s *Store) GetSystemSettings(ctx context.Context) (*domain.SystemSettings, error) {
	var st domain.SystemSettings
	var issueTpl, taskTpl sql.NullString
	err := s.db.QueryRowContext(ctx, `
		SELECT id, default_hours_per_day, default_issue_template_id, default_task_template_id, updated_at
		FROM system_settings LIMIT 1
	`).Scan(&st.ID, &st.DefaultHoursPerDay, &issueTpl, &taskTpl, &st.UpdatedAt)
	if err != nil {
		return nil, err
	}
	if issueTpl.Valid {
		st.DefaultIssueTemplateID = &issueTpl.String
	}
	if taskTpl.Valid {
		st.DefaultTaskTemplateID = &taskTpl.String
	}
	domains, err := s.GetAllowedDomains(ctx)
	if err != nil {
		return nil, err
	}
	st.AllowedDomains = domains
	return &st, nil
}

func (s *Store) UpdateSystemSettings(ctx context.Context, defaultHoursPerDay float64, defaultIssueTplID, defaultTaskTplID *string, allowedDomains *[]string) (*domain.SystemSettings, error) {
	if err := domain.ValidateHoursPerDay(defaultHoursPerDay, "default_hours_per_day"); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrBadRequest, err)
	}
	if allowedDomains != nil {
		if _, err := s.SetAllowedDomains(ctx, *allowedDomains); err != nil {
			return nil, err
		}
	}
	now := time.Now().UTC()
	_, err := s.db.ExecContext(ctx, `
		UPDATE system_settings
		SET default_hours_per_day = $1,
		    default_issue_template_id = COALESCE($2, default_issue_template_id),
		    default_task_template_id = COALESCE($3, default_task_template_id),
		    updated_at = $4
	`, defaultHoursPerDay, defaultIssueTplID, defaultTaskTplID, now)
	if err != nil {
		return nil, err
	}
	return s.GetSystemSettings(ctx)
}

func (s *Store) GetSMTPSettings(ctx context.Context) (*domain.SMTPSettings, error) {
	var st domain.SMTPSettings
	var encPass string
	err := s.db.QueryRowContext(ctx, `
		SELECT id, enabled, host, port, use_auth, username, encrypted_password, from_email, from_name, updated_at
		FROM system_smtp_settings LIMIT 1
	`).Scan(&st.ID, &st.Enabled, &st.Host, &st.Port, &st.UseAuth, &st.Username, &encPass, &st.FromEmail, &st.FromName, &st.UpdatedAt)
	if err != nil {
		return nil, err
	}
	st.HasPassword = encPass != ""
	return &st, nil
}

func (s *Store) UpdateSMTPSettings(ctx context.Context, enabled bool, host string, port int, useAuth bool, username string, password *string, fromEmail, fromName string) (*domain.SMTPSettings, error) {
	if port <= 0 || port > 65535 {
		port = 587
	}
	if fromEmail == "" {
		fromEmail = "noreply@localhost"
	}
	if fromName == "" {
		fromName = "Polaris"
	}

	var encPass *string
	if password != nil {
		if *password == "" {
			empty := ""
			encPass = &empty
		} else {
			enc, err := crypto.EncryptAESGCM(*password, s.encryptionKey)
			if err != nil {
				return nil, err
			}
			encPass = &enc
		}
	}

	now := time.Now().UTC()
	if encPass != nil {
		_, err := s.db.ExecContext(ctx, `
			UPDATE system_smtp_settings
			SET enabled = $1, host = $2, port = $3, use_auth = $4, username = $5,
			    encrypted_password = $6, from_email = $7, from_name = $8, updated_at = $9
		`, enabled, strings.TrimSpace(host), port, useAuth, strings.TrimSpace(username), *encPass, strings.TrimSpace(fromEmail), strings.TrimSpace(fromName), now)
		if err != nil {
			return nil, err
		}
	} else {
		_, err := s.db.ExecContext(ctx, `
			UPDATE system_smtp_settings
			SET enabled = $1, host = $2, port = $3, use_auth = $4, username = $5,
			    from_email = $6, from_name = $7, updated_at = $8
		`, enabled, strings.TrimSpace(host), port, useAuth, strings.TrimSpace(username), strings.TrimSpace(fromEmail), strings.TrimSpace(fromName), now)
		if err != nil {
			return nil, err
		}
	}
	return s.GetSMTPSettings(ctx)
}

func (s *Store) getMailConfig(ctx context.Context) (mail.Config, error) {
	var cfg mail.Config
	var encPass string
	err := s.db.QueryRowContext(ctx, `
		SELECT enabled, host, port, use_auth, username, encrypted_password, from_email, from_name
		FROM system_smtp_settings LIMIT 1
	`).Scan(&cfg.Enabled, &cfg.Host, &cfg.Port, &cfg.UseAuth, &cfg.Username, &encPass, &cfg.FromEmail, &cfg.FromName)
	if err != nil {
		return cfg, err
	}
	if encPass != "" {
		pw, err := crypto.DecryptAESGCM(encPass, s.encryptionKey)
		if err == nil {
			cfg.Password = pw
		}
	}
	return cfg, nil
}

func (s *Store) SendTestEmail(ctx context.Context, recipientEmail string) error {
	recipientEmail = strings.TrimSpace(recipientEmail)
	if recipientEmail == "" || !strings.Contains(recipientEmail, "@") {
		return fmt.Errorf("%w: valid recipient email is required", ErrBadRequest)
	}
	if s.mailer == nil {
		return nil
	}
	cfg, err := s.getMailConfig(ctx)
	if err != nil {
		return err
	}
	msg := mail.Message{
		To:      recipientEmail,
		Subject: "[Polaris] SMTPテストメール",
		Body:    "これはPolarisのメール送信設定テストメールです。",
	}
	return s.mailer.Send(cfg, msg)
}

// GetUserSystemPermissions returns all distinct system permissions attached to a user directly or via global groups.
func (s *Store) GetUserSystemPermissions(ctx context.Context, userID string) ([]string, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT DISTINCT sp.permission
		FROM system_permissions sp
		WHERE (sp.subject_type = 'user' AND sp.subject_id = $1)
		   OR (sp.subject_type = 'group' AND sp.subject_id IN (
		       SELECT gm.group_id FROM group_members gm
		       JOIN groups g ON g.id = gm.group_id
		       WHERE gm.user_id = $1 AND g.project_id IS NULL
		   ))
		ORDER BY sp.permission ASC
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	perms := []string{}
	for rows.Next() {
		var p string
		if err := rows.Scan(&p); err != nil {
			return nil, err
		}
		perms = append(perms, p)
	}
	return perms, rows.Err()
}

func (s *Store) HasUserSystemPermission(ctx context.Context, userID, perm string) (bool, error) {
	perms, err := s.GetUserSystemPermissions(ctx, userID)
	if err != nil {
		return false, err
	}
	for _, p := range perms {
		if p == perm {
			return true, nil
		}
	}
	return false, nil
}

func (s *Store) ListSystemPermissions(ctx context.Context) ([]domain.SystemPermissionEntry, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, subject_type, subject_id, permission, created_at
		FROM system_permissions ORDER BY created_at ASC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	entries := []domain.SystemPermissionEntry{}
	for rows.Next() {
		var e domain.SystemPermissionEntry
		if err := rows.Scan(&e.ID, &e.SubjectType, &e.SubjectID, &e.Permission, &e.CreatedAt); err != nil {
			return nil, err
		}
		entries = append(entries, e)
	}
	return entries, rows.Err()
}

func (s *Store) GrantSystemPermission(ctx context.Context, subjectType, subjectID, permission string) (*domain.SystemPermissionEntry, error) {
	if subjectType != domain.SubjectUser && subjectType != domain.SubjectGroup {
		return nil, fmt.Errorf("%w: subject_type must be 'user' or 'group'", ErrBadRequest)
	}
	if permission != domain.PermCreateProject && permission != domain.PermDeleteProject && permission != domain.PermManageSystem {
		return nil, fmt.Errorf("%w: invalid permission '%s'", ErrBadRequest, permission)
	}
	entryID := id.NewV7String()
	now := time.Now().UTC()
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO system_permissions (id, subject_type, subject_id, permission, created_at)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (subject_type, subject_id, permission) DO NOTHING
	`, entryID, subjectType, subjectID, permission, now)
	if err != nil {
		return nil, err
	}
	var e domain.SystemPermissionEntry
	err = s.db.QueryRowContext(ctx, `
		SELECT id, subject_type, subject_id, permission, created_at
		FROM system_permissions
		WHERE subject_type = $1 AND subject_id = $2 AND permission = $3
	`, subjectType, subjectID, permission).Scan(&e.ID, &e.SubjectType, &e.SubjectID, &e.Permission, &e.CreatedAt)
	return &e, err
}

func (s *Store) RevokeSystemPermission(ctx context.Context, entryID string) error {
	var perm string
	err := s.db.QueryRowContext(ctx, `SELECT permission FROM system_permissions WHERE id = $1`, entryID).Scan(&perm)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return err
	}

	if perm == domain.PermManageSystem {
		var activeAdminsCount int
		err = s.db.QueryRowContext(ctx, `
			SELECT COUNT(DISTINCT u.id)
			FROM users u
			WHERE u.status = 'active'
			  AND (
			    EXISTS (
			      SELECT 1 FROM system_permissions sp
			      WHERE sp.id != $1 AND sp.subject_type = 'user' AND sp.subject_id = u.id AND sp.permission = 'manage_system'
			    )
			    OR EXISTS (
			      SELECT 1 FROM system_permissions sp
			      JOIN group_members gm ON gm.group_id = sp.subject_id
			      JOIN groups g ON g.id = gm.group_id
			      WHERE sp.id != $1 AND sp.subject_type = 'group' AND g.project_id IS NULL AND gm.user_id = u.id AND sp.permission = 'manage_system'
			    )
			  )
		`, entryID).Scan(&activeAdminsCount)
		if err != nil {
			return err
		}
		if activeAdminsCount == 0 {
			return fmt.Errorf("%w: cannot revoke manage_system from the last active administrator", ErrBadRequest)
		}
	}

	res, err := s.db.ExecContext(ctx, `DELETE FROM system_permissions WHERE id = $1`, entryID)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// Groups (Global if projectID == nil, or Project-local if projectID != nil)
func (s *Store) ListGroups(ctx context.Context, projectID *string) ([]domain.Group, error) {
	var rows *sql.Rows
	var err error
	if projectID == nil {
		rows, err = s.db.QueryContext(ctx, `SELECT id, project_id, name, created_at FROM groups WHERE project_id IS NULL ORDER BY id ASC`)
	} else {
		rows, err = s.db.QueryContext(ctx, `SELECT id, project_id, name, created_at FROM groups WHERE project_id = $1 OR project_id IS NULL ORDER BY project_id NULLS FIRST, id ASC`, *projectID)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	groups := []domain.Group{}
	for rows.Next() {
		var g domain.Group
		var pid sql.NullString
		if err := rows.Scan(&g.ID, &pid, &g.Name, &g.CreatedAt); err != nil {
			return nil, err
		}
		if pid.Valid {
			g.ProjectID = &pid.String
		}
		g.MemberIDs, _ = s.getGroupMemberIDs(ctx, g.ID)
		groups = append(groups, g)
	}
	return groups, rows.Err()
}

func (s *Store) getGroupMemberIDs(ctx context.Context, groupID string) ([]string, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT user_id FROM group_members WHERE group_id = $1 ORDER BY created_at ASC`, groupID)
	if err != nil {
		return []string{}, err
	}
	defer rows.Close()
	ids := []string{}
	for rows.Next() {
		var uid string
		if err := rows.Scan(&uid); err == nil {
			ids = append(ids, uid)
		}
	}
	return ids, rows.Err()
}

func (s *Store) GetGroup(ctx context.Context, groupID string) (*domain.Group, error) {
	var g domain.Group
	var pid sql.NullString
	err := s.db.QueryRowContext(ctx, `SELECT id, project_id, name, created_at FROM groups WHERE id = $1`, groupID).Scan(&g.ID, &pid, &g.Name, &g.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	if pid.Valid {
		g.ProjectID = &pid.String
	}
	g.MemberIDs, _ = s.getGroupMemberIDs(ctx, g.ID)
	return &g, nil
}

func (s *Store) CreateGroup(ctx context.Context, projectID *string, name string, memberIDs []string) (*domain.Group, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, fmt.Errorf("%w: group name is required", ErrBadRequest)
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	groupID := id.NewV7String()
	now := time.Now().UTC()
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO groups (id, project_id, name, created_at)
		VALUES ($1, $2, $3, $4)
	`, groupID, projectID, name, now); err != nil {
		return nil, err
	}
	for _, uid := range memberIDs {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO group_members (id, group_id, user_id, created_at)
			VALUES ($1, $2, $3, $4)
			ON CONFLICT DO NOTHING
		`, id.NewV7String(), groupID, uid, now); err != nil {
			return nil, err
		}
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return s.GetGroup(ctx, groupID)
}

func (s *Store) UpdateGroup(ctx context.Context, groupID, name string, memberIDs []string) (*domain.Group, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, fmt.Errorf("%w: group name is required", ErrBadRequest)
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	res, err := tx.ExecContext(ctx, `UPDATE groups SET name = $1 WHERE id = $2`, name, groupID)
	if err != nil {
		return nil, err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return nil, ErrNotFound
	}
	if memberIDs != nil {
		if _, err := tx.ExecContext(ctx, `DELETE FROM group_members WHERE group_id = $1`, groupID); err != nil {
			return nil, err
		}
		now := time.Now().UTC()
		for _, uid := range memberIDs {
			if _, err := tx.ExecContext(ctx, `
				INSERT INTO group_members (id, group_id, user_id, created_at)
				VALUES ($1, $2, $3, $4)
				ON CONFLICT DO NOTHING
			`, id.NewV7String(), groupID, uid, now); err != nil {
				return nil, err
			}
		}
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return s.GetGroup(ctx, groupID)
}

func (s *Store) DeleteGroup(ctx context.Context, groupID string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Prevent deleting group if it leaves 0 active administrators with manage_system
	var hasManageSystem int
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM system_permissions WHERE subject_type = 'group' AND subject_id = $1 AND permission = 'manage_system'`, groupID).Scan(&hasManageSystem); err != nil {
		return err
	}
	if hasManageSystem > 0 {
		var activeAdminsCount int
		err = tx.QueryRowContext(ctx, `
			SELECT COUNT(DISTINCT u.id)
			FROM users u
			WHERE u.status = 'active'
			  AND (
			    EXISTS (
			      SELECT 1 FROM system_permissions sp
			      WHERE sp.subject_type = 'user' AND sp.subject_id = u.id AND sp.permission = 'manage_system'
			    )
			    OR EXISTS (
			      SELECT 1 FROM system_permissions sp
			      JOIN group_members gm ON gm.group_id = sp.subject_id
			      JOIN groups g ON g.id = gm.group_id
			      WHERE sp.subject_id != $1 AND sp.subject_type = 'group' AND g.project_id IS NULL AND gm.user_id = u.id AND sp.permission = 'manage_system'
			    )
			  )
		`, groupID).Scan(&activeAdminsCount)
		if err != nil {
			return err
		}
		if activeAdminsCount == 0 {
			return fmt.Errorf("%w: cannot delete group; would remove manage_system from the last active administrator", ErrBadRequest)
		}
	}

	if _, err := tx.ExecContext(ctx, `DELETE FROM system_permissions WHERE subject_type = 'group' AND subject_id = $1`, groupID); err != nil {
		return err
	}
	res, err := tx.ExecContext(ctx, `DELETE FROM groups WHERE id = $1`, groupID)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return tx.Commit()
}

// GetUserGroupIDs returns all global and project-specific group IDs that userID belongs to.
func (s *Store) GetUserGroupIDs(ctx context.Context, userID string, projectID *string) ([]string, error) {
	var rows *sql.Rows
	var err error
	if projectID == nil {
		rows, err = s.db.QueryContext(ctx, `
			SELECT gm.group_id FROM group_members gm
			JOIN groups g ON g.id = gm.group_id
			WHERE gm.user_id = $1 AND g.project_id IS NULL
		`, userID)
	} else {
		rows, err = s.db.QueryContext(ctx, `
			SELECT gm.group_id FROM group_members gm
			JOIN groups g ON g.id = gm.group_id
			WHERE gm.user_id = $1 AND (g.project_id IS NULL OR g.project_id = $2)
		`, userID, *projectID)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	gids := []string{}
	for rows.Next() {
		var gid string
		if err := rows.Scan(&gid); err != nil {
			return nil, err
		}
		gids = append(gids, gid)
	}
	return gids, rows.Err()
}

// Labels (System-common when projectID == nil, or Project-local when projectID != nil)
func (s *Store) ListLabels(ctx context.Context, projectID *string) ([]domain.Label, error) {
	var rows *sql.Rows
	var err error
	if projectID == nil {
		rows, err = s.db.QueryContext(ctx, `SELECT id, project_id, name, color, created_at FROM labels WHERE project_id IS NULL ORDER BY id ASC`)
	} else {
		rows, err = s.db.QueryContext(ctx, `SELECT id, project_id, name, color, created_at FROM labels WHERE project_id IS NULL OR project_id = $1 ORDER BY project_id NULLS FIRST, id ASC`, *projectID)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	labels := []domain.Label{}
	for rows.Next() {
		var l domain.Label
		var pid sql.NullString
		if err := rows.Scan(&l.ID, &pid, &l.Name, &l.Color, &l.CreatedAt); err != nil {
			return nil, err
		}
		if pid.Valid {
			l.ProjectID = &pid.String
		}
		labels = append(labels, l)
	}
	return labels, rows.Err()
}

func (s *Store) CreateLabel(ctx context.Context, projectID *string, name, color string) (*domain.Label, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, fmt.Errorf("%w: label name is required", ErrBadRequest)
	}
	if color == "" {
		color = "#3b82f6"
	}
	labelID := id.NewV7String()
	now := time.Now().UTC()
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO labels (id, project_id, name, color, created_at)
		VALUES ($1, $2, $3, $4, $5)
	`, labelID, projectID, name, color, now)
	if err != nil {
		return nil, err
	}
	return &domain.Label{
		ID:        labelID,
		ProjectID: projectID,
		Name:      name,
		Color:     color,
		CreatedAt: now,
	}, nil
}

func (s *Store) UpdateLabel(ctx context.Context, labelID, name, color string) (*domain.Label, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, fmt.Errorf("%w: label name is required", ErrBadRequest)
	}
	res, err := s.db.ExecContext(ctx, `UPDATE labels SET name = $1, color = $2 WHERE id = $3`, name, color, labelID)
	if err != nil {
		return nil, err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return nil, ErrNotFound
	}
	var l domain.Label
	var pid sql.NullString
	err = s.db.QueryRowContext(ctx, `SELECT id, project_id, name, color, created_at FROM labels WHERE id = $1`, labelID).Scan(&l.ID, &pid, &l.Name, &l.Color, &l.CreatedAt)
	if pid.Valid {
		l.ProjectID = &pid.String
	}
	return &l, err
}

func (s *Store) DeleteLabel(ctx context.Context, labelID string) error {
	res, err := s.db.ExecContext(ctx, `DELETE FROM labels WHERE id = $1`, labelID)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

