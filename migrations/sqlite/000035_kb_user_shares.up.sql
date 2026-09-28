CREATE TABLE IF NOT EXISTS kb_user_shares (
    id TEXT PRIMARY KEY,
    knowledge_base_id TEXT NOT NULL,
    target_user_id TEXT NOT NULL,
    shared_by_user_id TEXT NOT NULL,
    source_tenant_id INTEGER NOT NULL,
    permission TEXT NOT NULL DEFAULT 'viewer',
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    deleted_at DATETIME
);

CREATE INDEX IF NOT EXISTS idx_kb_user_shares_kb
    ON kb_user_shares (knowledge_base_id);
CREATE INDEX IF NOT EXISTS idx_kb_user_shares_target
    ON kb_user_shares (target_user_id);
CREATE INDEX IF NOT EXISTS idx_kb_user_shares_source_tenant
    ON kb_user_shares (source_tenant_id);
CREATE UNIQUE INDEX IF NOT EXISTS idx_kb_user_shares_active_unique
    ON kb_user_shares (knowledge_base_id, target_user_id)
    WHERE deleted_at IS NULL;
