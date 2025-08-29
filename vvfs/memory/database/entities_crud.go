package database

import (
	"context"
	"fmt"
	"strings"

	"github.com/ZanzyTHEbar/virtual-vectorfs/vvfs/memory/apptype"
)

// getEntityObservations retrieves all observations for an entity in a project.
func (dm *DBManager) getEntityObservations(ctx context.Context, projectName string, entityName string) ([]string, error) {
	db, err := dm.getDB(projectName)
	if err != nil {
		return nil, err
	}
	stmt, err := dm.getPreparedStmt(ctx, projectName, db, "SELECT content FROM observations WHERE entity_name = ? ORDER BY id")
	if err != nil {
		return nil, err
	}
	rows, err := stmt.QueryContext(ctx, entityName)
	if err != nil {
		return nil, fmt.Errorf("failed to query observations: %w", err)
	}
	defer rows.Close()
	var observations []string
	for rows.Next() {
		var content string
		if err := rows.Scan(&content); err != nil {
			return nil, fmt.Errorf("failed to scan observation: %w", err)
		}
		observations = append(observations, content)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return observations, nil
}

// CreateEntities creates or updates entities with their observations
func (dm *DBManager) CreateEntities(ctx context.Context, projectName string, entities []apptype.Entity) error {
	db, err := dm.getDB(projectName)
	if err != nil {
		return err
	}
	for _, entity := range entities {
		if strings.TrimSpace(entity.Name) == "" {
			return fmt.Errorf("entity name must be a non-empty string")
		}
		if strings.TrimSpace(entity.EntityType) == "" {
			return fmt.Errorf("invalid entity type for entity %q", entity.Name)
		}
		if len(entity.Observations) == 0 {
			return fmt.Errorf("entity %q must have at least one observation", entity.Name)
		}

		tx, err := db.BeginTx(ctx, nil)
		if err != nil {
			return fmt.Errorf("failed to begin transaction for entity %q: %w", entity.Name, err)
		}
		var commitErr error
		defer func() {
			if commitErr != nil {
				_ = tx.Rollback()
			}
		}()

		vectorString, vErr := dm.vectorToString(entity.Embedding)
		if vErr != nil {
			return fmt.Errorf("failed to convert embedding for entity %q: %w", entity.Name, vErr)
		}
		result, uErr := tx.ExecContext(ctx,
			"UPDATE entities SET entity_type = ?, embedding = vector32(?) WHERE name = ?",
			entity.EntityType, vectorString, entity.Name)
		if uErr != nil {
			return fmt.Errorf("failed to update entity %q: %w", entity.Name, uErr)
		}
		rowsAffected, raErr := result.RowsAffected()
		if raErr != nil {
			return fmt.Errorf("failed to get rows affected for update: %w", raErr)
		}
		if rowsAffected == 0 {
			if _, iErr := tx.ExecContext(ctx,
				"INSERT INTO entities (name, entity_type, embedding) VALUES (?, ?, vector32(?))",
				entity.Name, entity.EntityType, vectorString); iErr != nil {
				return fmt.Errorf("failed to insert entity %q: %w", entity.Name, iErr)
			}
		}
		if _, dErr := tx.ExecContext(ctx, "DELETE FROM observations WHERE entity_name = ?", entity.Name); dErr != nil {
			return fmt.Errorf("failed to delete old observations for entity %q: %w", entity.Name, dErr)
		}
		for _, observation := range entity.Observations {
			if observation == "" {
				return fmt.Errorf("observation cannot be empty for entity %q", entity.Name)
			}
			if _, oErr := tx.ExecContext(ctx,
				"INSERT INTO observations (entity_name, content) VALUES (?, ?)",
				entity.Name, observation); oErr != nil {
				return fmt.Errorf("failed to insert observation for entity %q: %w", entity.Name, oErr)
			}
		}
		commitErr = tx.Commit()
		if commitErr != nil {
			return commitErr
		}
	}
	return nil
}

// GetEntities fetches entities by name (embedding included)
func (dm *DBManager) GetEntities(ctx context.Context, projectName string, names []string) ([]apptype.Entity, error) {
	if len(names) == 0 {
		return []apptype.Entity{}, nil
	}
	db, err := dm.getDB(projectName)
	if err != nil {
		return nil, err
	}
	// Build IN clause
	placeholders := make([]string, len(names))
	args := make([]interface{}, len(names))
	for i, n := range names {
		placeholders[i] = "?"
		args[i] = n
	}
	query := fmt.Sprintf("SELECT name, entity_type, embedding FROM entities WHERE name IN (%s)", strings.Join(placeholders, ","))
	rows, err := db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var ents []apptype.Entity
	for rows.Next() {
		var name, et string
		var emb []byte
		if err := rows.Scan(&name, &et, &emb); err != nil {
			return nil, err
		}
		vec, _ := dm.ExtractVector(ctx, emb)
		ents = append(ents, apptype.Entity{Name: name, EntityType: et, Embedding: vec})
	}
	return ents, nil
}
