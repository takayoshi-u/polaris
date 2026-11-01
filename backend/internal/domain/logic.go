package domain

import (
	"errors"
	"fmt"
	"math"
	"strings"
	"time"
)

// ValidateAllowedDomainsList validates the list of allowed email domains (max 10).
func ValidateAllowedDomainsList(domains []string) ([]string, error) {
	seen := make(map[string]bool)
	cleaned := make([]string, 0, len(domains))
	for _, d := range domains {
		trimmed := strings.ToLower(strings.TrimSpace(d))
		if trimmed == "" {
			continue
		}
		if seen[trimmed] {
			continue
		}
		seen[trimmed] = true
		cleaned = append(cleaned, trimmed)
	}
	if len(cleaned) > MaxAllowedDomains {
		return nil, fmt.Errorf("maximum %d allowed domains can be registered", MaxAllowedDomains)
	}
	return cleaned, nil
}

// ValidateEmailAllowedDomain checks if the email matches at least one allowed domain by suffix.
// When allowedDomains is empty, any valid email is permitted.
func ValidateEmailAllowedDomain(email string, allowedDomains []string) error {
	trimmedEmail := strings.ToLower(strings.TrimSpace(email))
	if trimmedEmail == "" || !strings.Contains(trimmedEmail, "@") {
		return errors.New("invalid email address")
	}
	if len(allowedDomains) == 0 {
		return nil
	}
	atIdx := strings.LastIndex(trimmedEmail, "@")
	if atIdx == -1 || atIdx == len(trimmedEmail)-1 {
		return errors.New("invalid email address")
	}
	domainPart := trimmedEmail[atIdx+1:]

	for _, d := range allowedDomains {
		cleanDomain := strings.ToLower(strings.TrimSpace(d))
		cleanDomain = strings.TrimPrefix(cleanDomain, "@")
		cleanDomain = strings.TrimPrefix(cleanDomain, ".")
		if cleanDomain == "" {
			continue
		}
		if domainPart == cleanDomain || strings.HasSuffix(domainPart, "."+cleanDomain) {
			return nil
		}
	}
	return errors.New("email domain is not allowed by system domain restrictions")
}

// RolePriority returns a numeric priority for ProjectRole (Admin > Editor > Viewer > None).
func RolePriority(r ProjectRole) int {
	switch r {
	case RoleAdmin:
		return 3
	case RoleEditor:
		return 2
	case RoleViewer:
		return 1
	default:
		return 0
	}
}

// ResolveEffectiveProjectRole returns the strongest role from a slice of roles (union / max priority).
func ResolveEffectiveProjectRole(roles []ProjectRole) ProjectRole {
	best := RoleNone
	for _, r := range roles {
		if RolePriority(r) > RolePriority(best) {
			best = r
		}
	}
	return best
}

// IsSkeletonField returns true if the field is a skeleton field required for Gantt/Kanban rendering.
// Skeleton fields cannot have view restrictions (always viewable by project members).
func IsSkeletonField(fieldSource, fieldKey string) bool {
	if fieldSource != FieldSourceStandard {
		return false
	}
	switch fieldKey {
	case StdFieldTitle, StdFieldStatus, StdFieldPlannedDates, StdFieldActualDates, StdFieldPlannedHours, StdFieldActualHours, StdFieldProgress:
		return true
	default:
		return false
	}
}

// RequiredStandardFields returns the list of standard fields that must be placed in every template.
func RequiredStandardFields(targetType string) []string {
	if targetType == TargetIssue {
		return []string{
			StdFieldTitle,
			StdFieldDescription,
			StdFieldLabels,
		}
	}
	return []string{
		StdFieldTitle,
		StdFieldDescription,
		StdFieldStatus,
		StdFieldAssignees,
		StdFieldLabels,
		StdFieldPlannedDates,
		StdFieldActualDates,
		StdFieldPlannedHours,
		StdFieldActualHours,
		StdFieldProgress,
	}
}

