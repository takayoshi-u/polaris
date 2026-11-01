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

type IssueCreateInput struct {
	TemplateID        *string
	TypeID            *string
	Title             string
	Description       string
	Premortem         string
	Postmortem        string
	LabelIDs          []string
	CustomValues      map[string]json.RawMessage
	CreateInitialTask bool
}

type IssueUpdateInput struct {
	TemplateID   *string
	TypeID       *string
	ClearTypeID  bool
	State        *string
	Title        *string
	Description  *string
	Premortem    *string
	Postmortem   *string
	LabelIDs     *[]string
	CustomValues map[string]json.RawMessage
	DisplayOrder *int
}

type TaskCreateInput struct {
	IssueID          string
	SprintID         *string
	TemplateID       *string
	TypeID           *string
	StatusID         *string
	Title            string
	Description      string
	Progress         int
	PlannedStartDate *string
	PlannedEndDate   *string
	ActualStartDate  *string
	ActualEndDate    *string
	PlannedHours     *float64
	ActualHours      *float64
	PlannedCost      *float64
	ActualCost       *float64
	StoryPoints      *float64
	Assignees        []domain.SubjectRef
	LabelIDs         []string
	CustomValues     map[string]json.RawMessage
}

type TaskUpdateInput struct {
	IssueID           *string
	SprintID          *string
	ClearSprintID     bool
	TemplateID        *string
	TypeID            *string
	ClearTypeID       bool
	StatusID          *string
	Title             *string
	Description       *string
	Progress          *int
	PlannedStartDate  *string
	PlannedEndDate    *string
	ActualStartDate   *string
	ActualEndDate     *string
	PlannedHours      *float64
	ClearPlannedHours bool
	ActualHours       *float64
	ClearActualHours  bool
	PlannedCost       *float64
	ClearPlannedCost  bool
	ActualCost        *float64
	ClearActualCost   bool
	StoryPoints       *float64
	ClearStoryPoints  bool
	Assignees         *[]domain.SubjectRef
	LabelIDs          *[]string
	CustomValues      map[string]json.RawMessage
	KanbanOrder       *int
	GanttOrder        *int
}

type IssueTaskFilter struct {
	LabelIDs   []string
	AssigneeID string
	StatusID   string
	TypeID     string
	SprintID   string
	IssueState string // "open" (default), "closed", "all"
	Keyword    string
}

func (s *Store) getProjectDoneStatusIDs(ctx context.Context, projectID string) map[string]bool {
	statuses, err := s.ListStatuses(ctx, &projectID)
	if err != nil {
		return make(map[string]bool)
	}
	return domain.ResolveDoneStatusIDs(statuses)
}

func (s *Store) getLowestOrderStatusID(ctx context.Context, projectID string) (string, error) {
	var statusID string
	err := s.db.QueryRowContext(ctx, `
		SELECT id FROM statuses WHERE project_id = $1 ORDER BY display_order ASC LIMIT 1
	`, projectID).Scan(&statusID)
	if err != nil {
		return "", fmt.Errorf("%w: no status found for project", ErrBadRequest)
	}
	return statusID, nil
}

