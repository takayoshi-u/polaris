package migrations

import (
	"context"
	"database/sql"
	_ "embed"
)

//go:embed 001_init.sql
var InitSQL string

//go:embed 002_api_webhooks.sql
var APIWebhooksSQL string

// Run executes the embedded database schema migrations in order.
func Run(ctx context.Context, db *sql.DB) error {
	if _, err := db.ExecContext(ctx, InitSQL); err != nil {
		return err
	}
	if _, err := db.ExecContext(ctx, APIWebhooksSQL); err != nil {
		return err
	}
	return nil
}