// ValidateTemplateLayout validates 1-3 column constraints and ensures all required standard fields are present.
func ValidateTemplateLayout(targetType string, rows []TemplateRow) error {
	if targetType != TargetIssue && targetType != TargetTask {
		return errors.New("invalid template target_type")
	}
	placedStandard := make(map[string]bool)
	seenField := make(map[string]bool)

	for _, row := range rows {
		if row.ColumnsCount < 1 || row.ColumnsCount > 3 {
			return fmt.Errorf("row %d columns_count must be between 1 and 3", row.RowOrder)
		}
		usedCols := make(map[int]bool)
		for _, f := range row.Fields {
			if f.ColumnOrder < 1 || f.ColumnOrder > row.ColumnsCount {
				return fmt.Errorf("field %s column_order %d exceeds row columns_count %d", f.FieldKey, f.ColumnOrder, row.ColumnsCount)
			}
			if usedCols[f.ColumnOrder] {
				return fmt.Errorf("duplicate column_order %d in row %d", f.ColumnOrder, row.RowOrder)
			}
			usedCols[f.ColumnOrder] = true

			uniqueKey := f.FieldSource + ":" + f.FieldKey
			if seenField[uniqueKey] {
				return fmt.Errorf("duplicate field %s in template", uniqueKey)
			}
			seenField[uniqueKey] = true

			if f.FieldSource == FieldSourceStandard {
				if f.FieldKey == StdFieldTitle && f.IsHidden {
					return errors.New("title field cannot be hidden")
				}
				placedStandard[f.FieldKey] = true
			}
		}
	}

	for _, reqKey := range RequiredStandardFields(targetType) {
		if !placedStandard[reqKey] {
			return fmt.Errorf("required standard field '%s' must be placed in the template layout", reqKey)
		}
	}
	return nil
}

// SubjectMatches checks if userID or any of userGroupIDs matches the subject list.
func SubjectMatches(subjects []SubjectRef, userID string, userGroupIDs []string) bool {
	groupSet := make(map[string]bool, len(userGroupIDs))
	for _, gid := range userGroupIDs {
		groupSet[gid] = true
	}
	for _, s := range subjects {
		if s.Type == SubjectUser && s.ID == userID {
			return true
		}
		if s.Type == SubjectGroup && groupSet[s.ID] {
			return true
		}
	}
	return false
}

// EvaluateFieldPermission evaluates whether a user can view and/or edit a specific template field.
func EvaluateFieldPermission(tf TemplateField, userID string, userGroupIDs []string, projectRole ProjectRole, ownerUserID string) FieldPermissionState {
	if projectRole == RoleNone {
		return FieldPermissionState{
			IsHidden:    tf.IsHidden,
			CanView:     false,
			CanEdit:     false,
			Placeholder: MaskedPlaceholder,
		}
	}
	if projectRole == RoleAdmin {
		return FieldPermissionState{
			IsHidden: tf.IsHidden,
			CanView:  true,
			CanEdit:  true,
		}
	}

	isOwner := userID != "" && userID == ownerUserID

	if !tf.IsRestricted {
		return FieldPermissionState{
			IsHidden: tf.IsHidden,
			CanView:  true,
			CanEdit:  projectRole == RoleEditor,
		}
	}

	// View check
	canView := false
	if IsSkeletonField(tf.FieldSource, tf.FieldKey) {
		canView = true
	} else if tf.AllowOwnerView && isOwner {
		canView = true
	} else if len(tf.ViewSubjects) == 0 {
		canView = true
	} else {
		canView = SubjectMatches(tf.ViewSubjects, userID, userGroupIDs)
	}

	if !canView {
		return FieldPermissionState{
			IsHidden:    tf.IsHidden,
			CanView:     false,
			CanEdit:     false,
			Placeholder: MaskedPlaceholder,
		}
	}

	// Edit check (Viewer can never edit)
	if projectRole == RoleViewer {
		return FieldPermissionState{
			IsHidden: tf.IsHidden,
			CanView:  true,
			CanEdit:  false,
		}
	}

	canEdit := false
	if tf.AllowOwnerEdit && isOwner {
		canEdit = true
	} else if len(tf.EditSubjects) == 0 {
		canEdit = true
	} else {
		canEdit = SubjectMatches(tf.EditSubjects, userID, userGroupIDs)
	}

	return FieldPermissionState{
		IsHidden: tf.IsHidden,
		CanView:  true,
		CanEdit:  canEdit,
	}
}

