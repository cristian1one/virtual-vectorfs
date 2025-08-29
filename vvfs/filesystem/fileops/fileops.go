package fileops

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/ZanzyTHEbar/virtual-vectorfs/vvfs/filesystem/common"
	"github.com/ZanzyTHEbar/virtual-vectorfs/vvfs/filesystem/interfaces"
	"github.com/ZanzyTHEbar/virtual-vectorfs/vvfs/filesystem/options"
	"github.com/ZanzyTHEbar/virtual-vectorfs/vvfs/filesystem/types"
	"github.com/ZanzyTHEbar/virtual-vectorfs/vvfs/trees"
)

// FileOps provides low-level file system operations
type FileOps struct {
	conflictResolver interfaces.ConflictResolver
	cacheDir         string
	metrics          *common.FileOperationMetrics
	safetyUtils      *common.SafetyUtils
	pathUtils        *common.PathUtils
	batchOps         *BatchOps
}

// NewFileOps creates a new file operations instance
func NewFileOps(conflictResolver interfaces.ConflictResolver, cacheDir string) *FileOps {
	fo := &FileOps{
		conflictResolver: conflictResolver,
		cacheDir:         cacheDir,
		metrics:          &common.FileOperationMetrics{},
		safetyUtils:      common.NewSafetyUtils(),
		pathUtils:        common.NewPathUtils(),
	}
	fo.batchOps = NewBatchOps(fo, 4) // Default 4 workers
	return fo
}

// CopyFile copies a single file with optimized performance
func (fo *FileOps) CopyFile(ctx context.Context, srcPath, dstPath string, opts options.CopyOptions) error {
	start := time.Now()
	defer fo.updateMetrics(start, false)

	if opts.DryRun {
		slog.Info("Dry run: would copy file", "src", srcPath, "dst", dstPath)
		return nil
	}

	// Check for context cancellation
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	// Handle conflicts and get resolved destination path
	resolvedDstPath, err := fo.handleFileConflict(ctx, srcPath, dstPath, opts.Conflict)
	if err != nil {
		return fmt.Errorf("failed to handle file conflict: %w", err)
	}

	// Validate operation safety (using a default check space value)
	checkSpace := false // Default to false since CopyOptions doesn't have this field
	if err := fo.safetyUtils.ValidateOperationSafety(srcPath, resolvedDstPath, checkSpace); err != nil {
		return fmt.Errorf("operation not safe: %w", err)
	}

	// Perform the actual file copy
	if err := fo.performFileCopy(ctx, srcPath, resolvedDstPath, opts); err != nil {
		return fmt.Errorf("failed to copy file from %s to %s: %w", srcPath, resolvedDstPath, err)
	}

	// Copy file attributes if requested
	if opts.PreservePerms || opts.PreserveTimes {
		fileUtils := common.NewFileUtils()
		if err := fileUtils.CopyFileAttributes(srcPath, resolvedDstPath, opts.PreservePerms, opts.PreserveTimes); err != nil {
			slog.Warn("Failed to copy file attributes", "error", err)
		}
	}

	return nil
}

