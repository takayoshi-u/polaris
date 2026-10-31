package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"

	"github.com/takayoshi-u/polaris/backend/internal/domain"
	"github.com/takayoshi-u/polaris/backend/internal/fcm"
	"github.com/takayoshi-u/polaris/backend/internal/mail"
	"github.com/takayoshi-u/polaris/backend/internal/pkg/crypto"
	"github.com/takayoshi-u/polaris/backend/internal/pkg/id"
)

var (
	ErrNotFound     = errors.New("resource not found")
	ErrForbidden    = errors.New("permission denied")
	ErrUnauthorized = errors.New("unauthorized")
	ErrBadRequest   = errors.New("bad request")
	ErrConflict     = errors.New("conflict")
)

type Store struct {
	db            *sql.DB
	encryptionKey string
	appBaseURL    string
	mailer        mail.Sender
	fcmClient     fcm.Notifier
}

func New(db *sql.DB, encryptionKey, appBaseURL string, mailer mail.Sender, fcmClient fcm.Notifier) *Store {
	if encryptionKey == "" {
		encryptionKey = "polaris-default-encryption-secret"
	}
	if appBaseURL == "" {
		appBaseURL = "http://localhost:3000"
	}
	return &Store{
		db:            db,
		encryptionKey: encryptionKey,
		appBaseURL:    appBaseURL,
		mailer:        mailer,
		fcmClient:     fcmClient,
	}
}

func (s *Store) DB() *sql.DB {
	return s.db
}

