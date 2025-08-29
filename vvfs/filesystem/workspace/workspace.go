package workspace

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	internal "github.com/ZanzyTHEbar/virtual-vectorfs/vvfs"
	"github.com/ZanzyTHEbar/virtual-vectorfs/vvfs/db"

	"github.com/ZanzyTHEbar/assert-lib"
	"github.com/google/uuid"
)

// Manager handles workspace operations and lifecycle management
type Manager struct {
	centralDB     db.ICentralDBProvider
	AssertHandler *assert.AssertHandler
}

// NewManager creates a new workspace manager instance
func NewManager(centralDB db.ICentralDBProvider, assertHandler *assert.AssertHandler) *Manager {
	return &Manager{
		centralDB:     centralDB,
		AssertHandler: assertHandler,
	}
}

// createWorkspacePath creates the workspace path from a root path
func createWorkspacePath(rootPath string) string {
	return filepath.Join(rootPath, internal.DefaultWorkspaceDotDir)
}

// CreateWorkspace creates a new workspace, adding it to the central DB and initializing its own DB
func (wm *Manager) CreateWorkspace(rootPath, config string) (uuid.UUID, error) {
	slog.Debug(fmt.Sprintf("Creating workspace at path: %s\n", rootPath))

	rootPath = createWorkspacePath(rootPath)

	slog.Debug(fmt.Sprintf("Workspace path: %s\n", rootPath))

	// mkdirall to check if the directory exists, if not create it
	if _, err := os.Stat(rootPath); os.IsNotExist(err) {
		if err := os.MkdirAll(rootPath, 0o755); err != nil {
			slog.Error(fmt.Sprintf("Error creating directory at %s", rootPath), "error", err)
			// Decide if this should be a fatal error or if logging is sufficient.
			// For now, logging and continuing, but this might need to return an error.
		}
	}

	// create the ignore file
	ignoreFilePath := filepath.Join(rootPath, fmt.Sprintf(".%s_ignore", internal.DefaultWorkspaceDotDir))

	if _, err := os.Stat(ignoreFilePath); os.IsNotExist(err) {
		ignoreFile, createErr := os.Create(ignoreFilePath)
		if createErr != nil {
			slog.Error(fmt.Sprintf("Error creating ignore file at %s", ignoreFilePath), "error", createErr)
			// return uuid.Nil, fmt.Errorf("failed to create ignore file: %w", createErr) // Consider returning error
		} else {
			defer ignoreFile.Close()
			// Add the `.git` folder to the ignore file
			if _, writeErr := ignoreFile.WriteString(".git\n"); writeErr != nil {
				slog.Error(fmt.Sprintf("Error writing to ignore file at %s", ignoreFilePath), "error", writeErr)
			}
		}
	}

	// Initialize workspace-specific database
	workspaceDB, err := db.NewWorkspaceDB(rootPath)
	if err != nil {
		return uuid.Nil, fmt.Errorf("failed to initialize workspace database: %v", err)
	}
	defer workspaceDB.Close()

	workspace, err := wm.centralDB.AddWorkspace(rootPath, config)
	if err != nil {
		return uuid.Nil, fmt.Errorf("failed to add workspace: %v", err)
	}

	workspaceID := workspace.ID

	slog.Debug(fmt.Sprintf("Workspace created with ID: %s at path: %s\n", workspaceID, rootPath))
	return workspaceID, nil
}

// GetWorkspace retrieves a workspace by ID
func (wm *Manager) GetWorkspace(workspaceID uuid.UUID) (*db.Workspace, error) {
	workspace, err := wm.centralDB.GetWorkspace(workspaceID)
	if err != nil {
		return nil, fmt.Errorf("failed to get workspace: %v", err)
	}
	return workspace, nil
}

// UpdateWorkspace updates the configuration of an existing workspace by ID
func (wm *Manager) UpdateWorkspace(workspaceID uuid.UUID, newConfig string) error {
	_, err := wm.centralDB.UpdateWorkspaceConfig(workspaceID, newConfig)
	if err != nil {
		return fmt.Errorf("failed to update workspace configuration: %v", err)
	}
	slog.Debug(fmt.Sprintf("Workspace with ID %s updated.\n", workspaceID))
	return nil
}

// DeleteWorkspace deletes a workspace from the central DB and removes its specific database file
func (wm *Manager) DeleteWorkspace(workspaceID uuid.UUID) error {
	// Get the root path of the workspace to delete
	rootPath, err := wm.centralDB.GetWorkspacePath(workspaceID)
	if err != nil {
		return fmt.Errorf("failed to find workspace: %v", err)
	}

	// Delete the workspace entry from the central database
	err = wm.centralDB.DeleteWorkspace(workspaceID)
	if err != nil {
		return fmt.Errorf("failed to delete workspace from central DB: %v", err)
	}

	// Remove the workspace database file
	rootPath = createWorkspacePath(rootPath)
	workspaceDBPath := filepath.Join(rootPath, "workspace.db")

	if _, err := os.Stat(workspaceDBPath); os.IsNotExist(err) {
		return nil
	}

	if err := os.Remove(workspaceDBPath); err != nil {
		return fmt.Errorf("failed to delete workspace DB file: %v", err)
	}
	slog.Debug(fmt.Sprintf("Workspace with ID %s deleted.\n", workspaceID))
	return nil
}

// ListWorkspaces returns all workspaces
func (wm *Manager) ListWorkspaces() ([]db.Workspace, error) {
	workspaces, err := wm.centralDB.ListWorkspaces()
	if err != nil {
		return nil, fmt.Errorf("failed to list workspaces: %v", err)
	}
	return workspaces, nil
}

// GetCentralDB returns the central database instance
func (wm *Manager) GetCentralDB() db.ICentralDBProvider {
	return wm.centralDB
}
