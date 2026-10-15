package mysql

import (
	"context"
	"database/sql"
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"time"

	_ "github.com/go-sql-driver/mysql"

	"github.com/farulwananda/dataprobe/internal/db"
	"github.com/farulwananda/dataprobe/internal/model"
)

// Inspector implements db.Inspector for MySQL 5.7+ and 8.0+.
type Inspector struct {
	db *sql.DB
}

// NewInspector creates a new MySQL Inspector from a connection DSN.
func NewInspector(dsn string) (*Inspector, error) {
	conn, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed opening MySQL connection: %w", err)
	}

	// Conservative connection pool settings for production safety
	conn.SetMaxOpenConns(2)
	conn.SetMaxIdleConns(1)
	conn.SetConnMaxLifetime(1 * time.Minute)

	return &Inspector{db: conn}, nil
}

// Close closes the underlying connection pool.
func (i *Inspector) Close() error {
	if i.db != nil {
		return i.db.Close()
	}
	return nil
}

// Inspect collects table, column, index, and foreign key metadata via bounded INFORMATION_SCHEMA bulk queries.
func (i *Inspector) Inspect(ctx context.Context, scope db.Scope) (*model.Snapshot, error) {
	if len(scope.Schemas) == 0 {
		return nil, fmt.Errorf("at least one schema must be provided in scope")
	}

	// 1. Fetch server version
	var serverVersion string
	_ = i.db.QueryRowContext(ctx, "SELECT VERSION()").Scan(&serverVersion)

	// Build parameter placeholders for IN (...)
	schemaPlaceholders := make([]string, len(scope.Schemas))
	schemaArgs := make([]any, len(scope.Schemas))
	for idx, s := range scope.Schemas {
		schemaPlaceholders[idx] = "?"
		schemaArgs[idx] = s
	}
	inClause := strings.Join(schemaPlaceholders, ",")

	// 2. Query tables and row/storage estimates
	tablesQuery := fmt.Sprintf(`
		SELECT 
			TABLE_SCHEMA,
			TABLE_NAME,
			TABLE_TYPE,
			COALESCE(ENGINE, ''),
			COALESCE(TABLE_ROWS, 0),
			COALESCE(DATA_LENGTH, 0),
			COALESCE(INDEX_LENGTH, 0),
			COALESCE(TABLE_COMMENT, '')
		FROM INFORMATION_SCHEMA.TABLES
		WHERE TABLE_SCHEMA IN (%s)
		ORDER BY TABLE_SCHEMA, TABLE_NAME
	`, inClause)

	rows, err := i.db.QueryContext(ctx, tablesQuery, schemaArgs...)
	if err != nil {
		return nil, fmt.Errorf("failed querying INFORMATION_SCHEMA.TABLES: %w", err)
	}
	defer rows.Close()

	tableMap := make(map[string]*model.Table)
	var tables []model.Table

	for rows.Next() {
		var schema, name, tableType, engine, comment string
		var tableRows, dataLength, indexLength int64

		if err := rows.Scan(&schema, &name, &tableType, &engine, &tableRows, &dataLength, &indexLength, &comment); err != nil {
			return nil, fmt.Errorf("failed scanning table row: %w", err)
		}

		if isExcluded(name, scope.ExcludeTables) {
			continue
		}

		kind := "base_table"
		if strings.Contains(strings.ToUpper(tableType), "VIEW") {
			kind = "view"
		}

		tbl := model.Table{
			Schema: schema,
			Name:   name,
			Kind:   kind,
			Engine: engine,
			RowCount: model.RowCount{
				Value:   tableRows,
				Quality: model.AccuracyEstimated, // InnoDB TABLE_ROWS is an estimate
			},
			StorageBytes: model.StorageBytes{
				Data:    dataLength,
				Indexes: indexLength,
				Quality: model.AccuracyEstimated, // Storage allocation stats
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

	// 3. Query columns
	columnsQuery := fmt.Sprintf(`
		SELECT
			TABLE_SCHEMA,
			TABLE_NAME,
			COLUMN_NAME,
			COLUMN_TYPE,
			IS_NULLABLE,
			COLUMN_DEFAULT,
			COLUMN_KEY,
			EXTRA,
			COALESCE(COLUMN_COMMENT, '')
		FROM INFORMATION_SCHEMA.COLUMNS
		WHERE TABLE_SCHEMA IN (%s)
		ORDER BY TABLE_SCHEMA, TABLE_NAME, ORDINAL_POSITION
	`, inClause)

	colRows, err := i.db.QueryContext(ctx, columnsQuery, schemaArgs...)
	if err != nil {
		return nil, fmt.Errorf("failed querying INFORMATION_SCHEMA.COLUMNS: %w", err)
	}
	defer colRows.Close()

	for colRows.Next() {
		var schema, tableName, colName, colType, isNullable, colKey, extra, comment string
		var colDefault sql.NullString

		if err := colRows.Scan(&schema, &tableName, &colName, &colType, &isNullable, &colDefault, &colKey, &extra, &comment); err != nil {
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

		isPK := strings.ToUpper(colKey) == "PRI"
		if isPK {
			tbl.PrimaryKey = append(tbl.PrimaryKey, colName)
		}

		col := model.Column{
			Name:          colName,
			DBType:        colType,
			Nullable:      strings.ToUpper(isNullable) == "YES",
			DefaultValue:  defaultVal,
			IsPrimaryKey:  isPK,
			AutoIncrement: strings.Contains(strings.ToLower(extra), "auto_increment"),
			Comment:       comment,
		}

		tbl.Columns = append(tbl.Columns, col)
	}
	if err := colRows.Err(); err != nil {
		return nil, err
	}

	// 4. Query indexes via INFORMATION_SCHEMA.STATISTICS
	indexesQuery := fmt.Sprintf(`
		SELECT
			TABLE_SCHEMA,
			TABLE_NAME,
			INDEX_NAME,
			NON_UNIQUE,
			SEQ_IN_INDEX,
			COLUMN_NAME,
			COALESCE(COLLATION, ''),
			INDEX_TYPE
		FROM INFORMATION_SCHEMA.STATISTICS
		WHERE TABLE_SCHEMA IN (%s)
		ORDER BY TABLE_SCHEMA, TABLE_NAME, INDEX_NAME, SEQ_IN_INDEX
	`, inClause)

	idxRows, err := i.db.QueryContext(ctx, indexesQuery, schemaArgs...)
	if err != nil {
		return nil, fmt.Errorf("failed querying INFORMATION_SCHEMA.STATISTICS: %w", err)
	}
	defer idxRows.Close()

	type indexCollector struct {
		name      string
		nonUnique int
		indexType string
		columns   []model.IndexColumn
	}
	tableIndexMap := make(map[string]map[string]*indexCollector)

	for idxRows.Next() {
		var schema, tableName, idxName, collation, idxType string
		var colName sql.NullString
		var nonUnique, seqInIndex int

		if err := idxRows.Scan(&schema, &tableName, &idxName, &nonUnique, &seqInIndex, &colName, &collation, &idxType); err != nil {
			return nil, fmt.Errorf("failed scanning index row: %w", err)
		}
		key := schema + "." + tableName
		if _, exists := tableMap[key]; !exists {
			continue
		}

		if _, ok := tableIndexMap[key]; !ok {
			tableIndexMap[key] = make(map[string]*indexCollector)
		}

		collector, exists := tableIndexMap[key][idxName]
		if !exists {
			collector = &indexCollector{
				name:      idxName,
				nonUnique: nonUnique,
				indexType: idxType,
				columns:   []model.IndexColumn{},
			}
			tableIndexMap[key][idxName] = collector
		}
		cName := colName.String
		if !colName.Valid || cName == "" {
			cName = fmt.Sprintf("expr_col_%d", seqInIndex)
		}

		collector.columns = append(collector.columns, model.IndexColumn{
			Name:      cName,
			Seq:       seqInIndex,
			Collation: collation,
		})
	}
	if err := idxRows.Err(); err != nil {
		return nil, err
	}

	// Attach assembled indexes to tables
	for key, idxMap := range tableIndexMap {
		tbl, ok := tableMap[key]
		if !ok {
			continue
		}
		for _, col := range idxMap {
			tbl.Indexes = append(tbl.Indexes, model.Index{
				Name:      col.name,
				Columns:   col.columns,
				Unique:    col.nonUnique == 0,
				Primary:   strings.ToUpper(col.name) == "PRIMARY",
				IndexType: col.indexType,
			})
		}
	}

	// 5. Query Foreign Keys via KEY_COLUMN_USAGE + REFERENTIAL_CONSTRAINTS
	fkQuery := fmt.Sprintf(`
		SELECT
			k.CONSTRAINT_NAME,
			k.TABLE_SCHEMA,
			k.TABLE_NAME,
			k.COLUMN_NAME,
			k.REFERENCED_TABLE_SCHEMA,
			k.REFERENCED_TABLE_NAME,
			k.REFERENCED_COLUMN_NAME,
			k.ORDINAL_POSITION,
			COALESCE(r.DELETE_RULE, 'RESTRICT'),
			COALESCE(r.UPDATE_RULE, 'RESTRICT')
		FROM INFORMATION_SCHEMA.KEY_COLUMN_USAGE k
		LEFT JOIN INFORMATION_SCHEMA.REFERENTIAL_CONSTRAINTS r
			ON k.CONSTRAINT_SCHEMA = r.CONSTRAINT_SCHEMA
			AND k.CONSTRAINT_NAME = r.CONSTRAINT_NAME
		WHERE k.TABLE_SCHEMA IN (%s)
			AND k.REFERENCED_TABLE_NAME IS NOT NULL
		ORDER BY k.CONSTRAINT_SCHEMA, k.CONSTRAINT_NAME, k.ORDINAL_POSITION
	`, inClause)

	fkRows, err := i.db.QueryContext(ctx, fkQuery, schemaArgs...)
	if err != nil {
		return nil, fmt.Errorf("failed querying INFORMATION_SCHEMA.KEY_COLUMN_USAGE: %w", err)
	}
	defer fkRows.Close()

	type fkCollector struct {
		constraintName string
		fromSchema     string
		fromTable      string
		fromCols       []string
		toSchema       string
		toTable        string
		toCols         []string
		onDelete       string
		onUpdate       string
	}
	fkMap := make(map[string]*fkCollector)

	for fkRows.Next() {
		var constraintName, fromSchema, fromTable, fromCol string
		var toSchema, toTable, toCol, onDelete, onUpdate string
		var ordinal int

		if err := fkRows.Scan(&constraintName, &fromSchema, &fromTable, &fromCol, &toSchema, &toTable, &toCol, &ordinal, &onDelete, &onUpdate); err != nil {
			return nil, fmt.Errorf("failed scanning foreign key row: %w", err)
		}

		if isExcluded(fromTable, scope.ExcludeTables) || isExcluded(toTable, scope.ExcludeTables) {
			continue
		}

		// Composite key mapping identifier: schema + table + constraint
		key := fromSchema + "." + fromTable + "." + constraintName
		collector, exists := fkMap[key]
		if !exists {
			collector = &fkCollector{
				constraintName: constraintName,
				fromSchema:     fromSchema,
				fromTable:      fromTable,
				fromCols:       []string{},
				toSchema:       toSchema,
				toTable:        toTable,
				toCols:         []string{},
				onDelete:       onDelete,
				onUpdate:       onUpdate,
			}
			fkMap[key] = collector
		}

		collector.fromCols = append(collector.fromCols, fromCol)
		collector.toCols = append(collector.toCols, toCol)
	}
	if err := fkRows.Err(); err != nil {
		return nil, err
	}

	var foreignKeys []model.ForeignKey
	for _, f := range fkMap {
		foreignKeys = append(foreignKeys, model.ForeignKey{
			Name: f.constraintName,
			From: model.ForeignKeyEndpoint{
				Schema:  f.fromSchema,
				Table:   f.fromTable,
				Columns: f.fromCols,
			},
			To: model.ForeignKeyEndpoint{
				Schema:  f.toSchema,
				Table:   f.toTable,
				Columns: f.toCols,
			},
			Origin:         "declared",
			OnDelete:       f.onDelete,
			OnUpdate:       f.onUpdate,
			ConstraintName: f.constraintName,
		})
	}

	// Collect final tables slice
	for _, tbl := range tableMap {
		tables = append(tables, *tbl)
	}

	// Deterministic sorting
	sort.Slice(tables, func(i, j int) bool {
		if tables[i].Schema != tables[j].Schema {
			return tables[i].Schema < tables[j].Schema
		}
		return tables[i].Name < tables[j].Name
	})

	sort.Slice(foreignKeys, func(i, j int) bool {
		if foreignKeys[i].From.Schema != foreignKeys[j].From.Schema {
			return foreignKeys[i].From.Schema < foreignKeys[j].From.Schema
		}
		if foreignKeys[i].From.Table != foreignKeys[j].From.Table {
			return foreignKeys[i].From.Table < foreignKeys[j].From.Table
		}
		return foreignKeys[i].Name < foreignKeys[j].Name
	})

	alias := scope.Alias
	if alias == "" && len(scope.Schemas) > 0 {
		alias = scope.Schemas[0]
	}

	snapshot := &model.Snapshot{
		SchemaVersion: 1,
		ToolVersion:   "0.1.0",
		CapturedAt:    time.Now().UTC().Truncate(time.Second),
		Source: model.SourceMetadata{
			Engine:        "mysql",
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
	}

	return snapshot, nil
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
