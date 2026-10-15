package db

import (
	"context"

	"github.com/farulwananda/dataprobe/internal/model"
)

// Scope defines what schemas and tables to introspect.
type Scope struct {
	Schemas       []string
	ExcludeTables []string
	Alias         string
}

// Inspector defines the database introspection capability.
type Inspector interface {
	// Inspect gathers metadata without running full scans or touching row-level data.
	Inspect(ctx context.Context, scope Scope) (*model.Snapshot, error)
	// Close releases any database resources.
	Close() error
}
