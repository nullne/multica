-- name: GetUser :one
SELECT * FROM "user"
WHERE id = $1;

-- name: GetUserByEmail :one
SELECT * FROM "user"
WHERE email = $1;

-- name: CreateUser :one
INSERT INTO "user" (name, email, avatar_url, kind)
VALUES ($1, $2, $3, COALESCE(sqlc.narg('kind'), 'human'))
RETURNING *;

-- name: UpdateUser :one
UPDATE "user" SET
    name = COALESCE($2, name),
    avatar_url = COALESCE($3, avatar_url),
    updated_at = now()
WHERE id = $1
RETURNING *;

-- name: DeleteUser :exec
DELETE FROM "user" WHERE id = $1;

-- name: ListBotsInWorkspaceWithUsage :many
-- Lists bot users in a workspace and the number of webhooks currently bound
-- to each bot, so the UI can warn before deletion.
SELECT u.id, u.name, u.email, u.avatar_url, u.created_at, u.updated_at, u.kind,
       COUNT(w.id)::int AS webhook_count
FROM "user" u
JOIN member m ON m.user_id = u.id
LEFT JOIN webhook w ON w.bot_user_id = u.id AND w.workspace_id = m.workspace_id
WHERE m.workspace_id = $1 AND u.kind = 'bot'
GROUP BY u.id
ORDER BY u.created_at ASC;
