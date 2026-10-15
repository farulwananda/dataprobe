# Dataprobe — Product Requirements & Implementation Plan

> **Project:** `dataprobe`  
> **Tagline:** Understand your database. Detect anomalies. Audit growth.  
> **Category:** Open-source Database Intelligence & Integrity Auditor  
> **Language:** Go  
> **Interface:** CLI + local interactive database graph  
> **Target:** staged releases (`v0.1.x` → `v1.0.0`)  
> **Status:** Planning  
> **License:** MIT (proposed; confirm name/license conflicts before publishing)  
> **Principle:** **Production-safe by default, evidence-based findings, YAGNI.**

---

## 1. Executive Summary

**Dataprobe** adalah developer tool berbasis Go untuk memahami kondisi database tanpa harus membuat aplikasi monitoring permanen. Ia membaca metadata database (tabel, kolom, PK, FK, indeks), memetakan relasi ke **interactive graph**, menjalankan audit berbasis aturan, membandingkan **snapshot** dari waktu ke waktu, lalu menyajikan temuan yang dapat ditindaklanjuti.

Pengguna utamanya adalah backend engineer, database maintainer, dan engineer yang rutin menerima export/snapshot dari server untuk audit migrasi, integritas relasi, potensi anomali, serta pertumbuhan data yang tidak wajar.

**Karakteristik utama:**

- **Local-first:** satu Go binary; tidak memerlukan cloud account, Docker, Redis, queue, atau database tambahan.
- **Read-only secara desain:** tidak menulis/mengubah database yang diaudit.
- **Metadata-first:** default hanya membaca metadata dan statistik yang tersedia; **tidak melakukan full-table scan**.
- **Honest diagnostics:** bedakan angka perkiraan, sampel, dan hasil pemeriksaan penuh; tidak mengklaim “database optimal 95%” dari metadata saja.
- **Snapshot portable:** hasil audit yang sudah disanitasi dapat disimpan sebagai JSON, dibandingkan, dan dibuka kembali secara offline.
- **Progressive complexity:** dukungan database dan fitur berat baru ditambah setelah workflow dasar terbukti.

### 1.1 Problem yang diselesaikan

1. Memahami relasi puluhan/ratusan tabel pada database legacy sulit tanpa ERD terkini.
2. Foreign key tidak selalu didefinisikan, sehingga relasi nyata sulit ditelusuri.
3. Migrasi atau import besar dapat menghasilkan orphan, duplicate, atau distribusi data yang aneh.
4. Ukuran tabel, log, event, dan indeks bisa berkembang cepat tanpa terlihat penyebabnya.
5. Pemeriksaan optimasi indeks dan workload tersebar di banyak SQL/query dan dashboard.
6. Pengguna sering hanya memiliki snapshot hasil export dari server dan perlu membandingkan dua kondisi.
7. Audit production yang sembarangan justru dapat membebani sistem.

### 1.2 Definisi sukses

- Dalam satu sesi CLI, developer bisa menemukan struktur, PK/FK, indeks, dan perkiraan ukuran tabel.
- Database graph dapat dibuka offline dari file snapshot tanpa koneksi ke server asal.
- Setiap temuan menyebut objek terdampak, bukti, tingkat risiko, keterbatasan, dan rekomendasi **manual**.
- Dua snapshot dapat dibandingkan dan memunculkan tabel penyumbang pertumbuhan terbesar.
- Audit metadata default tidak melakukan query mahal terhadap tabel bisnis dan tidak menyentuh data row-level.
- Semua pemeriksaan berat memerlukan pilihan mode eksplisit, dibatasi oleh budget, serta dapat dilewati jika tidak aman.

---

## 2. Scope & Priorities

### 2.1 In-scope

| Area | Capability | Delivery |
|---|---|---|
| Connectivity | Koneksi MySQL read-only, TLS, timeout | v0.1 |
| Schema Explorer | Tabel, kolom, PK, FK, unique, indexes, row estimate, sizes | v0.1 |
| Relationship Graph | Graph interaktif dari metadata, filter/search, export SVG/PNG bila praktis | v0.1 |
| Snapshot | JSON versioned tanpa credentials/data row-level | v0.1 |
| Structural Audit | Missing PK, suspicious index overlap, FK/index coverage | v0.2 |
| Integrity Audit | Nullable/duplicate/orphan checks *opt-in* dengan batasan beban | v0.3 |
| Growth Audit | Snapshot diff, perubahan ukuran dan row estimate, baseline/anomali | v0.4 |
| PostgreSQL | Schema/stats adapter dan rule set yang relevan dengan engine | v0.5 |
| Offline Dataset | Inspect/validate CSV dan JSONL; relasi melalui mapping deklaratif | v0.6 (opsional) |
| Hardening | Kompatibilitas, performance, keamanan, test dan dokumentasi | v1.0 |

> **MVP nyata = v0.1**: MySQL → metadata snapshot → interactive graph. Jangan menunggu seluruh visi selesai baru merilis.

### 2.2 Explicit non-goals (v1)

- Mengubah schema, membuat indeks otomatis, menjalankan `ALTER TABLE`, `DELETE`, `UPDATE`, `VACUUM`, `ANALYZE TABLE`, atau “autofix”.
- Database migration framework, ORM, SQL editor, query builder, atau BI/analytics dashboard.
- Full-time monitoring daemon, cloud dashboard, scheduler terintegrasi, alerting service, user account/multi-tenant.
- Menyimpan credentials atau seluruh isi tabel database ke snapshot.
- Menentukan relasi berdasarkan tebakan kolom sebagai fakta; inferensi hanya opsional dengan label `inferred`.
- Memastikan “optimal/tidak optimal” dari metadata tanpa workload, execution plans, dan konteks aplikasi.
- Mendukung semua engine/database sekaligus; Oracle/SQL Server/MongoDB/SQLite di luar fase awal.
- Parsing seluruh format SQL dump secara universal (terutama `pg_dump -Fc`).
- AI/LLM sebagai dependency wajib; audit utama deterministik dan explainable.

---

## 3. Users & Use Cases

**Persona A: Backend engineer** — membuka database besar, memahami hubungan entity tanpa menggambar ERD manual.

**Persona B: Data migration engineer** — mengecek integritas PK/FK, missing records, serta membandingkan hasil sebelum/sesudah proses migrasi.

**Persona C: Production maintainer** — mengecek indeks, ukuran tabel, kecenderungan penumpukan log/event, dan risiko kapasitas dari snapshot berkala.

**Persona D: Offline auditor** — menerima dua file snapshot dari server berbeda lalu menganalisisnya di laptop tanpa koneksi database production.

