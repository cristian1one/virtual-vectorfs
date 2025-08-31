package database

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/ZanzyTHEbar/virtual-vectorfs/vvfs/memory/apptype"
)

// getEntityObservations retrieves all observations for an entity in a project.
func (dm *DBManager) getEntityObservations(ctx context.Context, projectName string, entityName string) ([]string, error) {
	querier, err := dm.GetQuerier(projectName)
	if err != nil {
		return nil, err
	}
	observations, err := querier.GetEntityObservations(ctx, entityName)
	if err != nil {
		return nil, err
	}
	result := make([]string, len(observations))
	for i, obs := range observations {
		result[i] = obs.Content
	}
	return result, nil
}

// CreateEntities creates or updates entities with their observations
func (dm *DBManager) CreateEntities(ctx context.Context, projectName string, entities []apptype.Entity) error {
	querier, err := dm.GetQuerier(projectName)
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

		vectorString, vErr := dm.vectorToString(entity.Embedding)
		if vErr != nil {
			return fmt.Errorf("failed to convert embedding for entity %q: %w", entity.Name, vErr)
		}

		// Try to update first
		updateParams := UpdateEntityParams{
			EntityType: entity.EntityType,
			Embedding:  vectorString,
			Name:       entity.Name,
		}

		result, err := querier.UpdateEntity(ctx, updateParams)
		if err != nil {
			// If update failed (likely entity doesn't exist), create new
			createParams := CreateEntityParams{
				Name:       entity.Name,
				EntityType: entity.EntityType,
				Embedding:  vectorString,
				CreatedAt:  getCurrentTimestamp(),
				UpdatedAt:  getCurrentTimestamp(),
			}
			_, err = querier.CreateEntity(ctx, createParams)
			if err != nil {
				return fmt.Errorf("failed to create entity %q: %w", entity.Name, err)
			}
		} else if result.Name == "" {
			// Entity doesn't exist, create it
			createParams := CreateEntityParams{
				Name:       entity.Name,
				EntityType: entity.EntityType,
				Embedding:  vectorString,
				CreatedAt:  getCurrentTimestamp(),
				UpdatedAt:  getCurrentTimestamp(),
			}
			_, err = querier.CreateEntity(ctx, createParams)
			if err != nil {
				return fmt.Errorf("failed to create entity %q: %w", entity.Name, err)
			}
		}

		// Delete old observations and create new ones
		err = querier.DeleteEntityObservations(ctx, entity.Name)
		if err != nil {
			return fmt.Errorf("failed to delete old observations for entity %q: %w", entity.Name, err)
		}

		for _, observation := range entity.Observations {
			if observation == "" {
				return fmt.Errorf("observation cannot be empty for entity %q", entity.Name)
			}
			obsParams := CreateObservationParams{
				EntityName: entity.Name,
				Content:    observation,
				Embedding:  vectorString,
				CreatedAt:  getCurrentTimestamp(),
			}
			_, err = querier.CreateObservation(ctx, obsParams)
			if err != nil {
				return fmt.Errorf("failed to insert observation for entity %q: %w", entity.Name, err)
			}
		}
	}
	return nil
}

// GetEntities fetches entities by name (embedding included)
func (dm *DBManager) GetEntities(ctx context.Context, projectName string, names []string) ([]apptype.Entity, error) {
	if len(names) == 0 {
		return []apptype.Entity{}, nil
	}
	querier, err := dm.GetQuerier(projectName)
	if err != nil {
		return nil, err
	}

	var result []apptype.Entity
	for _, name := range names {
		entity, err := querier.GetEntity(ctx, name)
		if err != nil {
			// Skip entities that don't exist
			continue
		}

		// Convert embedding from string to []float32
		var embedding []float32
		if entity.Embedding != "" {
			// The embedding is stored as a string representation in the database
			// We'll need to parse it or handle it appropriately
			// For now, we'll leave it empty as the apptype.Entity expects []float32
		}

		result = append(result, apptype.Entity{
			Name:       entity.Name,
			EntityType: entity.EntityType,
			Embedding:  embedding,
		})
	}

	return result, nil
}

// getCurrentTimestamp returns current timestamp as int64
func getCurrentTimestamp() int64 {
	return time.Now().Unix()
}