// HasAtMostTwoDecimals checks whether a float64 value has at most 2 decimal places.
func HasAtMostTwoDecimals(v float64) bool {
	if math.IsNaN(v) || math.IsInf(v, 0) {
		return false
	}
	scaled := v * 100.0
	return math.Abs(scaled-math.Round(scaled)) < 1e-6
}

// ValidateHoursPerDay validates that hours_per_day is between 1.00 and 24.00 and has at most 2 decimal places.
func ValidateHoursPerDay(v float64, fieldName string) error {
	if v < 1.0 || v > 24.0 {
		return fmt.Errorf("%s must be between 1 and 24", fieldName)
	}
	if !HasAtMostTwoDecimals(v) {
		return fmt.Errorf("%s must have at most 2 decimal places", fieldName)
	}
	return nil
}

// ValidateTaskHours validates that planned_hours / actual_hours is >= 0, <= 999999.99, and has at most 2 decimal places.
func ValidateTaskHours(v *float64, fieldName string) error {
	if v == nil {
		return nil
	}
	if *v < 0 || *v > 999999.99 {
		return fmt.Errorf("%s must be between 0 and 999999.99", fieldName)
	}
	if !HasAtMostTwoDecimals(*v) {
		return fmt.Errorf("%s must have at most 2 decimal places", fieldName)
	}
	return nil
}

// ValidateTaskCost validates that planned_cost / actual_cost is >= 0, <= 999999999999.99, and has at most 2 decimal places.
func ValidateTaskCost(v *float64, fieldName string) error {
	if v == nil {
		return nil
	}
	if *v < 0 || *v > 999999999999.99 {
		return fmt.Errorf("%s must be between 0 and 999999999999.99", fieldName)
	}
	if !HasAtMostTwoDecimals(*v) {
		return fmt.Errorf("%s must have at most 2 decimal places", fieldName)
	}
	return nil
}

// IsDoneStatusName returns true if the status name indicates completion ("完了", "クローズ", "done", "closed", or exact "close").
func IsDoneStatusName(name string) bool {
	lower := strings.ToLower(strings.TrimSpace(name))
	if lower == "close" || lower == "closed" {
		return true
	}
	if strings.Contains(lower, "完了") || strings.Contains(lower, "クローズ") || strings.Contains(lower, "done") {
		return true
	}
	// Check word-bounded "closed" (so "disclosed" or "unclosed" does not match)
	for _, token := range strings.FieldsFunc(lower, func(r rune) bool {
		return !(r >= 'a' && r <= 'z') && !(r >= '0' && r <= '9')
	}) {
		if token == "closed" || token == "close" {
			return true
		}
	}
	return false
}

// ResolveDoneStatusIDs determines the set of Done status IDs from a slice of statuses.
// First pass: any status matching IsDoneStatusName(st.Name).
// Fallback pass (only if 0 matched by name and len(statuses) >= 2): status(es) with display_order == maxOrder (when maxOrder > minOrder).
func ResolveDoneStatusIDs(statuses []Status) map[string]bool {
	doneIDs := make(map[string]bool)
	if len(statuses) == 0 {
		return doneIDs
	}
	for _, st := range statuses {
		if IsDoneStatusName(st.Name) {
			doneIDs[st.ID] = true
		}
	}
	if len(doneIDs) > 0 {
		return doneIDs
	}
	if len(statuses) >= 2 {
		minOrder := statuses[0].DisplayOrder
		maxOrder := statuses[0].DisplayOrder
		for _, st := range statuses[1:] {
			if st.DisplayOrder < minOrder {
				minOrder = st.DisplayOrder
			}
			if st.DisplayOrder > maxOrder {
				maxOrder = st.DisplayOrder
			}
		}
		if maxOrder > minOrder {
			for _, st := range statuses {
				if st.DisplayOrder == maxOrder {
					doneIDs[st.ID] = true
				}
			}
		}
	}
	return doneIDs
}

