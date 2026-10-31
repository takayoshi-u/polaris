-- Polaris Issue and Task Types Migration

CREATE TABLE IF NOT EXISTS types (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id UUID REFERENCES projects(id) ON DELETE CASCADE,
    name VARCHAR(255) NOT NULL,
    color VARCHAR(32) NOT NULL DEFAULT '#6366f1',
    display_order INT NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_types_project_id ON types(project_id);

ALTER TABLE issues ADD COLUMN IF NOT EXISTS type_id UUID REFERENCES types(id) ON DELETE SET NULL;
ALTER TABLE tasks ADD COLUMN IF NOT EXISTS type_id UUID REFERENCES types(id) ON DELETE SET NULL;

-- Initial system-wide default types (project_id IS NULL)
INSERT INTO types (id, project_id, name, color, display_order)
SELECT gen_random_uuid(), NULL, 'タスク', '#3b82f6', 1
WHERE NOT EXISTS (SELECT 1 FROM types WHERE project_id IS NULL AND name = 'タスク');

INSERT INTO types (id, project_id, name, color, display_order)
SELECT gen_random_uuid(), NULL, 'バグ', '#ef4444', 2
WHERE NOT EXISTS (SELECT 1 FROM types WHERE project_id IS NULL AND name = 'バグ');

INSERT INTO types (id, project_id, name, color, display_order)
SELECT gen_random_uuid(), NULL, '機能要望', '#10b981', 3
WHERE NOT EXISTS (SELECT 1 FROM types WHERE project_id IS NULL AND name = '機能要望');

INSERT INTO types (id, project_id, name, color, display_order)
SELECT gen_random_uuid(), NULL, '改善', '#f59e0b', 4
WHERE NOT EXISTS (SELECT 1 FROM types WHERE project_id IS NULL AND name = '改善');
