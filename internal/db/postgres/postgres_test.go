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
