package store

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	"github.com/takayoshi-u/polaris/backend/internal/domain"
	"github.com/takayoshi-u/polaris/backend/internal/pkg/crypto"
	"github.com/takayoshi-u/polaris/backend/internal/pkg/id"
)

// -------------------------------------------------------------
// 1. Task Dependencies
// -------------------------------------------------------------

func (s *Store) ListTaskDependencies(ctx context.Context, projectID string) ([]domain.TaskDependency, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, project_id, predecessor_id, successor_id, dependency_type, lag_days, created_at
		FROM task_dependencies
		WHERE project_id = $1
		ORDER BY created_at ASC
	`, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var deps []domain.TaskDependency
	for rows.Next() {
		var d domain.TaskDependency
		var dt string
		if err := rows.Scan(&d.ID, &d.ProjectID, &d.PredecessorID, &d.SuccessorID, &dt, &d.LagDays, &d.CreatedAt); err != nil {
			return nil, err
		}
		d.DependencyType = domain.DependencyType(dt)
		deps = append(deps, d)
	}
	return deps, rows.Err()
}

func (s *Store) CreateTaskDependency(ctx context.Context, projectID, actorUserID, predID, succID string, depType domain.DependencyType, lagDays int) (*domain.TaskDependency, error) {
	if predID == succID {
		return nil, fmt.Errorf("%w: self-dependency is not allowed", ErrBadRequest)
	}
	if depType == "" {
		depType = domain.DependencyFS
	}
	if depType != domain.DependencyFS && depType != domain.DependencySS && depType != domain.DependencyFF && depType != domain.DependencySF {
		return nil, fmt.Errorf("%w: invalid dependency type %s", ErrBadRequest, depType)
	}

	proj, err := s.GetProject(ctx, projectID, actorUserID)
	if err != nil {
		return nil, err
	}
	if domain.RolePriority(proj.CurrentUserRole) < domain.RolePriority(domain.RoleEditor) {
		return nil, ErrForbidden
	}

	// Verify predecessor and successor tasks exist in project
	var count int
	err = s.db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM tasks WHERE id IN ($1, $2) AND project_id = $3
	`, predID, succID, projectID).Scan(&count)
	if err != nil || count != 2 {
		return nil, fmt.Errorf("%w: both predecessor and successor tasks must exist in the project", ErrBadRequest)
	}

	// Check circular dependency:
	// Adding edge Pred -> Succ creates a cycle if there is already a path Succ -> ... -> Pred.
	existingDeps, err := s.ListTaskDependencies(ctx, projectID)
	if err != nil {
		return nil, err
	}

	adj := make(map[string][]string)
	for _, d := range existingDeps {
		adj[d.PredecessorID] = append(adj[d.PredecessorID], d.SuccessorID)
	}

	// DFS reachability check: from succID to predID
	visited := make(map[string]bool)
	var hasPath func(curr, target string) bool
	hasPath = func(curr, target string) bool {
		if curr == target {
			return true
		}
		visited[curr] = true
		for _, nxt := range adj[curr] {
			if !visited[nxt] {
				if hasPath(nxt, target) {
					return true
				}
			}
		}
		return false
	}

	if hasPath(succID, predID) {
		return nil, fmt.Errorf("%w: circular dependency detected", ErrBadRequest)
	}

	depID := id.NewV7String()
	now := time.Now().UTC()
	_, err = s.db.ExecContext(ctx, `
		INSERT INTO task_dependencies (id, project_id, predecessor_id, successor_id, dependency_type, lag_days, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`, depID, projectID, predID, succID, string(depType), lagDays, now)
	if err != nil {
		if strings.Contains(err.Error(), "unique") || strings.Contains(err.Error(), "duplicate") {
			return nil, fmt.Errorf("%w: dependency already exists", ErrConflict)
		}
		return nil, err
	}

	return &domain.TaskDependency{
		ID:             depID,
		ProjectID:      projectID,
		PredecessorID:  predID,
		SuccessorID:    succID,
		DependencyType: depType,
		LagDays:        lagDays,
		CreatedAt:      now,
	}, nil
}

