package database

import (
	"context"
	"sort"

	"github.com/ZanzyTHEbar/virtual-vectorfs/vvfs/memory/apptype"
)

// AdvancedSearchParams for complex search operations
type AdvancedSearchParams struct {
	TextQuery   string    `json:"text_query"`
	VectorQuery []float32 `json:"vector_query"`
	EntityType  string    `json:"entity_type"`
	Limit       int       `json:"limit"`
	Offset      int       `json:"offset"`
}

// HybridSearch combines text and vector search with RRF (Reciprocal Rank Fusion)
func (dm *DBManager) HybridSearch(ctx context.Context, projectName string, query string, vector []float32, limit, offset int) ([]apptype.Entity, error) {
	// Get text search results
	textResults, _, err := dm.SearchEntities(ctx, projectName, query, limit*2, 0)
	if err != nil {
		return nil, err
	}

	// Get vector search results
	vectorResults, err := dm.SearchSimilar(ctx, projectName, vector, limit*2, 0)
	if err != nil {
		return nil, err
	}

	// Convert vector results to entities
	vectorEntities := make([]apptype.Entity, len(vectorResults))
	for i, r := range vectorResults {
		vectorEntities[i] = r.Entity
	}

	// Combine and deduplicate using RRF
	combined := dm.combineWithRRF(textResults, vectorEntities, limit+offset)

	// Apply pagination
	start := offset
	end := start + limit
	if start > len(combined) {
		start = len(combined)
	}
	if end > len(combined) {
		end = len(combined)
	}

	return combined[start:end], nil
}

// FuzzySearch performs approximate string matching
func (dm *DBManager) FuzzySearch(ctx context.Context, projectName string, query string, limit, offset int) ([]apptype.Entity, error) {
	// For now, use the existing text search
	// In a full implementation, this would use fuzzy matching algorithms
	entities, _, err := dm.SearchEntities(ctx, projectName, query, limit, offset)
	return entities, err
}

// AdvancedSearch with multiple filters and ranking
func (dm *DBManager) AdvancedSearch(ctx context.Context, projectName string, params AdvancedSearchParams) ([]apptype.Entity, error) {
	querier, err := dm.GetQuerier(projectName)
	if err != nil {
		return nil, err
	}

	var results []apptype.Entity

	if params.TextQuery != "" && len(params.VectorQuery) > 0 {
		// Hybrid search
		return dm.HybridSearch(ctx, projectName, params.TextQuery, params.VectorQuery, params.Limit, params.Offset)
	} else if params.TextQuery != "" {
		// Text search with optional entity type filter
		if params.EntityType != "" {
			entities, err := querier.SearchEntitiesByType(ctx, SearchEntitiesByTypeParams{
				EntityType: params.EntityType,
				Limit:      int64(params.Limit),
				Offset:     int64(params.Offset),
			})
			if err != nil {
				return nil, err
			}
			results = make([]apptype.Entity, len(entities))
			for i, entity := range entities {
				// Handle embedding type conversion
				var embedding []float32
				if emb, ok := entity.Embedding.([]float32); ok {
					embedding = emb
				}
				results[i] = apptype.Entity{
					Name:       entity.Name,
					EntityType: entity.EntityType,
					Embedding:  embedding,
				}
			}
		} else {
			entities, _, err := dm.SearchEntities(ctx, projectName, params.TextQuery, params.Limit, params.Offset)
			return entities, err
		}
	} else if len(params.VectorQuery) > 0 {
		// Vector search
		vectorResults, err := dm.SearchSimilar(ctx, projectName, params.VectorQuery, params.Limit, params.Offset)
		if err != nil {
			return nil, err
		}
		results = make([]apptype.Entity, len(vectorResults))
		for i, r := range vectorResults {
			results[i] = r.Entity
		}
	}

	return results, nil
}

// combineWithRRF combines text and vector results using Reciprocal Rank Fusion
func (dm *DBManager) combineWithRRF(textResults []apptype.Entity, vectorResults []apptype.Entity, maxResults int) []apptype.Entity {
	// Create a map to track scores
	scoreMap := make(map[string]float64)
	entityMap := make(map[string]apptype.Entity)

	// Add text search results with RRF scoring
	for i, entity := range textResults {
		if i >= maxResults {
			break
		}
		rank := i + 1
		score := 1.0 / float64(rank)
		scoreMap[entity.Name] += score
		entityMap[entity.Name] = entity
	}

	// Add vector search results with RRF scoring
	for i, entity := range vectorResults {
		if i >= maxResults {
			break
		}
		rank := i + 1
		score := 1.0 / float64(rank)
		scoreMap[entity.Name] += score
		entityMap[entity.Name] = entity
	}

	// Sort by combined score
	type scoredEntity struct {
		entity apptype.Entity
		score  float64
	}

	var scored []scoredEntity
	for name, score := range scoreMap {
		if entity, exists := entityMap[name]; exists {
			scored = append(scored, scoredEntity{entity: entity, score: score})
		}
	}

	sort.Slice(scored, func(i, j int) bool {
		return scored[i].score > scored[j].score
	})

	// Extract entities
	results := make([]apptype.Entity, len(scored))
	for i, s := range scored {
		results[i] = s.entity
	}

	return results
}
