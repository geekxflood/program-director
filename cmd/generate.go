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
	)

	// TODO: Initialize services and generate playlists
	// This will be implemented in Phase 3-4

	if allThemes {
		logger.Info("generating all themes", "count", len(cfg.Themes))
		for _, theme := range cfg.Themes {
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
				logger.Info("generating playlist", "theme", theme.Name)
				// generator.GenerateAndApply(ctx, theme)
			}
		}
	} else {
		// Find the specific theme
		var found bool
		for _, theme := range cfg.Themes {
			if theme.Name == themeName {
				found = true
				logger.Info("generating playlist", "theme", theme.Name)
				// generator.GenerateAndApply(ctx, theme)
				break
			}
		}
		if !found {
			return fmt.Errorf("theme %q not found in configuration", themeName)
		}
	}

	logger.Info("playlist generation complete")
	return nil
}
