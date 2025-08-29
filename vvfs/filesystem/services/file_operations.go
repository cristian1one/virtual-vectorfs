package services

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"syscall"
	"time"

	"github.com/ZanzyTHEbar/virtual-vectorfs/vvfs/filesystem/common"
	"github.com/ZanzyTHEbar/virtual-vectorfs/vvfs/filesystem/fileops"
	"github.com/ZanzyTHEbar/virtual-vectorfs/vvfs/filesystem/interfaces"
	"github.com/ZanzyTHEbar/virtual-vectorfs/vvfs/filesystem/options"
	"github.com/ZanzyTHEbar/virtual-vectorfs/vvfs/filesystem/types"
	"github.com/ZanzyTHEbar/virtual-vectorfs/vvfs/trees"
)

// FileOperationsService provides high-performance file operations
// Integrates with Phase 1 batch database operations and concurrent systems
type FileOperationsService struct {
	fileOps  fileops.FileOpsInterface
	cacheDir string // Keep cacheDir for trash operations
}

// Use common.FileOperationMetrics instead

// NewFileOperationsService creates a new file operations service
func NewFileOperationsService(conflictResolver interfaces.ConflictResolver, cacheDir string) *FileOperationsService {
	return &FileOperationsService{
		fileOps:  fileops.NewFileOps(conflictResolver, cacheDir),
		cacheDir: cacheDir,
	}
}

// Copy copies a file or directory with enhanced performance and error handling
func (fos *FileOperationsService) Copy(ctx context.Context, src *trees.DirectoryNode, dst string, opts options.CopyOptions) error {
	if src == nil {
		return fmt.Errorf("source node cannot be nil")
	}

	if opts.DryRun {
		return fos.previewCopy(ctx, src, dst, opts)
	}

	// Check for context cancellation
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	// Handle directory copying
	if len(src.Children) > 0 || len(src.Files) > 0 {
		return fos.copyDirectory(ctx, src, dst, opts)
	}

	return fmt.Errorf("source node has no files or directories to copy")
}

// CopyFile copies a single file with optimized performance
func (fos *FileOperationsService) CopyFile(ctx context.Context, srcPath, dstPath string, opts options.CopyOptions) error {
	return fos.fileOps.CopyFile(ctx, srcPath, dstPath, opts)
}

// Move moves a file or directory with cross-device fallback
func (fos *FileOperationsService) Move(ctx context.Context, src *trees.DirectoryNode, dst string, opts options.MoveOptions) error {
	start := time.Now()
	defer fos.updateMetrics(start, true)

	if src == nil {
		return fmt.Errorf("source node cannot be nil")
	}

	if opts.DryRun {
		slog.Info("Dry run: would move directory", "src", src.Path, "dst", dst)
		return nil
	}

	// Check for context cancellation
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	// Try direct rename first (most efficient)
	if err := os.Rename(src.Path, dst); err != nil {
		// Handle cross-device link error with fallback
		if fos.isCrossDeviceError(err) && opts.FallbackToCopy {
			slog.Warn("Cross-device move detected, falling back to copy+delete", "src", src.Path, "dst", dst)

			copyOpts := options.CopyOptions{
				Recursive:     true,
				RemoveSource:  true,
				DryRun:        false,
				Conflict:      opts.Conflict,
				PreservePerms: opts.PreservePerms,
			}

			return fos.Copy(ctx, src, dst, copyOpts)
		}
		return fmt.Errorf("move operation failed: %w", err)
	}

	return nil
}

// Delete deletes a file or directory
func (fos *FileOperationsService) Delete(ctx context.Context, path string, opts options.DeleteOptions) error {
	start := time.Now()
	defer fos.updateMetrics(start, false)

	if opts.DryRun {
		slog.Info("Dry run: would delete", "path", path)
		return nil
	}

	// Check for context cancellation
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	// Move to trash if requested
	if opts.MoveToTrash {
		return fos.moveToTrash(ctx, path)
	}

	// Perform deletion
	var err error
	if opts.Recursive {
		err = os.RemoveAll(path)
	} else {
		err = os.Remove(path)
	}

	if err != nil {
		return fmt.Errorf("delete operation failed: %w", err)
	}

	return nil
}

// MoveToTrash moves a file or directory to the trash directory
func (fos *FileOperationsService) MoveToTrash(ctx context.Context, node *trees.DirectoryNode) error {
	if node == nil {
		return fmt.Errorf("node cannot be nil")
	}

	return fos.moveToTrash(ctx, node.Path)
}

