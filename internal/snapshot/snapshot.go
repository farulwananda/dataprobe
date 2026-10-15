package snapshot

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/farulwananda/dataprobe/internal/model"
)

const CurrentSchemaVersion = 1

// Save writes a Snapshot to the specified output file atomically with 0600 permissions.
func Save(s *model.Snapshot, outPath string) error {
	if s == nil {
		return errors.New("cannot save nil snapshot")
	}

	Normalize(s)

	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to serialize snapshot JSON: %w", err)
	}

	dir := filepath.Dir(outPath)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("failed to create directory for snapshot: %w", err)
	}

	tmpFile, err := os.CreateTemp(dir, "snapshot_tmp_*.json")
	if err != nil {
		return fmt.Errorf("failed to create temporary snapshot file: %w", err)
	}
	tmpPath := tmpFile.Name()

	success := false
	defer func() {
		if !success {
			_ = tmpFile.Close()
			_ = os.Remove(tmpPath)
		}
	}()

	_ = tmpFile.Chmod(0600)

	if _, err := tmpFile.Write(data); err != nil {
		return fmt.Errorf("failed writing snapshot data: %w", err)
	}
	if err := tmpFile.Sync(); err != nil {
		return fmt.Errorf("failed syncing snapshot file: %w", err)
	}
	if err := tmpFile.Close(); err != nil {
		return fmt.Errorf("failed closing temp snapshot file: %w", err)
	}

	if err := os.Rename(tmpPath, outPath); err != nil {
		return fmt.Errorf("failed atomically replacing snapshot file: %w", err)
	}

	success = true
	return nil
}

// Load reads and parses a Snapshot JSON file, verifying version compatibility.
func Load(inPath string) (*model.Snapshot, error) {
	data, err := os.ReadFile(inPath)
	if err != nil {
		return nil, fmt.Errorf("could not read snapshot file %q: %w", inPath, err)
	}

	var s model.Snapshot
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, fmt.Errorf("invalid snapshot JSON in %q: %w", inPath, err)
	}

	if s.SchemaVersion != CurrentSchemaVersion {
		return nil, fmt.Errorf("unsupported snapshot schema_version %d (supported: %d)", s.SchemaVersion, CurrentSchemaVersion)
	}

	Normalize(&s)
	return &s, nil
}

// Normalize ensures arrays are deterministically sorted for reproducible snapshots and diffs.
func Normalize(s *model.Snapshot) {
	if s == nil {
		return
	}

	// Sort schemas
	sort.Strings(s.Source.Schemas)

	// Sort tables by schema, then name
	sort.Slice(s.Tables, func(i, j int) bool {
		if s.Tables[i].Schema != s.Tables[j].Schema {
			return s.Tables[i].Schema < s.Tables[j].Schema
		}
		return s.Tables[i].Name < s.Tables[j].Name
	})

	// Inside each table, sort indexes by name
	for tIdx := range s.Tables {
		tbl := &s.Tables[tIdx]
		sort.Slice(tbl.Indexes, func(i, j int) bool {
			return tbl.Indexes[i].Name < tbl.Indexes[j].Name
		})
	}

	// Sort foreign keys by from.schema, from.table, name
	sort.Slice(s.ForeignKeys, func(i, j int) bool {
		if s.ForeignKeys[i].From.Schema != s.ForeignKeys[j].From.Schema {
			return s.ForeignKeys[i].From.Schema < s.ForeignKeys[j].From.Schema
		}
		if s.ForeignKeys[i].From.Table != s.ForeignKeys[j].From.Table {
			return s.ForeignKeys[i].From.Table < s.ForeignKeys[j].From.Table
		}
		return s.ForeignKeys[i].Name < s.ForeignKeys[j].Name
	})
}
