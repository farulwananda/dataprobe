package report

import (
	"encoding/json"
	"fmt"
	"io"
	"text/tabwriter"

	"github.com/farulwananda/dataprobe/internal/model"
)

// PrintInspectSummary outputs a clean human-readable table or structured JSON.
func PrintInspectSummary(s *model.Snapshot, format string, out io.Writer) error {
	if format == "json" {
		enc := json.NewEncoder(out)
		enc.SetIndent("", "  ")
		return enc.Encode(s)
	}

	if format == "markdown" {
		fmt.Fprintf(out, "# Database Inspect Summary\n\n")
		fmt.Fprintf(out, "- **Engine:** `%s %s`\n", s.Source.Engine, s.Source.ServerVersion)
		fmt.Fprintf(out, "- **Schemas:** %v\n", s.Source.Schemas)
		fmt.Fprintf(out, "- **Total Tables:** %d\n", len(s.Tables))
		fmt.Fprintf(out, "- **Total Foreign Keys:** %d\n\n", len(s.ForeignKeys))

		fmt.Fprintf(out, "### Tables\n\n")
		fmt.Fprintf(out, "| Table | Kind | Est. Rows | Data Size | Index Size | Columns | Indexes |\n")
		fmt.Fprintf(out, "|---|---|---|---|---|---|---|\n")
		for _, t := range s.Tables {
			fmt.Fprintf(out, "| `%s.%s` | %s | %d | %s | %s | %d | %d |\n",
				t.Schema, t.Name, t.Kind, t.RowCount.Value,
				formatBytes(t.StorageBytes.Data), formatBytes(t.StorageBytes.Indexes),
				len(t.Columns), len(t.Indexes))
		}

		if len(s.ForeignKeys) > 0 {
			fmt.Fprintf(out, "\n### Foreign Key Relationships\n\n")
			fmt.Fprintf(out, "| Name | From (Child) | To (Parent) | On Delete |\n")
			fmt.Fprintf(out, "|---|---|---|---|\n")
			for _, fk := range s.ForeignKeys {
				fromStr := fmt.Sprintf("`%s.%s` (%v)", fk.From.Schema, fk.From.Table, fk.From.Columns)
				toStr := fmt.Sprintf("`%s.%s` (%v)", fk.To.Schema, fk.To.Table, fk.To.Columns)
				fmt.Fprintf(out, "| `%s` | %s | %s | %s |\n", fk.Name, fromStr, toStr, fk.OnDelete)
			}
		}
		return nil
	}
	w := tabwriter.NewWriter(out, 0, 0, 3, ' ', 0)
	fmt.Fprintf(w, "DATABASE INSPECT SUMMARY\n")
	fmt.Fprintf(w, "Engine:\t%s %s\n", s.Source.Engine, s.Source.ServerVersion)
	fmt.Fprintf(w, "Schemas:\t%d (%v)\n", len(s.Source.Schemas), s.Source.Schemas)
	fmt.Fprintf(w, "Total Tables:\t%d\n", len(s.Tables))
	fmt.Fprintf(w, "Total Foreign Keys:\t%d\n\n", len(s.ForeignKeys))

	fmt.Fprintf(w, "TABLE\tKIND\tEST. ROWS\tDATA SIZE\tINDEX SIZE\tCOLUMNS\tINDEXES\n")
	for _, t := range s.Tables {
		fmt.Fprintf(w, "%s.%s\t%s\t%d\t%s\t%s\t%d\t%d\n",
			t.Schema,
			t.Name,
			t.Kind,
			t.RowCount.Value,
			formatBytes(t.StorageBytes.Data),
			formatBytes(t.StorageBytes.Indexes),
			len(t.Columns),
			len(t.Indexes),
		)
	}
	_ = w.Flush()

	if len(s.ForeignKeys) > 0 {
		fmt.Fprintf(out, "\nFOREIGN KEY RELATIONSHIPS (%d)\n", len(s.ForeignKeys))
		fw := tabwriter.NewWriter(out, 0, 0, 3, ' ', 0)
		fmt.Fprintf(fw, "NAME\tFROM (CHILD)\tTO (PARENT)\tON DELETE\n")
		for _, fk := range s.ForeignKeys {
			fromStr := fmt.Sprintf("%s.%s (%v)", fk.From.Schema, fk.From.Table, fk.From.Columns)
			toStr := fmt.Sprintf("%s.%s (%v)", fk.To.Schema, fk.To.Table, fk.To.Columns)
			fmt.Fprintf(fw, "%s\t%s\t%s\t%s\n", fk.Name, fromStr, toStr, fk.OnDelete)
		}
		_ = fw.Flush()
	}

	fmt.Fprintf(out, "\nNote: Row counts and storage byte sizes are storage-engine estimates (TABLE_ROWS).\n")
	return nil
}

func formatBytes(b int64) string {
	if b <= 0 {
		return "0 B"
	}
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %ciB", float64(b)/float64(div), "KMGTPE"[exp])
}
