-- Revert source_type 'standard' -> 'custom'
UPDATE webhook SET source_type = 'custom' WHERE source_type = 'standard';

ALTER TABLE webhook DROP CONSTRAINT IF EXISTS webhook_source_type_check;
ALTER TABLE webhook ADD CONSTRAINT webhook_source_type_check
    CHECK (source_type IN ('custom', 'oss-alert'));
