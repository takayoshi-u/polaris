package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/takayoshi-u/polaris/backend/internal/domain"
	"github.com/takayoshi-u/polaris/backend/internal/pkg/id"
)

var ErrNonMemberDeleteWarning = errors.New("warning: you are not a member of this project; set confirm_non_member=true to proceed with deletion")

// GetEffectiveProjectRole calculates the strongest role (Admin > Editor > Viewer > None)
// for a user in a project across direct user assignment and group assignments.
func (s *Store) GetEffectiveProjectRole(ctx context.Context, projectID, userID string) (domain.ProjectRole, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT pm.role
		FROM project_members pm
		WHERE pm.project_id = $1
		  AND (
		      (pm.subject_type = 'user' AND pm.subject_id = $2)
		      OR (pm.subject_type = 'group' AND pm.subject_id IN (
		          SELECT gm.group_id FROM group_members gm WHERE gm.user_id = $2
		      ))
		  )
	`, projectID, userID)
	if err != nil {
		return domain.RoleNone, err
	}
	defer rows.Close()

	var roles []domain.ProjectRole
	for rows.Next() {
		var r string
		if err := rows.Scan(&r); err != nil {
			return domain.RoleNone, err
		}
		roles = append(roles, domain.ProjectRole(r))
	}
	return domain.ResolveEffectiveProjectRole(roles), rows.Err()
}

// CreateProject creates a new project, copying system defaults (hours_per_day, statuses, custom fields, templates),
// and registers creatorUserID as project Admin.
func (s *Store) CreateProject(ctx context.Context, creatorUserID, name, description string, hoursPerDay *float64) (*domain.Project, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, fmt.Errorf("%w: project name is required", ErrBadRequest)
	}

	sysSettings, err := s.GetSystemSettings(ctx)
	if err != nil {
		return nil, err
	}
	hpd := sysSettings.DefaultHoursPerDay
	if hoursPerDay != nil {
		if err := domain.ValidateHoursPerDay(*hoursPerDay, "hours_per_day"); err != nil {
			return nil, fmt.Errorf("%w: %v", ErrBadRequest, err)
		}
		hpd = *hoursPerDay
	}

	sysStatuses, err := s.ListStatuses(ctx, nil)
	if err != nil {
		return nil, err
	}
	sysCustomFields, err := s.ListCustomFields(ctx, nil, "")
	if err != nil {
		return nil, err
	}
	sysTemplates, err := s.ListTemplates(ctx, nil, "")
	if err != nil {
		return nil, err
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	projectID := id.NewV7String()
	now := time.Now().UTC()

	if _, err := tx.ExecContext(ctx, `
		INSERT INTO projects (id, name, description, hours_per_day, created_by, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $6)
	`, projectID, name, description, hpd, creatorUserID, now); err != nil {
		return nil, err
	}

	// Register creator as Admin
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO project_members (id, project_id, subject_type, subject_id, role, created_at)
		VALUES ($1, $2, 'user', $3, 'admin', $4)
	`, id.NewV7String(), projectID, creatorUserID, now); err != nil {
		return nil, err
	}

	// Copy system default statuses
	for _, st := range sysStatuses {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO statuses (id, project_id, display_order, name, color, created_at)
			VALUES ($1, $2, $3, $4, $5, $6)
		`, id.NewV7String(), projectID, st.DisplayOrder, st.Name, st.Color, now); err != nil {
			return nil, err
		}
	}

	// Copy system default custom field definitions
	fieldIDMap := make(map[string]string)
	for _, cf := range sysCustomFields {
		newFieldID := id.NewV7String()
		fieldIDMap[cf.ID] = newFieldID
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO custom_field_definitions (id, project_id, target_type, name, field_type, options_json, created_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7)
		`, newFieldID, projectID, cf.TargetType, cf.Name, cf.FieldType, cf.OptionsJSON, now); err != nil {
			return nil, err
		}
	}

	// Copy system default templates
	var defaultIssueTplID, defaultTaskTplID *string
	for _, tpl := range sysTemplates {
		newTplID := id.NewV7String()
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO templates (id, project_id, target_type, name, created_at, updated_at)
			VALUES ($1, $2, $3, $4, $5, $5)
		`, newTplID, projectID, tpl.TargetType, tpl.Name, now); err != nil {
			return nil, err
		}
		for _, r := range tpl.Rows {
			newRowID := id.NewV7String()
			if _, err := tx.ExecContext(ctx, `
				INSERT INTO template_rows (id, template_id, row_order, columns_count)
				VALUES ($1, $2, $3, $4)
			`, newRowID, newTplID, r.RowOrder, r.ColumnsCount); err != nil {
				return nil, err
			}
			for _, f := range r.Fields {
				fKey := f.FieldKey
				if f.FieldSource == domain.FieldSourceCustom {
					if mapped, ok := fieldIDMap[fKey]; ok {
						fKey = mapped
					}
				}
				viewJSON, _ := json.Marshal(f.ViewSubjects)
				editJSON, _ := json.Marshal(f.EditSubjects)
				if _, err := tx.ExecContext(ctx, `
					INSERT INTO template_fields (
						id, template_row_id, column_order, field_source, field_key,
						is_hidden, is_restricted, allow_owner_view, allow_owner_edit, view_subjects_json, edit_subjects_json
					) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
				`, id.NewV7String(), newRowID, f.ColumnOrder, f.FieldSource, fKey, f.IsHidden, f.IsRestricted, f.AllowOwnerView, f.AllowOwnerEdit, viewJSON, editJSON); err != nil {
					return nil, err
				}
			}
		}
		if tpl.TargetType == domain.TargetIssue && (defaultIssueTplID == nil || (sysSettings.DefaultIssueTemplateID != nil && tpl.ID == *sysSettings.DefaultIssueTemplateID)) {
			val := newTplID
			defaultIssueTplID = &val
		}
		if tpl.TargetType == domain.TargetTask && (defaultTaskTplID == nil || (sysSettings.DefaultTaskTemplateID != nil && tpl.ID == *sysSettings.DefaultTaskTemplateID)) {
			val := newTplID
			defaultTaskTplID = &val
		}
	}

	if _, err := tx.ExecContext(ctx, `
		UPDATE projects
		SET default_issue_template_id = $1, default_task_template_id = $2
		WHERE id = $3
	`, defaultIssueTplID, defaultTaskTplID, projectID); err != nil {
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return s.GetProject(ctx, projectID, creatorUserID)
}

func (s *Store) GetProject(ctx context.Context, projectID, userID string) (*domain.Project, error) {
	var p domain.Project
	var issueTpl, taskTpl, createdBy sql.NullString
	err := s.db.QueryRowContext(ctx, `
		SELECT id, name, description, hours_per_day, default_issue_template_id, default_task_template_id, created_by, created_at, updated_at
		FROM projects WHERE id = $1
	`, projectID).Scan(&p.ID, &p.Name, &p.Description, &p.HoursPerDay, &issueTpl, &taskTpl, &createdBy, &p.CreatedAt, &p.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	if issueTpl.Valid {
		p.DefaultIssueTemplateID = &issueTpl.String
	}
	if taskTpl.Valid {
		p.DefaultTaskTemplateID = &taskTpl.String
	}
	if createdBy.Valid {
		p.CreatedBy = &createdBy.String
	}
	_ = s.db.QueryRowContext(ctx, `
		SELECT COALESCE(SUM(planned_cost), 0), COALESCE(SUM(actual_cost), 0)
		FROM tasks WHERE project_id = $1
	`, projectID).Scan(&p.TotalPlannedCost, &p.TotalActualCost)
	role, err := s.GetEffectiveProjectRole(ctx, projectID, userID)
	if err != nil {
		return nil, err
	}
	p.CurrentUserRole = role
	p.IsMember = role != domain.RoleNone
	if userID != "" && domain.RolePriority(p.CurrentUserRole) < domain.RolePriority(domain.RoleAdmin) {
		hasSysAdmin, _ := s.HasUserSystemPermission(ctx, userID, domain.PermManageSystem)
		if hasSysAdmin {
			p.CurrentUserRole = domain.RoleAdmin
			p.IsMember = true
		}
	}
	return &p, nil
}

// ListProjects returns all projects the user participates in.
// If includeNonMemberForDelete is true and the user has delete_project permission, non-member projects are also returned with IsMember=false.
func (s *Store) ListProjects(ctx context.Context, userID string, includeNonMemberForDelete bool) ([]domain.Project, error) {
	canDeleteAll := false
	if includeNonMemberForDelete {
		hasDel, _ := s.HasUserSystemPermission(ctx, userID, domain.PermDeleteProject)
		if hasDel {
			canDeleteAll = true
		}
	}

	rows, err := s.db.QueryContext(ctx, `
		SELECT id, name, description, hours_per_day, default_issue_template_id, default_task_template_id, created_by, created_at, updated_at
		FROM projects ORDER BY id ASC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var all []domain.Project
	for rows.Next() {
		var p domain.Project
		var issueTpl, taskTpl, createdBy sql.NullString
		if err := rows.Scan(&p.ID, &p.Name, &p.Description, &p.HoursPerDay, &issueTpl, &taskTpl, &createdBy, &p.CreatedAt, &p.UpdatedAt); err != nil {
			return nil, err
		}
		if issueTpl.Valid {
			p.DefaultIssueTemplateID = &issueTpl.String
		}
		if taskTpl.Valid {
			p.DefaultTaskTemplateID = &taskTpl.String
		}
		if createdBy.Valid {
			p.CreatedBy = &createdBy.String
		}
		all = append(all, p)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	result := []domain.Project{}
	for _, p := range all {
		role, err := s.GetEffectiveProjectRole(ctx, p.ID, userID)
		if err != nil {
			return nil, err
		}
		p.CurrentUserRole = role
		p.IsMember = role != domain.RoleNone
		if p.IsMember || canDeleteAll {
			_ = s.db.QueryRowContext(ctx, `
				SELECT COALESCE(SUM(planned_cost), 0), COALESCE(SUM(actual_cost), 0)
				FROM tasks WHERE project_id = $1
			`, p.ID).Scan(&p.TotalPlannedCost, &p.TotalActualCost)
			result = append(result, p)
		}
	}
	return result, nil
}

func (s *Store) UpdateProject(ctx context.Context, projectID, userID, name, description string, hoursPerDay float64, defaultIssueTplID, defaultTaskTplID *string) (*domain.Project, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, fmt.Errorf("%w: project name is required", ErrBadRequest)
	}
	if err := domain.ValidateHoursPerDay(hoursPerDay, "hours_per_day"); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrBadRequest, err)
	}
	res, err := s.db.ExecContext(ctx, `
		UPDATE projects
		SET name = $1,
		    description = $2,
		    hours_per_day = $3,
		    default_issue_template_id = COALESCE($4, default_issue_template_id),
		    default_task_template_id = COALESCE($5, default_task_template_id),
		    updated_at = $6
		WHERE id = $7
	`, name, description, hoursPerDay, defaultIssueTplID, defaultTaskTplID, time.Now().UTC(), projectID)
	if err != nil {
		return nil, err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return nil, ErrNotFound
	}
	return s.GetProject(ctx, projectID, userID)
}

