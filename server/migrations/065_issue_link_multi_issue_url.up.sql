ALTER TABLE issue_link
    DROP CONSTRAINT IF EXISTS issue_link_workspace_id_url_key;

ALTER TABLE issue_link
    ADD CONSTRAINT issue_link_issue_id_url_key UNIQUE (issue_id, url);