### 3.1 User stories (acceptance-oriented)

- Sebagai pengguna, saya dapat menyimpan profil koneksi **tanpa password** pada file konfigurasi, dengan DSN di environment variable.
- Sebagai pengguna, saya melihat PK/FK yang benar-benar dideklarasikan beserta composite keys dan arah referensinya.
- Sebagai pengguna, saya bisa mencari nama tabel dalam graph tanpa layout menjadi tidak terbaca.
- Sebagai pengguna, saya bisa membagikan snapshot yang sudah direduksi tanpa menyebarkan data pribadi atau secret.
- Sebagai pengguna, saya dapat melihat `old → new`: tabel bertambah/berkurang, selisih ukuran, perubahan index/schema, dan keterbatasan kualitas statistik.
- Sebagai pengguna, saya bisa menjalankan audit mendalam **secara sengaja**, dengan laporan `skipped` jika query berisiko atau tidak memiliki izin.

---

## 4. Technology Decisions

| Component | Choice | Why |
|---|---|---|
| Go | Go 1.27.x patched release (mulai 1.27.2 sebagai baseline; pin minimum yang masuk akal) | Portable single binary; test/race tooling |
| CLI | `flag` + internal dispatcher | Minim dependency di fase awal |
| HTTP local | `net/http` | Cukup untuk UI viewer statis |
| Database API | `database/sql` | Boundary Go idiomatis |
| MySQL | `github.com/go-sql-driver/mysql` | Driver standar untuk `database/sql` |
| PostgreSQL (later) | `github.com/jackc/pgx/v5/stdlib` | Kompatibel dengan `database/sql` |
| Config | `gopkg.in/yaml.v3` (atau JSON awal bila ingin stdlib) | Bisa dibaca/diedit manusia |
| Snapshot & report | `encoding/json` | Portable, schema versioned |
| Graph UI | Cytoscape.js, **vendored/local and embedded** | Graph interaktif tanpa CDN runtime |
| Assets | `//go:embed` | Binary berjalan di luar working directory repo |
| Testing | `testing`, `httptest`, fixture metadata/snapshot | Deterministik |
| CI/release | GitHub Actions dan Go cross-compilation | Multi-platform binary |

**Dependency rule:** hanya tambahkan library untuk driver, YAML, dan graph yang punya use case konkret. Hindari Cobra, ORM, DI framework, event bus, dan microservices sebelum benar-benar dibutuhkan.

---

## 5. Workflow / User Journey

### 5.1 Happy path — database production read-only

1. Buat akun database read-only dan batasi akses ke schema target.
2. Pasang DSN dalam `DATAPROBE_DSN` melalui environment/password manager (bukan `dataprobe.yaml`).
3. Jalankan `dataprobe inspect --config dataprobe.yaml` untuk summary metadata.
4. Jalankan `dataprobe snapshot --config dataprobe.yaml --out snapshots/2026-10-15.json`.
5. Jalankan `dataprobe graph --input snapshots/2026-10-15.json --open`.
6. Jalankan `dataprobe audit --input snapshots/2026-10-15.json --checks schema,indexes`.
7. Pada audit berikutnya, ambil snapshot baru dan jalankan `dataprobe diff old.json new.json`.

### 5.2 Advanced path — opt-in integrity analysis

1. Tinjau daftar pemeriksaan dan query plan/budget (tanpa menjalankan query besar).
2. Tentukan scope tabel/relasi, timeout, max rows, max runtime dan mode sample/full.
3. Jalankan `dataprobe audit --config dataprobe.yaml --checks integrity --mode sampled`.
4. Terima hasil berstatus `verified`, `sampled`, `estimated`, `unknown`, atau `skipped`.
5. Analisis temuan; keputusan perubahan database tetap dilakukan operator secara manual.

### 5.3 Offline export audit

- **Metadata snapshot** JSON: first-class sejak v0.1.
- CSV/JSONL rows: fase v0.6, dengan streaming dan explicit relation config.
- Full SQL dump: **tidak diparse generik**. Untuk dump MySQL/PostgreSQL, pengguna dapat me-restore ke database sementara yang terisolasi lalu audit melalui adapter terkait. Dokumentasikan langkah serta risikonya, jangan auto-restore.

---

## 6. CLI Contract

Executable `dataprobe`, command names stabil secara bertahap.

```bash
# General
 dataprobe --help
 dataprobe version

# v0.1: introspection dan snapshot
 dataprobe inspect --config dataprobe.yaml
 dataprobe inspect --config dataprobe.yaml --format json
 dataprobe snapshot --config dataprobe.yaml --out snapshots/baseline.json
 dataprobe graph --input snapshots/baseline.json --open
 dataprobe graph --input snapshots/baseline.json --port 0

# v0.2: structural rules
 dataprobe audit --input snapshots/baseline.json --checks schema,indexes
 dataprobe audit --input snapshots/baseline.json --format json --out audit.json

# v0.3: integrity sampling, sengaja connect ke DB
 dataprobe audit --config dataprobe.yaml --checks integrity --mode sampled
 dataprobe audit --config dataprobe.yaml --checks integrity --mode full --ack-impact

# v0.4: snapshot comparison & growth audit
 dataprobe diff snapshots/old.json snapshots/new.json
 dataprobe diff snapshots/old.json snapshots/new.json --format json --out diff.json
 dataprobe growth --dir snapshots/ --min-snapshots 3

# v0.6 (optional): audit export CSV/JSONL
 dataprobe data inspect --file users.csv
 dataprobe data validate --file orders.jsonl --rules integrity.yaml
```

**Konvensi:** semua command memakai executable `dataprobe`; flag parsing dan exit behavior harus konsisten.

### Exit codes (CLI commands berbasis audit)

| Code | Meaning |
|---|---|
| `0` | Command sukses; tidak ada finding yang memenuhi failure threshold |
| `1` | Audit selesai, menemukan issues >= `--fail-on` |
| `2` | CLI/config/schema validation error |
| `3` | Connection/permission/IO/runtime error, audit tidak lengkap |

Default `--fail-on high` untuk `audit`, dengan opsi `--fail-on none|low|medium|high|critical`. `inspect`, `graph`, dan `snapshot` sukses → `0`. `diff` sukses → `0` kecuali explicit `--fail-on`.

Output `--format json` harus JSON valid murni di stdout; progress, warnings, dan logs di stderr. Report dapat ditulis atomik (`temporary file` + rename).

---

## 7. Config Contract (contoh)

