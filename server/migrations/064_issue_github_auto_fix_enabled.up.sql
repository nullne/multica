ALTER TABLE issue
    ADD COLUMN github_auto_fix_enabled BOOLEAN NOT NULL DEFAULT FALSE;

ALTER TABLE routine
    ADD COLUMN github_auto_fix_enabled BOOLEAN NOT NULL DEFAULT FALSE;
