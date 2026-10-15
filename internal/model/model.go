package model

import "time"

// Accuracy represents the confidence/quality of a metric.
type Accuracy string

const (
	AccuracyExact     Accuracy = "exact"
	AccuracyEstimated Accuracy = "estimated"
	AccuracySampled   Accuracy = "sampled"
	AccuracyUnknown   Accuracy = "unknown"
)

// MetricSource describes where a metric came from.
type MetricSource struct {
	Quality Accuracy `json:"quality"`
	Source  string   `json:"source,omitempty"`
	Notes   string   `json:"notes,omitempty"`
}

// RowCount holds row count value and its accuracy quality.
type RowCount struct {
	Value   int64    `json:"value"`
	Quality Accuracy `json:"quality"`
}

// StorageBytes holds storage allocation measurements.
type StorageBytes struct {
	Data    int64    `json:"data"`
	Indexes int64    `json:"indexes"`
	Quality Accuracy `json:"quality"`
}

// Column represents a single table column.
type Column struct {
	Name          string  `json:"name"`
	DBType        string  `json:"db_type"`
	Nullable      bool    `json:"nullable"`
	DefaultValue  *string `json:"default_value,omitempty"`
	IsPrimaryKey  bool    `json:"is_primary_key"`
	AutoIncrement bool    `json:"auto_increment,omitempty"`
	Comment       string  `json:"comment,omitempty"`
}

// IndexColumn represents a column within an index.
type IndexColumn struct {
	Name      string `json:"name"`
	Seq       int    `json:"seq"`
	Collation string `json:"collation,omitempty"`
}

// Index represents a table index definition.
type Index struct {
	Name      string        `json:"name"`
	Columns   []IndexColumn `json:"columns"`
	Unique    bool          `json:"unique"`
	Primary   bool          `json:"primary"`
	IndexType string        `json:"index_type,omitempty"`
}

// Table represents a database table/view and its metadata.
type Table struct {
	Schema       string       `json:"schema"`
	Name         string       `json:"name"`
	Kind         string       `json:"kind"` // "base_table", "view"
	Engine       string       `json:"engine,omitempty"`
	RowCount     RowCount     `json:"row_count"`
	StorageBytes StorageBytes `json:"storage_bytes"`
	Columns      []Column     `json:"columns"`
	PrimaryKey   []string     `json:"primary_key"`
	Indexes      []Index      `json:"indexes"`
	Comment      string       `json:"comment,omitempty"`
}

// ForeignKeyEndpoint represents the source or target of a foreign key.
type ForeignKeyEndpoint struct {
	Schema  string   `json:"schema"`
	Table   string   `json:"table"`
	Columns []string `json:"columns"`
}

// ForeignKey represents a relationship between tables.
type ForeignKey struct {
	Name           string             `json:"name"`
	From           ForeignKeyEndpoint `json:"from"`
	To             ForeignKeyEndpoint `json:"to"`
	Origin         string             `json:"origin"` // "declared", "inferred"
	OnDelete       string             `json:"on_delete,omitempty"`
	OnUpdate       string             `json:"on_update,omitempty"`
	ConstraintName string             `json:"constraint_name,omitempty"`
}

// SourceMetadata describes the inspected database.
type SourceMetadata struct {
	Engine        string   `json:"engine"`
	ServerVersion string   `json:"server_version,omitempty"`
	Alias         string   `json:"alias,omitempty"`
	Schemas       []string `json:"schemas"`
}

// Capabilities tracks supported features for the source.
type Capabilities struct {
	TableStatistics    bool `json:"table_statistics"`
	WorkloadStatistics bool `json:"workload_statistics"`
	FullIntegrityScan  bool `json:"full_integrity_scan"`
}

// FindingSeverity represents the severity level of an audit finding.
type FindingSeverity string

const (
	SeverityCritical FindingSeverity = "critical"
	SeverityHigh     FindingSeverity = "high"
	SeverityMedium   FindingSeverity = "medium"
	SeverityLow      FindingSeverity = "low"
	SeverityInfo     FindingSeverity = "info"
)

// FindingStatus represents whether a check produced an issue or passed.
type FindingStatus string

const (
	StatusFinding FindingStatus = "finding"
	StatusPassed  FindingStatus = "passed"
	StatusSkipped FindingStatus = "skipped"
	StatusUnknown FindingStatus = "unknown"
)

// FindingScope identifies the object affected.
type FindingScope struct {
	Schema string `json:"schema,omitempty"`
	Table  string `json:"table,omitempty"`
	Index  string `json:"index,omitempty"`
	Column string `json:"column,omitempty"`
}

// Finding represents an individual rule inspection outcome.
type Finding struct {
	RuleID         string          `json:"rule_id"`
	Severity       FindingSeverity `json:"severity"`
	Status         FindingStatus   `json:"status"`
	Scope          FindingScope    `json:"scope"`
	Title          string          `json:"title"`
	Summary        string          `json:"summary"`
	Evidence       map[string]any  `json:"evidence,omitempty"`
	Measurement    MetricSource    `json:"measurement"`
	Impact         string          `json:"impact,omitempty"`
	Recommendation string          `json:"recommendation,omitempty"`
	AutoFix        bool            `json:"auto_fix"`
}

// Snapshot is the canonical JSON-serializable database metadata container.
type Snapshot struct {
	SchemaVersion int            `json:"schema_version"`
	ToolVersion   string         `json:"tool_version"`
	CapturedAt    time.Time      `json:"captured_at"`
	Source        SourceMetadata `json:"source"`
	Capabilities  Capabilities   `json:"capabilities"`
	Tables        []Table        `json:"tables"`
	ForeignKeys   []ForeignKey   `json:"foreign_keys"`
	Findings      []Finding      `json:"findings,omitempty"`
	Warnings      []string       `json:"warnings,omitempty"`
}
