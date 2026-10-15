package diff_test

import (
	"testing"
	"time"

	"github.com/farulwananda/dataprobe/internal/diff"
	"github.com/farulwananda/dataprobe/internal/model"
)

func TestDiff_Compare(t *testing.T) {
	t1 := time.Date(2026, 10, 8, 0, 0, 0, 0, time.UTC)
	t2 := time.Date(2026, 10, 15, 0, 0, 0, 0, time.UTC)

	oldSnap := &model.Snapshot{
		CapturedAt: t1,
		Tables: []model.Table{
			{
				Schema:   "app_db",
				Name:     "users",
				RowCount: model.RowCount{Value: 100},
				StorageBytes: model.StorageBytes{
					Data:    1000000,
					Indexes: 500000,
				},
				Columns: []model.Column{{Name: "id"}, {Name: "email"}},
			},
			{
				Schema:   "app_db",
				Name:     "old_logs",
				RowCount: model.RowCount{Value: 50},
			},
		},
	}

	newSnap := &model.Snapshot{
		CapturedAt: t2,
		Tables: []model.Table{
			{
				Schema:   "app_db",
				Name:     "users",
				RowCount: model.RowCount{Value: 180},
				StorageBytes: model.StorageBytes{
					Data:    1500000,
					Indexes: 600000,
				},
				Columns: []model.Column{{Name: "id"}, {Name: "email"}, {Name: "phone"}},
			},
			{
				Schema:   "app_db",
				Name:     "orders",
				RowCount: model.RowCount{Value: 200},
			},
		},
	}

	d := diff.Compare(oldSnap, newSnap)

	if d.ElapsedDays != 7.0 {
		t.Errorf("expected 7.0 elapsed days, got %v", d.ElapsedDays)
	}
	if d.TablesAdded != 1 {
		t.Errorf("expected 1 added table (orders), got %d", d.TablesAdded)
	}
	if d.TablesRemoved != 1 {
		t.Errorf("expected 1 removed table (old_logs), got %d", d.TablesRemoved)
	}
	if d.TablesModified != 1 {
		t.Errorf("expected 1 modified table (users), got %d", d.TablesModified)
	}
}
