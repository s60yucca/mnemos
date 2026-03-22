package config

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/viper"
)

// Config holds all application configuration
type Config struct {
	DataDir     string            `mapstructure:"data_dir"`
	LogLevel    string            `mapstructure:"log_level"`
	LogFormat   string            `mapstructure:"log_format"`
	Embeddings  EmbeddingConfig   `mapstructure:"embeddings"`
	Mirror      MirrorConfig      `mapstructure:"mirror"`
	Lifecycle   LifecycleConfig   `mapstructure:"lifecycle"`
	Dedup       DedupConfig       `mapstructure:"dedup"`
	Server      ServerConfig      `mapstructure:"server"`
	Hook        HookConfig        `mapstructure:"hook"`
	QualityGate QualityGateConfig `mapstructure:"quality_gate"`
}

// QualityGateConfig holds configuration for the memory quality gate pipeline stage.
type QualityGateConfig struct {
	Enabled            bool               `mapstructure:"enabled"`
	MinWords           int                `mapstructure:"min_words"`
	MaxWords           int                `mapstructure:"max_words"`
	MinDensity         float64            `mapstructure:"min_density"`
	DuplicateThreshold float64            `mapstructure:"duplicate_threshold"`
	RequireSpecific    bool               `mapstructure:"require_specific"`
	Penalties          map[string]float64 `mapstructure:"penalties"`
	ScoreBands         ScoreBandsConfig   `mapstructure:"score_bands"`
}

// ScoreBandsConfig defines the score thresholds for quality gate verdicts.
type ScoreBandsConfig struct {
	Accept    float64 `mapstructure:"accept"`    // default 0.8
	Fix       float64 `mapstructure:"fix"`       // default 0.5
	Downgrade float64 `mapstructure:"downgrade"` // default 0.3
}

type EmbeddingConfig struct {
	Enabled  bool   `mapstructure:"enabled"`
	Provider string `mapstructure:"provider"` // "noop", "ollama", "openai"
	Model    string `mapstructure:"model"`
	APIKey   string `mapstructure:"api_key"`
	BaseURL  string `mapstructure:"base_url"`
	Dims     int    `mapstructure:"dims"`
}

type MirrorConfig struct {
	Enabled bool   `mapstructure:"enabled"`
	BaseDir string `mapstructure:"base_dir"`
}

type LifecycleConfig struct {
	DecayInterval    time.Duration `mapstructure:"decay_interval"`
	GCRetentionDays  int           `mapstructure:"gc_retention_days"`
	ArchiveThreshold float64       `mapstructure:"archive_threshold"`
}

type DedupConfig struct {
	FuzzyThreshold    float64 `mapstructure:"fuzzy_threshold"`
	SemanticThreshold float64 `mapstructure:"semantic_threshold"`
}

type ServerConfig struct {
	Port    int    `mapstructure:"port"`
	MCPMode string `mapstructure:"mcp_mode"` // "stdio", "sse"
}

type HookConfig struct {
	Enabled                  bool          `mapstructure:"enabled"`
	SessionDir               string        `mapstructure:"session_dir"`
	StaleTimeout             time.Duration `mapstructure:"stale_timeout"`
	CleanupRetention         time.Duration `mapstructure:"cleanup_retention"`
	SearchCooldown           time.Duration `mapstructure:"search_cooldown"`
	TopicSimilarityThreshold float64       `mapstructure:"topic_similarity_threshold"`
	SessionEndBreadcrumb     bool          `mapstructure:"session_end_breadcrumb"`
	SessionStartMaxTokens    int           `mapstructure:"session_start_max_tokens"`
	PromptSearchLimit        int           `mapstructure:"prompt_search_limit"`
	LogLevel                 string        `mapstructure:"log_level"`
}

// DefaultConfig returns sensible defaults
func DefaultConfig() *Config {
	home, _ := os.UserHomeDir()
	dataDir := filepath.Join(home, ".mnemos")
	return &Config{
		DataDir:   dataDir,
		LogLevel:  "info",
		LogFormat: "text",
		Embeddings: EmbeddingConfig{
			Enabled:  false,
			Provider: "noop",
			Dims:     384,
		},
		Mirror: MirrorConfig{
			Enabled: true,
			BaseDir: filepath.Join(dataDir, "memories"),
		},
		Lifecycle: LifecycleConfig{
			DecayInterval:    24 * time.Hour,
			GCRetentionDays:  30,
			ArchiveThreshold: 0.1,
		},
		Dedup: DedupConfig{
			FuzzyThreshold:    0.85,
			SemanticThreshold: 0.92,
		},
		Server: ServerConfig{
			Port:    8080,
			MCPMode: "stdio",
		},
		Hook: HookConfig{
			Enabled:                  true,
			SessionDir:               "sessions",
			StaleTimeout:             1 * time.Hour,
			CleanupRetention:         24 * time.Hour,
			SearchCooldown:           5 * time.Minute,
			TopicSimilarityThreshold: 0.3,
			SessionEndBreadcrumb:     false,
			SessionStartMaxTokens:    2000,
			PromptSearchLimit:        5,
			LogLevel:                 "warn",
		},
		QualityGate: QualityGateConfig{
			Enabled:            true,
			MinWords:           5,
			MaxWords:           200,
			MinDensity:         0.3,
			DuplicateThreshold: 0.8,
			RequireSpecific:    true,
			Penalties: map[string]float64{
				"too_short":      1.0,
				"too_long":       0.1,
				"low_density":    0.3,
				"near_duplicate": 0.4,
				"too_generic":    0.2,
			},
			ScoreBands: ScoreBandsConfig{
				Accept:    0.8,
				Fix:       0.5,
				Downgrade: 0.3,
			},
		},
	}
}

