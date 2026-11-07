import test from 'node:test';
import assert from 'node:assert/strict';
import { PolarisClient, formatLocalYMD, addDaysYMD } from '../dist/client.js';
import { createPolarisMcpServer } from '../dist/server.js';

test('PolarisClient: sets Authorization Bearer token header', async () => {
  let capturedUrl = '';
  let capturedHeaders = {};

  const mockFetch = async (url, options) => {
    capturedUrl = String(url);
    capturedHeaders = options.headers;
    return {
      status: 200,
      ok: true,
      headers: new Headers({ 'content-type': 'application/json' }),
      json: async () => [{ id: 'proj-1', name: 'Project 1', key: 'P1' }],
    };
  };

  const client = new PolarisClient({
    baseUrl: 'http://test-polaris.local/api/v1',
    apiToken: 'pat_test_token_123',
    fetchFn: mockFetch,
  });

  const projects = await client.listProjects();
  assert.equal(projects.length, 1);
  assert.equal(projects[0].key, 'P1');
  assert.equal(capturedUrl, 'http://test-polaris.local/api/v1/projects');
  assert.equal(capturedHeaders['Authorization'], 'Bearer pat_test_token_123');
});

test('PolarisClient: auto-logs in with email and password when apiToken is not provided', async () => {
  const requests = [];

  const mockFetch = async (url, options) => {
    requests.push({ url: String(url), method: options.method || 'GET', headers: options.headers });
    if (url.includes('/auth/login')) {
      return {
        status: 200,
        ok: true,
        headers: new Headers({ 'content-type': 'application/json' }),
        json: async () => ({ session_token: 'session_token_xyz', user: { id: 'u1' } }),
      };
    }
    return {
      status: 200,
      ok: true,
      headers: new Headers({ 'content-type': 'application/json' }),
      json: async () => ({ id: 'proj-1', name: 'Project 1' }),
    };
  };

  const client = new PolarisClient({
    baseUrl: 'http://test-polaris.local/api/v1',
    email: 'admin@polaris.local',
    password: 'password123',
    fetchFn: mockFetch,
  });

  const proj = await client.getProject('proj-1');
  assert.equal(proj.id, 'proj-1');
  assert.equal(requests.length, 2);
  assert.equal(requests[0].url, 'http://test-polaris.local/api/v1/auth/login');
  assert.equal(requests[1].url, 'http://test-polaris.local/api/v1/projects/proj-1');
  assert.equal(requests[1].headers['Authorization'], 'Bearer session_token_xyz');
});

test('PolarisClient: handles API errors gracefully', async () => {
  const mockFetch = async () => ({
    status: 404,
    ok: false,
    statusText: 'Not Found',
    headers: new Headers({ 'content-type': 'application/json' }),
    json: async () => ({ error: 'project not found' }),
  });

  const client = new PolarisClient({
    baseUrl: 'http://test-polaris.local/api/v1',
    apiToken: 'test-token',
    fetchFn: mockFetch,
  });

  await assert.rejects(
    async () => {
      await client.getProject('non-existent');
    },
    (err) => {
      assert.match(err.message, /Polaris API Error \(404\): project not found/);
      return true;
    }
  );
});

