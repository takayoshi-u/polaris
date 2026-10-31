package domain_test

import (
	"testing"

	"github.com/takayoshi-u/polaris/backend/internal/domain"
)

func TestValidateAllowedDomainsAndSuffixMatch(t *testing.T) {
	// Empty list allows any domain
	if err := domain.ValidateEmailAllowedDomain("user@anything.org", nil); err != nil {
		t.Fatalf("expected any domain to be allowed when list is empty, got: %v", err)
	}

	domains, err := domain.ValidateAllowedDomainsList([]string{"example.com", "sub.corp.jp", "EXAMPLE.COM"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(domains) != 2 {
		t.Fatalf("expected 2 deduplicated domains, got %d", len(domains))
	}

	// Suffix match check
	if err := domain.ValidateEmailAllowedDomain("alice@example.com", domains); err != nil {
		t.Fatalf("expected alice@example.com to match, got: %v", err)
	}
	if err := domain.ValidateEmailAllowedDomain("bob@dev.sub.corp.jp", domains); err != nil {
		t.Fatalf("expected bob@dev.sub.corp.jp to match suffix sub.corp.jp, got: %v", err)
	}
	if err := domain.ValidateEmailAllowedDomain("mallory@other.com", domains); err == nil {
		t.Fatal("expected mallory@other.com to be rejected")
	}
	// Attacker domain with matching suffix must NOT match example.com
	if err := domain.ValidateEmailAllowedDomain("attacker@badexample.com", domains); err == nil {
		t.Fatal("expected attacker@badexample.com to be rejected")
	}

	// Initial list with more than 10 items but <= 10 after deduplication/trimming should succeed
	duplicates := []string{"d1.com", "d1.com", " ", "d2.com", "D2.COM", "d3.com", "d4.com", "d5.com", "d6.com", "d7.com", "d8.com", "d9.com"}
	if cleaned, err := domain.ValidateAllowedDomainsList(duplicates); err != nil || len(cleaned) != 9 {
		t.Fatalf("expected deduplicated list with length 9 to succeed, got len=%d, err=%v", len(cleaned), err)
	}

	// Exceeding 10 unique domains should fail
	eleven := []string{"d1.com", "d2.com", "d3.com", "d4.com", "d5.com", "d6.com", "d7.com", "d8.com", "d9.com", "d10.com", "d11.com"}
	if _, err := domain.ValidateAllowedDomainsList(eleven); err == nil {
		t.Fatal("expected error when registering more than 10 domains")
	}
}

func TestResolveEffectiveProjectRole(t *testing.T) {
	role := domain.ResolveEffectiveProjectRole([]domain.ProjectRole{domain.RoleViewer, domain.RoleEditor})
	if role != domain.RoleEditor {
		t.Fatalf("expected RoleEditor, got %v", role)
	}
	role = domain.ResolveEffectiveProjectRole([]domain.ProjectRole{domain.RoleViewer, domain.RoleAdmin, domain.RoleEditor})
	if role != domain.RoleAdmin {
		t.Fatalf("expected RoleAdmin, got %v", role)
	}
	role = domain.ResolveEffectiveProjectRole(nil)
	if role != domain.RoleNone {
		t.Fatalf("expected RoleNone, got %v", role)
	}
}

func TestEvaluateFieldPermission(t *testing.T) {
	ownerID := "user-owner"
	otherID := "user-other"
	managerGroupID := "group-managers"

	// 1. Restricted custom field with AllowOwnerView=false, AllowOwnerEdit=false (confidential manager field)
	secretField := domain.TemplateField{
		FieldSource:    domain.FieldSourceCustom,
		FieldKey:       "field-secret-budget",
		IsRestricted:   true,
		AllowOwnerView: false,
		AllowOwnerEdit: false,
		ViewSubjects:   []domain.SubjectRef{{Type: domain.SubjectGroup, ID: managerGroupID}},
		EditSubjects:   []domain.SubjectRef{{Type: domain.SubjectGroup, ID: managerGroupID}},
	}

	// Owner who is NOT in managerGroup cannot view or edit
	ownerPerm := domain.EvaluateFieldPermission(secretField, ownerID, []string{"group-devs"}, domain.RoleEditor, ownerID)
	if ownerPerm.CanView || ownerPerm.CanEdit {
		t.Fatalf("expected owner without manager group to be denied on AllowOwnerView=false field, got %+v", ownerPerm)
	}
	if ownerPerm.Placeholder != domain.MaskedPlaceholder {
		t.Fatalf("expected placeholder %q, got %q", domain.MaskedPlaceholder, ownerPerm.Placeholder)
	}

	// Member in managerGroup (Editor role) CAN view and edit
	mgrPerm := domain.EvaluateFieldPermission(secretField, otherID, []string{managerGroupID}, domain.RoleEditor, ownerID)
	if !mgrPerm.CanView || !mgrPerm.CanEdit {
		t.Fatalf("expected manager editor to view and edit, got %+v", mgrPerm)
	}

	// Member in managerGroup with Viewer role CAN view but CANNOT edit
	viewerPerm := domain.EvaluateFieldPermission(secretField, otherID, []string{managerGroupID}, domain.RoleViewer, ownerID)
	if !viewerPerm.CanView || viewerPerm.CanEdit {
		t.Fatalf("expected manager viewer to view only (not edit), got %+v", viewerPerm)
	}

	// 2. Restricted field with AllowOwnerView=true, AllowOwnerEdit=true
	ownerAllowedField := secretField
	ownerAllowedField.AllowOwnerView = true
	ownerAllowedField.AllowOwnerEdit = true
	ownerPerm2 := domain.EvaluateFieldPermission(ownerAllowedField, ownerID, []string{"group-devs"}, domain.RoleEditor, ownerID)
	if !ownerPerm2.CanView || !ownerPerm2.CanEdit {
		t.Fatalf("expected owner to view and edit when AllowOwnerView/Edit=true, got %+v", ownerPerm2)
	}

	// 3. Skeleton field (e.g., planned_dates) is ALWAYS viewable even if restricted, and only edit is restricted
	skeletonField := domain.TemplateField{
		FieldSource:    domain.FieldSourceStandard,
		FieldKey:       domain.StdFieldPlannedDates,
		IsRestricted:   true,
		AllowOwnerView: false,
		AllowOwnerEdit: false,
		ViewSubjects:   []domain.SubjectRef{{Type: domain.SubjectGroup, ID: managerGroupID}},
		EditSubjects:   []domain.SubjectRef{{Type: domain.SubjectGroup, ID: managerGroupID}},
	}
	skelPerm := domain.EvaluateFieldPermission(skeletonField, otherID, []string{"group-devs"}, domain.RoleEditor, ownerID)
	if !skelPerm.CanView || skelPerm.CanEdit {
		t.Fatalf("expected skeleton field to be viewable but not editable by general member, got %+v", skelPerm)
	}

	// 4. Cost field (planned_cost) is NOT a skeleton field, so it CAN be view-restricted and hidden
	costField := domain.TemplateField{
		FieldSource:    domain.FieldSourceStandard,
		FieldKey:       domain.StdFieldPlannedCost,
		IsHidden:       true,
		IsRestricted:   true,
		AllowOwnerView: false,
		AllowOwnerEdit: false,
		ViewSubjects:   []domain.SubjectRef{{Type: domain.SubjectGroup, ID: managerGroupID}},
		EditSubjects:   []domain.SubjectRef{{Type: domain.SubjectGroup, ID: managerGroupID}},
	}
	costPerm := domain.EvaluateFieldPermission(costField, otherID, []string{"group-devs"}, domain.RoleEditor, ownerID)
	if costPerm.CanView || costPerm.CanEdit || !costPerm.IsHidden {
		t.Fatalf("expected cost field to be view-restricted and hidden for general member, got %+v", costPerm)
	}
}

func TestValidateTemplateLayout(t *testing.T) {
	validIssueRows := []domain.TemplateRow{
		{
			RowOrder:     1,
			ColumnsCount: 2,
			Fields: []domain.TemplateField{
				{ColumnOrder: 1, FieldSource: domain.FieldSourceStandard, FieldKey: domain.StdFieldTitle},
				{ColumnOrder: 2, FieldSource: domain.FieldSourceStandard, FieldKey: domain.StdFieldLabels},
			},
		},
		{
			RowOrder:     2,
			ColumnsCount: 1,
			Fields: []domain.TemplateField{
				{ColumnOrder: 1, FieldSource: domain.FieldSourceStandard, FieldKey: domain.StdFieldDescription},
			},
		},
	}
	if err := domain.ValidateTemplateLayout(domain.TargetIssue, validIssueRows); err != nil {
		t.Fatalf("expected valid issue template layout to pass, got: %v", err)
	}

	// Missing required standard field 'description' should fail
	incompleteRows := validIssueRows[:1]
	if err := domain.ValidateTemplateLayout(domain.TargetIssue, incompleteRows); err == nil {
		t.Fatal("expected error when a required standard field is omitted from template")
	}

	// Invalid columns_count = 4 should fail
	invalidCols := []domain.TemplateRow{
		{RowOrder: 1, ColumnsCount: 4},
	}
	if err := domain.ValidateTemplateLayout(domain.TargetIssue, invalidCols); err == nil {
		t.Fatal("expected error when columns_count > 3")
	}

	// Hiding 'title' should fail, while hiding 'labels' should succeed
	hiddenLabelsRows := []domain.TemplateRow{
		{
			RowOrder:     1,
			ColumnsCount: 2,
			Fields: []domain.TemplateField{
				{ColumnOrder: 1, FieldSource: domain.FieldSourceStandard, FieldKey: domain.StdFieldTitle, IsHidden: false},
				{ColumnOrder: 2, FieldSource: domain.FieldSourceStandard, FieldKey: domain.StdFieldLabels, IsHidden: true},
			},
		},
		{
			RowOrder:     2,
			ColumnsCount: 1,
			Fields: []domain.TemplateField{
				{ColumnOrder: 1, FieldSource: domain.FieldSourceStandard, FieldKey: domain.StdFieldDescription, IsHidden: true},
			},
		},
	}
	if err := domain.ValidateTemplateLayout(domain.TargetIssue, hiddenLabelsRows); err != nil {
		t.Fatalf("expected hiding non-title fields to pass, got: %v", err)
	}

	hiddenTitleRows := []domain.TemplateRow{
		{
			RowOrder:     1,
			ColumnsCount: 2,
			Fields: []domain.TemplateField{
				{ColumnOrder: 1, FieldSource: domain.FieldSourceStandard, FieldKey: domain.StdFieldTitle, IsHidden: true},
				{ColumnOrder: 2, FieldSource: domain.FieldSourceStandard, FieldKey: domain.StdFieldLabels},
			},
		},
		{
			RowOrder:     2,
			ColumnsCount: 1,
			Fields: []domain.TemplateField{
				{ColumnOrder: 1, FieldSource: domain.FieldSourceStandard, FieldKey: domain.StdFieldDescription},
			},
		},
	}
	if err := domain.ValidateTemplateLayout(domain.TargetIssue, hiddenTitleRows); err == nil {
		t.Fatal("expected error when hiding title field")
	}
}

func TestCalculateIssueRollup(t *testing.T) {
	s1, e1 := "2026-10-01", "2026-10-02" // 2 days
	s2, e2 := "2026-10-01", "2026-10-05" // 5 days
	as1, ae1 := "2026-10-01", "2026-10-03"
	h1 := 4.0  // half-day task (4h), progress 100%
	h2 := 16.0 // 2-day task (16h), progress 50%
	ah1 := 3.5
	ah2 := 10.0
	pc1 := 12000.50
	pc2 := 48000.25
	ac1 := 11500.00
	ac2 := 30000.75

	tasks := []domain.Task{
		{
			ID:               "t1",
			Progress:         100,
			PlannedStartDate: &s1,
			PlannedEndDate:   &e1,
			ActualStartDate:  &as1,
			ActualEndDate:    &ae1,
			PlannedHours:     &h1,
			ActualHours:      &ah1,
			PlannedCost:      &pc1,
			ActualCost:       &ac1,
			Assignees: []domain.SubjectRef{
				{Type: domain.SubjectUser, ID: "u1"},
				{Type: domain.SubjectGroup, ID: "g1"},
			},
		},
		{
			ID:               "t2",
			Progress:         50,
			PlannedStartDate: &s2,
			PlannedEndDate:   &e2,
			PlannedHours:     &h2,
			ActualHours:      &ah2,
			PlannedCost:      &pc2,
			ActualCost:       &ac2,
			Assignees: []domain.SubjectRef{
				{Type: domain.SubjectUser, ID: "u1"}, // duplicate u1 should be deduplicated
				{Type: domain.SubjectUser, ID: "u2"},
			},
		},
	}

	// Weighted progress: (100*4 + 50*16) / (4 + 16) = 1200 / 20 = 60.0%
	rollup := domain.CalculateIssueRollup(tasks, 8.0)
	if rollup.PlannedStartDate == nil || *rollup.PlannedStartDate != "2026-10-01" {
		t.Fatalf("unexpected PlannedStartDate: %v", rollup.PlannedStartDate)
	}
	if rollup.PlannedEndDate == nil || *rollup.PlannedEndDate != "2026-10-05" {
		t.Fatalf("unexpected PlannedEndDate: %v", rollup.PlannedEndDate)
	}
	if rollup.TotalPlannedHours != 20.0 {
		t.Fatalf("expected TotalPlannedHours 20.0, got %v", rollup.TotalPlannedHours)
	}
	if rollup.TotalActualHours != 13.5 {
		t.Fatalf("expected TotalActualHours 13.5, got %v", rollup.TotalActualHours)
	}
	if rollup.TotalPlannedCost != 60000.75 {
		t.Fatalf("expected TotalPlannedCost 60000.75, got %v", rollup.TotalPlannedCost)
	}
	if rollup.TotalActualCost != 41500.75 {
		t.Fatalf("expected TotalActualCost 41500.75, got %v", rollup.TotalActualCost)
	}
	if rollup.WeightedProgress != 60.0 {
		t.Fatalf("expected WeightedProgress 60.0, got %v", rollup.WeightedProgress)
	}
	if len(rollup.Assignees) != 3 {
		t.Fatalf("expected 3 deduplicated assignees (u1, g1, u2), got %d", len(rollup.Assignees))
	}
}

func TestValidateHoursPrecision(t *testing.T) {
	// Valid hours_per_day (up to 2 decimal places)
	for _, valid := range []float64{1, 7.5, 7.75, 8.25, 24.0} {
		if err := domain.ValidateHoursPerDay(valid, "hours_per_day"); err != nil {
			t.Fatalf("expected %v to be valid, got: %v", valid, err)
		}
	}
	// Invalid hours_per_day (out of range or >2 decimal places)
	for _, invalid := range []float64{0.5, 24.01, 7.125, 8.333} {
		if err := domain.ValidateHoursPerDay(invalid, "hours_per_day"); err == nil {
			t.Fatalf("expected %v to fail validation, but passed", invalid)
		}
	}

	// Valid task hours (up to 2 decimal places)
	for _, valid := range []float64{0, 0.25, 1.5, 12.75, 100.99} {
		v := valid
		if err := domain.ValidateTaskHours(&v, "planned_hours"); err != nil {
			t.Fatalf("expected task hours %v to be valid, got: %v", valid, err)
		}
	}
	// Invalid task hours (<0 or >2 decimal places)
	for _, invalid := range []float64{-0.01, 1.125, 3.333} {
		v := invalid
		if err := domain.ValidateTaskHours(&v, "planned_hours"); err == nil {
			t.Fatalf("expected task hours %v to fail validation, but passed", invalid)
		}
	}

	// Valid task cost (up to 2 decimal places)
	for _, valid := range []float64{0, 0.50, 1500.25, 99999999.99} {
		v := valid
		if err := domain.ValidateTaskCost(&v, "planned_cost"); err != nil {
			t.Fatalf("expected task cost %v to be valid, got: %v", valid, err)
		}
	}
	// Invalid task cost (<0 or >2 decimal places)
	for _, invalid := range []float64{-0.01, 100.125, 99.999} {
		v := invalid
		if err := domain.ValidateTaskCost(&v, "planned_cost"); err == nil {
			t.Fatalf("expected task cost %v to fail validation, but passed", invalid)
		}
	}
}


