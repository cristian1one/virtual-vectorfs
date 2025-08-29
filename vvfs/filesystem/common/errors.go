package common

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strings"
)

// Common error types used across filesystem packages
var (
	ErrPathEmpty        = errors.New("path cannot be empty")
	ErrPathTooLong      = errors.New("path too long (max 4096 characters)")
	ErrPathInvalid      = errors.New("path contains invalid characters")
	ErrSourceNotExist   = errors.New("source does not exist")
	ErrDestNotExist     = errors.New("destination does not exist")
	ErrOperationUnsafe  = errors.New("operation is not safe to perform")
	ErrPermissionDenied = errors.New("permission denied")
)

// ValidationUtils provides common validation utilities used across packages
type ValidationUtils struct{}

// NewValidationUtils creates a new ValidationUtils instance
func NewValidationUtils() *ValidationUtils {
	return &ValidationUtils{}
}

// ValidateContextCancellation checks if context is cancelled and returns appropriate error
func (vu *ValidationUtils) ValidateContextCancellation(ctx context.Context) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
		return nil
	}
}

// ValidateRequiredString validates that a string is not empty
func (vu *ValidationUtils) ValidateRequiredString(value, fieldName string) error {
	if strings.TrimSpace(value) == "" {
		return fmt.Errorf("%s cannot be empty", fieldName)
	}
	return nil
}

// ValidatePathLength validates that a path is not too long
func (vu *ValidationUtils) ValidatePathLength(path string) error {
	if len(path) > 4096 {
		return ErrPathTooLong
	}
	return nil
}

// ValidatePathCharacters validates that a path doesn't contain invalid characters
func (vu *ValidationUtils) ValidatePathCharacters(path string) error {
	if strings.Contains(path, "\x00") {
		return ErrPathInvalid
	}
	return nil
}

// ValidateFileExists validates that a file exists
func (vu *ValidationUtils) ValidateFileExists(path string) error {
	if _, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			return ErrSourceNotExist
		}
		return fmt.Errorf("failed to access file %s: %w", path, err)
	}
	return nil
}

// ValidateDirectoryExists validates that a directory exists
func (vu *ValidationUtils) ValidateDirectoryExists(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return ErrDestNotExist
		}
		return fmt.Errorf("failed to access directory %s: %w", path, err)
	}
	if !info.IsDir() {
		return fmt.Errorf("path is not a directory: %s", path)
	}
	return nil
}

// ErrorUtils provides common error handling utilities
type ErrorUtils struct{}

// NewErrorUtils creates a new ErrorUtils instance
func NewErrorUtils() *ErrorUtils {
	return &ErrorUtils{}
}

// WrapError wraps an error with additional context
func (eu *ErrorUtils) WrapError(err error, message string, args ...interface{}) error {
	if err == nil {
		return nil
	}
	context := fmt.Sprintf(message, args...)
	return fmt.Errorf("%s: %w", context, err)
}

// LogAndWrapError logs an error and wraps it with context
func (eu *ErrorUtils) LogAndWrapError(err error, level slog.Level, message string, args ...interface{}) error {
	if err == nil {
		return nil
	}

	context := fmt.Sprintf(message, args...)

	switch level {
	case slog.LevelDebug:
		slog.Debug(context, "error", err)
	case slog.LevelInfo:
		slog.Info(context, "error", err)
	case slog.LevelWarn:
		slog.Warn(context, "error", err)
	case slog.LevelError:
		slog.Error(context, "error", err)
	}

	return fmt.Errorf("%s: %w", context, err)
}

// IsRetryableError determines if an error is retryable
func (eu *ErrorUtils) IsRetryableError(err error) bool {
	if err == nil {
		return false
	}

	errStr := err.Error()
	retryablePatterns := []string{
		"temporary failure",
		"timeout",
		"connection refused",
		"no such file or directory", // Sometimes temporary
		"resource temporarily unavailable",
	}

	for _, pattern := range retryablePatterns {
		if strings.Contains(strings.ToLower(errStr), pattern) {
			return true
		}
	}

	return false
}

// HandleOperationError provides common error handling for file operations
func (eu *ErrorUtils) HandleOperationError(err error, operation, path string, logError bool) error {
	if err == nil {
		return nil
	}

	if logError {
		slog.Error("Operation failed",
			"operation", operation,
			"path", path,
			"error", err)
	}

	return eu.WrapError(err, "failed to %s %s", operation, path)
}
