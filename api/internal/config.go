package lib

import (
	"fmt"
	"strings"

	"github.com/spf13/viper"
)

type Config struct {
	Env        Environment      `json:"env"`
	Server     ServerConfig     `mapstructure:"server"`
	Database   DatabaseConfig   `mapstructure:"database"`
	Ingest     IngestConfig     `mapstructure:"ingest"`
	Enrichment EnrichmentConfig `mapstructure:"enrichment"`
	Feed       FeedConfig       `mapstructure:"feed"`
}

type ServerConfig struct {
	Port string `mapstructure:"port"`
}

type DatabaseConfig struct {
	Host     string `mapstructure:"host"`
	Port     string `mapstructure:"port"`
	User     string `mapstructure:"user"`
	Password string `mapstructure:"password"`
	Name     string `mapstructure:"name"`
	SSLMode  string `mapstructure:"ssl_mode"`
}

type IngestConfig struct {
	SchedulerCron     string `mapstructure:"scheduler_cron"`
	RetryInterval     string `mapstructure:"retry_interval"`
	DefaultBatchLimit int    `mapstructure:"default_batch_limit"`
	BrokenThreshold   int    `mapstructure:"broken_threshold"`    // consecutive unreachable polls before Health = broken
	StaleThreshold    int    `mapstructure:"stale_threshold"`     // consecutive empty (reachable, 0 items) polls before Health = stale
	RetryBackoffBase  string `mapstructure:"retry_backoff_base"`  // exponential backoff base for both failure counters
	BrokenBackoffMax  string `mapstructure:"broken_backoff_max"`  // cap for the unreachable path only — stale uses each Source's own StaleAfter instead
	QueueBufferSize   int    `mapstructure:"queue_buffer_size"`
	QueueWorkers      int    `mapstructure:"queue_workers"`
}

type EnrichmentConfig struct {
	OllamaBaseURL    string `mapstructure:"ollama_base_url"`
	EmbeddingModel   string `mapstructure:"embedding_model"`
	WhisperBaseURL   string `mapstructure:"whisper_base_url"`
	QueueBufferSize  int    `mapstructure:"queue_buffer_size"`
	QueueWorkers     int    `mapstructure:"queue_workers"`
	RetryMaxAttempts int    `mapstructure:"retry_max_attempts"`
	RetryBackoffBase string `mapstructure:"retry_backoff_base"`
}

type FeedConfig struct {
	DefaultPageSize int `mapstructure:"default_page_size"`
	OverfetchFactor int `mapstructure:"overfetch_factor"`
}

func Load() (*Config, error) {

	v := viper.New()

	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.SetEnvPrefix("APP")
	v.AutomaticEnv()

	v.SetConfigName("base")
	v.SetConfigType("yaml")
	v.AddConfigPath("configs")

	if err := v.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("failed to read base config: %w", err)
	}

	env := Detect()

	v.SetConfigName(env.String())

	if err := v.MergeInConfig(); err != nil {
		fmt.Printf("Warning: environment config for '%s' not found, relying on base config\n", env)
	}

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	cfg.Env = env

	return &cfg, nil
}
