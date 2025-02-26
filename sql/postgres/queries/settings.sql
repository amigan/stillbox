-- name: GetSetting :one
SELECT value FROM settings WHERE name = @name;

-- name: SetSetting :exec
INSERT INTO settings (name, updated_by, value) VALUES (@name, @updated_by, @value)
	ON CONFLICT (name) DO UPDATE SET
	value = @value,
	updated_by = @updated_by;

-- name: DeleteSetting :exec
DELETE FROM settings WHERE name = @name;
