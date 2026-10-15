# dataprobe

> **Understand your database. Detect anomalies. Audit growth.**

[![Go](https://img.shields.io/badge/Go-1.25+-00ADD8?style=flat&logo=go)](https://go.dev/)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)
[![CI](https://img.shields.io/badge/CI-Passing-brightgreen?style=flat&logo=github-actions)](https://github.com/farulwananda/dataprobe/actions)

`dataprobe` is an open-source, local-first database intelligence and integrity auditor written in Go. It introspects schema metadata, maps entity relationships to an interactive local graph, and captures versioned, secret-free JSON snapshots.

Built for backend developers, data engineers, and database maintainers who need immediate visibility into legacy databases, migration drift, and storage growth without deploying heavy daemons, background agents, or running dangerous full-table scans on production.

---

## Key Principles

- **Production-Safe by Default:** Queries `INFORMATION_SCHEMA` and `pg_catalog` metadata only. **Zero full-table scans** on production tables.
- **Strictly Read-Only:** Does not execute DDL/DML, `ALTER TABLE`, `DROP`, `DELETE`, or "autofixes".
- **Multi-Engine Ready:** Introspects MySQL 5.7+/8.x and PostgreSQL 12+ schemas, columns, indexes, and composite foreign keys.
- **Local-First & Offline:** Single portable Go binary. Runs local interactive graph viewer with vendored assets (no internet or CDN required).
- **Secret-Free Snapshots:** Database credentials, passwords, and raw row payloads are **never persisted** to configuration files, logs, or snapshots.
- **Honest Diagnostics:** Explicitly labels statistical row counts and storage allocation numbers as `estimated` (InnoDB `TABLE_ROWS`, PostgreSQL `pg_class.reltuples`).

---

## Installation

### Using Go

```bash
go install github.com/farulwananda/dataprobe/cmd/dataprobe@latest
```

### From Source

```bash
git clone https://github.com/farulwananda/dataprobe.git
cd dataprobe
go build -o dataprobe ./cmd/dataprobe
```

---

## Quick Start (Phase 0 & 1 MVP)

### 1. Configure Connection

Create `dataprobe.yaml`:

```yaml
version: 1

source:
  engine: mysql
  # The environment variable storing your read-only database DSN
  dsn_env: DATAPROBE_DSN
  alias: production-store
  include_schemas:
    - shop_db
  exclude_tables:
    - "*_cache"

audit:
  profile: metadata
  timeout: 10s
  total_budget: 60s
  max_concurrency: 2
```

Export your read-only MySQL DSN in your terminal or secret manager:

```bash
export DATAPROBE_DSN="readonly_user:password@tcp(127.0.0.1:3306)/shop_db?tls=false&parseTime=true"
```

### 2. Inspect Database Schema

Generate a human-readable tabular summary of tables, columns, indexes, foreign keys, and storage estimates:

```bash
dataprobe inspect --config dataprobe.yaml
```

Output formatted as pure JSON for automated pipelines:

```bash
dataprobe inspect --config dataprobe.yaml --format json
```

### 3. Capture an Offline Snapshot

Export a clean, versioned metadata snapshot without storing database credentials or table rows:

```bash
dataprobe snapshot --config dataprobe.yaml --out snapshots/baseline.json
```

### 4. Open Interactive Relationship Graph

View database relationships, primary keys, and index structures offline in your browser:

```bash
dataprobe graph --input snapshots/baseline.json --open
```

Or run the viewer with sample mock data included in this repository:

```bash
dataprobe graph --input examples/synthetic-schema.json --open
```

*The viewer binds exclusively to loopback (`127.0.0.1`), enforces per-session tokens, and bundles Cytoscape.js locally.*

---

## CLI Reference

| Command | Flags | Description |
|---|---|---|
| `dataprobe inspect` | `--config <file>`, `--format table\|json` | Introspect and display schema and storage summary |
| `dataprobe snapshot` | `--config <file>`, `--out <file>` | Capture schema metadata to versioned JSON snapshot |
| `dataprobe graph` | `--input <file>`, `--port <num>`, `--open` | Launch local interactive relationship graph viewer with findings overlay |
| `dataprobe audit` | `--input <file>`, `--checks <list>`, `--fail-on <level>` | Run structural rules (missing PK, redundant indexes, missing FK indexes) |
| `dataprobe diff` | `<old.json> <new.json>`, `--format table\|json` | Compare two snapshots to analyze drift and growth anomalies |
| `dataprobe version` | | Show tool version |
### Exit Codes

| Code | Meaning |
|---|---|
| `0` | Success |
| `1` | Audit finding threshold reached (Phase 2+) |
| `2` | CLI usage, configuration, or input validation error |
| `3` | Database connection, permission, or runtime error |

---

## Roadmap

- **v0.1.0 (Current):** Phase 0 & 1 — MySQL schema introspection, snapshot contract v1, and embedded Cytoscape relationship graph.
- **v0.2.0:** Structural audit rules (missing primary keys, redundant/overlapping indexes, missing FK coverage).
- **v0.3.0:** Opt-in bounded integrity checks (orphan detection, cardinality outliers, query budgets).
- **v0.4.0:** Snapshot diffing, schema drift, and growth/data explosion analysis.
- **v0.5.0:** PostgreSQL engine adapter.
- **v1.0.0:** Production-grade stability, performance benchmarks, and multi-platform binary releases.

---

## License

This project is licensed under the [MIT License](LICENSE).
The vendored Cytoscape.js library is licensed under the [MIT License](internal/graph/web/vendor/LICENSE).
