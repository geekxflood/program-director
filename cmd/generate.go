package cmd

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"

	"github.com/geekxflood/program-director/internal/clients/ollama"
	"github.com/geekxflood/program-director/internal/clients/tunarr"
	"github.com/geekxflood/program-director/internal/database"
	"github.com/geekxflood/program-director/internal/database/repository"
	"github.com/geekxflood/program-director/internal/services/cooldown"
	"github.com/geekxflood/program-director/internal/services/playlist"
	"github.com/geekxflood/program-director/internal/services/similarity"
)

var (
	themeName string
	allThemes bool
	dryRun    bool
)

// generateCmd represents the generate command
var generateCmd = &cobra.Command{
	Use:   "generate",
	Short: "Generate themed playlists",
	Long: `Generate themed playlists using AI and apply them to Tunarr.

Examples:
  # Generate a specific theme
  program-director generate --theme sci-fi-night

  # Generate all configured themes
  program-director generate --all-themes

  # Preview without applying
  program-director generate --theme horror-night --dry-run`,
	RunE: runGenerate,
}

func init() {
	generateCmd.Flags().StringVarP(&themeName, "theme", "t", "", "theme name to generate")
	generateCmd.Flags().BoolVarP(&allThemes, "all-themes", "a", false, "generate all configured themes")
	generateCmd.Flags().BoolVarP(&dryRun, "dry-run", "n", false, "preview without applying to Tunarr")
}

func runGenerate(cmd *cobra.Command, args []string) error {
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

	if !allThemes && themeName == "" {
		return fmt.Errorf("specify --theme or --all-themes")
	}

	if allThemes && themeName != "" {
		return fmt.Errorf("cannot use both --theme and --all-themes")
	}

	logger.Info("starting playlist generation",
		"all_themes", allThemes,
		"theme", themeName,
		"dry_run", dryRun,
		"config_file", cfgFile,
	)

	// Initialize services
	logger.Debug("initializing services")
	services, cleanup, err := initializeServices(ctx)
	if err != nil {
		logger.Error("service initialization failed", "error", err)
		return fmt.Errorf("failed to initialize services: %w", err)
	}
	defer cleanup()
	logger.Debug("services initialized successfully")

	if allThemes {
		logger.Info("generating all themes", "count", len(cfg.Themes))

		results, err := services.generator.GenerateAll(ctx, cfg.Themes, dryRun)
		if err != nil {
			logger.Error("generation error", "error", err)
			return fmt.Errorf("generation error: %w", err)
		}

		// Report results with summary
		var successful, failed int
		for _, result := range results {
			if result.Error != nil {
				failed++
				logger.Error("theme generation failed",
					"theme", result.ThemeName,
					"channel_id", result.ChannelID,
					"error", result.Error,
					"duration", result.Duration,
				)
			} else {
				successful++
				logger.Info("theme generation completed",
					"theme", result.ThemeName,
					"channel_id", result.ChannelID,
					"items", result.ItemCount,
					"total_score", fmt.Sprintf("%.2f", result.TotalScore),
					"duration", result.Duration,
					"generated", result.Generated,
				)
			}
		}

		logger.Info("all themes processed",
			"total", len(results),
			"successful", successful,
			"failed", failed,
		)
	} else {
		// Find the specific theme
		logger.Debug("searching for theme", "name", themeName)
		var found bool
		for _, theme := range cfg.Themes {
			if theme.Name == themeName {
				found = true
				logger.Debug("theme found",
					"name", theme.Name,
					"channel_id", theme.ChannelID,
					"max_items", theme.MaxItems,
					"duration", theme.Duration,
				)

				result := services.generator.Generate(ctx, &theme, dryRun)

				if result.Error != nil {
					logger.Error("generation failed",
						"theme", theme.Name,
						"error", result.Error,
						"duration", result.Duration,
					)
					return fmt.Errorf("generation failed for theme %s: %w", theme.Name, result.Error)
				}

				logger.Info("theme generation completed",
					"theme", result.ThemeName,
					"channel_id", result.ChannelID,
					"items", result.ItemCount,
					"total_score", fmt.Sprintf("%.2f", result.TotalScore),
					"duration", result.Duration,
					"generated", result.Generated,
				)
				break
			}
		}
		if !found {
			logger.Error("theme not found", "theme", themeName, "available_themes", len(cfg.Themes))
			return fmt.Errorf("theme %q not found in configuration", themeName)
		}
	}

	logger.Info("playlist generation complete")
	return nil
}

// services holds initialized service instances
type services struct {
	db        database.DB
	generator *playlist.Generator
}

// initializeServices sets up all required services
func initializeServices(ctx context.Context) (*services, func(), error) {
	logger.Debug("initializing database",
		"driver", cfg.Database.Driver,
		"postgres_host", cfg.Database.Postgres.Host,
		"sqlite_path", cfg.Database.SQLite.Path,
	)

	// Initialize database
	db, err := database.New(ctx, &cfg.Database, logger)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to initialize database: %w", err)
	}
	logger.Debug("database connection established")

	// Run migrations
	logger.Debug("running database migrations")
	if err := db.Migrate(ctx); err != nil {
		return nil, nil, fmt.Errorf("failed to run migrations: %w", err)
	}
	logger.Debug("migrations completed")

	// Initialize repositories
	logger.Debug("initializing repositories")
	mediaRepo := repository.NewMediaRepository(db)
	historyRepo := repository.NewHistoryRepository(db)
	cooldownRepo := repository.NewCooldownRepository(db)
	logger.Debug("repositories initialized")

	// Initialize Tunarr client
	logger.Debug("initializing tunarr client", "url", cfg.Tunarr.URL)
	tunarrClient := tunarr.New(&cfg.Tunarr)

	// Initialize Ollama client
	logger.Debug("initializing ollama client",
		"url", cfg.Ollama.URL,
		"model", cfg.Ollama.Model,
		"temperature", cfg.Ollama.Temperature,
	)
	ollamaClient := ollama.New(&cfg.Ollama)

	// Initialize similarity scorer
	logger.Debug("initializing similarity scorer")
	scorer := similarity.NewScorer(mediaRepo, ollamaClient, logger)

	// Initialize cooldown manager
	logger.Debug("initializing cooldown manager",
		"movie_days", cfg.Cooldown.MovieDays,
		"series_days", cfg.Cooldown.SeriesDays,
		"anime_days", cfg.Cooldown.AnimeDays,
	)
	cooldownManager := cooldown.NewManager(cooldownRepo, historyRepo, &cfg.Cooldown, logger)

	// Initialize playlist generator
	logger.Debug("initializing playlist generator")
	generator := playlist.NewGenerator(tunarrClient, scorer, cooldownManager, logger)

	cleanup := func() {
		logger.Debug("cleaning up resources")
		if err := db.Close(); err != nil {
			logger.Error("failed to close database", "error", err)
		}
		logger.Debug("cleanup complete")
	}

	return &services{
		db:        db,
		generator: generator,
	}, cleanup, nil
}
