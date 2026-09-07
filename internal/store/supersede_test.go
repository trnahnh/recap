package store

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"

	"github.com/trnahnh/recap/internal/model"
)

func TestSupersedeReplacesActiveRecord(t *testing.T) {
	s, ctx := newTestStore(t)
	project := seedProject(ctx, t, s)

	old := seedActiveRecord(ctx, t, s, project.ID)
	replacement, err := s.CreateRecord(ctx, draftRecord(project.ID))
	if err != nil {
		t.Fatalf("create replacement: %v", err)
	}

	got, err := s.Supersede(ctx, project.ID, old.ID, replacement.ID)
	if err != nil {
		t.Fatalf("supersede: %v", err)
	}
	if got.ID != replacement.ID || got.Status != model.RecordStatusActive {
		t.Fatalf("expected the replacement %s to come back active, got %+v", replacement.ID, got)
	}
	if len(got.Relationships) != 1 ||
		got.Relationships[0].TargetRecordID != old.ID ||
		got.Relationships[0].RelationshipType != model.RelationshipSupersedes {
		t.Fatalf("expected one supersedes relationship to %s, got %+v", old.ID, got.Relationships)
	}

	previous, err := s.GetRecord(ctx, project.ID, old.ID)
	if err != nil {
		t.Fatalf("get old: %v", err)
	}
	if previous.Status != model.RecordStatusSuperseded {
		t.Fatalf("old record is %q, want superseded", previous.Status)
	}
	if len(previous.IncomingRelationships) != 1 || previous.IncomingRelationships[0].RecordID != replacement.ID {
		t.Fatalf("expected the old record to be pointed at by %s, got %+v",
			replacement.ID, previous.IncomingRelationships)
	}

	retrievable, err := s.ListRetrievableRecords(ctx, project.ID, RecordFilter{})
	if err != nil {
		t.Fatalf("list retrievable: %v", err)
	}
	if len(retrievable) != 1 || retrievable[0].ID != replacement.ID {
		t.Fatalf("expected only the replacement to be retrievable, got %+v", retrievable)
	}
}

func TestSupersedeRaceHasExactlyOneWinner(t *testing.T) {
	s, ctx := newTestStore(t)
	project := seedProject(ctx, t, s)

	old := seedActiveRecord(ctx, t, s, project.ID)
	contenders := make([]model.Record, 2)
	for i := range contenders {
		created, err := s.CreateRecord(ctx, draftRecord(project.ID))
		if err != nil {
			t.Fatalf("create contender %d: %v", i, err)
		}
		contenders[i] = created
	}

	results := make([]error, len(contenders))
	var wg sync.WaitGroup
	for i := range contenders {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_, results[i] = s.Supersede(ctx, project.ID, old.ID, contenders[i].ID)
		}(i)
	}
	wg.Wait()

	winners, losers := 0, 0
	var winner, loserErr string
	for i, err := range results {
		switch {
		case err == nil:
			winners++
			winner = contenders[i].ID
		case errors.Is(err, ErrAlreadySuperseded):
			losers++
			loserErr = err.Error()
		default:
			t.Fatalf("contender %d failed with an unexpected error: %v", i, err)
		}
	}
	if winners != 1 || losers != 1 {
		t.Fatalf("expected exactly one winner and one loser, got %d winners, %d losers", winners, losers)
	}
	if !strings.Contains(loserErr, winner) {
		t.Fatalf("loser error %q does not name the winning record %s", loserErr, winner)
	}

	previous, err := s.GetRecord(ctx, project.ID, old.ID)
	if err != nil {
		t.Fatalf("get old: %v", err)
	}
	if previous.Status != model.RecordStatusSuperseded {
		t.Fatalf("old record is %q, want superseded", previous.Status)
	}
	if len(previous.IncomingRelationships) != 1 || previous.IncomingRelationships[0].RecordID != winner {
		t.Fatalf("expected exactly one supersedes relationship from the winner %s, got %+v",
			winner, previous.IncomingRelationships)
	}
	for i, c := range contenders {
		got, err := s.GetRecord(ctx, project.ID, c.ID)
		if err != nil {
			t.Fatalf("get contender %d: %v", i, err)
		}
		want := model.RecordStatusDraft
		if c.ID == winner {
			want = model.RecordStatusActive
		}
		if got.Status != want {
			t.Fatalf("contender %s is %q, want %q", c.ID, got.Status, want)
		}
	}
}

func TestSupersedeRejectsBadInputs(t *testing.T) {
	s, ctx := newTestStore(t)
	project := seedProject(ctx, t, s)
	missing := "00000000-0000-0000-0000-000000000000"

	active := seedActiveRecord(ctx, t, s, project.ID)
	draft, err := s.CreateRecord(ctx, draftRecord(project.ID))
	if err != nil {
		t.Fatalf("create draft: %v", err)
	}
	archived, err := s.CreateRecord(ctx, draftRecord(project.ID))
	if err != nil {
		t.Fatalf("create archived: %v", err)
	}
	setRecordStatus(ctx, t, s, archived.ID, model.RecordStatusArchived)

	if _, err := s.Supersede(ctx, project.ID, active.ID, active.ID); !errors.Is(err, ErrIllegalTransition) {
		t.Fatalf("self-supersede: expected ErrIllegalTransition, got %v", err)
	}
	if _, err := s.Supersede(ctx, project.ID, missing, draft.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("missing old: expected ErrNotFound, got %v", err)
	}
	if _, err := s.Supersede(ctx, project.ID, active.ID, missing); !errors.Is(err, ErrNotFound) {
		t.Fatalf("missing new: expected ErrNotFound, got %v", err)
	}
	if _, err := s.Supersede(ctx, project.ID, draft.ID, active.ID); !errors.Is(err, ErrIllegalTransition) {
		t.Fatalf("supersede a draft: expected ErrIllegalTransition, got %v", err)
	}
	if _, err := s.Supersede(ctx, project.ID, active.ID, archived.ID); !errors.Is(err, ErrIllegalTransition) {
		t.Fatalf("archived replacement: expected ErrIllegalTransition, got %v", err)
	}

	got, err := s.GetRecord(ctx, project.ID, active.ID)
	if err != nil {
		t.Fatalf("get active: %v", err)
	}
	if got.Status != model.RecordStatusActive {
		t.Fatalf("active record is %q after rejected supersedes, want active", got.Status)
	}
}

func seedActiveRecord(ctx context.Context, t *testing.T, s *Store, projectID string) model.Record {
	t.Helper()
	created, err := s.CreateRecord(ctx, draftRecord(projectID))
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	approved, err := s.Approve(ctx, projectID, created.ID)
	if err != nil {
		t.Fatalf("approve: %v", err)
	}
	return approved
}