func (s *Store) DeleteTaskDependency(ctx context.Context, projectID, depID, actorUserID string) error {
	proj, err := s.GetProject(ctx, projectID, actorUserID)
	if err != nil {
		return err
	}
	if domain.RolePriority(proj.CurrentUserRole) < domain.RolePriority(domain.RoleEditor) {
		return ErrForbidden
	}

	res, err := s.db.ExecContext(ctx, `
		DELETE FROM task_dependencies WHERE id = $1 AND project_id = $2
	`, depID, projectID)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

// -------------------------------------------------------------
// 2. Sprints & Burndown
// -------------------------------------------------------------

type SprintCreateInput struct {
	Name      string
	Goal      *string
	StartDate *string
	EndDate   *string
	Status    string
}

type SprintUpdateInput struct {
	Name      *string
	Goal      *string
	StartDate *string
	EndDate   *string
	Status    *string
}

func (s *Store) ListSprints(ctx context.Context, projectID string) ([]domain.Sprint, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, project_id, name, goal, start_date, end_date, status, created_at, updated_at
		FROM sprints
		WHERE project_id = $1
		ORDER BY created_at ASC
	`, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var sprints []domain.Sprint
	for rows.Next() {
		var sp domain.Sprint
		var goal sql.NullString
		var sDate, eDate sql.NullTime
		if err := rows.Scan(&sp.ID, &sp.ProjectID, &sp.Name, &goal, &sDate, &eDate, &sp.Status, &sp.CreatedAt, &sp.UpdatedAt); err != nil {
			return nil, err
		}
		if goal.Valid {
			sp.Goal = &goal.String
		}
		if sDate.Valid {
			v := sDate.Time.Format("2006-01-02")
			sp.StartDate = &v
		}
		if eDate.Valid {
			v := eDate.Time.Format("2006-01-02")
			sp.EndDate = &v
		}
		sprints = append(sprints, sp)
	}

	// Calculate aggregate task metrics for each sprint
	statRows, err := s.db.QueryContext(ctx, `
		SELECT sprint_id, COUNT(*), COALESCE(SUM(story_points), 0), COALESCE(SUM(planned_hours), 0), COALESCE(SUM(actual_hours), 0)
		FROM tasks
		WHERE project_id = $1 AND sprint_id IS NOT NULL
		GROUP BY sprint_id
	`, projectID)
	if err == nil {
		defer statRows.Close()
		type stats struct {
			count int
			sp    float64
			pHrs  float64
			aHrs  float64
		}
		sm := make(map[string]stats)
		for statRows.Next() {
			var sid string
			var st stats
			if err := statRows.Scan(&sid, &st.count, &st.sp, &st.pHrs, &st.aHrs); err == nil {
				sm[sid] = st
			}
		}
		for i := range sprints {
			if st, ok := sm[sprints[i].ID]; ok {
				sprints[i].TaskCount = st.count
				sprints[i].TotalStoryPoints = st.sp
				sprints[i].TotalPlannedHours = st.pHrs
				sprints[i].TotalActualHours = st.aHrs
			}
		}
	}

	return sprints, nil
}

func (s *Store) CreateSprint(ctx context.Context, projectID, actorUserID string, in SprintCreateInput) (*domain.Sprint, error) {
	name := strings.TrimSpace(in.Name)
	if name == "" {
		return nil, fmt.Errorf("%w: sprint name is required", ErrBadRequest)
	}
	status := in.Status
	if status == "" {
		status = domain.SprintStatusPlanned
	}
	if status != domain.SprintStatusPlanned && status != domain.SprintStatusActive && status != domain.SprintStatusClosed {
		return nil, fmt.Errorf("%w: invalid sprint status %s", ErrBadRequest, status)
	}
	if err := validateDateRange(in.StartDate, in.EndDate); err != nil {
		return nil, err
	}

	proj, err := s.GetProject(ctx, projectID, actorUserID)
	if err != nil {
		return nil, err
	}
	if domain.RolePriority(proj.CurrentUserRole) < domain.RolePriority(domain.RoleEditor) {
		return nil, ErrForbidden
	}

	sprintID := id.NewV7String()
	now := time.Now().UTC()
	_, err = s.db.ExecContext(ctx, `
		INSERT INTO sprints (id, project_id, name, goal, start_date, end_date, status, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $8)
	`, sprintID, projectID, name, in.Goal, nullDate(in.StartDate), nullDate(in.EndDate), status, now)
	if err != nil {
		return nil, err
	}

	_ = s.LogActivity(ctx, projectID, "sprint", sprintID, "created", map[string]interface{}{"name": name, "status": status}, &actorUserID)

	return &domain.Sprint{
		ID:        sprintID,
		ProjectID: projectID,
		Name:      name,
		Goal:      in.Goal,
		StartDate: in.StartDate,
		EndDate:   in.EndDate,
		Status:    status,
		CreatedAt: now,
		UpdatedAt: now,
	}, nil
}

func (s *Store) GetSprint(ctx context.Context, sprintID, actorUserID string) (*domain.Sprint, error) {
	var sp domain.Sprint
	var goal sql.NullString
	var sDate, eDate sql.NullTime

	err := s.db.QueryRowContext(ctx, `
		SELECT id, project_id, name, goal, start_date, end_date, status, created_at, updated_at
		FROM sprints WHERE id = $1
	`, sprintID).Scan(&sp.ID, &sp.ProjectID, &sp.Name, &goal, &sDate, &eDate, &sp.Status, &sp.CreatedAt, &sp.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	if goal.Valid {
		sp.Goal = &goal.String
	}
	if sDate.Valid {
		v := sDate.Time.Format("2006-01-02")
		sp.StartDate = &v
	}
	if eDate.Valid {
		v := eDate.Time.Format("2006-01-02")
		sp.EndDate = &v
	}

	proj, err := s.GetProject(ctx, sp.ProjectID, actorUserID)
	if err != nil {
		return nil, err
	}
	if !proj.IsMember {
		return nil, ErrForbidden
	}

	_ = s.db.QueryRowContext(ctx, `
		SELECT COUNT(*), COALESCE(SUM(story_points), 0), COALESCE(SUM(planned_hours), 0), COALESCE(SUM(actual_hours), 0)
		FROM tasks WHERE sprint_id = $1
	`, sprintID).Scan(&sp.TaskCount, &sp.TotalStoryPoints, &sp.TotalPlannedHours, &sp.TotalActualHours)

	return &sp, nil
}

func (s *Store) UpdateSprint(ctx context.Context, sprintID, actorUserID string, in SprintUpdateInput) (*domain.Sprint, error) {
	sp, err := s.GetSprint(ctx, sprintID, actorUserID)
	if err != nil {
		return nil, err
	}

	proj, err := s.GetProject(ctx, sp.ProjectID, actorUserID)
	if err != nil {
		return nil, err
	}
	if domain.RolePriority(proj.CurrentUserRole) < domain.RolePriority(domain.RoleEditor) {
		return nil, ErrForbidden
	}

	name := sp.Name
	if in.Name != nil && strings.TrimSpace(*in.Name) != "" {
		name = strings.TrimSpace(*in.Name)
	}
	goal := sp.Goal
	if in.Goal != nil {
		goal = in.Goal
	}
	sDate := sp.StartDate
	if in.StartDate != nil {
		sDate = in.StartDate
	}
	eDate := sp.EndDate
	if in.EndDate != nil {
		eDate = in.EndDate
	}
	if err := validateDateRange(sDate, eDate); err != nil {
		return nil, err
	}
	status := sp.Status
	if in.Status != nil {
		st := *in.Status
		if st != domain.SprintStatusPlanned && st != domain.SprintStatusActive && st != domain.SprintStatusClosed {
			return nil, fmt.Errorf("%w: invalid sprint status %s", ErrBadRequest, st)
		}
		status = st
	}

	now := time.Now().UTC()
	_, err = s.db.ExecContext(ctx, `
		UPDATE sprints
		SET name = $1, goal = $2, start_date = $3, end_date = $4, status = $5, updated_at = $6
		WHERE id = $7
	`, name, goal, nullDate(sDate), nullDate(eDate), status, now, sprintID)
	if err != nil {
		return nil, err
	}

	_ = s.LogActivity(ctx, sp.ProjectID, "sprint", sprintID, "updated", map[string]interface{}{"name": name, "status": status}, &actorUserID)

	return s.GetSprint(ctx, sprintID, actorUserID)
}

func (s *Store) DeleteSprint(ctx context.Context, sprintID, actorUserID string) error {
	sp, err := s.GetSprint(ctx, sprintID, actorUserID)
	if err != nil {
		return err
	}

	proj, err := s.GetProject(ctx, sp.ProjectID, actorUserID)
	if err != nil {
		return err
	}
	if domain.RolePriority(proj.CurrentUserRole) < domain.RolePriority(domain.RoleEditor) {
		return ErrForbidden
	}

	_, err = s.db.ExecContext(ctx, `DELETE FROM sprints WHERE id = $1`, sprintID)
	return err
}

func (s *Store) GetSprintBurndown(ctx context.Context, sprintID, actorUserID string) (*domain.SprintBurndown, error) {
	sp, err := s.GetSprint(ctx, sprintID, actorUserID)
	if err != nil {
		return nil, err
	}

	startStr := ""
	if sp.StartDate != nil {
		startStr = *sp.StartDate
	} else {
		startStr = sp.CreatedAt.Format("2006-01-02")
	}

	endStr := ""
	if sp.EndDate != nil {
		endStr = *sp.EndDate
	} else {
		t, _ := time.Parse("2006-01-02", startStr)
		endStr = t.AddDate(0, 0, 14).Format("2006-01-02")
	}

	startDate, _ := time.Parse("2006-01-02", startStr)
	endDate, _ := time.Parse("2006-01-02", endStr)
	if endDate.Before(startDate) {
		endDate = startDate.AddDate(0, 0, 14)
	}

	totalDays := int(endDate.Sub(startDate).Hours()/24) + 1
	if totalDays <= 0 {
		totalDays = 1
	}

	// Fetch tasks in sprint
	type taskInfo struct {
		sp       float64
		pHours   float64
		progress int
		endVal   *string
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT COALESCE(story_points, 0), COALESCE(planned_hours, 0), progress, actual_end_date
		FROM tasks WHERE sprint_id = $1
	`, sprintID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tasks []taskInfo
	var totalSP, totalHours float64
	for rows.Next() {
		var ti taskInfo
		var aEnd sql.NullTime
		if err := rows.Scan(&ti.sp, &ti.pHours, &ti.progress, &aEnd); err == nil {
			if aEnd.Valid {
				v := aEnd.Time.Format("2006-01-02")
				ti.endVal = &v
			}
			totalSP += ti.sp
			totalHours += ti.pHours
			tasks = append(tasks, ti)
		}
	}

	var points []domain.BurndownDataPoint
	for i := 0; i < totalDays; i++ {
		currentDate := startDate.AddDate(0, 0, i)
		dateStr := currentDate.Format("2006-01-02")

		fraction := float64(i) / float64(totalDays-1)
		if totalDays == 1 {
			fraction = 1.0
		}
		idealSP := totalSP * (1.0 - fraction)
		idealHours := totalHours * (1.0 - fraction)

		// Calculate completed SP & hours on or before this day
		var completedSP, completedHours float64
		for _, t := range tasks {
			if t.progress == 100 {
				if t.endVal != nil && *t.endVal <= dateStr {
					completedSP += t.sp
					completedHours += t.pHours
				} else if t.endVal == nil && dateStr >= time.Now().Format("2006-01-02") {
					completedSP += t.sp
					completedHours += t.pHours
				}
			}
		}

		remSP := totalSP - completedSP
		if remSP < 0 {
			remSP = 0
		}
		remHours := totalHours - completedHours
		if remHours < 0 {
			remHours = 0
		}

		points = append(points, domain.BurndownDataPoint{
			Date:                 dateStr,
			IdealStoryPoints:     idealSP,
			RemainingStoryPoints: remSP,
			IdealHours:           idealHours,
			RemainingHours:       remHours,
		})
	}

	return &domain.SprintBurndown{
		SprintID:   sp.ID,
		SprintName: sp.Name,
		StartDate:  startStr,
		EndDate:    endStr,
		TotalSP:    totalSP,
		TotalHours: totalHours,
		Points:     points,
	}, nil
}

// -------------------------------------------------------------
// 3. Activity Logs
// -------------------------------------------------------------

func (s *Store) LogActivity(ctx context.Context, projectID, entityType, entityID, action string, details map[string]interface{}, actorID *string) error {
	if details == nil {
		details = make(map[string]interface{})
	}
	raw, err := json.Marshal(details)
	if err != nil {
		raw = []byte("{}")
	}

	logID := id.NewV7String()
	now := time.Now().UTC()

	_, err = s.db.ExecContext(ctx, `
		INSERT INTO activity_logs (id, project_id, entity_type, entity_id, action, details, actor_id, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`, logID, projectID, entityType, entityID, action, raw, actorID, now)
	return err
}

func (s *Store) ListActivities(ctx context.Context, projectID string, entityType, entityID *string, limit int) ([]domain.ActivityLog, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}

	query := `
		SELECT a.id, a.project_id, a.entity_type, a.entity_id, a.action, a.details, a.actor_id,
		       COALESCE(u.display_name, ''), a.created_at
		FROM activity_logs a
		LEFT JOIN users u ON a.actor_id = u.id
		WHERE a.project_id = $1
	`
	var args []interface{}
	args = append(args, projectID)

	if entityType != nil && *entityType != "" {
		args = append(args, *entityType)
		query += fmt.Sprintf(` AND a.entity_type = $%d`, len(args))
	}
	if entityID != nil && *entityID != "" {
		args = append(args, *entityID)
		query += fmt.Sprintf(` AND a.entity_id = $%d`, len(args))
	}

	args = append(args, limit)
	query += fmt.Sprintf(` ORDER BY a.created_at DESC LIMIT $%d`, len(args))

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var logs []domain.ActivityLog
	for rows.Next() {
		var l domain.ActivityLog
		var rawDetails []byte
		var actorID sql.NullString
		var actorName string
		if err := rows.Scan(&l.ID, &l.ProjectID, &l.EntityType, &l.EntityID, &l.Action, &rawDetails, &actorID, &actorName, &l.CreatedAt); err != nil {
			return nil, err
		}
		l.Details = json.RawMessage(rawDetails)
		if actorID.Valid {
			l.ActorID = &actorID.String
		}
		if actorName != "" {
			l.ActorName = &actorName
		}
		logs = append(logs, l)
	}
	return logs, rows.Err()
}

