package audit

import (
	"strings"

	"github.com/farulwananda/dataprobe/internal/model"
)

// RuleDuplicateIndex checks for indexes with identical column signatures and uniqueness (IDX_001).
type RuleDuplicateIndex struct{}

func (r *RuleDuplicateIndex) ID() string       { return "IDX_001" }
func (r *RuleDuplicateIndex) Name() string     { return "Duplicate index definition" }
func (r *RuleDuplicateIndex) Category() string { return "indexes" }

func (r *RuleDuplicateIndex) Run(snap *model.Snapshot) []model.Finding {
	var findings []model.Finding
	for _, tbl := range snap.Tables {
		seen := make(map[string]string)
		for _, idx := range tbl.Indexes {
			var colNames []string
			for _, c := range idx.Columns {
				colNames = append(colNames, c.Name)
			}
			sig := strings.Join(colNames, ",")
			if existingIdx, exists := seen[sig]; exists {
				findings = append(findings, model.Finding{
					RuleID:   r.ID(),
					Severity: model.SeverityMedium,
					Status:   model.StatusFinding,
					Scope: model.FindingScope{
						Schema: tbl.Schema,
						Table:  tbl.Name,
						Index:  idx.Name,
					},
					Title:   "Duplicate index definition detected",
					Summary: "Multiple indexes cover the exact same columns in the exact same order.",
					Evidence: map[string]any{
						"duplicate_index": idx.Name,
						"identical_to":    existingIdx,
						"columns":         colNames,
					},
					Measurement: model.MetricSource{
						Quality: model.AccuracyExact,
						Source:  "information_schema.statistics",
					},
					Impact:         "Consumes redundant disk space and incurs write/insert latency without providing query optimization benefits.",
					Recommendation: "Review index usage with your team and drop the redundant index after confirming application workload.",
					AutoFix:        false,
				})
			} else {
				seen[sig] = idx.Name
			}
		}
	}
	return findings
}

// RuleOverlappingIndex checks for left-prefix redundant indexes (IDX_002).
type RuleOverlappingIndex struct{}

func (r *RuleOverlappingIndex) ID() string       { return "IDX_002" }
func (r *RuleOverlappingIndex) Name() string     { return "Potential left-prefix overlapping index" }
func (r *RuleOverlappingIndex) Category() string { return "indexes" }

func (r *RuleOverlappingIndex) Run(snap *model.Snapshot) []model.Finding {
	var findings []model.Finding
	for _, tbl := range snap.Tables {
		for i := range tbl.Indexes {
			idxA := tbl.Indexes[i]
			if idxA.Primary || idxA.Unique {
				continue // Do not flag primary or unique constraints as redundant prefixes
			}
			colsA := getColNames(idxA)

			for j := range tbl.Indexes {
				if i == j {
					continue
				}
				idxB := tbl.Indexes[j]
				colsB := getColNames(idxB)

				// Check if idxA is a strict left prefix of idxB (len(colsB) > len(colsA))
				if len(colsB) > len(colsA) && isPrefix(colsA, colsB) {
					findings = append(findings, model.Finding{
						RuleID:   r.ID(),
						Severity: model.SeverityLow,
						Status:   model.StatusFinding,
						Scope: model.FindingScope{
							Schema: tbl.Schema,
							Table:  tbl.Name,
							Index:  idxA.Name,
						},
						Title:   "Potential left-prefix overlapping index",
						Summary: "Index columns form a strict left prefix of a longer composite index, which may be redundant for B-Tree lookups.",
						Evidence: map[string]any{
							"smaller_index": idxA.Name,
							"smaller_cols":  colsA,
							"longer_index":  idxB.Name,
							"longer_cols":   colsB,
						},
						Measurement: model.MetricSource{
							Quality: model.AccuracyMetadata,
							Source:  "information_schema.statistics",
						},
						Impact:         "May incur unnecessary storage and write overhead.",
						Recommendation: "Verify whether queries rely exclusively on the shorter index before dropping.",
						AutoFix:        false,
					})
					break
				}
			}
		}
	}
	return findings
}

// RuleForeignKeyMissingIndex checks if foreign key child columns lack a supporting leading index (IDX_003).
type RuleForeignKeyMissingIndex struct{}

func (r *RuleForeignKeyMissingIndex) ID() string { return "IDX_003" }
func (r *RuleForeignKeyMissingIndex) Name() string {
	return "Foreign key missing supporting leading index"
}
func (r *RuleForeignKeyMissingIndex) Category() string { return "indexes" }

func (r *RuleForeignKeyMissingIndex) Run(snap *model.Snapshot) []model.Finding {
	tableMap := make(map[string]model.Table)
	for _, t := range snap.Tables {
		tableMap[t.Schema+"."+t.Name] = t
	}

	var findings []model.Finding
	for _, fk := range snap.ForeignKeys {
		tblKey := fk.From.Schema + "." + fk.From.Table
		tbl, exists := tableMap[tblKey]
		if !exists {
			continue
		}

		fkCols := fk.From.Columns
		hasIndex := false

		for _, idx := range tbl.Indexes {
			idxCols := getColNames(idx)
			if isPrefix(fkCols, idxCols) {
				hasIndex = true
				break
			}
		}

		if !hasIndex {
			findings = append(findings, model.Finding{
				RuleID:   r.ID(),
				Severity: model.SeverityHigh,
				Status:   model.StatusFinding,
				Scope: model.FindingScope{
					Schema: fk.From.Schema,
					Table:  fk.From.Table,
				},
				Title:   "Foreign key child columns lack leading index",
				Summary: "Child columns in foreign key do not have an index with those columns as leading prefix, which can cause full table locks or slow cascading operations.",
				Evidence: map[string]any{
					"foreign_key": fk.Name,
					"table":       fk.From.Table,
					"columns":     fkCols,
				},
				Measurement: model.MetricSource{
					Quality: model.AccuracyExact,
					Source:  "information_schema.statistics",
				},
				Impact:         "Deletes or updates on the parent table may require full scans or table-level locks on this child table.",
				Recommendation: "Create an index on the child table starting with the foreign key column(s).",
				AutoFix:        false,
			})
		}
	}
	return findings
}

func getColNames(idx model.Index) []string {
	var cols []string
	for _, c := range idx.Columns {
		cols = append(cols, c.Name)
	}
	return cols
}

func isPrefix(prefix, full []string) bool {
	if len(prefix) > len(full) {
		return false
	}
	for i := range prefix {
		if prefix[i] != full[i] {
			return false
		}
	}
	return true
}
