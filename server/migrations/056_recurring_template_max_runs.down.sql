DROP INDEX IF EXISTS idx_recurring_template_due;
CREATE INDEX idx_recurring_template_due ON recurring_issue_template(next_run_at)
    WHERE enabled = TRUE AND next_run_at IS NOT NULL;

ALTER TABLE recurring_issue_template
    DROP COLUMN successful_runs_count,
    DROP COLUMN max_runs;