// MoveFile moves a single file (copy + delete)
func (fo *FileOps) MoveFile(ctx context.Context, srcPath, dstPath string, opts options.MoveOptions) error {
	start := time.Now()
	defer fo.updateMetrics(start, false)

	if opts.DryRun {
		slog.Info("Dry run: would move file", "src", srcPath, "dst", dstPath)
		return nil
	}

	// Check for context cancellation
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	// Validate operation safety (using a default check space value)
	checkSpace := false // Default to false since MoveOptions doesn't have this field
	if err := fo.safetyUtils.ValidateOperationSafety(srcPath, dstPath, checkSpace); err != nil {
		return fmt.Errorf("operation not safe: %w", err)
	}

	// Handle conflicts and get resolved destination path
	resolvedDstPath, err := fo.handleFileConflict(ctx, srcPath, dstPath, opts.Conflict)
	if err != nil {
		return fmt.Errorf("failed to handle file conflict: %w", err)
	}

	// Try cross-device move first (rename)
	if err := os.Rename(srcPath, resolvedDstPath); err == nil {
		// Rename succeeded - file moved within same device
		return nil
	} else if !fo.isCrossDeviceError(err) {
		// Unexpected error
		return fmt.Errorf("failed to move file: %w", err)
	}

	// Cross-device move - copy then delete
	copyOpts := options.CopyOptions{
		PreservePerms: opts.PreservePerms,
		Conflict:      opts.Conflict,
	}

	if err := fo.performFileCopy(ctx, srcPath, resolvedDstPath, copyOpts); err != nil {
		return fmt.Errorf("failed to copy file during move: %w", err)
	}

	// Remove source file
	if err := os.Remove(srcPath); err != nil {
		return fmt.Errorf("failed to remove source file after copy: %w", err)
	}

	return nil
}

// DeleteFile deletes a single file
func (fo *FileOps) DeleteFile(ctx context.Context, path string) error {
	start := time.Now()
	defer fo.updateMetrics(start, false)

	// Check for context cancellation
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	// Validate path
	if err := fo.pathUtils.ValidatePath(path); err != nil {
		return fmt.Errorf("invalid path: %w", err)
	}

	// Check if file exists
	if _, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("file does not exist: %s", path)
		}
		return fmt.Errorf("failed to access file: %w", err)
	}

	// Delete the file
	if err := os.Remove(path); err != nil {
		return fmt.Errorf("failed to delete file %s: %w", path, err)
	}

	return nil
}

// CreateDirectory creates a directory with the specified permissions
func (fo *FileOps) CreateDirectory(ctx context.Context, path string, perms os.FileMode) error {
	start := time.Now()
	defer fo.updateMetrics(start, true)

	// Check for context cancellation
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	// Validate path
	if err := fo.pathUtils.ValidatePath(path); err != nil {
		return fmt.Errorf("invalid path: %w", err)
	}

	// Create directory
	if err := os.MkdirAll(path, perms); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", path, err)
	}

	return nil
}

// DeleteDirectory deletes a directory (optionally recursively)
func (fo *FileOps) DeleteDirectory(ctx context.Context, path string, recursive bool) error {
	start := time.Now()
	defer fo.updateMetrics(start, true)

	// Check for context cancellation
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	// Validate path
	if err := fo.pathUtils.ValidatePath(path); err != nil {
		return fmt.Errorf("invalid path: %w", err)
	}

	// Check if directory exists
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("directory does not exist: %s", path)
		}
		return fmt.Errorf("failed to access directory: %w", err)
	}

	if !info.IsDir() {
		return fmt.Errorf("path is not a directory: %s", path)
	}

	if recursive {
		if err := os.RemoveAll(path); err != nil {
			return fmt.Errorf("failed to delete directory recursively %s: %w", path, err)
		}
	} else {
		if err := os.Remove(path); err != nil {
			return fmt.Errorf("failed to delete directory %s: %w", path, err)
		}
	}

	return nil
}