// CalculateChecksum calculates the checksum of a file
func (fos *FileOperationsService) CalculateChecksum(path string) (string, error) {
	// Use SHA256 as default algorithm
	fileUtils := common.NewFileUtils()
	return fileUtils.CalculateChecksum(path, "sha256")
}

// GetFileInfo returns file information as a FileNode
func (fos *FileOperationsService) GetFileInfo(path string) (*trees.FileNode, error) {
	fileInfo, err := fos.fileOps.GetFileInfo(path)
	if err != nil {
		return nil, err
	}

	return &trees.FileNode{
		Path:      fileInfo.Path,
		Name:      fileInfo.Name,
		Extension: fileInfo.Extension,
		Metadata: trees.Metadata{
			Size:       fileInfo.Metadata.Size,
			ModifiedAt: fileInfo.Metadata.ModifiedAt,
			NodeType:   trees.File,
		},
	}, nil
}

// Checksum calculation is now handled by the common package

// CopyDirectory copies a directory recursively from source path to destination path
func (fos *FileOperationsService) CopyDirectory(ctx context.Context, srcPath, dstPath string, opts options.CopyOptions) error {
	// TODO: This is a wrapper around the internal copyDirectory method
	// For now, we'll create a simple DirectoryNode from the source path
	srcInfo, err := os.Stat(srcPath)
	if err != nil {
		return fmt.Errorf("failed to stat source directory %s: %w", srcPath, err)
	}

	if !srcInfo.IsDir() {
		return fmt.Errorf("source is not a directory: %s", srcPath)
	}

	// Create a simple DirectoryNode for the source
	srcNode := &trees.DirectoryNode{
		Path: srcPath,
		Type: trees.Directory,
	}

	return fos.copyDirectory(ctx, srcNode, dstPath, opts)
}

// MoveDirectory moves a directory recursively from source path to destination path
func (fos *FileOperationsService) MoveDirectory(ctx context.Context, srcPath, dstPath string, opts options.CopyOptions) error {
	// Copy first
	if err := fos.CopyDirectory(ctx, srcPath, dstPath, opts); err != nil {
		return fmt.Errorf("failed to copy directory during move: %w", err)
	}

	// Remove source if copy was successful and RemoveSource is true
	if opts.RemoveSource {
		if err := fos.DeleteDirectory(ctx, srcPath, true); err != nil {
			return fmt.Errorf("failed to remove source directory after copy: %w", err)
		}
	}

	return nil
}

// DeleteDirectory deletes a directory recursively
func (fos *FileOperationsService) DeleteDirectory(ctx context.Context, path string, recursive bool) error {
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("failed to stat directory %s: %w", path, err)
	}

	if !info.IsDir() {
		return fmt.Errorf("path is not a directory: %s", path)
	}

	if recursive {
		return os.RemoveAll(path)
	}

	// Non-recursive delete - directory must be empty
	return os.Remove(path)
}

// DeleteFile deletes a single file
func (fos *FileOperationsService) DeleteFile(ctx context.Context, path string) error {
	start := time.Now()
	defer fos.updateMetrics(start, false)

	// Validate path
	if err := fos.ValidatePath(path); err != nil {
		return fmt.Errorf("invalid path: %w", err)
	}

	// Check if file exists
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("file does not exist: %s", path)
		}
		return fmt.Errorf("failed to stat file: %w", err)
	}

	// Ensure it's not a directory
	if info.IsDir() {
		return fmt.Errorf("path is a directory, use DeleteDirectory instead: %s", path)
	}

	slog.Debug("Deleting file", "path", path)

	if err := os.Remove(path); err != nil {
		return fmt.Errorf("failed to delete file %s: %w", path, err)
	}

	slog.Info("File deleted successfully", "path", path)
	return nil
}

// ValidatePath validates that a path is safe and accessible
func (fos *FileOperationsService) ValidatePath(path string) error {
	if path == "" {
		return fmt.Errorf("path cannot be empty")
	}

	// Check for invalid characters (basic check)
	if len(path) > 4096 {
		return fmt.Errorf("path too long (max 4096 characters)")
	}

	return nil
}

