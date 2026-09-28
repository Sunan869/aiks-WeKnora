CREATE TABLE IF NOT EXISTS kb_user_shares (
    id VARCHAR(36) PRIMARY KEY,
    knowledge_base_id VARCHAR(36) NOT NULL,
    target_user_id VARCHAR(36) NOT NULL,
    shared_by_user_id VARCHAR(36) NOT NULL,
    source_tenant_id BIGINT NOT NULL,
    permission VARCHAR(32) NOT NULL DEFAULT 'viewer',
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP WITH TIME ZONE
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
