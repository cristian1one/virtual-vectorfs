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
	"time"

	_ "github.com/tursodatabase/go-libsql"
)

const defaultProject = "default"

// NewDBManager creates a new database manager
func NewDBManager(config *Config) (*DBManager, error) {
	if config.EmbeddingDims <= 0 || config.EmbeddingDims > 65536 {
		return nil, fmt.Errorf("EMBEDDING_DIMS must be between 1 and 65536 inclusive: %d", config.EmbeddingDims)
	}
	manager := &DBManager{
		config:        config,
		dbs:           make(map[string]*sql.DB),
		stmtCache:     make(map[string]map[string]*sql.Stmt),
		capsByProject: make(map[string]capFlags),
	}

	// initialize default DB in single-project mode
	if !config.MultiProjectMode {
		if _, err := manager.getDB(defaultProject); err != nil {
			return nil, fmt.Errorf("failed to initialize default database: %w", err)
		}
	}
	return manager, nil
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
		if err := os.MkdirAll(filepath.Dir(dbPath), 0755); err != nil {
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

// initialize creates schema
func (dm *DBManager) initialize(db *sql.DB) error {
	tx, err := db.BeginTx(context.Background(), nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction for initialization: %w", err)
	}
	defer tx.Rollback()
	for _, statement := range dynamicSchema(dm.config.EmbeddingDims) {
		if _, err := tx.Exec(statement); err != nil {
			return fmt.Errorf("failed to execute schema statement: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	// Try to set up FTS triggers/table if module exists (best-effort)
	_ = dm.ensureFTSSchema(context.Background(), db)
	return nil
}