```yaml
version: 1

source:
  engine: mysql
  dsn_env: DATAPROBE_DSN
  include_schemas:
    - app_db
  exclude_tables:
    - "*_tmp"

audit:
  profile: metadata             # metadata | sampled | full
  timeout: 10s                  # batas per query
  total_budget: 60s             # keseluruhan sesi audit
  max_concurrency: 2            # konservatif pada production
  max_sample_rows: 1000
  max_scan_rows: 100000         # upper bound untuk mode full; stop/skip bila tak aman
  fail_on: high
  rules:
    - schema
    - indexes

growth:
  min_snapshots: 3
  alert_relative_pct: 30
  alert_absolute_mib: 100
  lookback_days: 7

graph:
  hide_system_tables: true
  show_indexes: false
  show_inferred_relations: false
```

**DSN contoh:** `export DATAPROBE_DSN='user:password@tcp(db.example:3306)/app_db?tls=true&parseTime=true'` — bukan disimpan di file config atau README nyata. Hindari DSN di shell history; dokumentasikan cara menggunakan secret manager atau input tersembunyi/variable injection. Jangan tampilkan DSN mentah pada error/log/snapshot.

Validation rules: `version` dikenal, engine didukung, included schemas tidak kosong, pola exclude aman, timeout/row budget positif dan terbatas, dan full mode membutuhkan `--ack-impact`.

---

## 8. Architecture & Modules

```text
              MySQL (v0.1) / PostgreSQL (v0.5)
                            |
                 database/sql + engine adapter
                            |
                 Introspection / Statistics
                            |
                   Canonical Snapshot Model
                      /       |         \
                  Graph     Auditors     Diff/Growth
                    |          |             |
             Local Web UI   Findings     Trend Report
                      \       |            /
                       CLI / JSON / Markdown
```

### 8.1 Practical repo structure

```text
dataprobe/
├── cmd/
│   └── dataprobe/
│       └── main.go
├── internal/
│   ├── cli/                   # command parsing, exit codes, human output
│   ├── config/                # parse config and validation
│   ├── db/
│   │   ├── source.go          # small interface, only actual shared methods
│   │   ├── mysql/             # engine-specific introspection + stats
│   │   └── postgres/          # add only in v0.5
│   ├── model/                 # Snapshot, Table, Column, Index, ForeignKey, Finding
│   ├── snapshot/              # JSON save/load + schema/version validation
│   ├── graph/                 # graph data preparation + local UI
│   │   └── web/               # embed assets must live under this Go package
│   │       ├── index.html
│   │       ├── graph.js
│   │       ├── styles.css
│   │       └── vendor/
│   ├── audit/
│   │   ├── schema.go
│   │   ├── indexes.go
│   │   └── integrity.go       # add only in v0.3
│   ├── diff/                  # add only in v0.4
│   └── report/                # terminal/JSON/Markdown rendering
├── testdata/
│   ├── snapshots/
│   └── fixtures/
├── examples/
│   ├── dataprobe.yaml
│   └── synthetic-schema.json
├── .github/workflows/
│   ├── ci.yml
│   └── release.yml
├── README.md
├── LICENSE
├── go.mod
└── go.sum
```

**Embedding note:** pada Go, path `//go:embed` harus berada di bawah direktori package yang mendeklarasikannya. Karena itu file HTML/JS/CSS viewer ditempatkan dalam `internal/graph/web/` dan di-embed dari package `graph`; jangan mencoba meng-embed `../../web` dari `internal/graph`. Simpan LICENSE vendored Cytoscape di folder vendor tersebut.

**No premature layers:** tidak perlu repository/service/provider abstraction berlapis. Adapter hanya diperlukan ketika benar-benar ada 2 engine. Paket `audit/` punya rule functions biasa, bukan extensible plugin framework.

### 8.2 Internal abstractions (minimal)

- `DBInspector` (adapter): `Inspect(ctx, scope) (Snapshot, error)`; tambah metode khusus stats hanya saat diperlukan.
- `Snapshot`: metadata lintas engine dalam JSON canonical dengan `engine` dan `capabilities`.
- `Finding`: satu rekomendasi + bukti + kemampuan sumber.
- `Rule`: implementasi berbentuk fungsi, tidak perlu registri dinamis.
- `GraphData`: node tabel dan edge FK yang dihasilkan dari snapshot.

Tidak boleh menyamakan statistik engine yang secara definisi berbeda tanpa memberi `metric_source`, `accuracy`, dan notes.

---

## 9. Database Schema Introspection

### 9.1 MySQL first (v0.1)

Gunakan metadata `INFORMATION_SCHEMA` untuk:

| Data | Source (example) |
|---|---|
| Schemas/tables/views | `INFORMATION_SCHEMA.SCHEMATA`, `TABLES` |
| Columns/type/default/nullability | `COLUMNS` |
| PK, unique, FK components | `TABLE_CONSTRAINTS`, `KEY_COLUMN_USAGE` |
| FK delete/update rules | `REFERENTIAL_CONSTRAINTS` |
| Indexes, order, uniqueness | `STATISTICS` |
| Approximate rows and allocation | `TABLES.TABLE_ROWS`, `DATA_LENGTH`, `INDEX_LENGTH` |

**Critical:** `TABLE_ROWS` pada InnoDB adalah **estimate**, bukan exact count. `DATA_LENGTH` dan `INDEX_LENGTH` mencerminkan statistik alokasi engine, bukan total disk yang benar-benar tersisa. Jangan tampilkan seolah hasil `COUNT(*)` atau kapasitas physical storage. Label `estimated` pada model dan UI.

Ambil composite FK berdasarkan constraint identity + column order. Hindari N+1 metadata queries per tabel: lakukan bounded bulk introspection per schema.

### 9.2 PostgreSQL later (v0.5)

Gunakan katalog `pg_catalog`, `information_schema` jika sesuai, `pg_stat_user_tables`, `pg_stat_user_indexes`, `pg_relation_size`/`pg_total_relation_size`. Pertahankan konsep statistik per engine: `n_live_tup` juga estimate, stats counter dapat reset, storage values punya semantik berbeda.

Jangan menyalin query MySQL secara literal ke PostgreSQL; buat adapter yang benar-benar engine-aware.

### 9.3 Relationship resolution

