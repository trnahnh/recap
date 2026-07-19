package daemon

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/trnahnh/recap/internal/config"
	"github.com/trnahnh/recap/internal/db"
)

const readyTimeout = 60 * time.Second

func Init(ctx context.Context, configPath string) error {
	cfg, err := loadOrCreateConfig(configPath)
	if err != nil {
		return err
	}
	return bringUp(ctx, cfg, configPath)
}

func Start(ctx context.Context, configPath string) error {
	cfg, err := config.Load(configPath)
	if err != nil {
		return fmt.Errorf("not initialized — run `recap init` first: %w", err)
	}
	return bringUp(ctx, cfg, configPath)
}

func Stop(ctx context.Context, configPath string) error {
	cfg, err := config.Load(configPath)
	if err != nil {
		return fmt.Errorf("not initialized — nothing to stop: %w", err)
	}
	composePath, err := writeComposeFile(filepath.Dir(configPath))
	if err != nil {
		return err
	}
	return composeDown(ctx, cfg, composePath)
}

type Report struct {
	ConfigPath string
	Host       string
	Port       int
	Database   string
	Ready      bool
	Detail     string
}

func Status(ctx context.Context, configPath string) (*Report, error) {
	cfg, err := config.Load(configPath)
	if err != nil {
		return nil, fmt.Errorf("not initialized — run `recap init` first: %w", err)
	}
	r := &Report{
		ConfigPath: configPath,
		Host:       cfg.DB.Host,
		Port:       cfg.DB.Port,
		Database:   cfg.DB.Name,
	}
	if err := db.AssertLoopback(cfg); err != nil {
		return r, err
	}
	pool, err := db.NewPool(ctx, cfg)
	if err != nil {
		r.Detail = err.Error()
		return r, nil
	}
	pool.Close()
	r.Ready = true
	return r, nil
}

func bringUp(ctx context.Context, cfg *config.Config, configPath string) error {
	if err := db.AssertLoopback(cfg); err != nil {
		return err
	}
	composePath, err := writeComposeFile(filepath.Dir(configPath))
	if err != nil {
		return err
	}
	if err := composeUp(ctx, cfg, composePath); err != nil {
		return err
	}
	if err := waitReady(ctx, cfg); err != nil {
		return err
	}
	if err := db.RunMigrations(cfg); err != nil {
		return fmt.Errorf("migrating schema: %w", err)
	}
	return nil
}

func waitReady(ctx context.Context, cfg *config.Config) error {
	deadline := time.Now().Add(readyTimeout)
	var lastErr error
	for time.Now().Before(deadline) {
		pool, err := db.NewPool(ctx, cfg)
		if err == nil {
			pool.Close()
			return nil
		}
		lastErr = err
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(time.Second):
		}
	}
	return fmt.Errorf("database not ready after %s: %w", readyTimeout, lastErr)
}

func loadOrCreateConfig(path string) (*config.Config, error) {
	if _, err := os.Stat(path); err == nil {
		return config.Load(path)
	}
	cfg, err := config.Generate()
	if err != nil {
		return nil, err
	}
	if err := cfg.Save(path); err != nil {
		return nil, err
	}
	return cfg, nil
}
