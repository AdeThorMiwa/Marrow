package lib

import (
	"fmt"
	"strings"

	"github.com/joho/godotenv"
	"github.com/spf13/viper"
)

type Config struct {
	Env        Environment      `json:"env"`
	Server     ServerConfig     `mapstructure:"server"`
	Database   DatabaseConfig   `mapstructure:"database"`
	Ingest     IngestConfig     `mapstructure:"ingest"`
	Enrichment EnrichmentConfig `mapstructure:"enrichment"`
	Feed       FeedConfig       `mapstructure:"feed"`
	Auth       AuthConfig       `mapstructure:"auth"`
	Twitter    TwitterConfig    `mapstructure:"twitter"`
	Instagram  InstagramConfig  `mapstructure:"instagram"`
	Redis      RedisConfig      `mapstructure:"redis"`
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
	BrokenThreshold   int    `mapstructure:"broken_threshold"`   // consecutive unreachable polls before Health = broken
	StaleThreshold    int    `mapstructure:"stale_threshold"`    // consecutive empty (reachable, 0 items) polls before Health = stale
	RetryBackoffBase  string `mapstructure:"retry_backoff_base"` // exponential backoff base for both failure counters
	BrokenBackoffMax  string `mapstructure:"broken_backoff_max"` // cap for the unreachable path only — stale uses each Source's own StaleAfter instead
	QueueWorkers      int    `mapstructure:"queue_workers"`
}

type EnrichmentConfig struct {
	OllamaBaseURL    string `mapstructure:"ollama_base_url"`
	EmbeddingModel   string `mapstructure:"embedding_model"`
	WhisperBaseURL   string `mapstructure:"whisper_base_url"`
	QueueWorkers     int    `mapstructure:"queue_workers"`
	RetryMaxAttempts int    `mapstructure:"retry_max_attempts"`
	RetryBackoffBase string `mapstructure:"retry_backoff_base"`
}

type FeedConfig struct {
	DefaultPageSize      int `mapstructure:"default_page_size"`
	OverfetchFactor      int `mapstructure:"overfetch_factor"`
	MaxSameSourcePerPage int `mapstructure:"max_same_source_per_page"`
}

// TwitterConfig holds the real account credentials the Twitter adapter
// authenticates as (no public, unauthenticated API exists for this
// platform — see adapter/impl/twitter.go). Deliberately left empty in
// base.yaml/every committed config file; set via APP_TWITTER_USERNAME /
// APP_TWITTER_COOKIES environment variables (or a local, untracked
// api/.env — see Load's godotenv call) instead, same as any other secret.
// Cookies is the raw `Cookie:` header string copied from a logged-in
// browser session (needs at least auth_token and ct0).
type TwitterConfig struct {
	Username string `mapstructure:"username"`
	Cookies  string `mapstructure:"cookies"`
	// AdditionalAccountsJSON: see docs/twitter-rate-limit-handling/design.md §2.
	AdditionalAccountsJSON string `mapstructure:"additional_accounts_json"`
}

// TwitterAccount is one entry of TwitterConfig.AdditionalAccountsJSON.
type TwitterAccount struct {
	Username string `json:"username"`
	Cookies  string `json:"cookies"`
}

// InstagramConfig is the Instagram counterpart to TwitterConfig — same
// deliberately-empty-in-committed-config, env-var-only rule, same "raw
// Cookie header string" shape for Cookies (see
// adapter/impl/instagram.go).
type InstagramConfig struct {
	Username string `mapstructure:"username"`
	Cookies  string `mapstructure:"cookies"`
}

// RedisConfig backs internal/queue's AsynqBroker — see
// docs/durable-queue/design.md.
type RedisConfig struct {
	Addr string `mapstructure:"addr"`
}

type AuthConfig struct {
	JWTSecret      string `mapstructure:"jwt_secret"`
	TokenIssuer    string `mapstructure:"token_issuer"`
	AccessTTL      string `mapstructure:"access_ttl"`
	RefreshTTL     string `mapstructure:"refresh_ttl"`
	BcryptCost     int    `mapstructure:"bcrypt_cost"`
	AllowedOrigin  string `mapstructure:"allowed_origin"`
	GoogleClientID string `mapstructure:"google_client_id"`

	// Not used for now, will be required if we want to use 'resources' like gmail or photos etc
	GoogleClientSecret string `mapstructure:"google_client_secret"`
}

func Load() (*Config, error) {
	// Best-effort: populates the process environment from a local,
	// untracked .env file (see api/.env) if one exists — lets secrets like
	// TwitterConfig's be set without ever touching a committed yaml file.
	// Silently no-ops when the file doesn't exist (e.g. in production,
	// where real env vars are set directly).
	_ = godotenv.Load()

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
