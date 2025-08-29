package fileops

import (
	"context"
	"os"

	"github.com/ZanzyTHEbar/virtual-vectorfs/vvfs/filesystem/options"
	"github.com/ZanzyTHEbar/virtual-vectorfs/vvfs/filesystem/types"
	"github.com/ZanzyTHEbar/virtual-vectorfs/vvfs/trees"
)

// FileOperations defines the interface for basic file operations
type FileOperations interface {
	CopyFile(ctx context.Context, srcPath, dstPath string, opts options.CopyOptions) error
	MoveFile(ctx context.Context, srcPath, dstPath string, opts options.MoveOptions) error
	DeleteFile(ctx context.Context, path string) error
	GetFileInfo(path string) (*trees.FileNode, error)
	ValidatePath(path string) error
	GetMetrics() map[string]interface{}
}

// DirectoryOperations defines the interface for directory operations
type DirectoryOperations interface {
	CreateDirectory(ctx context.Context, path string, perms os.FileMode) error
	DeleteDirectory(ctx context.Context, path string, recursive bool) error
	CopyDirectory(ctx context.Context, srcPath, dstPath string, opts options.CopyOptions) error
	MoveDirectory(ctx context.Context, srcPath, dstPath string, opts options.MoveOptions) error
}

// BatchOperations defines the interface for batch operations
type BatchOperations interface {
	CopyBatch(ctx context.Context, operations []types.OrganizationOperation, opts options.CopyOptions) (*types.OperationResult, error)
	MoveBatch(ctx context.Context, operations []types.OrganizationOperation, opts options.MoveOptions) (*types.OperationResult, error)
	DeleteBatch(ctx context.Context, paths []string) (*types.OperationResult, error)
}

// FileOpsInterface combines all file operation interfaces
type FileOpsInterface interface {
	FileOperations
	DirectoryOperations
	BatchOperations
}