// DeleteProject deletes a project if the user has delete_project system permission.
// If the user is not a member of the project and confirmNonMember is false, returns ErrNonMemberDeleteWarning.
func (s *Store) DeleteProject(ctx context.Context, projectID, userID string, confirmNonMember bool) error {
	hasPerm, err := s.HasUserSystemPermission(ctx, userID, domain.PermDeleteProject)
	if err != nil {
		return err
	}
	if !hasPerm {
		return fmt.Errorf("%w: delete_project system permission is required", ErrForbidden)
	}

	p, err := s.GetProject(ctx, projectID, userID)
	if err != nil {
		return err
	}
	if !p.IsMember && !confirmNonMember {
		return ErrNonMemberDeleteWarning
	}

	_, err = s.db.ExecContext(ctx, `DELETE FROM projects WHERE id = $1`, projectID)
	return err
}

// Project Members
func (s *Store) ListProjectMembers(ctx context.Context, projectID string) ([]domain.ProjectMember, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, project_id, subject_type, subject_id, role, created_at
		FROM project_members WHERE project_id = $1 ORDER BY created_at ASC
	`, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	members := []domain.ProjectMember{}
	for rows.Next() {
		var m domain.ProjectMember
		var r string
		if err := rows.Scan(&m.ID, &m.ProjectID, &m.SubjectType, &m.SubjectID, &r, &m.CreatedAt); err != nil {
			return nil, err
		}
		m.Role = domain.ProjectRole(r)
		members = append(members, m)
	}
	return members, rows.Err()
}

func (s *Store) UpsertProjectMember(ctx context.Context, projectID, subjectType, subjectID string, role domain.ProjectRole) (*domain.ProjectMember, error) {
	if subjectType != domain.SubjectUser && subjectType != domain.SubjectGroup {
		return nil, fmt.Errorf("%w: subject_type must be 'user' or 'group'", ErrBadRequest)
	}
	if role != domain.RoleAdmin && role != domain.RoleEditor && role != domain.RoleViewer {
		return nil, fmt.Errorf("%w: role must be 'admin', 'editor', or 'viewer'", ErrBadRequest)
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	// If demoting from admin to editor/viewer, check if there are other admins
	if role != domain.RoleAdmin {
		var currentRole string
		err := tx.QueryRowContext(ctx, `
			SELECT role FROM project_members
			WHERE project_id = $1 AND subject_type = $2 AND subject_id = $3
		`, projectID, subjectType, subjectID).Scan(&currentRole)
		if err == nil && currentRole == string(domain.RoleAdmin) {
			var otherAdminsCount int
			err := tx.QueryRowContext(ctx, `
				SELECT COUNT(*) FROM project_members
				WHERE project_id = $1 AND role = 'admin' AND NOT (subject_type = $2 AND subject_id = $3)
			`, projectID, subjectType, subjectID).Scan(&otherAdminsCount)
			if err != nil {
				return nil, err
			}
			if otherAdminsCount == 0 {
				return nil, fmt.Errorf("%w: cannot demote the last project admin", ErrBadRequest)
			}
		} else if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return nil, err
		}
	}

	now := time.Now().UTC()
	_, err = tx.ExecContext(ctx, `
		INSERT INTO project_members (id, project_id, subject_type, subject_id, role, created_at)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (project_id, subject_type, subject_id)
		DO UPDATE SET role = EXCLUDED.role
	`, id.NewV7String(), projectID, subjectType, subjectID, string(role), now)
	if err != nil {
		return nil, err
	}
	var m domain.ProjectMember
	var r string
	err = tx.QueryRowContext(ctx, `
		SELECT id, project_id, subject_type, subject_id, role, created_at
		FROM project_members WHERE project_id = $1 AND subject_type = $2 AND subject_id = $3
	`, projectID, subjectType, subjectID).Scan(&m.ID, &m.ProjectID, &m.SubjectType, &m.SubjectID, &r, &m.CreatedAt)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	m.Role = domain.ProjectRole(r)
	return &m, nil
}

func (s *Store) RemoveProjectMember(ctx context.Context, projectID, memberEntryID string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var currentRole string
	err = tx.QueryRowContext(ctx, `
		SELECT role FROM project_members WHERE id = $1 AND project_id = $2
	`, memberEntryID, projectID).Scan(&currentRole)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return err
	}

	if currentRole == string(domain.RoleAdmin) {
		var otherAdminsCount int
		err := tx.QueryRowContext(ctx, `
			SELECT COUNT(*) FROM project_members
			WHERE project_id = $1 AND role = 'admin' AND id != $2
		`, projectID, memberEntryID).Scan(&otherAdminsCount)
		if err != nil {
			return err
		}
		if otherAdminsCount == 0 {
			return fmt.Errorf("%w: cannot remove the last project admin", ErrBadRequest)
		}
	}

	res, err := tx.ExecContext(ctx, `DELETE FROM project_members WHERE id = $1 AND project_id = $2`, memberEntryID, projectID)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return tx.Commit()
}

// Statuses (1..10, unique display_order per project or system default)
func (s *Store) ListStatuses(ctx context.Context, projectID *string) ([]domain.Status, error) {
	var rows *sql.Rows
	var err error
	if projectID == nil {
		rows, err = s.db.QueryContext(ctx, `
			SELECT id, project_id, display_order, name, color, created_at
			FROM statuses WHERE project_id IS NULL ORDER BY display_order ASC
		`)
	} else {
		rows, err = s.db.QueryContext(ctx, `
			SELECT id, project_id, display_order, name, color, created_at
			FROM statuses WHERE project_id = $1 ORDER BY display_order ASC
		`, *projectID)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	statuses := []domain.Status{}
	for rows.Next() {
		var st domain.Status
		var pid sql.NullString
		if err := rows.Scan(&st.ID, &pid, &st.DisplayOrder, &st.Name, &st.Color, &st.CreatedAt); err != nil {
			return nil, err
		}
		if pid.Valid {
			st.ProjectID = &pid.String
		}
		statuses = append(statuses, st)
	}
	return statuses, rows.Err()
}

// ResetSystemStatuses resets system default statuses (project_id IS NULL) to the canonical 4 defaults.
func (s *Store) ResetSystemStatuses(ctx context.Context) ([]*domain.Status, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, `DELETE FROM statuses WHERE project_id IS NULL`); err != nil {
		return nil, err
	}
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
	now := time.Now().UTC()
	var list []*domain.Status
	for _, d := range defaults {
		statusID := id.NewV7String()
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO statuses (id, project_id, display_order, name, color, created_at)
			VALUES ($1, NULL, $2, $3, $4, $5)
		`, statusID, d.order, d.name, d.color, now); err != nil {
			return nil, err
		}
		list = append(list, &domain.Status{
			ID:           statusID,
			ProjectID:    nil,
			DisplayOrder: d.order,
			Name:         d.name,
			Color:        d.color,
			CreatedAt:    now,
		})
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return list, nil
}

