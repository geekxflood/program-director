package playlist

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/geekxflood/program-director/internal/clients/tunarr"
	"github.com/geekxflood/program-director/internal/config"
	"github.com/geekxflood/program-director/internal/services/cooldown"
	"github.com/geekxflood/program-director/internal/services/similarity"
	"github.com/geekxflood/program-director/pkg/models"
)

// Generator handles playlist generation and Tunarr integration
type Generator struct {
	tunarr   *tunarr.Client
	scorer   *similarity.Scorer
	cooldown *cooldown.Manager
	logger   *slog.Logger
}

// NewGenerator creates a new playlist Generator
func NewGenerator(
	tunarrClient *tunarr.Client,
	scorer *similarity.Scorer,
	cooldownManager *cooldown.Manager,
	logger *slog.Logger,
) *Generator {
	return &Generator{
		tunarr:   tunarrClient,
		scorer:   scorer,
		cooldown: cooldownManager,
		logger:   logger,
	}
}

// GenerationResult contains the results of a playlist generation
type GenerationResult struct {
	ThemeName   string
	ChannelID   string
	Generated   bool
	ItemCount   int
	TotalScore  float64
	Duration    time.Duration
	Error       error
	Playlist    *models.Playlist
}

// GenerateAll generates playlists for all themes
func (g *Generator) GenerateAll(ctx context.Context, themes []config.ThemeConfig, dryRun bool) ([]GenerationResult, error) {
	var results []GenerationResult

	for _, theme := range themes {
		select {
		case <-ctx.Done():
			return results, ctx.Err()
		default:
		}

		result := g.Generate(ctx, &theme, dryRun)
		results = append(results, result)
	}

	return results, nil
}

// Generate creates a playlist for a single theme
func (g *Generator) Generate(ctx context.Context, theme *config.ThemeConfig, dryRun bool) GenerationResult {
	start := time.Now()
	result := GenerationResult{
		ThemeName: theme.Name,
		ChannelID: theme.ChannelID,
	}

	g.logger.Info("generating playlist",
		"theme", theme.Name,
		"channel", theme.ChannelID,
		"dry_run", dryRun,
	)

	// Get media on cooldown
	excludeIDs, err := g.cooldown.GetActiveCooldownMediaIDs(ctx)
	if err != nil {
		g.logger.Warn("failed to get cooldown IDs", "error", err)
		excludeIDs = nil
	}

	g.logger.Debug("excluding media on cooldown", "count", len(excludeIDs))

	// Find matching candidates
	candidates, err := g.scorer.FindCandidates(ctx, theme, excludeIDs)
	if err != nil {
		result.Error = fmt.Errorf("failed to find candidates: %w", err)
		result.Duration = time.Since(start)
		return result
	}

	if len(candidates) == 0 {
		g.logger.Warn("no candidates found for theme", "theme", theme.Name)
		result.Duration = time.Since(start)
		return result
	}

	g.logger.Info("found candidates",
		"theme", theme.Name,
		"count", len(candidates),
	)

	// Build playlist
	playlist := &models.Playlist{
		ThemeName:   theme.Name,
		ChannelID:   theme.ChannelID,
		GeneratedAt: time.Now(),
		Items:       candidates,
	}

	// Calculate totals
	var totalScore float64
	var totalDuration int
	for _, c := range candidates {
		totalScore += c.Score
		totalDuration += c.Runtime
	}
	playlist.TotalScore = totalScore
	playlist.Duration = totalDuration

	result.Playlist = playlist
	result.ItemCount = len(candidates)
	result.TotalScore = totalScore

	// Log playlist
	g.logger.Info("playlist generated",
		"theme", theme.Name,
		"items", len(candidates),
		"total_score", fmt.Sprintf("%.2f", totalScore),
		"duration_mins", totalDuration,
	)

	for i, c := range candidates {
		g.logger.Debug("playlist item",
			"position", i+1,
			"title", c.Title,
			"year", c.Year,
			"score", fmt.Sprintf("%.2f", c.Score),
			"reason", c.MatchReason,
		)
	}

	// Apply to Tunarr if not dry run
	if !dryRun {
		if err := g.applyToTunarr(ctx, theme.ChannelID, candidates); err != nil {
			result.Error = fmt.Errorf("failed to apply to Tunarr: %w", err)
		} else {
			result.Generated = true

			// Record plays and cooldowns
			for _, c := range candidates {
				if err := g.cooldown.RecordPlay(ctx, &c.Media, theme.ChannelID, theme.Name); err != nil {
					g.logger.Warn("failed to record play",
						"media_id", c.ID,
						"title", c.Title,
						"error", err,
					)
				}
			}
		}
	} else {
		result.Generated = true // Mark as successful for dry run
	}

	result.Duration = time.Since(start)
	return result
}

// applyToTunarr updates the Tunarr channel with the generated playlist
func (g *Generator) applyToTunarr(ctx context.Context, channelID string, items []models.MediaWithScore) error {
	// First, get channel info to verify it exists
	channel, err := g.tunarr.GetChannel(ctx, channelID)
	if err != nil {
		return fmt.Errorf("failed to get channel %s: %w", channelID, err)
	}

	g.logger.Debug("updating Tunarr channel",
		"channel_id", channelID,
		"channel_name", channel.Name,
	)

	// Get media sources to find the Plex source
	sources, err := g.tunarr.GetMediaSources(ctx)
	if err != nil {
		return fmt.Errorf("failed to get media sources: %w", err)
	}

	var plexSourceID string
	for _, source := range sources {
		if source.Type == "plex" {
			plexSourceID = source.ID
			break
		}
	}

	if plexSourceID == "" {
		return fmt.Errorf("no Plex media source found in Tunarr")
	}

	// Build programming lineup
	var programs []tunarr.Program
	for _, item := range items {
		// Convert runtime to milliseconds
		durationMs := int64(item.Runtime) * 60 * 1000

		program := tunarr.Program{
			Type:               "content",
			Duration:           durationMs,
			ExternalSourceType: "plex",
			ExternalSourceName: "Plex",
			// Note: We'd need the Plex rating key here
			// For now, use file path as a fallback identifier
			PlexFilePath: item.Path,
			Title:        item.Title,
			Year:         item.Year,
		}
		programs = append(programs, program)
	}

	// Create programming object
	programming := &tunarr.Programming{
		Type:     "manual",
		Programs: programs,
	}

	// Apply to Tunarr
	if err := g.tunarr.SetProgramming(ctx, channelID, programming); err != nil {
		return err
	}

	g.logger.Info("Tunarr channel updated",
		"channel_id", channelID,
		"programs", len(programs),
	)

	return nil
}

// ValidateChannel checks if a channel exists in Tunarr
func (g *Generator) ValidateChannel(ctx context.Context, channelID string) error {
	_, err := g.tunarr.GetChannel(ctx, channelID)
	return err
}

// GetChannels retrieves all Tunarr channels
func (g *Generator) GetChannels(ctx context.Context) ([]tunarr.Channel, error) {
	return g.tunarr.GetChannels(ctx)
}
