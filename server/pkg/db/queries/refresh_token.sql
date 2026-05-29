-- name: CreateRefreshToken :one
INSERT INTO refresh_token (user_id, token_hash, expires_at)
VALUES ($1, $2, $3)
RETURNING *;

-- name: GetRefreshTokenByHash :one
SELECT * FROM refresh_token
WHERE token_hash = $1
  AND revoked = FALSE
  AND expires_at > now();

-- name: RevokeRefreshToken :exec
UPDATE refresh_token
SET revoked = TRUE
WHERE token_hash = $1;

-- name: RevokeRefreshTokensByUser :exec
UPDATE refresh_token
SET revoked = TRUE
WHERE user_id = $1 AND revoked = FALSE;
