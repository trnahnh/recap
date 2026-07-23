package daemon

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/trnahnh/recap/internal/config"
	"github.com/trnahnh/recap/internal/db"
)

func Export(ctx context.Context, configPath, outPath string) error {
	cfg, composePath, err := prepareBackup(ctx, configPath)
	if err != nil {
		return err
	}

	f, err := os.OpenFile(outPath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600)
	if err != nil {
		return fmt.Errorf("creating export file %s: %w", outPath, err)
	}

	var stderr bytes.Buffer
	cmd := composeCmd(ctx, cfg, composePath,
		"exec", "-T", "-e", "PGPASSWORD="+cfg.DB.Password, "postgres",
		"pg_dump", "-U", cfg.DB.User, "-d", cfg.DB.Name, "-Fc", "--no-owner", "--no-privileges",
	)
	cmd.Stdout = f
	cmd.Stderr = &stderr

	runErr := cmd.Run()
	closeErr := f.Close()
	if runErr != nil {
		os.Remove(outPath)
		return fmt.Errorf("pg_dump failed: %w\n%s", runErr, stderr.String())
	}
	if closeErr != nil {
		os.Remove(outPath)
		return fmt.Errorf("finalizing export file %s: %w", outPath, closeErr)
	}
	return nil
}

func Import(ctx context.Context, configPath, inPath string) error {
	f, err := os.Open(inPath)
	if err != nil {
		return fmt.Errorf("opening import file %s: %w", inPath, err)
	}
	defer f.Close()

	cfg, composePath, err := prepareBackup(ctx, configPath)
	if err != nil {
		return err
	}

	var stderr bytes.Buffer
	cmd := composeCmd(ctx, cfg, composePath,
		"exec", "-T", "-e", "PGPASSWORD="+cfg.DB.Password, "postgres",
		"pg_restore", "-U", cfg.DB.User, "-d", cfg.DB.Name, "--clean", "--if-exists", "--no-owner",
	)
	cmd.Stdin = f
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("pg_restore failed: %w\n%s", err, stderr.String())
	}
	return nil
}

func prepareBackup(ctx context.Context, configPath string) (*config.Config, string, error) {
	cfg, err := config.Load(configPath)
	if err != nil {
		return nil, "", fmt.Errorf("not initialized — run `recap init` first: %w", err)
	}
	if err := db.AssertLoopback(cfg); err != nil {
		return nil, "", err
	}
	pool, err := db.NewPool(ctx, cfg)
	if err != nil {
		return nil, "", fmt.Errorf("database is not running — run `recap start` first: %w", err)
	}
	pool.Close()

	composePath, err := writeComposeFile(filepath.Dir(configPath))
	if err != nil {
		return nil, "", err
	}
	return cfg, composePath, nil
}
