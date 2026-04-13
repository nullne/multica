-- Dispatch hints: optional constraints on which provider/daemon to use for agent tasks.
ALTER TABLE issue ADD COLUMN dispatch_provider TEXT;
ALTER TABLE issue ADD COLUMN dispatch_daemon_id UUID REFERENCES daemon(id) ON DELETE SET NULL;
ALTER TABLE issue ADD COLUMN dispatch_daemon_label TEXT;

-- Daemon labels: user-managed tags for categorizing daemons (e.g. "gpu", "staging").
ALTER TABLE daemon ADD COLUMN labels TEXT[] NOT NULL DEFAULT '{}';
