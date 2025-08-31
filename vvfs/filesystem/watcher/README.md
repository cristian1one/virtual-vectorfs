# File System Watcher

A high-performance, cross-platform file system watcher for Go applications with advanced features like event debouncing, fingerprinting, and batch processing.

## Features

- **Cross-platform support**: Linux (fanotify/fsnotify), Windows, macOS
- **Event debouncing**: Prevents duplicate events from rapid file changes
- **Fingerprinting**: Simhash and SHA256-based change detection
- **Batch processing**: Efficient handling of multiple events
- **Configurable**: Tunable parameters for different use cases
- **Context support**: Proper cancellation and timeout handling
- **Structured logging**: slog-based logging with configurable levels

## Architecture

### Components

1. **Watcher Interface**: Core abstraction for file system watching
2. **FSNotify Watcher**: Cross-platform implementation using fsnotify
3. **Fanotify Watcher**: Linux-specific high-performance implementation
4. **Debouncer**: Event debouncing with configurable delays
5. **Fingerprinter**: File content fingerprinting for change detection
6. **Batch Processor**: Efficient event processing in batches

### Event Flow

```
File Change → Watcher → Debouncer → Batch Processor → Event Handler
```

## Quick Start

```go
package main

import (
    "context"
    "fmt"
    "time"

    "github.com/your-org/virtual-vectorfs/vvfs/filesystem/watcher"
)

func main() {
    // Create configuration
    config := watcher.WatcherConfig{
        DebounceDelay:     200 * time.Millisecond,
        MaxDebounceDelay:  1 * time.Second,
        BatchSize:         50,
        WorkerCount:       2,
        QueueCapacity:     100,
        FingerprintEnabled: true,
        SimhashThreshold:   0.9,
    }

    // Create watcher
    w, err := watcher.NewWatcher(config)
    if err != nil {
        panic(err)
    }
    defer w.Close()

    ctx := context.Background()

    // Start watching paths
    if err := w.Start(ctx, []string{"/path/to/watch"}); err != nil {
        panic(err)
    }

    // Handle events
    for {
        select {
        case <-ctx.Done():
            return
        case event := <-w.Events():
            fmt.Printf("Event: %v, Path: %s\n", event.Type, event.Path)
        case err := <-w.Errors():
            fmt.Printf("Error: %v\n", err)
        }
    }
}
```

## Advanced Usage

### With Batch Processing

```go
// Create batch processor
processor := watcher.NewReindexProcessor(4, 100)

// Set event handlers
processor.SetCallbacks(
    func(ctx context.Context, path string) error {
        fmt.Printf("File created: %s\n", path)
        return nil
    },
    func(ctx context.Context, path string) error {
        fmt.Printf("File modified: %s\n", path)
        return nil
    },
    nil, // onRemove
    nil, // onRename
)

// Create watcher with processor
w, err := watcher.NewWatcherWithProcessor(config, processor)
if err != nil {
    panic(err)
}

processor.Start() // Don't forget to start the processor
```

### With Fingerprinting

```go
// Create fingerprinter
fp := watcher.NewSimhashFingerprinter()

// Generate fingerprints
fp1, err := fp.Fingerprint("/path/to/file1")
if err != nil {
    panic(err)
}

fp2, err := fp.Fingerprint("/path/to/file2")
if err != nil {
    panic(err)
}

// Compare similarity
similarity := fp.Compare(fp1, fp2)
if similarity > 0.9 {
    fmt.Println("Files are very similar")
}
```

## Configuration

```go
type WatcherConfig struct {
    // DebounceDelay is the time to wait before processing events
    DebounceDelay time.Duration

    // MaxDebounceDelay is the maximum time to wait before processing events
    MaxDebounceDelay time.Duration

    // BatchSize is the maximum number of events to process in a batch
    BatchSize int

    // WorkerCount is the number of workers for processing events
    WorkerCount int

    // QueueCapacity is the capacity of the event processing queue
    QueueCapacity int

    // FingerprintEnabled enables file fingerprinting for change detection
    FingerprintEnabled bool

    // SimhashThreshold is the similarity threshold for simhash comparison
    SimhashThreshold float64
}
```

## Event Types

- `EventCreate`: File/directory creation
- `EventWrite`: File modification
- `EventRemove`: File/directory removal
- `EventRename`: File/directory rename
- `EventChmod`: Permission changes

## Platform-Specific Features

### Linux (fanotify)

When built with `fanotify` tag, uses Linux fanotify for superior performance:

```bash
go build -tags fanotify
```

Features:
- Inode-based monitoring (more efficient than path-based)
- Lower CPU overhead
- Better handling of moved/renamed files

### Fallback (fsnotify)

Default cross-platform implementation using fsnotify:

```bash
go build
```

Works on:
- Linux
- Windows
- macOS
- BSD variants

## Performance Tuning

### For High-Volume Scenarios

```go
config := watcher.WatcherConfig{
    DebounceDelay:     50 * time.Millisecond,  // Shorter debounce
    MaxDebounceDelay:  500 * time.Millisecond,
    BatchSize:         200,                    // Larger batches
    WorkerCount:       8,                      // More workers
    QueueCapacity:     1000,                   // Larger queue
    FingerprintEnabled: false,                 // Disable for speed
}
```

### For Low-Latency Scenarios

```go
config := watcher.WatcherConfig{
    DebounceDelay:     10 * time.Millisecond,  // Minimal debounce
    MaxDebounceDelay:  100 * time.Millisecond,
    BatchSize:         10,                     // Smaller batches
    WorkerCount:       2,                      // Fewer workers
    QueueCapacity:     50,                     // Smaller queue
    FingerprintEnabled: true,                  // Enable for accuracy
}
```

## Error Handling

The watcher provides comprehensive error handling:

```go
// Handle both events and errors
for {
    select {
    case event := <-watcher.Events():
        // Process event
    case err := <-watcher.Errors():
        // Handle error (e.g., permission denied, path not found)
        log.Printf("Watcher error: %v", err)
    }
}
```

## Testing

Run the test suite:

```bash
go test ./watcher -v
```

Run with race detection:

```bash
go test ./watcher -race -v
```

## Integration with Virtual VectorFS

This watcher is designed to integrate seamlessly with the Virtual VectorFS system:

- **Incremental Re-indexing**: Automatically re-index changed files
- **Debounced Updates**: Prevents excessive re-indexing during rapid changes
- **Batch Processing**: Efficiently handles bulk file operations
- **Fingerprinting**: Detects actual content changes vs metadata changes

## Contributing

1. Follow Go best practices
2. Add tests for new features
3. Update documentation
4. Use structured logging
5. Handle context cancellation properly

## License

See project LICENSE file.

