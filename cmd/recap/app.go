package main

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/trnahnh/recap/internal/config"
	"github.com/trnahnh/recap/internal/db"
	"github.com/trnahnh/recap/internal/store"
)

type app struct {
	cfg   *config.Config
	pool  *pgxpool.Pool
	store *store.Store
}

func openApp(ctx context.Context, configPath string) (*app, error) {
	cfg, err := config.Load(configPath)
	if err != nil {
		return nil, fmt.Errorf("not initialized — run `recap init` first: %w", err)
	}
	pool, err := db.NewPool(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("database is not running — run `recap start` first: %w", err)
	}
	return &app{cfg: cfg, pool: pool, store: store.New(pool)}, nil
}

func (a *app) close() {
	a.pool.Close()
}
