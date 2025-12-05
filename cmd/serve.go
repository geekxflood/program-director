package cmd

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"
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

	// TODO: Initialize all services
	// - Database connection
	// - API clients (Radarr, Sonarr, Tunarr, Ollama)
	// - Services (sync, similarity, cooldown, playlist)
	// - HTTP handlers
	// - Prometheus metrics
	// - Optional scheduler

	// Placeholder: print server info
	fmt.Printf("Server starting on http://0.0.0.0:%d\n", servePort)
	fmt.Println()
	fmt.Println("Endpoints:")
	fmt.Println("  GET  /health              - Health check")
	fmt.Println("  GET  /metrics             - Prometheus metrics")
	fmt.Println("  GET  /api/v1/media        - List media")
	fmt.Println("  POST /api/v1/media/sync   - Trigger sync")
	fmt.Println("  GET  /api/v1/themes       - List themes")
	fmt.Println("  POST /api/v1/generate     - Generate all playlists")
	fmt.Println("  POST /api/v1/generate/:id - Generate specific theme")
	fmt.Println("  GET  /api/v1/history      - Play history")
	fmt.Println("  GET  /api/v1/cooldowns    - Current cooldowns")
	fmt.Println("  POST /api/v1/webhooks     - Webhook triggers")

	if serveEnableScheduler {
		logger.Info("scheduler enabled",
			"themes", len(cfg.Themes),
		)
		// scheduler.Start()
	}

	// TODO: Start HTTP server
	// server.ListenAndServe(ctx)

	// Block until context cancelled
	<-ctx.Done()

	logger.Info("server shutdown complete")
	return nil
}