func (s *Store) CreateStatus(ctx context.Context, projectID *string, displayOrder int, name, color string) (*domain.Status, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, fmt.Errorf("%w: status name is required", ErrBadRequest)
	}
	if displayOrder < 1 || displayOrder > 10 {
		return nil, fmt.Errorf("%w: display_order must be between 1 and 10", ErrBadRequest)
	}
	if color == "" {
		color = "#64748b"
	}

	existing, err := s.ListStatuses(ctx, projectID)
	if err != nil {
		return nil, err
	}
	if len(existing) >= domain.MaxStatusesPerProject {
		return nil, fmt.Errorf("%w: maximum %d statuses allowed", ErrBadRequest, domain.MaxStatusesPerProject)
	}
	for _, st := range existing {
		if st.DisplayOrder == displayOrder {
			return nil, fmt.Errorf("%w: display_order %d is already used", ErrConflict, displayOrder)
		}
	}

	statusID := id.NewV7String()
	now := time.Now().UTC()
	if _, err := s.db.ExecContext(ctx, `
		INSERT INTO statuses (id, project_id, display_order, name, color, created_at)
		VALUES ($1, $2, $3, $4, $5, $6)
	`, statusID, projectID, displayOrder, name, color, now); err != nil {
		return nil, err
	}
	return &domain.Status{
		ID:           statusID,
		ProjectID:    projectID,
		DisplayOrder: displayOrder,
		Name:         name,
		Color:        color,
		CreatedAt:    now,
	}, nil
}

