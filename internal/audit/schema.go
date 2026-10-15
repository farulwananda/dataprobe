package audit

import (
	"strings"

	"github.com/farulwananda/dataprobe/internal/model"
)

// Rule represents a structural inspection rule.
type Rule interface {
	ID() string
	Name() string
	Category() string
	Run(snap *model.Snapshot) []model.Finding
}

// Registry holds the active audit rules.
type Registry struct {
	rules []Rule
}

// NewRegistry initializes the default structural rules.
func NewRegistry() *Registry {
	return &Registry{
		rules: []Rule{
			&RuleMissingPrimaryKey{},
			&RuleUnresolvedForeignKey{},
			&RuleDuplicateIndex{},
			&RuleOverlappingIndex{},
			&RuleForeignKeyMissingIndex{},
		},
	}
}

// Run executes the rules against a snapshot.
func (r *Registry) Run(snap *model.Snapshot, ruleFilter []string) []model.Finding {
	filterMap := make(map[string]bool)
	for _, f := range ruleFilter {
		f = strings.ToLower(strings.TrimSpace(f))
		if f != "" {
			filterMap[f] = true
		}
	}

	var findings []model.Finding
	for _, rule := range r.rules {
		if len(filterMap) > 0 {
			matchCategory := filterMap[strings.ToLower(rule.Category())]
			matchID := filterMap[strings.ToLower(rule.ID())]
			if !matchCategory && !matchID {
				continue
			}
		}
		findings = append(findings, rule.Run(snap)...)
	}

	return findings
}

// RuleMissingPrimaryKey checks for base tables lacking a primary key (SCH_001).
type RuleMissingPrimaryKey struct{}

func (r *RuleMissingPrimaryKey) ID() string       { return "SCH_001" }
func (r *RuleMissingPrimaryKey) Name() string     { return "Table without declared primary key" }
func (r *RuleMissingPrimaryKey) Category() string { return "schema" }

func (r *RuleMissingPrimaryKey) Run(snap *model.Snapshot) []model.Finding {
	var findings []model.Finding
	for _, tbl := range snap.Tables {
		if tbl.Kind != "base_table" {
			continue
		}
		if len(tbl.PrimaryKey) == 0 {
			findings = append(findings, model.Finding{
				RuleID:   r.ID(),
				Severity: model.SeverityHigh,
				Status:   model.StatusFinding,
				Scope: model.FindingScope{
					Schema: tbl.Schema,
					Table:  tbl.Name,
				},
				Title:   "Table lacks declared primary key",
				Summary: "Table has no primary key defined. Without a primary key, rows lack unique identity which can impact replication and lookup efficiency.",
				Evidence: map[string]any{
					"table": tbl.Name,
				},
				Measurement: model.MetricSource{
					Quality: model.AccuracyExact,
					Source:  "information_schema.table_constraints",
				},
				Impact:         "Can lead to table scans during point lookups and replication issues in clustered setups.",
				Recommendation: "Evaluate adding a synthetic auto-increment or UUID primary key unless this is an intentional append-only staging table.",
				AutoFix:        false,
			})
		}
	}
	return findings
}

// RuleUnresolvedForeignKey checks if a foreign key references a table outside the snapshot scope (SCH_002).
type RuleUnresolvedForeignKey struct{}

func (r *RuleUnresolvedForeignKey) ID() string       { return "SCH_002" }
func (r *RuleUnresolvedForeignKey) Name() string     { return "Unresolved foreign key reference" }
func (r *RuleUnresolvedForeignKey) Category() string { return "schema" }

func (r *RuleUnresolvedForeignKey) Run(snap *model.Snapshot) []model.Finding {
	tableSet := make(map[string]bool)
	for _, t := range snap.Tables {
		tableSet[t.Schema+"."+t.Name] = true
	}

	var findings []model.Finding
	for _, fk := range snap.ForeignKeys {
		targetKey := fk.To.Schema + "." + fk.To.Table
		if !tableSet[targetKey] {
			findings = append(findings, model.Finding{
				RuleID:   r.ID(),
				Severity: model.SeverityMedium,
				Status:   model.StatusFinding,
				Scope: model.FindingScope{
					Schema: fk.From.Schema,
					Table:  fk.From.Table,
				},
				Title:   "Foreign key references unresolved table",
				Summary: "Foreign key references a target table not found in the introspected scope.",
				Evidence: map[string]any{
					"foreign_key":    fk.Name,
					"from":           fk.From.Schema + "." + fk.From.Table,
					"target_missing": targetKey,
				},
				Measurement: model.MetricSource{
					Quality: model.AccuracyExact,
					Source:  "information_schema.key_column_usage",
				},
				Impact:         "Referential integrity cannot be validated in this snapshot scope; target may reside in a excluded schema.",
				Recommendation: "Check if the target schema should be included in config include_schemas.",
				AutoFix:        false,
			})
		}
	}
	return findings
}