- **Declared relations:** FK yang ditemukan di katalog → garis solid `declared`.
- **Composite FK:** satu edge relasi dengan pasangan kolom yang diurutkan, bukan satu edge per kolom.
- **Join/pivot table:** terlihat melalui 2+ FK; klasifikasi pivot sebagai hint, bukan kesimpulan business many-to-many yang pasti.
- **Missing FK:** bila tidak ada constraint, tampilkan `no declared FK`; jangan mengklaim bahwa tidak ada hubungan pada level aplikasi.
- **Inferred relations (later, opt-in):** matching naming pattern (`user_id` → `users.id`) hanyalah *candidate*, garis putus-putus dan confidence rendah; tidak dihitung sebagai integrity rule otomatis.
- **Cross-schema:** hanya tampilkan target FK bila metadata/izin mendukung, otherwise tandai unresolved.

---

## 10. Interactive Relationship Graph (v0.1)

### 10.1 Core UI

- Daftar tabel yang bisa dicari, difilter berdasarkan schema dan status temuan.
- Node tabel menampilkan nama, perkiraan row count, ukuran data/index, dan finding count.
- Edge FK menampilkan nama constraint, child/reference columns, dan action rules.
- Zoom/pan, fit-to-screen, highlight neighbors, expand/collapse per schema.
- Klik tabel: panel `Columns`, `Indexes`, `Relations`, `Stats`, `Findings`.
- Render graph dari **snapshot JSON**; tidak melakukan SQL dari browser.
- Ukuran dan warna node ditetapkan berdasarkan indikator yang valid (mis. size/rule), bukan sekadar angka baris yang diperkirakan.
- Saat graph besar (mis. 100+ tabel), default tampilkan filtered/collapsed group, bukan semua edge sekaligus.

### 10.2 Local web server

- `dataprobe graph --input <snapshot.json> --open` membuat server statis di `127.0.0.1`.
- Default port dipilih otomatis (`:0`) untuk mengurangi benturan; tampilkan URL yang dipakai.
- Hanya handler statis snapshot dan UI; **tidak** membuat endpoint yang dapat melakukan arbitrer SQL.
- UI JS/CSS dan Cytoscape.js di-embed melalui `go:embed`; tidak bergantung CDN atau working directory.
- No wildcard CORS. No external image/font/analytics assets by default.
- Set security headers (`Content-Security-Policy` semampunya, `X-Content-Type-Options: nosniff`); hati-hati rendering text dari nama tabel agar tidak `innerHTML` unescaped.
- Token random per viewer/session pada URL atau header untuk membatasi akses cross-origin local requests bila diperlukan; tetap hanya bind localhost.
- Sanitize output dan batasi snapshot size sebelum diberikan ke browser.

### 10.3 Graph output

- v0.1 wajib: interactive local graph.
- Export graph sebagai PNG/SVG bisa ditunda jika menambah kompleksitas; HTML static standalone optional dengan asset lokal.
- Graph wajib tampil dari synthetic fixture pada CI smoke test (tanpa browser integration besar dahulu).

---

## 11. Audit Engine & Findings Model

Setiap rule menghasilkan salah satu:

- `finding`: ada kondisi yang teramati dan layak diperiksa.
- `passed`: bukti yang diperiksa memenuhi rule.
- `skipped`: dilewati karena capability/permission/budget/tabel tidak layak.
- `unknown`: data tidak cukup untuk menyimpulkan.

Format finding (contoh, **bukan data aktual**):

```json
{
  "rule_id": "IDX_002",
  "severity": "medium",
  "status": "finding",
  "scope": { "schema": "app_db", "table": "orders", "index": "idx_orders_user_id" },
  "title": "Potentially overlapping index",
  "summary": "Index columns overlap with another index prefix; manual review required.",
  "evidence": {
    "index_columns": ["user_id"],
    "overlaps_with": "idx_user_id_created_at"
  },
  "measurement": { "quality": "metadata", "source": "information_schema.statistics" },
  "impact": "Possible extra index maintenance/storage cost",
  "recommendation": "Review actual query patterns before changing indexes.",
  "auto_fix": false
}
```

### 11.1 Severity & evidence

- `critical`: bukti kuat masalah yang bisa menyebabkan kehilangan integritas/kondisi operasional berat; jangan dipakai hanya karena tabel besar.
- `high`: indikasi kuat yang harus diprioritaskan.
- `medium`: berpotensi berdampak, perlu konfirmasi workload/semantik.
- `low`: informational improvement.
- `info`: metrik saja.

**Confidence / measurement quality harus eksplisit:** `exact`, `metadata`, `estimated`, `sampled`, `unknown`; tidak mengarang persentase keyakinan statistik.

### 11.2 Initial structural rule set (v0.2)

| ID | Rule | Caveat |
|---|---|---|
| `SCH_001` | Table tanpa declared primary key | Bisa valid untuk staging/append-only tertentu |
| `SCH_002` | Foreign key menunjuk target yang tidak dapat di-resolve dalam scope | Bisa akibat permission/cross-schema |
| `IDX_001` | Duplicate index definition (columns/order/direction/uniqueness) | Tinjau index type, prefix, include/expression sebelum simpulkan |
| `IDX_002` | Potential left-prefix overlapping index | Jangan otomatis drop index; workload bisa membutuhkannya |
| `IDX_003` | FK child columns mungkin tidak memiliki index yang cocok | Engine-specific; cek urutan leading columns |
| `SCM_001` | Schema/index definition berubah dari snapshot | Drift bisa disengaja |

Tidak otomatis menganggap setiap foreign key wajib, setiap nullable column buruk, atau tabel besar bermasalah.

---

## 12. Integrity & Anomaly Audit (v0.3)

### 12.1 Rule matrix

| ID | Check | Default cost | Evidence |
|---|---|---|---|
| `INT_001` | Orphan FK references | Skip pada metadata-only | `sampled` atau `exact` |
| `INT_002` | Duplicate key berdasarkan explicit key | Skip pada metadata-only | `sampled` atau `exact` |
| `INT_003` | Missing/null fraction pada configured important columns | Skip pada metadata-only | `sampled` atau `exact` |
| `INT_004` | Relation cardinality outlier (mis. 1 user → ekstrem orders) | Tidak aktif tanpa rule config | `sampled` |
| `INT_005` | Unexpected distribution shift | Butuh baseline + field allowlist | `estimated/sample` |
| `INT_006` | Referential changes setelah migrasi | Butuh old/new snapshot atau dataset key inventory | `exact/sample` sesuai input |

**FK yang benar-benar ditegakkan DB** biasanya tidak memiliki orphan biasa pada data konsisten; pengecekan tetap berguna untuk relasi aplikasi tanpa constraint, legacy imports, atau data/export terpisah. Jangan menjual fitur ini seolah selalu menemukan orphan pada database dengan FK enforced.

### 12.2 Rules of execution

