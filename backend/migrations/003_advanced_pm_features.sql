-- Polaris Advanced Project Management Features Schema
-- Task Dependencies, Sprints, Story Points, Activity Logs, Project Wiki, and Inbound Webhooks

-- 1. Sprints
CREATE TABLE IF NOT EXISTS sprints (
    id UUID PRIMARY KEY,
    project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    name VARCHAR(255) NOT NULL,
    goal TEXT,
    start_date DATE,
    end_date DATE,
    status VARCHAR(20) NOT NULL DEFAULT 'planned',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_sprints_project_id ON sprints(project_id);
CREATE INDEX IF NOT EXISTS idx_sprints_status ON sprints(project_id, status);

-- 2. Tasks extension (sprint_id, story_points)
ALTER TABLE tasks ADD COLUMN IF NOT EXISTS sprint_id UUID REFERENCES sprints(id) ON DELETE SET NULL;
ALTER TABLE tasks ADD COLUMN IF NOT EXISTS story_points NUMERIC(6,2);
CREATE INDEX IF NOT EXISTS idx_tasks_sprint_id ON tasks(sprint_id);

-- 3. Task Dependencies (FS, SS, FF, SF)
CREATE TABLE IF NOT EXISTS task_dependencies (
    id UUID PRIMARY KEY,
    project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    predecessor_id UUID NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
    successor_id UUID NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
    dependency_type VARCHAR(10) NOT NULL DEFAULT 'FS',
    lag_days INT NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT uq_task_dep UNIQUE (predecessor_id, successor_id),
    CONSTRAINT chk_no_self_dep CHECK (predecessor_id != successor_id)
);
CREATE INDEX IF NOT EXISTS idx_task_deps_project_id ON task_dependencies(project_id);
CREATE INDEX IF NOT EXISTS idx_task_deps_pred ON task_dependencies(predecessor_id);
CREATE INDEX IF NOT EXISTS idx_task_deps_succ ON task_dependencies(successor_id);

-- 4. Activity Logs
CREATE TABLE IF NOT EXISTS activity_logs (
    id UUID PRIMARY KEY,
    project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    entity_type VARCHAR(50) NOT NULL,
    entity_id UUID NOT NULL,
    action VARCHAR(50) NOT NULL,
    details JSONB NOT NULL DEFAULT '{}'::jsonb,
    actor_id UUID REFERENCES users(id) ON DELETE SET NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_activity_logs_project_id ON activity_logs(project_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_activity_logs_entity ON activity_logs(entity_type, entity_id, created_at DESC);

-- 5. Wiki Pages
CREATE TABLE IF NOT EXISTS wiki_pages (
    id UUID PRIMARY KEY,
    project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    title VARCHAR(255) NOT NULL,
    slug VARCHAR(255) NOT NULL,
    content TEXT NOT NULL DEFAULT '',
    parent_id UUID REFERENCES wiki_pages(id) ON DELETE SET NULL,
    sort_order INT NOT NULL DEFAULT 0,
    created_by UUID REFERENCES users(id) ON DELETE SET NULL,
    updated_by UUID REFERENCES users(id) ON DELETE SET NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT uq_wiki_project_slug UNIQUE (project_id, slug)
);
CREATE INDEX IF NOT EXISTS idx_wiki_pages_project_id ON wiki_pages(project_id, sort_order ASC);
CREATE INDEX IF NOT EXISTS idx_wiki_pages_parent_id ON wiki_pages(parent_id);

-- 6. Inbound Webhooks
CREATE TABLE IF NOT EXISTS inbound_webhooks (
    id UUID PRIMARY KEY,
    project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    name VARCHAR(255) NOT NULL,
    token_hash VARCHAR(64) NOT NULL UNIQUE,
    secret VARCHAR(255) NOT NULL,
    default_issue_id UUID REFERENCES issues(id) ON DELETE SET NULL,
    default_status_id INT NOT NULL DEFAULT 1,
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_inbound_webhooks_token_hash ON inbound_webhooks(token_hash);
CREATE INDEX IF NOT EXISTS idx_inbound_webhooks_project_id ON inbound_webhooks(project_id);
