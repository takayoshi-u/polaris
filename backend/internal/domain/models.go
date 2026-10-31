package domain

import (
	"encoding/json"
	"time"
)

// User statuses
const (
	UserStatusInvited  = "invited"
	UserStatusActive   = "active"
	UserStatusDisabled = "disabled"
)

// System permissions
const (
	PermCreateProject = "create_project"
	PermDeleteProject = "delete_project"
	PermManageSystem  = "manage_system"
)

// Project roles
type ProjectRole string

const (
	RoleNone   ProjectRole = ""
	RoleViewer ProjectRole = "viewer"
	RoleEditor ProjectRole = "editor"
	RoleAdmin  ProjectRole = "admin"
)

// Subject types
const (
	SubjectUser  = "user"
	SubjectGroup = "group"
)

// Target types
const (
	TargetIssue = "issue"
	TargetTask  = "task"
)

// Issue states
const (
	IssueStateOpen   = "open"
	IssueStateClosed = "closed"
)

// Notification types
const (
	NotificationTypeMention = "mention"
	NotificationTypeAssign  = "assign"
)

// Custom field types (9 types)
const (
	FieldTypeTextSingle = "text_single"
	FieldTypeTextMulti  = "text_multi"
	FieldTypeMarkdown   = "markdown"
	FieldTypeNumber     = "number"
	FieldTypeDate       = "date"
	FieldTypeRadio      = "radio"
	FieldTypeCheckbox   = "checkbox"
	FieldTypeSelect     = "select"
	FieldTypeUserGroup  = "user_group"
)

// Field sources
const (
	FieldSourceStandard = "standard"
	FieldSourceCustom   = "custom"
)

// Standard field keys
const (
	StdFieldTitle        = "title"
	StdFieldDescription  = "description"
	StdFieldStatus       = "status"
	StdFieldAssignees    = "assignees"
	StdFieldLabels       = "labels"
	StdFieldPlannedDates = "planned_dates"
	StdFieldActualDates  = "actual_dates"
	StdFieldPlannedHours = "planned_hours"
	StdFieldActualHours  = "actual_hours"
	StdFieldPlannedCost  = "planned_cost"
	StdFieldActualCost   = "actual_cost"
	StdFieldProgress     = "progress"
)

// Placeholder for unviewable fields (Method A)
const MaskedPlaceholder = "🔒 閲覧権限がありません"

// Limits
const (
	MaxAllowedDomains     = 10
	MaxStatusesPerProject = 10
	MinStatusesPerProject = 1
	MaxAttachmentsPerItem = 100
	MaxAttachmentBytes    = 1073741824 // 1 GiB
	FileChunkSizeBytes    = 2 * 1024 * 1024 // 2 MiB per chunk
)

