/**
 * Polaris HTTP REST Client for MCP Server
 */

export interface PolarisClientConfig {
  baseUrl?: string;
  apiToken?: string;
  email?: string;
  password?: string;
  fetchFn?: typeof fetch;
}

export interface Project {
  id: string;
  name: string;
  key: string;
  description?: string;
  sprint_duration_days?: number;
  created_at?: string;
  updated_at?: string;
  [key: string]: any;
}

export interface Issue {
  id: string;
  project_id: string;
  key_number?: number;
  title: string;
  description?: string;
  state?: string;
  type_id?: string | null;
  template_id?: string;
  premortem?: string;
  postmortem?: string;
  label_ids?: string[];
  custom_values?: Record<string, any>;
  tasks?: Task[];
  [key: string]: any;
}

export interface Task {
  id: string;
  issue_id: string;
  project_id?: string;
  title: string;
  description?: string;
  status_id?: string;
  status_name?: string;
  progress?: number;
  story_points?: number;
  planned_hours?: number;
  actual_hours?: number;
  planned_cost?: number;
  actual_cost?: number;
  sprint_id?: string | null;
  type_id?: string | null;
  planned_start_date?: string;
  planned_end_date?: string;
  actual_start_date?: string;
  actual_end_date?: string;
  assignees?: any[];
  custom_values?: Record<string, any>;
  label_ids?: string[];
  [key: string]: any;
}

export interface Sprint {
  id: string;
  project_id: string;
  name: string;
  start_date: string;
  end_date: string;
  goal?: string;
  status?: string;
  [key: string]: any;
}

export interface WikiPage {
  id: string;
  project_id: string;
  parent_id?: string | null;
  title: string;
  content: string;
  version?: number;
  created_at?: string;
  updated_at?: string;
  [key: string]: any;
}

export interface IssueCreateInput {
  title: string;
  description?: string;
  type_id?: string;
  template_id?: string;
  premortem?: string;
  postmortem?: string;
  label_ids?: string[];
  custom_values?: Record<string, any>;
  create_initial_task?: boolean;
}

export interface IssueUpdateInput {
  title?: string;
  description?: string;
  state?: string;
  type_id?: string | null;
  template_id?: string;
  premortem?: string;
  postmortem?: string;
  label_ids?: string[];
  custom_values?: Record<string, any>;
  display_order?: number;
}

export interface TaskCreateInput {
  issue_id: string;
  title: string;
  description?: string;
  status_id?: string;
  progress?: number;
  story_points?: number;
  planned_hours?: number;
  actual_hours?: number;
  sprint_id?: string;
  type_id?: string;
  planned_start_date?: string;
  planned_end_date?: string;
  assignees?: any[];
  label_ids?: string[];
  custom_values?: Record<string, any>;
}

export interface TaskUpdateInput {
  issue_id?: string;
  title?: string;
  description?: string;
  status_id?: string;
  progress?: number;
  story_points?: number;
  planned_hours?: number;
  actual_hours?: number;
  sprint_id?: string | null;
  type_id?: string | null;
  planned_start_date?: string | null;
  planned_end_date?: string | null;
  assignees?: any[];
  label_ids?: string[];
  custom_values?: Record<string, any>;
}

export interface SprintCreateInput {
  name?: string;
  start_date?: string;
  end_date?: string;
  duration_days?: number;
  goal?: string;
}

export interface WikiPageSaveInput {
  wiki_id?: string;
  title: string;
  content: string;
  parent_id?: string | null;
}

export interface ProjectSummary {
  project: Project;
  members_count: number;
  issues_count: number;
  tasks_count: number;
  completed_tasks_count: number;
  task_completion_rate: number;
  total_story_points: number;
  completed_story_points: number;
  total_planned_hours: number;
  total_actual_hours: number;
  sprints_count: number;
  active_sprint?: Sprint | null;
  issues_by_state: Record<string, number>;
  tasks_by_status: Record<string, number>;
}

/**
 * Helper to calculate local YYYY-MM-DD
 */
export function formatLocalYMD(d: Date = new Date()): string {
  const year = d.getFullYear();
  const month = String(d.getMonth() + 1).padStart(2, '0');
  const day = String(d.getDate()).padStart(2, '0');
  return `${year}-${month}-${day}`;
}

/**
 * Helper to add days to YYYY-MM-DD
 */
export function addDaysYMD(ymd: string, days: number): string {
  const parts = ymd.split('-').map(Number);
  const date = new Date(parts[0], parts[1] - 1, parts[2]);
  date.setDate(date.getDate() + days);
  return formatLocalYMD(date);
}

