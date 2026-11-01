package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/takayoshi-u/polaris/backend/internal/domain"
	"github.com/takayoshi-u/polaris/backend/internal/pkg/id"
)

// ListTypes returns item types.
// If projectID is nil, returns system-wide types only (project_id IS NULL).
// If projectID is not nil, returns both project-specific and system-wide types.
func (s *Store) ListTypes(ctx context.Context, projectID *string) ([]domain.ItemType, error) {
	var rows *sql.Rows
	var err error

	if projectID == nil {
		rows, err = s.db.QueryContext(ctx, `
			SELECT id, project_id, name, color, display_order, created_at
			FROM types
			WHERE project_id IS NULL
			ORDER BY display_order ASC, name ASC
		`)
	} else {
		rows, err = s.db.QueryContext(ctx, `
			SELECT id, project_id, name, color, display_order, created_at
			FROM types
			WHERE project_id = $1 OR project_id IS NULL
			ORDER BY project_id NULLS FIRST, display_order ASC, name ASC
		`, *projectID)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	types := []domain.ItemType{}
	for rows.Next() {
		var it domain.ItemType
		var pid sql.NullString
		if err := rows.Scan(&it.ID, &pid, &it.Name, &it.Color, &it.DisplayOrder, &it.CreatedAt); err != nil {
			return nil, err
		}
		if pid.Valid {
			it.ProjectID = &pid.String
		}
		types = append(types, it)
	}
	return types, rows.Err()
}

// GetType retrieves a single item type by ID.
func (s *Store) GetType(ctx context.Context, typeID string) (*domain.ItemType, error) {
	var it domain.ItemType
	var pid sql.NullString
	err := s.db.QueryRowContext(ctx, `
		SELECT id, project_id, name, color, display_order, created_at
		FROM types
		WHERE id = $1
	`, typeID).Scan(&it.ID, &pid, &it.Name, &it.Color, &it.DisplayOrder, &it.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	if pid.Valid {
		it.ProjectID = &pid.String
	}
	return &it, nil
}

// CreateType creates a new item type.
func (s *Store) CreateType(ctx context.Context, projectID *string, name, color string, displayOrder int) (*domain.ItemType, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, fmt.Errorf("%w: type name is required", ErrBadRequest)
	}
	if color == "" {
		color = "#6366f1"
	}
	typeID := id.NewV7String()
	now := time.Now().UTC()

	_, err := s.db.ExecContext(ctx, `
		INSERT INTO types (id, project_id, name, color, display_order, created_at)
		VALUES ($1, $2, $3, $4, $5, $6)
	`, typeID, projectID, name, color, displayOrder, now)
	if err != nil {
		return nil, err
	}

	return &domain.ItemType{
		ID:           typeID,
		ProjectID:    projectID,
		Name:         name,
		Color:        color,
		DisplayOrder: displayOrder,
		CreatedAt:    now,
	}, nil
}

// UpdateType updates an existing item type.
func (s *Store) UpdateType(ctx context.Context, typeID string, name, color string, displayOrder *int) (*domain.ItemType, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, fmt.Errorf("%w: type name is required", ErrBadRequest)
	}
	if color == "" {
		color = "#6366f1"
	}

	existing, err := s.GetType(ctx, typeID)
	if err != nil {
		return nil, err
	}

	order := existing.DisplayOrder
	if displayOrder != nil {
		order = *displayOrder
	}

	res, err := s.db.ExecContext(ctx, `
		UPDATE types
		SET name = $1, color = $2, display_order = $3
		WHERE id = $4
	`, name, color, order, typeID)
	if err != nil {
		return nil, err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return nil, ErrNotFound
	}

	existing.Name = name
	existing.Color = color
	existing.DisplayOrder = order
	return existing, nil
}

// DeleteType deletes an item type by ID.
func (s *Store) DeleteType(ctx context.Context, typeID string) error {
	res, err := s.db.ExecContext(ctx, `DELETE FROM types WHERE id = $1`, typeID)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}