// CopyDirectory copies a directory tree
func (fo *FileOps) CopyDirectory(ctx context.Context, srcPath, dstPath string, opts options.CopyOptions) error {
	start := time.Now()
	defer fo.updateMetrics(start, true)

	if opts.DryRun {
		slog.Info("Dry run: would copy directory", "src", srcPath, "dst", dstPath)
		return nil
	}

	// Check for context cancellation
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	// Validate paths
	if err := fo.pathUtils.ValidatePath(srcPath); err != nil {
		return fmt.Errorf("invalid source path: %w", err)
	}
	if err := fo.pathUtils.ValidatePath(dstPath); err != nil {
		return fmt.Errorf("invalid destination path: %w", err)
	}

	// Check if source exists and is a directory
	srcInfo, err := os.Stat(srcPath)
	if err != nil {
		return fmt.Errorf("failed to access source directory: %w", err)
	}
	if !srcInfo.IsDir() {
		return fmt.Errorf("source is not a directory: %s", srcPath)
	}

	// Create destination directory
	if err := os.MkdirAll(dstPath, srcInfo.Mode()); err != nil {
		return fmt.Errorf("failed to create destination directory: %w", err)
	}

	// Walk through source directory and copy files
	return filepath.WalkDir(srcPath, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}

		// Get relative path from source
		relPath, err := filepath.Rel(srcPath, path)
		if err != nil {
			return fmt.Errorf("failed to get relative path: %w", err)
		}

		// Skip root directory
		if relPath == "." {
			return nil
		}

		// Construct destination path
		dstFilePath := filepath.Join(dstPath, relPath)

		if d.IsDir() {
			// Create subdirectory
			info, err := d.Info()
			if err != nil {
				return fmt.Errorf("failed to get directory info: %w", err)
			}
			return os.MkdirAll(dstFilePath, info.Mode())
		} else {
			// Copy file
			return fo.performFileCopy(ctx, path, dstFilePath, opts)
		}
	})
}

// MoveDirectory moves a directory (rename or copy+delete)
func (fo *FileOps) MoveDirectory(ctx context.Context, srcPath, dstPath string, opts options.MoveOptions) error {
	start := time.Now()
	defer fo.updateMetrics(start, true)

	if opts.DryRun {
		slog.Info("Dry run: would move directory", "src", srcPath, "dst", dstPath)
		return nil
	}

	// Check for context cancellation
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	// Validate paths
	if err := fo.pathUtils.ValidatePath(srcPath); err != nil {
		return fmt.Errorf("invalid source path: %w", err)
	}
	if err := fo.pathUtils.ValidatePath(dstPath); err != nil {
		return fmt.Errorf("invalid destination path: %w", err)
	}

	// Try cross-device move first (rename)
	if err := os.Rename(srcPath, dstPath); err == nil {
		// Rename succeeded - directory moved within same device
		return nil
	} else if !fo.isCrossDeviceError(err) {
		// Unexpected error
		return fmt.Errorf("failed to move directory: %w", err)
	}

	// Cross-device move - copy then delete
	copyOpts := options.CopyOptions{
		PreservePerms: opts.PreservePerms,
		Conflict:      opts.Conflict,
	}

	if err := fo.CopyDirectory(ctx, srcPath, dstPath, copyOpts); err != nil {
		return fmt.Errorf("failed to copy directory during move: %w", err)
	}

	// Remove source directory
	if err := os.RemoveAll(srcPath); err != nil {
		return fmt.Errorf("failed to remove source directory after copy: %w", err)
	}

	return nil
}

// GetFileInfo returns information about a file
func (fo *FileOps) GetFileInfo(path string) (*trees.FileNode, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("failed to stat file %s: %w", path, err)
	}

	// Create FileNode with proper structure
	fileNode := &trees.FileNode{
		Path:      path,
		Name:      filepath.Base(path),
		Extension: filepath.Ext(path),
		Metadata: trees.Metadata{
			Size:       info.Size(),
			ModifiedAt: info.ModTime(),
			NodeType:   trees.File,
		},
	}

	return fileNode, nil
}

// ValidatePath validates that a path is safe and accessible
func (fo *FileOps) ValidatePath(path string) error {
	return fo.pathUtils.ValidatePath(path)
}

// GetMetrics returns performance metrics
func (fo *FileOps) GetMetrics() map[string]interface{} {
	return fo.metrics.GetMetrics()
}

// Batch operations (delegate to batchOps)

// CopyBatch performs batch copy operations
func (fo *FileOps) CopyBatch(ctx context.Context, operations []types.OrganizationOperation, opts options.CopyOptions) (*types.OperationResult, error) {
	return fo.batchOps.CopyBatch(ctx, operations, opts)
}

// MoveBatch performs batch move operations
func (fo *FileOps) MoveBatch(ctx context.Context, operations []types.OrganizationOperation, opts options.MoveOptions) (*types.OperationResult, error) {
	return fo.batchOps.MoveBatch(ctx, operations, opts)
}