func (s *Store) CreateIssue(ctx context.Context, projectID, creatorUserID string, in IssueCreateInput) (*domain.Issue, error) {
	title := strings.TrimSpace(in.Title)
	if title == "" {
		return nil, fmt.Errorf("%w: issue title is required", ErrBadRequest)
	}
	if len(in.Premortem) > 100000 {
		return nil, fmt.Errorf("%w: premortem exceeds maximum length of 100000 bytes", ErrBadRequest)
	}
	if len(in.Postmortem) > 100000 {
		return nil, fmt.Errorf("%w: postmortem exceeds maximum length of 100000 bytes", ErrBadRequest)
	}

	proj, err := s.GetProject(ctx, projectID, creatorUserID)
	if err != nil {
		return nil, err
	}
	if domain.RolePriority(proj.CurrentUserRole) < domain.RolePriority(domain.RoleEditor) {
		return nil, ErrForbidden
	}

	tplID := in.TemplateID
	if tplID == nil || *tplID == "" {
		tplID = proj.DefaultIssueTemplateID
	}

	var tpl *domain.Template
	if tplID != nil && *tplID != "" {
		tpl, _ = s.GetTemplate(ctx, *tplID)
	}
	userGroups, _ := s.GetUserGroupIDs(ctx, creatorUserID, &projectID)
	perms := s.computeTemplatePermissions(tpl, domain.TargetIssue, creatorUserID, userGroups, proj.CurrentUserRole, creatorUserID)

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	issueID := id.NewV7String()
	now := time.Now().UTC()

	desc := in.Description
	if p, ok := perms[domain.StdFieldDescription]; ok && !p.CanEdit {
		desc = ""
	}
	premortem := in.Premortem
	if p, ok := perms[domain.StdFieldPremortem]; ok && !p.CanEdit {
		premortem = ""
	}
	postmortem := in.Postmortem
	if p, ok := perms[domain.StdFieldPostmortem]; ok && !p.CanEdit {
		postmortem = ""
	}

	var maxOrder int
	_ = tx.QueryRowContext(ctx, `SELECT COALESCE(MAX(display_order), 0) FROM issues WHERE project_id = $1`, projectID).Scan(&maxOrder)

	var typeID *string
	if in.TypeID != nil && *in.TypeID != "" {
		it, err := s.GetType(ctx, *in.TypeID)
		if err != nil {
			return nil, fmt.Errorf("%w: invalid type_id", ErrBadRequest)
		}
		if it.ProjectID != nil && *it.ProjectID != projectID {
			return nil, fmt.Errorf("%w: type does not belong to project", ErrBadRequest)
		}
		typeID = in.TypeID
	}

	if _, err := tx.ExecContext(ctx, `
		INSERT INTO issues (id, project_id, template_id, type_id, owner_user_id, state, title, description, premortem, postmortem, display_order, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, 'open', $6, $7, $8, $9, $10, $11, $11)
	`, issueID, projectID, tplID, typeID, creatorUserID, title, desc, premortem, postmortem, maxOrder+1, now); err != nil {
		return nil, err
	}

	if p, ok := perms[domain.StdFieldLabels]; !ok || p.CanEdit {
		for _, lid := range in.LabelIDs {
			if _, err := tx.ExecContext(ctx, `
				INSERT INTO item_labels (id, target_type, target_id, label_id, created_at)
				VALUES ($1, 'issue', $2, $3, $4)
				ON CONFLICT DO NOTHING
			`, id.NewV7String(), issueID, lid, now); err != nil {
				return nil, err
			}
		}
	}

	for fieldDefID, rawVal := range in.CustomValues {
		// On creation, if a field is restricted and the creator/owner cannot edit it, skip saving its value
		if p, ok := perms[fieldDefID]; ok && !p.CanEdit {
			continue
		}
		if len(rawVal) == 0 {
			rawVal = json.RawMessage(`null`)
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO custom_field_values (id, target_type, target_id, field_definition_id, value_json, updated_at)
			VALUES ($1, 'issue', $2, $3, $4, $5)
			ON CONFLICT (target_type, target_id, field_definition_id)
			DO UPDATE SET value_json = EXCLUDED.value_json, updated_at = EXCLUDED.updated_at
		`, id.NewV7String(), issueID, fieldDefID, rawVal, now); err != nil {
			return nil, err
		}
	}

	// Simultaneous initial task creation option (Plan 2)
	if in.CreateInitialTask {
		initialStatusID, err := s.getLowestOrderStatusID(ctx, projectID)
		if err != nil {
			return nil, err
		}
		taskID := id.NewV7String()
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO tasks (
				id, project_id, issue_id, template_id, owner_user_id, status_id,
				title, description, progress, kanban_order, gantt_order, created_at, updated_at
			) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, 0, 1, 1, $9, $9)
		`, taskID, projectID, issueID, proj.DefaultTaskTemplateID, creatorUserID, initialStatusID, title, desc, now); err != nil {
			return nil, err
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return s.GetIssue(ctx, projectID, issueID, creatorUserID)
}

func (s *Store) GetIssue(ctx context.Context, projectID, issueID, viewerUserID string) (*domain.Issue, error) {
	proj, err := s.GetProject(ctx, projectID, viewerUserID)
	if err != nil {
		return nil, err
	}
	if !proj.IsMember {
		return nil, ErrForbidden
	}

	var iss domain.Issue
	var tplID, typeID sql.NullString
	var desc string
	err = s.db.QueryRowContext(ctx, `
		SELECT id, project_id, template_id, type_id, owner_user_id, state, title, description, premortem, postmortem, display_order, created_at, updated_at
		FROM issues WHERE id = $1 AND project_id = $2
	`, issueID, projectID).Scan(
		&iss.ID, &iss.ProjectID, &tplID, &typeID, &iss.OwnerUserID, &iss.State,
		&iss.Title, &desc, &iss.Premortem, &iss.Postmortem, &iss.DisplayOrder, &iss.CreatedAt, &iss.UpdatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	if tplID.Valid {
		iss.TemplateID = &tplID.String
	} else {
		iss.TemplateID = proj.DefaultIssueTemplateID
	}
	if typeID.Valid {
		iss.TypeID = &typeID.String
		if it, err := s.GetType(ctx, typeID.String); err == nil {
			iss.Type = it
		}
	}
	iss.Description = &desc
	iss.LabelIDs, _ = s.getItemLabelIDs(ctx, domain.TargetIssue, iss.ID)
	allCustomValues, _ := s.getItemCustomValues(ctx, domain.TargetIssue, iss.ID)

	userGroups, _ := s.GetUserGroupIDs(ctx, viewerUserID, &projectID)
	var tpl *domain.Template
	if iss.TemplateID != nil {
		tpl, _ = s.GetTemplate(ctx, *iss.TemplateID)
	}
	s.applyIssueFieldPermissions(&iss, tpl, allCustomValues, viewerUserID, userGroups, proj.CurrentUserRole)

	tasks, err := s.listTasksByIssue(ctx, proj, iss.ID, viewerUserID, userGroups)
	if err != nil {
		return nil, err
	}
	iss.Tasks = tasks
	doneStatusIDs := s.getProjectDoneStatusIDs(ctx, projectID)
	iss.Rollup = domain.CalculateIssueRollup(tasks, proj.HoursPerDay, doneStatusIDs)
	return &iss, nil
}

func (s *Store) ListIssuesWithTasks(ctx context.Context, projectID, viewerUserID string, filter IssueTaskFilter) ([]domain.Issue, error) {
	proj, err := s.GetProject(ctx, projectID, viewerUserID)
	if err != nil {
		return nil, err
	}
	if !proj.IsMember {
		return nil, ErrForbidden
	}

	if filter.SprintID == "active" {
		var activeSprintID string
		err := s.db.QueryRowContext(ctx, `
			SELECT id FROM sprints WHERE project_id = $1 AND status = 'active' ORDER BY created_at DESC LIMIT 1
		`, projectID).Scan(&activeSprintID)
		if err != nil {
			filter.SprintID = "__no_active_sprint__"
		} else {
			filter.SprintID = activeSprintID
		}
	}

	stateFilter := filter.IssueState
	if stateFilter == "" {
		stateFilter = domain.IssueStateOpen
	}

	rows, err := s.db.QueryContext(ctx, `
		SELECT id FROM issues WHERE project_id = $1 ORDER BY display_order ASC, id ASC
	`, projectID)
	if err != nil {
		return nil, err
	}
	var issueIDs []string
	for rows.Next() {
		var iid string
		if err := rows.Scan(&iid); err != nil {
			rows.Close()
			return nil, err
		}
		issueIDs = append(issueIDs, iid)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, err
	}
	rows.Close()

	var assigneeGroupIDs []string
	if filter.AssigneeID != "" {
		assigneeGroupIDs, _ = s.GetUserGroupIDs(ctx, filter.AssigneeID, &projectID)
	}

	issues := []domain.Issue{}
	for _, iid := range issueIDs {
		iss, err := s.GetIssue(ctx, projectID, iid, viewerUserID)
		if err != nil {
			continue
		}
		if stateFilter != "all" && iss.State != stateFilter {
			continue
		}

		// Filter child tasks if task-level filters or combined filters are active
		issueMatchesSelf := s.issueMatchesFilter(iss, filter)
		filteredTasks := []domain.Task{}
		for _, t := range iss.Tasks {
			if s.taskMatchesFilter(&t, filter, assigneeGroupIDs) {
				filteredTasks = append(filteredTasks, t)
			}
		}

		hasActiveTaskFilter := len(filter.LabelIDs) > 0 || filter.AssigneeID != "" || filter.StatusID != "" || filter.TypeID != "" || filter.SprintID != "" || strings.TrimSpace(filter.Keyword) != ""
		if hasActiveTaskFilter {
			if len(filteredTasks) == 0 && !issueMatchesSelf {
				continue
			}
			if len(filteredTasks) > 0 {
				iss.Tasks = filteredTasks
			}
		}
		issues = append(issues, *iss)
	}
	return issues, nil
}

func (s *Store) issueMatchesFilter(iss *domain.Issue, filter IssueTaskFilter) bool {
	if filter.AssigneeID != "" || filter.StatusID != "" || filter.SprintID != "" {
		return false
	}
	if filter.TypeID != "" {
		if iss.TypeID == nil || *iss.TypeID != filter.TypeID {
			return false
		}
	}
	if len(filter.LabelIDs) > 0 {
		if !hasAllLabels(iss.LabelIDs, filter.LabelIDs) {
			return false
		}
	}
	if kw := strings.ToLower(strings.TrimSpace(filter.Keyword)); kw != "" {
		desc := ""
		if iss.Description != nil {
			desc = *iss.Description
		}
		if !strings.Contains(strings.ToLower(iss.Title), kw) && !strings.Contains(strings.ToLower(desc), kw) {
			return false
		}
	}
	return true
}

func (s *Store) taskMatchesFilter(t *domain.Task, filter IssueTaskFilter, assigneeGroupIDs []string) bool {
	if filter.SprintID == "unassigned" {
		if t.SprintID != nil && *t.SprintID != "" {
			return false
		}
	} else if filter.SprintID != "" {
		if t.SprintID == nil || *t.SprintID != filter.SprintID {
			return false
		}
	}
	if filter.TypeID != "" {
		if t.TypeID == nil || *t.TypeID != filter.TypeID {
			return false
		}
	}
	if filter.StatusID != "" && t.StatusID != filter.StatusID {
		return false
	}
	if len(filter.LabelIDs) > 0 {
		if !hasAllLabels(t.LabelIDs, filter.LabelIDs) {
			return false
		}
	}
	if filter.AssigneeID != "" {
		if !domain.SubjectMatches(t.Assignees, filter.AssigneeID, assigneeGroupIDs) {
			// Also check direct group match if filter.AssigneeID is a group ID
			matchedGroup := false
			for _, a := range t.Assignees {
				if a.ID == filter.AssigneeID {
					matchedGroup = true
					break
				}
			}
			if !matchedGroup {
				return false
			}
		}
	}
	if kw := strings.ToLower(strings.TrimSpace(filter.Keyword)); kw != "" {
		desc := ""
		if t.Description != nil {
			desc = *t.Description
		}
		if !strings.Contains(strings.ToLower(t.Title), kw) && !strings.Contains(strings.ToLower(desc), kw) {
			return false
		}
	}
	return true
}

func hasAllLabels(itemLabels, requiredLabels []string) bool {
	set := make(map[string]bool, len(itemLabels))
	for _, l := range itemLabels {
		set[l] = true
	}
	for _, req := range requiredLabels {
		if !set[req] {
			return false
		}
	}
	return true
}

func (s *Store) UpdateIssue(ctx context.Context, projectID, issueID, actorUserID string, in IssueUpdateInput) (*domain.Issue, error) {
	proj, err := s.GetProject(ctx, projectID, actorUserID)
	if err != nil {
		return nil, err
	}
	if domain.RolePriority(proj.CurrentUserRole) < domain.RolePriority(domain.RoleEditor) {
		return nil, ErrForbidden
	}

	existing, err := s.GetIssue(ctx, projectID, issueID, actorUserID)
	if err != nil {
		return nil, err
	}

	tplID := existing.TemplateID
	if in.TemplateID != nil && *in.TemplateID != "" {
		tplID = in.TemplateID
	}
	var tpl *domain.Template
	if tplID != nil {
		tpl, _ = s.GetTemplate(ctx, *tplID)
	}
	userGroups, _ := s.GetUserGroupIDs(ctx, actorUserID, &projectID)
	perms := s.computeTemplatePermissions(tpl, domain.TargetIssue, actorUserID, userGroups, proj.CurrentUserRole, existing.OwnerUserID)

	if in.Title != nil {
		if p, ok := perms[domain.StdFieldTitle]; ok && !p.CanEdit {
			return nil, fmt.Errorf("%w: no edit permission for title", ErrForbidden)
		}
	}
	if in.Description != nil {
		if p, ok := perms[domain.StdFieldDescription]; ok && !p.CanEdit {
			return nil, fmt.Errorf("%w: no edit permission for description", ErrForbidden)
		}
	}
	if in.Premortem != nil {
		if p, ok := perms[domain.StdFieldPremortem]; ok && !p.CanEdit {
			return nil, fmt.Errorf("%w: no edit permission for premortem", ErrForbidden)
		}
		if len(*in.Premortem) > 100000 {
			return nil, fmt.Errorf("%w: premortem exceeds maximum length of 100000 bytes", ErrBadRequest)
		}
	}
	if in.Postmortem != nil {
		if p, ok := perms[domain.StdFieldPostmortem]; ok && !p.CanEdit {
			return nil, fmt.Errorf("%w: no edit permission for postmortem", ErrForbidden)
		}
		if len(*in.Postmortem) > 100000 {
			return nil, fmt.Errorf("%w: postmortem exceeds maximum length of 100000 bytes", ErrBadRequest)
		}
	}
	if in.LabelIDs != nil {
		if p, ok := perms[domain.StdFieldLabels]; ok && !p.CanEdit {
			return nil, fmt.Errorf("%w: no edit permission for labels", ErrForbidden)
		}
	}
	for fieldDefID := range in.CustomValues {
		if p, ok := perms[fieldDefID]; ok && !p.CanEdit {
			return nil, fmt.Errorf("%w: no edit permission for custom field %s", ErrForbidden, fieldDefID)
		}
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	now := time.Now().UTC()
	title := existing.Title
	if in.Title != nil {
		title = strings.TrimSpace(*in.Title)
		if title == "" {
			return nil, fmt.Errorf("%w: title cannot be empty", ErrBadRequest)
		}
	}
	var currentDesc, currentPremortem, currentPostmortem string
	_ = tx.QueryRowContext(ctx, `SELECT description, premortem, postmortem FROM issues WHERE id = $1`, issueID).Scan(&currentDesc, &currentPremortem, &currentPostmortem)
	if in.Description != nil {
		currentDesc = *in.Description
	}
	premortem := currentPremortem
	if in.Premortem != nil {
		premortem = *in.Premortem
	}
	postmortem := currentPostmortem
	if in.Postmortem != nil {
		postmortem = *in.Postmortem
	}
	state := existing.State
	if in.State != nil {
		if *in.State != domain.IssueStateOpen && *in.State != domain.IssueStateClosed {
			return nil, fmt.Errorf("%w: state must be 'open' or 'closed'", ErrBadRequest)
		}
		state = *in.State
	}
	displayOrder := existing.DisplayOrder
	if in.DisplayOrder != nil {
		displayOrder = *in.DisplayOrder
	}

	rawTypeID := existing.TypeID
	if in.ClearTypeID {
		rawTypeID = nil
	} else if in.TypeID != nil {
		if *in.TypeID != "" {
			it, err := s.GetType(ctx, *in.TypeID)
			if err != nil {
				return nil, fmt.Errorf("%w: invalid type_id", ErrBadRequest)
			}
			if it.ProjectID != nil && *it.ProjectID != projectID {
				return nil, fmt.Errorf("%w: type does not belong to project", ErrBadRequest)
			}
			rawTypeID = in.TypeID
		} else {
			rawTypeID = nil
		}
	}

	if _, err := tx.ExecContext(ctx, `
		UPDATE issues
		SET template_id = $1, type_id = $2, state = $3, title = $4, description = $5, premortem = $6, postmortem = $7, display_order = $8, updated_at = $9
		WHERE id = $10 AND project_id = $11
	`, tplID, rawTypeID, state, title, currentDesc, premortem, postmortem, displayOrder, now, issueID, projectID); err != nil {
		return nil, err
	}

	if in.LabelIDs != nil {
		if _, err := tx.ExecContext(ctx, `DELETE FROM item_labels WHERE target_type = 'issue' AND target_id = $1`, issueID); err != nil {
			return nil, err
		}
		for _, lid := range *in.LabelIDs {
			if _, err := tx.ExecContext(ctx, `
				INSERT INTO item_labels (id, target_type, target_id, label_id, created_at)
				VALUES ($1, 'issue', $2, $3, $4)
				ON CONFLICT DO NOTHING
			`, id.NewV7String(), issueID, lid, now); err != nil {
				return nil, err
			}
		}
	}

	// Custom values are upserted per field_definition_id (values from other templates are NOT deleted!)
	for fieldDefID, rawVal := range in.CustomValues {
		if len(rawVal) == 0 {
			rawVal = json.RawMessage(`null`)
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO custom_field_values (id, target_type, target_id, field_definition_id, value_json, updated_at)
			VALUES ($1, 'issue', $2, $3, $4, $5)
			ON CONFLICT (target_type, target_id, field_definition_id)
			DO UPDATE SET value_json = EXCLUDED.value_json, updated_at = EXCLUDED.updated_at
		`, id.NewV7String(), issueID, fieldDefID, rawVal, now); err != nil {
			return nil, err
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return s.GetIssue(ctx, projectID, issueID, actorUserID)
}

func (s *Store) TransferIssueOwner(ctx context.Context, projectID, issueID, actorUserID, newOwnerUserID string) (*domain.Issue, error) {
	proj, err := s.GetProject(ctx, projectID, actorUserID)
	if err != nil {
		return nil, err
	}
	existing, err := s.GetIssue(ctx, projectID, issueID, actorUserID)
	if err != nil {
		return nil, err
	}
	if actorUserID != existing.OwnerUserID && proj.CurrentUserRole != domain.RoleAdmin {
		return nil, fmt.Errorf("%w: only current owner or project admin can transfer ownership", ErrForbidden)
	}
	newOwnerRole, err := s.GetEffectiveProjectRole(ctx, projectID, newOwnerUserID)
	if err != nil || newOwnerRole == domain.RoleNone {
		return nil, fmt.Errorf("%w: new owner must be a member of the project", ErrBadRequest)
	}

	if _, err := s.db.ExecContext(ctx, `
		UPDATE issues SET owner_user_id = $1, updated_at = $2 WHERE id = $3 AND project_id = $4
	`, newOwnerUserID, time.Now().UTC(), issueID, projectID); err != nil {
		return nil, err
	}
	return s.GetIssue(ctx, projectID, issueID, actorUserID)
}

func (s *Store) DeleteIssue(ctx context.Context, projectID, issueID, actorUserID string) (int, error) {
	proj, err := s.GetProject(ctx, projectID, actorUserID)
	if err != nil {
		return 0, err
	}
	if domain.RolePriority(proj.CurrentUserRole) < domain.RolePriority(domain.RoleEditor) {
		return 0, ErrForbidden
	}

	var taskCount int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM tasks WHERE issue_id = $1 AND project_id = $2`, issueID, projectID).Scan(&taskCount); err != nil {
		return 0, err
	}

	// Clean up attachments on issue and its tasks before deleting
	rows, err := s.db.QueryContext(ctx, `SELECT id FROM tasks WHERE issue_id = $1 AND project_id = $2`, issueID, projectID)
	if err == nil {
		var tids []string
		for rows.Next() {
			var tid string
			if err := rows.Scan(&tid); err == nil {
				tids = append(tids, tid)
			}
		}
		_ = rows.Err()
		rows.Close()
		for _, tid := range tids {
			_ = s.deleteAllAttachmentsForTarget(ctx, domain.TargetTask, tid)
		}
	}
	_ = s.deleteAllAttachmentsForTarget(ctx, domain.TargetIssue, issueID)

	res, err := s.db.ExecContext(ctx, `DELETE FROM issues WHERE id = $1 AND project_id = $2`, issueID, projectID)
	if err != nil {
		return 0, err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return 0, ErrNotFound
	}
	return taskCount, nil
}

// Tasks
func (s *Store) CreateTask(ctx context.Context, projectID, creatorUserID string, in TaskCreateInput) (*domain.Task, error) {
	title := strings.TrimSpace(in.Title)
	if title == "" {
		return nil, fmt.Errorf("%w: task title is required", ErrBadRequest)
	}
	if strings.TrimSpace(in.IssueID) == "" {
		return nil, fmt.Errorf("%w: issue_id is required (every task must belong to an issue)", ErrBadRequest)
	}
	if in.Progress < 0 || in.Progress > 100 {
		return nil, fmt.Errorf("%w: progress must be between 0 and 100", ErrBadRequest)
	}
	if err := validateDateRange(in.PlannedStartDate, in.PlannedEndDate); err != nil {
		return nil, err
	}
	if err := validateDateRange(in.ActualStartDate, in.ActualEndDate); err != nil {
		return nil, err
	}
	if err := domain.ValidateTaskHours(in.PlannedHours, "planned_hours"); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrBadRequest, err)
	}
	if err := domain.ValidateTaskHours(in.ActualHours, "actual_hours"); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrBadRequest, err)
	}
	if err := domain.ValidateTaskCost(in.PlannedCost, "planned_cost"); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrBadRequest, err)
	}
	if err := domain.ValidateTaskCost(in.ActualCost, "actual_cost"); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrBadRequest, err)
	}

	proj, err := s.GetProject(ctx, projectID, creatorUserID)
	if err != nil {
		return nil, err
	}
	if domain.RolePriority(proj.CurrentUserRole) < domain.RolePriority(domain.RoleEditor) {
		return nil, ErrForbidden
	}

	// Verify parent issue belongs to project
	var issueExists int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM issues WHERE id = $1 AND project_id = $2`, in.IssueID, projectID).Scan(&issueExists); err != nil || issueExists == 0 {
		return nil, fmt.Errorf("%w: parent issue not found in project", ErrBadRequest)
	}

	statusID := ""
	if in.StatusID != nil && *in.StatusID != "" {
		statusID = *in.StatusID
	} else {
		statusID, err = s.getLowestOrderStatusID(ctx, projectID)
		if err != nil {
			return nil, err
		}
	}

	tplID := in.TemplateID
	if tplID == nil || *tplID == "" {
		tplID = proj.DefaultTaskTemplateID
	}

	var tpl *domain.Template
	if tplID != nil {
		tpl, _ = s.GetTemplate(ctx, *tplID)
	}
	userGroups, _ := s.GetUserGroupIDs(ctx, creatorUserID, &projectID)
	perms := s.computeTemplatePermissions(tpl, domain.TargetTask, creatorUserID, userGroups, proj.CurrentUserRole, creatorUserID)

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	taskID := id.NewV7String()
	now := time.Now().UTC()

	desc := in.Description
	if p, ok := perms[domain.StdFieldDescription]; ok && !p.CanEdit {
		desc = ""
	}
	plannedCost := in.PlannedCost
	if p, ok := perms[domain.StdFieldPlannedCost]; ok && !p.CanEdit {
		plannedCost = nil
	}
	actualCost := in.ActualCost
	if p, ok := perms[domain.StdFieldActualCost]; ok && !p.CanEdit {
		actualCost = nil
	}

	var maxKanban, maxGantt int
	_ = tx.QueryRowContext(ctx, `SELECT COALESCE(MAX(kanban_order), 0), COALESCE(MAX(gantt_order), 0) FROM tasks WHERE issue_id = $1`, in.IssueID).Scan(&maxKanban, &maxGantt)

	var taskTypeID *string
	if in.TypeID != nil && *in.TypeID != "" {
		it, err := s.GetType(ctx, *in.TypeID)
		if err != nil {
			return nil, fmt.Errorf("%w: invalid type_id", ErrBadRequest)
		}
		if it.ProjectID != nil && *it.ProjectID != projectID {
			return nil, fmt.Errorf("%w: type does not belong to project", ErrBadRequest)
		}
		taskTypeID = in.TypeID
	}

	if _, err := tx.ExecContext(ctx, `
		INSERT INTO tasks (
			id, project_id, issue_id, sprint_id, template_id, type_id, owner_user_id, status_id,
			title, description, progress, planned_start_date, planned_end_date,
			actual_start_date, actual_end_date, planned_hours, actual_hours,
			planned_cost, actual_cost, story_points,
			kanban_order, gantt_order, created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20, $21, $22, $23, $23)
	`, taskID, projectID, in.IssueID, in.SprintID, tplID, taskTypeID, creatorUserID, statusID, title, desc, in.Progress,
		nullDate(in.PlannedStartDate), nullDate(in.PlannedEndDate),
		nullDate(in.ActualStartDate), nullDate(in.ActualEndDate),
		in.PlannedHours, in.ActualHours, plannedCost, actualCost, in.StoryPoints, maxKanban+1, maxGantt+1, now); err != nil {
		return nil, err
	}

	if p, ok := perms[domain.StdFieldAssignees]; !ok || p.CanEdit {
		for _, a := range in.Assignees {
			if _, err := tx.ExecContext(ctx, `
				INSERT INTO task_assignees (id, task_id, assignee_type, assignee_id, created_at)
				VALUES ($1, $2, $3, $4, $5)
				ON CONFLICT DO NOTHING
			`, id.NewV7String(), taskID, a.Type, a.ID, now); err != nil {
				return nil, err
			}
		}
	}

	if p, ok := perms[domain.StdFieldLabels]; !ok || p.CanEdit {
		for _, lid := range in.LabelIDs {
			if _, err := tx.ExecContext(ctx, `
				INSERT INTO item_labels (id, target_type, target_id, label_id, created_at)
				VALUES ($1, 'task', $2, $3, $4)
				ON CONFLICT DO NOTHING
			`, id.NewV7String(), taskID, lid, now); err != nil {
				return nil, err
			}
		}
	}

	for fieldDefID, rawVal := range in.CustomValues {
		if p, ok := perms[fieldDefID]; ok && !p.CanEdit {
			continue
		}
		if len(rawVal) == 0 {
			rawVal = json.RawMessage(`null`)
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO custom_field_values (id, target_type, target_id, field_definition_id, value_json, updated_at)
			VALUES ($1, 'task', $2, $3, $4, $5)
			ON CONFLICT (target_type, target_id, field_definition_id)
			DO UPDATE SET value_json = EXCLUDED.value_json, updated_at = EXCLUDED.updated_at
		`, id.NewV7String(), taskID, fieldDefID, rawVal, now); err != nil {
			return nil, err
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}

	_ = s.LogActivity(ctx, projectID, "task", taskID, "created", map[string]interface{}{"title": title}, &creatorUserID)

	// Notify assignees (excluding creatorUserID)
	if len(in.Assignees) > 0 {
		_ = s.notifySubjects(ctx, projectID, domain.NotificationTypeAssign, domain.TargetTask, taskID, creatorUserID, in.Assignees, fmt.Sprintf("タスク「%s」の担当者に割り当てられました", title))
	}

	return s.GetTask(ctx, projectID, taskID, creatorUserID)
}

