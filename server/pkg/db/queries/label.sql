-- name: ListLabels :many
SELECT * FROM issue_label
WHERE workspace_id = $1
ORDER BY name;

-- name: GetLabel :one
SELECT * FROM issue_label
WHERE id = $1 AND workspace_id = $2;

-- name: CreateLabel :one
INSERT INTO issue_label (workspace_id, name, color)
VALUES ($1, $2, $3)
RETURNING *;

-- name: UpdateLabel :one
UPDATE issue_label SET
    name = $3,
    color = $4
WHERE id = $1 AND workspace_id = $2
RETURNING *;

-- name: DeleteLabel :exec
DELETE FROM issue_label
WHERE id = $1 AND workspace_id = $2;

-- name: ListLabelsForIssue :many
SELECT il.* FROM issue_label il
JOIN issue_to_label itl ON il.id = itl.label_id
WHERE itl.issue_id = $1
ORDER BY il.name;

-- name: AddLabelToIssue :exec
INSERT INTO issue_to_label (issue_id, label_id)
VALUES ($1, $2)
ON CONFLICT DO NOTHING;

-- name: RemoveLabelFromIssue :exec
DELETE FROM issue_to_label
WHERE issue_id = $1 AND label_id = $2;

-- name: RemoveAllLabelsFromIssue :exec
DELETE FROM issue_to_label
WHERE issue_id = $1;

-- name: ListLabelsForIssues :many
SELECT itl.issue_id, il.* FROM issue_label il
JOIN issue_to_label itl ON il.id = itl.label_id
WHERE itl.issue_id = ANY($1::uuid[])
ORDER BY il.name;
