package fileops

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"github.com/ZanzyTHEbar/virtual-vectorfs/vvfs/filesystem/options"
	"github.com/ZanzyTHEbar/virtual-vectorfs/vvfs/filesystem/types"
)

// BatchOps handles batch file operations with concurrency control
type BatchOps struct {
	fileOps    *FileOps
	maxWorkers int
}

// NewBatchOps creates a new batch operations instance
func NewBatchOps(fileOps *FileOps, maxWorkers int) *BatchOps {
	if maxWorkers <= 0 {
		maxWorkers = 4 // Default to 4 workers
	}
	return &BatchOps{
		fileOps:    fileOps,
		maxWorkers: maxWorkers,
	}
}

// CopyBatch performs batch copy operations
func (bo *BatchOps) CopyBatch(ctx context.Context, operations []types.OrganizationOperation, opts options.CopyOptions) (*types.OperationResult, error) {
	start := time.Now()

	result := &types.OperationResult{
		Success: false, // Will be set to true if all operations succeed
	}

	// Create semaphore for concurrency control
	semaphore := make(chan struct{}, bo.maxWorkers)
	var wg sync.WaitGroup
	var successCount int32
	var errorCount int32
	var mu sync.Mutex
	var errors []error

	// Process operations concurrently
	for i, op := range operations {
		wg.Add(1)
		go func(index int, operation types.OrganizationOperation) {
			defer wg.Done()

			// Acquire semaphore
			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			// Execute operation
			err := bo.fileOps.CopyFile(ctx, operation.SourcePath, operation.TargetPath, opts)

			if err != nil {
				atomic.AddInt32(&errorCount, 1)
				mu.Lock()
				errors = append(errors, fmt.Errorf("operation %d failed: %w", index, err))
				mu.Unlock()
				slog.Error("Batch copy operation failed", "index", index, "src", operation.SourcePath, "dst", operation.TargetPath, "error", err)
			} else {
				atomic.AddInt32(&successCount, 1)
				mu.Lock()
				result.ProcessedFiles++
				mu.Unlock()
				slog.Debug("Batch copy operation completed", "index", index, "src", operation.SourcePath, "dst", operation.TargetPath)
			}
		}(i, op)
	}

	// Wait for all operations to complete
	wg.Wait()

	result.Duration = time.Since(start)
	result.Success = errorCount == 0

	// Add errors to metadata if any
	if len(errors) > 0 {
		result.Metadata = map[string]interface{}{
			"errors": errors,
		}
	}

	slog.Info("Batch copy operations completed",
		"total", len(operations),
		"successful", successCount,
		"failed", errorCount,
		"duration", result.Duration)

	return result, nil
}

// MoveBatch performs batch move operations
func (bo *BatchOps) MoveBatch(ctx context.Context, operations []types.OrganizationOperation, opts options.MoveOptions) (*types.OperationResult, error) {
	start := time.Now()

	result := &types.OperationResult{
		Success: false, // Will be set to true if all operations succeed
	}

	// Create semaphore for concurrency control
	semaphore := make(chan struct{}, bo.maxWorkers)
	var wg sync.WaitGroup
	var successCount int32
	var errorCount int32
	var mu sync.Mutex
	var errors []error

	// Process operations concurrently
	for i, op := range operations {
		wg.Add(1)
		go func(index int, operation types.OrganizationOperation) {
			defer wg.Done()

			// Acquire semaphore
			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			// Execute operation
			err := bo.fileOps.MoveFile(ctx, operation.SourcePath, operation.TargetPath, opts)

			if err != nil {
				atomic.AddInt32(&errorCount, 1)
				mu.Lock()
				errors = append(errors, fmt.Errorf("operation %d failed: %w", index, err))
				mu.Unlock()
				slog.Error("Batch move operation failed", "index", index, "src", operation.SourcePath, "dst", operation.TargetPath, "error", err)
			} else {
				atomic.AddInt32(&successCount, 1)
				mu.Lock()
				result.ProcessedFiles++
				mu.Unlock()
				slog.Debug("Batch move operation completed", "index", index, "src", operation.SourcePath, "dst", operation.TargetPath)
			}
		}(i, op)
	}

	// Wait for all operations to complete
	wg.Wait()

	result.Duration = time.Since(start)
	result.Success = errorCount == 0

	// Add errors to metadata if any
	if len(errors) > 0 {
		result.Metadata = map[string]interface{}{
			"errors": errors,
		}
	}

	slog.Info("Batch move operations completed",
		"total", len(operations),
		"successful", successCount,
		"failed", errorCount,
		"duration", result.Duration)

	return result, nil
}

// DeleteBatch performs batch delete operations
func (bo *BatchOps) DeleteBatch(ctx context.Context, paths []string) (*types.OperationResult, error) {
	start := time.Now()

	result := &types.OperationResult{
		Success: false, // Will be set to true if all operations succeed
	}

	// Create semaphore for concurrency control
	semaphore := make(chan struct{}, bo.maxWorkers)
	var wg sync.WaitGroup
	var successCount int32
	var errorCount int32
	var mu sync.Mutex
	var errors []error

	// Process operations concurrently
	for i, path := range paths {
		wg.Add(1)
		go func(index int, filePath string) {
			defer wg.Done()

			// Acquire semaphore
			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			// Determine if it's a file or directory and delete accordingly
			info, err := os.Stat(filePath)
			if err != nil {
				atomic.AddInt32(&errorCount, 1)
				mu.Lock()
				errors = append(errors, fmt.Errorf("failed to stat %s: %w", filePath, err))
				mu.Unlock()
				return
			}

			if info.IsDir() {
				err = bo.fileOps.DeleteDirectory(ctx, filePath, true) // Recursive delete for directories
			} else {
				err = bo.fileOps.DeleteFile(ctx, filePath)
			}

			if err != nil {
				atomic.AddInt32(&errorCount, 1)
				mu.Lock()
				errors = append(errors, fmt.Errorf("delete operation %d failed: %w", index, err))
				mu.Unlock()
				slog.Error("Batch delete operation failed", "index", index, "path", filePath, "error", err)
			} else {
				atomic.AddInt32(&successCount, 1)
				mu.Lock()
				if info.IsDir() {
					result.ProcessedDirs++
				} else {
					result.ProcessedFiles++
				}
				mu.Unlock()
				slog.Debug("Batch delete operation completed", "index", index, "path", filePath)
			}
		}(i, path)
	}

	// Wait for all operations to complete
	wg.Wait()

	result.Duration = time.Since(start)
	result.Success = errorCount == 0

	// Add errors to metadata if any
	if len(errors) > 0 {
		result.Metadata = map[string]interface{}{
			"errors": errors,
		}
	}

	slog.Info("Batch delete operations completed",
		"total", len(paths),
		"successful", successCount,
		"failed", errorCount,
		"duration", result.Duration)

	return result, nil
}
