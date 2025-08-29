package service

import (
	"github.com/ZanzyTHEbar/virtual-vectorfs/vvfs/memory/database"
)

// Config exposes a stable wrapper for memory database configuration
type Config struct {
	URL              string
	AuthToken        string
	ProjectsDir      string
	MultiProjectMode bool
	EmbeddingDims    int
	MaxOpenConns     int
	MaxIdleConns     int
	ConnMaxIdleSec   int
	ConnMaxLifeSec   int
}

// ToInternal converts to internal database config
func (c *Config) ToInternal() *database.Config {
	return &database.Config{
		URL:              c.URL,
		AuthToken:        c.AuthToken,
		ProjectsDir:      c.ProjectsDir,
		MultiProjectMode: c.MultiProjectMode,
		EmbeddingDims:    c.EmbeddingDims,
		MaxOpenConns:     c.MaxOpenConns,
		MaxIdleConns:     c.MaxIdleConns,
		ConnMaxIdleSec:   c.ConnMaxIdleSec,
		ConnMaxLifeSec:   c.ConnMaxLifeSec,
	}
}
