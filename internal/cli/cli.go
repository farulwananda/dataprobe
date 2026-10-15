package cli

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"

	"github.com/farulwananda/dataprobe/internal/config"
	"github.com/farulwananda/dataprobe/internal/db"
	"github.com/farulwananda/dataprobe/internal/db/mysql"
	"github.com/farulwananda/dataprobe/internal/graph"
	"github.com/farulwananda/dataprobe/internal/report"
	"github.com/farulwananda/dataprobe/internal/snapshot"
)

// Exit codes following Section 6 of DATAPROBE_PLAN.md
const (
	ExitSuccess = 0 // Command succeeded
	ExitFinding = 1 // Audit finished with issues >= threshold
	ExitUsage   = 2 // CLI / config / schema validation error
	ExitRuntime = 3 // Connection / permission / IO / runtime error
)

const rootHelp = `dataprobe - Database Intelligence & Integrity Auditor

Usage:
  dataprobe <command> [flags]

Commands:
  inspect      Inspect database schema metadata, row counts, and storage estimates
  snapshot     Capture database metadata into an offline versioned JSON snapshot
  graph        Launch interactive local relationship graph viewer
  version      Show dataprobe version information

Flags:
  -h, --help   Show help for dataprobe or a subcommand
`

// Run dispatches CLI execution and returns the appropriate exit code.
func Run(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprint(stdout, rootHelp)
		return ExitSuccess
	}

	cmd := args[0]
	cmdArgs := args[1:]

	switch cmd {
	case "help", "-h", "--help":
		fmt.Fprint(stdout, rootHelp)
		return ExitSuccess

	case "version", "-v", "--version":
		fmt.Fprintln(stdout, "dataprobe v0.1.0 (Phase 0 & 1 MVP)")
		return ExitSuccess

	case "inspect":
		return runInspect(cmdArgs, stdout, stderr)

	case "snapshot":
		return runSnapshot(cmdArgs, stdout, stderr)

	case "graph":
		return runGraph(cmdArgs, stdout, stderr)

	default:
		fmt.Fprintf(stderr, "dataprobe: unknown command %q\nRun 'dataprobe --help' for usage.\n", cmd)
		return ExitUsage
	}
}

func runInspect(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("inspect", flag.ContinueOnError)
	fs.SetOutput(stderr)

	cfgPath := fs.String("config", "", "Path to dataprobe.yaml configuration file")
	format := fs.String("format", "table", "Output format: 'table' or 'json'")

	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return ExitSuccess
		}
		return ExitUsage
	}

	if *cfgPath == "" {
		fmt.Fprintln(stderr, "dataprobe inspect: --config flag is required")
		return ExitUsage
	}

	cfg, err := config.Load(*cfgPath)
	if err != nil {
		fmt.Fprintf(stderr, "dataprobe inspect: %v\n", err)
		return ExitUsage
	}

	dsn, err := cfg.ResolveDSN()
	if err != nil {
		fmt.Fprintf(stderr, "dataprobe inspect: %v\n", err)
		return ExitRuntime
	}

	ctx, cancel := context.WithTimeout(context.Background(), cfg.Audit.Timeout)
	defer cancel()

	inspector, err := mysql.NewInspector(dsn)
	if err != nil {
		fmt.Fprintf(stderr, "dataprobe inspect: failed connecting to database: %v\n", err)
		return ExitRuntime
	}
	defer inspector.Close()

	snap, err := inspector.Inspect(ctx, db.Scope{
		Schemas:       cfg.Source.IncludeSchemas,
		ExcludeTables: cfg.Source.ExcludeTables,
		Alias:         cfg.Source.Alias,
	})
	if err != nil {
		fmt.Fprintf(stderr, "dataprobe inspect: introspection failed: %v\n", err)
		return ExitRuntime
	}

	if err := report.PrintInspectSummary(snap, *format, stdout); err != nil {
		fmt.Fprintf(stderr, "dataprobe inspect: failed rendering output: %v\n", err)
		return ExitRuntime
	}

	return ExitSuccess
}

