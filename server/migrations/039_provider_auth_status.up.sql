ALTER TABLE agent_runtime
    ADD COLUMN auth_status TEXT NOT NULL DEFAULT 'unknown'
        CHECK (auth_status IN ('unknown', 'not_installed', 'unauthenticated', 'ready'));