func (s *Store) GetTask(ctx context.Context, projectID, taskID, viewerUserID string) (*domain.Task, error) {
	proj, err := s.GetProject(ctx, projectID, viewerUserID)
	if err != nil {
		return nil, err
	}
	if !proj.IsMember {
		return nil, ErrForbidden
	}
	userGroups, _ := s.GetUserGroupIDs(ctx, viewerUserID, &projectID)
	return s.getTaskInternal(ctx, proj, taskID, viewerUserID, userGroups)
}

func (s *Store) getTaskInternal(ctx context.Context, proj *domain.Project, taskID, viewerUserID string, userGroups []string) (*domain.Task, error) {
	var t domain.Task
	var tplID, sprintID, typeID sql.NullString
	var desc string
	var pStart, pEnd, aStart, aEnd sql.NullTime
	var pHours, aHours, pCost, aCost, storyPoints sql.NullFloat64

	err := s.db.QueryRowContext(ctx, `
		SELECT id, project_id, issue_id, sprint_id, template_id, type_id, owner_user_id, status_id,
		       title, description, progress, planned_start_date, planned_end_date,
		       actual_start_date, actual_end_date, planned_hours, actual_hours,
		       planned_cost, actual_cost, story_points,
		       kanban_order, gantt_order, created_at, updated_at
		FROM tasks WHERE id = $1 AND project_id = $2
	`, taskID, proj.ID).Scan(
		&t.ID, &t.ProjectID, &t.IssueID, &sprintID, &tplID, &typeID, &t.OwnerUserID, &t.StatusID,
		&t.Title, &desc, &t.Progress, &pStart, &pEnd,
		&aStart, &aEnd, &pHours, &aHours,
		&pCost, &aCost, &storyPoints,
		&t.KanbanOrder, &t.GanttOrder, &t.CreatedAt, &t.UpdatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	if sprintID.Valid {
		t.SprintID = &sprintID.String
	}
	if typeID.Valid {
		t.TypeID = &typeID.String
		if it, err := s.GetType(ctx, typeID.String); err == nil {
			t.Type = it
		}
	}
	if storyPoints.Valid {
		v := storyPoints.Float64
		t.StoryPoints = &v
	}
	if tplID.Valid {
		t.TemplateID = &tplID.String
	} else {
		t.TemplateID = proj.DefaultTaskTemplateID
	}
	t.Description = &desc
	if pStart.Valid {
		v := pStart.Time.Format("2006-01-02")
		t.PlannedStartDate = &v
	}
	if pEnd.Valid {
		v := pEnd.Time.Format("2006-01-02")
		t.PlannedEndDate = &v
	}
	if aStart.Valid {
		v := aStart.Time.Format("2006-01-02")
		t.ActualStartDate = &v
	}
	if aEnd.Valid {
		v := aEnd.Time.Format("2006-01-02")
		t.ActualEndDate = &v
	}
	if pHours.Valid {
		v := pHours.Float64
		t.PlannedHours = &v
	}
	if aHours.Valid {
		v := aHours.Float64
		t.ActualHours = &v
	}
	if pCost.Valid {
		v := pCost.Float64
		t.PlannedCost = &v
	}
	if aCost.Valid {
		v := aCost.Float64
		t.ActualCost = &v
	}

	t.Assignees, _ = s.getTaskAssignees(ctx, t.ID)
	t.LabelIDs, _ = s.getItemLabelIDs(ctx, domain.TargetTask, t.ID)
	allCustomValues, _ := s.getItemCustomValues(ctx, domain.TargetTask, t.ID)

	var tpl *domain.Template
	if t.TemplateID != nil {
		tpl, _ = s.GetTemplate(ctx, *t.TemplateID)
	}
	s.applyTaskFieldPermissions(&t, tpl, allCustomValues, viewerUserID, userGroups, proj.CurrentUserRole)
	return &t, nil
}

func (s *Store) listTasksByIssue(ctx context.Context, proj *domain.Project, issueID, viewerUserID string, userGroups []string) ([]domain.Task, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id FROM tasks WHERE issue_id = $1 AND project_id = $2 ORDER BY gantt_order ASC, kanban_order ASC, id ASC
	`, issueID, proj.ID)
	if err != nil {
		return nil, err
	}
	var ids []string
	for rows.Next() {
		var tid string
		if err := rows.Scan(&tid); err != nil {
			rows.Close()
			return nil, err
		}
		ids = append(ids, tid)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, err
	}
	rows.Close()

	tasks := []domain.Task{}
	for _, tid := range ids {
		t, err := s.getTaskInternal(ctx, proj, tid, viewerUserID, userGroups)
		if err == nil {
			tasks = append(tasks, *t)
		}
	}
	return tasks, nil
}

func (s *Store) UpdateTask(ctx context.Context, projectID, taskID, actorUserID string, in TaskUpdateInput) (*domain.Task, error) {
	proj, err := s.GetProject(ctx, projectID, actorUserID)
	if err != nil {
		return nil, err
	}
	if domain.RolePriority(proj.CurrentUserRole) < domain.RolePriority(domain.RoleEditor) {
		return nil, ErrForbidden
	}

	var rawOwnerID, rawIssueID, rawStatusID, rawTitle, rawDesc string
	var rawTplID, currentSprintID, currentTypeID sql.NullString
	var rawProgress, rawKanbanOrder, rawGanttOrder int
	var pStart, pEnd, aStart, aEnd sql.NullTime
	var pHours, aHours, pCost, aCost, currentSP sql.NullFloat64

	err = s.db.QueryRowContext(ctx, `
		SELECT issue_id, template_id, sprint_id, type_id, story_points, owner_user_id, status_id, title, description, progress,
		       planned_start_date, planned_end_date, actual_start_date, actual_end_date,
		       planned_hours, actual_hours, planned_cost, actual_cost, kanban_order, gantt_order
		FROM tasks WHERE id = $1 AND project_id = $2
	`, taskID, projectID).Scan(
		&rawIssueID, &rawTplID, &currentSprintID, &currentTypeID, &currentSP, &rawOwnerID, &rawStatusID, &rawTitle, &rawDesc, &rawProgress,
		&pStart, &pEnd, &aStart, &aEnd, &pHours, &aHours, &pCost, &aCost, &rawKanbanOrder, &rawGanttOrder,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}

	tplID := proj.DefaultTaskTemplateID
	if rawTplID.Valid {
		tplID = &rawTplID.String
	}
	if in.TemplateID != nil && *in.TemplateID != "" {
		tplID = in.TemplateID
	}

	var tpl *domain.Template
	if tplID != nil {
		tpl, _ = s.GetTemplate(ctx, *tplID)
	}
	userGroups, _ := s.GetUserGroupIDs(ctx, actorUserID, &projectID)
	perms := s.computeTemplatePermissions(tpl, domain.TargetTask, actorUserID, userGroups, proj.CurrentUserRole, rawOwnerID)

	if in.Title != nil {
		if p, ok := perms[domain.StdFieldTitle]; ok && !p.CanEdit {
			return nil, fmt.Errorf("%w: no edit permission for title", ErrForbidden)
		}
		rawTitle = strings.TrimSpace(*in.Title)
		if rawTitle == "" {
			return nil, fmt.Errorf("%w: title cannot be empty", ErrBadRequest)
		}
	}
	if in.Description != nil {
		if p, ok := perms[domain.StdFieldDescription]; ok && !p.CanEdit {
			return nil, fmt.Errorf("%w: no edit permission for description", ErrForbidden)
		}
		rawDesc = *in.Description
	}
	if in.StatusID != nil {
		if p, ok := perms[domain.StdFieldStatus]; ok && !p.CanEdit {
			return nil, fmt.Errorf("%w: no edit permission for status", ErrForbidden)
		}
		rawStatusID = *in.StatusID
	}
	if in.Progress != nil {
		if p, ok := perms[domain.StdFieldProgress]; ok && !p.CanEdit {
			return nil, fmt.Errorf("%w: no edit permission for progress", ErrForbidden)
		}
		if *in.Progress < 0 || *in.Progress > 100 {
			return nil, fmt.Errorf("%w: progress must be between 0 and 100", ErrBadRequest)
		}
		rawProgress = *in.Progress
	}

	plannedStart := timePtrStr(pStart)
	plannedEnd := timePtrStr(pEnd)
	if in.PlannedStartDate != nil || in.PlannedEndDate != nil {
		if p, ok := perms[domain.StdFieldPlannedDates]; ok && !p.CanEdit {
			return nil, fmt.Errorf("%w: no edit permission for planned dates", ErrForbidden)
		}
		if in.PlannedStartDate != nil {
			plannedStart = in.PlannedStartDate
		}
		if in.PlannedEndDate != nil {
			plannedEnd = in.PlannedEndDate
		}
		if err := validateDateRange(plannedStart, plannedEnd); err != nil {
			return nil, err
		}
	}

	actualStart := timePtrStr(aStart)
	actualEnd := timePtrStr(aEnd)
	if in.ActualStartDate != nil || in.ActualEndDate != nil {
		if p, ok := perms[domain.StdFieldActualDates]; ok && !p.CanEdit {
			return nil, fmt.Errorf("%w: no edit permission for actual dates", ErrForbidden)
		}
		if in.ActualStartDate != nil {
			actualStart = in.ActualStartDate
		}
		if in.ActualEndDate != nil {
			actualEnd = in.ActualEndDate
		}
		if err := validateDateRange(actualStart, actualEnd); err != nil {
			return nil, err
		}
	}

	plannedHours := floatPtr(pHours)
	if in.ClearPlannedHours {
		if p, ok := perms[domain.StdFieldPlannedHours]; ok && !p.CanEdit {
			return nil, fmt.Errorf("%w: no edit permission for planned hours", ErrForbidden)
		}
		plannedHours = nil
	} else if in.PlannedHours != nil {
		if p, ok := perms[domain.StdFieldPlannedHours]; ok && !p.CanEdit {
			return nil, fmt.Errorf("%w: no edit permission for planned hours", ErrForbidden)
		}
		if err := domain.ValidateTaskHours(in.PlannedHours, "planned_hours"); err != nil {
			return nil, fmt.Errorf("%w: %v", ErrBadRequest, err)
		}
		plannedHours = in.PlannedHours
	}

	actualHours := floatPtr(aHours)
	if in.ClearActualHours {
		if p, ok := perms[domain.StdFieldActualHours]; ok && !p.CanEdit {
			return nil, fmt.Errorf("%w: no edit permission for actual hours", ErrForbidden)
		}
		actualHours = nil
	} else if in.ActualHours != nil {
		if p, ok := perms[domain.StdFieldActualHours]; ok && !p.CanEdit {
			return nil, fmt.Errorf("%w: no edit permission for actual hours", ErrForbidden)
		}
		if err := domain.ValidateTaskHours(in.ActualHours, "actual_hours"); err != nil {
			return nil, fmt.Errorf("%w: %v", ErrBadRequest, err)
		}
		actualHours = in.ActualHours
	}

	plannedCost := floatPtr(pCost)
	if in.ClearPlannedCost {
		if p, ok := perms[domain.StdFieldPlannedCost]; ok && !p.CanEdit {
			return nil, fmt.Errorf("%w: no edit permission for planned cost", ErrForbidden)
		}
		plannedCost = nil
	} else if in.PlannedCost != nil {
		if p, ok := perms[domain.StdFieldPlannedCost]; ok && !p.CanEdit {
			return nil, fmt.Errorf("%w: no edit permission for planned cost", ErrForbidden)
		}
		if err := domain.ValidateTaskCost(in.PlannedCost, "planned_cost"); err != nil {
			return nil, fmt.Errorf("%w: %v", ErrBadRequest, err)
		}
		plannedCost = in.PlannedCost
	}

	actualCost := floatPtr(aCost)
	if in.ClearActualCost {
		if p, ok := perms[domain.StdFieldActualCost]; ok && !p.CanEdit {
			return nil, fmt.Errorf("%w: no edit permission for actual cost", ErrForbidden)
		}
		actualCost = nil
	} else if in.ActualCost != nil {
		if p, ok := perms[domain.StdFieldActualCost]; ok && !p.CanEdit {
			return nil, fmt.Errorf("%w: no edit permission for actual cost", ErrForbidden)
		}
		if err := domain.ValidateTaskCost(in.ActualCost, "actual_cost"); err != nil {
			return nil, fmt.Errorf("%w: %v", ErrBadRequest, err)
		}
		actualCost = in.ActualCost
	}

	if in.Assignees != nil {
		if p, ok := perms[domain.StdFieldAssignees]; ok && !p.CanEdit {
			return nil, fmt.Errorf("%w: no edit permission for assignees", ErrForbidden)
		}
	}
	if in.LabelIDs != nil {
		if p, ok := perms[domain.StdFieldLabels]; ok && !p.CanEdit {
			return nil, fmt.Errorf("%w: no edit permission for labels", ErrForbidden)
		}
	}
	for fieldDefID := range in.CustomValues {
		if p, ok := perms[fieldDefID]; ok && !p.CanEdit {
			return nil, fmt.Errorf("%w: no edit permission for custom field %s", ErrForbidden, fieldDefID)
		}
	}

	var rawSprintID *string
	if currentSprintID.Valid {
		rawSprintID = &currentSprintID.String
	}
	if in.ClearSprintID {
		rawSprintID = nil
	} else if in.SprintID != nil {
		rawSprintID = in.SprintID
	}

	var rawStoryPoints *float64
	if currentSP.Valid {
		v := currentSP.Float64
		rawStoryPoints = &v
	}
	if in.ClearStoryPoints {
		rawStoryPoints = nil
	} else if in.StoryPoints != nil {
		rawStoryPoints = in.StoryPoints
	}

	var rawTypeID *string
	if currentTypeID.Valid {
		rawTypeID = &currentTypeID.String
	}
	if in.ClearTypeID {
		rawTypeID = nil
	} else if in.TypeID != nil {
		if *in.TypeID != "" {
			it, err := s.GetType(ctx, *in.TypeID)
			if err != nil {
				return nil, fmt.Errorf("%w: invalid type_id", ErrBadRequest)
			}
			if it.ProjectID != nil && *it.ProjectID != projectID {
				return nil, fmt.Errorf("%w: type does not belong to project", ErrBadRequest)
			}
			rawTypeID = in.TypeID
		} else {
			rawTypeID = nil
		}
	}

	if in.IssueID != nil && *in.IssueID != "" {
		var issueExists int
		if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM issues WHERE id = $1 AND project_id = $2`, *in.IssueID, projectID).Scan(&issueExists); err != nil || issueExists == 0 {
			return nil, fmt.Errorf("%w: parent issue not found in project", ErrBadRequest)
		}
		rawIssueID = *in.IssueID
	}
	if in.KanbanOrder != nil {
		rawKanbanOrder = *in.KanbanOrder
	}
	if in.GanttOrder != nil {
		rawGanttOrder = *in.GanttOrder
	}

	oldAssignees, _ := s.getTaskAssignees(ctx, taskID)

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	now := time.Now().UTC()
	if _, err := tx.ExecContext(ctx, `
		UPDATE tasks
		SET issue_id = $1, sprint_id = $2, template_id = $3, type_id = $4, status_id = $5, title = $6, description = $7,
		    progress = $8, planned_start_date = $9, planned_end_date = $10,
		    actual_start_date = $11, actual_end_date = $12, planned_hours = $13, actual_hours = $14,
		    planned_cost = $15, actual_cost = $16, story_points = $17,
		    kanban_order = $18, gantt_order = $19, updated_at = $20
		WHERE id = $21 AND project_id = $22
	`, rawIssueID, rawSprintID, tplID, rawTypeID, rawStatusID, rawTitle, rawDesc, rawProgress,
		nullDate(plannedStart), nullDate(plannedEnd), nullDate(actualStart), nullDate(actualEnd),
		plannedHours, actualHours, plannedCost, actualCost, rawStoryPoints, rawKanbanOrder, rawGanttOrder, now, taskID, projectID); err != nil {
		return nil, err
	}

	var addedAssignees []domain.SubjectRef
	if in.Assignees != nil {
		if _, err := tx.ExecContext(ctx, `DELETE FROM task_assignees WHERE task_id = $1`, taskID); err != nil {
			return nil, err
		}
		oldSet := make(map[string]bool)
		for _, oa := range oldAssignees {
			oldSet[oa.Type+":"+oa.ID] = true
		}
		for _, a := range *in.Assignees {
			if _, err := tx.ExecContext(ctx, `
				INSERT INTO task_assignees (id, task_id, assignee_type, assignee_id, created_at)
				VALUES ($1, $2, $3, $4, $5)
				ON CONFLICT DO NOTHING
			`, id.NewV7String(), taskID, a.Type, a.ID, now); err != nil {
				return nil, err
			}
			if !oldSet[a.Type+":"+a.ID] {
				addedAssignees = append(addedAssignees, a)
			}
		}
	}

	if in.LabelIDs != nil {
		if _, err := tx.ExecContext(ctx, `DELETE FROM item_labels WHERE target_type = 'task' AND target_id = $1`, taskID); err != nil {
			return nil, err
		}
		for _, lid := range *in.LabelIDs {
			if _, err := tx.ExecContext(ctx, `
				INSERT INTO item_labels (id, target_type, target_id, label_id, created_at)
				VALUES ($1, 'task', $2, $3, $4)
				ON CONFLICT DO NOTHING
			`, id.NewV7String(), taskID, lid, now); err != nil {
				return nil, err
			}
		}
	}

	for fieldDefID, rawVal := range in.CustomValues {
		if len(rawVal) == 0 {
			rawVal = json.RawMessage(`null`)
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO custom_field_values (id, target_type, target_id, field_definition_id, value_json, updated_at)
			VALUES ($1, 'task', $2, $3, $4, $5)
			ON CONFLICT (target_type, target_id, field_definition_id)
			DO UPDATE SET value_json = EXCLUDED.value_json, updated_at = EXCLUDED.updated_at
		`, id.NewV7String(), taskID, fieldDefID, rawVal, now); err != nil {
			return nil, err
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}

	_ = s.LogActivity(ctx, projectID, "task", taskID, "updated", map[string]interface{}{"title": rawTitle, "progress": rawProgress, "status_id": rawStatusID}, &actorUserID)

	if len(addedAssignees) > 0 {
		_ = s.notifySubjects(ctx, projectID, domain.NotificationTypeAssign, domain.TargetTask, taskID, actorUserID, addedAssignees, fmt.Sprintf("タスク「%s」の担当者に割り当てられました", rawTitle))
	}

	return s.GetTask(ctx, projectID, taskID, actorUserID)
}

func (s *Store) TransferTaskOwner(ctx context.Context, projectID, taskID, actorUserID, newOwnerUserID string) (*domain.Task, error) {
	proj, err := s.GetProject(ctx, projectID, actorUserID)
	if err != nil {
		return nil, err
	}
	existing, err := s.GetTask(ctx, projectID, taskID, actorUserID)
	if err != nil {
		return nil, err
	}
	if actorUserID != existing.OwnerUserID && proj.CurrentUserRole != domain.RoleAdmin {
		return nil, fmt.Errorf("%w: only current owner or project admin can transfer ownership", ErrForbidden)
	}
	newOwnerRole, err := s.GetEffectiveProjectRole(ctx, projectID, newOwnerUserID)
	if err != nil || newOwnerRole == domain.RoleNone {
		return nil, fmt.Errorf("%w: new owner must be a member of the project", ErrBadRequest)
	}

	if _, err := s.db.ExecContext(ctx, `
		UPDATE tasks SET owner_user_id = $1, updated_at = $2 WHERE id = $3 AND project_id = $4
	`, newOwnerUserID, time.Now().UTC(), taskID, projectID); err != nil {
		return nil, err
	}
	return s.GetTask(ctx, projectID, taskID, actorUserID)
}

// MoveTaskKanban handles horizontal (status_id), vertical (issue_id), and in-cell (kanban_order) drag & drop on Kanban.
// Progress (%) is NOT automatically changed when moving to the final status.
func (s *Store) MoveTaskKanban(ctx context.Context, projectID, taskID, actorUserID string, targetIssueID, targetStatusID string, kanbanOrder int) (*domain.Task, error) {
	in := TaskUpdateInput{
		KanbanOrder: &kanbanOrder,
	}
	if targetIssueID != "" {
		in.IssueID = &targetIssueID
	}
	if targetStatusID != "" {
		in.StatusID = &targetStatusID
	}
	return s.UpdateTask(ctx, projectID, taskID, actorUserID, in)
}

// UpdateTaskGanttDates handles drag & drop of Planned or Actual bars on the Gantt chart (including creating bars for unset dates).
func (s *Store) UpdateTaskGanttDates(ctx context.Context, projectID, taskID, actorUserID string, plannedStart, plannedEnd, actualStart, actualEnd *string, ganttOrder *int) (*domain.Task, error) {
	in := TaskUpdateInput{
		PlannedStartDate: plannedStart,
		PlannedEndDate:   plannedEnd,
		ActualStartDate:  actualStart,
		ActualEndDate:    actualEnd,
		GanttOrder:       ganttOrder,
	}
	return s.UpdateTask(ctx, projectID, taskID, actorUserID, in)
}

func (s *Store) DeleteTask(ctx context.Context, projectID, taskID, actorUserID string) error {
	proj, err := s.GetProject(ctx, projectID, actorUserID)
	if err != nil {
		return err
	}
	if domain.RolePriority(proj.CurrentUserRole) < domain.RolePriority(domain.RoleEditor) {
		return ErrForbidden
	}
	_ = s.deleteAllAttachmentsForTarget(ctx, domain.TargetTask, taskID)
	res, err := s.db.ExecContext(ctx, `DELETE FROM tasks WHERE id = $1 AND project_id = $2`, taskID, projectID)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	_ = s.LogActivity(ctx, projectID, "task", taskID, "deleted", map[string]interface{}{"task_id": taskID}, &actorUserID)
	return nil
}

// Helpers for labels, assignees, custom values, and field-level permissions
func (s *Store) getTaskAssignees(ctx context.Context, taskID string) ([]domain.SubjectRef, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT assignee_type, assignee_id FROM task_assignees WHERE task_id = $1 ORDER BY created_at ASC
	`, taskID)
	if err != nil {
		return []domain.SubjectRef{}, err
	}
	defer rows.Close()
	list := []domain.SubjectRef{}
	for rows.Next() {
		var r domain.SubjectRef
		if err := rows.Scan(&r.Type, &r.ID); err == nil {
			list = append(list, r)
		}
	}
	return list, rows.Err()
}