func (s *Store) UpdateStatus(ctx context.Context, statusID string, displayOrder int, name, color string) (*domain.Status, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, fmt.Errorf("%w: status name is required", ErrBadRequest)
	}
	if displayOrder < 1 || displayOrder > 10 {
		return nil, fmt.Errorf("%w: display_order must be between 1 and 10", ErrBadRequest)
	}
	res, err := s.db.ExecContext(ctx, `
		UPDATE statuses SET display_order = $1, name = $2, color = $3 WHERE id = $4
	`, displayOrder, name, color, statusID)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrConflict, err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return nil, ErrNotFound
	}
	var st domain.Status
	var pid sql.NullString
	err = s.db.QueryRowContext(ctx, `
		SELECT id, project_id, display_order, name, color, created_at FROM statuses WHERE id = $1
	`, statusID).Scan(&st.ID, &pid, &st.DisplayOrder, &st.Name, &st.Color, &st.CreatedAt)
	if pid.Valid {
		st.ProjectID = &pid.String
	}
	return &st, err
}

// DeleteStatus deletes a status. If tasks belong to this status, migrateToStatusID must be provided.
func (s *Store) DeleteStatus(ctx context.Context, projectID *string, statusID string, migrateToStatusID *string) error {
	existing, err := s.ListStatuses(ctx, projectID)
	if err != nil {
		return err
	}
	if len(existing) <= domain.MinStatusesPerProject {
		return fmt.Errorf("%w: at least 1 status must remain", ErrBadRequest)
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var taskCount int
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM tasks WHERE status_id = $1`, statusID).Scan(&taskCount); err != nil {
		return err
	}
	if migrateToStatusID != nil && *migrateToStatusID != "" {
		if projectID != nil {
			var targetCount int
			if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM statuses WHERE id = $1 AND project_id = $2`, *migrateToStatusID, *projectID).Scan(&targetCount); err != nil {
				return err
			}
			if targetCount == 0 {
				return fmt.Errorf("%w: migration target status must belong to the same project", ErrBadRequest)
			}
		} else {
			var targetCount int
			if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM statuses WHERE id = $1 AND project_id IS NULL`, *migrateToStatusID).Scan(&targetCount); err != nil {
				return err
			}
			if targetCount == 0 {
				return fmt.Errorf("%w: migration target status must belong to system default statuses", ErrBadRequest)
			}
		}
	}
	if taskCount > 0 {
		if migrateToStatusID == nil || *migrateToStatusID == "" || *migrateToStatusID == statusID {
			return fmt.Errorf("%w: status has %d tasks; migrate_to_status_id is required", ErrBadRequest, taskCount)
		}
		if _, err := tx.ExecContext(ctx, `UPDATE tasks SET status_id = $1, updated_at = $2 WHERE status_id = $3`, *migrateToStatusID, time.Now().UTC(), statusID); err != nil {
			return err
		}
	}

	res, err := tx.ExecContext(ctx, `DELETE FROM statuses WHERE id = $1`, statusID)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return tx.Commit()
}

// Custom Field Definitions (9 types)
func isValidFieldType(ft string) bool {
	switch ft {
	case domain.FieldTypeTextSingle, domain.FieldTypeTextMulti, domain.FieldTypeMarkdown,
		domain.FieldTypeNumber, domain.FieldTypeDate, domain.FieldTypeRadio,
		domain.FieldTypeCheckbox, domain.FieldTypeSelect, domain.FieldTypeUserGroup:
		return true
	default:
		return false
	}
}

func (s *Store) ListCustomFields(ctx context.Context, projectID *string, targetType string) ([]domain.CustomFieldDefinition, error) {
	query := `SELECT id, project_id, target_type, name, field_type, options_json, created_at FROM custom_field_definitions WHERE `
	var args []any
	if projectID == nil {
		query += `project_id IS NULL`
	} else {
		query += `project_id = $1`
		args = append(args, *projectID)
	}
	if targetType != "" {
		args = append(args, targetType)
		query += fmt.Sprintf(` AND target_type = $%d`, len(args))
	}
	query += ` ORDER BY id ASC`

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	defs := []domain.CustomFieldDefinition{}
	for rows.Next() {
		var d domain.CustomFieldDefinition
		var pid sql.NullString
		if err := rows.Scan(&d.ID, &pid, &d.TargetType, &d.Name, &d.FieldType, &d.OptionsJSON, &d.CreatedAt); err != nil {
			return nil, err
		}
		if pid.Valid {
			d.ProjectID = &pid.String
		}
		defs = append(defs, d)
	}
	return defs, rows.Err()
}

func (s *Store) CreateCustomField(ctx context.Context, projectID *string, targetType, name, fieldType string, optionsJSON json.RawMessage) (*domain.CustomFieldDefinition, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, fmt.Errorf("%w: field name is required", ErrBadRequest)
	}
	if targetType != domain.TargetIssue && targetType != domain.TargetTask {
		return nil, fmt.Errorf("%w: target_type must be 'issue' or 'task'", ErrBadRequest)
	}
	if !isValidFieldType(fieldType) {
		return nil, fmt.Errorf("%w: invalid field_type '%s'", ErrBadRequest, fieldType)
	}
	if len(optionsJSON) == 0 {
		optionsJSON = json.RawMessage(`[]`)
	}
	defID := id.NewV7String()
	now := time.Now().UTC()
	if _, err := s.db.ExecContext(ctx, `
		INSERT INTO custom_field_definitions (id, project_id, target_type, name, field_type, options_json, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`, defID, projectID, targetType, name, fieldType, optionsJSON, now); err != nil {
		return nil, err
	}
	return &domain.CustomFieldDefinition{
		ID:          defID,
		ProjectID:   projectID,
		TargetType:  targetType,
		Name:        name,
		FieldType:   fieldType,
		OptionsJSON: optionsJSON,
		CreatedAt:   now,
	}, nil
}

func (s *Store) UpdateCustomField(ctx context.Context, defID, name string, optionsJSON json.RawMessage) (*domain.CustomFieldDefinition, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, fmt.Errorf("%w: field name is required", ErrBadRequest)
	}
	if len(optionsJSON) == 0 {
		optionsJSON = json.RawMessage(`[]`)
	}
	res, err := s.db.ExecContext(ctx, `
		UPDATE custom_field_definitions SET name = $1, options_json = $2 WHERE id = $3
	`, name, optionsJSON, defID)
	if err != nil {
		return nil, err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return nil, ErrNotFound
	}
	var d domain.CustomFieldDefinition
	var pid sql.NullString
	err = s.db.QueryRowContext(ctx, `
		SELECT id, project_id, target_type, name, field_type, options_json, created_at
		FROM custom_field_definitions WHERE id = $1
	`, defID).Scan(&d.ID, &pid, &d.TargetType, &d.Name, &d.FieldType, &d.OptionsJSON, &d.CreatedAt)
	if pid.Valid {
		d.ProjectID = &pid.String
	}
	return &d, err
}

func (s *Store) DeleteCustomField(ctx context.Context, defID string) error {
	res, err := s.db.ExecContext(ctx, `DELETE FROM custom_field_definitions WHERE id = $1`, defID)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

// Templates & 1-3 Column Layout
func (s *Store) ListTemplates(ctx context.Context, projectID *string, targetType string) ([]domain.Template, error) {
	query := `SELECT id FROM templates WHERE `
	var args []any
	if projectID == nil {
		query += `project_id IS NULL`
	} else {
		query += `project_id = $1`
		args = append(args, *projectID)
	}
	if targetType != "" {
		args = append(args, targetType)
		query += fmt.Sprintf(` AND target_type = $%d`, len(args))
	}
	query += ` ORDER BY id ASC`

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var ids []string
	for rows.Next() {
		var tid string
		if err := rows.Scan(&tid); err != nil {
			return nil, err
		}
		ids = append(ids, tid)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	templates := []domain.Template{}
	for _, tid := range ids {
		tpl, err := s.GetTemplate(ctx, tid)
		if err != nil {
			return nil, err
		}
		templates = append(templates, *tpl)
	}
	return templates, nil
}

func (s *Store) GetTemplate(ctx context.Context, templateID string) (*domain.Template, error) {
	var tpl domain.Template
	var pid sql.NullString
	err := s.db.QueryRowContext(ctx, `
		SELECT id, project_id, target_type, name, created_at, updated_at
		FROM templates WHERE id = $1
	`, templateID).Scan(&tpl.ID, &pid, &tpl.TargetType, &tpl.Name, &tpl.CreatedAt, &tpl.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	if pid.Valid {
		tpl.ProjectID = &pid.String
	}

	rowRecords, err := s.db.QueryContext(ctx, `
		SELECT id, template_id, row_order, columns_count
		FROM template_rows WHERE template_id = $1 ORDER BY row_order ASC
	`, templateID)
	if err != nil {
		return nil, err
	}
	defer rowRecords.Close()

	tpl.Rows = []domain.TemplateRow{}
	for rowRecords.Next() {
		var r domain.TemplateRow
		if err := rowRecords.Scan(&r.ID, &r.TemplateID, &r.RowOrder, &r.ColumnsCount); err != nil {
			return nil, err
		}
		r.Fields = []domain.TemplateField{}
		tpl.Rows = append(tpl.Rows, r)
	}
	if err := rowRecords.Err(); err != nil {
		return nil, err
	}

	for i := range tpl.Rows {
		fRecords, err := s.db.QueryContext(ctx, `
			SELECT id, template_row_id, column_order, field_source, field_key,
			       is_hidden, is_restricted, allow_owner_view, allow_owner_edit, view_subjects_json, edit_subjects_json
			FROM template_fields WHERE template_row_id = $1 ORDER BY column_order ASC
		`, tpl.Rows[i].ID)
		if err != nil {
			return nil, err
		}
		for fRecords.Next() {
			var f domain.TemplateField
			var viewRaw, editRaw []byte
			if err := fRecords.Scan(&f.ID, &f.TemplateRowID, &f.ColumnOrder, &f.FieldSource, &f.FieldKey, &f.IsHidden, &f.IsRestricted, &f.AllowOwnerView, &f.AllowOwnerEdit, &viewRaw, &editRaw); err != nil {
				fRecords.Close()
				return nil, err
			}
			_ = json.Unmarshal(viewRaw, &f.ViewSubjects)
			_ = json.Unmarshal(editRaw, &f.EditSubjects)
			if f.ViewSubjects == nil {
				f.ViewSubjects = []domain.SubjectRef{}
			}
			if f.EditSubjects == nil {
				f.EditSubjects = []domain.SubjectRef{}
			}
			tpl.Rows[i].Fields = append(tpl.Rows[i].Fields, f)
		}
		if err := fRecords.Err(); err != nil {
			fRecords.Close()
			return nil, err
		}
		fRecords.Close()
	}

	return &tpl, nil
}

func (s *Store) CreateTemplate(ctx context.Context, projectID *string, targetType, name string, rows []domain.TemplateRow) (*domain.Template, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, fmt.Errorf("%w: template name is required", ErrBadRequest)
	}
	if err := domain.ValidateTemplateLayout(targetType, rows); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrBadRequest, err)
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	tplID := id.NewV7String()
	now := time.Now().UTC()
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO templates (id, project_id, target_type, name, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $5)
	`, tplID, projectID, targetType, name, now); err != nil {
		return nil, err
	}

	if err := saveTemplateRowsTx(ctx, tx, tplID, rows); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return s.GetTemplate(ctx, tplID)
}

