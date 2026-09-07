package testdb

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/trnahnh/recap/internal/config"
	"github.com/trnahnh/recap/internal/db"
)

const (
	EnvIntegration = "RECAP_INTEGRATION"
	EnvConfigPath  = "RECAP_TEST_CONFIG"
)

func Setup(t *testing.T) (*config.Config, *pgxpool.Pool) {
	t.Helper()

	if os.Getenv(EnvIntegration) == "" {
		t.Skipf("set %s=1 with the recap database running to enable integration tests", EnvIntegration)
	}

	cfgPath := os.Getenv(EnvConfigPath)
	if cfgPath == "" {
		p, err := config.DefaultPath()
		if err != nil {
			t.Fatalf("locating config: %v", err)
		}
		cfgPath = p
	}
	cfg, err := config.Load(cfgPath)
	if err != nil {
		t.Fatalf("loading config %s: %v", cfgPath, err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	t.Cleanup(cancel)

	admin, err := db.NewPool(ctx, cfg)
	if err != nil {
		t.Fatalf("connecting to %s: %v", cfg.DB.Name, err)
	}
	t.Cleanup(admin.Close)

	name := "recap_test_" + randomSuffix(t)
	ident := pgx.Identifier{name}.Sanitize()
	if _, err := admin.Exec(ctx, "CREATE DATABASE "+ident); err != nil {
		t.Fatalf("creating test database %s: %v", name, err)
	}
	t.Cleanup(func() {
		dropCtx, dropCancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer dropCancel()
		if _, err := admin.Exec(dropCtx, "DROP DATABASE IF EXISTS "+ident+" WITH (FORCE)"); err != nil {
			t.Errorf("dropping test database %s: %v", name, err)
		}
	})

	testCfg := *cfg
	testCfg.DB.Name = name
	if err := db.RunMigrations(&testCfg); err != nil {
		t.Fatalf("migrating test database %s: %v", name, err)
	}

	pool, err := db.NewPool(ctx, &testCfg)
	if err != nil {
		t.Fatalf("connecting to test database %s: %v", name, err)
	}
	t.Cleanup(pool.Close)

	return &testCfg, pool
}

func randomSuffix(t *testing.T) string {
	t.Helper()
	buf := make([]byte, 6)
	if _, err := rand.Read(buf); err != nil {
		t.Fatalf("generating database suffix: %v", err)
	}
	return hex.EncodeToString(buf)
}