func (s *Store) getItemLabelIDs(ctx context.Context, targetType, targetID string) ([]string, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT label_id FROM item_labels WHERE target_type = $1 AND target_id = $2 ORDER BY created_at ASC
	`, targetType, targetID)
	if err != nil {
		return []string{}, err
	}
	defer rows.Close()
	ids := []string{}
	for rows.Next() {
		var lid string
		if err := rows.Scan(&lid); err == nil {
			ids = append(ids, lid)
		}
	}
	return ids, rows.Err()
}

func (s *Store) getItemCustomValues(ctx context.Context, targetType, targetID string) (map[string]json.RawMessage, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT field_definition_id, value_json FROM custom_field_values WHERE target_type = $1 AND target_id = $2
	`, targetType, targetID)
	if err != nil {
		return map[string]json.RawMessage{}, err
	}
	defer rows.Close()
	vals := make(map[string]json.RawMessage)
	for rows.Next() {
		var fid string
		var raw json.RawMessage
		if err := rows.Scan(&fid, &raw); err == nil {
			vals[fid] = raw
		}
	}
	return vals, rows.Err()
}

func (s *Store) computeTemplatePermissions(tpl *domain.Template, targetType, userID string, userGroups []string, role domain.ProjectRole, ownerUserID string) map[string]domain.FieldPermissionState {
	perms := make(map[string]domain.FieldPermissionState)
	defaultCanEdit := domain.RolePriority(role) >= domain.RolePriority(domain.RoleEditor)
	for _, stdKey := range domain.RequiredStandardFields(targetType) {
		perms[stdKey] = domain.FieldPermissionState{
			CanView: true,
			CanEdit: defaultCanEdit,
		}
	}
	if targetType == domain.TargetIssue {
		for _, optKey := range []string{domain.StdFieldPremortem, domain.StdFieldPostmortem} {
			perms[optKey] = domain.FieldPermissionState{
				CanView: true,
				CanEdit: defaultCanEdit,
			}
		}
	}
	if targetType == domain.TargetTask {
		for _, optKey := range []string{domain.StdFieldPlannedCost, domain.StdFieldActualCost} {
			perms[optKey] = domain.FieldPermissionState{
				CanView: true,
				CanEdit: defaultCanEdit,
			}
		}
	}
	if tpl == nil {
		return perms
	}
	for _, row := range tpl.Rows {
		for _, f := range row.Fields {
			perms[f.FieldKey] = domain.EvaluateFieldPermission(f, userID, userGroups, role, ownerUserID)
		}
	}
	return perms
}

