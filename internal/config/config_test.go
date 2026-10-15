package config_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/farulwananda/dataprobe/internal/config"
)

func TestConfig_LoadAndValidate(t *testing.T) {
	tempDir := t.TempDir()
	cfgPath := filepath.Join(tempDir, "dataprobe.yaml")

	yamlContent := `version: 1
source:
  engine: mysql
  dsn_env: TEST_DATAPROBE_DSN
  alias: test-store
  include_schemas:
    - store_db
  exclude_tables:
    - "*_cache"

audit:
  profile: metadata
  timeout: 5s
  total_budget: 30s
  max_concurrency: 3
  fail_on: high
`
	if err := os.WriteFile(cfgPath, []byte(yamlContent), 0600); err != nil {
		t.Fatal(err)
	}

	cfg, err := config.Load(cfgPath)
	if err != nil {
		t.Fatalf("config.Load failed: %v", err)
	}

	if cfg.Version != 1 {
		t.Errorf("expected version 1, got %d", cfg.Version)
	}
	if cfg.Source.Engine != "mysql" {
		t.Errorf("expected engine mysql, got %s", cfg.Source.Engine)
	}
	if cfg.Audit.Timeout != 5*time.Second {
		t.Errorf("expected timeout 5s, got %v", cfg.Audit.Timeout)
	}

	// Test DSN resolution
	t.Setenv("TEST_DATAPROBE_DSN", "user:secret@tcp(localhost:3306)/store_db")
	dsn, err := cfg.ResolveDSN()
	if err != nil {
		t.Fatalf("ResolveDSN failed: %v", err)
	}
	if dsn != "user:secret@tcp(localhost:3306)/store_db" {
		t.Errorf("unexpected dsn: %s", dsn)
	}

	redacted := config.RedactDSN(dsn)
	if redacted != "user:[REDACTED]@tcp(localhost:3306)/store_db" {
		t.Errorf("expected redacted DSN, got %s", redacted)
	}
}

func TestConfig_MissingDSNEnv(t *testing.T) {
	tempDir := t.TempDir()
	cfgPath := filepath.Join(tempDir, "dataprobe.yaml")

	yamlContent := `version: 1
source:
  engine: mysql
  dsn_env: NON_EXISTENT_VAR_12345
  include_schemas:
    - store_db
`
	if err := os.WriteFile(cfgPath, []byte(yamlContent), 0600); err != nil {
		t.Fatal(err)
	}

	cfg, err := config.Load(cfgPath)
	if err != nil {
		t.Fatalf("config.Load failed: %v", err)
	}

	_, err = cfg.ResolveDSN()
	if err == nil {
		t.Fatal("expected error for unset DSN env var, got nil")
	}
}
