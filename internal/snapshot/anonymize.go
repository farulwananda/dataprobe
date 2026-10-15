package snapshot

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"

	"github.com/farulwananda/dataprobe/internal/model"
)

// Anonymize masks schema, table, column, index, and foreign key names in a Snapshot
// using deterministic pseudonyms while strictly preserving structural relationships and sizes.
// It deep-copies all slices so caller data is never mutated in-place.
func Anonymize(s *model.Snapshot) *model.Snapshot {
	if s == nil {
		return nil
	}

	clone := *s
	clone.Source.Alias = "anonymized-source"

	schemaMap := make(map[string]string)
	tableMap := make(map[string]string)               // key: schema.table -> anonTable
	tableColMap := make(map[string]map[string]string) // key: schema.table -> (colName -> anonCol)

	for idx, sch := range s.Source.Schemas {
		anonSch := fmt.Sprintf("schema_%d", idx+1)
		schemaMap[sch] = anonSch
	}
	clone.Source.Schemas = make([]string, len(s.Source.Schemas))
	for idx, sch := range s.Source.Schemas {
		clone.Source.Schemas[idx] = schemaMap[sch]
	}

	// 1. Anonymize tables and collect accurate column mappings
	clone.Tables = make([]model.Table, len(s.Tables))
	for tIdx, t := range s.Tables {
		anonSchema := schemaMap[t.Schema]
		if anonSchema == "" {
			anonSchema = fmt.Sprintf("schema_%d", len(schemaMap)+1)
			schemaMap[t.Schema] = anonSchema
		}
		anonTable := fmt.Sprintf("tbl_%03d", tIdx+1)
		fullOldKey := t.Schema + "." + t.Name
		tableMap[fullOldKey] = anonTable

		tblCopy := t
		tblCopy.Schema = anonSchema
		tblCopy.Name = anonTable
		tblCopy.Comment = ""

		// Columns
		colMap := make(map[string]string)
		tblCopy.Columns = make([]model.Column, len(t.Columns))
		for cIdx, col := range t.Columns {
			anonCol := fmt.Sprintf("col_%02d", cIdx+1)
			colMap[col.Name] = anonCol

			colCopy := col
			colCopy.Name = anonCol
			colCopy.Comment = ""
			colCopy.DefaultValue = nil // Strip potential business defaults
			tblCopy.Columns[cIdx] = colCopy
		}
		tableColMap[fullOldKey] = colMap

		// Primary Key (deep copy)
		tblCopy.PrimaryKey = make([]string, len(t.PrimaryKey))
		for pkIdx, pk := range t.PrimaryKey {
			if anon, ok := colMap[pk]; ok {
				tblCopy.PrimaryKey[pkIdx] = anon
			} else {
				tblCopy.PrimaryKey[pkIdx] = fmt.Sprintf("pk_col_%02d", pkIdx+1)
			}
		}

		// Indexes (deep copy & SEC-01 fix: anonymize primary key index names too)
		tblCopy.Indexes = make([]model.Index, len(t.Indexes))
		for iIdx, idx := range t.Indexes {
			idxCopy := idx
			if idx.Primary {
				idxCopy.Name = fmt.Sprintf("pk_%s", anonTable)
			} else {
				idxCopy.Name = fmt.Sprintf("idx_%s_%02d", anonTable, iIdx+1)
			}
			idxCopy.Columns = make([]model.IndexColumn, len(idx.Columns))
			for cIdx, ic := range idx.Columns {
				icCopy := ic
				if anon, ok := colMap[ic.Name]; ok {
					icCopy.Name = anon
				} else {
					// Handle expression or functional index column text
					icCopy.Name = fmt.Sprintf("expr_col_%02d", cIdx+1)
				}
				idxCopy.Columns[cIdx] = icCopy
			}
			tblCopy.Indexes[iIdx] = idxCopy
		}

		clone.Tables[tIdx] = tblCopy
	}

	// 2. Anonymize foreign keys (SEC-02, SEC-03, DAT-02 fixes)
	clone.ForeignKeys = make([]model.ForeignKey, len(s.ForeignKeys))
	for fkIdx, fk := range s.ForeignKeys {
		fkCopy := fk
		fkCopy.Name = fmt.Sprintf("fk_%03d", fkIdx+1)
		if fkCopy.ConstraintName != "" {
			fkCopy.ConstraintName = fkCopy.Name
		}

		fromOldKey := fk.From.Schema + "." + fk.From.Table
		if anonTable, ok := tableMap[fromOldKey]; ok {
			fkCopy.From.Table = anonTable
		} else {
			extTable := fmt.Sprintf("ext_tbl_%03d", len(tableMap)+1)
			tableMap[fromOldKey] = extTable
			fkCopy.From.Table = extTable
		}
		if anonSch, ok := schemaMap[fk.From.Schema]; ok {
			fkCopy.From.Schema = anonSch
		} else {
			extSch := fmt.Sprintf("ext_schema_%d", len(schemaMap)+1)
			schemaMap[fk.From.Schema] = extSch
			fkCopy.From.Schema = extSch
		}

		toOldKey := fk.To.Schema + "." + fk.To.Table
		if anonTable, ok := tableMap[toOldKey]; ok {
			fkCopy.To.Table = anonTable
		} else {
			extTable := fmt.Sprintf("ext_tbl_%03d", len(tableMap)+1)
			tableMap[toOldKey] = extTable
			fkCopy.To.Table = extTable
		}
		if anonSch, ok := schemaMap[fk.To.Schema]; ok {
			fkCopy.To.Schema = anonSch
		} else {
			extSch := fmt.Sprintf("ext_schema_%d", len(schemaMap)+1)
			schemaMap[fk.To.Schema] = extSch
			fkCopy.To.Schema = extSch
		}

		// Allocate new slices so original caller snapshot is NOT mutated (DAT-02 fix)
		fromCols := make([]string, len(fk.From.Columns))
		fromColMap := tableColMap[fromOldKey]
		for cIdx, cName := range fk.From.Columns {
			if fromColMap != nil && fromColMap[cName] != "" {
				fromCols[cIdx] = fromColMap[cName]
			} else {
				fromCols[cIdx] = fmt.Sprintf("col_%02d", cIdx+1)
			}
		}
		fkCopy.From.Columns = fromCols

		toCols := make([]string, len(fk.To.Columns))
		toColMap := tableColMap[toOldKey]
		for cIdx, cName := range fk.To.Columns {
			if toColMap != nil && toColMap[cName] != "" {
				toCols[cIdx] = toColMap[cName]
			} else {
				toCols[cIdx] = fmt.Sprintf("col_%02d", cIdx+1)
			}
		}
		fkCopy.To.Columns = toCols

		clone.ForeignKeys[fkIdx] = fkCopy
	}

	// Clear findings or warnings that might leak business names
	clone.Findings = nil
	clone.Warnings = nil

	Normalize(&clone)
	return &clone
}

// ShortHash produces an 8-character deterministic hex hash.
func ShortHash(s string) string {
	h := sha256.Sum256([]byte(s))
	return hex.EncodeToString(h[:4])
}