// -------------------------------------------------------------
// 4. Project Wiki
// -------------------------------------------------------------

type WikiPageCreateInput struct {
	Title     string
	Slug      string
	Content   string
	ParentID  *string
	SortOrder int
}

type WikiPageUpdateInput struct {
	Title     *string
	Slug      *string
	Content   *string
	ParentID  *string
	ClearParent bool
	SortOrder *int
}

func (s *Store) ListWikiPages(ctx context.Context, projectID string) ([]domain.WikiPage, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, project_id, title, slug, content, parent_id, sort_order, created_by, updated_by, created_at, updated_at
		FROM wiki_pages
		WHERE project_id = $1
		ORDER BY sort_order ASC, created_at ASC
	`, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var pages []domain.WikiPage
	for rows.Next() {
		var p domain.WikiPage
		var parentID, cBy, uBy sql.NullString
		if err := rows.Scan(&p.ID, &p.ProjectID, &p.Title, &p.Slug, &p.Content, &parentID, &p.SortOrder, &cBy, &uBy, &p.CreatedAt, &p.UpdatedAt); err != nil {
			return nil, err
		}
		if parentID.Valid {
			p.ParentID = &parentID.String
		}
		if cBy.Valid {
			p.CreatedBy = &cBy.String
		}
		if uBy.Valid {
			p.UpdatedBy = &uBy.String
		}
		pages = append(pages, p)
	}

	return pages, rows.Err()
}

func (s *Store) CreateWikiPage(ctx context.Context, projectID, actorUserID string, in WikiPageCreateInput) (*domain.WikiPage, error) {
	title := strings.TrimSpace(in.Title)
	if title == "" {
		return nil, fmt.Errorf("%w: wiki title is required", ErrBadRequest)
	}
	slug := strings.TrimSpace(strings.ToLower(in.Slug))
	if slug == "" {
		slug = strings.ReplaceAll(title, " ", "-")
	}

	proj, err := s.GetProject(ctx, projectID, actorUserID)
	if err != nil {
		return nil, err
	}
	if domain.RolePriority(proj.CurrentUserRole) < domain.RolePriority(domain.RoleEditor) {
		return nil, ErrForbidden
	}

	pageID := id.NewV7String()
	now := time.Now().UTC()

	_, err = s.db.ExecContext(ctx, `
		INSERT INTO wiki_pages (id, project_id, title, slug, content, parent_id, sort_order, created_by, updated_by, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $8, $9, $9)
	`, pageID, projectID, title, slug, in.Content, in.ParentID, in.SortOrder, actorUserID, now)
	if err != nil {
		if strings.Contains(err.Error(), "unique") || strings.Contains(err.Error(), "duplicate") {
			return nil, fmt.Errorf("%w: wiki page slug '%s' already exists in project", ErrConflict, slug)
		}
		return nil, err
	}

	_ = s.LogActivity(ctx, projectID, "wiki", pageID, "created", map[string]interface{}{"title": title, "slug": slug}, &actorUserID)

	return &domain.WikiPage{
		ID:        pageID,
		ProjectID: projectID,
		Title:     title,
		Slug:      slug,
		Content:   in.Content,
		ParentID:  in.ParentID,
		SortOrder: in.SortOrder,
		CreatedBy: &actorUserID,
		UpdatedBy: &actorUserID,
		CreatedAt: now,
		UpdatedAt: now,
	}, nil
}

func (s *Store) GetWikiPage(ctx context.Context, wikiID, actorUserID string) (*domain.WikiPage, error) {
	var p domain.WikiPage
	var parentID, cBy, uBy sql.NullString

	err := s.db.QueryRowContext(ctx, `
		SELECT id, project_id, title, slug, content, parent_id, sort_order, created_by, updated_by, created_at, updated_at
		FROM wiki_pages WHERE id = $1
	`, wikiID).Scan(&p.ID, &p.ProjectID, &p.Title, &p.Slug, &p.Content, &parentID, &p.SortOrder, &cBy, &uBy, &p.CreatedAt, &p.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	if parentID.Valid {
		p.ParentID = &parentID.String
	}
	if cBy.Valid {
		p.CreatedBy = &cBy.String
	}
	if uBy.Valid {
		p.UpdatedBy = &uBy.String
	}

	proj, err := s.GetProject(ctx, p.ProjectID, actorUserID)
	if err != nil {
		return nil, err
	}
	if !proj.IsMember {
		return nil, ErrForbidden
	}

	return &p, nil
}

func (s *Store) UpdateWikiPage(ctx context.Context, wikiID, actorUserID string, in WikiPageUpdateInput) (*domain.WikiPage, error) {
	page, err := s.GetWikiPage(ctx, wikiID, actorUserID)
	if err != nil {
		return nil, err
	}

	proj, err := s.GetProject(ctx, page.ProjectID, actorUserID)
	if err != nil {
		return nil, err
	}
	if domain.RolePriority(proj.CurrentUserRole) < domain.RolePriority(domain.RoleEditor) {
		return nil, ErrForbidden
	}

	title := page.Title
	if in.Title != nil && strings.TrimSpace(*in.Title) != "" {
		title = strings.TrimSpace(*in.Title)
	}
	slug := page.Slug
	if in.Slug != nil && strings.TrimSpace(*in.Slug) != "" {
		slug = strings.TrimSpace(strings.ToLower(*in.Slug))
	}
	content := page.Content
	if in.Content != nil {
		content = *in.Content
	}
	parentID := page.ParentID
	if in.ClearParent {
		parentID = nil
	} else if in.ParentID != nil {
		if *in.ParentID == wikiID {
			return nil, fmt.Errorf("%w: page cannot be its own parent", ErrBadRequest)
		}
		parentID = in.ParentID
	}
	sortOrder := page.SortOrder
	if in.SortOrder != nil {
		sortOrder = *in.SortOrder
	}

	now := time.Now().UTC()
	_, err = s.db.ExecContext(ctx, `
		UPDATE wiki_pages
		SET title = $1, slug = $2, content = $3, parent_id = $4, sort_order = $5, updated_by = $6, updated_at = $7
		WHERE id = $8
	`, title, slug, content, parentID, sortOrder, actorUserID, now, wikiID)
	if err != nil {
		if strings.Contains(err.Error(), "unique") || strings.Contains(err.Error(), "duplicate") {
			return nil, fmt.Errorf("%w: wiki page slug '%s' already exists in project", ErrConflict, slug)
		}
		return nil, err
	}

	_ = s.LogActivity(ctx, page.ProjectID, "wiki", wikiID, "updated", map[string]interface{}{"title": title, "slug": slug}, &actorUserID)

	return s.GetWikiPage(ctx, wikiID, actorUserID)
}

func (s *Store) DeleteWikiPage(ctx context.Context, wikiID, actorUserID string) error {
	page, err := s.GetWikiPage(ctx, wikiID, actorUserID)
	if err != nil {
		return err
	}

	proj, err := s.GetProject(ctx, page.ProjectID, actorUserID)
	if err != nil {
		return err
	}
	if domain.RolePriority(proj.CurrentUserRole) < domain.RolePriority(domain.RoleEditor) {
		return ErrForbidden
	}

	_, err = s.db.ExecContext(ctx, `DELETE FROM wiki_pages WHERE id = $1`, wikiID)
	return err
}

// -------------------------------------------------------------
// 5. Inbound Webhooks
// -------------------------------------------------------------

func (s *Store) ListInboundWebhooks(ctx context.Context, projectID string) ([]domain.InboundWebhook, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, project_id, name, secret, default_issue_id, default_status_id, is_active, created_at, updated_at
		FROM inbound_webhooks
		WHERE project_id = $1
		ORDER BY created_at ASC
	`, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []domain.InboundWebhook
	for rows.Next() {
		var w domain.InboundWebhook
		var issueID sql.NullString
		if err := rows.Scan(&w.ID, &w.ProjectID, &w.Name, &w.Secret, &issueID, &w.DefaultStatusID, &w.IsActive, &w.CreatedAt, &w.UpdatedAt); err != nil {
			return nil, err
		}
		if issueID.Valid {
			w.DefaultIssueID = &issueID.String
		}
		list = append(list, w)
	}
	return list, rows.Err()
}