// MoveFile
func (fos *FileOperationsService) MoveFile(ctx context.Context, srcPath, dstPath string, opts options.CopyOptions) error {
	// Convert CopyOptions to MoveOptions for internal use
	moveOpts := options.MoveOptions{
		DryRun:         opts.DryRun,
		Conflict:       opts.Conflict,
		PreservePerms:  opts.PreservePerms,
		FallbackToCopy: true,
		MaxRetries:     3,
	}
	return fos.moveFileInternal(ctx, srcPath, dstPath, moveOpts)
}

// Internal move implementation with MoveOptions
func (fos *FileOperationsService) moveFileInternal(ctx context.Context, srcPath, dstPath string, opts options.MoveOptions) error {
	start := time.Now()
	defer fos.updateMetrics(start, false)

	if opts.DryRun {
		slog.Info("DRY RUN: Would move file", "source", srcPath, "dest", dstPath)
		return nil
	}

	// Validate paths
	if err := fos.ValidatePath(srcPath); err != nil {
		return fmt.Errorf("invalid source path: %w", err)
	}
	if err := fos.ValidatePath(dstPath); err != nil {
		return fmt.Errorf("invalid destination path: %w", err)
	}

	// Check source exists
	if _, err := os.Stat(srcPath); err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("source file does not exist: %s", srcPath)
		}
		return fmt.Errorf("failed to stat source file: %w", err)
	}

	// Conflicts are now handled internally by the fileops package

	// Create destination directory if needed
	dstDir := filepath.Dir(dstPath)
	if err := os.MkdirAll(dstDir, 0o755); err != nil {
		return fmt.Errorf("failed to create destination directory: %w", err)
	}

	// Try atomic move first
	if err := os.Rename(srcPath, dstPath); err != nil {
		if opts.FallbackToCopy && fos.isCrossDeviceError(err) {
			// Fallback to copy+delete for cross-device moves
			copyOpts := options.CopyOptions{
				Conflict:      opts.Conflict,
				PreservePerms: opts.PreservePerms,
				PreserveTimes: true,
				DryRun:        false,
			}

			if err := fos.CopyFile(ctx, srcPath, dstPath, copyOpts); err != nil {
				return fmt.Errorf("failed to copy file during cross-device move: %w", err)
			}

			if err := os.Remove(srcPath); err != nil {
				slog.Error("Failed to remove source after copy", "path", srcPath, "error", err)
				return fmt.Errorf("failed to remove source file after copy: %w", err)
			}
		} else {
			return fmt.Errorf("failed to move file: %w", err)
		}
	}

	slog.Info("File moved successfully", "source", srcPath, "dest", dstPath)

	return nil
}

// Private helper methods

func (fos *FileOperationsService) previewCopy(ctx context.Context, src *trees.DirectoryNode, dst string, opts options.CopyOptions) error {
	slog.Info("Dry run: would copy directory", "src", src.Path, "dst", dst, "recursive", opts.Recursive)

	// Recursively preview copy operations
	if opts.Recursive {
		for _, child := range src.Children {
			childDst := filepath.Join(dst, filepath.Base(child.Path))
			if err := fos.previewCopy(ctx, child, childDst, opts); err != nil {
				return err
			}
		}
	}

	for _, file := range src.Files {
		fileDst := filepath.Join(dst, filepath.Base(file.Path))
		slog.Info("Dry run: would copy file", "src", file.Path, "dst", fileDst)
	}

	return nil
}

func (fos *FileOperationsService) copyDirectory(ctx context.Context, src *trees.DirectoryNode, dst string, opts options.CopyOptions) error {
	if !opts.Recursive {
		return fmt.Errorf("source is a directory, use recursive flag to copy directories")
	}

	// Create destination directory
	if err := os.MkdirAll(dst, 0o755); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", dst, err)
	}

	// Copy files in the directory
	for _, fileNode := range src.Files {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		fileDst := filepath.Join(dst, filepath.Base(fileNode.Path))
		fileOpts := options.CopyOptions{
			RemoveSource:  opts.RemoveSource,
			DryRun:        false,
			Conflict:      opts.Conflict,
			PreservePerms: opts.PreservePerms,
			PreserveTimes: opts.PreserveTimes,
		}

		if err := fos.CopyFile(ctx, fileNode.Path, fileDst, fileOpts); err != nil {
			return fmt.Errorf("failed to copy file %s: %w", fileNode.Path, err)
		}
	}

	// Copy child directories recursively
	for _, childDir := range src.Children {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		childDst := filepath.Join(dst, filepath.Base(childDir.Path))
		if err := fos.copyDirectory(ctx, childDir, childDst, opts); err != nil {
			return fmt.Errorf("failed to copy directory %s: %w", childDir.Path, err)
		}
	}

	// Remove source directory if requested
	if opts.RemoveSource {
		if err := os.RemoveAll(src.Path); err != nil {
			return fmt.Errorf("failed to remove source directory after copy: %w", err)
		}
	}

	return nil
}