// LoadConfig loads configuration from files and environment variables
func LoadConfig(cfgFile string) (*Config, error) {
	v := viper.New()
	cfg := DefaultConfig()

	// Set defaults
	v.SetDefault("data_dir", cfg.DataDir)
	v.SetDefault("log_level", cfg.LogLevel)
	v.SetDefault("log_format", cfg.LogFormat)
	v.SetDefault("embeddings.enabled", cfg.Embeddings.Enabled)
	v.SetDefault("embeddings.provider", cfg.Embeddings.Provider)
	v.SetDefault("embeddings.dims", cfg.Embeddings.Dims)
	v.SetDefault("mirror.enabled", cfg.Mirror.Enabled)
	v.SetDefault("mirror.base_dir", cfg.Mirror.BaseDir)
	v.SetDefault("lifecycle.decay_interval", cfg.Lifecycle.DecayInterval)
	v.SetDefault("lifecycle.gc_retention_days", cfg.Lifecycle.GCRetentionDays)
	v.SetDefault("lifecycle.archive_threshold", cfg.Lifecycle.ArchiveThreshold)
	v.SetDefault("dedup.fuzzy_threshold", cfg.Dedup.FuzzyThreshold)
	v.SetDefault("dedup.semantic_threshold", cfg.Dedup.SemanticThreshold)
	v.SetDefault("server.port", cfg.Server.Port)
	v.SetDefault("server.mcp_mode", cfg.Server.MCPMode)
	v.SetDefault("hook.enabled", cfg.Hook.Enabled)
	v.SetDefault("hook.session_dir", cfg.Hook.SessionDir)
	v.SetDefault("hook.stale_timeout", cfg.Hook.StaleTimeout)
	v.SetDefault("hook.cleanup_retention", cfg.Hook.CleanupRetention)
	v.SetDefault("hook.search_cooldown", cfg.Hook.SearchCooldown)
	v.SetDefault("hook.topic_similarity_threshold", cfg.Hook.TopicSimilarityThreshold)
	v.SetDefault("hook.session_end_breadcrumb", cfg.Hook.SessionEndBreadcrumb)
	v.SetDefault("hook.session_start_max_tokens", cfg.Hook.SessionStartMaxTokens)
	v.SetDefault("hook.prompt_search_limit", cfg.Hook.PromptSearchLimit)
	v.SetDefault("hook.log_level", cfg.Hook.LogLevel)

	v.SetEnvPrefix("MNEMOS")
	v.AutomaticEnv()

	if cfgFile != "" {
		v.SetConfigFile(cfgFile)
	} else {
		v.SetConfigName("config")
		v.SetConfigType("yaml")
		// Project-local takes precedence
		v.AddConfigPath(".mnemos")
		home, _ := os.UserHomeDir()
		v.AddConfigPath(filepath.Join(home, ".mnemos"))
	}

	_ = v.ReadInConfig() // ignore not found

	if err := v.Unmarshal(cfg); err != nil {
		return nil, err
	}
	return cfg, nil
}

// DBPath returns the path to the SQLite database file
func (c *Config) DBPath() string {
	return filepath.Join(c.DataDir, "mnemos.db")
}

// Validate checks that the configuration values are within acceptable ranges.
func (c *Config) Validate() error {
	qg := c.QualityGate
	if !qg.Enabled {
		return nil
	}
	if qg.MinWords <= 0 {
		return fmt.Errorf("quality_gate.min_words must be > 0, got %d", qg.MinWords)
	}
	if qg.MaxWords <= qg.MinWords {
		return fmt.Errorf("quality_gate.max_words (%d) must be > min_words (%d)", qg.MaxWords, qg.MinWords)
	}
	if qg.MinDensity <= 0 || qg.MinDensity >= 1 {
		return fmt.Errorf("quality_gate.min_density must be in (0,1), got %f", qg.MinDensity)
	}
	if qg.DuplicateThreshold <= 0 || qg.DuplicateThreshold >= 1 {
		return fmt.Errorf("quality_gate.duplicate_threshold must be in (0,1), got %f", qg.DuplicateThreshold)
	}
	sb := qg.ScoreBands
	if !(sb.Accept > sb.Fix && sb.Fix > sb.Downgrade && sb.Downgrade > 0) {
		return fmt.Errorf("quality_gate.score_bands must satisfy accept > fix > downgrade > 0, got accept=%f fix=%f downgrade=%f", sb.Accept, sb.Fix, sb.Downgrade)
	}
	for k, v := range qg.Penalties {
		if v < 0 || v > 1 {
			return fmt.Errorf("quality_gate.penalties[%s] must be in [0,1], got %f", k, v)
		}
	}
	return nil
}
