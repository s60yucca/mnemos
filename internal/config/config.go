package config

import (
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/viper"
)

// Config holds all application configuration
type Config struct {
	DataDir   string          `mapstructure:"data_dir"`
	LogLevel  string          `mapstructure:"log_level"`
	LogFormat string          `mapstructure:"log_format"`
	Embeddings EmbeddingConfig `mapstructure:"embeddings"`
	Mirror    MirrorConfig    `mapstructure:"mirror"`
	Lifecycle LifecycleConfig `mapstructure:"lifecycle"`
	Dedup     DedupConfig     `mapstructure:"dedup"`
	Server    ServerConfig    `mapstructure:"server"`
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
	DecayInterval   time.Duration `mapstructure:"decay_interval"`
	GCRetentionDays int           `mapstructure:"gc_retention_days"`
	ArchiveThreshold float64      `mapstructure:"archive_threshold"`
}

type DedupConfig struct {
	FuzzyThreshold    float64 `mapstructure:"fuzzy_threshold"`
	SemanticThreshold float64 `mapstructure:"semantic_threshold"`
}

type ServerConfig struct {
	Port    int    `mapstructure:"port"`
	MCPMode string `mapstructure:"mcp_mode"` // "stdio", "sse"
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
