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
	"marrow/internal/database"
	"marrow/internal/pubsub"
	"marrow/internal/queue"
	"marrow/internal/scheduler"
	"marrow/internal/tasks"
	"marrow/internal/workers"

	"github.com/gin-gonic/gin"
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

	// Twitter is opt-in (unlike Ollama/whisper.cpp, which this app always
	// needs) — only registered, and only asserts twscrape is installed, once
	// real credentials are actually configured. See lib.TwitterConfig.
	if c.Twitter.Username != "" && c.Twitter.Cookies != "" {
		registry.Register(adapter.NewTwitterAdapter(c.Twitter))
	}

	ingestQueue := queue.NewInMemory[workers.IngestJobPayload](queue.InMemoryOptions[workers.IngestJobPayload]{
		BufferSize:   c.Ingest.QueueBufferSize,
		DefaultRetry: queue.NoRetry[workers.IngestJobPayload](),
	})

	ingestWorker := workers.NewIngestWorker(ingestQueue)
	ingestWorker.Start(ctx, appCtx, c.Ingest.QueueWorkers)

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

	if err := startEnrichment(ctx, appCtx, c); err != nil {
		return fmt.Errorf("failed to start enrichment: %w", err)
	}

	ginEngine := gin.Default()
	if err := ginEngine.SetTrustedProxies([]string{}); err != nil {
		log.Fatalf("Failed to set trusted proxies: %v", err)
	}

	AttachRoutes(ginEngine, appCtx)

	return ginEngine.Run(":" + c.Server.Port)
}

// startEnrichment wires the Enrichment worker, its queue, and its
// ContentIngested trigger. Unlike Ingest (which self-heals via the next
// scheduler tick), Enrichment has no natural re-trigger, so its queue's
// retry policy carries an OnExhausted hook — see EnrichmentWorker.OnExhausted.
func startEnrichment(ctx context.Context, appCtx *app.Context, c *lib.Config) error {
	backoffBase, err := time.ParseDuration(c.Enrichment.RetryBackoffBase)
	if err != nil {
		return fmt.Errorf("invalid enrichment.retry_backoff_base: %w", err)
	}

	enrichmentQueue := queue.NewInMemory[workers.EnrichmentJobPayload](queue.InMemoryOptions[workers.EnrichmentJobPayload]{
		BufferSize: c.Enrichment.QueueBufferSize,
	})

	enrichmentWorker := workers.NewEnrichmentWorker(
		enrichmentQueue,
		adapter.NewTranscriber(c.Enrichment.WhisperBaseURL),
		adapter.NewOllamaEmbedder(c.Enrichment.OllamaBaseURL),
		api.EmbeddingModel(c.Enrichment.EmbeddingModel),
	)
	enrichmentWorker.Start(ctx, appCtx, c.Enrichment.QueueWorkers)

	retry := queue.RetryPolicy[workers.EnrichmentJobPayload]{
		MaxAttempts: c.Enrichment.RetryMaxAttempts,
		Backoff:     queue.ExponentialBackoff(backoffBase),
		OnExhausted: enrichmentWorker.OnExhausted,
	}
	workers.RegisterEnrichmentTrigger(appCtx, enrichmentQueue, retry)

	return nil
}