func (s *Store) CreateInboundWebhook(ctx context.Context, projectID, actorUserID, name string, defaultIssueID *string, defaultStatusID int) (*domain.InboundWebhook, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, fmt.Errorf("%w: webhook name is required", ErrBadRequest)
	}
	if defaultStatusID <= 0 {
		defaultStatusID = 1
	}

	proj, err := s.GetProject(ctx, projectID, actorUserID)
	if err != nil {
		return nil, err
	}
	if domain.RolePriority(proj.CurrentUserRole) < domain.RolePriority(domain.RoleAdmin) {
		return nil, ErrForbidden
	}

	rawToken, _, err := crypto.GenerateRandomToken(24)
	if err != nil {
		return nil, err
	}
	plainToken := "polaris_inb_" + rawToken
	tokenHash := crypto.HashToken(plainToken)
	rawSecret, _, _ := crypto.GenerateRandomToken(16)
	secret := "whsec_" + rawSecret

	webhookID := id.NewV7String()
	now := time.Now().UTC()

	_, err = s.db.ExecContext(ctx, `
		INSERT INTO inbound_webhooks (id, project_id, name, token_hash, secret, default_issue_id, default_status_id, is_active, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, TRUE, $8, $8)
	`, webhookID, projectID, name, tokenHash, secret, defaultIssueID, defaultStatusID, now)
	if err != nil {
		return nil, err
	}

	return &domain.InboundWebhook{
		ID:              webhookID,
		ProjectID:       projectID,
		Name:            name,
		TokenPlain:      plainToken,
		Secret:          secret,
		DefaultIssueID:  defaultIssueID,
		DefaultStatusID: defaultStatusID,
		IsActive:        true,
		CreatedAt:       now,
		UpdatedAt:       now,
	}, nil
}

