ALTER TABLE workspace ADD COLUMN github_installation_id BIGINT DEFAULT NULL;

ALTER TABLE agent ADD COLUMN github_code_access TEXT NOT NULL DEFAULT 'write'
    CHECK (github_code_access IN ('read', 'write', 'admin'));
