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
