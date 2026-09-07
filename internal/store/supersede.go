package store

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"

	"github.com/trnahnh/recap/internal/model"
)

func (s *Store) Supersede(ctx context.Context, projectID, oldID, newID string) (model.Record, error) {
	if oldID == newID {
		return model.Record{}, fmt.Errorf("%w: a record cannot supersede itself", ErrIllegalTransition)
	}

	var replacement model.Record
	err := s.withTx(ctx, func(tx pgx.Tx) error {
		oldStatus, err := lockRecordStatus(ctx, tx, projectID, oldID)
		if err != nil {
			return fmt.Errorf("superseded record: %w", err)
		}
		if oldStatus == model.RecordStatusSuperseded {
			winner, err := supersededBy(ctx, tx, oldID)
			if err != nil {
				return err
			}
			return fmt.Errorf("%w: record %s was already superseded by record %s; rerun against the current record",
				ErrAlreadySuperseded, oldID, winner)
		}
		if oldStatus != model.RecordStatusActive {
			return fmt.Errorf("%w: record %s is %s, only active records can be superseded",
				ErrIllegalTransition, oldID, oldStatus)
		}

		newStatus, err := lockRecordStatus(ctx, tx, projectID, newID)
		if err != nil {
			return fmt.Errorf("replacement record: %w", err)
		}
		if newStatus != model.RecordStatusDraft && newStatus != model.RecordStatusActive {
			return fmt.Errorf("%w: replacement record %s is %s, must be draft or active",
				ErrIllegalTransition, newID, newStatus)
		}

		if _, err := tx.Exec(ctx,
			`UPDATE records SET status = $3 WHERE project_id = $1 AND id = $2`,
			projectID, oldID, model.RecordStatusSuperseded,
		); err != nil {
			return fmt.Errorf("store: marking record superseded: %w", err)
		}
		if _, err := tx.Exec(ctx,
			`INSERT INTO relationships (record_id, target_record_id, relationship_type)
			 VALUES ($1, $2, $3)`,
			newID, oldID, model.RelationshipSupersedes,
		); err != nil {
			return fmt.Errorf("store: recording supersedes relationship: %w", err)
		}

		rows, err := tx.Query(ctx,
			`UPDATE records SET status = $3
			 WHERE project_id = $1 AND id = $2
			 RETURNING `+recordColumns,
			projectID, newID, model.RecordStatusActive,
		)
		if err != nil {
			return fmt.Errorf("store: activating replacement record: %w", err)
		}
		replacement, err = pgx.CollectExactlyOneRow(rows, pgx.RowToStructByNameLax[model.Record])
		if err != nil {
			return fmt.Errorf("store: scanning replacement record: %w", err)
		}
		return loadChildren(ctx, tx, &replacement)
	})
	if err != nil {
		return model.Record{}, err
	}
	return replacement, nil
}

func supersededBy(ctx context.Context, tx pgx.Tx, oldID string) (string, error) {
	var winner string
	err := tx.QueryRow(ctx,
		`SELECT record_id FROM relationships
		 WHERE target_record_id = $1 AND relationship_type = $2
		 ORDER BY created_at DESC, id DESC
		 LIMIT 1`,
		oldID, model.RelationshipSupersedes,
	).Scan(&winner)
	if errors.Is(err, pgx.ErrNoRows) {
		return "<unknown>", nil
	}
	if err != nil {
		return "", fmt.Errorf("store: finding superseding record: %w", err)
	}
	return winner, nil
}
