package main

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/uptrace/bun"
	"github.com/uptrace/bun/migrate"

	"github.com/rhuda/fullstack-boilerplate/backend/migrations"
)

func runMigrations(ctx context.Context, db *bun.DB) error {
	migrator := migrate.NewMigrator(db, migrations.Migrations)

	if err := migrator.Init(ctx); err != nil {
		return fmt.Errorf("init migrator: %w", err)
	}

	group, err := migrator.Migrate(ctx)
	if err != nil {
		return fmt.Errorf("migrate: %w", err)
	}

	if group.IsZero() {
		slog.Info("database is up to date")
	} else {
		slog.Info("migrations applied", "group", group.ID, "count", len(group.Migrations))
	}
	return nil
}
