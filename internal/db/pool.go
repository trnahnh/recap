package db

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/trnahnh/recap/internal/config"
)

func AssertLoopback(cfg *config.Config) error {
	if cfg.DB.Host != config.LoopbackHost {
		return fmt.Errorf(
			"refusing to start: database host is %q, but Recap only binds %s (localhost-only invariant)",
			cfg.DB.Host, config.LoopbackHost,
		)
	}
	return nil
}

func NewPool(ctx context.Context, cfg *config.Config) (*pgxpool.Pool, error) {
	if err := AssertLoopback(cfg); err != nil {
		return nil, err
	}

	poolCfg, err := pgxpool.ParseConfig(cfg.DSN())
	if err != nil {
		return nil, fmt.Errorf("parsing pool config: %w", err)
	}
	if cfg.DB.PoolSize > 0 {
		poolCfg.MaxConns = int32(cfg.DB.PoolSize)
	}

	pool, err := pgxpool.NewWithConfig(ctx, poolCfg)
	if err != nil {
		return nil, fmt.Errorf("creating pool: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("pinging database: %w", err)
	}
	return pool, nil
}