- Metadata-only adalah default dan **tidak boleh** melakukan `SELECT COUNT(*)` pada tabel besar.
- `sampled` secara eksplisit memilih keyset/PK-index-bounded rows; hindari `ORDER BY RAND()`, full scan acak, dan query mahal tanpa indeks.
- Sampling dengan `LIMIT` **tidak otomatis murah** jika predicate/order membutuhkan scan; gunakan budget dan `EXPLAIN` non-executing bila engine mendukung.
- Untuk missing/orphan, gunakan index-aware restricted joins atau data sample; pada plan berisiko, `skipped: budget_exceeded` dengan alasan.
- `full` butuh `--ack-impact`, `--max-scan-rows`, timeout, max concurrency, dan preflight query plan. Jika tidak bisa dipastikan aman, **skip** daripada memaksa.
- Identifiers tidak boleh di-bind layaknya SQL values: gunakan allowlist identifier hasil introspection + quoting sesuai dialect. Values harus parameterized.
- Jangan menjalankan arbitrary SQL dari config dalam versi awal.
- Jangan simpan contoh raw row ke report secara default. Jika suatu saat ditambah, wajib opt-in dan masking PII.

### 12.3 Config for optional business rules

```yaml
integrity:
  relationships:
    - name: order_user
      child_table: orders
      child_columns: [user_id]
      parent_table: users
      parent_columns: [id]
      nullable: false
  unique_keys:
    - table: orders
      columns: [external_order_id]
  required_columns:
    - table: orders
      columns: [id, user_id, created_at]
```

Mapping relasi custom hanya digunakan setelah diverifikasi kolom/tabel memang ada. Self-reference dan composite key harus didukung pada desain key normalization, tidak perlu full implementation semuanya di MVP.

---

## 13. Growth & "Data Explosion" Analysis (v0.4)

### 13.1 Snapshot-first principle

Pertumbuhan **tidak** dapat didiagnosis dengan akurat dari satu inspeksi. Dataprobe menyimpan beberapa snapshot metadata, lalu mengamati:

- Estimasi ukuran tabel (data/index/total).
- Estimasi row count (dengan metadata kualitas statistik).
- Jumlah tabel/indeks, perubahan struktur, tabel muncul/hilang.
- Perubahan proporsi ukuran antar tabel dan penyumbang pertumbuhan terbesar.
- (Jika engine mendukung) workload/IO counters, dengan informasi kapan stats reset.

### 13.2 Metrics & detection

| Metric | Formula / Interpretation |
|---|---|
| Absolute growth | `size_new - size_old` |
| Relative growth | `((new-old)/old)*100`, hanya saat `old>0` |
| Growth rate | `size_delta / elapsed_days`; hindari bias interval tidak sama |
| Top contributors | Urutkan tabel menurut `size_delta` |
| Suspected explosion | Flag jika **keduanya** melewati minimum absolute delta dan relative delta pada window |
| Anomaly vs baseline | Bandingkan rate terbaru dengan median/robust baseline setelah >= 3 snapshots |
| Capacity projection | Tampilkan hanya bila capacity limit diketahui, snapshots cukup, dan interval konsisten; beri range/uncertainty |

Threshold default ilustratif yang bisa diubah: `>30%` **dan** `>100 MiB` dalam 7 hari. Nilai ini **heuristic**, bukan standar industri. Tabel tumbuh 200% dari 1 KiB tetap bukan insiden.

### 13.3 Storage & false-positive policy

- SQL metadata table bytes ≠ seluruh ukuran database pada filesystem.
- Ukuran indeks yang naik bukan otomatis index bloat.
- Disk penuh tidak bisa diprediksi hanya dari total SQL table sizes; **free disk / disk quota** harus diberikan dari sumber tersendiri yang dipercaya.
- Metadata InnoDB dapat berubah akibat refresh statistik tanpa perubahan row nyata.
- Jangan menjanjikan pendeteksian "database meledak" yang pasti; output adalah **growth anomaly / capacity risk signal**, bukan diagnosis final.
- Hindari melabeli vacuum/bloat pada PostgreSQL tanpa evidence yang sesuai.

### 13.4 Snapshot comparison example

```text
DATAPROBE GROWTH REPORT (illustrative)
Period: 2026-10-08 → 2026-10-15

Database estimated table+index allocation: 2.1 GiB → 3.0 GiB
Delta: +0.9 GiB (+42.9%)

Top growth contributors:
  app_db.event_logs     +620 MiB   REVIEW
  app_db.orders         +190 MiB   INFO
  app_db.order_items     +65 MiB   INFO

Findings:
  [MEDIUM] event_logs growth above configured threshold
  [INFO]   table row counts are storage-engine estimates
  [UNKNOWN] disk exhaustion risk — no free-space metric available
```

---

## 14. Performance/Optimization Advisor (v0.2+ metadata, later workload)

### 14.1 What tool CAN say

- `orders` has overlapping index signatures worth reviewing.
- FK child columns may lack usable leading-prefix index.
- A table is large relative to others; investigate intended retention policy.
- MySQL `performance_schema` counters (if available) indicate heavy table/index I/O and warrant investigation.
- PostgreSQL table/index stats (if available) show scans, estimated tuples, and maintenance-related signals.

### 14.2 What tool CANNOT claim automatically

- "This index is useless" from zero usage alone (counters may reset and periodic workload matters).
- "This query is slow" without query workload or execution evidence.
- "Database is optimized" based purely on schema health.
- "Drop this index" as an automatic fix.

### 14.3 Optional workload phase (after baseline)

- Prefer read-only metrics from engine monitoring catalogs/views.
- `EXPLAIN` only for **explicit user-supplied safe SELECT** in a later feature, without automatically executing query; run guarded and with privileges/timeouts.
- **Never invoke `EXPLAIN ANALYZE` automatically**: it executes statements and can be expensive.
- Avoid `SHOW FULL PROCESSLIST` collection or raw SQL text retention by default; SQL literals may contain personal/sensitive data.

---

## 15. Snapshot Schema & Data Contract

Snapshot is the most important reusable contract. Stabilize it before adding more engines.

### 15.1 Minimal sample (illustrative)

