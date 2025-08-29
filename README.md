# Virtual Vector Filesystem (vvfs)

[![Go Version](https://img.shields.io/badge/go-1.25-blue.svg)](https://golang.org)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)
[![GitHub Repo](https://img.shields.io/badge/GitHub-virtual--vectorfs-181717.svg)](https://github.com/ZanzyTHEbar/virtual-vectorfs)

A high-performance, AI-enhanced virtual filesystem implementation in Go, designed for modern file organization and management with advanced indexing, concurrent operations, and machine learning capabilities.

## 🌟 Features

### Core Filesystem Operations
- **Hierarchical Directory Structures** - Advanced tree-based file organization
- **Concurrent File Operations** - High-performance parallel processing using goroutines
- **Intelligent File Organization** - Automated categorization and workflow management
- **Conflict Resolution** - Smart handling of file conflicts with multiple strategies
- **Git Integration** - Seamless version control operations within the filesystem

### Advanced Indexing & Search
- **Spatial Indexing** - KD-tree based spatial indexing for efficient file location
- **Bitmap Indexing** - Roaring bitmaps for ultra-fast set operations
- **Multi-dimensional Indexing** - Eytzinger layout optimization for cache efficiency
- **Path-based Indexing** - Hierarchical path indexing for rapid traversal

### AI/ML Integration
- **ONNX Runtime Integration** - Native support for machine learning models
- **Embedding Providers** - Multiple embedding backends for semantic search
- **Tokenizer Support** - Advanced text tokenization for NLP tasks
- **Matryoshka Embeddings** - Hierarchical embedding representations

### Database & Persistence
- **SQLite/Turso Integration** - High-performance embedded database
- **Workspace Management** - Multi-workspace support with isolated configurations
- **Metadata Persistence** - Comprehensive file metadata storage
- **Central Database** - Shared metadata across workspaces

### Developer Experience
- **Hexagonal Architecture** - Clean, testable, and maintainable code structure
- **Comprehensive Testing** - Extensive test suites with table-driven tests
- **Structured Logging** - Zerolog integration for observability
- **Configuration Management** - Viper-based configuration with multiple sources
- **CLI Integration** - Command-line interface for filesystem operations

## 🚀 Quick Start

### Prerequisites
- Go 1.25 or later
- SQLite3 development libraries (optional, for enhanced performance)

### Installation

```bash
# Clone the repository
git clone https://github.com/ZanzyTHEbar/virtual-vectorfs.git
cd virtual-vectorfs

# Install dependencies
go mod download

# Run tests
go test ./...

# Build the project
go build ./...
```

### Basic Usage

```go
package main

import (
    "context"
    "log"

    "github.com/ZanzyTHEbar/virtual-vectorfs/vvfs/filesystem"
    "github.com/ZanzyTHEbar/virtual-vectorfs/vvfs/db"
    "github.com/ZanzyTHEbar/virtual-vectorfs/vvfs/ports"
)

func main() {
    // Create database provider
    centralDB, err := db.NewCentralDBProvider()
    if err != nil {
        log.Fatal(err)
    }
    defer centralDB.Close()

    // Create terminal interactor
    interactor := ports.NewTerminalInteractor()

    // Create filesystem manager
    fs, err := filesystem.New(interactor, centralDB)
    if err != nil {
        log.Fatal(err)
    }

    // Index a directory
    ctx := context.Background()
    err = fs.IndexDirectory(ctx, "/path/to/directory", filesystem.DefaultIndexOptions())
    if err != nil {
        log.Fatal(err)
    }

    // Build directory tree with analysis
    tree, analysis, err := fs.BuildDirectoryTreeWithAnalysis(ctx, "/path/to/directory", filesystem.DefaultTraversalOptions())
    if err != nil {
        log.Fatal(err)
    }

    log.Printf("Indexed %d files, %d directories", analysis.FileCount, analysis.DirectoryCount)
}
```

## 📖 Documentation

### Architecture Overview

The project follows a **hexagonal architecture** (ports and adapters) pattern:

```
├── ports/           # Application ports (interfaces)
├── filesystem/      # Core filesystem business logic
│   ├── interfaces/  # Service interfaces
│   ├── services/    # Service implementations
│   ├── types/       # Data types and DTOs
│   ├── options/     # Configuration options
│   └── common/      # Shared utilities
├── trees/           # Tree data structures and algorithms
├── indexing/        # Advanced indexing implementations
├── embedding/       # AI/ML embedding providers
├── db/              # Database providers and interfaces
├── memory/          # In-memory data structures
└── config/          # Configuration management
```

### Key Components

#### Filesystem Services
- **DirectoryService** - Directory indexing and tree building
- **FileOperations** - File manipulation operations
- **OrganizationService** - Intelligent file organization
- **ConflictResolver** - File conflict detection and resolution
- **GitService** - Git repository operations

#### Advanced Features
- **ConcurrentTraverser** - High-performance parallel directory traversal
- **KDTree** - Spatial indexing for file locations
- **RoaringBitmaps** - Efficient set operations for file indexing
- **ONNX Providers** - Machine learning model execution

## 🔧 Configuration

Create a configuration file at `~/.config/vvfs/config.toml`:

```toml
[database]
type = "sqlite3"
dsn = "file:~/vvfs/central.db"

[filesystem]
cache_dir = "~/.config/vvfs/.cache"
max_concurrent_operations = 10

[embedding]
default_provider = "onnx"
model_path = "~/.config/vvfs/models"

[logging]
level = "info"
format = "json"
```

## 🧪 Testing

Run the comprehensive test suite:

```bash
# Run all tests
go test ./...

# Run tests with coverage
go test -cover ./...

# Run integration tests
go test -tags=integration ./...

# Run specific test
go test -run TestConcurrentTraverser ./vvfs/filesystem/
```

## 📊 Performance

The filesystem is optimized for high-performance operations:

- **Concurrent Processing** - Utilizes all available CPU cores
- **Memory-Efficient** - Streaming operations for large file sets
- **Cache-Optimized** - Eytzinger layout for improved cache locality
- **Database Performance** - Connection pooling and prepared statements

## 🤝 Contributing

We welcome contributions! Please follow these steps:

1. Fork the repository
2. Create a feature branch: `git checkout -b feature/your-feature`
3. Write tests for your changes
4. Ensure all tests pass: `go test ./...`
5. Follow conventional commit format for commits
6. Submit a pull request

### Development Guidelines

- **Code Style** - Follow standard Go formatting (`go fmt`)
- **Testing** - Write table-driven tests for new functionality
- **Documentation** - Update documentation for API changes
- **Performance** - Include benchmarks for performance-critical code
- **Security** - Validate inputs and handle errors properly

## 📄 License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

## 🙏 Acknowledgments

- **Roaring Bitmaps** - For efficient bitmap operations
- **ONNX Runtime** - For machine learning model execution
- **Turso** - For distributed SQLite database
- **Go Community** - For the excellent standard library and ecosystem

## 🔗 Related Projects

- [go-fuse](https://github.com/hanwen/go-fuse) - FUSE filesystem implementation
- [bleve](https://github.com/blevesearch/bleve) - Full-text search library
- [badger](https://github.com/dgraph-io/badger) - Key-value database

## 📞 Support

For questions and support:
- Open an issue on [GitHub](https://github.com/ZanzyTHEbar/virtual-vectorfs/issues)
- Check the [documentation](docs/) for detailed guides
- Join our community discussions

---

**Built with ❤️ in Go**
