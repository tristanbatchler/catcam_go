package users

import "fmt"

type ErrUserAlreadyExists struct {
	Username string
}

func (e ErrUserAlreadyExists) Error() string {
	return fmt.Sprintf("user with username %s already exists", e.Username)
}

type ErrUserNotFound struct {
	ID       int64
	Username string
}

func (e ErrUserNotFound) Error() string {
	return fmt.Sprintf("user with id %d not found", e.ID)
}

type ErrNoMatchingCredentials struct {
	Username string
}

func (e ErrNoMatchingCredentials) Error() string {
	return fmt.Sprintf("no user found with username %s and supplied password hash", e.Username)
}

type ErrMissingField struct {
	Field string
}

func (e ErrMissingField) Error() string {
	return fmt.Sprintf("missing field: %s", e.Field)
}