// DeleteBatch performs batch delete operations
func (fo *FileOps) DeleteBatch(ctx context.Context, paths []string) (*types.OperationResult, error) {
	return fo.batchOps.DeleteBatch(ctx, paths)
}

// Private helper methods

func (fo *FileOps) performFileCopy(ctx context.Context, srcPath, dstPath string, opts options.CopyOptions) error {
	srcFile, err := os.Open(srcPath)
	if err != nil {
		return fmt.Errorf("failed to open source file: %w", err)
	}
	defer srcFile.Close()

	// Create destination directory if it doesn't exist
	dstDir := filepath.Dir(dstPath)
	if err := os.MkdirAll(dstDir, 0o755); err != nil {
		return fmt.Errorf("failed to create destination directory: %w", err)
	}

	dstFile, err := os.Create(dstPath)
	if err != nil {
		return fmt.Errorf("failed to create destination file: %w", err)
	}
	defer dstFile.Close()

	// Copy with progress tracking
	bytesCopied, err := fo.copyWithProgress(ctx, dstFile, srcFile)
	if err != nil {
		return fmt.Errorf("failed to copy file content: %w", err)
	}

	// Update metrics
	fo.metrics.TotalBytesTransferred += bytesCopied

	return nil
}

func (fo *FileOps) copyWithProgress(ctx context.Context, dst io.Writer, src io.Reader) (int64, error) {
	buffer := make([]byte, 32*1024) // 32KB buffer
	var totalBytes int64

	for {
		select {
		case <-ctx.Done():
			return totalBytes, ctx.Err()
		default:
		}

		n, readErr := src.Read(buffer)
		if n > 0 {
			if _, writeErr := dst.Write(buffer[:n]); writeErr != nil {
				return totalBytes, writeErr
			}
			totalBytes += int64(n)
		}

		if readErr != nil {
			if readErr == io.EOF {
				return totalBytes, nil
			}
			return totalBytes, readErr
		}
	}
}

func (fo *FileOps) handleFileConflict(ctx context.Context, srcPath, dstPath string, strategy options.ConflictStrategy) (string, error) {
	// Check if destination exists
	if _, err := os.Stat(dstPath); err != nil {
		if os.IsNotExist(err) {
			// Destination doesn't exist, no conflict
			return dstPath, nil
		}
		return "", fmt.Errorf("failed to check destination: %w", err)
	}

	// Handle conflict based on strategy
	switch strategy {
	case options.ConflictSkip:
		return "", fmt.Errorf("destination already exists and skip strategy is set")
	case options.ConflictOverwrite:
		return dstPath, nil
	case options.ConflictRename:
		return fo.generateUniqueName(dstPath), nil
	case options.ConflictPrompt:
		// For now, default to skip. In a real implementation, this would prompt the user
		return "", fmt.Errorf("destination already exists and ask strategy requires user input")
	default:
		return "", fmt.Errorf("unknown conflict strategy: %v", strategy)
	}
}

func (fo *FileOps) generateUniqueName(path string) string {
	dir := filepath.Dir(path)
	name := filepath.Base(path)
	ext := filepath.Ext(name)
	baseName := strings.TrimSuffix(name, ext)

	counter := 1
	for {
		newName := fmt.Sprintf("%s (%d)%s", baseName, counter, ext)
		newPath := filepath.Join(dir, newName)

		if _, err := os.Stat(newPath); os.IsNotExist(err) {
			return newPath
		}
		counter++
	}
}

func (fo *FileOps) isCrossDeviceError(err error) bool {
	return strings.Contains(err.Error(), "cross-device link") ||
		strings.Contains(err.Error(), "invalid cross-device link")
}

func (fo *FileOps) updateMetrics(start time.Time, isDirectory bool) {
	fo.metrics.UpdateMetrics(start, isDirectory, true, 0)
}
