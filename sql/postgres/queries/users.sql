-- name: GetUserByID :one
SELECT * FROM users
WHERE id = $1 LIMIT 1;

-- name: GetUserByUsername :one
SELECT * FROM users
WHERE username = $1 LIMIT 1;

-- name: GetUserByUID :one
SELECT * FROM users
WHERE id = $1 LIMIT 1;

-- name: GetUsers :many
SELECT * FROM users;

-- name: CreateUser :one
INSERT INTO users (
		username,
		password,
		email,
		is_admin
	) VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: DeleteUser :exec
DELETE FROM users WHERE username = $1;

-- name: UpdatePassword :exec
UPDATE users SET password = $2 WHERE username = $1;

-- name: CreateAPIKey :one
INSERT INTO api_keys(
	owner,
	created_at,
	expires,
	disabled,
	api_key
	) VALUES ($1, NOW(), $2, $3, gen_random_uuid())
RETURNING *;

-- name: DeleteAPIKey :exec
DELETE FROM api_keys WHERE api_key = $1;

-- name: GetAPIKey :one
SELECT * FROM api_keys WHERE api_key = $1;

-- name: GetAppPrefs :one
SELECT (prefs->>(@app_name::TEXT))::JSONB FROM users WHERE id = @uid;

-- name: SetAppPrefs :exec
UPDATE users SET prefs = COALESCE(prefs, '{}'::JSONB) || jsonb_build_object(@app_name::TEXT, @prefs::JSONB) WHERE id = @uid;
