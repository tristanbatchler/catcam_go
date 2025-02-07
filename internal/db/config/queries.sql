/* === CONTACTS === */

-- name: AddUser :one
INSERT INTO users (username, password_hash) 
VALUES (?, ?)
RETURNING *;

-- name: GetUserById :one
SELECT * 
FROM users
WHERE id = ?;

-- name: GetUserByUsername :one
SELECT *
FROM users
WHERE username = ?;

-- name: GetUsers :many
SELECT *
FROM users;

-- name: DeleteUser :one
DELETE FROM users
WHERE id = ?
RETURNING *;

-- name: CountUsers :one
SELECT COUNT(*)
FROM users;

-- name: SetUserLastLogin :exec
UPDATE users
SET last_login = datetime()
WHERE id = ?;

