package diff

import (
	"sort"
	"time"

	"github.com/farulwananda/dataprobe/internal/model"
)

// TableDelta captures size, row, and structural differences between two table states.
type TableDelta struct {
	Schema         string   `json:"schema"`
	Name           string   `json:"name"`
	ChangeType     string   `json:"change_type"` // "added", "removed", "modified", "unchanged"
	OldRowCount    int64    `json:"old_row_count"`
	NewRowCount    int64    `json:"new_row_count"`
	RowDelta       int64    `json:"row_delta"`
	OldDataBytes   int64    `json:"old_data_bytes"`
	NewDataBytes   int64    `json:"new_data_bytes"`
	DataByteDelta  int64    `json:"data_byte_delta"`
	OldIndexBytes  int64    `json:"old_index_bytes"`
	NewIndexBytes  int64    `json:"new_index_bytes"`
	IndexByteDelta int64    `json:"index_byte_delta"`
	TotalByteDelta int64    `json:"total_byte_delta"`
	GrowthPct      float64  `json:"growth_pct"`
	ColumnsAdded   []string `json:"columns_added,omitempty"`
	ColumnsRemoved []string `json:"columns_removed,omitempty"`
}

// SnapshotDiff summarizes the drift and growth between two snapshots.
type SnapshotDiff struct {
	OldCapturedAt   time.Time    `json:"old_captured_at"`
	NewCapturedAt   time.Time    `json:"new_captured_at"`
	ElapsedDays     float64      `json:"elapsed_days"`
	OldTotalBytes   int64        `json:"old_total_bytes"`
	NewTotalBytes   int64        `json:"new_total_bytes"`
	TotalByteDelta  int64        `json:"total_byte_delta"`
	TotalGrowthPct  float64      `json:"total_growth_pct"`
	TablesAdded     int          `json:"tables_added"`
	TablesRemoved   int          `json:"tables_removed"`
	TablesModified  int          `json:"tables_modified"`
	TopGrowthTables []TableDelta `json:"top_growth_tables"`
	TableDeltas     []TableDelta `json:"table_deltas"`
}