type User struct {
	ID          string    `json:"id"`
	Email       string    `json:"email"`
	HasPassword bool      `json:"has_password"`
	GoogleSub   *string   `json:"google_sub,omitempty"`
	DisplayName string    `json:"display_name"`
	AvatarURL   string    `json:"avatar_url"`
	Status      string    `json:"status"`
	Permissions []string  `json:"permissions,omitempty"`
	InviteURL   string    `json:"invite_url,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type Group struct {
	ID        string    `json:"id"`
	ProjectID *string   `json:"project_id"`
	Name      string    `json:"name"`
	MemberIDs []string  `json:"member_ids"`
	CreatedAt time.Time `json:"created_at"`
}

type SystemPermissionEntry struct {
	ID          string    `json:"id"`
	SubjectType string    `json:"subject_type"`
	SubjectID   string    `json:"subject_id"`
	Permission  string    `json:"permission"`
	CreatedAt   time.Time `json:"created_at"`
}

type SystemSettings struct {
	ID                     string    `json:"id"`
	DefaultHoursPerDay     float64   `json:"default_hours_per_day"`
	DefaultIssueTemplateID *string   `json:"default_issue_template_id"`
	DefaultTaskTemplateID  *string   `json:"default_task_template_id"`
	AllowedDomains         []string  `json:"allowed_domains"`
	UpdatedAt              time.Time `json:"updated_at"`
}

type SMTPSettings struct {
	ID          string    `json:"id"`
	Enabled     bool      `json:"enabled"`
	Host        string    `json:"host"`
	Port        int       `json:"port"`
	UseAuth     bool      `json:"use_auth"`
	Username    string    `json:"username"`
	HasPassword bool      `json:"has_password"`
	FromEmail   string    `json:"from_email"`
	FromName    string    `json:"from_name"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type Project struct {
	ID                     string      `json:"id"`
	Name                   string      `json:"name"`
	Description            string      `json:"description"`
	HoursPerDay            float64     `json:"hours_per_day"`
	DefaultIssueTemplateID *string     `json:"default_issue_template_id"`
	DefaultTaskTemplateID  *string     `json:"default_task_template_id"`
	TotalPlannedCost       float64     `json:"total_planned_cost"`
	TotalActualCost        float64     `json:"total_actual_cost"`
	CreatedBy              *string     `json:"created_by"`
	CurrentUserRole        ProjectRole `json:"current_user_role"`
	IsMember               bool        `json:"is_member"`
	CreatedAt              time.Time   `json:"created_at"`
	UpdatedAt              time.Time   `json:"updated_at"`
}

type ProjectMember struct {
	ID          string      `json:"id"`
	ProjectID   string      `json:"project_id"`
	SubjectType string      `json:"subject_type"`
	SubjectID   string      `json:"subject_id"`
	Role        ProjectRole `json:"role"`
	CreatedAt   time.Time   `json:"created_at"`
}

type Status struct {
	ID           string    `json:"id"`
	ProjectID    *string   `json:"project_id"`
	DisplayOrder int       `json:"display_order"`
	Name         string    `json:"name"`
	Color        string    `json:"color"`
	CreatedAt    time.Time `json:"created_at"`
}

type Label struct {
	ID        string    `json:"id"`
	ProjectID *string   `json:"project_id"`
	Name      string    `json:"name"`
	Color     string    `json:"color"`
	CreatedAt time.Time `json:"created_at"`
}

type SubjectRef struct {
	Type string `json:"type"` // "user" or "group"
	ID   string `json:"id"`
}

type CustomFieldDefinition struct {
	ID          string          `json:"id"`
	ProjectID   *string         `json:"project_id"`
	TargetType  string          `json:"target_type"`
	Name        string          `json:"name"`
	FieldType   string          `json:"field_type"`
	OptionsJSON json.RawMessage `json:"options"`
	CreatedAt   time.Time       `json:"created_at"`
}

type TemplateField struct {
	ID             string       `json:"id"`
	TemplateRowID  string       `json:"template_row_id"`
	ColumnOrder    int          `json:"column_order"`
	FieldSource    string       `json:"field_source"`
	FieldKey       string       `json:"field_key"`
	IsHidden       bool         `json:"is_hidden"`
	IsRestricted   bool         `json:"is_restricted"`
	AllowOwnerView bool         `json:"allow_owner_view"`
	AllowOwnerEdit bool         `json:"allow_owner_edit"`
	ViewSubjects   []SubjectRef `json:"view_subjects"`
	EditSubjects   []SubjectRef `json:"edit_subjects"`
}

type TemplateRow struct {
	ID           string          `json:"id"`
	TemplateID   string          `json:"template_id"`
	RowOrder     int             `json:"row_order"`
	ColumnsCount int             `json:"columns_count"`
	Fields       []TemplateField `json:"fields"`
}

type Template struct {
	ID         string        `json:"id"`
	ProjectID  *string       `json:"project_id"`
	TargetType string        `json:"target_type"`
	Name       string        `json:"name"`
	Rows       []TemplateRow `json:"rows"`
	CreatedAt  time.Time     `json:"created_at"`
	UpdatedAt  time.Time     `json:"updated_at"`
}

type FieldPermissionState struct {
	IsHidden    bool   `json:"is_hidden,omitempty"`
	CanView     bool   `json:"can_view"`
	CanEdit     bool   `json:"can_edit"`
	Placeholder string `json:"placeholder,omitempty"`
}

type IssueRollup struct {
	PlannedStartDate  *string      `json:"planned_start_date"`
	PlannedEndDate    *string      `json:"planned_end_date"`
	ActualStartDate   *string      `json:"actual_start_date"`
	ActualEndDate     *string      `json:"actual_end_date"`
	TotalPlannedHours float64      `json:"total_planned_hours"`
	TotalActualHours  float64      `json:"total_actual_hours"`
	TotalPlannedCost  float64      `json:"total_planned_cost"`
	TotalActualCost   float64      `json:"total_actual_cost"`
	WeightedProgress  float64      `json:"weighted_progress"`
	Assignees         []SubjectRef `json:"assignees"`
	TaskCount         int          `json:"task_count"`
}

type Issue struct {
	ID               string                          `json:"id"`
	ProjectID        string                          `json:"project_id"`
	TemplateID       *string                         `json:"template_id"`
	OwnerUserID      string                          `json:"owner_user_id"`
	State            string                          `json:"state"`
	Title            string                          `json:"title"`
	Description      *string                         `json:"description"`
	LabelIDs         []string                        `json:"label_ids"`
	CustomValues     map[string]json.RawMessage      `json:"custom_values"`
	FieldPermissions map[string]FieldPermissionState `json:"field_permissions"`
	DisplayOrder     int                             `json:"display_order"`
	Rollup           IssueRollup                     `json:"rollup"`
	Tasks            []Task                          `json:"tasks,omitempty"`
	CreatedAt        time.Time                       `json:"created_at"`
	UpdatedAt        time.Time                       `json:"updated_at"`
}

type Task struct {
	ID               string                          `json:"id"`
	ProjectID        string                          `json:"project_id"`
	IssueID          string                          `json:"issue_id"`
	SprintID         *string                         `json:"sprint_id"`
	TemplateID       *string                         `json:"template_id"`
	OwnerUserID      string                          `json:"owner_user_id"`
	StatusID         string                          `json:"status_id"`
	Title            string                          `json:"title"`
	Description      *string                         `json:"description"`
	Progress         int                             `json:"progress"`
	PlannedStartDate *string                         `json:"planned_start_date"`
	PlannedEndDate   *string                         `json:"planned_end_date"`
	ActualStartDate  *string                         `json:"actual_start_date"`
	ActualEndDate    *string                         `json:"actual_end_date"`
	PlannedHours     *float64                        `json:"planned_hours"`
	ActualHours      *float64                        `json:"actual_hours"`
	PlannedCost      *float64                        `json:"planned_cost"`
	ActualCost       *float64                        `json:"actual_cost"`
	StoryPoints      *float64                        `json:"story_points"`
	Assignees        []SubjectRef                    `json:"assignees"`
	LabelIDs         []string                        `json:"label_ids"`
	CustomValues     map[string]json.RawMessage      `json:"custom_values"`
	FieldPermissions map[string]FieldPermissionState `json:"field_permissions"`
	KanbanOrder      int                             `json:"kanban_order"`
	GanttOrder       int                             `json:"gantt_order"`
	CreatedAt        time.Time                       `json:"created_at"`
	UpdatedAt        time.Time                       `json:"updated_at"`
}

type Attachment struct {
	ID         string    `json:"id"`
	TargetType string    `json:"target_type"`
	TargetID   string    `json:"target_id"`
	BlobID     string    `json:"blob_id"`
	SHA512Hash string    `json:"sha512_hash"`
	FileName   string    `json:"file_name"`
	SizeBytes  int64     `json:"size_bytes"`
	MimeType   string    `json:"mime_type"`
	UploadedBy string    `json:"uploaded_by"`
	CreatedAt  time.Time `json:"created_at"`
}

type Comment struct {
	ID           string       `json:"id"`
	TargetType   string       `json:"target_type"`
	TargetID     string       `json:"target_id"`
	AuthorUserID string       `json:"author_user_id"`
	Body         string       `json:"body"`
	Mentions     []SubjectRef `json:"mentions"`
	CreatedAt    time.Time    `json:"created_at"`
	UpdatedAt    time.Time    `json:"updated_at"`
}

type Notification struct {
	ID               string    `json:"id"`
	UserID           string    `json:"user_id"`
	NotificationType string    `json:"notification_type"`
	TargetType       string    `json:"target_type"`
	TargetID         string    `json:"target_id"`
	ProjectID        string    `json:"project_id"`
	Message          string    `json:"message"`
	IsRead           bool      `json:"is_read"`
	CreatedAt        time.Time `json:"created_at"`
}

// Personal Access Token scopes
const (
	ScopeReadProjects  = "read:projects"
	ScopeWriteProjects = "write:projects"
	ScopeReadIssues    = "read:issues"
	ScopeWriteIssues   = "write:issues"
	ScopeReadTasks     = "read:tasks"
	ScopeWriteTasks    = "write:tasks"
	ScopeReadComments  = "read:comments"
	ScopeWriteComments = "write:comments"
	ScopeAdminSystem   = "admin:system"
)

type PersonalAccessToken struct {
	ID          string     `json:"id"`
	UserID      string     `json:"user_id"`
	Name        string     `json:"name"`
	TokenPrefix string     `json:"token_prefix"`
	Scopes      []string   `json:"scopes"`
	ExpiresAt   *time.Time `json:"expires_at"`
	LastUsedAt  *time.Time `json:"last_used_at"`
	RevokedAt   *time.Time `json:"revoked_at"`
	CreatedAt   time.Time  `json:"created_at"`
}

// Webhook events
const (
	WebhookEventTaskCreated       = "task.created"
	WebhookEventTaskUpdated       = "task.updated"
	WebhookEventTaskStatusChanged = "task.status_changed"
	WebhookEventIssueCreated      = "issue.created"
	WebhookEventIssueStateChanged = "issue.state_changed"
	WebhookEventCommentCreated    = "comment.created"
	WebhookEventAttachmentCreated = "attachment.created"
)

type Webhook struct {
	ID        string    `json:"id"`
	ProjectID *string   `json:"project_id"`
	Name      string    `json:"name"`
	URL       string    `json:"url"`
	Secret    string    `json:"secret,omitempty"`
	Events    []string  `json:"events"`
	IsActive  bool      `json:"is_active"`
	CreatedBy string    `json:"created_by"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type WebhookDelivery struct {
	ID           string    `json:"id"`
	WebhookID    string    `json:"webhook_id"`
	Event        string    `json:"event"`
	StatusCode   *int      `json:"status_code"`
	DurationMs   int       `json:"duration_ms"`
	RequestBody  any       `json:"request_body"`
	ResponseBody *string   `json:"response_body"`
	ErrorMessage *string   `json:"error_message"`
	CreatedAt    time.Time `json:"created_at"`
}

// Advanced Project Management Features

// Task Dependencies
type DependencyType string

const (
	DependencyFS DependencyType = "FS" // Finish-to-Start (default)
	DependencySS DependencyType = "SS" // Start-to-Start
	DependencyFF DependencyType = "FF" // Finish-to-Finish
	DependencySF DependencyType = "SF" // Start-to-Finish
)

type TaskDependency struct {
	ID             string         `json:"id"`
	ProjectID      string         `json:"project_id"`
	PredecessorID  string         `json:"predecessor_id"`
	SuccessorID    string         `json:"successor_id"`
	DependencyType DependencyType `json:"dependency_type"`
	LagDays        int            `json:"lag_days"`
	CreatedAt      time.Time      `json:"created_at"`
}

// Sprints
const (
	SprintStatusPlanned = "planned"
	SprintStatusActive  = "active"
	SprintStatusClosed  = "closed"
)

type Sprint struct {
	ID                string    `json:"id"`
	ProjectID         string    `json:"project_id"`
	Name              string    `json:"name"`
	Goal              *string   `json:"goal"`
	StartDate         *string   `json:"start_date"`
	EndDate           *string   `json:"end_date"`
	Status            string    `json:"status"`
	TaskCount         int       `json:"task_count"`
	TotalStoryPoints  float64   `json:"total_story_points"`
	TotalPlannedHours float64   `json:"total_planned_hours"`
	TotalActualHours  float64   `json:"total_actual_hours"`
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
}

type BurndownDataPoint struct {
	Date                 string  `json:"date"`
	IdealStoryPoints     float64 `json:"ideal_story_points"`
	RemainingStoryPoints float64 `json:"remaining_story_points"`
	IdealHours           float64 `json:"ideal_hours"`
	RemainingHours       float64 `json:"remaining_hours"`
}

type SprintBurndown struct {
	SprintID   string              `json:"sprint_id"`
	SprintName string              `json:"sprint_name"`
	StartDate  string              `json:"start_date"`
	EndDate    string              `json:"end_date"`
	TotalSP    float64             `json:"total_story_points"`
	TotalHours float64             `json:"total_hours"`
	Points     []BurndownDataPoint `json:"points"`
}

// Activity Logs
type ActivityLog struct {
	ID         string          `json:"id"`
	ProjectID  string          `json:"project_id"`
	EntityType string          `json:"entity_type"`
	EntityID   string          `json:"entity_id"`
	Action     string          `json:"action"`
	Details    json.RawMessage `json:"details"`
	ActorID    *string         `json:"actor_id"`
	ActorName  *string         `json:"actor_name,omitempty"`
	CreatedAt  time.Time       `json:"created_at"`
}

// Wiki Pages
type WikiPage struct {
	ID        string      `json:"id"`
	ProjectID string      `json:"project_id"`
	Title     string      `json:"title"`
	Slug      string      `json:"slug"`
	Content   string      `json:"content"`
	ParentID  *string     `json:"parent_id"`
	SortOrder int         `json:"sort_order"`
	CreatedBy *string     `json:"created_by"`
	UpdatedBy *string     `json:"updated_by"`
	Children  []*WikiPage `json:"children,omitempty"`
	CreatedAt time.Time   `json:"created_at"`
	UpdatedAt time.Time   `json:"updated_at"`
}

// Inbound Webhooks
type InboundWebhook struct {
	ID              string    `json:"id"`
	ProjectID       string    `json:"project_id"`
	Name            string    `json:"name"`
	TokenHash       string    `json:"-"`
	TokenPlain      string    `json:"token,omitempty"`
	Secret          string    `json:"secret"`
	DefaultIssueID  *string   `json:"default_issue_id"`
	DefaultStatusID int       `json:"default_status_id"`
	IsActive        bool      `json:"is_active"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

type InboundWebhookPayload struct {
	Title        string   `json:"title"`
	Description  *string  `json:"description"`
	StatusID     *string  `json:"status_id"`
	IssueID      *string  `json:"issue_id"`
	PlannedHours *float64 `json:"planned_hours"`
	StoryPoints  *float64 `json:"story_points"`
	AssigneeIDs  []string `json:"assignee_ids"`
	LabelIDs     []string `json:"label_ids"`
}

