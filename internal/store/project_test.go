package store

import (
	"context"
	"errors"
	"os"
	"runtime"
	"strings"
	"testing"
)

func TestProjectRoundTrip(t *testing.T) {
	s, ctx := newTestStore(t)
	dir := t.TempDir()

	created, err := s.CreateProject(ctx, "recap", dir)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	normalized, err := NormalizeRootPath(dir)
	if err != nil {
		t.Fatalf("normalize: %v", err)
	}
	if created.RootPath != normalized {
		t.Fatalf("stored root_path %q, want normalized %q", created.RootPath, normalized)
	}
	if created.ID == "" || created.CreatedAt.IsZero() {
		t.Fatalf("expected a populated project, got %+v", created)
	}

	byID, err := s.GetProject(ctx, created.ID)
	if err != nil {
		t.Fatalf("get by id: %v", err)
	}
	if byID.ID != created.ID || byID.Name != "recap" {
		t.Fatalf("get by id returned %+v, want %+v", byID, created)
	}

	byRoot, err := s.GetProjectByRoot(ctx, dir)
	if err != nil {
		t.Fatalf("get by root: %v", err)
	}
	if byRoot.ID != created.ID {
		t.Fatalf("get by root returned %s, want %s", byRoot.ID, created.ID)
	}

	listed, err := s.ListProjects(ctx)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(listed) != 1 || listed[0].ID != created.ID {
		t.Fatalf("expected exactly the created project, got %+v", listed)
	}

	if err := s.DeleteProject(ctx, created.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if _, err := s.GetProject(ctx, created.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound after delete, got %v", err)
	}
}

func TestCreateProjectIsIdempotent(t *testing.T) {
	s, ctx := newTestStore(t)
	dir := t.TempDir()

	first, err := s.CreateProject(ctx, "recap", dir)
	if err != nil {
		t.Fatalf("first create: %v", err)
	}

	variants := []string{dir, dir + string(os.PathSeparator)}
	if runtime.GOOS == "windows" {
		variants = append(variants, strings.ToUpper(dir))
	}

	for _, v := range variants {
		again, err := s.CreateProject(ctx, "recap", v)
		if err != nil {
			t.Fatalf("re-register %q: %v", v, err)
		}
		if again.ID != first.ID {
			t.Fatalf("re-register %q created %s, want %s", v, again.ID, first.ID)
		}
	}

	listed, err := s.ListProjects(ctx)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(listed) != 1 {
		t.Fatalf("expected 1 project after %d registrations, got %d", len(variants)+1, len(listed))
	}
}

func TestCreateProjectRejectsNameMismatch(t *testing.T) {
	s, ctx := newTestStore(t)
	dir := t.TempDir()

	if _, err := s.CreateProject(ctx, "recap", dir); err != nil {
		t.Fatalf("create: %v", err)
	}

	_, err := s.CreateProject(ctx, "something-else", dir)
	if !errors.Is(err, ErrProjectNameMismatch) {
		t.Fatalf("expected ErrProjectNameMismatch, got %v", err)
	}

	listed, err := s.ListProjects(ctx)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(listed) != 1 || listed[0].Name != "recap" {
		t.Fatalf("expected the original project untouched, got %+v", listed)
	}
}

func TestDeleteProjectCascadesToRecords(t *testing.T) {
	s, ctx := newTestStore(t)

	project, err := s.CreateProject(ctx, "recap", t.TempDir())
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	recordID := insertRecordDirect(ctx, t, s, project.ID)

	if err := s.DeleteProject(ctx, project.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}

	var remaining int
	if err := s.pool.QueryRow(ctx,
		`SELECT count(*) FROM records WHERE id = $1`, recordID,
	).Scan(&remaining); err != nil {
		t.Fatalf("count records: %v", err)
	}
	if remaining != 0 {
		t.Fatalf("expected the record to cascade away, %d remain", remaining)
	}
}

func TestProjectNotFound(t *testing.T) {
	s, ctx := newTestStore(t)

	missing := "00000000-0000-0000-0000-000000000000"
	if _, err := s.GetProject(ctx, missing); !errors.Is(err, ErrNotFound) {
		t.Fatalf("get: expected ErrNotFound, got %v", err)
	}
	if _, err := s.GetProjectByRoot(ctx, t.TempDir()); !errors.Is(err, ErrNotFound) {
		t.Fatalf("get by root: expected ErrNotFound, got %v", err)
	}
	if err := s.DeleteProject(ctx, missing); !errors.Is(err, ErrNotFound) {
		t.Fatalf("delete: expected ErrNotFound, got %v", err)
	}
}

func insertRecordDirect(ctx context.Context, t *testing.T, s *Store, projectID string) string {
	t.Helper()
	var id string
	if err := s.pool.QueryRow(ctx,
		`INSERT INTO records (project_id, record_type, title, task, summary, created_by)
		 VALUES ($1, 'decision', $2, $3, $4, $5) RETURNING id`,
		projectID, "seed", "seed task", "seed summary", "store-test",
	).Scan(&id); err != nil {
		t.Fatalf("seed record: %v", err)
	}
	return id
}
