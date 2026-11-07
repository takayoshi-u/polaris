# Polaris Model Context Protocol (MCP) Server

[![TypeScript](https://img.shields.io/badge/TypeScript-5.7-blue.svg)](https://www.typescriptlang.org/)
[![Model Context Protocol](https://img.shields.io/badge/MCP-1.1.0-purple.svg)](https://modelcontextprotocol.io/)
[![Node.js](https://img.shields.io/badge/Node.js-22+-green.svg)](https://nodejs.org/)
[![Docker](https://img.shields.io/badge/Docker-Ready-blue.svg)](https://www.docker.com/)

A standardized [Model Context Protocol (MCP)](https://modelcontextprotocol.io/) server for the **Polaris Project Management System**. It enables LLMs and AI assistants (Claude Desktop, Cursor, Antigravity, etc.) to query and operate on projects, issues, tasks, agile sprints, burndown charts, and wiki documentation.

---

## Features

- **Project Insights**: List accessible projects and compute instant project-wide summaries (SP totals, task completion percentages, hour estimates, active sprint status).
- **Issue Tracking**: Create, list, search, inspect, and update issues (with pre-mortem risk assessment and post-mortem retrospective fields).
- **Agile Task Management**: List, inspect, create, and update tasks (status, progress %, story points, planned/actual hours, assignees).
- **Sprint & Scrum Support**: Manage sprints, fetch real-time sprint burndown charts, and auto-calculate sprint dates and sequential names (`Sprint N`).
- **Project Wiki**: List, view, and save (create/update) markdown wiki pages.
- **Dual Authentication**: Authenticate via Personal Access Token (`POLARIS_API_TOKEN`) or credentials (`POLARIS_EMAIL` & `POLARIS_PASSWORD`).
- **Container Ready**: Lightweight, multi-stage Docker image built on `node:22-alpine` running as a non-root user.

---

## Environment Variables

| Variable | Required | Default | Description |
| :--- | :---: | :---: | :--- |
| `POLARIS_BASE_URL` | No | `http://localhost:8080/api/v1` | Base URL of the Polaris REST API. |
| `POLARIS_API_TOKEN` | Yes* | *(None)* | Personal Access Token (PAT) for Polaris API. Preferred for headless agents. |
| `POLARIS_EMAIL` | Yes* | *(None)* | Account email for automatic session login (used when `POLARIS_API_TOKEN` is not set). |
| `POLARIS_PASSWORD` | Yes* | *(None)* | Account password for automatic session login (used when `POLARIS_API_TOKEN` is not set). |

> *\* Either `POLARIS_API_TOKEN` or both `POLARIS_EMAIL` and `POLARIS_PASSWORD` must be provided.*

---

## Quick Start

### 1. Run with Docker (Recommended)

Build the container image:

```bash
cd mcp
docker build -t polaris-mcp:latest .
```

Run interactively over stdio:

```bash
docker run -i --rm \
  -e POLARIS_BASE_URL="http://host.docker.internal:8080/api/v1" \
  -e POLARIS_API_TOKEN="pat_your_personal_access_token_here" \
  polaris-mcp:latest
```

---

### 2. Run Directly with Node.js

Prerequisites: Node.js 22+ and npm.

```bash
cd mcp
npm install
npm run build
```

Set environment variables and launch the server:

```bash
export POLARIS_BASE_URL="http://localhost:8080/api/v1"
export POLARIS_API_TOKEN="pat_your_token_here"

npm start
# or: node dist/index.js
```

---

## Client Configurations

### Claude Desktop

Add the server to your `claude_desktop_config.json`:

- **macOS**: `~/Library/Application Support/Claude/claude_desktop_config.json`
- **Windows**: `%APPDATA%\Claude\claude_desktop_config.json`
- **Linux**: `~/.config/Claude/claude_desktop_config.json`

#### Option A: Using Docker

```json
{
  "mcpServers": {
    "polaris": {
      "command": "docker",
      "args": [
        "run",
        "-i",
        "--rm",
        "-e", "POLARIS_BASE_URL=http://host.docker.internal:8080/api/v1",
        "-e", "POLARIS_API_TOKEN=pat_your_token_here",
        "polaris-mcp:latest"
      ]
    }
  }
}
```

#### Option B: Using Node

```json
{
  "mcpServers": {
    "polaris": {
      "command": "node",
      "args": ["/path/to/polaris/mcp/dist/index.js"],
      "env": {
        "POLARIS_BASE_URL": "http://localhost:8080/api/v1",
        "POLARIS_API_TOKEN": "pat_your_token_here"
      }
    }
  }
}
```

---

### Cursor / Antigravity

In your project's `.cursor/mcp.json` or Antigravity MCP settings:

```json
{
  "mcpServers": {
    "polaris": {
      "command": "node",
      "args": ["/absolute/path/to/polaris/mcp/dist/index.js"],
      "env": {
        "POLARIS_BASE_URL": "http://localhost:8080/api/v1",
        "POLARIS_API_TOKEN": "pat_your_token_here"
      }
    }
  }
}
```

---

## Available MCP Tools

### Projects

| Tool Name | Parameters | Description |
| :--- | :--- | :--- |
| `list_projects` | *(None)* | List all accessible Polaris projects. |
| `get_project_summary` | `projectId` (UUID) | High-level summary of total issues, tasks, completion %, story points, hours, and active sprint. |

### Issues

| Tool Name | Parameters | Description |
| :--- | :--- | :--- |
| `list_issues` | `projectId` (UUID), `status_id?`, `type_id?`, `search?`, `sprint_id?` | List issues in a project with optional search filter. |
| `get_issue` | `projectId` (UUID), `issueId` (UUID) | Detailed issue information with custom fields and nested tasks. |
| `create_issue` | `projectId` (UUID), `title`, `description?`, `type_id?`, `template_id?`, `premortem?`, `postmortem?`, `label_ids?`, `create_initial_task?` | Create a new issue in a project. |
| `update_issue` | `projectId` (UUID), `issueId` (UUID), `title?`, `description?`, `state?`, `type_id?`, `premortem?`, `postmortem?`, `label_ids?` | Update issue attributes and status. |

### Tasks

| Tool Name | Parameters | Description |
| :--- | :--- | :--- |
| `list_tasks` | `projectId` (UUID), `issue_id?`, `status_id?`, `sprint_id?` | List all tasks across project issues with optional filters. |
| `get_task` | `projectId` (UUID), `taskId` (UUID) | Detailed task information. |
| `create_task` | `projectId` (UUID), `issue_id` (UUID), `title`, `description?`, `status_id?`, `progress?`, `story_points?`, `planned_hours?`, `actual_hours?`, `sprint_id?`, `type_id?`, `planned_start_date?`, `planned_end_date?`, `assignees?` | Create a new task under an issue. |
| `update_task` | `projectId` (UUID), `taskId` (UUID), `title?`, `description?`, `status_id?`, `progress?`, `story_points?`, `planned_hours?`, `actual_hours?`, `sprint_id?`, `type_id?`, `planned_start_date?`, `planned_end_date?`, `assignees?` | Update task progress %, hours, story points, or status. |

### Sprints

| Tool Name | Parameters | Description |
| :--- | :--- | :--- |
| `list_sprints` | `projectId` (UUID) | List all sprints in a project. |
| `get_sprint_burndown` | `sprintId` (UUID) | Retrieve burndown chart datapoints and velocity progress for a sprint. |
| `create_sprint` | `projectId` (UUID), `name?`, `start_date?`, `end_date?`, `duration_days?`, `goal?` | Create sprint with automatic name and date calculation when omitted. |

### Wiki

| Tool Name | Parameters | Description |
| :--- | :--- | :--- |
| `list_wiki_pages` | `projectId` (UUID) | List all wiki documentation pages for a project. |
| `get_wiki_page` | `wikiId` (UUID) | Get markdown content and metadata for a wiki page. |
| `save_wiki_page` | `projectId` (UUID), `wiki_id?`, `title`, `content`, `parent_id?` | Create (if `wiki_id` omitted) or update (if `wiki_id` supplied) a wiki documentation page. |

---

## Development & Testing

Run unit tests:

```bash
cd mcp
npm test
```

Build TypeScript:

```bash
cd mcp
npm run build
```

---

## License

MIT