```json
{
  "schema_version": 1,
  "tool_version": "0.1.0",
  "captured_at": "2026-10-15T00:00:00Z",
  "source": {
    "engine": "mysql",
    "server_version": "8.4.x",
    "alias": "prod-app",
    "schemas": ["app_db"]
  },
  "capabilities": {
    "table_statistics": true,
    "workload_statistics": false,
    "full_integrity_scan": false
  },
  "tables": [
    {
      "schema": "app_db",
      "name": "users",
      "kind": "base_table",
      "row_count": {"value": 120000, "quality": "estimated"},
      "storage_bytes": {"data": 12000000, "indexes": 4000000, "quality": "estimated"},
      "columns": [{"name":"id","db_type":"bigint","nullable":false}],
      "primary_key": ["id"],
      "indexes": [{"name":"PRIMARY","columns":["id"],"unique":true}]
    },
    {
      "schema": "app_db",
      "name": "orders",
      "kind": "base_table",
      "row_count": {"value": 850000, "quality": "estimated"},
      "storage_bytes": {"data": 65000000, "indexes": 22000000, "quality": "estimated"},
      "columns": [
        {"name":"id","db_type":"bigint","nullable":false},
        {"name":"user_id","db_type":"bigint","nullable":false}
      ],
      "primary_key": ["id"],
      "indexes": []
    }
  ],
  "foreign_keys": [{
    "name": "fk_orders_users",
    "from": {"schema":"app_db","table":"orders","columns":["user_id"]},
    "to": {"schema":"app_db","table":"users","columns":["id"]},
    "origin": "declared",
    "on_delete": "RESTRICT"
  }],
  "findings": [],
  "warnings": []
}
```

### 15.2 Contract rules

- Top-level `schema_version` wajib; reject unsupported **major** contract changes dengan pesan jelas.
- Semua array stabil dan diurutkan deterministik (schema, table, index) untuk diff.
- Tidak mencantumkan DSN, password, username koneksi, IP/host internal default, query parameter sensitif, raw data rows, atau SQL text.
- `source.alias` diisi pengguna agar report mudah dibandingkan; bisa dihapus dengan mode anonymize.
- Simpan `NULL/unknown` saat stats tidak tersedia; jangan mengubahnya menjadi `0`.
- Snapshot dari 2 engine yang berbeda bisa dibandingkan struktur secara terbatas, tetapi storage stats **tidak dianggap setara** secara otomatis.
- Type differences dicatat; JSON fields untuk fitur baru harus additive sebisa mungkin.
- Time always UTC RFC3339. Integers untuk byte, counts, duration; hindari floating precision untuk total bytes.

### 15.3 Report formats

- CLI `table` manusiawi, `json` machine-readable; Markdown optional.
- Finding memiliki `rule_id`, `severity`, `status`, `scope`, `evidence`, `measurement`, `recommendation`, `auto_fix=false`.
- Report footer menjelaskan `partial/incomplete`, skipped rules, dan scope audit.
- **Snapshot** = kondisi database; **audit report** = interpretasi rules; **diff** = perubahan lintas snapshot. Jangan mencampur ketiganya tanpa skema jelas.

---

## 16. Security, Privacy & Production Safety

### 16.1 Default safeguards (MUST)

1. **Credentials:** DSN berasal dari env var, secret file/manager, atau prompt tersembunyi; tidak dipersist ke snapshot/report/log.
2. **Account privileges:** minta akun DB read-only dan minimal scoped grants. Akses metadata bisa berbeda menurut engine dan privileges.
3. **No mutation:** tidak pernah mengeksekusi DDL/DML untuk autofix, bahkan jika akun ternyata punya privilege write.
4. **Query budget:** context timeout, overall timeout, capped open connections, max concurrency konservatif.
5. **Metadata only default:** tidak melakukan full `COUNT(*)`, `GROUP BY` pada tabel bisnis, key scans, atau cross-table joins.
6. **Deeper audit explicit:** sampled/full hanya ketika dipilih; full memerlukan `--ack-impact` dan preflight.
7. **No raw rows by default:** hanya schema + stats + agregat yang aman; agregat tetap berpotensi sensitif dalam konteks tertentu, ingatkan pengguna.
8. **Local UI:** bind loopback saja, no write endpoints, embedded static assets, no analytics/telemetry.
9. **Safe identifiers:** schema/table/column berasal dari introspection + validated allowlist + dialect quoting; never interpolate user strings unsafely.
10. **Safe logging:** output error menghapus password/token; snapshot boleh memiliki anonymization mode.
11. **Permissions:** snapshot lokal sebaiknya file mode `0600`, direktori sesuai kebutuhan.
12. **No auto uploads:** tidak ada telemetry dan transfer data ke cloud.

### 16.2 Explicit production boundaries

- Read-only SQL **tetap bisa mahal**: join/aggregate besar dapat mengganggu CPU, I/O, buffer cache. Batas harus berbasis plan bila memungkinkan dan `skipped` adalah outcome valid.
- Timeout client bukan jaminan sempurna bahwa DB server tidak pernah bekerja sesaat setelah cancel; gunakan native server-side statement timeouts apabila tersedia dan terdokumentasi.
- No automatic exact row counts on InnoDB.
- Metadata visibility tergantung grant; gagal membaca metrik bukan berarti nilainya nol.
- Audit schedule manual di v1, sehingga pengguna mengontrol kapan aktivitas DB terjadi.

---

## 17. Performance & Scalability

- Target metadata introspection skema kecil/sedang (mis. 10–200 tabel) dalam beberapa detik pada koneksi sehat; ini **target benchmark, bukan SLA**.
- Jangan melakukan per-table query ketika set metadata dapat diperoleh dalam satu query per tipe sumber.
- Viewer dibangun dari snapshot; setelah load, tidak men-query database.
- Gunakan bounded result sets, page/filter graph, dan lazy details untuk 500+ node.
- Snapshot report harus streaming output bila ukurannya besar; untuk v0.1 metadata ringan, `encoding/json` sudah cukup.
- Untuk CSV/JSONL v0.6: gunakan `bufio`/`encoding/csv`/`encoding/json.Decoder` dan iterator streaming; duplicate/cross-file joins berskala jutaan butuh external sort/temp disk atau index, **jangan** berasumsi semua key muat di RAM.
- Boleh menyediakan `--profile` atau benchmark internal; **jangan** menambahkan OpenTelemetry/Prometheus default jika belum dibutuhkan.

---

## 18. Testing Strategy

### 18.1 Unit tests (tanpa DB)

- Config validation dan secret-redaction.
- Normalisasi composite PK/FK dan penyusunan relationship edges.
- Deterministic snapshot serialization dan migration/error untuk schema version berbeda.
- Structural rules: missing PK, duplicate/overlapping indexes, FK child index coverage.
- Diff: table created/dropped, index changed, null metrics, zero baseline, elapsed time = 0.
- Growth: two snapshots (insufficient baseline), three+ snapshots, jitter/estimate changes, absolute + relative threshold, missing stats.
- JSON schema, exit status, `--format json` pure output, file permissions.
- Graph injection-safe rendering untuk nama tabel tidak tepercaya.

