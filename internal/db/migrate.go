package db

import (
	"database/sql"
	"errors"
	"fmt"

	"github.com/golang-migrate/migrate/v4"
	migratepgx "github.com/golang-migrate/migrate/v4/database/pgx/v5"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	_ "github.com/jackc/pgx/v5/stdlib"

	"github.com/trnahnh/recap/internal/config"
	"github.com/trnahnh/recap/migrations"
)

func RunMigrations(cfg *config.Config) error {
	src, err := iofs.New(migrations.FS, ".")
	if err != nil {
		return fmt.Errorf("loading embedded migrations: %w", err)
	}
	defer src.Close()

	sqlDB, err := sql.Open("pgx", cfg.DSN())
	if err != nil {
		return fmt.Errorf("opening migration connection: %w", err)
	}
	defer sqlDB.Close()

	driver, err := migratepgx.WithInstance(sqlDB, &migratepgx.Config{})
	if err != nil {
		return fmt.Errorf("initializing migrate driver: %w", err)
	}

	m, err := migrate.NewWithInstance("iofs", src, "recap", driver)
	if err != nil {
		return fmt.Errorf("initializing migrator: %w", err)
	}

	if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return fmt.Errorf("applying migrations: %w", err)
	}
	return nil
}
