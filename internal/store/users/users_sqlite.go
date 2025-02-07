package users

import (
	"catcam_go/internal/db"
	"context"
	"database/sql"
	"log"
	"strings"

	"modernc.org/sqlite"
	sqlite3 "modernc.org/sqlite/lib"
)

type UserStore struct {
	queries *db.Queries
	logger  *log.Logger
}

func NewUserStore(queries *db.Queries, logger *log.Logger) *UserStore {
	return &UserStore{
		logger:  logger,
		queries: queries,
	}
}

func (us *UserStore) AddUser(ctx context.Context, params db.AddUserParams) (db.User, error) {
	zero := db.User{}

	if params.Username == "" {
		return zero, ErrMissingField{Field: "username"}
	}

	// Normalize username
	params.Username = strings.ToLower(params.Username)

	// Add the user to the database
	user, err := us.queries.AddUser(ctx, params)
	if err != nil {
		if sqlErr, ok := err.(*sqlite.Error); ok {
			if sqlErr.Code() == sqlite3.SQLITE_CONSTRAINT_UNIQUE {
				return zero, ErrUserAlreadyExists{Username: params.Username}
			}
			us.logger.Printf("error adding user: %v, %v", err, sqlErr)
		}
		us.logger.Printf("error adding user: %v", err)
		return zero, err
	}

	us.logger.Printf("user added: %v", user)
	return user, nil
}

func (us *UserStore) GetUser(ctx context.Context, id int64) (db.User, error) {
	user, err := us.queries.GetUserById(ctx, id)
	if err != nil {
		if err == sql.ErrNoRows {
			return db.User{}, ErrUserNotFound{ID: id}
		}
		us.logger.Printf("error getting brewer: %v", err)
		return db.User{}, err
	}
	return user, nil
}

func (us *UserStore) GetUsers(ctx context.Context) ([]db.User, error) {
	users, err := us.queries.GetUsers(ctx)
	if err != nil {
		us.logger.Printf("error getting users: %v", err)
		return nil, err
	}
	return users, nil
}

func (us *UserStore) GetUserById(ctx context.Context, id int64) (db.User, error) {
	zero := db.User{}

	user, err := us.queries.GetUserById(ctx, id)
	if err != nil {
		if err == sql.ErrNoRows {
			return zero, ErrUserNotFound{ID: id}
		}
		us.logger.Printf("error getting user by id: %v", err)
		return zero, err
	}

	return user, nil
}

func (us *UserStore) GetUserByUsername(ctx context.Context, username string) (db.User, error) {
	zero := db.User{}

	user, err := us.queries.GetUserByUsername(ctx, username)
	if err != nil {
		if err == sql.ErrNoRows {
			return zero, ErrUserNotFound{Username: username}
		}
		us.logger.Printf("error getting user by username: %v", err)
		return zero, err
	}

	return user, nil
}

func (us *UserStore) DeleteUser(ctx context.Context, id int64) (db.User, error) {
	zero := db.User{}

	user, err := us.queries.DeleteUser(ctx, id)
	if err != nil {
		if sqlErr, ok := err.(*sqlite.Error); ok {
			if sqlErr.Code() == sqlite3.SQLITE_CONSTRAINT_FOREIGNKEY {
				return zero, ErrUserNotFound{ID: id}
			}
		}
		us.logger.Printf("error deleting user: %v", err)
		return zero, err
	}

	us.logger.Printf("user deleted: %v", user)
	return user, nil
}

func (us *UserStore) CountUsers(ctx context.Context) (int64, error) {
	count, err := us.queries.CountUsers(ctx)
	if err != nil {
		us.logger.Printf("error counting users: %v", err)
		return 0, err
	}
	return count, nil
}

func (us *UserStore) SetUserLastLogin(ctx context.Context, id int64) error {
	err := us.queries.SetUserLastLogin(ctx, id)
	if err != nil {
		us.logger.Printf("error setting user last login: %v", err)
		return err
	}
	return nil
}
