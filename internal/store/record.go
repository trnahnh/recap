package store

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"

	"github.com/trnahnh/recap/internal/model"
)

const (
	recordColumns = `id, project_id, session_id, record_type, title, task, summary,
		chosen_approach, rationale, status, confidence, created_by, created_at, updated_at`
	alternativeColumns  = `id, record_id, approach, result, reason, position`
	recordFileColumns   = `id, record_id, file_path, commit_hash`
	relationshipColumns = `id, record_id, target_record_id, relationship_type, created_at`
)

type RecordFilter struct {
	Statuses    []model.RecordStatus
	RecordTypes []model.RecordType
}

func (s *Store) CreateRecord(ctx context.Context, record model.Record) (model.Record, error) {
	record.Status = model.RecordStatusDraft
	if err := record.Validate(); err != nil {
		return model.Record{}, err
	}

	var created model.Record
	err := s.withTx(ctx, func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx,
			`INSERT INTO records
			   (project_id, session_id, record_type, title, task, summary,
			    chosen_approach, rationale, status, confidence, created_by)
			 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
			 RETURNING `+recordColumns,
			record.ProjectID, record.SessionID, record.RecordType, record.Title,
			record.Task, record.Summary, record.ChosenApproach, record.Rationale,
			model.RecordStatusDraft, record.Confidence, record.CreatedBy,
		)
		if err != nil {
			return fmt.Errorf("store: inserting record: %w", err)
		}
		created, err = pgx.CollectExactlyOneRow(rows, pgx.RowToStructByNameLax[model.Record])
		if err != nil {
			return fmt.Errorf("store: scanning record: %w", err)
		}

		if created.Alternatives, err = insertAlternatives(ctx, tx, created.ID, record.Alternatives); err != nil {
			return err
		}
		if created.Files, err = insertRecordFiles(ctx, tx, created.ID, record.Files); err != nil {
			return err
		}
		created.Relationships = []model.Relationship{}
		created.IncomingRelationships = []model.Relationship{}
		return nil
	})
	if err != nil {
		return model.Record{}, err
	}
	return created, nil
}

func (s *Store) GetRecord(ctx context.Context, projectID, recordID string) (model.Record, error) {
	var record model.Record
	err := s.withTx(ctx, func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx,
			`SELECT `+recordColumns+` FROM records WHERE project_id = $1 AND id = $2`,
			projectID, recordID,
		)
		if err != nil {
			return fmt.Errorf("store: querying record: %w", err)
		}
		record, err = pgx.CollectExactlyOneRow(rows, pgx.RowToStructByNameLax[model.Record])
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		if err != nil {
			return fmt.Errorf("store: scanning record: %w", err)
		}

		if record.Alternatives, err = loadAlternatives(ctx, tx, record.ID); err != nil {
			return err
		}
		if record.Files, err = loadRecordFiles(ctx, tx, record.ID); err != nil {
			return err
		}
		return loadRelationships(ctx, tx, &record)
	})
	if err != nil {
		return model.Record{}, err
	}
	return record, nil
}

func (s *Store) UpdateRecord(ctx context.Context, record model.Record) (model.Record, error) {
	if err := record.Validate(); err != nil {
		return model.Record{}, err
	}

	var updated model.Record
	err := s.withTx(ctx, func(tx pgx.Tx) error {
		var current model.RecordStatus
		err := tx.QueryRow(ctx,
			`SELECT status FROM records WHERE project_id = $1 AND id = $2`,
			record.ProjectID, record.ID,
		).Scan(&current)
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		if err != nil {
			return fmt.Errorf("store: reading record status: %w", err)
		}
		if record.Status != current {
			return fmt.Errorf("%w: record is %s, update requested %s",
				ErrStatusChangeRejected, current, record.Status)
		}

		rows, err := tx.Query(ctx,
			`UPDATE records SET
			   session_id = $3, record_type = $4, title = $5, task = $6, summary = $7,
			   chosen_approach = $8, rationale = $9, confidence = $10
			 WHERE project_id = $1 AND id = $2
			 RETURNING `+recordColumns,
			record.ProjectID, record.ID, record.SessionID, record.RecordType, record.Title,
			record.Task, record.Summary, record.ChosenApproach, record.Rationale, record.Confidence,
		)
		if err != nil {
			return fmt.Errorf("store: updating record: %w", err)
		}
		updated, err = pgx.CollectExactlyOneRow(rows, pgx.RowToStructByNameLax[model.Record])
		if err != nil {
			return fmt.Errorf("store: scanning record: %w", err)
		}

		if _, err := tx.Exec(ctx, `DELETE FROM alternatives WHERE record_id = $1`, record.ID); err != nil {
			return fmt.Errorf("store: clearing alternatives: %w", err)
		}
		if _, err := tx.Exec(ctx, `DELETE FROM record_files WHERE record_id = $1`, record.ID); err != nil {
			return fmt.Errorf("store: clearing record files: %w", err)
		}
		if updated.Alternatives, err = insertAlternatives(ctx, tx, record.ID, record.Alternatives); err != nil {
			return err
		}
		if updated.Files, err = insertRecordFiles(ctx, tx, record.ID, record.Files); err != nil {
			return err
		}
		return loadRelationships(ctx, tx, &updated)
	})
	if err != nil {
		return model.Record{}, err
	}
	return updated, nil
}

