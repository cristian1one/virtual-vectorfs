package common

import (
	"fmt"
	"sync"
	"time"
)

// PerformanceMetrics defines the interface for performance tracking
type PerformanceMetrics interface {
	UpdateMetrics(start time.Time)
	GetMetrics() map[string]interface{}
}

// BaseMetrics provides common fields used across different metrics types
type BaseMetrics struct {
	TotalOperations int64
	SuccessfulOps   int64
	FailedOps       int64
	LastOperation   time.Time
	Mu              sync.RWMutex
}

// UpdateBaseMetrics updates common metrics fields
func (bm *BaseMetrics) UpdateBaseMetrics(start time.Time, success bool) {
	bm.Mu.Lock()
	defer bm.Mu.Unlock()

	bm.TotalOperations++
	if success {
		bm.SuccessfulOps++
	} else {
		bm.FailedOps++
	}
	bm.LastOperation = time.Now()
}

// GetBaseMetrics returns the common metrics as a map
func (bm *BaseMetrics) GetBaseMetrics() map[string]interface{} {
	bm.Mu.RLock()
	defer bm.Mu.RUnlock()

	return map[string]interface{}{
		"total_operations": bm.TotalOperations,
		"successful_ops":   bm.SuccessfulOps,
		"failed_ops":       bm.FailedOps,
		"last_operation":   bm.LastOperation,
	}
}

// FileOperationMetrics tracks performance for file operations
type FileOperationMetrics struct {
	BaseMetrics
	TotalBytesTransferred int64
	AverageSpeed          float64 // bytes per second
}

// UpdateMetrics updates file operation metrics
func (fom *FileOperationMetrics) UpdateMetrics(start time.Time, isDirectory bool, success bool, bytesTransferred int64) {
	fom.UpdateBaseMetrics(start, success)

	duration := time.Since(start)
	if bytesTransferred > 0 {
		fom.TotalBytesTransferred += bytesTransferred
		fom.AverageSpeed = float64(fom.TotalBytesTransferred) / duration.Seconds()
	}
}

// GetMetrics returns file operation metrics as a map
func (fom *FileOperationMetrics) GetMetrics() map[string]interface{} {
	metrics := fom.GetBaseMetrics()
	metrics["total_bytes_transferred"] = fom.TotalBytesTransferred
	metrics["average_speed"] = fom.AverageSpeed
	return metrics
}

// DirectoryMetrics tracks performance metrics for directory operations
type DirectoryMetrics struct {
	BaseMetrics
	TotalTraversals  int64
	TotalFiles       int64
	TotalDirectories int64
	AverageTime      time.Duration
}

// UpdateMetrics updates directory operation metrics
func (dm *DirectoryMetrics) UpdateMetrics(start time.Time) {
	dm.Mu.Lock()
	defer dm.Mu.Unlock()

	dm.TotalTraversals++
	duration := time.Since(start)

	// Calculate rolling average
	if dm.TotalTraversals == 1 {
		dm.AverageTime = duration
	} else {
		dm.AverageTime = (dm.AverageTime*time.Duration(dm.TotalTraversals-1) + duration) / time.Duration(dm.TotalTraversals)
	}

	dm.LastOperation = time.Now()
}

// GetMetrics returns directory metrics as a map
func (dm *DirectoryMetrics) GetMetrics() map[string]interface{} {
	metrics := dm.GetBaseMetrics()
	dm.Mu.RLock()
	defer dm.Mu.RUnlock()

	metrics["total_traversals"] = dm.TotalTraversals
	metrics["total_files"] = dm.TotalFiles
	metrics["total_directories"] = dm.TotalDirectories
	metrics["average_time"] = dm.AverageTime
	return metrics
}

// TimeUtils provides time-related utilities used across packages
type TimeUtils struct{}

// NewTimeUtils creates a new TimeUtils instance
func NewTimeUtils() *TimeUtils {
	return &TimeUtils{}
}

// GetCurrentTime returns current time in milliseconds for performance tracking
func (tu TimeUtils) GetCurrentTime() int64 {
	return time.Now().UnixMilli()
}

// GetCurrentTimeUnix returns current time in Unix seconds
func (tu TimeUtils) GetCurrentTimeUnix() int64 {
	return time.Now().Unix()
}

// CalculateDuration calculates duration between start and now
func (tu TimeUtils) CalculateDuration(start time.Time) time.Duration {
	return time.Since(start)
}

// FormatDuration formats a duration for human-readable display
func (tu TimeUtils) FormatDuration(duration time.Duration) string {
	if duration < time.Millisecond {
		return fmt.Sprintf("%.2fµs", float64(duration.Nanoseconds())/1000)
	} else if duration < time.Second {
		return fmt.Sprintf("%.2fms", float64(duration.Nanoseconds())/1000000)
	} else if duration < time.Minute {
		return fmt.Sprintf("%.2fs", duration.Seconds())
	} else if duration < time.Hour {
		return fmt.Sprintf("%.2fm", duration.Minutes())
	} else {
		return fmt.Sprintf("%.2fh", duration.Hours())
	}
}