// File operations are now handled by the fileops package

// File operations are now handled by the fileops package

// Conflict handling is now done internally by the fileops package

func (fos *FileOperationsService) moveToTrash(_ context.Context, path string) error {
	if fos.cacheDir == "" {
		return fmt.Errorf("trash directory not configured")
	}

	trashPath := filepath.Join(fos.cacheDir, "trash")
	if err := os.MkdirAll(trashPath, 0o755); err != nil {
		return fmt.Errorf("failed to create trash directory: %w", err)
	}

	// Generate unique filename in trash
	baseName := filepath.Base(path)
	timestamp := time.Now().Format("20060102_150405")
	trashFile := filepath.Join(trashPath, fmt.Sprintf("%s_%s", timestamp, baseName))

	return os.Rename(path, trashFile)
}

func (fos *FileOperationsService) isCrossDeviceError(err error) bool {
	if linkErr, ok := err.(*os.LinkError); ok {
		return linkErr.Err == syscall.EXDEV
	}
	return false
}

func (fos *FileOperationsService) updateMetrics(start time.Time, isDirectory bool) {
	// Metrics are now handled by the fileops package
	duration := time.Since(start)

	// Log different operation types for better metrics tracking
	if isDirectory {
		slog.Debug("Directory operation completed", "duration", duration)
	} else {
		slog.Debug("File operation completed", "duration", duration)
	}
}

// CopyBatch performs batch copy operations
func (fos *FileOperationsService) CopyBatch(ctx context.Context, operations []types.OrganizationOperation, opts options.CopyOptions) (*types.OperationResult, error) {
	start := time.Now()
	result := &types.OperationResult{
		Success:        true,
		ProcessedFiles: 0,
		ProcessedDirs:  0,
		SkippedFiles:   0,
		Conflicts:      make([]*types.ConflictInfo, 0),
		Events:         make([]types.Event, 0),
	}

	for _, op := range operations {
		select {
		case <-ctx.Done():
			result.Success = false
			result.Error = ctx.Err()
			return result, ctx.Err()
		default:
		}

		switch op.Type {
		case types.OpCopy:
			if err := fos.CopyFile(ctx, op.SourcePath, op.TargetPath, opts); err != nil {
				result.SkippedFiles++
				slog.Error("Failed to copy file in batch", "source", op.SourcePath, "error", err)
				continue
			}
			result.ProcessedFiles++
		case types.OpSkip:
			result.SkippedFiles++
		default:
			slog.Warn("Unknown operation type in batch", "type", op.Type)
			result.SkippedFiles++
		}
	}

	result.Duration = time.Since(start)
	return result, nil
}

// MoveBatch performs batch move operations
func (fos *FileOperationsService) MoveBatch(ctx context.Context, operations []types.OrganizationOperation, opts options.CopyOptions) (*types.OperationResult, error) {
	start := time.Now()
	result := &types.OperationResult{
		Success:        true,
		ProcessedFiles: 0,
		ProcessedDirs:  0,
		SkippedFiles:   0,
		Conflicts:      make([]*types.ConflictInfo, 0),
		Events:         make([]types.Event, 0),
	}

	for _, op := range operations {
		select {
		case <-ctx.Done():
			result.Success = false
			result.Error = ctx.Err()
			return result, ctx.Err()
		default:
		}

		switch op.Type {
		case types.OpMove:
			if err := fos.MoveFile(ctx, op.SourcePath, op.TargetPath, opts); err != nil {
				result.SkippedFiles++
				slog.Error("Failed to move file in batch", "source", op.SourcePath, "error", err)
				continue
			}
			result.ProcessedFiles++
		case types.OpSkip:
			result.SkippedFiles++
		default:
			slog.Warn("Unknown operation type in batch", "type", op.Type)
			result.SkippedFiles++
		}
	}

	result.Duration = time.Since(start)
	return result, nil
}
