export type SystemPermissionType = 'create_project' | 'delete_project' | 'manage_system'
export type ProjectRole = 'admin' | 'editor' | 'viewer' | 'none'
export type TargetType = 'issue' | 'task'
export type SubjectType = 'user' | 'group'
export type CustomFieldType =
  | 'text_single'
  | 'text_multi'
  | 'markdown'
  | 'number'
  | 'date'
  | 'radio'
  | 'checkbox'
  | 'select'
  | 'user_group'

export interface SubjectRef {
  type: SubjectType
  id: string
}

export interface User {
  id: string
  email: string
  has_password: boolean
  google_sub?: string | null
  display_name: string
  avatar_url: string
  status: 'invited' | 'active' | 'disabled'
  permissions?: SystemPermissionType[]
  invite_url?: string
  created_at: string
  updated_at: string
}

export interface SystemSettings {
  id: string
  default_hours_per_day: number
  default_issue_template_id?: string | null
  default_task_template_id?: string | null
  allowed_domains: string[]
  updated_at: string
}

export interface SMTPSettings {
  id: string
  enabled: boolean
  host: string
  port: number
  use_auth: boolean
  username: string
  has_password: boolean
  from_email: string
  from_name: string
  updated_at: string
}

export interface Group {
  id: string
  project_id?: string | null
  name: string
  member_ids: string[]
  created_at: string
}

export interface SystemPermission {
  id: string
  subject_type: SubjectType
  subject_id: string
  permission: SystemPermissionType
  created_at: string
}

export interface Project {
  id: string
  name: string
  description: string
  hours_per_day: number
  default_issue_template_id?: string | null
  default_task_template_id?: string | null
  total_planned_cost: number
  total_actual_cost: number
  created_by?: string | null
  current_user_role: ProjectRole
  is_member: boolean
  created_at: string
  updated_at: string
}

export interface ProjectMember {
  id: string
  project_id: string
  subject_type: SubjectType
  subject_id: string
  role: 'admin' | 'editor' | 'viewer'
  created_at: string
}

export interface Status {
  id: string
  project_id?: string | null
  display_order: number
  name: string
  color: string
  created_at: string
}

export interface Label {
  id: string
  project_id?: string | null
  name: string
  color: string
  created_at: string
}

export interface CustomFieldDefinition {
  id: string
  project_id?: string | null
  target_type: TargetType
  name: string
  field_type: CustomFieldType
  options: string[]
  created_at: string
}

export interface TemplateField {
  id?: string
  template_row_id?: string
  column_order: number
  field_source: 'standard' | 'custom'
  field_key: string
  is_hidden?: boolean
  is_restricted: boolean
  allow_owner_view: boolean
  allow_owner_edit: boolean
  view_subjects: SubjectRef[]
  edit_subjects: SubjectRef[]
}

export interface TemplateRow {
  id?: string
  template_id?: string
  row_order: number
  columns_count: number
  fields: TemplateField[]
}

export interface Template {
  id: string
  project_id?: string | null
  target_type: TargetType
  name: string
  rows: TemplateRow[]
  created_at: string
  updated_at: string
}

export interface FieldPermissionState {
  is_hidden?: boolean
  can_view: boolean
  can_edit: boolean
  placeholder?: string
}

export interface IssueRollup {
  planned_start_date?: string | null
  planned_end_date?: string | null
  actual_start_date?: string | null
  actual_end_date?: string | null
  total_planned_hours: number
  total_actual_hours: number
  total_planned_cost: number
  total_actual_cost: number
  weighted_progress: number
  assignees: SubjectRef[]
  task_count: number
}

