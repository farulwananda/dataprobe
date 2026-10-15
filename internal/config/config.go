package config

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// Config represents the dataprobe configuration file schema.
type Config struct {
	Version int          `yaml:"version"`
	Source  SourceConfig `yaml:"source"`
	Audit   AuditConfig  `yaml:"audit"`
	Growth  GrowthConfig `yaml:"growth"`
	Graph   GraphConfig  `yaml:"graph"`
}

// SourceConfig defines database connection details.
type SourceConfig struct {
	Engine         string   `yaml:"engine"`
	DSNEnv         string   `yaml:"dsn_env"`
	Alias          string   `yaml:"alias"`
	IncludeSchemas []string `yaml:"include_schemas"`
	ExcludeTables  []string `yaml:"exclude_tables"`
}

// AuditConfig controls audit behavior and thresholds.
type AuditConfig struct {
	Profile        string        `yaml:"profile"` // metadata | sampled | full
	Timeout        time.Duration `yaml:"timeout"`
	TotalBudget    time.Duration `yaml:"total_budget"`
	MaxConcurrency int           `yaml:"max_concurrency"`
	MaxSampleRows  int64         `yaml:"max_sample_rows"`
	MaxScanRows    int64         `yaml:"max_scan_rows"`
	FailOn         string        `yaml:"fail_on"`
	Rules          []string      `yaml:"rules"`
}

// GrowthConfig controls snapshot delta evaluations.
type GrowthConfig struct {
	MinSnapshots     int     `yaml:"min_snapshots"`
	AlertRelativePct float64 `yaml:"alert_relative_pct"`
	AlertAbsoluteMiB int64   `yaml:"alert_absolute_mib"`
	LookbackDays     int     `yaml:"lookback_days"`
}

// GraphConfig controls relationship graph options.
type GraphConfig struct {
	HideSystemTables      bool `yaml:"hide_system_tables"`
	ShowIndexes           bool `yaml:"show_indexes"`
	ShowInferredRelations bool `yaml:"show_inferred_relations"`
}

// Load loads and parses a YAML configuration file from path.
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("could not read config file: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("invalid YAML in config file: %w", err)
	}

	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("config validation failed: %w", err)
	}

	return &cfg, nil
}

// Validate ensures required fields and safety boundaries are observed.
func (c *Config) Validate() error {
	if c.Version != 1 {
		return fmt.Errorf("unsupported config version: %d (expected 1)", c.Version)
	}

	engine := strings.ToLower(strings.TrimSpace(c.Source.Engine))
	if engine == "" {
		return errors.New("source.engine is required")
	}
	if engine != "mysql" {
		return fmt.Errorf("unsupported database engine: %q (supported: mysql)", engine)
	}
	c.Source.Engine = engine

	if strings.TrimSpace(c.Source.DSNEnv) == "" {
		c.Source.DSNEnv = "DATAPROBE_DSN"
	}

	if len(c.Source.IncludeSchemas) == 0 {
		return errors.New("source.include_schemas must contain at least one schema name")
	}

	if c.Audit.Timeout <= 0 {
		c.Audit.Timeout = 10 * time.Second
	}
	if c.Audit.TotalBudget <= 0 {
		c.Audit.TotalBudget = 60 * time.Second
	}
	if c.Audit.MaxConcurrency <= 0 {
		c.Audit.MaxConcurrency = 2
	}
	if c.Audit.MaxConcurrency > 8 {
		c.Audit.MaxConcurrency = 8 // Guard against excessive connection spikes
	}

	if c.Audit.FailOn == "" {
		c.Audit.FailOn = "high"
	}

	return nil
}

// ResolveDSN retrieves the connection DSN from the environment variable specified in config.
// The DSN is NEVER persisted to disk or snapshots.
func (c *Config) ResolveDSN() (string, error) {
	envVar := strings.TrimSpace(c.Source.DSNEnv)
	if envVar == "" {
		envVar = "DATAPROBE_DSN"
	}

	dsn := strings.TrimSpace(os.Getenv(envVar))
	if dsn == "" {
		return "", fmt.Errorf("environment variable %q is empty or not set; export your read-only database DSN", envVar)
	}

	return dsn, nil
}

// RedactDSN strips passwords and secrets from connection strings for safe logging/errors.
func RedactDSN(dsn string) string {
	// Standard MySQL DSN: user:pass@tcp(host:port)/dbname
	if atIdx := strings.Index(dsn, "@"); atIdx != -1 {
		userPass := dsn[:atIdx]
		rest := dsn[atIdx:]
		if colonIdx := strings.Index(userPass, ":"); colonIdx != -1 {
			return userPass[:colonIdx] + ":[REDACTED]" + rest
		}
	}
	return dsn
}