func (s *Store) DeleteRecord(ctx context.Context, projectID, recordID string) error {
	tag, err := s.pool.Exec(ctx,
		`DELETE FROM records WHERE project_id = $1 AND id = $2`, projectID, recordID)
	if err != nil {
		return fmt.Errorf("store: deleting record: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) ListRecords(ctx context.Context, projectID string, filter RecordFilter) ([]model.Record, error) {
	sql := `SELECT ` + recordColumns + ` FROM records WHERE project_id = $1`
	args := []any{projectID}

	if len(filter.Statuses) > 0 {
		values := make([]string, len(filter.Statuses))
		for i, status := range filter.Statuses {
			values[i] = string(status)
		}
		args = append(args, values)
		sql += fmt.Sprintf(` AND status = ANY($%d::record_status[])`, len(args))
	}
	if len(filter.RecordTypes) > 0 {
		values := make([]string, len(filter.RecordTypes))
		for i, recordType := range filter.RecordTypes {
			values[i] = string(recordType)
		}
		args = append(args, values)
		sql += fmt.Sprintf(` AND record_type = ANY($%d::record_type[])`, len(args))
	}
	sql += ` ORDER BY created_at DESC, id DESC`

	rows, err := s.pool.Query(ctx, sql, args...)
	if err != nil {
		return nil, fmt.Errorf("store: listing records: %w", err)
	}
	records, err := pgx.CollectRows(rows, pgx.RowToStructByNameLax[model.Record])
	if err != nil {
		return nil, fmt.Errorf("store: scanning records: %w", err)
	}
	return records, nil
}

func insertAlternatives(ctx context.Context, tx pgx.Tx, recordID string, in []model.Alternative) ([]model.Alternative, error) {
	out := make([]model.Alternative, 0, len(in))
	for i := range in {
		rows, err := tx.Query(ctx,
			`INSERT INTO alternatives (record_id, approach, result, reason, position)
			 VALUES ($1, $2, $3, $4, $5)
			 RETURNING `+alternativeColumns,
			recordID, in[i].Approach, in[i].Result, in[i].Reason, i,
		)
		if err != nil {
			return nil, fmt.Errorf("store: inserting alternative %d: %w", i, err)
		}
		alternative, err := pgx.CollectExactlyOneRow(rows, pgx.RowToStructByNameLax[model.Alternative])
		if err != nil {
			return nil, fmt.Errorf("store: scanning alternative %d: %w", i, err)
		}
		out = append(out, alternative)
	}
	return out, nil
}

func insertRecordFiles(ctx context.Context, tx pgx.Tx, recordID string, in []model.RecordFile) ([]model.RecordFile, error) {
	out := make([]model.RecordFile, 0, len(in))
	for i := range in {
		rows, err := tx.Query(ctx,
			`INSERT INTO record_files (record_id, file_path, commit_hash)
			 VALUES ($1, $2, $3)
			 RETURNING `+recordFileColumns,
			recordID, in[i].FilePath, in[i].CommitHash,
		)
		if err != nil {
			return nil, fmt.Errorf("store: inserting record file %d: %w", i, err)
		}
		file, err := pgx.CollectExactlyOneRow(rows, pgx.RowToStructByNameLax[model.RecordFile])
		if err != nil {
			return nil, fmt.Errorf("store: scanning record file %d: %w", i, err)
		}
		out = append(out, file)
	}
	return out, nil
}

func loadAlternatives(ctx context.Context, tx pgx.Tx, recordID string) ([]model.Alternative, error) {
	rows, err := tx.Query(ctx,
		`SELECT `+alternativeColumns+` FROM alternatives WHERE record_id = $1 ORDER BY position, id`,
		recordID)
	if err != nil {
		return nil, fmt.Errorf("store: querying alternatives: %w", err)
	}
	alternatives, err := pgx.CollectRows(rows, pgx.RowToStructByNameLax[model.Alternative])
	if err != nil {
		return nil, fmt.Errorf("store: scanning alternatives: %w", err)
	}
	return alternatives, nil
}

func loadRecordFiles(ctx context.Context, tx pgx.Tx, recordID string) ([]model.RecordFile, error) {
	rows, err := tx.Query(ctx,
		`SELECT `+recordFileColumns+` FROM record_files WHERE record_id = $1 ORDER BY file_path, id`,
		recordID)
	if err != nil {
		return nil, fmt.Errorf("store: querying record files: %w", err)
	}
	files, err := pgx.CollectRows(rows, pgx.RowToStructByNameLax[model.RecordFile])
	if err != nil {
		return nil, fmt.Errorf("store: scanning record files: %w", err)
	}
	return files, nil
}

func loadRelationships(ctx context.Context, tx pgx.Tx, record *model.Record) error {
	outgoing, err := queryRelationships(ctx, tx,
		`SELECT `+relationshipColumns+` FROM relationships WHERE record_id = $1 ORDER BY created_at, id`,
		record.ID)
	if err != nil {
		return fmt.Errorf("store: loading relationships: %w", err)
	}
	incoming, err := queryRelationships(ctx, tx,
		`SELECT `+relationshipColumns+` FROM relationships WHERE target_record_id = $1 ORDER BY created_at, id`,
		record.ID)
	if err != nil {
		return fmt.Errorf("store: loading incoming relationships: %w", err)
	}
	record.Relationships = outgoing
	record.IncomingRelationships = incoming
	return nil
}

func queryRelationships(ctx context.Context, tx pgx.Tx, sql string, args ...any) ([]model.Relationship, error) {
	rows, err := tx.Query(ctx, sql, args...)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, pgx.RowToStructByNameLax[model.Relationship])
}