func (s *Store) DeleteInboundWebhook(ctx context.Context, webhookID, actorUserID string) error {
	var projectID string
	err := s.db.QueryRowContext(ctx, `SELECT project_id FROM inbound_webhooks WHERE id = $1`, webhookID).Scan(&projectID)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return err
	}

	proj, err := s.GetProject(ctx, projectID, actorUserID)
	if err != nil {
		return err
	}
	if domain.RolePriority(proj.CurrentUserRole) < domain.RolePriority(domain.RoleAdmin) {
		return ErrForbidden
	}

	_, err = s.db.ExecContext(ctx, `DELETE FROM inbound_webhooks WHERE id = $1`, webhookID)
	return err
}

func (s *Store) ProcessInboundWebhook(ctx context.Context, token string, payload domain.InboundWebhookPayload) (*domain.Task, error) {
	tokenHash := crypto.HashToken(token)

	var wh domain.InboundWebhook
	var defIssueID sql.NullString
	err := s.db.QueryRowContext(ctx, `
		SELECT id, project_id, name, secret, default_issue_id, default_status_id, is_active
		FROM inbound_webhooks WHERE token_hash = $1
	`, tokenHash).Scan(&wh.ID, &wh.ProjectID, &wh.Name, &wh.Secret, &defIssueID, &wh.DefaultStatusID, &wh.IsActive)
	if errors.Is(err, sql.ErrNoRows) || !wh.IsActive {
		return nil, ErrUnauthorized
	}
	if err != nil {
		return nil, err
	}
	if defIssueID.Valid {
		wh.DefaultIssueID = &defIssueID.String
	}

	// Determine parent issue ID
	targetIssueID := ""
	if payload.IssueID != nil && *payload.IssueID != "" {
		var count int
		_ = s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM issues WHERE id = $1 AND project_id = $2`, *payload.IssueID, wh.ProjectID).Scan(&count)
		if count > 0 {
			targetIssueID = *payload.IssueID
		}
	}
	if targetIssueID == "" && wh.DefaultIssueID != nil {
		targetIssueID = *wh.DefaultIssueID
	}
	if targetIssueID == "" {
		// Use project's first issue or create one
		_ = s.db.QueryRowContext(ctx, `SELECT id FROM issues WHERE project_id = $1 ORDER BY display_order ASC LIMIT 1`, wh.ProjectID).Scan(&targetIssueID)
	}
	if targetIssueID == "" {
		// Create a fallback issue
		adminUser, _ := s.GetFirstAdminUser(ctx)
		creatorID := ""
		if adminUser != nil {
			creatorID = adminUser.ID
		}
		newIss, err := s.CreateIssue(ctx, wh.ProjectID, creatorID, IssueCreateInput{
			Title:       "Inbound Webhooks",
			Description: "Automated issue created by inbound webhooks",
		})
		if err != nil {
			return nil, err
		}
		targetIssueID = newIss.ID
	}

	// Status ID
	targetStatusID := ""
	if payload.StatusID != nil && *payload.StatusID != "" {
		targetStatusID = *payload.StatusID
	} else {
		_ = s.db.QueryRowContext(ctx, `SELECT id FROM statuses WHERE project_id = $1 ORDER BY display_order ASC LIMIT 1`, wh.ProjectID).Scan(&targetStatusID)
	}

	title := strings.TrimSpace(payload.Title)
	if title == "" {
		title = "Inbound Webhook Task " + time.Now().Format("2006-01-02 15:04:05")
	}

	var assignees []domain.SubjectRef
	for _, aid := range payload.AssigneeIDs {
		assignees = append(assignees, domain.SubjectRef{Type: domain.SubjectUser, ID: aid})
	}

	desc := ""
	if payload.Description != nil {
		desc = *payload.Description
	}

	// Create task
	taskID := id.NewV7String()
	now := time.Now().UTC()
	var maxKanban, maxGantt int
	_ = s.db.QueryRowContext(ctx, `SELECT COALESCE(MAX(kanban_order), 0), COALESCE(MAX(gantt_order), 0) FROM tasks WHERE issue_id = $1`, targetIssueID).Scan(&maxKanban, &maxGantt)

	_, err = s.db.ExecContext(ctx, `
		INSERT INTO tasks (
			id, project_id, issue_id, template_id, owner_user_id, status_id,
			title, description, progress, planned_hours, story_points,
			kanban_order, gantt_order, created_at, updated_at
		) VALUES ($1, $2, $3, NULL, NULL, $4, $5, $6, 0, $7, $8, $9, $10, $11, $11)
	`, taskID, wh.ProjectID, targetIssueID, targetStatusID, title, desc, payload.PlannedHours, payload.StoryPoints, maxKanban+1, maxGantt+1, now)
	if err != nil {
		return nil, err
	}

	for _, a := range assignees {
		_, _ = s.db.ExecContext(ctx, `
			INSERT INTO task_assignees (id, task_id, assignee_type, assignee_id, created_at)
			VALUES ($1, $2, $3, $4, $5)
			ON CONFLICT DO NOTHING
		`, id.NewV7String(), taskID, a.Type, a.ID, now)
	}
	for _, lid := range payload.LabelIDs {
		_, _ = s.db.ExecContext(ctx, `
			INSERT INTO item_labels (id, target_type, target_id, label_id, created_at)
			VALUES ($1, 'task', $2, $3, $4)
			ON CONFLICT DO NOTHING
		`, id.NewV7String(), taskID, lid, now)
	}

	_ = s.LogActivity(ctx, wh.ProjectID, "task", taskID, "created", map[string]interface{}{"title": title, "source": "inbound_webhook", "webhook_name": wh.Name}, nil)

	return &domain.Task{
		ID:           taskID,
		ProjectID:    wh.ProjectID,
		IssueID:      targetIssueID,
		StatusID:     targetStatusID,
		Title:        title,
		Description:  &desc,
		Progress:     0,
		PlannedHours: payload.PlannedHours,
		StoryPoints:  payload.StoryPoints,
		CreatedAt:    now,
		UpdatedAt:    now,
	}, nil
}

func (s *Store) GetFirstAdminUser(ctx context.Context) (*domain.User, error) {
	var u domain.User
	err := s.db.QueryRowContext(ctx, `
		SELECT id, email, display_name FROM users WHERE status = 'active' ORDER BY created_at ASC LIMIT 1
	`).Scan(&u.ID, &u.Email, &u.DisplayName)
	if err != nil {
		return nil, err
	}
	return &u, nil
}

// -------------------------------------------------------------
// 6. CSV Export & Import
// -------------------------------------------------------------

type CSVImportResult struct {
	ImportedTasks int      `json:"imported_tasks"`
	Errors        []string `json:"errors"`
}

func (s *Store) ExportProjectCSV(ctx context.Context, projectID, actorUserID string) ([]byte, error) {
	proj, err := s.GetProject(ctx, projectID, actorUserID)
	if err != nil {
		return nil, err
	}
	if !proj.IsMember {
		return nil, ErrForbidden
	}

	// Status map
	statuses, err := s.ListStatuses(ctx, &projectID)
	if err != nil {
		return nil, err
	}
	statusMap := make(map[string]string)
	for _, st := range statuses {
		statusMap[st.ID] = st.Name
	}

	// Issues with tasks
	issues, err := s.ListIssuesWithTasks(ctx, projectID, actorUserID, IssueTaskFilter{IssueState: "all"})
	if err != nil {
		return nil, err
	}

	var buf bytes.Buffer
	w := csv.NewWriter(&buf)

	// Write header
	header := []string{
		"Issue ID", "Issue Title", "Task ID", "Task Title", "Status",
		"Progress (%)", "Planned Start", "Planned End", "Actual Start", "Actual End",
		"Planned Hours", "Actual Hours", "Planned Cost", "Actual Cost",
		"Story Points", "Sprint ID",
	}
	if err := w.Write(header); err != nil {
		return nil, err
	}

	for _, iss := range issues {
		if len(iss.Tasks) == 0 {
			_ = w.Write([]string{
				iss.ID, iss.Title, "", "", "",
				"", "", "", "", "",
				"", "", "", "",
				"", "",
			})
			continue
		}
		for _, t := range iss.Tasks {
			stName := statusMap[t.StatusID]
			if stName == "" {
				stName = t.StatusID
			}

			pStart, pEnd, aStart, aEnd := "", "", "", ""
			if t.PlannedStartDate != nil {
				pStart = *t.PlannedStartDate
			}
			if t.PlannedEndDate != nil {
				pEnd = *t.PlannedEndDate
			}
			if t.ActualStartDate != nil {
				aStart = *t.ActualStartDate
			}
			if t.ActualEndDate != nil {
				aEnd = *t.ActualEndDate
			}

			pHours, aHours, pCost, aCost, sp, sprintID := "", "", "", "", "", ""
			if t.PlannedHours != nil {
				pHours = fmt.Sprintf("%.2f", *t.PlannedHours)
			}
			if t.ActualHours != nil {
				aHours = fmt.Sprintf("%.2f", *t.ActualHours)
			}
			if t.PlannedCost != nil {
				pCost = fmt.Sprintf("%.2f", *t.PlannedCost)
			}
			if t.ActualCost != nil {
				aCost = fmt.Sprintf("%.2f", *t.ActualCost)
			}
			if t.StoryPoints != nil {
				sp = fmt.Sprintf("%.2f", *t.StoryPoints)
			}
			if t.SprintID != nil {
				sprintID = *t.SprintID
			}

			_ = w.Write([]string{
				iss.ID, iss.Title, t.ID, t.Title, stName,
				strconv.Itoa(t.Progress), pStart, pEnd, aStart, aEnd,
				pHours, aHours, pCost, aCost,
				sp, sprintID,
			})
		}
	}

	w.Flush()
	return buf.Bytes(), w.Error()
}

func (s *Store) ImportProjectCSV(ctx context.Context, projectID, actorUserID string, r io.Reader) (*CSVImportResult, error) {
	proj, err := s.GetProject(ctx, projectID, actorUserID)
	if err != nil {
		return nil, err
	}
	if domain.RolePriority(proj.CurrentUserRole) < domain.RolePriority(domain.RoleEditor) {
		return nil, ErrForbidden
	}

	statuses, err := s.ListStatuses(ctx, &projectID)
	if err != nil {
		return nil, err
	}
	statusByName := make(map[string]string)
	for _, st := range statuses {
		statusByName[strings.ToLower(strings.TrimSpace(st.Name))] = st.ID
	}
	defaultStatusID, _ := s.getLowestOrderStatusID(ctx, projectID)

	reader := csv.NewReader(r)
	records, err := reader.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("%w: failed to parse CSV: %v", ErrBadRequest, err)
	}
	if len(records) < 2 {
		return &CSVImportResult{ImportedTasks: 0}, nil
	}

	// Map headers
	headers := records[0]
	idxMap := make(map[string]int)
	for i, h := range headers {
		norm := strings.ToLower(strings.TrimSpace(h))
		idxMap[norm] = i
	}

	var imported int
	var errs []string

	// Track issues in project
	issueCache := make(map[string]string) // title -> id

	for rowIdx, row := range records[1:] {
		getVal := func(keys ...string) string {
			for _, k := range keys {
				if i, ok := idxMap[k]; ok && i < len(row) {
					return strings.TrimSpace(row[i])
				}
			}
			return ""
		}

		issueTitle := getVal("issue title", "issue_title", "課題名", "課題タイトル")
		taskTitle := getVal("task title", "task_title", "タスク名", "タスクタイトル", "title")
		if taskTitle == "" {
			continue // skip empty task row
		}

		// Find or create parent issue
		parentIssueID := getVal("issue id", "issue_id")
		if parentIssueID == "" && issueTitle != "" {
			if cachedID, ok := issueCache[issueTitle]; ok {
				parentIssueID = cachedID
			} else {
				// Query or create issue
				var existingID string
				err := s.db.QueryRowContext(ctx, `SELECT id FROM issues WHERE project_id = $1 AND title = $2 LIMIT 1`, projectID, issueTitle).Scan(&existingID)
				if err == nil {
					parentIssueID = existingID
				} else {
					newIss, err := s.CreateIssue(ctx, projectID, actorUserID, IssueCreateInput{
						Title: issueTitle,
					})
					if err == nil {
						parentIssueID = newIss.ID
					}
				}
				if parentIssueID != "" {
					issueCache[issueTitle] = parentIssueID
				}
			}
		}

		if parentIssueID == "" {
			// Find first issue in project or create Default Issue
			_ = s.db.QueryRowContext(ctx, `SELECT id FROM issues WHERE project_id = $1 ORDER BY display_order ASC LIMIT 1`, projectID).Scan(&parentIssueID)
			if parentIssueID == "" {
				newIss, err := s.CreateIssue(ctx, projectID, actorUserID, IssueCreateInput{Title: "Imported Tasks"})
				if err == nil {
					parentIssueID = newIss.ID
				}
			}
		}

		if parentIssueID == "" {
			errs = append(errs, fmt.Sprintf("Row %d: unable to determine parent issue for task '%s'", rowIdx+2, taskTitle))
			continue
		}

		// Status
		stVal := strings.ToLower(getVal("status", "ステータス"))
		targetStatusID := defaultStatusID
		if sid, ok := statusByName[stVal]; ok {
			targetStatusID = sid
		}

		progress := 0
		if progStr := getVal("progress (%)", "progress", "進捗率"); progStr != "" {
			if p, err := strconv.Atoi(progStr); err == nil && p >= 0 && p <= 100 {
				progress = p
			}
		}

		pStart := getVal("planned start", "planned_start_date")
		pEnd := getVal("planned end", "planned_end_date")
		aStart := getVal("actual start", "actual_start_date")
		aEnd := getVal("actual end", "actual_end_date")

		var pHours, aHours, pCost, aCost, sp *float64
		if val := getVal("planned hours", "planned_hours"); val != "" {
			if f, err := strconv.ParseFloat(val, 64); err == nil {
				pHours = &f
			}
		}
		if val := getVal("actual hours", "actual_hours"); val != "" {
			if f, err := strconv.ParseFloat(val, 64); err == nil {
				aHours = &f
			}
		}
		if val := getVal("planned cost", "planned_cost"); val != "" {
			if f, err := strconv.ParseFloat(val, 64); err == nil {
				pCost = &f
			}
		}
		if val := getVal("actual cost", "actual_cost"); val != "" {
			if f, err := strconv.ParseFloat(val, 64); err == nil {
				aCost = &f
			}
		}
		if val := getVal("story points", "story_points"); val != "" {
			if f, err := strconv.ParseFloat(val, 64); err == nil {
				sp = &f
			}
		}

		var pStartPtr, pEndPtr, aStartPtr, aEndPtr *string
		if pStart != "" {
			pStartPtr = &pStart
		}
		if pEnd != "" {
			pEndPtr = &pEnd
		}
		if aStart != "" {
			aStartPtr = &aStart
		}
		if aEnd != "" {
			aEndPtr = &aEnd
		}

		_, err = s.CreateTask(ctx, projectID, actorUserID, TaskCreateInput{
			IssueID:          parentIssueID,
			StatusID:         &targetStatusID,
			Title:            taskTitle,
			Progress:         progress,
			PlannedStartDate: pStartPtr,
			PlannedEndDate:   pEndPtr,
			ActualStartDate:  aStartPtr,
			ActualEndDate:    aEndPtr,
			PlannedHours:     pHours,
			ActualHours:      aHours,
			PlannedCost:      pCost,
			ActualCost:       aCost,
			StoryPoints:      sp,
		})
		if err != nil {
			errs = append(errs, fmt.Sprintf("Row %d: %v", rowIdx+2, err))
		} else {
			imported++
		}
	}

	return &CSVImportResult{
		ImportedTasks: imported,
		Errors:        errs,
	}, nil
}
