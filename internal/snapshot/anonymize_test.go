package snapshot_test

import (
	"strings"
	"testing"
	"time"

	"github.com/farulwananda/dataprobe/internal/model"
	"github.com/farulwananda/dataprobe/internal/snapshot"
)

func TestSnapshot_Anonymize(t *testing.T) {
	snap := &model.Snapshot{
		SchemaVersion: 1,
		ToolVersion:   "0.1.0",
		CapturedAt:    time.Now().UTC(),
		Source: model.SourceMetadata{
			Engine:  "mysql",
			Alias:   "secret-bank-db",
			Schemas: []string{"customers_vault"},
		},
		Tables: []model.Table{
			{
				Schema:     "customers_vault",
				Name:       "credit_cards",
				Kind:       "base_table",
				Comment:    "Confidential customer PAN and CVV data",
				PrimaryKey: []string{"card_id"},
				Columns: []model.Column{
					{Name: "card_id", DBType: "bigint", IsPrimaryKey: true},
					{Name: "cardholder_name", DBType: "varchar(255)"},
				},
				Indexes: []model.Index{
					{
						Name:    "idx_cardholder_name",
						Columns: []model.IndexColumn{{Name: "cardholder_name", Seq: 1}},
					},
				},
			},
		},
		ForeignKeys: []model.ForeignKey{
			{
				Name: "fk_cards_users",
				From: model.ForeignKeyEndpoint{
					Schema:  "customers_vault",
					Table:   "credit_cards",
					Columns: []string{"card_id"},
				},
				To: model.ForeignKeyEndpoint{
					Schema:  "customers_vault",
					Table:   "users",
					Columns: []string{"id"},
				},
			},
		},
	}

	anon := snapshot.Anonymize(snap)

	if anon.Source.Alias != "anonymized-source" {
		t.Errorf("expected anonymized alias, got %s", anon.Source.Alias)
	}

	if anon.Tables[0].Name == "credit_cards" || strings.Contains(anon.Tables[0].Name, "credit") {
		t.Errorf("table name was not anonymized: %s", anon.Tables[0].Name)
	}
	if anon.Tables[0].Columns[1].Name == "cardholder_name" {
		t.Errorf("column name was not anonymized: %s", anon.Tables[0].Columns[1].Name)
	}
	if anon.Tables[0].Comment != "" {
		t.Errorf("comment should be stripped, got: %s", anon.Tables[0].Comment)
	}
	if anon.ForeignKeys[0].Name == "fk_cards_users" {
		t.Errorf("foreign key name was not anonymized: %s", anon.ForeignKeys[0].Name)
	}
}
