ALTER TABLE recurring_issue_template
    ADD COLUMN max_runs              INTEGER
        CHECK (max_runs IS NULL OR max_runs > 0),
    ADD COLUMN successful_runs_count INTEGER NOT NULL DEFAULT 0
        CHECK (successful_runs_count >= 0);

DROP INDEX IF EXISTS idx_recurring_template_due;
CREATE INDEX idx_recurring_template_due ON recurring_issue_template(next_run_at)
    WHERE enabled = TRUE
      AND next_run_at IS NOT NULL
      AND (max_runs IS NULL OR successful_runs_count < max_runs);
