package snapshot

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"

	"github.com/farulwananda/dataprobe/internal/model"
)

// Anonymize masks schema, table, column, index, and foreign key names in a Snapshot
// using deterministic pseudonyms while strictly preserving structural relationships and sizes.
// This allows engineers to share snapshots with third parties or open issues without leaking internal table names.
func Anonymize(s *model.Snapshot) *model.Snapshot {
	if s == nil {
		return nil
	}

	clone := *s
	clone.Source.Alias = "anonymized-source"

	schemaMap := make(map[string]string)
	tableMap := make(map[string]string) // key: schema.table -> anonTable

	for idx, sch := range s.Source.Schemas {
		anonSch := fmt.Sprintf("schema_%d", idx+1)
		schemaMap[sch] = anonSch
	}
	clone.Source.Schemas = make([]string, len(s.Source.Schemas))
	for idx, sch := range s.Source.Schemas {
		clone.Source.Schemas[idx] = schemaMap[sch]
	}

	// Anonymize tables
	clone.Tables = make([]model.Table, len(s.Tables))
	for tIdx, t := range s.Tables {
		anonSchema := schemaMap[t.Schema]
		if anonSchema == "" {
			anonSchema = "schema_1"
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

		// Primary Key
		tblCopy.PrimaryKey = make([]string, len(t.PrimaryKey))
		for pkIdx, pk := range t.PrimaryKey {
			if anon, ok := colMap[pk]; ok {
				tblCopy.PrimaryKey[pkIdx] = anon
			} else {
				tblCopy.PrimaryKey[pkIdx] = pk
			}
		}

		// Indexes
		tblCopy.Indexes = make([]model.Index, len(t.Indexes))
		for iIdx, idx := range t.Indexes {
			idxCopy := idx
			if !idx.Primary {
				idxCopy.Name = fmt.Sprintf("idx_%s_%02d", anonTable, iIdx+1)
			}
			idxCopy.Columns = make([]model.IndexColumn, len(idx.Columns))
			for cIdx, ic := range idx.Columns {
				icCopy := ic
				if anon, ok := colMap[ic.Name]; ok {
					icCopy.Name = anon
				}
				idxCopy.Columns[cIdx] = icCopy
			}
			tblCopy.Indexes[iIdx] = idxCopy
		}

		clone.Tables[tIdx] = tblCopy
	}

	// Anonymize foreign keys
	clone.ForeignKeys = make([]model.ForeignKey, len(s.ForeignKeys))
	for fkIdx, fk := range s.ForeignKeys {
		fkCopy := fk
		fkCopy.Name = fmt.Sprintf("fk_%03d", fkIdx+1)

		fromOldKey := fk.From.Schema + "." + fk.From.Table
		if anonTable, ok := tableMap[fromOldKey]; ok {
			fkCopy.From.Table = anonTable
		}
		if anonSch, ok := schemaMap[fk.From.Schema]; ok {
			fkCopy.From.Schema = anonSch
		}

		toOldKey := fk.To.Schema + "." + fk.To.Table
		if anonTable, ok := tableMap[toOldKey]; ok {
			fkCopy.To.Table = anonTable
		}
		if anonSch, ok := schemaMap[fk.To.Schema]; ok {
			fkCopy.To.Schema = anonSch
		}

		// Columns mapping based on position
		for cIdx := range fk.From.Columns {
			fkCopy.From.Columns[cIdx] = fmt.Sprintf("col_%02d", cIdx+1)
		}
		for cIdx := range fk.To.Columns {
			fkCopy.To.Columns[cIdx] = fmt.Sprintf("col_%02d", cIdx+1)
		}

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
