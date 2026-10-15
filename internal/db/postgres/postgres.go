package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"

	"github.com/farulwananda/dataprobe/internal/db"
	"github.com/farulwananda/dataprobe/internal/model"
)

// Inspector implements db.Inspector for PostgreSQL 12+.
type Inspector struct {
	db *sql.DB
}

// NewInspector creates a new PostgreSQL Inspector from a connection DSN or URL.
func NewInspector(dsn string) (*Inspector, error) {
	conn, err := sql.Open("pgx", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed opening PostgreSQL connection: %w", err)
	}

	conn.SetMaxOpenConns(2)
	conn.SetMaxIdleConns(1)
	conn.SetConnMaxLifetime(1 * time.Minute)

	return &Inspector{db: conn}, nil
}

// Close closes the underlying database pool.
func (i *Inspector) Close() error {
	if i.db != nil {
		return i.db.Close()
	}
	return nil
}

// Inspect gathers table, column, index, and foreign key metadata using pg_catalog.
func (i *Inspector) Inspect(ctx context.Context, scope db.Scope) (*model.Snapshot, error) {
	if len(scope.Schemas) == 0 {
		return nil, fmt.Errorf("at least one schema must be provided in scope")
	}

	// 1. Server Version
	var serverVersion string
	_ = i.db.QueryRowContext(ctx, "SHOW server_version").Scan(&serverVersion)

	// Build placeholders ($1, $2, ...)
	schemaPlaceholders := make([]string, len(scope.Schemas))
	schemaArgs := make([]any, len(scope.Schemas))
	for idx, s := range scope.Schemas {
		schemaPlaceholders[idx] = fmt.Sprintf("$%d", idx+1)
		schemaArgs[idx] = s
	}
	inClause := strings.Join(schemaPlaceholders, ",")

	// 2. Query Tables, Estimated Rows (pg_class.reltuples), and Estimated Relation Sizes
	tablesQuery := fmt.Sprintf(`
		SELECT
			n.nspname AS schema_name,
			c.relname AS table_name,
			CASE c.relkind
				WHEN 'r' THEN 'base_table'
				WHEN 'p' THEN 'partitioned_table'
				WHEN 'v' THEN 'view'
				WHEN 'm' THEN 'materialized_view'
				ELSE 'other'
			END AS kind,
			GREATEST(0, c.reltuples::bigint) AS estimated_rows,
			pg_relation_size(c.oid) AS data_bytes,
			pg_indexes_size(c.oid) AS index_bytes,
			COALESCE(obj_description(c.oid, 'pg_class'), '') AS comment
		FROM pg_class c
		JOIN pg_namespace n ON n.oid = c.relnamespace
		WHERE n.nspname IN (%s)
		  AND c.relkind IN ('r', 'p', 'v', 'm')
		ORDER BY n.nspname, c.relname
	`, inClause)

	rows, err := i.db.QueryContext(ctx, tablesQuery, schemaArgs...)
	if err != nil {
		return nil, fmt.Errorf("failed querying pg_class tables: %w", err)
	}
	defer rows.Close()

	tableMap := make(map[string]*model.Table)

	for rows.Next() {
		var schema, name, kind, comment string
		var estimatedRows, dataBytes, indexBytes int64

		if err := rows.Scan(&schema, &name, &kind, &estimatedRows, &dataBytes, &indexBytes, &comment); err != nil {
			return nil, fmt.Errorf("failed scanning pg table row: %w", err)
		}

		if isExcluded(name, scope.ExcludeTables) {
			continue
		}

		tbl := model.Table{
			Schema: schema,
			Name:   name,
			Kind:   kind,
			Engine: "PostgreSQL",
			RowCount: model.RowCount{
				Value:   estimatedRows,
				Quality: model.AccuracyEstimated, // reltuples is a statistics planner estimate
			},
			StorageBytes: model.StorageBytes{
				Data:    dataBytes,
				Indexes: indexBytes,
				Quality: model.AccuracyEstimated,
			},
			Columns:    []model.Column{},
			PrimaryKey: []string{},
			Indexes:    []model.Index{},
			Comment:    comment,
		}

		key := schema + "." + name
		tableMap[key] = &tbl
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	// 3. Query Columns from information_schema.columns
	columnsQuery := fmt.Sprintf(`
		SELECT
			table_schema,
			table_name,
			column_name,
			data_type,
			is_nullable,
			column_default
		FROM information_schema.columns
		WHERE table_schema IN (%s)
		ORDER BY table_schema, table_name, ordinal_position
	`, inClause)

	colRows, err := i.db.QueryContext(ctx, columnsQuery, schemaArgs...)
	if err != nil {
		return nil, fmt.Errorf("failed querying columns: %w", err)
	}
	defer colRows.Close()

	for colRows.Next() {
		var schema, tableName, colName, dataType, isNullable string
		var colDefault sql.NullString

		if err := colRows.Scan(&schema, &tableName, &colName, &dataType, &isNullable, &colDefault); err != nil {
			return nil, fmt.Errorf("failed scanning column row: %w", err)
		}

		key := schema + "." + tableName
		tbl, exists := tableMap[key]
		if !exists {
			continue
		}

		var defaultVal *string
		if colDefault.Valid {
			v := colDefault.String
			defaultVal = &v
		}

		col := model.Column{
			Name:         colName,
			DBType:       dataType,
			Nullable:     strings.ToUpper(isNullable) == "YES",
			DefaultValue: defaultVal,
		}
		tbl.Columns = append(tbl.Columns, col)
	}
	if err := colRows.Err(); err != nil {
		return nil, err
	}

	// 4. Query Indexes and Primary Keys from pg_index + pg_class
	indexesQuery := fmt.Sprintf(`
		SELECT
			n.nspname AS schema_name,
			t.relname AS table_name,
			i.relname AS index_name,
			idx.indisunique AS is_unique,
			idx.indisprimary AS is_primary,
			am.amname AS index_type,
			ARRAY(
				SELECT pg_get_indexdef(idx.indexrelid, k + 1, true)
				FROM generate_subscripts(idx.indkey, 1) as k
				ORDER BY k
			) AS column_names
		FROM pg_index idx
		JOIN pg_class t ON t.oid = idx.indrelid
		JOIN pg_class i ON i.oid = idx.indexrelid
		JOIN pg_namespace n ON n.oid = t.relnamespace
		JOIN pg_am am ON am.oid = i.relam
		WHERE n.nspname IN (%s)
		ORDER BY n.nspname, t.relname, i.relname
	`, inClause)

	idxRows, err := i.db.QueryContext(ctx, indexesQuery, schemaArgs...)
	if err != nil {
		return nil, fmt.Errorf("failed querying pg_index: %w", err)
	}
	defer idxRows.Close()

	for idxRows.Next() {
		var schema, tableName, indexName, indexType string
		var isUnique, isPrimary bool
		var colNames []string

		var rawCols string
		if err := idxRows.Scan(&schema, &tableName, &indexName, &isUnique, &isPrimary, &indexType, &rawCols); err != nil {
			return nil, fmt.Errorf("failed scanning pg index row: %w", err)
		}

		key := schema + "." + tableName
		tbl, exists := tableMap[key]
		if !exists {
			continue
		}

		colNames = parsePgArray(rawCols)

		if isPrimary {
			tbl.PrimaryKey = append([]string(nil), colNames...)
			for idxC := range tbl.Columns {
				for _, pkCol := range colNames {
					if tbl.Columns[idxC].Name == pkCol {
						tbl.Columns[idxC].IsPrimaryKey = true
					}
				}
			}
		}

		var idxCols []model.IndexColumn
		for seq, cName := range colNames {
			idxCols = append(idxCols, model.IndexColumn{
				Name: cName,
				Seq:  seq + 1,
			})
		}

		tbl.Indexes = append(tbl.Indexes, model.Index{
			Name:      indexName,
			Columns:   idxCols,
			Unique:    isUnique,
			Primary:   isPrimary,
			IndexType: indexType,
		})
	}
	if err := idxRows.Err(); err != nil {
		return nil, err
	}

	// 5. Query Foreign Keys via pg_constraint
	fkQuery := fmt.Sprintf(`
		SELECT
			con.conname AS constraint_name,
			src_ns.nspname AS from_schema,
			src_tbl.relname AS from_table,
			ARRAY(
				SELECT att.attname
				FROM unnest(con.conkey) WITH ORDINALITY AS k(attnum, ord)
				JOIN pg_attribute att ON att.attrelid = con.conrelid AND att.attnum = k.attnum
				ORDER BY k.ord
			) AS from_columns,
			tgt_ns.nspname AS to_schema,
			tgt_tbl.relname AS to_table,
			ARRAY(
				SELECT att.attname
				FROM unnest(con.confkey) WITH ORDINALITY AS k(attnum, ord)
				JOIN pg_attribute att ON att.attrelid = con.confrelid AND att.attnum = k.attnum
				ORDER BY k.ord
			) AS to_columns,
			CASE con.confdeltype
				WHEN 'a' THEN 'NO ACTION'
				WHEN 'r' THEN 'RESTRICT'
				WHEN 'c' THEN 'CASCADE'
				WHEN 'n' THEN 'SET NULL'
				WHEN 'd' THEN 'SET DEFAULT'
				ELSE 'RESTRICT'
			END AS on_delete,
			CASE con.confupdtype
				WHEN 'a' THEN 'NO ACTION'
				WHEN 'r' THEN 'RESTRICT'
				WHEN 'c' THEN 'CASCADE'
				WHEN 'n' THEN 'SET NULL'
				WHEN 'd' THEN 'SET DEFAULT'
				ELSE 'RESTRICT'
			END AS on_update
		FROM pg_constraint con
		JOIN pg_class src_tbl ON src_tbl.oid = con.conrelid
		JOIN pg_namespace src_ns ON src_ns.oid = src_tbl.relnamespace
		JOIN pg_class tgt_tbl ON tgt_tbl.oid = con.confrelid
		JOIN pg_namespace tgt_ns ON tgt_ns.oid = tgt_tbl.relnamespace
		WHERE con.contype = 'f'
		  AND src_ns.nspname IN (%s)
		ORDER BY src_ns.nspname, src_tbl.relname, con.conname
	`, inClause)

	fkRows, err := i.db.QueryContext(ctx, fkQuery, schemaArgs...)
	if err != nil {
		return nil, fmt.Errorf("failed querying pg_constraint: %w", err)
	}
	defer fkRows.Close()

	var foreignKeys []model.ForeignKey

	for fkRows.Next() {
		var constraintName, fromSchema, fromTable, toSchema, toTable, onDelete, onUpdate string
		var rawFromCols, rawToCols string

		if err := fkRows.Scan(&constraintName, &fromSchema, &fromTable, &rawFromCols, &toSchema, &toTable, &rawToCols, &onDelete, &onUpdate); err != nil {
			return nil, fmt.Errorf("failed scanning pg foreign key row: %w", err)
		}

		if isExcluded(fromTable, scope.ExcludeTables) {
			continue
		}

		fromCols := parsePgArray(rawFromCols)
		toCols := parsePgArray(rawToCols)

		foreignKeys = append(foreignKeys, model.ForeignKey{
			Name: constraintName,
			From: model.ForeignKeyEndpoint{
				Schema:  fromSchema,
				Table:   fromTable,
				Columns: fromCols,
			},
			To: model.ForeignKeyEndpoint{
				Schema:  toSchema,
				Table:   toTable,
				Columns: toCols,
			},
			Origin:         "declared",
			OnDelete:       onDelete,
			OnUpdate:       onUpdate,
			ConstraintName: constraintName,
		})
	}
	if err := fkRows.Err(); err != nil {
		return nil, err
	}

	var tables []model.Table
	for _, tbl := range tableMap {
		tables = append(tables, *tbl)
	}

	sort.Slice(tables, func(a, b int) bool {
		if tables[a].Schema != tables[b].Schema {
			return tables[a].Schema < tables[b].Schema
		}
		return tables[a].Name < tables[b].Name
	})

	alias := scope.Alias
	if alias == "" && len(scope.Schemas) > 0 {
		alias = scope.Schemas[0]
	}

	return &model.Snapshot{
		SchemaVersion: 1,
		ToolVersion:   "0.1.0",
		CapturedAt:    time.Now().UTC().Truncate(time.Second),
		Source: model.SourceMetadata{
			Engine:        "postgres",
			ServerVersion: serverVersion,
			Alias:         alias,
			Schemas:       scope.Schemas,
		},
		Capabilities: model.Capabilities{
			TableStatistics:    true,
			WorkloadStatistics: false,
			FullIntegrityScan:  false,
		},
		Tables:      tables,
		ForeignKeys: foreignKeys,
		Findings:    []model.Finding{},
		Warnings:    []string{},
	}, nil
}

func parsePgArray(raw string) []string {
	raw = strings.TrimSpace(raw)
	raw = strings.TrimPrefix(raw, "{")
	raw = strings.TrimSuffix(raw, "}")
	if raw == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	res := make([]string, len(parts))
	for idx, p := range parts {
		res[idx] = strings.Trim(strings.TrimSpace(p), `"`)
	}
	return res
}

func isExcluded(table string, patterns []string) bool {
	for _, pattern := range patterns {
		pattern = strings.TrimSpace(pattern)
		if pattern == "" {
			continue
		}
		if matched, _ := filepath.Match(pattern, table); matched {
			return true
		}
	}
	return false
}
