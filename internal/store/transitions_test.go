package store

import (
	"context"
	"errors"
	"testing"

	"github.com/trnahnh/recap/internal/model"
)

func TestApproveMakesDraftRetrievable(t *testing.T) {
	s, ctx := newTestStore(t)
	project := seedProject(ctx, t, s)

	created, err := s.CreateRecord(ctx, draftRecord(project.ID))
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	before, err := s.ListRetrievableRecords(ctx, project.ID, RecordFilter{})
	if err != nil {
		t.Fatalf("list before approve: %v", err)
	}
	if len(before) != 0 {
		t.Fatalf("draft must not be retrievable, got %d records", len(before))
	}

	approved, err := s.Approve(ctx, project.ID, created.ID)
	if err != nil {
		t.Fatalf("approve: %v", err)
	}
	if approved.Status != model.RecordStatusActive {
		t.Fatalf("approved record has status %q, want active", approved.Status)
	}
	if !approved.UpdatedAt.After(created.UpdatedAt) {
		t.Fatalf("updated_at %v did not advance past %v", approved.UpdatedAt, created.UpdatedAt)
	}

	after, err := s.ListRetrievableRecords(ctx, project.ID, RecordFilter{})
	if err != nil {
		t.Fatalf("list after approve: %v", err)
	}
	if len(after) != 1 || after[0].ID != created.ID {
		t.Fatalf("expected only %s to be retrievable, got %+v", created.ID, after)
	}
}

func TestRetrievableExcludesEveryNonActiveStatus(t *testing.T) {
	s, ctx := newTestStore(t)
	project := seedProject(ctx, t, s)

	hidden := []model.RecordStatus{
		model.RecordStatusDraft,
		model.RecordStatusSuperseded,
		model.RecordStatusArchived,
		model.RecordStatusInvalid,
	}
	for _, status := range hidden {
		created, err := s.CreateRecord(ctx, draftRecord(project.ID))
		if err != nil {
			t.Fatalf("create %s: %v", status, err)
		}
		setRecordStatus(ctx, t, s, created.ID, status)
	}
	visible, err := s.CreateRecord(ctx, draftRecord(project.ID))
	if err != nil {
		t.Fatalf("create active: %v", err)
	}
	if _, err := s.Approve(ctx, project.ID, visible.ID); err != nil {
		t.Fatalf("approve: %v", err)
	}

	retrievable, err := s.ListRetrievableRecords(ctx, project.ID,
		RecordFilter{Statuses: hidden})
	if err != nil {
		t.Fatalf("list retrievable: %v", err)
	}
	if len(retrievable) != 1 || retrievable[0].ID != visible.ID {
		t.Fatalf("expected only the active record %s, got %+v", visible.ID, retrievable)
	}
}

func TestApproveRejectsNonDraft(t *testing.T) {
	s, ctx := newTestStore(t)
	project := seedProject(ctx, t, s)

	for _, status := range []model.RecordStatus{
		model.RecordStatusActive,
		model.RecordStatusSuperseded,
		model.RecordStatusArchived,
		model.RecordStatusInvalid,
	} {
		created, err := s.CreateRecord(ctx, draftRecord(project.ID))
		if err != nil {
			t.Fatalf("create %s: %v", status, err)
		}
		setRecordStatus(ctx, t, s, created.ID, status)

		_, err = s.Approve(ctx, project.ID, created.ID)
		if !errors.Is(err, ErrIllegalTransition) {
			t.Fatalf("approve from %s: expected ErrIllegalTransition, got %v", status, err)
		}

		got, err := s.GetRecord(ctx, project.ID, created.ID)
		if err != nil {
			t.Fatalf("get: %v", err)
		}
		if got.Status != status {
			t.Fatalf("status changed to %q after a rejected approve, want %q", got.Status, status)
		}
	}
}