func (s *Store) applyIssueFieldPermissions(iss *domain.Issue, tpl *domain.Template, allCustomValues map[string]json.RawMessage, userID string, userGroups []string, role domain.ProjectRole) {
	perms := s.computeTemplatePermissions(tpl, domain.TargetIssue, userID, userGroups, role, iss.OwnerUserID)
	iss.FieldPermissions = perms
	iss.CustomValues = make(map[string]json.RawMessage)

	// Non-skeleton standard fields: mask/omit if CanView == false
	if p, ok := perms[domain.StdFieldDescription]; ok && !p.CanView {
		iss.Description = nil
	}
	if p, ok := perms[domain.StdFieldPremortem]; ok && !p.CanView {
		iss.Premortem = ""
	}
	if p, ok := perms[domain.StdFieldPostmortem]; ok && !p.CanView {
		iss.Postmortem = ""
	}
	if p, ok := perms[domain.StdFieldLabels]; ok && !p.CanView {
		iss.LabelIDs = []string{}
	}

	// Only include custom fields present in the current template where CanView == true
	if tpl != nil {
		for _, row := range tpl.Rows {
			for _, f := range row.Fields {
				if f.FieldSource == domain.FieldSourceCustom {
					if p, ok := perms[f.FieldKey]; ok && p.CanView {
						if val, exists := allCustomValues[f.FieldKey]; exists {
							iss.CustomValues[f.FieldKey] = val
						}
					}
				}
			}
		}
	}
}

