package report_test

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/farulwananda/dataprobe/internal/model"
	"github.com/farulwananda/dataprobe/internal/report"
)

func TestReport_PrintInspectSummary(t *testing.T) {
	snap := &model.Snapshot{
		SchemaVersion: 1,
		ToolVersion:   "0.1.0",
		CapturedAt:    time.Now().UTC(),
		Source: model.SourceMetadata{
			Engine:        "mysql",
			ServerVersion: "8.0.35",
			Schemas:       []string{"store_db"},
		},
		Tables: []model.Table{
			{
				Schema:   "store_db",
				Name:     "users",
				Kind:     "base_table",
				RowCount: model.RowCount{Value: 100, Quality: model.AccuracyEstimated},
				StorageBytes: model.StorageBytes{
					Data:    1048576, // 1 MiB
					Indexes: 524288,  // 512 KiB
					Quality: model.AccuracyEstimated,
				},
				Columns: []model.Column{{Name: "id", DBType: "int"}},
			},
		},
		ForeignKeys: []model.ForeignKey{
			{
				Name: "fk_demo",
				From: model.ForeignKeyEndpoint{Schema: "store_db", Table: "orders", Columns: []string{"user_id"}},
				To:   model.ForeignKeyEndpoint{Schema: "store_db", Table: "users", Columns: []string{"id"}},
			},
		},
	}

	// 1. Table format
	var outTable bytes.Buffer
	if err := report.PrintInspectSummary(snap, "table", &outTable); err != nil {
		t.Fatalf("PrintInspectSummary failed: %v", err)
	}
	s := outTable.String()
	if !strings.Contains(s, "DATABASE INSPECT SUMMARY") {
		t.Errorf("expected header, got: %s", s)
	}
	if !strings.Contains(s, "store_db.users") {
		t.Errorf("expected table row, got: %s", s)
	}
	if !strings.Contains(s, "fk_demo") {
		t.Errorf("expected foreign key section, got: %s", s)
	}

	// 2. JSON format
	var outJSON bytes.Buffer
	if err := report.PrintInspectSummary(snap, "json", &outJSON); err != nil {
		t.Fatalf("PrintInspectSummary json failed: %v", err)
	}
	if !strings.HasPrefix(strings.TrimSpace(outJSON.String()), "{") {
		t.Errorf("expected valid JSON object, got: %s", outJSON.String())
	}
}