### 18.2 Integration tests

- MySQL local test DB (isolated ephemeral service di CI bila diperlukan) dengan schema fixture: tables, views, composite FK, no FK, unique indexes, generated columns.
- `inspect -> snapshot -> graph` end-to-end dari fixture; graph endpoint hanya loopback dan tidak menjalankan SQL.
- Test missing privilege dan metadata partial visibility.
- Test query cancellation, connection timeout, slow server, statement timeouts.
- Real data tidak boleh masuk repository.

### 18.3 Security & negative tests

- DSN tidak pernah muncul pada report/error.
- Template/DOM graph bebas XSS melalui malicious table names.
- `--mode full` tanpa explicit confirmation ditolak.
- Audit yang melewati budget di-skip, bukan retry tanpa batas.
- Default audit tidak menjalankan expensive row queries (assert menggunakan query spy/fake adapter).
- Export snapshot tidak pernah menampung raw rows/credential fields.
- Concurrency/race test untuk code yang memang menjalankan goroutine.

### 18.4 CI

```bash
gofmt -l .
go vet ./...
go test ./...
go test -race ./...
go build ./cmd/dataprobe
```

Tambahkan vulnerability/security scan bila memberi nilai (misalnya `govulncheck`). GitHub release multi-platform (`darwin/linux/windows`, `amd64/arm64` sesuai dukungan), checksums, changelog, license.

---

## 19. Implementation Phases & Milestones

### Phase 0 — Foundation (1–2 hari engineering, estimasi non-binding)

- [ ] Repo `dataprobe`, Go module, MIT LICENSE, README skeleton.
- [ ] CLI `version`, `--help`, error/exit-code conventions.
- [ ] YAML config + DSN env; strict validation, no-credential logging.
- [ ] Canonical snapshot model v1 + JSON serializer.
- [ ] CI test/vet/build/race; synthetic sample fixture.

**Exit criteria:** CLI build di Mac/Linux, config dan serialization tests lolos.

### Phase 1 — MySQL Inspector & Graph MVP (v0.1)

- [ ] MySQL read-only connection test, connection pool, timeout/TLS.
- [ ] Bulk schema/table/column/PK/FK/index introspection.
- [ ] Row-size estimate dengan label `estimated`.
- [ ] Composite key normalization + deterministic ordering.
- [ ] `dataprobe inspect` terminal/JSON.
- [ ] `dataprobe snapshot --out` file aman dan versioned.
- [ ] `dataprobe graph --input` local server + Cytoscape + `go:embed`.
- [ ] Search/filter table, relationship highlight, details panel.
- [ ] Synthetic demo snapshot dan tests untuk graph/metadata.

**Exit criteria:** membuka ER graph dari snapshot tanpa DB connection, termasuk composite FK, installer single binary bekerja dari direktori mana pun.

### Phase 2 — Structural Audit (v0.2)

- [ ] Rule engine sederhana berbasis fungsi dan finding JSON.
- [ ] Missing PK, duplicate/overlap index (dengan caveats), FK index coverage.
- [ ] Findings table & graph overlay.
- [ ] `audit --input` offline, output JSON/table.
- [ ] `skipped`, `unknown`, dan issue evidence.

**Exit criteria:** temuan explainable, tidak pernah membuat rekomendasi drop indeks secara otomatis.

### Phase 3 — Integrity Audit (v0.3)

- [ ] Preflight query-cost/budget dan permission guard.
- [ ] Explicit relationship/required/unique rules config.
- [ ] Sampling dengan index-aware bounded queries.
- [ ] Optional full mode hanya jika layak dan dikonfirmasi.
- [ ] Orphan/duplicate/null findings, exact/sample labels, skipped reasons.
- [ ] Integration tests pada data fixture (bukan prod).

**Exit criteria:** metadata-only tidak scan data; sampled/full tidak diam-diam membebani tabel besar dan tidak pernah menyimpan raw rows default.

### Phase 4 — Growth & Snapshot Diff (v0.4)

- [ ] Snapshot diff: tables, relations, indexes, sizes, estimate deltas.
- [ ] Top growth contributors dan heuristic growth anomalies.
- [ ] Minimum 3 snapshots untuk baseline; dua cukup untuk delta saja.
- [ ] Trend UI lokal yang sederhana; jangan buat dashboard monitoring.
- [ ] Disk capacity hanya jika supplied/credible data available.

**Exit criteria:** no false precision, missing metrics ditandai unknown, non-comparable engine stats tidak dibandingkan sembarangan.

### Phase 5 — PostgreSQL Support (v0.5)

- [ ] Adapter introspection PostgreSQL.
- [ ] Engine-specific stats (`pg_stat_*`, relation size) dengan kualitas metrik.
- [ ] Schema/composite FK fixtures dan integration CI.
- [ ] Per-engine rule applicability/capabilities matrix.

**Exit criteria:** fungsi utama inspect/snapshot/graph/diff lintas engine, audit hanya menjalankan rules yang sah untuk engine masing-masing.

### Phase 6 — Offline Data Files (v0.6, optional)

- [ ] CSV/JSONL inspect + schema/column inference terbatas.
- [ ] Required fields, type validation, duplicate keys streaming.
- [ ] Relationship checks antar-file dengan declarative mapping.
- [ ] External sort/disk-based algorithm untuk file besar atau explicit limit/skip.

**Exit criteria:** export audit berguna tanpa DB hidup; tidak mengklaim bisa parse seluruh SQL dump.

### Phase 7 — Stable OSS Release (v1.0)

- [ ] Review docs, CLI compatibility, JSON schema versioning.
- [ ] Threat model dan security hardening.
- [ ] Performance benchmarks MySQL 100–500 tables, sample multi-million rows di test env.
- [ ] Full CI/release artifacts, checksum, supported platform matrix.
- [ ] Known limitations, SECURITY.md, CONTRIBUTING.md, CHANGELOG.md.
- [ ] Publish release setelah confidence pada correctness; jangan mengunci tanggal artifisial.

---

## 20. Suggested GitHub Issues / Kanban Backlog