func runSnapshot(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("snapshot", flag.ContinueOnError)
	fs.SetOutput(stderr)

	cfgPath := fs.String("config", "", "Path to dataprobe.yaml configuration file")
	outPath := fs.String("out", "", "Destination file for snapshot JSON (required)")

	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return ExitSuccess
		}
		return ExitUsage
	}

	if *cfgPath == "" {
		fmt.Fprintln(stderr, "dataprobe snapshot: --config flag is required")
		return ExitUsage
	}
	if *outPath == "" {
		fmt.Fprintln(stderr, "dataprobe snapshot: --out flag is required")
		return ExitUsage
	}

	cfg, err := config.Load(*cfgPath)
	if err != nil {
		fmt.Fprintf(stderr, "dataprobe snapshot: %v\n", err)
		return ExitUsage
	}

	dsn, err := cfg.ResolveDSN()
	if err != nil {
		fmt.Fprintf(stderr, "dataprobe snapshot: %v\n", err)
		return ExitRuntime
	}

	ctx, cancel := context.WithTimeout(context.Background(), cfg.Audit.Timeout)
	defer cancel()

	inspector, err := mysql.NewInspector(dsn)
	if err != nil {
		fmt.Fprintf(stderr, "dataprobe snapshot: failed connecting to database: %v\n", err)
		return ExitRuntime
	}
	defer inspector.Close()

	snap, err := inspector.Inspect(ctx, db.Scope{
		Schemas:       cfg.Source.IncludeSchemas,
		ExcludeTables: cfg.Source.ExcludeTables,
		Alias:         cfg.Source.Alias,
	})
	if err != nil {
		fmt.Fprintf(stderr, "dataprobe snapshot: introspection failed: %v\n", err)
		return ExitRuntime
	}

	if err := snapshot.Save(snap, *outPath); err != nil {
		fmt.Fprintf(stderr, "dataprobe snapshot: failed saving snapshot: %v\n", err)
		return ExitRuntime
	}

	fmt.Fprintf(stdout, "dataprobe: snapshot saved successfully to %q (%d tables, %d foreign keys)\n",
		*outPath, len(snap.Tables), len(snap.ForeignKeys))
	return ExitSuccess
}

func runGraph(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("graph", flag.ContinueOnError)
	fs.SetOutput(stderr)

	inPath := fs.String("input", "", "Path to snapshot JSON file (required)")
	port := fs.Int("port", 0, "Local port to listen on (0 for automatic free port)")
	openBrowser := fs.Bool("open", false, "Automatically open graph in default browser")

	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return ExitSuccess
		}
		return ExitUsage
	}

	if *inPath == "" {
		fmt.Fprintln(stderr, "dataprobe graph: --input flag is required")
		return ExitUsage
	}

	snap, err := snapshot.Load(*inPath)
	if err != nil {
		fmt.Fprintf(stderr, "dataprobe graph: %v\n", err)
		return ExitUsage
	}

	srv, err := graph.NewServer(graph.Config{
		Snapshot: snap,
		Port:     *port,
	})
	if err != nil {
		fmt.Fprintf(stderr, "dataprobe graph: failed creating viewer server: %v\n", err)
		return ExitRuntime
	}
	defer srv.Close()

	viewerURL := srv.URL()
	fmt.Fprintf(stdout, "dataprobe: Relationship Graph viewer active at:\n  %s\nPress Ctrl+C to stop.\n", viewerURL)

	if *openBrowser {
		go graph.OpenBrowser(viewerURL)
	}

	sigCtx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	errChan := make(chan error, 1)
	go func() {
		if err := srv.Serve(); err != nil && !errors.Is(err, httpErrClosed) {
			errChan <- err
		}
	}()

	select {
	case <-sigCtx.Done():
		fmt.Fprintln(stdout, "\ndataprobe: graph viewer stopped.")
		return ExitSuccess
	case err := <-errChan:
		fmt.Fprintf(stderr, "dataprobe graph: server error: %v\n", err)
		return ExitRuntime
	}
}

var httpErrClosed = errors.New("http: Server closed")
