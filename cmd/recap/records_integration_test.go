package main

import (
	"bytes"
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/trnahnh/recap/internal/model"
	"github.com/trnahnh/recap/internal/store"
	"github.com/trnahnh/recap/internal/testdb"
)

type cliFixture struct {
	configPath string
	root       string
	project    model.Project
	store      *store.Store
}

func newCLIFixture(t *testing.T) (cliFixture, context.Context) {
	t.Helper()
	cfg, pool := testdb.Setup(t)

	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.json")
	if err := cfg.Save(configPath); err != nil {
		t.Fatalf("saving test config: %v", err)
	}

	ctx := context.Background()
	s := store.New(pool)
	root := filepath.Join(dir, "project")
	project, err := s.CreateProject(ctx, "cli-test", root)
	if err != nil {
		t.Fatalf("create project: %v", err)
	}
	return cliFixture{configPath: configPath, root: root, project: project, store: s}, ctx
}

func (f cliFixture) run(t *testing.T, args ...string) (string, error) {
	t.Helper()
	var out bytes.Buffer
	full := append(args, "--config", f.configPath, "--project", f.root)
	err := run(full, &out)
	return out.String(), err
}

func (f cliFixture) seed(ctx context.Context, t *testing.T, title string) model.Record {
	t.Helper()
	r := model.NewRecord()
	r.ProjectID = f.project.ID
	r.RecordType = model.RecordTypeDecision
	r.Title = title
	r.Task = "Choose a storage engine"
	r.Summary = "Postgres gives concurrent access and full-text search"
	r.CreatedBy = "cli-test"
	r.Alternatives = []model.Alternative{{Approach: "SQLite", Reason: optionalString("file locking")}}
	r.Files = []model.RecordFile{{FilePath: "internal/store/record.go"}}
	created, err := f.store.CreateRecord(ctx, r)
	if err != nil {
		t.Fatalf("seed %q: %v", title, err)
	}
	return created
}

func TestCLIListShowApproveArchive(t *testing.T) {
	f, ctx := newCLIFixture(t)
	draft := f.seed(ctx, t, "Postgres over SQLite")
	other := f.seed(ctx, t, "Second record")

	out, err := f.run(t, "list")
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if !strings.Contains(out, draft.ID) || !strings.Contains(out, other.ID) {
		t.Fatalf("list output missing seeded ids:\n%s", out)
	}
	if strings.Count(out, "DRAFT") != 2 {
		t.Fatalf("expected both rows to show DRAFT status:\n%s", out)
	}

	out, err = f.run(t, "show", draft.ID)
	if err != nil {
		t.Fatalf("show: %v", err)
	}
	for _, want := range []string{"[DRAFT] Postgres over SQLite", "SQLite", "file locking", "internal/store/record.go"} {
		if !strings.Contains(out, want) {
			t.Fatalf("show output missing %q:\n%s", want, out)
		}
	}

	out, err = f.run(t, "approve", draft.ID)
	if err != nil {
		t.Fatalf("approve: %v", err)
	}
	if !strings.Contains(out, "ACTIVE") {
		t.Fatalf("approve output should report ACTIVE:\n%s", out)
	}
	out, err = f.run(t, "show", draft.ID)
	if err != nil {
		t.Fatalf("show after approve: %v", err)
	}
	if strings.Contains(out, "[DRAFT]") {
		t.Fatalf("approved record still marked DRAFT:\n%s", out)
	}

	out, err = f.run(t, "list", "--status", "active")
	if err != nil {
		t.Fatalf("list active: %v", err)
	}
	if !strings.Contains(out, draft.ID) || strings.Contains(out, other.ID) {
		t.Fatalf("status filter returned the wrong rows:\n%s", out)
	}

	if _, err := f.run(t, "archive", draft.ID); err != nil {
		t.Fatalf("archive: %v", err)
	}
	got, err := f.store.GetRecord(ctx, f.project.ID, draft.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Status != model.RecordStatusArchived {
		t.Fatalf("record is %q after archive, want archived", got.Status)
	}
}

func TestCLIEditAndDelete(t *testing.T) {
	f, ctx := newCLIFixture(t)
	record := f.seed(ctx, t, "Before edit")

	if _, err := f.run(t, "edit", record.ID, "--title", "After edit", "--confidence", "0.9"); err != nil {
		t.Fatalf("edit: %v", err)
	}
	got, err := f.store.GetRecord(ctx, f.project.ID, record.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Title != "After edit" || got.Confidence == nil || *got.Confidence != 0.9 {
		t.Fatalf("edit not applied: %+v", got)
	}
	if len(got.Alternatives) != 1 || len(got.Files) != 1 {
		t.Fatalf("flag edit must preserve children, got %d alternatives, %d files",
			len(got.Alternatives), len(got.Files))
	}

	if _, err := f.run(t, "delete", record.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if _, err := f.store.GetRecord(ctx, f.project.ID, record.ID); err == nil {
		t.Fatal("record still present after delete")
	}
}

func TestCLIErrorsAreClear(t *testing.T) {
	f, _ := newCLIFixture(t)

	_, err := f.run(t, "show", "nope")
	if err == nil || !strings.Contains(err.Error(), "invalid record id") {
		t.Fatalf("malformed id: got %v", err)
	}

	missing := "00000000-0000-0000-0000-000000000000"
	_, err = f.run(t, "show", missing)
	if err == nil || !strings.Contains(err.Error(), "not found") {
		t.Fatalf("missing id: got %v", err)
	}

	var out bytes.Buffer
	err = run([]string{"list", "--config", f.configPath, "--project", filepath.Join(f.root, "nowhere")}, &out)
	if err == nil || !strings.Contains(err.Error(), "recap project add") {
		t.Fatalf("unregistered project: got %v", err)
	}
}
