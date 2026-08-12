package store

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"

	"github.com/trnahnh/recap/internal/model"
)

const projectColumns = `id, name, root_path, created_at, updated_at`

func (s *Store) CreateProject(ctx context.Context, name, rootPath string) (model.Project, error) {
	if strings.TrimSpace(name) == "" {
		return model.Project{}, fmt.Errorf("store: project name is required")
	}
	root, err := NormalizeRootPath(rootPath)
	if err != nil {
		return model.Project{}, err
	}

	created, err := s.queryProject(ctx,
		`INSERT INTO projects (name, root_path) VALUES ($1, $2)
		 ON CONFLICT (root_path) DO NOTHING
		 RETURNING `+projectColumns,
		name, root,
	)
	if err == nil {
		return created, nil
	}
	if !errors.Is(err, ErrNotFound) {
		return model.Project{}, err
	}

	existing, err := s.projectByNormalizedRoot(ctx, root)
	if err != nil {
		return model.Project{}, err
	}
	if existing.Name != name {
		return model.Project{}, fmt.Errorf(
			"%w: %s is registered as %q, not %q", ErrProjectNameMismatch, root, existing.Name, name)
	}
	return existing, nil
}

func (s *Store) GetProject(ctx context.Context, id string) (model.Project, error) {
	return s.queryProject(ctx,
		`SELECT `+projectColumns+` FROM projects WHERE id = $1`, id)
}

func (s *Store) GetProjectByRoot(ctx context.Context, rootPath string) (model.Project, error) {
	root, err := NormalizeRootPath(rootPath)
	if err != nil {
		return model.Project{}, err
	}
	return s.projectByNormalizedRoot(ctx, root)
}

func (s *Store) ListProjects(ctx context.Context) ([]model.Project, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT `+projectColumns+` FROM projects ORDER BY name, root_path`)
	if err != nil {
		return nil, fmt.Errorf("store: listing projects: %w", err)
	}
	projects, err := pgx.CollectRows(rows, pgx.RowToStructByNameLax[model.Project])
	if err != nil {
		return nil, fmt.Errorf("store: scanning projects: %w", err)
	}
	return projects, nil
}

func (s *Store) DeleteProject(ctx context.Context, id string) error {
	tag, err := s.pool.Exec(ctx, `DELETE FROM projects WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("store: deleting project: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) projectByNormalizedRoot(ctx context.Context, root string) (model.Project, error) {
	return s.queryProject(ctx,
		`SELECT `+projectColumns+` FROM projects WHERE root_path = $1`, root)
}

func (s *Store) queryProject(ctx context.Context, sql string, args ...any) (model.Project, error) {
	rows, err := s.pool.Query(ctx, sql, args...)
	if err != nil {
		return model.Project{}, fmt.Errorf("store: querying project: %w", err)
	}
	project, err := pgx.CollectExactlyOneRow(rows, pgx.RowToStructByNameLax[model.Project])
	if errors.Is(err, pgx.ErrNoRows) {
		return model.Project{}, ErrNotFound
	}
	if err != nil {
		return model.Project{}, fmt.Errorf("store: scanning project: %w", err)
	}
	return project, nil
}