func TestArchiveAndInvalidateTransitions(t *testing.T) {
	s, ctx := newTestStore(t)
	project := seedProject(ctx, t, s)

	cases := []struct {
		name   string
		from   model.RecordStatus
		apply  func(projectID, recordID string) (model.Record, error)
		want   model.RecordStatus
		wantOK bool
	}{
		{"archive draft", model.RecordStatusDraft, s.archiveFn(ctx), model.RecordStatusArchived, true},
		{"archive active", model.RecordStatusActive, s.archiveFn(ctx), model.RecordStatusArchived, true},
		{"archive superseded", model.RecordStatusSuperseded, s.archiveFn(ctx), model.RecordStatusArchived, true},
		{"archive archived", model.RecordStatusArchived, s.archiveFn(ctx), model.RecordStatusArchived, false},
		{"archive invalid", model.RecordStatusInvalid, s.archiveFn(ctx), model.RecordStatusInvalid, false},
		{"invalidate draft", model.RecordStatusDraft, s.invalidateFn(ctx), model.RecordStatusInvalid, true},
		{"invalidate active", model.RecordStatusActive, s.invalidateFn(ctx), model.RecordStatusInvalid, true},
		{"invalidate superseded", model.RecordStatusSuperseded, s.invalidateFn(ctx), model.RecordStatusInvalid, true},
		{"invalidate archived", model.RecordStatusArchived, s.invalidateFn(ctx), model.RecordStatusArchived, false},
		{"invalidate invalid", model.RecordStatusInvalid, s.invalidateFn(ctx), model.RecordStatusInvalid, false},
	}
	for _, tc := range cases {
		created, err := s.CreateRecord(ctx, draftRecord(project.ID))
		if err != nil {
			t.Fatalf("%s: create: %v", tc.name, err)
		}
		setRecordStatus(ctx, t, s, created.ID, tc.from)

		result, err := tc.apply(project.ID, created.ID)
		if tc.wantOK {
			if err != nil {
				t.Fatalf("%s: unexpected error: %v", tc.name, err)
			}
			if result.Status != tc.want {
				t.Fatalf("%s: got status %q, want %q", tc.name, result.Status, tc.want)
			}
		} else if !errors.Is(err, ErrIllegalTransition) {
			t.Fatalf("%s: expected ErrIllegalTransition, got %v", tc.name, err)
		}

		got, err := s.GetRecord(ctx, project.ID, created.ID)
		if err != nil {
			t.Fatalf("%s: get: %v", tc.name, err)
		}
		if got.Status != tc.want {
			t.Fatalf("%s: stored status is %q, want %q", tc.name, got.Status, tc.want)
		}
	}
}

func TestTransitionNotFoundAndProjectScoped(t *testing.T) {
	s, ctx := newTestStore(t)
	project := seedProject(ctx, t, s)
	other, err := s.CreateProject(ctx, "other", t.TempDir())
	if err != nil {
		t.Fatalf("create other project: %v", err)
	}

	created, err := s.CreateRecord(ctx, draftRecord(project.ID))
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	missing := "00000000-0000-0000-0000-000000000000"
	if _, err := s.Approve(ctx, project.ID, missing); !errors.Is(err, ErrNotFound) {
		t.Fatalf("approve missing: expected ErrNotFound, got %v", err)
	}
	if _, err := s.Approve(ctx, other.ID, created.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("approve from another project: expected ErrNotFound, got %v", err)
	}
	if _, err := s.Archive(ctx, other.ID, created.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("archive from another project: expected ErrNotFound, got %v", err)
	}
}

func (s *Store) archiveFn(ctx context.Context) func(projectID, recordID string) (model.Record, error) {
	return func(projectID, recordID string) (model.Record, error) {
		return s.Archive(ctx, projectID, recordID)
	}
}

func (s *Store) invalidateFn(ctx context.Context) func(projectID, recordID string) (model.Record, error) {
	return func(projectID, recordID string) (model.Record, error) {
		return s.Invalidate(ctx, projectID, recordID)
	}
}
