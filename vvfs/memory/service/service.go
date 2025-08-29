package service

import (
	"context"

	"github.com/ZanzyTHEbar/virtual-vectorfs/vvfs/memory/apptype"
	"github.com/ZanzyTHEbar/virtual-vectorfs/vvfs/memory/database"
)

// Service provides a library-first API over the internal memory database
type Service struct {
	db *database.DBManager
}

func NewService(cfg *Config) (*Service, error) {
	dm, err := database.NewDBManager(cfg.ToInternal())
	if err != nil {
		return nil, err
	}
	return &Service{db: dm}, nil
}

func (s *Service) Close() error { return s.db.Close() }

// CreateEntities inserts or updates entities and their observations
func (s *Service) CreateEntities(ctx context.Context, project string, ents []apptype.Entity) error {
	return s.db.CreateEntities(ctx, project, ents)
}

// CreateRelations inserts relations
func (s *Service) CreateRelations(ctx context.Context, project string, rels []apptype.Relation) error {
	return s.db.CreateRelations(ctx, project, rels)
}

// SearchText uses FTS5/LIKE
func (s *Service) SearchText(ctx context.Context, project, query string, limit, offset int) ([]apptype.Entity, []apptype.Relation, error) {
	return s.db.SearchEntities(ctx, project, query, limit, offset)
}

// SearchVector uses ANN operator or fallback
func (s *Service) SearchVector(ctx context.Context, project string, vector []float32, limit, offset int) ([]apptype.Entity, []apptype.Relation, error) {
	res, err := s.db.SearchSimilar(ctx, project, vector, limit, offset)
	if err != nil {
		return nil, nil, err
	}
	ents := make([]apptype.Entity, len(res))
	for i, r := range res {
		ents[i] = r.Entity
	}
	rels, err := s.db.GetRelationsForEntities(ctx, project, ents)
	if err != nil {
		return nil, nil, err
	}
	return ents, rels, nil
}

// OpenNodes fetches entities by names and optionally their relations
func (s *Service) OpenNodes(ctx context.Context, project string, names []string, includeRelations bool) ([]apptype.Entity, []apptype.Relation, error) {
	ents, err := s.db.GetEntities(ctx, project, names)
	if err != nil {
		return nil, nil, err
	}
	if !includeRelations {
		return ents, []apptype.Relation{}, nil
	}
	rels, err := s.db.GetRelationsForEntities(ctx, project, ents)
	if err != nil {
		return nil, nil, err
	}
	return ents, rels, nil
}
