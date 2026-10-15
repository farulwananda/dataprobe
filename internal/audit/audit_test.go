package audit_test

import (
	"testing"

	"github.com/farulwananda/dataprobe/internal/audit"
	"github.com/farulwananda/dataprobe/internal/model"
)

func TestAudit_StructuralRules(t *testing.T) {
	snap := &model.Snapshot{
		SchemaVersion: 1,
		Source: model.SourceMetadata{
			Engine:  "mysql",
			Schemas: []string{"app_db"},
		},
		Tables: []model.Table{
			{
				// Missing Primary Key -> SCH_001
				Schema:     "app_db",
				Name:       "events_log",
				Kind:       "base_table",
				PrimaryKey: []string{},
			},
			{
				Schema:     "app_db",
				Name:       "users",
				Kind:       "base_table",
				PrimaryKey: []string{"id"},
				Indexes: []model.Index{
					{Name: "PRIMARY", Columns: []model.IndexColumn{{Name: "id", Seq: 1}}, Primary: true},
					// Duplicate indexes -> IDX_001
					{Name: "idx_email_1", Columns: []model.IndexColumn{{Name: "email", Seq: 1}}},
					{Name: "idx_email_2", Columns: []model.IndexColumn{{Name: "email", Seq: 1}}},
					// Overlapping index -> IDX_002
					{Name: "idx_org_id", Columns: []model.IndexColumn{{Name: "org_id", Seq: 1}}},
					{Name: "idx_org_id_role", Columns: []model.IndexColumn{{Name: "org_id", Seq: 1}, {Name: "role", Seq: 2}}},
				},
			},
			{
				// FK child lacking index -> IDX_003
				Schema:     "app_db",
				Name:       "orders",
				Kind:       "base_table",
				PrimaryKey: []string{"id"},
				Indexes: []model.Index{
					{Name: "PRIMARY", Columns: []model.IndexColumn{{Name: "id", Seq: 1}}, Primary: true},
				},
			},
		},
		ForeignKeys: []model.ForeignKey{
			{
				Name: "fk_orders_users",
				From: model.ForeignKeyEndpoint{Schema: "app_db", Table: "orders", Columns: []string{"user_id"}},
				To:   model.ForeignKeyEndpoint{Schema: "app_db", Table: "users", Columns: []string{"id"}},
			},
			{
				// Unresolved target table -> SCH_002
				Name: "fk_orders_billing",
				From: model.ForeignKeyEndpoint{Schema: "app_db", Table: "orders", Columns: []string{"billing_id"}},
				To:   model.ForeignKeyEndpoint{Schema: "billing_db", Table: "invoices", Columns: []string{"id"}},
			},
		},
	}

	registry := audit.NewRegistry()
	findings := registry.Run(snap, nil)

	foundRules := make(map[string]bool)
	for _, f := range findings {
		foundRules[f.RuleID] = true
	}

	expectedRules := []string{"SCH_001", "SCH_002", "IDX_001", "IDX_002", "IDX_003"}
	for _, r := range expectedRules {
		if !foundRules[r] {
			t.Errorf("expected rule %s to produce a finding, but it was missing", r)
		}
	}
}
