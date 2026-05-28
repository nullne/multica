ALTER TABLE routine
    DROP COLUMN IF EXISTS github_auto_fix_enabled;

ALTER TABLE issue
    DROP COLUMN IF EXISTS github_auto_fix_enabled;