func (s *Store) applyTaskFieldPermissions(t *domain.Task, tpl *domain.Template, allCustomValues map[string]json.RawMessage, userID string, userGroups []string, role domain.ProjectRole) {
	perms := s.computeTemplatePermissions(tpl, domain.TargetTask, userID, userGroups, role, t.OwnerUserID)
	t.FieldPermissions = perms
	t.CustomValues = make(map[string]json.RawMessage)

	if p, ok := perms[domain.StdFieldDescription]; ok && !p.CanView {
		t.Description = nil
	}
	if p, ok := perms[domain.StdFieldAssignees]; ok && !p.CanView {
		t.Assignees = []domain.SubjectRef{}
	}
	if p, ok := perms[domain.StdFieldLabels]; ok && !p.CanView {
		t.LabelIDs = []string{}
	}
	if p, ok := perms[domain.StdFieldPlannedCost]; ok && !p.CanView {
		t.PlannedCost = nil
	}
	if p, ok := perms[domain.StdFieldActualCost]; ok && !p.CanView {
		t.ActualCost = nil
	}

	if tpl != nil {
		for _, row := range tpl.Rows {
			for _, f := range row.Fields {
				if f.FieldSource == domain.FieldSourceCustom {
					if p, ok := perms[f.FieldKey]; ok && p.CanView {
						if val, exists := allCustomValues[f.FieldKey]; exists {
							t.CustomValues[f.FieldKey] = val
						}
					}
				}
			}
		}
	}
}

func validateDateRange(start, end *string) error {
	if start == nil || end == nil || *start == "" || *end == "" {
		return nil
	}
	s, err1 := time.Parse("2006-01-02", *start)
	e, err2 := time.Parse("2006-01-02", *end)
	if err1 != nil || err2 != nil {
		return fmt.Errorf("%w: date must be in YYYY-MM-DD format", ErrBadRequest)
	}
	if e.Before(s) {
		return fmt.Errorf("%w: start date cannot be after end date", ErrBadRequest)
	}
	return nil
}

func nullDate(d *string) any {
	if d == nil || strings.TrimSpace(*d) == "" {
		return nil
	}
	return strings.TrimSpace(*d)
}

func timePtrStr(nt sql.NullTime) *string {
	if !nt.Valid {
		return nil
	}
	s := nt.Time.Format("2006-01-02")
	return &s
}

func floatPtr(nf sql.NullFloat64) *float64 {
	if !nf.Valid {
		return nil
	}
	v := nf.Float64
	return &v
}

