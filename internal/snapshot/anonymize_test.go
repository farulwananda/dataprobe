package snapshot_test

import (
	"strings"
	"testing"
	"time"

	"github.com/farulwananda/dataprobe/internal/model"
	"github.com/farulwananda/dataprobe/internal/snapshot"
)

func TestSnapshot_Anonymize_NoLeaksAndNoCallerMutation(t *testing.T) {
	originalColName := "card_id"
	originalCols := []string{originalColName}

	snap := &model.Snapshot{
		SchemaVersion: 1,
		ToolVersion:   "0.1.0",
		CapturedAt:    time.Now().UTC(),
		Source: model.SourceMetadata{
			Engine:  "postgres",
			Alias:   "secret-bank-db",
			Schemas: []string{"customers_vault"},
		},
		Tables: []model.Table{
			{
				Schema:     "customers_vault",
				Name:       "credit_cards",
				Kind:       "base_table",
				Comment:    "Confidential customer PAN and CVV data",
				PrimaryKey: []string{originalColName},
				Columns: []model.Column{
					{Name: originalColName, DBType: "bigint", IsPrimaryKey: true},
					{Name: "cardholder_name", DBType: "varchar(255)"},
				},
				Indexes: []model.Index{
					{
						// SEC-01 test: Postgres style PK index name
						Name:    "credit_cards_pkey",
						Primary: true,
						Columns: []model.IndexColumn{{Name: originalColName, Seq: 1}},
					},
					{
						Name:    "idx_cardholder_name",
						Columns: []model.IndexColumn{{Name: "cardholder_name", Seq: 1}},
					},
				},
			},
		},
		ForeignKeys: []model.ForeignKey{
			{
				// SEC-02 & SEC-03 test: raw constraint name & external/missing target table
				Name:           "fk_cards_users",
				ConstraintName: "fk_customers_vault_credit_cards_users",
				From: model.ForeignKeyEndpoint{
					Schema:  "customers_vault",
					Table:   "credit_cards",
					Columns: originalCols,
				},
				To: model.ForeignKeyEndpoint{
					Schema:  "auth_schema",
					Table:   "auth_users",
					Columns: []string{"user_id"},
				},
			},
		},
	}

	anon := snapshot.Anonymize(snap)

	// 1. Check alias anonymized
	if anon.Source.Alias != "anonymized-source" {
		t.Errorf("expected anonymized alias, got %s", anon.Source.Alias)
	}

	// 2. SEC-01 check: Primary key index name must NOT leak original table name
	var pkIdxName string
	for _, idx := range anon.Tables[0].Indexes {
		if idx.Primary {
			pkIdxName = idx.Name
		}
	}
	if strings.Contains(pkIdxName, "credit_cards") {
		t.Errorf("SEC-01 leak: primary key index name was not anonymized: %s", pkIdxName)
	}
	if !strings.HasPrefix(pkIdxName, "pk_") {
		t.Errorf("expected pk_ prefix, got %s", pkIdxName)
	}

	// 3. SEC-02 check: ConstraintName must be anonymized
	if strings.Contains(anon.ForeignKeys[0].ConstraintName, "credit_cards") || strings.Contains(anon.ForeignKeys[0].ConstraintName, "customers_vault") {
		t.Errorf("SEC-02 leak: ConstraintName retained sensitive info: %s", anon.ForeignKeys[0].ConstraintName)
	}

	// 4. SEC-03 check: External/excluded target table and schema must be anonymized
	if anon.ForeignKeys[0].To.Table == "auth_users" || strings.Contains(anon.ForeignKeys[0].To.Table, "auth") {
		t.Errorf("SEC-03 leak: external target table was not anonymized: %s", anon.ForeignKeys[0].To.Table)
	}
	if anon.ForeignKeys[0].To.Schema == "auth_schema" || strings.Contains(anon.ForeignKeys[0].To.Schema, "auth") {
		t.Errorf("SEC-03 leak: external target schema was not anonymized: %s", anon.ForeignKeys[0].To.Schema)
	}

	// 5. DAT-02 check: Original caller snapshot must NOT be mutated in-place
	if snap.ForeignKeys[0].From.Columns[0] != originalColName {
		t.Errorf("DAT-02 regression: caller snapshot slice was mutated in-place: got %s, want %s",
			snap.ForeignKeys[0].From.Columns[0], originalColName)
	}
}
