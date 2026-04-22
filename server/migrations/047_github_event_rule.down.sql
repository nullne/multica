DROP TABLE IF EXISTS github_event_rule;

ALTER TABLE issue DROP CONSTRAINT issue_creator_type_check;
ALTER TABLE issue ADD CONSTRAINT issue_creator_type_check CHECK (creator_type IN ('member', 'agent', 'webhook'));
