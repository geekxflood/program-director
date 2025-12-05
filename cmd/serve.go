package cmd

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"

	"github.com/geekxflood/program-director/internal/clients/ollama"
	"github.com/geekxflood/program-director/internal/clients/radarr"
	"github.com/geekxflood/program-director/internal/clients/sonarr"
	"github.com/geekxflood/program-director/internal/clients/tunarr"
	"github.com/geekxflood/program-director/internal/database"
	"github.com/geekxflood/program-director/internal/database/repository"
	"github.com/geekxflood/program-director/internal/server"
	"github.com/geekxflood/program-director/internal/services/cooldown"
	"github.com/geekxflood/program-director/internal/services/media"
	"github.com/geekxflood/program-director/internal/services/playlist"
	"github.com/geekxflood/program-director/internal/services/similarity"
)

var (
	servePort            int
	serveEnableScheduler bool
	serveMetricsEnabled  bool
)

// serveCmd represents the serve command
var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Start HTTP server mode",
	Long: `Start the program-director HTTP server.

This runs program-director as a long-running service with an HTTP API
for triggering playlist generation, viewing status, and receiving webhooks.

Optionally enables a built-in scheduler for automatic playlist generation.

Examples:
  # Start server on default port 8080
  program-director serve

  # Start server on custom port
  program-director serve --port 9000

  # Start server with built-in scheduler
  program-director serve --enable-scheduler

  # Disable prometheus metrics
  program-director serve --metrics=false`,
	RunE: runServe,
}

func init() {
	serveCmd.Flags().IntVarP(&servePort, "port", "p", 8080, "HTTP server port")
	serveCmd.Flags().BoolVar(&serveEnableScheduler, "enable-scheduler", false, "enable built-in cron scheduler")
	serveCmd.Flags().BoolVar(&serveMetricsEnabled, "metrics", true, "enable prometheus metrics endpoint")
}

func runServe(cmd *cobra.Command, args []string) error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigChan
		logger.Info("received shutdown signal, starting graceful shutdown")
		cancel()
	}()

	logger.Info("starting HTTP server",
		"port", servePort,
		"scheduler", serveEnableScheduler,
		"metrics", serveMetricsEnabled,
	)

	logger.Debug("initializing database connection")

	// Initialize database
	db, err := database.New(ctx, &cfg.Database, logger)
	if err != nil {
		logger.Error("failed to initialize database", "error", err)
		return fmt.Errorf("failed to initialize database: %w", err)
	}
	defer func() {
		if err := db.Close(); err != nil {
			logger.Error("failed to close database", "error", err)
		}
	}()

	// Run migrations
	logger.Debug("running database migrations")
	if err := db.Migrate(ctx); err != nil {
		return fmt.Errorf("failed to run migrations: %w", err)
	}

	logger.Debug("initializing repositories")

	// Initialize repositories
	mediaRepo := repository.NewMediaRepository(db)
	historyRepo := repository.NewHistoryRepository(db)
	cooldownRepo := repository.NewCooldownRepository(db)

	logger.Debug("initializing API clients",
		"radarr_url", cfg.Radarr.URL,
		"sonarr_url", cfg.Sonarr.URL,
		"tunarr_url", cfg.Tunarr.URL,
		"ollama_url", cfg.Ollama.URL,
	)

	// Initialize API clients
	radarrClient := radarr.New(&cfg.Radarr)
	sonarrClient := sonarr.New(&cfg.Sonarr)
	tunarrClient := tunarr.New(&cfg.Tunarr)
	ollamaClient := ollama.New(&cfg.Ollama)

	logger.Debug("initializing services")

	// Initialize services
	syncService := media.NewSyncService(radarrClient, sonarrClient, mediaRepo, logger)
	cooldownManager := cooldown.NewManager(cooldownRepo, historyRepo, &cfg.Cooldown, logger)
	similarityScorer := similarity.NewScorer(mediaRepo, ollamaClient, logger)
	playlistGenerator := playlist.NewGenerator(tunarrClient, similarityScorer, cooldownManager, logger)

	logger.Debug("initializing HTTP server")

	// Create HTTP server
	serverCfg := &server.Config{
		Port:           servePort,
		MetricsEnabled: serveMetricsEnabled,
	}

	httpServer := server.NewServer(
		cfg,
		serverCfg,
		mediaRepo,
		historyRepo,
		cooldownRepo,
		syncService,
		playlistGenerator,
		cooldownManager,
		logger,
	)

	// Print server info
	fmt.Printf("\nServer starting on http://0.0.0.0:%d\n", servePort)
	fmt.Println()
	fmt.Println("Endpoints:")
	fmt.Println("  GET  /health              - Health check")
	fmt.Println("  GET  /ready               - Readiness check")
	if serveMetricsEnabled {
		fmt.Println("  GET  /metrics             - Prometheus metrics")
	}
	fmt.Println("  GET  /api/v1/media        - List media")
	fmt.Println("  POST /api/v1/media/sync   - Trigger sync")
	fmt.Println("  GET  /api/v1/themes       - List themes")
	fmt.Println("  POST /api/v1/generate     - Generate all playlists")
	fmt.Println("  POST /api/v1/generate/:id - Generate specific theme")
	fmt.Println("  GET  /api/v1/history      - Play history")
	fmt.Println("  GET  /api/v1/cooldowns    - Current cooldowns")
	fmt.Println("  POST /api/v1/webhooks     - Webhook triggers")
	fmt.Println()

	if serveEnableScheduler {
		logger.Info("scheduler enabled",
			"themes", len(cfg.Themes),
		)
		// TODO: Initialize and start scheduler
		logger.Warn("scheduler not yet implemented")
	}

	// Start HTTP server (blocking)
	if err := httpServer.Start(ctx, servePort); err != nil {
		return fmt.Errorf("server error: %w", err)
	}

	logger.Info("server shutdown complete")
	return nil
}