func (s *Store) UpdateTemplate(ctx context.Context, templateID, name string, rows []domain.TemplateRow) (*domain.Template, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, fmt.Errorf("%w: template name is required", ErrBadRequest)
	}
	existing, err := s.GetTemplate(ctx, templateID)
	if err != nil {
		return nil, err
	}
	if err := domain.ValidateTemplateLayout(existing.TargetType, rows); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrBadRequest, err)
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, `
		UPDATE templates SET name = $1, updated_at = $2 WHERE id = $3
	`, name, time.Now().UTC(), templateID); err != nil {
		return nil, err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM template_rows WHERE template_id = $1`, templateID); err != nil {
		return nil, err
	}
	if err := saveTemplateRowsTx(ctx, tx, templateID, rows); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return s.GetTemplate(ctx, templateID)
}

func (s *Store) DeleteTemplate(ctx context.Context, templateID string) error {
	res, err := s.db.ExecContext(ctx, `DELETE FROM templates WHERE id = $1`, templateID)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

func saveTemplateRowsTx(ctx context.Context, tx *sql.Tx, templateID string, rows []domain.TemplateRow) error {
	for idx, r := range rows {
		rowOrder := r.RowOrder
		if rowOrder <= 0 {
			rowOrder = idx + 1
		}
		rowID := id.NewV7String()
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO template_rows (id, template_id, row_order, columns_count)
			VALUES ($1, $2, $3, $4)
		`, rowID, templateID, rowOrder, r.ColumnsCount); err != nil {
			return err
		}
		for _, f := range r.Fields {
			viewSubjects := f.ViewSubjects
			if viewSubjects == nil {
				viewSubjects = []domain.SubjectRef{}
			}
			editSubjects := f.EditSubjects
			if editSubjects == nil {
				editSubjects = []domain.SubjectRef{}
			}
			viewJSON, _ := json.Marshal(viewSubjects)
			editJSON, _ := json.Marshal(editSubjects)
			if _, err := tx.ExecContext(ctx, `
				INSERT INTO template_fields (
					id, template_row_id, column_order, field_source, field_key,
					is_hidden, is_restricted, allow_owner_view, allow_owner_edit, view_subjects_json, edit_subjects_json
				) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
			`, id.NewV7String(), rowID, f.ColumnOrder, f.FieldSource, f.FieldKey, f.IsHidden, f.IsRestricted, f.AllowOwnerView, f.AllowOwnerEdit, viewJSON, editJSON); err != nil {
				return err
			}
		}
	}
	return nil
}

