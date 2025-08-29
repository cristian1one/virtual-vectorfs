package database

import (
	"context"
	"database/sql"
	"sync"

	"github.com/ZanzyTHEbar/virtual-vectorfs/vvfs/memory/apptype"
)

// DBManager handles all database operations
type DBManager struct {
	config *Config
	dbs    map[string]*sql.DB
	mu     sync.RWMutex
	// stmtCache holds prepared statements per project DB: project -> (sql -> *Stmt)
	stmtCache map[string]map[string]*sql.Stmt
	stmtMu    sync.RWMutex
	// capsByProject holds runtime-detected optional capabilities per project
	capMu         sync.RWMutex
	capsByProject map[string]capFlags
}

// Close closes all cached prepared statements and DBs.
func (dm *DBManager) Close() error {
	// close statements
	dm.stmtMu.Lock()
	for _, cache := range dm.stmtCache {
		for _, stmt := range cache {
			_ = stmt.Close()
		}
	}
	dm.stmtCache = make(map[string]map[string]*sql.Stmt)
	dm.stmtMu.Unlock()
	// close dbs
	dm.mu.Lock()
	for name, db := range dm.dbs {
		_ = db.Close()
		delete(dm.dbs, name)
	}
	dm.mu.Unlock()
	return nil
}

// GetRelations returns all relations where either source or target belongs to the provided entity names.
func (dm *DBManager) GetRelations(ctx context.Context, projectName string, entityNames []string) ([]apptype.Relation, error) {
	if len(entityNames) == 0 {
		return []apptype.Relation{}, nil
	}
	entities := make([]apptype.Entity, len(entityNames))
	for i, n := range entityNames {
		entities[i] = apptype.Entity{Name: n}
	}
	return dm.GetRelationsForEntities(ctx, projectName, entities)
}

// ensureFTSSchema creates FTS5 virtual table and triggers if supported (best-effort)
func (dm *DBManager) ensureFTSSchema(ctx context.Context, db *sql.DB) error {
	stmts := []string{
		`DROP TRIGGER IF EXISTS trg_obs_ai`,
		`DROP TRIGGER IF EXISTS trg_obs_ad`,
		`DROP TRIGGER IF EXISTS trg_obs_au`,
		`CREATE VIRTUAL TABLE IF NOT EXISTS fts_observations USING fts5(
            entity_name,
            content,
            tokenize = 'unicode61 tokenchars=:-_@./',
            prefix = '2 3 4 5 6 7'
        )`,
		`CREATE TRIGGER IF NOT EXISTS trg_obs_ai AFTER INSERT ON observations BEGIN
            INSERT INTO fts_observations(rowid, entity_name, content) VALUES (new.id, new.entity_name, new.content);
        END;`,
		`CREATE TRIGGER IF NOT EXISTS trg_obs_ad AFTER DELETE ON observations BEGIN
            INSERT INTO fts_observations(fts_observations, rowid, entity_name, content) VALUES ('delete', old.id, old.entity_name, old.content);
        END;`,
		`CREATE TRIGGER IF NOT EXISTS trg_obs_au AFTER UPDATE ON observations BEGIN
            INSERT INTO fts_observations(fts_observations, rowid, entity_name, content) VALUES ('delete', old.id, old.entity_name, old.content);
            INSERT INTO fts_observations(rowid, entity_name, content) VALUES (new.id, new.entity_name, new.content);
        END;`,
	}
	for _, s := range stmts {
		if _, err := db.ExecContext(ctx, s); err != nil {
			return nil
		}
	}
	return nil
}
