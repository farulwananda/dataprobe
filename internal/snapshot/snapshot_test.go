package snapshot_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/farulwananda/dataprobe/internal/model"
	"github.com/farulwananda/dataprobe/internal/snapshot"
)

func TestSnapshot_Roundtrip(t *testing.T) {
	tempDir := t.TempDir()
	outPath := filepath.Join(tempDir, "test-snapshot.json")

	snap := &model.Snapshot{
		SchemaVersion: 1,
		ToolVersion:   "0.1.0",
		CapturedAt:    time.Now().UTC().Truncate(time.Second),
		Source: model.SourceMetadata{
			Engine:        "mysql",
			ServerVersion: "8.0.35",
			Alias:         "test-db",
			Schemas:       []string{"shop_db"},
		},
		Capabilities: model.Capabilities{
			TableStatistics:    true,
			WorkloadStatistics: false,
			FullIntegrityScan:  false,
		},
		Tables: []model.Table{
			{
				Schema: "shop_db",
				Name:   "users",
				Kind:   "base_table",
				RowCount: model.RowCount{
					Value:   1500,
					Quality: model.AccuracyEstimated,
				},
				StorageBytes: model.StorageBytes{
					Data:    16384,
					Indexes: 32768,
					Quality: model.AccuracyEstimated,
				},
				Columns: []model.Column{
					{Name: "id", DBType: "bigint", Nullable: false, IsPrimaryKey: true},
					{Name: "email", DBType: "varchar(255)", Nullable: false},
				},
				PrimaryKey: []string{"id"},
				Indexes: []model.Index{
					{
						Name:    "PRIMARY",
						Columns: []model.IndexColumn{{Name: "id", Seq: 1}},
						Unique:  true,
						Primary: true,
					},
					{
						Name:    "idx_email",
						Columns: []model.IndexColumn{{Name: "email", Seq: 1}},
						Unique:  true,
					},
				},
			},
			{
				Schema: "shop_db",
				Name:   "orders",
				Kind:   "base_table",
				RowCount: model.RowCount{
					Value:   5000,
					Quality: model.AccuracyEstimated,
				},
				Columns: []model.Column{
					{Name: "id", DBType: "bigint", Nullable: false, IsPrimaryKey: true},
					{Name: "user_id", DBType: "bigint", Nullable: false},
				},
				PrimaryKey: []string{"id"},
			},
		},
		ForeignKeys: []model.ForeignKey{
			{
				Name: "fk_orders_users",
				From: model.ForeignKeyEndpoint{
					Schema:  "shop_db",
					Table:   "orders",
					Columns: []string{"user_id"},
				},
				To: model.ForeignKeyEndpoint{
					Schema:  "shop_db",
					Table:   "users",
					Columns: []string{"id"},
				},
				Origin:   "declared",
				OnDelete: "RESTRICT",
			},
		},
	}

	if err := snapshot.Save(snap, outPath); err != nil {
		t.Fatalf("snapshot.Save failed: %v", err)
	}

	// Verify file permissions (0600)
	fi, err := os.Stat(outPath)
	if err != nil {
		t.Fatalf("os.Stat failed: %v", err)
	}
	if fi.Mode().Perm() != 0600 {
		t.Errorf("expected file perm 0600, got %v", fi.Mode().Perm())
	}

	// Load and check parity
	loaded, err := snapshot.Load(outPath)
	if err != nil {
		t.Fatalf("snapshot.Load failed: %v", err)
	}

	if loaded.SchemaVersion != 1 {
		t.Errorf("expected schema_version 1, got %d", loaded.SchemaVersion)
	}
	if len(loaded.Tables) != 2 {
		t.Fatalf("expected 2 tables, got %d", len(loaded.Tables))
	}
	if len(loaded.ForeignKeys) != 1 {
		t.Fatalf("expected 1 foreign key, got %d", len(loaded.ForeignKeys))
	}
	if loaded.ForeignKeys[0].Name != "fk_orders_users" {
		t.Errorf("foreign key name mismatch: got %s", loaded.ForeignKeys[0].Name)
	}
}

func TestSnapshot_InvalidVersion(t *testing.T) {
	tempDir := t.TempDir()
	badVersionPath := filepath.Join(tempDir, "bad-version.json")

	badJSON := `{
		"schema_version": 99,
		"tool_version": "0.1.0",
		"tables": []
	}`
	if err := os.WriteFile(badVersionPath, []byte(badJSON), 0600); err != nil {
		t.Fatal(err)
	}

	_, err := snapshot.Load(badVersionPath)
	if err == nil {
		t.Fatal("expected error for unsupported schema_version, got nil")
	}
}
