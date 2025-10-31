-- name: GetUserByID :one
SELECT * FROM users
WHERE id = $1;

-- name: GetUserByUsername :one
SELECT * FROM users
WHERE username = $1 AND disabled_at IS NULL;

-- name: GetUsers :many
SELECT * FROM users;

-- name: CreateUser :one
INSERT INTO users (
		username,
		password,
		email,
		roles,
		password_set_at
	) VALUES ($1, $2, $3, $4, NOW())
RETURNING *;

-- name: DeleteUser :exec
DELETE FROM users WHERE username = $1;

-- name: UpdatePassword :exec
UPDATE users SET password = $2, password_set_at = NOW() WHERE username = $1;

-- name: UpdateUser :one
UPDATE users SET
	email = COALESCE(sqlc.narg('email'), email),
	roles = COALESCE(sqlc.narg('roles'), roles)
WHERE
	username = $1
RETURNING *;

-- name: RecordUserLogin :exec
UPDATE users SET
	last_login_at = @last_login_at,
	last_login_from = @last_login_from
WHERE username = $1;

-- name: CreateAPIKey :exec
INSERT INTO api_keys(
	owner,
	name,
	created_at,
	expires,
	disabled,
	api_key
	) VALUES (@owner, @name, @created_at, @expires, @disabled, @hashed_key);

-- name: DeleteAPIKey :exec
DELETE FROM api_keys WHERE api_key = $1;

-- name: GetAPIKey :one
SELECT
	a.id,
	a.owner,
	a.name,
	a.created_at,
	a.expires,
	a.disabled,
	a.api_key,
	u.username
FROM api_keys a
JOIN users u ON (a.owner = u.id)
WHERE api_key = $1;

-- name: GetAppPrefs :one
SELECT (prefs->>(@app_name::TEXT))::JSONB FROM users WHERE id = @uid;

-- name: SetAppPrefs :exec
UPDATE users SET prefs = COALESCE(prefs, '{}'::JSONB) || jsonb_build_object(@app_name::TEXT, @prefs::JSONB) WHERE id = @uid;

-- name: DisableUser :exec
UPDATE users SET disabled_at = NOW() WHERE username = @username;
