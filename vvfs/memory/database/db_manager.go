// Package database implements the core database operations with sqlc + goose
package database

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	_ "github.com/tursodatabase/go-libsql"
)

const defaultProject = "default"

// DBManager handles all database operations with sqlc integration
type DBManager struct {
	config        *Config
	dbs           map[string]*sql.DB
	mu            sync.RWMutex
	stmtCache     map[string]map[string]*sql.Stmt
	stmtMu        sync.RWMutex
	capsByProject map[string]capFlags
	capMu         sync.RWMutex        // mutex for capabilities
	queries       map[string]*Queries // sqlc generated queriers
}

// NewDBManager creates a new database manager with sqlc integration
func NewDBManager(config *Config) (*DBManager, error) {
	if config.EmbeddingDims <= 0 || config.EmbeddingDims > 65536 {
		return nil, fmt.Errorf("EMBEDDING_DIMS must be between 1 and 65536 inclusive: %d", config.EmbeddingDims)
	}

	manager := &DBManager{
		config:        config,
		dbs:           make(map[string]*sql.DB),
		stmtCache:     make(map[string]map[string]*sql.Stmt),
		capsByProject: make(map[string]capFlags),
		queries:       make(map[string]*Queries),
	}

	// initialize default DB in single-project mode
	if !config.MultiProjectMode {
		if _, err := manager.getDB(defaultProject); err != nil {
			return nil, fmt.Errorf("failed to initialize default database: %w", err)
		}
	}

	return manager, nil
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

// getDB retrieves or creates a DB connection for a project
func (dm *DBManager) getDB(projectName string) (*sql.DB, error) {
	dm.mu.RLock()
	db, ok := dm.dbs[projectName]
	dm.mu.RUnlock()
	if ok {
		return db, nil
	}

	dm.mu.Lock()
	defer dm.mu.Unlock()
	if db, ok = dm.dbs[projectName]; ok {
		return db, nil
	}

	var dbURL string
	if dm.config.MultiProjectMode {
		if projectName == "" {
			return nil, fmt.Errorf("project name cannot be empty in multi-project mode")
		}
		dbPath := filepath.Join(dm.config.ProjectsDir, projectName, "libsql.db")
		if err := os.MkdirAll(filepath.Dir(dbPath), 0o755); err != nil {
			return nil, fmt.Errorf("failed to create project directory for %s: %w", projectName, err)
		}
		dbURL = fmt.Sprintf("file:%s", dbPath)
	} else {
		dbURL = dm.config.URL
	}

	var newDb *sql.DB
	var err error
	if strings.HasPrefix(dbURL, "file:") {
		newDb, err = sql.Open("libsql", dbURL)
	} else {
		authURL := dbURL
		if dm.config.AuthToken != "" {
			if u, perr := url.Parse(dbURL); perr == nil {
				q := u.Query()
				q.Set("authToken", dm.config.AuthToken)
				u.RawQuery = q.Encode()
				authURL = u.String()
			} else {
				if strings.Contains(dbURL, "?") {
					authURL = dbURL + "&authToken=" + url.QueryEscape(dm.config.AuthToken)
				} else {
					authURL = dbURL + "?authToken=" + url.QueryEscape(dm.config.AuthToken)
				}
			}
		}
		newDb, err = sql.Open("libsql", authURL)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to create database connector for project %s: %w", projectName, err)
	}

	if err := dm.initialize(newDb); err != nil {
		newDb.Close()
		return nil, fmt.Errorf("failed to initialize database for project %s: %w", projectName, err)
	}

	// pool tuning
	if dm.config.MaxOpenConns > 0 {
		newDb.SetMaxOpenConns(dm.config.MaxOpenConns)
	}
	if dm.config.MaxIdleConns > 0 {
		newDb.SetMaxIdleConns(dm.config.MaxIdleConns)
	}
	if dm.config.ConnMaxIdleSec > 0 {
		newDb.SetConnMaxIdleTime(time.Duration(dm.config.ConnMaxIdleSec) * time.Second)
	}
	if dm.config.ConnMaxLifeSec > 0 {
		newDb.SetConnMaxLifetime(time.Duration(dm.config.ConnMaxLifeSec) * time.Second)
	}

	// reconcile embedding dims with DB if needed
	if dbDims := detectDBEmbeddingDims(newDb); dbDims > 0 && dbDims != dm.config.EmbeddingDims {
		log.Printf("Embedding dims mismatch: DB=%d, Config=%d. Adopting DB dims.", dbDims, dm.config.EmbeddingDims)
		dm.config.EmbeddingDims = dbDims
	}

	dm.dbs[projectName] = newDb
	if _, ok := dm.stmtCache[projectName]; !ok {
		dm.stmtCache[projectName] = make(map[string]*sql.Stmt)
	}

	// detect caps
	dm.detectCapabilitiesForProject(context.Background(), projectName, newDb)

	// initialize sqlc querier
	dm.queries[projectName] = New(newDb)

	_ = newDb.Stats() // touch stats (future metrics)
	return newDb, nil
}

// detectDBEmbeddingDims introspects F32_BLOB size for entities.embedding
func detectDBEmbeddingDims(db *sql.DB) int {
	var sqlText string
	_ = db.QueryRow("SELECT sql FROM sqlite_master WHERE type='table' AND name='entities'").Scan(&sqlText)
	if sqlText != "" {
		low := strings.ToLower(sqlText)
		idx := strings.Index(low, "f32_blob(")
		if idx >= 0 {
			rest := low[idx+len("f32_blob("):]
			end := strings.Index(rest, ")")
			if end > 0 {
				num := strings.TrimSpace(rest[:end])
				if n, err := strconv.Atoi(num); err == nil && n > 0 {
					return n
				}
			}
		}
	}
	var blob []byte
	_ = db.QueryRow("SELECT embedding FROM entities LIMIT 1").Scan(&blob)
	if len(blob) > 0 && len(blob)%4 == 0 {
		return len(blob) / 4
	}
	return 0
}

// initialize creates schema using goose
func (dm *DBManager) initialize(db *sql.DB) error {
	// For now, rely on goose migrations for schema initialization
	// This will be called after goose has run migrations
	return nil
}

// getPreparedStmt returns or prepares and caches a statement for the given project DB
func (dm *DBManager) getPreparedStmt(ctx context.Context, projectName string, db *sql.DB, sqlText string) (*sql.Stmt, error) {
	// fast path read
	dm.stmtMu.RLock()
	if projCache, ok := dm.stmtCache[projectName]; ok {
		if stmt, ok2 := projCache[sqlText]; ok2 {
			dm.stmtMu.RUnlock()
			return stmt, nil
		}
	}
	dm.stmtMu.RUnlock()

	// prepare and store
	stmt, err := db.PrepareContext(ctx, sqlText)
	if err != nil {
		return nil, fmt.Errorf("failed to prepare statement: %w", err)
	}
	dm.stmtMu.Lock()
	if _, ok := dm.stmtCache[projectName]; !ok {
		dm.stmtCache[projectName] = make(map[string]*sql.Stmt)
	}
	dm.stmtCache[projectName][sqlText] = stmt
	dm.stmtMu.Unlock()
	return stmt, nil
}

// GetQuerier returns the sqlc querier for a project
func (dm *DBManager) GetQuerier(projectName string) (*Queries, error) {
	dm.mu.RLock()
	querier, ok := dm.queries[projectName]
	dm.mu.RUnlock()
	if !ok {
		// Try to get DB to initialize querier
		_, err := dm.getDB(projectName)
		if err != nil {
			return nil, err
		}
		dm.mu.RLock()
		querier = dm.queries[projectName]
		dm.mu.RUnlock()
	}
	return querier, nil
}
