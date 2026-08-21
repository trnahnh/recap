package store

import (
	"context"
	"errors"
	"testing"

	"github.com/trnahnh/recap/internal/model"
)

func TestRecordRoundTrip(t *testing.T) {
	s, ctx := newTestStore(t)
	project := seedProject(ctx, t, s)

	in := draftRecord(project.ID)
	in.Status = model.RecordStatusActive
	in.Alternatives = []model.Alternative{
		{Approach: "SQLite", Result: strptr("rejected"), Reason: strptr("file-level locking"), Position: 7},
		{Approach: "flat files", Reason: strptr("no query surface"), Position: 3},
	}
	in.Files = []model.RecordFile{
		{FilePath: "docs/ARCHITECTURE_DECISIONS.md", CommitHash: strptr("abc123")},
		{FilePath: "internal/store/record.go"},
	}

	created, err := s.CreateRecord(ctx, in)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if created.Status != model.RecordStatusDraft {
		t.Fatalf("created record has status %q, want draft regardless of the requested status", created.Status)
	}
	if created.ID == "" || created.CreatedAt.IsZero() {
		t.Fatalf("expected a populated record, got %+v", created)
	}
	for i := range created.Alternatives {
		if created.Alternatives[i].Position != i {
			t.Fatalf("alternative %d stored position %d, want the slice index",
				i, created.Alternatives[i].Position)
		}
	}

	got, err := s.GetRecord(ctx, project.ID, created.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Title != in.Title || got.Task != in.Task || got.Summary != in.Summary {
		t.Fatalf("get returned %+v, want the created record", got)
	}
	if len(got.Alternatives) != 2 {
		t.Fatalf("expected 2 alternatives, got %d", len(got.Alternatives))
	}
	if got.Alternatives[0].Approach != "SQLite" || got.Alternatives[1].Approach != "flat files" {
		t.Fatalf("alternatives came back as %q, %q, want input order",
			got.Alternatives[0].Approach, got.Alternatives[1].Approach)
	}
	if len(got.Files) != 2 {
		t.Fatalf("expected 2 files, got %d", len(got.Files))
	}
	if len(got.Relationships) != 0 || len(got.IncomingRelationships) != 0 {
		t.Fatalf("expected no relationships on a fresh record, got %+v / %+v",
			got.Relationships, got.IncomingRelationships)
	}
}

func TestCreateRecordRollsBackOnChildFailure(t *testing.T) {
	s, ctx := newTestStore(t)
	project := seedProject(ctx, t, s)

	in := draftRecord(project.ID)
	in.Alternatives = []model.Alternative{{Approach: "SQLite"}}
	in.Files = []model.RecordFile{
		{FilePath: "internal/store/record.go"},
		{FilePath: "internal/store/record.go"},
	}

	if _, err := s.CreateRecord(ctx, in); err == nil {
		t.Fatal("expected a duplicate file path to fail the insert")
	}

	for table, count := range map[string]int{
		"records":      countRows(ctx, t, s, `SELECT count(*) FROM records`),
		"alternatives": countRows(ctx, t, s, `SELECT count(*) FROM alternatives`),
		"record_files": countRows(ctx, t, s, `SELECT count(*) FROM record_files`),
	} {
		if count != 0 {
			t.Fatalf("expected the failed create to leave %s empty, %d rows remain", table, count)
		}
	}
}

func TestUpdateRecordReplacesChildren(t *testing.T) {
	s, ctx := newTestStore(t)
	project := seedProject(ctx, t, s)

	in := draftRecord(project.ID)
	in.Alternatives = []model.Alternative{{Approach: "SQLite"}, {Approach: "flat files"}}
	in.Files = []model.RecordFile{{FilePath: "a.go"}, {FilePath: "b.go"}}
	created, err := s.CreateRecord(ctx, in)
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	created.Title = "Postgres over SQLite, revised"
	created.Alternatives = []model.Alternative{{Approach: "DuckDB"}}
	created.Files = nil

	updated, err := s.UpdateRecord(ctx, created)
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	if updated.Title != "Postgres over SQLite, revised" {
		t.Fatalf("title not updated, got %q", updated.Title)
	}
	if len(updated.Alternatives) != 1 || updated.Alternatives[0].Approach != "DuckDB" {
		t.Fatalf("expected the alternatives to be replaced, got %+v", updated.Alternatives)
	}
	if len(updated.Files) != 0 {
		t.Fatalf("expected nil files to clear the collection, got %+v", updated.Files)
	}

	got, err := s.GetRecord(ctx, project.ID, created.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if len(got.Alternatives) != 1 || len(got.Files) != 0 {
		t.Fatalf("re-read disagrees with the update: %d alternatives, %d files",
			len(got.Alternatives), len(got.Files))
	}
	if total := countRows(ctx, t, s, `SELECT count(*) FROM alternatives`); total != 1 {
		t.Fatalf("expected the replaced alternatives to be deleted, %d rows remain", total)
	}
}

func TestUpdateRecordRejectsStatusChange(t *testing.T) {
	s, ctx := newTestStore(t)
	project := seedProject(ctx, t, s)

	created, err := s.CreateRecord(ctx, draftRecord(project.ID))
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	created.Status = model.RecordStatusActive
	if _, err := s.UpdateRecord(ctx, created); !errors.Is(err, ErrStatusChangeRejected) {
		t.Fatalf("expected ErrStatusChangeRejected, got %v", err)
	}

	got, err := s.GetRecord(ctx, project.ID, created.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Status != model.RecordStatusDraft {
		t.Fatalf("record status is %q, want it left as draft", got.Status)
	}
}

func TestListRecordsFilters(t *testing.T) {
	s, ctx := newTestStore(t)
	project := seedProject(ctx, t, s)

	decision := draftRecord(project.ID)
	decision.Title = "decision draft"
	draft, err := s.CreateRecord(ctx, decision)
	if err != nil {
		t.Fatalf("create decision: %v", err)
	}

	promoted := draftRecord(project.ID)
	promoted.Title = "decision active"
	active, err := s.CreateRecord(ctx, promoted)
	if err != nil {
		t.Fatalf("create promoted: %v", err)
	}
	setRecordStatus(ctx, t, s, active.ID, model.RecordStatusActive)

	other := draftRecord(project.ID)
	other.Title = "constraint draft"
	other.RecordType = model.RecordTypeConstraint
	constraint, err := s.CreateRecord(ctx, other)
	if err != nil {
		t.Fatalf("create constraint: %v", err)
	}

	all, err := s.ListRecords(ctx, project.ID, RecordFilter{})
	if err != nil {
		t.Fatalf("list all: %v", err)
	}
	if len(all) != 3 {
		t.Fatalf("expected 3 records with no filter, got %d", len(all))
	}
	for _, r := range all {
		if len(r.Alternatives) != 0 || len(r.Files) != 0 {
			t.Fatalf("expected a shallow list, record %s carries children", r.ID)
		}
	}

	drafts, err := s.ListRecords(ctx, project.ID,
		RecordFilter{Statuses: []model.RecordStatus{model.RecordStatusDraft}})
	if err != nil {
		t.Fatalf("list drafts: %v", err)
	}
	if len(drafts) != 2 {
		t.Fatalf("expected 2 drafts, got %d", len(drafts))
	}
	for _, r := range drafts {
		if r.ID == active.ID {
			t.Fatalf("status filter returned the active record %s", active.ID)
		}
	}

	decisions, err := s.ListRecords(ctx, project.ID,
		RecordFilter{RecordTypes: []model.RecordType{model.RecordTypeDecision}})
	if err != nil {
		t.Fatalf("list decisions: %v", err)
	}
	if len(decisions) != 2 {
		t.Fatalf("expected 2 decisions, got %d", len(decisions))
	}
	for _, r := range decisions {
		if r.ID == constraint.ID {
			t.Fatalf("type filter returned the constraint record %s", constraint.ID)
		}
	}

	both, err := s.ListRecords(ctx, project.ID, RecordFilter{
		Statuses:    []model.RecordStatus{model.RecordStatusDraft},
		RecordTypes: []model.RecordType{model.RecordTypeDecision},
	})
	if err != nil {
		t.Fatalf("list drafts and decisions: %v", err)
	}
	if len(both) != 1 || both[0].ID != draft.ID {
		t.Fatalf("expected only the draft decision %s, got %+v", draft.ID, both)
	}
}

func TestListRecordsUnknownProjectIsEmpty(t *testing.T) {
	s, ctx := newTestStore(t)

	records, err := s.ListRecords(ctx, "00000000-0000-0000-0000-000000000000", RecordFilter{})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(records) != 0 {
		t.Fatalf("expected no records for an unknown project, got %d", len(records))
	}
}

func TestGetRecordLoadsBothRelationshipDirections(t *testing.T) {
	s, ctx := newTestStore(t)
	project := seedProject(ctx, t, s)

	newer, err := s.CreateRecord(ctx, draftRecord(project.ID))
	if err != nil {
		t.Fatalf("create newer: %v", err)
	}
	older, err := s.CreateRecord(ctx, draftRecord(project.ID))
	if err != nil {
		t.Fatalf("create older: %v", err)
	}
	if _, err := s.pool.Exec(ctx,
		`INSERT INTO relationships (record_id, target_record_id, relationship_type)
		 VALUES ($1, $2, 'supersedes')`, newer.ID, older.ID,
	); err != nil {
		t.Fatalf("seed relationship: %v", err)
	}

	source, err := s.GetRecord(ctx, project.ID, newer.ID)
	if err != nil {
		t.Fatalf("get newer: %v", err)
	}
	if len(source.Relationships) != 1 || source.Relationships[0].TargetRecordID != older.ID {
		t.Fatalf("expected one outgoing relationship to %s, got %+v", older.ID, source.Relationships)
	}
	if len(source.IncomingRelationships) != 0 {
		t.Fatalf("expected no incoming relationships, got %+v", source.IncomingRelationships)
	}

	target, err := s.GetRecord(ctx, project.ID, older.ID)
	if err != nil {
		t.Fatalf("get older: %v", err)
	}
	if len(target.IncomingRelationships) != 1 || target.IncomingRelationships[0].RecordID != newer.ID {
		t.Fatalf("expected one incoming relationship from %s, got %+v",
			newer.ID, target.IncomingRelationships)
	}
	if len(target.Relationships) != 0 {
		t.Fatalf("expected no outgoing relationships, got %+v", target.Relationships)
	}
}

func TestRecordIsScopedToItsProject(t *testing.T) {
	s, ctx := newTestStore(t)
	owner := seedProject(ctx, t, s)

	intruder, err := s.CreateProject(ctx, "other", t.TempDir())
	if err != nil {
		t.Fatalf("create other project: %v", err)
	}

	created, err := s.CreateRecord(ctx, draftRecord(owner.ID))
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	if _, err := s.GetRecord(ctx, intruder.ID, created.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("get from another project: expected ErrNotFound, got %v", err)
	}
	if err := s.DeleteRecord(ctx, intruder.ID, created.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("delete from another project: expected ErrNotFound, got %v", err)
	}

	trespass := created
	trespass.ProjectID = intruder.ID
	if _, err := s.UpdateRecord(ctx, trespass); !errors.Is(err, ErrNotFound) {
		t.Fatalf("update from another project: expected ErrNotFound, got %v", err)
	}

	if _, err := s.GetRecord(ctx, owner.ID, created.ID); err != nil {
		t.Fatalf("expected the record to survive in its own project: %v", err)
	}
}

func TestDeleteRecordCascadesToChildren(t *testing.T) {
	s, ctx := newTestStore(t)
	project := seedProject(ctx, t, s)

	in := draftRecord(project.ID)
	in.Alternatives = []model.Alternative{{Approach: "SQLite"}}
	in.Files = []model.RecordFile{{FilePath: "a.go"}}
	created, err := s.CreateRecord(ctx, in)
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	if err := s.DeleteRecord(ctx, project.ID, created.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if _, err := s.GetRecord(ctx, project.ID, created.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound after delete, got %v", err)
	}
	if total := countRows(ctx, t, s, `SELECT count(*) FROM alternatives`); total != 0 {
		t.Fatalf("expected alternatives to cascade away, %d remain", total)
	}
	if total := countRows(ctx, t, s, `SELECT count(*) FROM record_files`); total != 0 {
		t.Fatalf("expected record files to cascade away, %d remain", total)
	}
}

func TestRecordNotFound(t *testing.T) {
	s, ctx := newTestStore(t)
	project := seedProject(ctx, t, s)
	missing := "00000000-0000-0000-0000-000000000000"

	if _, err := s.GetRecord(ctx, project.ID, missing); !errors.Is(err, ErrNotFound) {
		t.Fatalf("get: expected ErrNotFound, got %v", err)
	}
	if err := s.DeleteRecord(ctx, project.ID, missing); !errors.Is(err, ErrNotFound) {
		t.Fatalf("delete: expected ErrNotFound, got %v", err)
	}

	absent := draftRecord(project.ID)
	absent.ID = missing
	if _, err := s.UpdateRecord(ctx, absent); !errors.Is(err, ErrNotFound) {
		t.Fatalf("update: expected ErrNotFound, got %v", err)
	}
}

func seedProject(ctx context.Context, t *testing.T, s *Store) model.Project {
	t.Helper()
	project, err := s.CreateProject(ctx, "recap", t.TempDir())
	if err != nil {
		t.Fatalf("create project: %v", err)
	}
	return project
}

func draftRecord(projectID string) model.Record {
	record := model.NewRecord()
	record.ProjectID = projectID
	record.RecordType = model.RecordTypeDecision
	record.Title = "Postgres over SQLite"
	record.Task = "Choose a storage engine"
	record.Summary = "Postgres gives concurrent access and full-text search"
	record.ChosenApproach = strptr("PostgreSQL in Docker")
	record.Rationale = strptr("SQLite locks the whole file on write")
	record.CreatedBy = "store-test"
	return record
}

func setRecordStatus(ctx context.Context, t *testing.T, s *Store, id string, status model.RecordStatus) {
	t.Helper()
	if _, err := s.pool.Exec(ctx,
		`UPDATE records SET status = $2::record_status WHERE id = $1`, id, string(status),
	); err != nil {
		t.Fatalf("setting status of %s to %s: %v", id, status, err)
	}
}

func countRows(ctx context.Context, t *testing.T, s *Store, sql string) int {
	t.Helper()
	var count int
	if err := s.pool.QueryRow(ctx, sql).Scan(&count); err != nil {
		t.Fatalf("counting rows: %v", err)
	}
	return count
}

func strptr(s string) *string { return &s }
