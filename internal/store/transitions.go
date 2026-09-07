package store

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"

	"github.com/trnahnh/recap/internal/model"
)

var allowedSources = map[model.RecordStatus][]model.RecordStatus{
	model.RecordStatusActive:   {model.RecordStatusDraft},
	model.RecordStatusArchived: {model.RecordStatusDraft, model.RecordStatusActive, model.RecordStatusSuperseded},
	model.RecordStatusInvalid:  {model.RecordStatusDraft, model.RecordStatusActive, model.RecordStatusSuperseded},
}

func (s *Store) Approve(ctx context.Context, projectID, recordID string) (model.Record, error) {
	return s.transition(ctx, projectID, recordID, model.RecordStatusActive)
}

func (s *Store) Archive(ctx context.Context, projectID, recordID string) (model.Record, error) {
	return s.transition(ctx, projectID, recordID, model.RecordStatusArchived)
}

func (s *Store) Invalidate(ctx context.Context, projectID, recordID string) (model.Record, error) {
	return s.transition(ctx, projectID, recordID, model.RecordStatusInvalid)
}

func (s *Store) transition(ctx context.Context, projectID, recordID string, target model.RecordStatus) (model.Record, error) {
	var record model.Record
	err := s.withTx(ctx, func(tx pgx.Tx) error {
		current, err := lockRecordStatus(ctx, tx, projectID, recordID)
		if err != nil {
			return err
		}
		if !transitionAllowed(current, target) {
			return fmt.Errorf("%w: record %s is %s, cannot become %s",
				ErrIllegalTransition, recordID, current, target)
		}

		rows, err := tx.Query(ctx,
			`UPDATE records SET status = $3
			 WHERE project_id = $1 AND id = $2
			 RETURNING `+recordColumns,
			projectID, recordID, target,
		)
		if err != nil {
			return fmt.Errorf("store: updating record status: %w", err)
		}
		record, err = pgx.CollectExactlyOneRow(rows, pgx.RowToStructByNameLax[model.Record])
		if err != nil {
			return fmt.Errorf("store: scanning record: %w", err)
		}
		return loadChildren(ctx, tx, &record)
	})
	if err != nil {
		return model.Record{}, err
	}
	return record, nil
}

func transitionAllowed(current, target model.RecordStatus) bool {
	for _, source := range allowedSources[target] {
		if source == current {
			return true
		}
	}
	return false
}

func lockRecordStatus(ctx context.Context, tx pgx.Tx, projectID, recordID string) (model.RecordStatus, error) {
	var current model.RecordStatus
	err := tx.QueryRow(ctx,
		`SELECT status FROM records WHERE project_id = $1 AND id = $2 FOR UPDATE`,
		projectID, recordID,
	).Scan(&current)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", ErrNotFound
	}
	if err != nil {
		return "", fmt.Errorf("store: locking record: %w", err)
	}
	return current, nil
}
