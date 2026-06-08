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

-- name: ListBotsInWorkspace :many
-- Lists bot users in a workspace. Bots are used as the author for
-- system-driven comments (e.g. routine comment_issue actions).
SELECT u.* FROM "user" u
JOIN member m ON m.user_id = u.id
WHERE m.workspace_id = $1 AND u.kind = 'bot'
ORDER BY u.created_at ASC;
