ALTER TABLE issue_link
    DROP CONSTRAINT IF EXISTS issue_link_issue_id_url_key;

DELETE FROM issue_link a
USING issue_link b
WHERE a.workspace_id = b.workspace_id
  AND a.url = b.url
  AND a.created_at > b.created_at;

ALTER TABLE issue_link
    ADD CONSTRAINT issue_link_workspace_id_url_key UNIQUE (workspace_id, url);