export interface Task {
  id: string
  project_id: string
  issue_id: string
  template_id?: string | null
  owner_user_id: string
  status_id: string
  title: string
  description?: string | null
  progress: number
  planned_start_date?: string | null
  planned_end_date?: string | null
  actual_start_date?: string | null
  actual_end_date?: string | null
  planned_hours?: number | null
  actual_hours?: number | null
  planned_cost?: number | null
  actual_cost?: number | null
  sprint_id?: string | null
  story_points?: number | null
  assignees: SubjectRef[]
  label_ids: string[]
  custom_values: Record<string, any>
  field_permissions: Record<string, FieldPermissionState>
  kanban_order: number
  gantt_order: number
  created_at: string
  updated_at: string
}

export interface Issue {
  id: string
  project_id: string
  template_id?: string | null
  owner_user_id: string
  state: 'open' | 'closed'
  title: string
  description?: string | null
  label_ids: string[]
  custom_values: Record<string, any>
  field_permissions: Record<string, FieldPermissionState>
  display_order: number
  rollup: IssueRollup
  tasks?: Task[]
  created_at: string
  updated_at: string
}

export interface Attachment {
  id: string
  target_type: TargetType
  target_id: string
  blob_id: string
  sha512_hash: string
  file_name: string
  size_bytes: number
  mime_type: string
  uploaded_by: string
  created_at: string
}

export interface Comment {
  id: string
  target_type: TargetType
  target_id: string
  author_user_id: string
  body: string
  mentions: SubjectRef[]
  created_at: string
  updated_at: string
}

export interface NotificationItem {
  id: string
  user_id: string
  notification_type: 'mention' | 'assign'
  target_type: TargetType
  target_id: string
  project_id: string
  message: string
  is_read: boolean
  created_at: string
}

export interface PersonalAccessToken {
  id: string
  user_id: string
  name: string
  token_prefix: string
  scopes: string[]
  expires_at?: string | null
  last_used_at?: string | null
  revoked_at?: string | null
  created_at: string
}

export interface Webhook {
  id: string
  project_id?: string | null
  name: string
  url: string
  secret?: string
  events: string[]
  is_active: boolean
  created_by: string
  created_at: string
  updated_at: string
}

export interface WebhookDelivery {
  id: string
  webhook_id: string
  event: string
  status_code?: number | null
  duration_ms: number
  request_body?: any
  response_body?: string | null
  error_message?: string | null
  created_at: string
}

export type DependencyType = 'FS' | 'SS' | 'FF' | 'SF'

export interface TaskDependency {
  id: string
  project_id: string
  predecessor_id: string
  successor_id: string
  dependency_type: DependencyType
  lag_days: number
  created_at: string
}

export interface Sprint {
  id: string
  project_id: string
  name: string
  goal?: string | null
  start_date?: string | null
  end_date?: string | null
  status: 'planned' | 'active' | 'closed'
  task_count?: number
  total_story_points?: number
  total_planned_hours?: number
  total_actual_hours?: number
  created_at: string
  updated_at: string
}

export interface BurndownDataPoint {
  date: string
  ideal_story_points: number
  remaining_story_points: number
  ideal_hours: number
  remaining_hours: number
}

export interface SprintBurndown {
  sprint_id: string
  sprint_name: string
  start_date?: string | null
  end_date?: string | null
  total_story_points: number
  total_hours: number
  points: BurndownDataPoint[]
}

export interface ActivityLog {
  id: string
  project_id: string
  entity_type: string
  entity_id: string
  action: string
  details?: Record<string, any>
  actor_id?: string | null
  actor_name?: string | null
  user_id?: string
  created_at: string
}

export interface WikiPage {
  id: string
  project_id: string
  title: string
  slug: string
  content: string
  parent_id?: string | null
  sort_order?: number
  created_by?: string | null
  updated_by?: string | null
  children?: WikiPage[]
  created_at: string
  updated_at: string
}

export interface InboundWebhook {
  id: string
  project_id: string
  name: string
  token?: string
  secret?: string
  default_issue_id?: string | null
  default_status_id?: string | number | null
  is_active: boolean
  created_at: string
  updated_at: string
}

export interface CSVImportResult {
  imported_tasks: number
  errors: string[]
}



