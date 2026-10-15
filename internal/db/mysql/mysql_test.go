package mysql_test

import (
	"context"
	"testing"

	"github.com/farulwananda/dataprobe/internal/db"
	"github.com/farulwananda/dataprobe/internal/db/mysql"
)

func TestMySQL_EmptyScopeError(t *testing.T) {
	inspector, err := mysql.NewInspector("root:secret@tcp(127.0.0.1:3306)/mysql")
	if err != nil {
		t.Fatalf("unexpected NewInspector error: %v", err)
	}
	defer inspector.Close()

	_, err = inspector.Inspect(context.Background(), db.Scope{Schemas: []string{}})
	if err == nil {
		t.Fatal("expected error when schemas is empty, got nil")
	}
}
