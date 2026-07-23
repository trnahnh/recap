package daemon

import (
	"context"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/trnahnh/recap/internal/config"
	"github.com/trnahnh/recap/internal/db"
)

func TestImportMissingFile(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.json")
	cfg, err := config.Generate()
	if err != nil {
		t.Fatalf("generate config: %v", err)
	}
	if err := cfg.Save(cfgPath); err != nil {
		t.Fatalf("save config: %v", err)
	}

	err = Import(context.Background(), cfgPath, filepath.Join(dir, "does-not-exist.dump"))
	if err == nil {
		t.Fatal("expected error importing a missing file, got nil")
	}
}

func TestExportImportRoundTrip(t *testing.T) {
	if _, err := exec.LookPath("docker"); err != nil {
		t.Skip("docker not available")
	}

	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.json")

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	if err := Init(ctx, cfgPath); err != nil {
		t.Skipf("cannot bring up database: %v", err)
	}
	t.Cleanup(func() {
		_ = Stop(context.Background(), cfgPath)
	})

	cfg, err := config.Load(cfgPath)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}

	var recordID string
	func() {
		pool, err := db.NewPool(ctx, cfg)
		if err != nil {
			t.Fatalf("pool: %v", err)
		}
		defer pool.Close()

		var projectID string
		if err := pool.QueryRow(ctx,
			`INSERT INTO projects (name, root_path) VALUES ($1, $2) RETURNING id`,
			"roundtrip", "/tmp/roundtrip",
		).Scan(&projectID); err != nil {
			t.Fatalf("insert project: %v", err)
		}
		if err := pool.QueryRow(ctx,
			`INSERT INTO records (project_id, record_type, title, task, summary, created_by)
			 VALUES ($1, 'decision', $2, $3, $4, $5) RETURNING id`,
			projectID, "pick postgres", "choose a store", "went with postgres", "tester",
		).Scan(&recordID); err != nil {
			t.Fatalf("insert record: %v", err)
		}
	}()

	dumpPath := filepath.Join(dir, "dump.dump")
	if err := Export(ctx, cfgPath, dumpPath); err != nil {
		t.Fatalf("export: %v", err)
	}

	func() {
		pool, err := db.NewPool(ctx, cfg)
		if err != nil {
			t.Fatalf("pool: %v", err)
		}
		defer pool.Close()
		if _, err := pool.Exec(ctx, `DELETE FROM projects`); err != nil {
			t.Fatalf("truncate: %v", err)
		}
	}()

	if err := Import(ctx, cfgPath, dumpPath); err != nil {
		t.Fatalf("import: %v", err)
	}

	pool, err := db.NewPool(ctx, cfg)
	if err != nil {
		t.Fatalf("pool: %v", err)
	}
	defer pool.Close()

	var projectCount, recordCount int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM projects`).Scan(&projectCount); err != nil {
		t.Fatalf("count projects: %v", err)
	}
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM records`).Scan(&recordCount); err != nil {
		t.Fatalf("count records: %v", err)
	}
	if projectCount != 1 || recordCount != 1 {
		t.Fatalf("expected 1 project and 1 record after import, got %d and %d", projectCount, recordCount)
	}

	var title, createdBy string
	if err := pool.QueryRow(ctx,
		`SELECT title, created_by FROM records WHERE id = $1`, recordID,
	).Scan(&title, &createdBy); err != nil {
		t.Fatalf("fetch restored record: %v", err)
	}
	if title != "pick postgres" || createdBy != "tester" {
		t.Fatalf("restored record mismatch: title=%q created_by=%q", title, createdBy)
	}
}
