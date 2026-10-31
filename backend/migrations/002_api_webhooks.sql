-- Polaris API Integration (Personal Access Tokens) & Outbound Webhooks Schema

CREATE TABLE IF NOT EXISTS personal_access_tokens (
    id UUID PRIMARY KEY,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name VARCHAR(100) NOT NULL,
    token_hash VARCHAR(64) NOT NULL UNIQUE,
    token_prefix VARCHAR(16) NOT NULL,
    scopes JSONB NOT NULL DEFAULT '[]'::jsonb,
    expires_at TIMESTAMPTZ,
    last_used_at TIMESTAMPTZ,
    revoked_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_pat_token_hash ON personal_access_tokens(token_hash);
CREATE INDEX IF NOT EXISTS idx_pat_user_id ON personal_access_tokens(user_id);

CREATE TABLE IF NOT EXISTS webhooks (
    id UUID PRIMARY KEY,
    project_id UUID REFERENCES projects(id) ON DELETE CASCADE,
    name VARCHAR(100) NOT NULL,
    url TEXT NOT NULL,
    secret VARCHAR(128) NOT NULL,
    events JSONB NOT NULL DEFAULT '[]'::jsonb,
    is_active BOOLEAN NOT NULL DEFAULT true,
    created_by UUID NOT NULL REFERENCES users(id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_webhooks_project_id ON webhooks(project_id);

CREATE TABLE IF NOT EXISTS webhook_deliveries (
    id UUID PRIMARY KEY,
    webhook_id UUID NOT NULL REFERENCES webhooks(id) ON DELETE CASCADE,
    event VARCHAR(64) NOT NULL,
    status_code INT,
    duration_ms INT NOT NULL DEFAULT 0,
    request_body JSONB,
    response_body TEXT,
    error_message TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_webhook_deliveries_webhook_id ON webhook_deliveries(webhook_id);

