package cmd

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"

	"github.com/geekxflood/program-director/internal/clients/radarr"
	"github.com/geekxflood/program-director/internal/clients/sonarr"
	"github.com/geekxflood/program-director/internal/database"
	"github.com/geekxflood/program-director/internal/database/repository"
	"github.com/geekxflood/program-director/internal/services/media"
)

var (
	syncMovies  bool
	syncSeries  bool
	syncCleanup bool
)

// syncCmd represents the sync command
var syncCmd = &cobra.Command{
	Use:   "sync",
	Short: "Sync media catalog from Radarr/Sonarr",
	Long: `Synchronize the local media catalog with Radarr and Sonarr.

This command fetches all media metadata from your media management
applications and stores it in the local database for fast querying
during playlist generation.

Examples:
  # Sync all media (movies and series)
  program-director sync

  # Sync only movies
  program-director sync --movies

  # Sync only series (TV shows and anime)
  program-director sync --series

  # Sync and cleanup removed media
  program-director sync --cleanup`,
	RunE: runSync,
}

func init() {
	syncCmd.Flags().BoolVar(&syncMovies, "movies", false, "sync only movies from Radarr")
	syncCmd.Flags().BoolVar(&syncSeries, "series", false, "sync only series from Sonarr")
	syncCmd.Flags().BoolVar(&syncCleanup, "cleanup", false, "remove media no longer in source")
}

func runSync(cmd *cobra.Command, args []string) error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigChan
		logger.Info("received shutdown signal")
		cancel()
	}()

	// Default to syncing everything if no specific flags
	syncAll := !syncMovies && !syncSeries
	if syncAll {
		syncMovies = true
		syncSeries = true
	}

	logger.Info("starting media sync",
		"movies", syncMovies,
		"series", syncSeries,
		"cleanup", syncCleanup,
		"radarr_url", cfg.Radarr.URL,
		"sonarr_url", cfg.Sonarr.URL,
	)

	logger.Debug("initializing sync services")

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

	// Initialize repository
	mediaRepo := repository.NewMediaRepository(db)

	// Initialize API clients
	radarrClient := radarr.New(&cfg.Radarr)
	sonarrClient := sonarr.New(&cfg.Sonarr)

	// Create sync service
	syncService := media.NewSyncService(radarrClient, sonarrClient, mediaRepo, logger)

	var results []media.SyncResult

	if syncMovies {
		logger.Info("syncing movies from Radarr",
			"url", cfg.Radarr.URL,
		)
		result, err := syncService.SyncMovies(ctx, syncCleanup)
		if err != nil {
			logger.Error("movie sync failed", "error", err)
			return fmt.Errorf("movie sync failed: %w", err)
		}
		results = append(results, *result)
	}

	if syncSeries {
		logger.Info("syncing series from Sonarr",
			"url", cfg.Sonarr.URL,
		)
		result, err := syncService.SyncSeries(ctx, syncCleanup)
		if err != nil {
			logger.Error("series sync failed", "error", err)
			return fmt.Errorf("series sync failed: %w", err)
		}
		results = append(results, *result)
	}

	// Calculate totals
	totalCreated := 0
	totalUpdated := 0
	totalDeleted := 0
	totalErrors := 0

	for _, result := range results {
		totalCreated += result.Created
		totalUpdated += result.Updated
		totalDeleted += result.Deleted
		totalErrors += result.Errors
	}

	logger.Info("media sync complete",
		"created", totalCreated,
		"updated", totalUpdated,
		"deleted", totalDeleted,
		"errors", totalErrors,
	)

	// Display summary
	fmt.Println()
	fmt.Println("Sync Summary")
	fmt.Println("============")
	for _, result := range results {
		fmt.Printf("\n%s:\n", result.Source)
		fmt.Printf("  Created:  %d\n", result.Created)
		fmt.Printf("  Updated:  %d\n", result.Updated)
		if syncCleanup {
			fmt.Printf("  Deleted:  %d\n", result.Deleted)
		}
		if result.Errors > 0 {
			fmt.Printf("  Errors:   %d\n", result.Errors)
		}
		fmt.Printf("  Duration: %s\n", result.Duration)
	}
	fmt.Println()

	return nil
}
