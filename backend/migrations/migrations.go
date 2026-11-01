package migrations

import (
	"context"
	"database/sql"
	_ "embed"
)

//go:embed 001_init.sql
var InitSQL string

// Run executes the embedded database schema migrations.
func Run(ctx context.Context, db *sql.DB) error {
	if _, err := db.ExecContext(ctx, InitSQL); err != nil {
		return err
	}
	return nil
}