test('PolarisClient: getProjectSummary calculates metrics accurately', async () => {
  const mockFetch = async (url) => {
    const sUrl = String(url);
    if (sUrl.includes('/projects/p1/issues')) {
      return {
        status: 200,
        ok: true,
        headers: new Headers({ 'content-type': 'application/json' }),
        json: async () => [
          {
            id: 'iss-1',
            title: 'Issue 1',
            state: 'open',
            tasks: [
              { id: 't1', title: 'Task 1', status_name: '完了', progress: 100, story_points: 3, planned_hours: 4, actual_hours: 3.5 },
              { id: 't2', title: 'Task 2', status_name: '進行中', progress: 50, story_points: 5, planned_hours: 8, actual_hours: 2 },
            ],
          },
          {
            id: 'iss-2',
            title: 'Issue 2',
            state: 'closed',
            tasks: [],
          },
        ],
      };
    }
    if (sUrl.includes('/projects/p1/sprints')) {
      return {
        status: 200,
        ok: true,
        headers: new Headers({ 'content-type': 'application/json' }),
        json: async () => [
          { id: 'sp-1', name: 'Sprint 1', start_date: '2026-01-01', end_date: '2026-01-14', status: 'closed' },
          { id: 'sp-2', name: 'Sprint 2', start_date: '2026-01-15', end_date: '2099-01-28', status: 'active' },
        ],
      };
    }
    if (sUrl.includes('/projects/p1/members')) {
      return {
        status: 200,
        ok: true,
        headers: new Headers({ 'content-type': 'application/json' }),
        json: async () => [{ user_id: 'u1' }, { user_id: 'u2' }],
      };
    }
    return {
      status: 200,
      ok: true,
      headers: new Headers({ 'content-type': 'application/json' }),
      json: async () => ({ id: 'p1', name: 'Alpha Project', key: 'ALP' }),
    };
  };

  const client = new PolarisClient({
    baseUrl: 'http://test-polaris.local/api/v1',
    apiToken: 'token',
    fetchFn: mockFetch,
  });

  const summary = await client.getProjectSummary('p1');
  assert.equal(summary.project.name, 'Alpha Project');
  assert.equal(summary.members_count, 2);
  assert.equal(summary.issues_count, 2);
  assert.equal(summary.tasks_count, 2);
  assert.equal(summary.completed_tasks_count, 1);
  assert.equal(summary.task_completion_rate, 50);
  assert.equal(summary.total_story_points, 8);
  assert.equal(summary.completed_story_points, 3);
  assert.equal(summary.total_planned_hours, 12);
  assert.equal(summary.total_actual_hours, 5.5);
  assert.equal(summary.sprints_count, 2);
  assert.equal(summary.active_sprint?.id, 'sp-2');
  assert.equal(summary.issues_by_state['open'], 1);
  assert.equal(summary.issues_by_state['closed'], 1);
});

test('PolarisClient: createSprint auto-calculates dates and name if omitted', async () => {
  let createdPayload = null;

  const mockFetch = async (url, options) => {
    const sUrl = String(url);
    if (options?.method === 'POST') {
      createdPayload = JSON.parse(options.body);
      return {
        status: 201,
        ok: true,
        headers: new Headers({ 'content-type': 'application/json' }),
        json: async () => ({ id: 'new-sp', ...createdPayload }),
      };
    }
    // GET /sprints
    return {
      status: 200,
      ok: true,
      headers: new Headers({ 'content-type': 'application/json' }),
      json: async () => [
        { id: 'sp-1', name: 'Sprint 1', start_date: '2026-05-01', end_date: '2026-05-14' },
        { id: 'sp-2', name: 'Sprint 2', start_date: '2026-05-15', end_date: '2026-05-28' },
      ],
    };
  };

  const client = new PolarisClient({
    baseUrl: 'http://test-polaris.local/api/v1',
    apiToken: 'token',
    fetchFn: mockFetch,
  });

  const created = await client.createSprint('proj-1', { duration_days: 14 });
  assert.equal(createdPayload.name, 'Sprint 3');
  assert.equal(createdPayload.start_date, '2026-05-29'); // Day after Sprint 2 end_date
  assert.equal(createdPayload.end_date, '2026-06-11'); // 14 days later inclusive
});

test('PolarisClient: saveWikiPage creates new page when wiki_id is omitted and updates when provided', async () => {
  const operations = [];

  const mockFetch = async (url, options) => {
    operations.push({ url: String(url), method: options.method, body: JSON.parse(options.body) });
    return {
      status: 200,
      ok: true,
      headers: new Headers({ 'content-type': 'application/json' }),
      json: async () => ({ id: 'w1', ...JSON.parse(options.body) }),
    };
  };

  const client = new PolarisClient({
    baseUrl: 'http://test-polaris.local/api/v1',
    apiToken: 'token',
    fetchFn: mockFetch,
  });

  // Create
  await client.saveWikiPage('p1', { title: 'Welcome', content: '# Welcome' });
  assert.equal(operations[0].method, 'POST');
  assert.equal(operations[0].url, 'http://test-polaris.local/api/v1/projects/p1/wiki');
  assert.equal(operations[0].body.title, 'Welcome');

  // Update
  await client.saveWikiPage('p1', { wiki_id: 'w1', title: 'Welcome Updated', content: '# Welcome V2' });
  assert.equal(operations[1].method, 'PUT');
  assert.equal(operations[1].url, 'http://test-polaris.local/api/v1/wiki/w1');
  assert.equal(operations[1].body.title, 'Welcome Updated');
});