/**
 * Polaris HTTP REST Client
 */
export class PolarisClient {
  public readonly baseUrl: string;
  private apiToken?: string;
  private email?: string;
  private password?: string;
  private sessionToken?: string;
  private fetchFn: typeof fetch;

  constructor(config: PolarisClientConfig = {}) {
    const rawUrl = config.baseUrl || process.env.POLARIS_BASE_URL || 'http://localhost:8080/api/v1';
    this.baseUrl = rawUrl.replace(/\/+$/, '');
    this.apiToken = config.apiToken || process.env.POLARIS_API_TOKEN;
    this.email = config.email || process.env.POLARIS_EMAIL;
    this.password = config.password || process.env.POLARIS_PASSWORD;
    this.fetchFn = config.fetchFn || globalThis.fetch;
  }

  /**
   * Set API Token manually
   */
  public setApiToken(token: string) {
    this.apiToken = token;
  }

  /**
   * Ensures an authentication token is available.
   * If apiToken is missing but email & password are provided, performs login.
   */
  private async ensureAuthenticated(): Promise<string> {
    if (this.apiToken) {
      return this.apiToken;
    }
    if (this.sessionToken) {
      return this.sessionToken;
    }
    if (this.email && this.password) {
      const res = await this.fetchFn(`${this.baseUrl}/auth/login`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ email: this.email, password: this.password }),
      });
      if (!res.ok) {
        const errText = await res.text();
        throw new Error(`Login failed with status ${res.status}: ${errText}`);
      }
      const data = (await res.json()) as { session_token?: string; error?: string };
      if (!data.session_token) {
        throw new Error(`Login failed: No session token returned (${JSON.stringify(data)})`);
      }
      this.sessionToken = data.session_token;
      return this.sessionToken;
    }
    throw new Error('Authentication required: POLARIS_API_TOKEN or POLARIS_EMAIL & POLARIS_PASSWORD must be configured.');
  }

  /**
   * Execute an authenticated HTTP request
   */
  public async request<T = any>(endpoint: string, options: RequestInit = {}): Promise<T> {
    const token = await this.ensureAuthenticated();
    const cleanEndpoint = endpoint.startsWith('/') ? endpoint : `/${endpoint}`;
    const url = `${this.baseUrl}${cleanEndpoint}`;

    const headers: Record<string, string> = {
      'Content-Type': 'application/json',
      Authorization: `Bearer ${token}`,
      ...(options.headers as Record<string, string>),
    };

    const res = await this.fetchFn(url, {
      ...options,
      headers,
    });

    if (res.status === 204) {
      return {} as T;
    }

    const contentType = res.headers.get('content-type') || '';
    if (!res.ok) {
      let errorMessage = `HTTP error ${res.status} ${res.statusText}`;
      if (contentType.includes('application/json')) {
        try {
          const errData = await res.json();
          errorMessage = errData.error || errData.message || JSON.stringify(errData);
        } catch {
          // ignore parsing error
        }
      } else {
        const text = await res.text();
        if (text) errorMessage = text;
      }
      throw new Error(`Polaris API Error (${res.status}): ${errorMessage}`);
    }

    if (contentType.includes('application/json')) {
      return (await res.json()) as T;
    }
    return (await res.text()) as unknown as T;
  }

  // ==========================================
  // 1. Projects
  // ==========================================

  public async listProjects(): Promise<Project[]> {
    const projects = await this.request<Project[]>('/projects');
    return Array.isArray(projects) ? projects : [];
  }

  public async getProject(projectId: string): Promise<Project> {
    return this.request<Project>(`/projects/${encodeURIComponent(projectId)}`);
  }

  public async getProjectSummary(projectId: string): Promise<ProjectSummary> {
    const pId = encodeURIComponent(projectId);
    const [project, issues, sprints, members] = await Promise.all([
      this.getProject(projectId),
      this.listIssues(projectId),
      this.listSprints(projectId).catch(() => [] as Sprint[]),
      this.request<any[]>(`/projects/${pId}/members`).catch(() => []),
    ]);

    const issuesList = Array.isArray(issues) ? issues : [];
    const sprintsList = Array.isArray(sprints) ? sprints : [];
    const membersList = Array.isArray(members) ? members : [];

    const issuesByState: Record<string, number> = {};
    const tasksByStatus: Record<string, number> = {};
    let allTasks: Task[] = [];
    let totalPlannedSP = 0;
    let completedSP = 0;
    let totalPlannedHours = 0;
    let totalActualHours = 0;
    let completedTasksCount = 0;

    for (const iss of issuesList) {
      const state = iss.state || 'open';
      issuesByState[state] = (issuesByState[state] || 0) + 1;

      if (Array.isArray(iss.tasks)) {
        for (const t of iss.tasks) {
          allTasks.push(t);
          const stKey = t.status_name || t.status_id || 'unknown';
          tasksByStatus[stKey] = (tasksByStatus[stKey] || 0) + 1;

          const sp = Number(t.story_points) || 0;
          totalPlannedSP += sp;

          const prog = Number(t.progress) || 0;
          const isDone = prog >= 100 || (t.status_name && /完了|done|closed/i.test(t.status_name));
          if (isDone) {
            completedTasksCount++;
            completedSP += sp;
          }

          totalPlannedHours += Number(t.planned_hours) || 0;
          totalActualHours += Number(t.actual_hours) || 0;
        }
      }
    }

    const todayYMD = formatLocalYMD();
    const activeSprint = sprintsList.find((sp) => {
      if (sp.status === 'active') return true;
      if (sp.start_date && sp.end_date) {
        return sp.start_date <= todayYMD && sp.end_date >= todayYMD;
      }
      return false;
    }) || null;

    const taskCount = allTasks.length;
    const taskCompletionRate = taskCount > 0 ? Math.round((completedTasksCount / taskCount) * 100) : 0;

    return {
      project,
      members_count: membersList.length,
      issues_count: issuesList.length,
      tasks_count: taskCount,
      completed_tasks_count: completedTasksCount,
      task_completion_rate: taskCompletionRate,
      total_story_points: Math.round(totalPlannedSP * 100) / 100,
      completed_story_points: Math.round(completedSP * 100) / 100,
      total_planned_hours: Math.round(totalPlannedHours * 100) / 100,
      total_actual_hours: Math.round(totalActualHours * 100) / 100,
      sprints_count: sprintsList.length,
      active_sprint: activeSprint,
      issues_by_state: issuesByState,
      tasks_by_status: tasksByStatus,
    };
  }

  // ==========================================
  // 2. Issues
  // ==========================================

  public async listIssues(
    projectId: string,
    params?: { status_id?: string; type_id?: string; search?: string; sprint_id?: string }
  ): Promise<Issue[]> {
    const pId = encodeURIComponent(projectId);
    const searchParams = new URLSearchParams();
    if (params?.status_id) searchParams.set('status_id', params.status_id);
    if (params?.type_id) searchParams.set('type_id', params.type_id);
    if (params?.search) searchParams.set('search', params.search);
    if (params?.sprint_id) searchParams.set('sprint_id', params.sprint_id);

    const query = searchParams.toString();
    const endpoint = `/projects/${pId}/issues${query ? `?${query}` : ''}`;
    const issues = await this.request<Issue[]>(endpoint);
    return Array.isArray(issues) ? issues : [];
  }

  public async getIssue(projectId: string, issueId: string): Promise<Issue> {
    return this.request<Issue>(`/projects/${encodeURIComponent(projectId)}/issues/${encodeURIComponent(issueId)}`);
  }

  public async createIssue(projectId: string, data: IssueCreateInput): Promise<Issue> {
    return this.request<Issue>(`/projects/${encodeURIComponent(projectId)}/issues`, {
      method: 'POST',
      body: JSON.stringify(data),
    });
  }

  public async updateIssue(projectId: string, issueId: string, data: IssueUpdateInput): Promise<Issue> {
    return this.request<Issue>(`/projects/${encodeURIComponent(projectId)}/issues/${encodeURIComponent(issueId)}`, {
      method: 'PUT',
      body: JSON.stringify(data),
    });
  }

  // ==========================================
  // 3. Tasks
  // ==========================================

  public async listTasks(
    projectId: string,
    params?: { issue_id?: string; status_id?: string; sprint_id?: string }
  ): Promise<Task[]> {
    const issues = await this.listIssues(projectId);
    let allTasks: Task[] = [];
    for (const iss of issues) {
      if (params?.issue_id && iss.id !== params.issue_id) {
        continue;
      }
      if (Array.isArray(iss.tasks)) {
        for (const t of iss.tasks) {
          if (params?.status_id && t.status_id !== params.status_id) {
            continue;
          }
          if (params?.sprint_id && t.sprint_id !== params.sprint_id) {
            continue;
          }
          allTasks.push({
            ...t,
            issue_title: iss.title,
            issue_key_number: iss.key_number,
          });
        }
      }
    }
    return allTasks;
  }

  public async getTask(projectId: string, taskId: string): Promise<Task> {
    return this.request<Task>(`/projects/${encodeURIComponent(projectId)}/tasks/${encodeURIComponent(taskId)}`);
  }

  public async createTask(projectId: string, data: TaskCreateInput): Promise<Task> {
    return this.request<Task>(`/projects/${encodeURIComponent(projectId)}/tasks`, {
      method: 'POST',
      body: JSON.stringify(data),
    });
  }

  public async updateTask(projectId: string, taskId: string, data: TaskUpdateInput): Promise<Task> {
    return this.request<Task>(`/projects/${encodeURIComponent(projectId)}/tasks/${encodeURIComponent(taskId)}`, {
      method: 'PUT',
      body: JSON.stringify(data),
    });
  }

  // ==========================================
  // 4. Sprints
  // ==========================================

  public async listSprints(projectId: string): Promise<Sprint[]> {
    const sprints = await this.request<Sprint[]>(`/projects/${encodeURIComponent(projectId)}/sprints`);
    return Array.isArray(sprints) ? sprints : [];
  }

  public async getSprint(sprintId: string): Promise<Sprint> {
    return this.request<Sprint>(`/sprints/${encodeURIComponent(sprintId)}`);
  }

  public async getSprintBurndown(sprintId: string): Promise<any> {
    return this.request<any>(`/sprints/${encodeURIComponent(sprintId)}/burndown`);
  }

  public async createSprint(projectId: string, data: SprintCreateInput): Promise<Sprint> {
    let { name, start_date, end_date, duration_days, goal } = data;

    // Auto-calculate start_date, end_date, and name if omitted
    if (!start_date || !end_date || !name) {
      const sprints = await this.listSprints(projectId);
      const duration = Math.max(1, Math.min(90, Math.round(Number(duration_days) || 14)));

      if (!start_date || !end_date) {
        let maxEndDate = '';
        for (const sp of sprints) {
          if (sp && sp.end_date && /^\d{4}-\d{2}-\d{2}$/.test(sp.end_date)) {
            if (!maxEndDate || sp.end_date > maxEndDate) {
              maxEndDate = sp.end_date;
            }
          }
        }

        if (!start_date) {
          start_date = maxEndDate ? addDaysYMD(maxEndDate, 1) : formatLocalYMD();
        }
        if (!end_date) {
          end_date = addDaysYMD(start_date, duration - 1);
        }
      }

      if (!name) {
        let maxNum = 0;
        for (const sp of sprints) {
          if (sp && typeof sp.name === 'string') {
            const m = sp.name.match(/(?:Sprint|スプリント)\s*#?(\d+)/i);
            if (m) {
              const num = parseInt(m[1], 10);
              if (num > maxNum) maxNum = num;
            }
          }
        }
        name = `Sprint ${Math.max(sprints.length + 1, maxNum + 1)}`;
      }
    }

    return this.request<Sprint>(`/projects/${encodeURIComponent(projectId)}/sprints`, {
      method: 'POST',
      body: JSON.stringify({
        name,
        start_date,
        end_date,
        goal: goal || '',
      }),
    });
  }

  // ==========================================
  // 5. Wiki
  // ==========================================

  public async listWikiPages(projectId: string): Promise<WikiPage[]> {
    const pages = await this.request<WikiPage[]>(`/projects/${encodeURIComponent(projectId)}/wiki`);
    return Array.isArray(pages) ? pages : [];
  }

  public async getWikiPage(wikiId: string): Promise<WikiPage> {
    return this.request<WikiPage>(`/wiki/${encodeURIComponent(wikiId)}`);
  }

  public async saveWikiPage(projectId: string, data: WikiPageSaveInput): Promise<WikiPage> {
    if (data.wiki_id) {
      return this.request<WikiPage>(`/wiki/${encodeURIComponent(data.wiki_id)}`, {
        method: 'PUT',
        body: JSON.stringify({
          title: data.title,
          content: data.content,
          parent_id: data.parent_id !== undefined ? data.parent_id : null,
        }),
      });
    }

    return this.request<WikiPage>(`/projects/${encodeURIComponent(projectId)}/wiki`, {
      method: 'POST',
      body: JSON.stringify({
        title: data.title,
        content: data.content,
        parent_id: data.parent_id || null,
      }),
    });
  }
}
