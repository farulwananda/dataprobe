package postgres_test

import (
	"context"
	"testing"

	"github.com/farulwananda/dataprobe/internal/db"
	"github.com/farulwananda/dataprobe/internal/db/postgres"
)

func TestPostgres_EmptyScope(t *testing.T) {
	inspector, err := postgres.NewInspector("postgres://user:pass@127.0.0.1:5432/testdb")
	if err != nil {
		t.Fatalf("unexpected NewInspector error: %v", err)
	}
	defer inspector.Close()

	_, err = inspector.Inspect(context.Background(), db.Scope{Schemas: []string{}})
	if err == nil {
		t.Fatal("expected error when schemas list is empty, got nil")
	}
}

func TestPostgres_ParsePgArray(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []string
	}{
		{
			name:     "plain columns",
			input:    "{id,user_id,created_at}",
			expected: []string{"id", "user_id", "created_at"},
		},
		{
			name:     "quoted columns with commas",
			input:    `{"first,last","middle name",id}`,
			expected: []string{"first,last", "middle name", "id"},
		},
		{
			name:     "expression with arguments and commas",
			input:    `{"substr(title, 1, 3)",id,"coalesce(discount, 0)"}`,
			expected: []string{"substr(title, 1, 3)", "id", "coalesce(discount, 0)"},
		},
		{
			name:     "empty array",
			input:    "{}",
			expected: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := postgres.ExportParsePgArray(tt.input)
			if len(got) != len(tt.expected) {
				t.Fatalf("expected %d elements, got %d (%v)", len(tt.expected), len(got), got)
			}
			for i := range got {
				if got[i] != tt.expected[i] {
					t.Errorf("at index %d: expected %q, got %q", i, tt.expected[i], got[i])
				}
			}
		})
	}
}
