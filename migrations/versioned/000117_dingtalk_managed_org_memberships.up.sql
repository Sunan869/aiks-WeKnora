CREATE TABLE IF NOT EXISTS dingtalk_managed_org_memberships (
    id VARCHAR(36) PRIMARY KEY,
    user_id VARCHAR(36) NOT NULL,
    tenant_id BIGINT NOT NULL,
    organization_id VARCHAR(36) NOT NULL,
    corp_id VARCHAR(128) NOT NULL,
    department_ids TEXT NOT NULL,
    role VARCHAR(32) NOT NULL DEFAULT 'viewer',
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_dingtalk_managed_org_memberships_user_tenant_org
    ON dingtalk_managed_org_memberships (user_id, tenant_id, organization_id);

CREATE INDEX IF NOT EXISTS idx_dingtalk_managed_org_memberships_tenant_org
    ON dingtalk_managed_org_memberships (tenant_id, organization_id);
