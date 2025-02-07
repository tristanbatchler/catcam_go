package db

import (
	"context"
	"database/sql"
	_ "embed"
	"fmt"
)

// Embed the database schema to be used when creating the database tables
//
//go:embed config/schema.sql
var schemaGenSql string

func GenSchema(dbPool *sql.DB) error {
	_, err := dbPool.ExecContext(context.Background(), schemaGenSql)
	if err != nil {
		return fmt.Errorf("error initializing database: %w", err)
	}
	return nil
}
