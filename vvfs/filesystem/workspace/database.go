package workspace

import (
	"database/sql"
	"encoding/json"
	"fmt"

	"github.com/ZanzyTHEbar/virtual-vectorfs/vvfs/trees"
	"github.com/google/uuid"
)

// SQLiteWorkspaceDB represents a workspace-specific database
type SQLiteWorkspaceDB struct {
	DB *sql.DB // This would need to be imported from database/sql
}

// AddFileMetadata adds file metadata to the workspace database
func (db *SQLiteWorkspaceDB) AddFileMetadata(workspaceID uuid.UUID, path string, metadata trees.Metadata) error {
	metadataJSON, err := json.Marshal(metadata)
	if err != nil {
		return fmt.Errorf("failed to marshal metadata into JSON: %w", err)
	}

	_, err = db.DB.Exec("INSERT INTO file_metadata (workspace_id, path, metadata_json) VALUES (?, ?, ?)", workspaceID, path, string(metadataJSON))
	if err != nil {
		return fmt.Errorf("failed to insert file metadata: %w", err)
	}

	return nil
}

// UpdateFileMetadata updates the metadata for a given file in the workspace
func (db *SQLiteWorkspaceDB) UpdateFileMetadata(workspaceID uuid.UUID, path string, metadata trees.Metadata) error {
	metadataJSON, err := json.Marshal(metadata)
	if err != nil {
		return fmt.Errorf("failed to marshal metadata into JSON: %w", err)
	}

	_, err = db.DB.Exec("UPDATE file_metadata SET metadata_json = ? WHERE workspace_id = ? AND path = ?", string(metadataJSON), workspaceID, path)
	if err != nil {
		return fmt.Errorf("failed to update file metadata: %w", err)
	}

	return nil
}

// StoreVector stores a vector embedding for a specific file
func (db *SQLiteWorkspaceDB) StoreVector(fileID int, vector []float64) error {
	vectorBlob, err := json.Marshal(vector)
	if err != nil {
		return fmt.Errorf("failed to marshal vector into blob: %w", err)
	}

	_, err = db.DB.Exec("INSERT INTO file_vectors (file_id, vector) VALUES (?, ?, ?)", fileID, vectorBlob)
	if err != nil {
		return fmt.Errorf("failed to insert file vector: %w", err)
	}

	return nil
}

// AddHistoryEvent adds a historical event to track workspace changes
func (db *SQLiteWorkspaceDB) AddHistoryEvent(workspaceID uuid.UUID, eventType string, eventJSON string) error {
	_, err := db.DB.Exec("INSERT INTO history (workspace_id, event_type, event_json) VALUES (?, ?, ?)", workspaceID, eventType, eventJSON)
	if err != nil {
		return fmt.Errorf("failed to insert history event: %w", err)
	}

	return nil
}

// GetWorkspaceID gets the workspace ID by root path
func (db *SQLiteWorkspaceDB) GetWorkspaceID(rootPath string) (uuid.UUID, error) {
	var workspaceID uuid.UUID
	err := db.DB.QueryRow("SELECT id FROM workspaces WHERE root_path = ?", rootPath).Scan(&workspaceID)
	if err != nil {
		return uuid.Nil, fmt.Errorf("failed to get workspace ID: %w", err)
	}

	return workspaceID, nil
}

// Close closes the database connection
func (db *SQLiteWorkspaceDB) Close() error {
	return db.DB.Close()
}
