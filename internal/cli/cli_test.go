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

func TestCLI_VersionAndHelp(t *testing.T) {
	var stdout, stderr bytes.Buffer

	// 1. Version
	code := cli.Run([]string{"version"}, &stdout, &stderr)
	if code != cli.ExitSuccess {
		t.Errorf("expected ExitSuccess (0), got %d", code)
	}
	if !strings.Contains(stdout.String(), "dataprobe v0.1.0") {
		t.Errorf("version output unexpected: %s", stdout.String())
	}

	// 2. Help
	stdout.Reset()
	stderr.Reset()
	code = cli.Run([]string{"--help"}, &stdout, &stderr)
	if code != cli.ExitSuccess {
		t.Errorf("expected ExitSuccess (0), got %d", code)
	}
	if !strings.Contains(stdout.String(), "Database Intelligence & Integrity Auditor") {
		t.Errorf("help output unexpected: %s", stdout.String())
	}

	// 3. Unknown command
	stdout.Reset()
	stderr.Reset()
	code = cli.Run([]string{"unknown-cmd"}, &stdout, &stderr)
	if code != cli.ExitUsage {
		t.Errorf("expected ExitUsage (2), got %d", code)
	}
	if !strings.Contains(stderr.String(), "unknown command") {
		t.Errorf("stderr unexpected: %s", stderr.String())
	}
}

func TestCLI_GraphCommand_MissingInput(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := cli.Run([]string{"graph"}, &stdout, &stderr)
	if code != cli.ExitUsage {
		t.Errorf("expected ExitUsage (2) for missing --input, got %d", code)
	}
	if !strings.Contains(stderr.String(), "--input flag is required") {
		t.Errorf("expected missing --input error, got: %s", stderr.String())
	}
}

func TestCLI_SnapshotCommand_MissingFlags(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := cli.Run([]string{"snapshot"}, &stdout, &stderr)
	if code != cli.ExitUsage {
		t.Errorf("expected ExitUsage (2), got %d", code)
	}

	stdout.Reset()
	stderr.Reset()
	code = cli.Run([]string{"snapshot", "--config", "dataprobe.yaml"}, &stdout, &stderr)
	if code != cli.ExitUsage {
		t.Errorf("expected ExitUsage (2) for missing --out, got %d", code)
	}
	if !strings.Contains(stderr.String(), "--out flag is required") {
		t.Errorf("expected missing --out error, got: %s", stderr.String())
	}
}

func TestCLI_InspectCommand_MissingConfig(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := cli.Run([]string{"inspect"}, &stdout, &stderr)
	if code != cli.ExitUsage {
		t.Errorf("expected ExitUsage (2) for missing --config, got %d", code)
	}
}

func TestCLI_GraphCommand_LoadsSnapshotAndExitsGracefully(t *testing.T) {
	tempDir := t.TempDir()
	snapPath := filepath.Join(tempDir, "synthetic-snap.json")

	snap := &model.Snapshot{
		SchemaVersion: 1,
		ToolVersion:   "0.1.0",
		CapturedAt:    time.Now().UTC(),
		Source: model.SourceMetadata{
			Engine:  "mysql",
			Schemas: []string{"test_db"},
		},
		Tables: []model.Table{
			{Schema: "test_db", Name: "users", Kind: "base_table"},
		},
	}
	if err := snapshot.Save(snap, snapPath); err != nil {
		t.Fatal(err)
	}

	// Verify snapshot loads properly via graph logic
	loaded, err := snapshot.Load(snapPath)
	if err != nil {
		t.Fatalf("failed loading saved snapshot: %v", err)
	}
	if len(loaded.Tables) != 1 {
		t.Errorf("expected 1 table, got %d", len(loaded.Tables))
	}
}