// CalculateIssueRollup computes the aggregated dates, hours, story points, weighted average progress, and deduplicated assignees for an Issue.
func CalculateIssueRollup(tasks []Task, hoursPerDay float64, doneStatusIDs ...map[string]bool) IssueRollup {
	if hoursPerDay < 1.0 || hoursPerDay > 24.0 {
		hoursPerDay = 8.0
	}
	rollup := IssueRollup{
		Assignees: []SubjectRef{},
		TaskCount: len(tasks),
	}
	if len(tasks) == 0 {
		return rollup
	}

	var doneSet map[string]bool
	if len(doneStatusIDs) > 0 && doneStatusIDs[0] != nil {
		doneSet = doneStatusIDs[0]
	}

	var minPlanStart, maxPlanEnd *string
	var minActStart, maxActEnd *string
	var totalWeight float64
	var weightedProgressSum float64
	seenAssignees := make(map[string]bool)

	for _, t := range tasks {
		if t.PlannedStartDate != nil && *t.PlannedStartDate != "" {
			if minPlanStart == nil || *t.PlannedStartDate < *minPlanStart {
				val := *t.PlannedStartDate
				minPlanStart = &val
			}
		}
		if t.PlannedEndDate != nil && *t.PlannedEndDate != "" {
			if maxPlanEnd == nil || *t.PlannedEndDate > *maxPlanEnd {
				val := *t.PlannedEndDate
				maxPlanEnd = &val
			}
		}
		if t.ActualStartDate != nil && *t.ActualStartDate != "" {
			if minActStart == nil || *t.ActualStartDate < *minActStart {
				val := *t.ActualStartDate
				minActStart = &val
			}
		}
		if t.ActualEndDate != nil && *t.ActualEndDate != "" {
			if maxActEnd == nil || *t.ActualEndDate > *maxActEnd {
				val := *t.ActualEndDate
				maxActEnd = &val
			}
		}

		if t.PlannedHours != nil {
			rollup.TotalPlannedHours += *t.PlannedHours
		}
		if t.ActualHours != nil {
			rollup.TotalActualHours += *t.ActualHours
		}
		if t.PlannedCost != nil {
			rollup.TotalPlannedCost += *t.PlannedCost
		}
		if t.ActualCost != nil {
			rollup.TotalActualCost += *t.ActualCost
		}
		if t.StoryPoints != nil {
			rollup.TotalStoryPoints += *t.StoryPoints
			isDone := t.Progress >= 100 || (doneSet != nil && doneSet[t.StatusID])
			if isDone {
				rollup.CompletedStoryPoints += *t.StoryPoints
			}
		}

		// Determine weight for weighted progress calculation
		weight := 0.0
		if t.PlannedHours != nil && *t.PlannedHours > 0 {
			weight = *t.PlannedHours
		} else if t.PlannedStartDate != nil && t.PlannedEndDate != nil && *t.PlannedStartDate != "" && *t.PlannedEndDate != "" {
			s, err1 := time.Parse("2006-01-02", *t.PlannedStartDate)
			e, err2 := time.Parse("2006-01-02", *t.PlannedEndDate)
			if err1 == nil && err2 == nil && !e.Before(s) {
				days := e.Sub(s).Hours()/24.0 + 1.0
				weight = days * hoursPerDay
			}
		}
		if weight <= 0 {
			weight = 1.0
		}

		totalWeight += weight
		weightedProgressSum += float64(t.Progress) * weight

		for _, a := range t.Assignees {
			key := a.Type + ":" + a.ID
			if !seenAssignees[key] {
				seenAssignees[key] = true
				rollup.Assignees = append(rollup.Assignees, a)
			}
		}
	}

	rollup.PlannedStartDate = minPlanStart
	rollup.PlannedEndDate = maxPlanEnd
	rollup.ActualStartDate = minActStart
	rollup.ActualEndDate = maxActEnd
	rollup.TotalPlannedHours = math.Round(rollup.TotalPlannedHours*100) / 100
	rollup.TotalActualHours = math.Round(rollup.TotalActualHours*100) / 100
	rollup.TotalPlannedCost = math.Round(rollup.TotalPlannedCost*100) / 100
	rollup.TotalActualCost = math.Round(rollup.TotalActualCost*100) / 100
	rollup.TotalStoryPoints = math.Round(rollup.TotalStoryPoints*100) / 100
	rollup.CompletedStoryPoints = math.Round(rollup.CompletedStoryPoints*100) / 100
	if totalWeight > 0 {
		rollup.WeightedProgress = math.Round((weightedProgressSum/totalWeight)*10) / 10
	}
	return rollup
}

