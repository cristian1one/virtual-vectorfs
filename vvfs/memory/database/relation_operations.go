package database

import (
	"context"
	"fmt"
	"strings"

	"github.com/ZanzyTHEbar/virtual-vectorfs/vvfs/memory/apptype"
)

// CreateRelations creates multiple relations between entities
func (dm *DBManager) CreateRelations(ctx context.Context, projectName string, relations []apptype.Relation) error {
	querier, err := dm.GetQuerier(projectName)
	if err != nil {
		return err
	}
	if len(relations) == 0 {
		return nil
	}

	for _, r := range relations {
		if r.From == "" || r.To == "" || r.RelationType == "" {
			return fmt.Errorf("relation fields cannot be empty")
		}

		params := CreateEntityFileRelationParams{
			EntityName:   r.From,
			FileID:       r.To, // Note: This assumes To is a file ID, may need adjustment
			RelationType: r.RelationType,
			Confidence:   1.0,
			Metadata:     "",
			CreatedAt:    getCurrentTimestamp(),
		}

		_, err := querier.CreateEntityFileRelation(ctx, params)
		if err != nil {
			return fmt.Errorf("failed to create relation (%s -> %s): %w", r.From, r.To, err)
		}
	}

	return nil
}

// UpdateRelations updates relation tuples via delete/insert
func (dm *DBManager) UpdateRelations(ctx context.Context, projectName string, updates []apptype.UpdateRelationChange) error {
	querier, err := dm.GetQuerier(projectName)
	if err != nil {
		return err
	}
	if len(updates) == 0 {
		return nil
	}

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

		// Delete old relation
		deleteParams := DeleteEntityFileRelationParams{
			EntityName:   up.From,
			FileID:       up.To,
			RelationType: up.RelationType,
		}
		err := querier.DeleteEntityFileRelation(ctx, deleteParams)
		if err != nil {
			return fmt.Errorf("failed to delete old relation: %w", err)
		}

		// Create new relation
		createParams := CreateEntityFileRelationParams{
			EntityName:   nf,
			FileID:       nt,
			RelationType: nr,
			Confidence:   1.0,
			Metadata:     "",
			CreatedAt:    getCurrentTimestamp(),
		}
		_, err = querier.CreateEntityFileRelation(ctx, createParams)
		if err != nil {
			return fmt.Errorf("failed to insert new relation: %w", err)
		}
	}
	return nil
}
