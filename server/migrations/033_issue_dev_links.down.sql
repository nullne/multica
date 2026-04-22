ALTER TABLE issue
DROP COLUMN IF EXISTS linked_pr_url,
DROP COLUMN IF EXISTS linked_branch;
