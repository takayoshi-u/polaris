/**
 * Polaris Model Context Protocol (MCP) Server Implementation
 */

import { McpServer } from '@modelcontextprotocol/sdk/server/mcp.js';
import { z } from 'zod';
import { PolarisClient } from './client.js';

export function createPolarisMcpServer(client?: PolarisClient): McpServer {
  const polaris = client || new PolarisClient();
  const server = new McpServer({
    name: 'polaris-mcp-server',
    version: '1.0.0',
  });

  const formatSuccess = (data: any) => ({
    content: [{ type: 'text' as const, text: typeof data === 'string' ? data : JSON.stringify(data, null, 2) }],
  });

  const formatError = (tool: string, error: any) => ({
    isError: true,
    content: [{ type: 'text' as const, text: `Error [${tool}]: ${error?.message || String(error)}` }],
  });

  // ==========================================
  // Projects Tools
  // ==========================================

  server.tool(
    'list_projects',
    'List all accessible Polaris projects',
    {},
    async () => {
      try {
        const projects = await polaris.listProjects();
        return formatSuccess(projects);
      } catch (err) {
        return formatError('list_projects', err);
      }
    }
  );

  server.tool(
    'get_project_summary',
    'Get high-level project summary including issue/task counts, completion rates, story points, hours, and active sprint',
    {
      projectId: z.string().describe('Project ID (UUID)'),
    },
    async ({ projectId }) => {
      try {
        const summary = await polaris.getProjectSummary(projectId);
        return formatSuccess(summary);
      } catch (err) {
        return formatError('get_project_summary', err);
      }
    }
  );

  // ==========================================
  // Issues Tools
  // ==========================================

  server.tool(
    'list_issues',
    'List issues in a project with optional filters',
    {
      projectId: z.string().describe('Project ID (UUID)'),
      status_id: z.string().optional().describe('Filter by status ID'),
      type_id: z.string().optional().describe('Filter by type ID'),
      search: z.string().optional().describe('Search keyword across title and description'),
      sprint_id: z.string().optional().describe('Filter by sprint ID'),
    },
    async ({ projectId, status_id, type_id, search, sprint_id }) => {
      try {
        const issues = await polaris.listIssues(projectId, { status_id, type_id, search, sprint_id });
        return formatSuccess(issues);
      } catch (err) {
        return formatError('list_issues', err);
      }
    }
  );

  server.tool(
    'get_issue',
    'Get detailed information for a specific issue including tasks and custom fields',
    {
      projectId: z.string().describe('Project ID (UUID)'),
      issueId: z.string().describe('Issue ID (UUID)'),
    },
    async ({ projectId, issueId }) => {
      try {
        const issue = await polaris.getIssue(projectId, issueId);
        return formatSuccess(issue);
      } catch (err) {
        return formatError('get_issue', err);
      }
    }
  );

  server.tool(
    'create_issue',
    'Create a new issue in a project',
    {
      projectId: z.string().describe('Project ID (UUID)'),
      title: z.string().describe('Issue title'),
      description: z.string().optional().describe('Markdown issue description'),
      type_id: z.string().optional().describe('Issue type ID'),
      template_id: z.string().optional().describe('Template ID'),
      premortem: z.string().optional().describe('Pre-mortem risk assessment'),
      postmortem: z.string().optional().describe('Post-mortem retrospective'),
      label_ids: z.array(z.string()).optional().describe('List of label IDs to attach'),
      create_initial_task: z.boolean().optional().describe('Whether to automatically generate an initial task'),
    },
    async ({ projectId, ...data }) => {
      try {
        const issue = await polaris.createIssue(projectId, data);
        return formatSuccess(issue);
      } catch (err) {
        return formatError('create_issue', err);
      }
    }
  );

  server.tool(
    'update_issue',
    'Update an existing issue',
    {
      projectId: z.string().describe('Project ID (UUID)'),
      issueId: z.string().describe('Issue ID (UUID)'),
      title: z.string().optional().describe('Updated issue title'),
      description: z.string().optional().describe('Updated markdown description'),
      state: z.string().optional().describe('Issue state (open, closed, etc.)'),
      type_id: z.string().nullable().optional().describe('Issue type ID (pass null to clear)'),
      template_id: z.string().optional().describe('Template ID'),
      premortem: z.string().optional().describe('Pre-mortem risk assessment'),
      postmortem: z.string().optional().describe('Post-mortem retrospective'),
      label_ids: z.array(z.string()).optional().describe('List of label IDs to attach'),
    },
    async ({ projectId, issueId, ...data }) => {
      try {
        const issue = await polaris.updateIssue(projectId, issueId, data);
        return formatSuccess(issue);
      } catch (err) {
        return formatError('update_issue', err);
      }
    }
  );

  // ==========================================
  // Tasks Tools
  // ==========================================

  server.tool(
    'list_tasks',
    'List tasks in a project with optional filtering by issue, status, or sprint',
    {
      projectId: z.string().describe('Project ID (UUID)'),
      issue_id: z.string().optional().describe('Filter by issue ID'),
      status_id: z.string().optional().describe('Filter by status ID'),
      sprint_id: z.string().optional().describe('Filter by sprint ID'),
    },
    async ({ projectId, issue_id, status_id, sprint_id }) => {
      try {
        const tasks = await polaris.listTasks(projectId, { issue_id, status_id, sprint_id });
        return formatSuccess(tasks);
      } catch (err) {
        return formatError('list_tasks', err);
      }
    }
  );

  server.tool(
    'get_task',
    'Get details for a specific task',
    {
      projectId: z.string().describe('Project ID (UUID)'),
      taskId: z.string().describe('Task ID (UUID)'),
    },
    async ({ projectId, taskId }) => {
      try {
        const task = await polaris.getTask(projectId, taskId);
        return formatSuccess(task);
      } catch (err) {
        return formatError('get_task', err);
      }
    }
  );

  server.tool(
    'create_task',
    'Create a new task under an issue',
    {
      projectId: z.string().describe('Project ID (UUID)'),
      issue_id: z.string().describe('Parent issue ID (UUID)'),
      title: z.string().describe('Task title'),
      description: z.string().optional().describe('Task description'),
      status_id: z.string().optional().describe('Initial status ID'),
      progress: z.number().min(0).max(100).optional().describe('Progress percentage (0-100)'),
      story_points: z.number().optional().describe('Story points estimate'),
      planned_hours: z.number().optional().describe('Planned hours estimate'),
      actual_hours: z.number().optional().describe('Actual hours spent'),
      sprint_id: z.string().optional().describe('Sprint ID to assign task to'),
      type_id: z.string().optional().describe('Task type ID'),
      planned_start_date: z.string().optional().describe('Planned start date (YYYY-MM-DD)'),
      planned_end_date: z.string().optional().describe('Planned end date (YYYY-MM-DD)'),
      assignees: z.array(z.string()).optional().describe('User IDs assigned to task'),
    },
    async ({ projectId, ...data }) => {
      try {
        const task = await polaris.createTask(projectId, data);
        return formatSuccess(task);
      } catch (err) {
        return formatError('create_task', err);
      }
    }
  );

  server.tool(
    'update_task',
    'Update task properties including status, progress, actual_hours, and story_points',
    {
      projectId: z.string().describe('Project ID (UUID)'),
      taskId: z.string().describe('Task ID (UUID)'),
      title: z.string().optional().describe('Updated task title'),
      description: z.string().optional().describe('Updated task description'),
      status_id: z.string().optional().describe('Updated status ID'),
      progress: z.number().min(0).max(100).optional().describe('Updated progress percentage (0-100)'),
      story_points: z.number().optional().describe('Updated story points'),
      planned_hours: z.number().optional().describe('Updated planned hours'),
      actual_hours: z.number().optional().describe('Updated actual hours spent'),
      sprint_id: z.string().nullable().optional().describe('Updated sprint ID (pass null to unassign)'),
      type_id: z.string().nullable().optional().describe('Updated task type ID (pass null to clear)'),
      planned_start_date: z.string().nullable().optional().describe('Planned start date (YYYY-MM-DD)'),
      planned_end_date: z.string().nullable().optional().describe('Planned end date (YYYY-MM-DD)'),
      assignees: z.array(z.string()).optional().describe('Updated assignee user IDs'),
    },
    async ({ projectId, taskId, ...data }) => {
      try {
        const task = await polaris.updateTask(projectId, taskId, data);
        return formatSuccess(task);
      } catch (err) {
        return formatError('update_task', err);
      }
    }
  );

  // ==========================================
  // Sprints Tools
  // ==========================================

  server.tool(
    'list_sprints',
    'List all sprints for a project',
    {
      projectId: z.string().describe('Project ID (UUID)'),
    },
    async ({ projectId }) => {
      try {
        const sprints = await polaris.listSprints(projectId);
        return formatSuccess(sprints);
      } catch (err) {
        return formatError('list_sprints', err);
      }
    }
  );

  server.tool(
    'get_sprint_burndown',
    'Get burndown chart datapoints and metrics for a sprint',
    {
      sprintId: z.string().describe('Sprint ID (UUID)'),
    },
    async ({ sprintId }) => {
      try {
        const burndown = await polaris.getSprintBurndown(sprintId);
        return formatSuccess(burndown);
      } catch (err) {
        return formatError('get_sprint_burndown', err);
      }
    }
  );

  server.tool(
    'create_sprint',
    'Create a new sprint with automatic date and name calculation support',
    {
      projectId: z.string().describe('Project ID (UUID)'),
      name: z.string().optional().describe("Sprint name (auto-generated if omitted, e.g. 'Sprint N')"),
      start_date: z.string().optional().describe('Start date (YYYY-MM-DD, defaults to day after latest sprint or today)'),
      end_date: z.string().optional().describe('End date (YYYY-MM-DD, defaults to start_date + duration_days - 1)'),
      duration_days: z.number().min(1).max(90).optional().describe('Sprint duration in days if auto-calculating (default 14)'),
      goal: z.string().optional().describe('Sprint goal description'),
    },
    async ({ projectId, ...data }) => {
      try {
        const sprint = await polaris.createSprint(projectId, data);
        return formatSuccess(sprint);
      } catch (err) {
        return formatError('create_sprint', err);
      }
    }
  );

  // ==========================================
  // Wiki Tools
  // ==========================================

  server.tool(
    'list_wiki_pages',
    'List all wiki documentation pages for a project',
    {
      projectId: z.string().describe('Project ID (UUID)'),
    },
    async ({ projectId }) => {
      try {
        const pages = await polaris.listWikiPages(projectId);
        return formatSuccess(pages);
      } catch (err) {
        return formatError('list_wiki_pages', err);
      }
    }
  );

  server.tool(
    'get_wiki_page',
    'Get content of a wiki page by its ID',
    {
      wikiId: z.string().describe('Wiki page ID (UUID)'),
    },
    async ({ wikiId }) => {
      try {
        const page = await polaris.getWikiPage(wikiId);
        return formatSuccess(page);
      } catch (err) {
        return formatError('get_wiki_page', err);
      }
    }
  );

  server.tool(
    'save_wiki_page',
    'Create or update a wiki page (create if wiki_id is omitted, update if wiki_id is provided)',
    {
      projectId: z.string().describe('Project ID (UUID)'),
      wiki_id: z.string().optional().describe('Wiki page ID to update (omit to create a new page)'),
      title: z.string().describe('Wiki page title'),
      content: z.string().describe('Markdown content for the wiki page'),
      parent_id: z.string().nullable().optional().describe('Parent page ID for nested document hierarchy'),
    },
    async ({ projectId, ...data }) => {
      try {
        const page = await polaris.saveWikiPage(projectId, data);
        return formatSuccess(page);
      } catch (err) {
        return formatError('save_wiki_page', err);
      }
    }
  );

  return server;
}
