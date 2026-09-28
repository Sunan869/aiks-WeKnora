CREATE TABLE IF NOT EXISTS external_identities (
    provider VARCHAR(50) NOT NULL,
    subject VARCHAR(255) NOT NULL,
    user_id VARCHAR(36) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (provider, subject),
    CONSTRAINT fk_external_identities_user FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
);
CREATE INDEX IF NOT EXISTS idx_external_identities_user_id ON external_identities(user_id);