| # | Title | Phase | Priority | Acceptance |
|---|---|---|---|---|
| 1 | Bootstrap Go CLI + CI | 0 | P0 | builds/tests |
| 2 | Add configuration & secret-safe DSN loading | 0 | P0 | credentials redacted |
| 3 | Define v1 snapshot contract | 0 | P0 | versioned JSON roundtrip |
| 4 | Implement MySQL schema metadata reader | 1 | P0 | table/column/PK/FK |
| 5 | Read MySQL indexes and estimated sizes | 1 | P0 | composite keys, accuracy label |
| 6 | Add inspect/snapshot CLI commands | 1 | P0 | deterministic output |
| 7 | Implement static local graph viewer | 1 | P0 | no CDN, go:embed |
| 8 | Add graph filters and per-table detail panel | 1 | P1 | usable on 100+ tables |
| 9 | Implement structural findings & audits | 2 | P0 | evidence and caveats |
| 10 | Add audit CLI and graph finding overlays | 2 | P1 | JSON/table parity |
| 11 | Add query-budget and integrity preflight | 3 | P0 | skip unsafe checks |
| 12 | Implement opt-in bounded integrity checks | 3 | P0 | sampled/exact distinguished |
| 13 | Implement snapshot diff & schema drift | 4 | P0 | old/new delta |
| 14 | Implement growth rules & trend report | 4 | P1 | baselines & unknowns |
| 15 | PostgreSQL inspector & tests | 5 | P1 | capability parity |
| 16 | CSV/JSONL streaming validation | 6 | P2 | bounded RAM |
| 17 | Security & reliability hardening | 7 | P0 | acceptance suite |
| 18 | Publish cross-platform v1 release | 7 | P1 | binary + checksums |

**Issue policy:** satu issue = deliverable yang bisa diuji. Jangan buat tiket untuk puluhan kelas/interface sebelum kebutuhan muncul.

---

## 21. Risks, Constraints, Decision Log

| Risk | Consequence | Mitigation |
|---|---|---|
| Full-table scan mengganggu production | Latency/CPU/I/O | metadata-first, opt-in budget, skip unsafe |
| MySQL TABLE_ROWS tidak akurat | false anomaly | evidence quality, delta caveat, repeated sampling |
| Relasi tak dideklarasikan FK | missing graph edges | custom mapping + inferred dashed links later |
| Index advice tanpa workload | destructive suggestion | candidates only; manual verification |
| Sensitive schema/host leakage | security exposure | anonymize snapshot, secret-free logs, no remote UI |
| Graph ratusan tabel rumit | poor UX | filter, collapse, focus neighbors, layout control |
| Overengineering multi-engine | slow release | MySQL first; adapter only when PG arrives |
| Too many features | maintenance burden | release per phase, hard non-goals |
| External SQL dumps vary | incorrect parsing | only CSV/JSONL; restore to isolated DB if needed |

### Decisions already made

- **ADR-001:** CLI-first, single binary, local graph UI.
- **ADR-002:** No database for Dataprobe's own storage; JSON snapshots.
- **ADR-003:** Metadata-only by default; explicit sampled/full scan.
- **ADR-004:** MySQL first; PostgreSQL later with engine-aware adapter.
- **ADR-005:** No automatic mutation or database optimization fixes.
- **ADR-006:** Structural evidence and audit findings are separate from health/optimization verdicts.

---

## 22. Definition of Done (for each phase)

- [ ] Implements only scoped requirements; no unused abstractions or dead features.
- [ ] User-facing CLI behavior documented with examples and stable exit codes.
- [ ] `go test ./...`, `go test -race ./...`, `go vet ./...` pass.
- [ ] No credentials or raw sample data included in tests/reports by accident.
- [ ] Relevant security limits covered by negative tests.
- [ ] Metadata estimates, skips, and uncertainty explicitly reflected in output.
- [ ] README/CHANGELOG updated when interface/schema changes.
- [ ] Binary runs outside source checkout (`go:embed` for web assets).
- [ ] Existing snapshot files remain readable within documented compatibility range.
- [ ] No production data mutation; no unconsented row-level scans.

### Definition of Done (v0.1 MVP)

> On a local MySQL fixture with users/orders/order_items, executing `inspect` and `snapshot` produces deterministic, secret-free metadata, and `graph --input` opens a relationship view showing columns, indexes, and FK edges, without needing a running database after snapshot capture.

### Definition of Done (long-term v1.0)

> A developer can connect read-only to MySQL or PostgreSQL, inspect schema and relationships, run bounded evidence-based audits, capture sanitized snapshots, compare changes/growth, and understand what was verified versus estimated, all from a portable Go binary without a separate service.

---

## 23. Recommended First Implementation Slice (for Codex)

Use this order **without implementing later phases yet**:

1. Scaffold `cmd/dataprobe`, `internal/cli`, `internal/config`, `internal/model`, `internal/snapshot`.
2. Implement `dataprobe version`, `--help`, and strict config parser that loads DSN from environment.
3. Define snapshot schema v1 + fixtures + round-trip tests.
4. Implement MySQL introspection for columns/PK/FK/indexes using bulk metadata queries.
5. Implement `inspect` and `snapshot` CLI.
6. Implement embedded static Cytoscape graph reading a snapshot file.
7. Add tests and README examples, release `v0.1.0`.

**Do not implement now:** PostgreSQL, CSV/JSONL, AI analysis, full scans, historical growth, schedulers, cloud services, and generic plugin systems.

### Prompt to Codex

```text
Implement the Dataprobe project using DATAPROBE_PLAN.md as the source of truth.
Start with Phase 0 and Phase 1 only (target v0.1.0).
Prioritize correctness, secure read-only defaults, idiomatic Go, simple package design,
and deterministic tests. Do not add database writes, full row scans, or premature
abstractions. Build a local graph viewer from a versioned snapshot using embedded
static assets so that the binary works outside the repository directory.
At the end: provide CLI usage, changed files, test results, known limitations,
and a checklist mapping work to the Phase 0/1 acceptance criteria.
```

---

## 24. Technical Reference Sources

- Go release history: https://go.dev/doc/devel/release
- MySQL INFORMATION_SCHEMA: https://dev.mysql.com/doc/refman/8.4/en/information-schema-general-table-reference.html
- MySQL TABLES row estimates / size caveats: https://dev.mysql.com/doc/refman/8.4/en/information-schema-tables-table.html
- MySQL Performance Schema: https://dev.mysql.com/doc/refman/8.4/en/performance-schema-table-wait-summary-tables.html
- MySQL EXPLAIN / EXPLAIN ANALYZE: https://dev.mysql.com/doc/refman/8.4/en/explain.html
- PostgreSQL cumulative statistics: https://www.postgresql.org/docs/current/monitoring-stats.html
- Cytoscape.js: https://js.cytoscape.org/

**Maintenance note:** verify driver compatibility and SQL catalog fields against the exact server versions supported by the release. Avoid implying all server versions expose identical statistics.
