package main

import (
	"context"
	"fmt"
	"log"
	"time"

	lib "marrow/internal"
	api "marrow/internal/adapter/api"
	adapter "marrow/internal/adapter/impl"
	"marrow/internal/adapter/registry"
	"marrow/internal/app"
	"marrow/internal/auth"
	"marrow/internal/auth/google"
	"marrow/internal/database"
	"marrow/internal/database/dbo"
	"marrow/internal/pubsub"
	"marrow/internal/queue"
	"marrow/internal/scheduler"
	"marrow/internal/tasks"
	"marrow/internal/workers"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/spf13/cobra"
)

func serveCommand(cfg *lib.Config) *cobra.Command {
	return &cobra.Command{
		Use:   "serve",
		Short: "Start the Marrow API server",
		RunE: func(cmd *cobra.Command, args []string) error {
			return serve(cfg)
		},
	}
}

func serve(c *lib.Config) error {
	gin.SetMode(c.Env.ToGinMode())

	ctx := context.Background()

	pool, err := database.Connect(ctx, c.Database)
	if err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}
	defer pool.Close()

	appCtx := &app.Context{Pool: pool, Bus: pubsub.New(), Config: c}
	defer appCtx.Bus.Shutdown()

	authCfg, err := buildAuthComponents(c, pool)
	if err != nil {
		return fmt.Errorf("failed to build auth components: %w", err)
	}

	appCtx.Auth = authCfg

	// Twitter and Instagram are opt-in (unlike Ollama/whisper.cpp, which this
	// app always needs) — only registered, and only asserted installed, once
	// real credentials are actually configured. See lib.TwitterConfig /
	// lib.InstagramConfig.
	if c.Twitter.Username != "" && c.Twitter.Cookies != "" {
		registry.Register(adapter.NewTwitterAdapter(c.Twitter))
	}
	if c.Instagram.Username != "" && c.Instagram.Cookies != "" {
		registry.Register(adapter.NewInstagramAdapter(c.Instagram))
	}

	ingestQueue := queue.NewAsynqBroker[workers.IngestJobPayload](
		c.Redis.Addr, "ingest", c.Ingest.QueueWorkers, queue.NoRetry[workers.IngestJobPayload](),
	)
	defer ingestQueue.Shutdown(ctx)

	ingestWorker := workers.NewIngestWorker()
	if err := ingestQueue.Start(ctx, appCtx, ingestWorker.ProcessJob); err != nil {
		return fmt.Errorf("failed to start ingest queue: %w", err)
	}

	discoveryTask, err := tasks.NewIngestDiscoveryTask(appCtx, ingestQueue, c.Ingest)
	if err != nil {
		return fmt.Errorf("failed to construct ingest discovery task: %w", err)
	}

	sched := scheduler.New()
	if err := sched.Schedule(discoveryTask); err != nil {
		return fmt.Errorf("failed to schedule ingest discovery task: %w", err)
	}
	sched.Start()
	defer sched.Stop()

	enrichmentQueue, err := startEnrichment(ctx, appCtx, c)
	if err != nil {
		return fmt.Errorf("failed to start enrichment: %w", err)
	}
	defer enrichmentQueue.Shutdown(ctx)

	ginEngine := gin.Default()
	if err := ginEngine.SetTrustedProxies([]string{}); err != nil {
		log.Fatalf("Failed to set trusted proxies: %v", err)
	}

	AttachRoutes(ginEngine, appCtx)

	return ginEngine.Run(":" + c.Server.Port)
}

func buildAuthComponents(c *lib.Config, pool *pgxpool.Pool) (api.AuthComponents, error) {
	accessTTL, err := time.ParseDuration(c.Auth.AccessTTL)
	if err != nil {
		return api.AuthComponents{}, fmt.Errorf("invalid auth.access_ttl: %w", err)
	}

	jwt, err := auth.NewJWTManager(c.Auth.JWTSecret, c.Auth.TokenIssuer, accessTTL)
	if err != nil {
		return api.AuthComponents{}, err
	}

	hasher, err := auth.NewPasswordHasher(c.Auth.BcryptCost)
	if err != nil {
		return api.AuthComponents{}, err
	}

	tokenStore := dbo.NewRefreshTokenStore(pool)
	refreshTTL, err := time.ParseDuration(c.Auth.RefreshTTL)
	if err != nil {
		return api.AuthComponents{}, fmt.Errorf("invalid auth.refresh_ttl: %w", err)
	}
	refreshTokens := auth.NewRefreshTokenService(refreshTTL, tokenStore)

	oauth := auth.NewOAuthRegistry()
	if c.Auth.GoogleClientID != "" {
		oauth.Register(google.NewProvider(c.Auth.GoogleClientID))
	}

	return api.AuthComponents{
		JWTManager:     jwt,
		PasswordHasher: hasher,
		OAuth:          oauth,
		RefreshTokens:  refreshTokens,
	}, nil
}

// startEnrichment wires the Enrichment worker, its queue, and its
// ContentIngested trigger. Unlike Ingest (which self-heals via the next
// scheduler tick), Enrichment has no natural re-trigger from a live event
// alone, so its queue's retry policy carries an OnExhausted hook (see
// EnrichmentWorker.OnExhausted) and every boot also reconciles any Content
// already in the DB with no matching EnrichedContent row — see
// docs/durable-queue/design.md Requirement 2. Returns the broker so the
// caller can Shutdown it on process exit.
func startEnrichment(ctx context.Context, appCtx *app.Context, c *lib.Config) (*queue.AsynqBroker[workers.EnrichmentJobPayload], error) {
	backoffBase, err := time.ParseDuration(c.Enrichment.RetryBackoffBase)
	if err != nil {
		return nil, fmt.Errorf("invalid enrichment.retry_backoff_base: %w", err)
	}

	enrichmentWorker := workers.NewEnrichmentWorker(
		adapter.NewTranscriber(c.Enrichment.WhisperBaseURL),
		adapter.NewOllamaEmbedder(c.Enrichment.OllamaBaseURL),
		api.EmbeddingModel(c.Enrichment.EmbeddingModel),
	)

	retry := queue.RetryPolicy[workers.EnrichmentJobPayload]{
		MaxAttempts: c.Enrichment.RetryMaxAttempts,
		Backoff:     queue.ExponentialBackoff(backoffBase),
		OnExhausted: enrichmentWorker.OnExhausted,
	}
	enrichmentQueue := queue.NewAsynqBroker[workers.EnrichmentJobPayload](
		c.Redis.Addr, "enrichment", c.Enrichment.QueueWorkers, retry,
	)
	if err := enrichmentQueue.Start(ctx, appCtx, enrichmentWorker.ProcessJob); err != nil {
		return nil, fmt.Errorf("failed to start enrichment queue: %w", err)
	}

	workers.RegisterEnrichmentTrigger(appCtx, enrichmentQueue)

	reconciled, err := workers.ReconcileEnrichment(ctx, appCtx, enrichmentQueue)
	if err != nil {
		return nil, fmt.Errorf("failed to reconcile unenriched content: %w", err)
	}
	if reconciled > 0 {
		log.Printf("enrichment: reconciled %d unenriched content row(s) on startup", reconciled)
	}

	return enrichmentQueue, nil
}