test('MCP Server: registers all expected tools in schema', () => {
  const client = new PolarisClient({ baseUrl: 'http://localhost:8080/api/v1', apiToken: 'dummy' });
  const server = createPolarisMcpServer(client);

  const registeredToolNames = Object.keys(server._registeredTools);

  const expectedTools = [
    // Projects
    'list_projects',
    'get_project_summary',
    // Issues
    'list_issues',
    'get_issue',
    'create_issue',
    'update_issue',
    // Tasks
    'list_tasks',
    'get_task',
    'create_task',
    'update_task',
    // Sprints
    'list_sprints',
    'get_sprint_burndown',
    'create_sprint',
    // Wiki
    'list_wiki_pages',
    'get_wiki_page',
    'save_wiki_page',
  ];

  for (const toolName of expectedTools) {
    assert.ok(
      registeredToolNames.includes(toolName),
      `Expected tool '${toolName}' to be registered on McpServer, but found: ${registeredToolNames.join(', ')}`
    );
  }
  assert.equal(registeredToolNames.length, 16);
});

test('MCP Server: tool handlers invoke client and return formatted responses', async () => {
  let createdTaskData = null;

  const mockFetch = async (url, options) => {
    if (options?.method === 'POST' && String(url).includes('/tasks')) {
      createdTaskData = JSON.parse(options.body);
      return {
        status: 201,
        ok: true,
        headers: new Headers({ 'content-type': 'application/json' }),
        json: async () => ({ id: 'task-100', ...createdTaskData }),
      };
    }
    return {
      status: 200,
      ok: true,
      headers: new Headers({ 'content-type': 'application/json' }),
      json: async () => [],
    };
  };

  const client = new PolarisClient({
    baseUrl: 'http://test-polaris.local/api/v1',
    apiToken: 'valid-token',
    fetchFn: mockFetch,
  });

  const server = createPolarisMcpServer(client);
  const createTaskTool = server._registeredTools['create_task'];

  const res = await createTaskTool.handler({
    projectId: 'p1',
    issue_id: 'iss-1',
    title: 'Implement OAuth',
    story_points: 5,
    planned_hours: 8,
    actual_hours: 2,
    progress: 25,
  });

  assert.ok(res.content && res.content[0]);
  const parsed = JSON.parse(res.content[0].text);
  assert.equal(parsed.id, 'task-100');
  assert.equal(parsed.title, 'Implement OAuth');
  assert.equal(createdTaskData.story_points, 5);
  assert.equal(createdTaskData.progress, 25);
  assert.equal(createdTaskData.actual_hours, 2);
});

test('MCP Server: tool handles errors and returns formatted error object', async () => {
  const mockFetch = async () => ({
    status: 500,
    ok: false,
    statusText: 'Internal Server Error',
    headers: new Headers({ 'content-type': 'application/json' }),
    json: async () => ({ error: 'database connection failed' }),
  });

  const client = new PolarisClient({
    baseUrl: 'http://test-polaris.local/api/v1',
    apiToken: 'valid-token',
    fetchFn: mockFetch,
  });

  const server = createPolarisMcpServer(client);
  const getTaskTool = server._registeredTools['get_task'];

  const res = await getTaskTool.handler({
    projectId: 'p1',
    taskId: 'task-999',
  });

  assert.equal(res.isError, true);
  assert.match(res.content[0].text, /Error \[get_task\]: Polaris API Error \(500\): database connection failed/);
});
