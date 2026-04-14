-- Rename source_type 'custom' -> 'standard'
UPDATE webhook SET source_type = 'standard' WHERE source_type = 'custom';

ALTER TABLE webhook DROP CONSTRAINT IF EXISTS webhook_source_type_check;
ALTER TABLE webhook ADD CONSTRAINT webhook_source_type_check
    CHECK (source_type IN ('standard', 'oss-alert'));