// Compare computes the delta between oldSnap and newSnap.
func Compare(oldSnap, newSnap *model.Snapshot) *SnapshotDiff {
	oldTables := make(map[string]model.Table)
	if oldSnap != nil {
		for _, t := range oldSnap.Tables {
			oldTables[t.Schema+"."+t.Name] = t
		}
	}

	newTables := make(map[string]model.Table)
	if newSnap != nil {
		for _, t := range newSnap.Tables {
			newTables[t.Schema+"."+t.Name] = t
		}
	}

	var allKeys []string
	keySet := make(map[string]bool)
	for k := range oldTables {
		if !keySet[k] {
			allKeys = append(allKeys, k)
			keySet[k] = true
		}
	}
	for k := range newTables {
		if !keySet[k] {
			allKeys = append(allKeys, k)
			keySet[k] = true
		}
	}
	sort.Strings(allKeys)

	var deltas []TableDelta
	var oldTotal, newTotal int64
	var addedCount, removedCount, modCount int

	for _, k := range allKeys {
		oldT, hasOld := oldTables[k]
		newT, hasNew := newTables[k]

		if hasOld {
			oldTotal += oldT.StorageBytes.Data + oldT.StorageBytes.Indexes
		}
		if hasNew {
			newTotal += newT.StorageBytes.Data + newT.StorageBytes.Indexes
		}

		delta := TableDelta{}
		if hasOld && !hasNew {
			delta.Schema = oldT.Schema
			delta.Name = oldT.Name
			delta.ChangeType = "removed"
			delta.OldRowCount = oldT.RowCount.Value
			delta.OldDataBytes = oldT.StorageBytes.Data
			delta.OldIndexBytes = oldT.StorageBytes.Indexes
			delta.TotalByteDelta = -(oldT.StorageBytes.Data + oldT.StorageBytes.Indexes)
			removedCount++
		} else if !hasOld && hasNew {
			delta.Schema = newT.Schema
			delta.Name = newT.Name
			delta.ChangeType = "added"
			delta.NewRowCount = newT.RowCount.Value
			delta.NewDataBytes = newT.StorageBytes.Data
			delta.NewIndexBytes = newT.StorageBytes.Indexes
			delta.TotalByteDelta = newT.StorageBytes.Data + newT.StorageBytes.Indexes
			addedCount++
		} else {
			delta.Schema = newT.Schema
			delta.Name = newT.Name
			delta.OldRowCount = oldT.RowCount.Value
			delta.NewRowCount = newT.RowCount.Value
			delta.RowDelta = newT.RowCount.Value - oldT.RowCount.Value

			delta.OldDataBytes = oldT.StorageBytes.Data
			delta.NewDataBytes = newT.StorageBytes.Data
			delta.DataByteDelta = newT.StorageBytes.Data - oldT.StorageBytes.Data

			delta.OldIndexBytes = oldT.StorageBytes.Indexes
			delta.NewIndexBytes = newT.StorageBytes.Indexes
			delta.IndexByteDelta = newT.StorageBytes.Indexes - oldT.StorageBytes.Indexes

			oldSum := oldT.StorageBytes.Data + oldT.StorageBytes.Indexes
			newSum := newT.StorageBytes.Data + newT.StorageBytes.Indexes
			delta.TotalByteDelta = newSum - oldSum

			if oldSum > 0 {
				delta.GrowthPct = (float64(delta.TotalByteDelta) / float64(oldSum)) * 100
			}

			// Column differences
			oldColMap := make(map[string]bool)
			for _, c := range oldT.Columns {
				oldColMap[c.Name] = true
			}
			newColMap := make(map[string]bool)
			for _, c := range newT.Columns {
				newColMap[c.Name] = true
			}
			for c := range newColMap {
				if !oldColMap[c] {
					delta.ColumnsAdded = append(delta.ColumnsAdded, c)
				}
			}
			sort.Strings(delta.ColumnsAdded)

			for c := range oldColMap {
				if !newColMap[c] {
					delta.ColumnsRemoved = append(delta.ColumnsRemoved, c)
				}
			}
			sort.Strings(delta.ColumnsRemoved)
			if delta.TotalByteDelta != 0 || delta.RowDelta != 0 || len(delta.ColumnsAdded) > 0 || len(delta.ColumnsRemoved) > 0 {
				delta.ChangeType = "modified"
				modCount++
			} else {
				delta.ChangeType = "unchanged"
			}
		}

		deltas = append(deltas, delta)
	}

	totalDelta := newTotal - oldTotal
	var totalGrowthPct float64
	if oldTotal > 0 {
		totalGrowthPct = (float64(totalDelta) / float64(oldTotal)) * 100
	}

	var elapsedDays float64
	if oldSnap != nil && newSnap != nil && !oldSnap.CapturedAt.IsZero() && !newSnap.CapturedAt.IsZero() {
		dur := newSnap.CapturedAt.Sub(oldSnap.CapturedAt)
		elapsedDays = dur.Hours() / 24.0
	}

	// Identify top growth tables sorted by positive TotalByteDelta
	var growthList []TableDelta
	for _, d := range deltas {
		if d.TotalByteDelta > 0 {
			growthList = append(growthList, d)
		}
	}
	sort.Slice(growthList, func(i, j int) bool {
		return growthList[i].TotalByteDelta > growthList[j].TotalByteDelta
	})
	topN := 5
	if len(growthList) < topN {
		topN = len(growthList)
	}
	topGrowth := growthList[:topN]

	var oldTime, newTime time.Time
	if oldSnap != nil {
		oldTime = oldSnap.CapturedAt
	}
	if newSnap != nil {
		newTime = newSnap.CapturedAt
	}

	return &SnapshotDiff{
		OldCapturedAt:   oldTime,
		NewCapturedAt:   newTime,
		ElapsedDays:     elapsedDays,
		OldTotalBytes:   oldTotal,
		NewTotalBytes:   newTotal,
		TotalByteDelta:  totalDelta,
		TotalGrowthPct:  totalGrowthPct,
		TablesAdded:     addedCount,
		TablesRemoved:   removedCount,
		TablesModified:  modCount,
		TopGrowthTables: topGrowth,
		TableDeltas:     deltas,
	}
}
