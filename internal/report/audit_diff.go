package report

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"text/tabwriter"

	"github.com/farulwananda/dataprobe/internal/diff"
	"github.com/farulwananda/dataprobe/internal/model"
)

// PrintAuditReport outputs audit findings in human table or JSON format.
func PrintAuditReport(findings []model.Finding, format string, out io.Writer) error {
	if format == "json" {
		enc := json.NewEncoder(out)
		enc.SetIndent("", "  ")
		return enc.Encode(findings)
	}

	if format == "markdown" {
		fmt.Fprintf(out, "# Database Structural Audit Report\n\n")
		fmt.Fprintf(out, "**Total Findings:** %d\n\n", len(findings))
		if len(findings) == 0 {
			fmt.Fprintf(out, "No structural audit findings detected. All checks passed!\n")
			return nil
		}
		fmt.Fprintf(out, "| Severity | Rule ID | Target | Title | Recommendation |\n")
		fmt.Fprintf(out, "|---|---|---|---|---|\n")
		for _, f := range findings {
			target := f.Scope.Schema + "." + f.Scope.Table
			if f.Scope.Index != "" {
				target += " (" + f.Scope.Index + ")"
			}
			fmt.Fprintf(out, "| `%s` | `%s` | `%s` | %s | %s |\n",
				strings.ToUpper(string(f.Severity)),
				f.RuleID,
				target,
				f.Title,
				f.Recommendation,
			)
		}
		return nil
	}
	w := tabwriter.NewWriter(out, 0, 0, 3, ' ', 0)
	fmt.Fprintf(w, "DATABASE STRUCTURAL AUDIT REPORT\n")
	fmt.Fprintf(w, "Total Findings:\t%d\n\n", len(findings))

	if len(findings) == 0 {
		fmt.Fprintf(out, "No structural audit findings detected. All checks passed!\n")
		return nil
	}

	fmt.Fprintf(w, "SEVERITY\tRULE ID\tTARGET\tTITLE\tRECOMMENDATION\n")
	for _, f := range findings {
		target := f.Scope.Schema + "." + f.Scope.Table
		if f.Scope.Index != "" {
			target += " (" + f.Scope.Index + ")"
		}
		fmt.Fprintf(w, "[%s]\t%s\t%s\t%s\t%s\n",
			strings.ToUpper(string(f.Severity)),
			f.RuleID,
			target,
			f.Title,
			f.Recommendation,
		)
	}
	_ = w.Flush()

	fmt.Fprintf(out, "\nNote: Findings are heuristic advice based on metadata; verify against production workload before altering schema or indexes.\n")
	return nil
}

func PrintDiffReport(d *diff.SnapshotDiff, format string, out io.Writer) error {
	if format == "json" {
		enc := json.NewEncoder(out)
		enc.SetIndent("", "  ")
		return enc.Encode(d)
	}

	if format == "markdown" {
		fmt.Fprintf(out, "# Snapshot Diff & Growth Report\n\n")
		fmt.Fprintf(out, "- **Period:** `%s` &rarr; `%s` (%.1f days)\n",
			d.OldCapturedAt.Format("2006-01-02 15:04:05"),
			d.NewCapturedAt.Format("2006-01-02 15:04:05"),
			d.ElapsedDays)
		fmt.Fprintf(out, "- **Storage Allocation:** `%s` &rarr; `%s` (Delta: `%s`, %.1f%%)\n",
			formatBytes(d.OldTotalBytes), formatBytes(d.NewTotalBytes),
			formatBytes(d.TotalByteDelta), d.TotalGrowthPct)
		fmt.Fprintf(out, "- **Tables Summary:** %d added, %d removed, %d modified\n\n",
			d.TablesAdded, d.TablesRemoved, d.TablesModified)

		if len(d.TopGrowthTables) > 0 {
			fmt.Fprintf(out, "### Top Growth Contributors\n\n")
			fmt.Fprintf(out, "| Table | Old Size | New Size | Delta | Growth %% | Row Delta |\n")
			fmt.Fprintf(out, "|---|---|---|---|---|---|\n")
			for _, t := range d.TopGrowthTables {
				oldSize := t.OldDataBytes + t.OldIndexBytes
				newSize := t.NewDataBytes + t.NewIndexBytes
				fmt.Fprintf(out, "| `%s.%s` | %s | %s | +%s | +%.1f%% | +%d |\n",
					t.Schema, t.Name, formatBytes(oldSize), formatBytes(newSize),
					formatBytes(t.TotalByteDelta), t.GrowthPct, t.RowDelta)
			}
		}
		return nil
	}
	w := tabwriter.NewWriter(out, 0, 0, 3, ' ', 0)
	fmt.Fprintf(w, "DATAPROBE SNAPSHOT DIFF & GROWTH REPORT\n")
	fmt.Fprintf(w, "Period:\t%s -> %s (%.1f days)\n",
		d.OldCapturedAt.Format("2006-01-02 15:04:05"),
		d.NewCapturedAt.Format("2006-01-02 15:04:05"),
		d.ElapsedDays,
	)
	fmt.Fprintf(w, "Storage Allocation:\t%s -> %s (Delta: %s, %.1f%%)\n",
		formatBytes(d.OldTotalBytes),
		formatBytes(d.NewTotalBytes),
		formatBytes(d.TotalByteDelta),
		d.TotalGrowthPct,
	)
	fmt.Fprintf(w, "Tables Summary:\t%d added, %d removed, %d modified\n\n",
		d.TablesAdded, d.TablesRemoved, d.TablesModified)
	_ = w.Flush()

	if len(d.TopGrowthTables) > 0 {
		fmt.Fprintf(out, "TOP GROWTH CONTRIBUTORS\n")
		tw := tabwriter.NewWriter(out, 0, 0, 3, ' ', 0)
		fmt.Fprintf(tw, "TABLE\tOLD SIZE\tNEW SIZE\tDELTA\tGROWTH %%\tROW DELTA\n")
		for _, t := range d.TopGrowthTables {
			oldSize := t.OldDataBytes + t.OldIndexBytes
			newSize := t.NewDataBytes + t.NewIndexBytes
			fmt.Fprintf(tw, "%s.%s\t%s\t%s\t+%s\t+%.1f%%\t+%d\n",
				t.Schema,
				t.Name,
				formatBytes(oldSize),
				formatBytes(newSize),
				formatBytes(t.TotalByteDelta),
				t.GrowthPct,
				t.RowDelta,
			)
		}
		_ = tw.Flush()
	}

	return nil
}
