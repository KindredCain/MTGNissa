package appdata

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/pressly/goose/v3"
	"mtgnissa/migrations"
)

// migrationFiles contains only App DB migrations. Card DB schema is managed by
// the card-data loader and must never be passed to Goose.
var migrationFiles = migrations.App

// Migrate upgrades the App DB to the latest embedded schema version.
func Migrate(ctx context.Context, db *sql.DB) error {
	goose.SetBaseFS(migrationFiles)
	if err := goose.SetDialect("mysql"); err != nil {
		return fmt.Errorf("set app database migration dialect: %w", err)
	}
	if err := goose.UpContext(ctx, db, "app"); err != nil {
		return fmt.Errorf("migrate app database: %w", err)
	}
	return nil
}
