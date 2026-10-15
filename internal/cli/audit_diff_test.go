package cli_test

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/farulwananda/dataprobe/internal/cli"
	"github.com/farulwananda/dataprobe/internal/model"
	"github.com/farulwananda/dataprobe/internal/snapshot"
)

func TestCLI_AuditCommand(t *testing.T) {
	tempDir := t.TempDir()
	snapPath := filepath.Join(tempDir, "snap-with-issues.json")

	snap := &model.Snapshot{
		SchemaVersion: 1,
		ToolVersion:   "0.1.0",
		CapturedAt:    time.Now().UTC(),
		Source: model.SourceMetadata{
			Engine:  "mysql",
			Schemas: []string{"app_db"},
		},
		Tables: []model.Table{
			{
				// Table without PK triggers SCH_001 (high severity)
				Schema:     "app_db",
				Name:       "unindexed_logs",
				Kind:       "base_table",
				PrimaryKey: []string{},
			},
		},
	}
	if err := snapshot.Save(snap, snapPath); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer

	// Audit with fail-on high should return ExitFinding (1)
	code := cli.Run([]string{"audit", "--input", snapPath, "--fail-on", "high"}, &stdout, &stderr)
	if code != cli.ExitFinding {
		t.Errorf("expected ExitFinding (1), got %d. Output: %s", code, stdout.String())
	}
	if !strings.Contains(stdout.String(), "SCH_001") {
		t.Errorf("expected finding SCH_001 in output, got: %s", stdout.String())
	}

	// Audit with fail-on critical should pass with ExitSuccess (0)
	stdout.Reset()
	stderr.Reset()
	code = cli.Run([]string{"audit", "--input", snapPath, "--fail-on", "critical"}, &stdout, &stderr)
	if code != cli.ExitSuccess {
		t.Errorf("expected ExitSuccess (0) with fail-on critical, got %d", code)
	}
}

func TestCLI_DiffCommand(t *testing.T) {
	tempDir := t.TempDir()
	snap1Path := filepath.Join(tempDir, "snap1.json")
	snap2Path := filepath.Join(tempDir, "snap2.json")

	snap1 := &model.Snapshot{
		SchemaVersion: 1,
		CapturedAt:    time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC),
		Tables: []model.Table{
			{
				Schema:   "app_db",
				Name:     "users",
				RowCount: model.RowCount{Value: 100},
				StorageBytes: model.StorageBytes{
					Data:    1048576, // 1 MiB
					Indexes: 524288,
				},
			},
		},
	}
	snap2 := &model.Snapshot{
		SchemaVersion: 1,
		CapturedAt:    time.Date(2026, 10, 8, 0, 0, 0, 0, time.UTC),
		Tables: []model.Table{
			{
				Schema:   "app_db",
				Name:     "users",
				RowCount: model.RowCount{Value: 250},
				StorageBytes: model.StorageBytes{
					Data:    2097152, // 2 MiB
					Indexes: 1048576,
				},
			},
		},
	}

	if err := snapshot.Save(snap1, snap1Path); err != nil {
		t.Fatal(err)
	}
	if err := snapshot.Save(snap2, snap2Path); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	code := cli.Run([]string{"diff", snap1Path, snap2Path}, &stdout, &stderr)
	if code != cli.ExitSuccess {
		t.Errorf("expected ExitSuccess (0), got %d. Stderr: %s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "SNAPSHOT DIFF & GROWTH REPORT") {
		t.Errorf("diff output missing header: %s", stdout.String())
	}
	if !strings.Contains(stdout.String(), "TOP GROWTH CONTRIBUTORS") {
		t.Errorf("diff output missing growth contributors: %s", stdout.String())
	}
}
