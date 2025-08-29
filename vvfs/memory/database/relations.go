package database

import (
	"context"
	"fmt"
	"strings"

	"github.com/ZanzyTHEbar/virtual-vectorfs/vvfs/memory/apptype"
)

// CreateRelations creates multiple relations between entities
func (dm *DBManager) CreateRelations(ctx context.Context, projectName string, relations []apptype.Relation) error {
	db, err := dm.getDB(projectName)
	if err != nil {
		return err
	}
	if len(relations) == 0 {
		return nil
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()
	stmt, err := tx.PrepareContext(ctx, "INSERT INTO relations (source, target, relation_type) VALUES (?, ?, ?)")
	if err != nil {
		return fmt.Errorf("failed to prepare statement: %w", err)
	}
	defer stmt.Close()
	for _, r := range relations {
		if r.From == "" || r.To == "" || r.RelationType == "" {
			return fmt.Errorf("relation fields cannot be empty")
		}
		if _, err := stmt.ExecContext(ctx, r.From, r.To, r.RelationType); err != nil {
			return fmt.Errorf("failed to insert relation (%s -> %s): %w", r.From, r.To, err)
		}
	}
	return tx.Commit()
}

// UpdateRelations updates relation tuples via delete/insert
func (dm *DBManager) UpdateRelations(ctx context.Context, projectName string, updates []apptype.UpdateRelationChange) error {
	db, err := dm.getDB(projectName)
	if err != nil {
		return err
	}
	if len(updates) == 0 {
		return nil
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()
	for _, up := range updates {
		nf := strings.TrimSpace(up.NewFrom)
		if nf == "" {
			nf = strings.TrimSpace(up.From)
		}
		nt := strings.TrimSpace(up.NewTo)
		if nt == "" {
			nt = strings.TrimSpace(up.To)
		}
		nr := strings.TrimSpace(up.NewRelationType)
		if nr == "" {
			nr = strings.TrimSpace(up.RelationType)
		}
		if nf == "" || nt == "" || nr == "" {
			return fmt.Errorf("relation endpoints and type cannot be empty")
		}
		if _, err := tx.ExecContext(ctx, "DELETE FROM relations WHERE source = ? AND target = ? AND relation_type = ?", up.From, up.To, up.RelationType); err != nil {
			return fmt.Errorf("failed to delete old relation: %w", err)
		}
		if _, err := tx.ExecContext(ctx, "INSERT INTO relations (source, target, relation_type) VALUES (?, ?, ?)", nf, nt, nr); err != nil {
			return fmt.Errorf("failed to insert new relation: %w", err)
		}
	}
	return tx.Commit()
}