// EnsureSystemDefaults initializes system settings, SMTP settings, system default statuses,
// system default templates, and optional initial admin user from environment variables.
func (s *Store) EnsureSystemDefaults(ctx context.Context, initAdminEmail, initAdminPassword, initAdminName string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// 1. Ensure system_smtp_settings row exists
	var smtpCount int
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM system_smtp_settings`).Scan(&smtpCount); err != nil {
		return err
	}
	if smtpCount == 0 {
		_, err := tx.ExecContext(ctx, `
			INSERT INTO system_smtp_settings (id, enabled, host, port, use_auth, username, encrypted_password, from_email, from_name)
			VALUES ($1, FALSE, '', 587, FALSE, '', '', 'noreply@localhost', 'Polaris')
		`, id.NewV7String())
		if err != nil {
			return err
		}
	}

	// 2. Ensure system default statuses (project_id IS NULL) exist
	var statusCount int
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM statuses WHERE project_id IS NULL`).Scan(&statusCount); err != nil {
		return err
	}
	if statusCount == 0 {
		defaults := []struct {
			order int
			name  string
			color string
		}{
			{1, "未着手", "#64748b"},
			{2, "進行中", "#3b82f6"},
			{3, "レビュー中", "#f59e0b"},
			{4, "完了", "#10b981"},
		}
		for _, d := range defaults {
			if _, err := tx.ExecContext(ctx, `
				INSERT INTO statuses (id, project_id, display_order, name, color)
				VALUES ($1, NULL, $2, $3, $4)
			`, id.NewV7String(), d.order, d.name, d.color); err != nil {
				return err
			}
		}
	}

	// 3. Ensure system default templates (project_id IS NULL) exist
	var issueTplID, taskTplID string
	err = tx.QueryRowContext(ctx, `SELECT id FROM templates WHERE project_id IS NULL AND target_type = 'issue' ORDER BY created_at LIMIT 1`).Scan(&issueTplID)
	if errors.Is(err, sql.ErrNoRows) {
		issueTplID, err = insertDefaultTemplateTx(ctx, tx, nil, domain.TargetIssue, "デフォルト課題テンプレート")
		if err != nil {
			return err
		}
	} else if err != nil {
		return err
	}

	err = tx.QueryRowContext(ctx, `SELECT id FROM templates WHERE project_id IS NULL AND target_type = 'task' ORDER BY created_at LIMIT 1`).Scan(&taskTplID)
	if errors.Is(err, sql.ErrNoRows) {
		taskTplID, err = insertDefaultTemplateTx(ctx, tx, nil, domain.TargetTask, "デフォルトタスクテンプレート")
		if err != nil {
			return err
		}
	} else if err != nil {
		return err
	}

	// 4. Ensure system_settings row exists
	var settingsCount int
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM system_settings`).Scan(&settingsCount); err != nil {
		return err
	}
	if settingsCount == 0 {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO system_settings (id, default_hours_per_day, default_issue_template_id, default_task_template_id)
			VALUES ($1, 8.0, $2, $3)
		`, id.NewV7String(), issueTplID, taskTplID); err != nil {
			return err
		}
	}

	// 5. Hybrid Initial Admin Setup: if users == 0 and INIT_ADMIN_EMAIL is provided, seed initial admin
	if initAdminEmail != "" {
		var userCount int
		if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM users`).Scan(&userCount); err != nil {
			return err
		}
		if userCount == 0 {
			var pwHash *string
			if initAdminPassword != "" {
				h, err := crypto.HashPassword(initAdminPassword)
				if err != nil {
					return err
				}
				pwHash = &h
			}
			if initAdminName == "" {
				initAdminName = "System Admin"
			}
			adminID := id.NewV7String()
			if _, err := tx.ExecContext(ctx, `
				INSERT INTO users (id, email, password_hash, display_name, status)
				VALUES ($1, $2, $3, $4, $5)
			`, adminID, initAdminEmail, pwHash, initAdminName, domain.UserStatusActive); err != nil {
				return err
			}
			for _, perm := range []string{domain.PermManageSystem, domain.PermCreateProject, domain.PermDeleteProject} {
				if _, err := tx.ExecContext(ctx, `
					INSERT INTO system_permissions (id, subject_type, subject_id, permission)
					VALUES ($1, $2, $3, $4)
				`, id.NewV7String(), domain.SubjectUser, adminID, perm); err != nil {
					return err
				}
			}
		}
	}

	return tx.Commit()
}

func insertDefaultTemplateTx(ctx context.Context, tx *sql.Tx, projectID *string, targetType, name string) (string, error) {
	tplID := id.NewV7String()
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO templates (id, project_id, target_type, name)
		VALUES ($1, $2, $3, $4)
	`, tplID, projectID, targetType, name); err != nil {
		return "", err
	}

	type fieldSpec struct {
		col int
		key string
	}
	type rowSpec struct {
		order  int
		cols   int
		fields []fieldSpec
	}

	var rows []rowSpec
	if targetType == domain.TargetIssue {
		rows = []rowSpec{
			{1, 1, []fieldSpec{{1, domain.StdFieldTitle}}},
			{2, 1, []fieldSpec{{1, domain.StdFieldDescription}}},
			{3, 1, []fieldSpec{{1, domain.StdFieldLabels}}},
		}
	} else {
		rows = []rowSpec{
			{1, 1, []fieldSpec{{1, domain.StdFieldTitle}}},
			{2, 3, []fieldSpec{{1, domain.StdFieldStatus}, {2, domain.StdFieldProgress}, {3, domain.StdFieldAssignees}}},
			{3, 2, []fieldSpec{{1, domain.StdFieldPlannedDates}, {2, domain.StdFieldActualDates}}},
			{4, 2, []fieldSpec{{1, domain.StdFieldPlannedHours}, {2, domain.StdFieldActualHours}}},
			{5, 2, []fieldSpec{{1, domain.StdFieldPlannedCost}, {2, domain.StdFieldActualCost}}},
			{6, 1, []fieldSpec{{1, domain.StdFieldLabels}}},
			{7, 1, []fieldSpec{{1, domain.StdFieldDescription}}},
		}
	}

	emptyJSON, _ := json.Marshal([]domain.SubjectRef{})
	for _, r := range rows {
		rowID := id.NewV7String()
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO template_rows (id, template_id, row_order, columns_count)
			VALUES ($1, $2, $3, $4)
		`, rowID, tplID, r.order, r.cols); err != nil {
			return "", err
		}
		for _, f := range r.fields {
			if _, err := tx.ExecContext(ctx, `
				INSERT INTO template_fields (
					id, template_row_id, column_order, field_source, field_key,
					is_hidden, is_restricted, allow_owner_view, allow_owner_edit, view_subjects_json, edit_subjects_json
				) VALUES ($1, $2, $3, $4, $5, FALSE, FALSE, TRUE, TRUE, $6, $6)
			`, id.NewV7String(), rowID, f.col, domain.FieldSourceStandard, f.key, emptyJSON); err != nil {
				return "", err
			}
		}
	}
	return tplID, nil
}

