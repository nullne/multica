ALTER TABLE issue DROP COLUMN IF EXISTS dispatch_provider;
ALTER TABLE issue DROP COLUMN IF EXISTS dispatch_daemon_id;
ALTER TABLE issue DROP COLUMN IF EXISTS dispatch_daemon_label;
ALTER TABLE daemon DROP COLUMN IF EXISTS labels;
