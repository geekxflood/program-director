package cooldown

import (
	"context"
	"log/slog"
	"time"

	"github.com/geekxflood/program-director/internal/config"
	"github.com/geekxflood/program-director/internal/database/repository"
	"github.com/geekxflood/program-director/pkg/models"
)

// Manager handles media cooldown tracking
type Manager struct {
	cooldownRepo *repository.CooldownRepository
	historyRepo  *repository.HistoryRepository
	config       *config.CooldownConfig
	logger       *slog.Logger
}

// NewManager creates a new cooldown Manager
func NewManager(
	cooldownRepo *repository.CooldownRepository,
	historyRepo *repository.HistoryRepository,
	cfg *config.CooldownConfig,
	logger *slog.Logger,
) *Manager {
	return &Manager{
		cooldownRepo: cooldownRepo,
		historyRepo:  historyRepo,
		config:       cfg,
		logger:       logger,
	}
}

// RecordPlay records that a media item was played and sets its cooldown
func (m *Manager) RecordPlay(ctx context.Context, media *models.Media, channelID, themeName string) error {
	now := time.Now()

	// Create play history record
	history := &models.PlayHistory{
		MediaID:    media.ID,
		ChannelID:  channelID,
		ThemeName:  themeName,
		PlayedAt:   now,
		MediaTitle: media.Title,
		MediaType:  media.MediaType,
	}

	if err := m.historyRepo.Create(ctx, history); err != nil {
		return err
	}

	// Determine cooldown days based on media type
	cooldownDays := m.getCooldownDays(media.MediaType)

	// Create or update cooldown
	cooldown := &models.MediaCooldown{
		MediaID:      media.ID,
		CooldownDays: cooldownDays,
		LastPlayedAt: now,
		CanReplayAt:  now.AddDate(0, 0, cooldownDays),
		MediaTitle:   media.Title,
		MediaType:    media.MediaType,
	}

	if err := m.cooldownRepo.Upsert(ctx, cooldown); err != nil {
		return err
	}

	m.logger.Debug("recorded play and cooldown",
		"media_id", media.ID,
		"title", media.Title,
		"cooldown_days", cooldownDays,
		"can_replay_at", cooldown.CanReplayAt,
	)

	return nil
}

// IsOnCooldown checks if a media item is currently on cooldown
func (m *Manager) IsOnCooldown(ctx context.Context, mediaID int64) (bool, error) {
	return m.cooldownRepo.IsOnCooldown(ctx, mediaID)
}

// GetCooldown retrieves the cooldown info for a media item
func (m *Manager) GetCooldown(ctx context.Context, mediaID int64) (*models.MediaCooldown, error) {
	return m.cooldownRepo.GetByMediaID(ctx, mediaID)
}

// GetActiveCooldownMediaIDs returns IDs of all media currently on cooldown
func (m *Manager) GetActiveCooldownMediaIDs(ctx context.Context) ([]int64, error) {
	return m.cooldownRepo.GetActiveCooldownMediaIDs(ctx)
}

// CleanupExpired removes all expired cooldowns
func (m *Manager) CleanupExpired(ctx context.Context) (int64, error) {
	count, err := m.cooldownRepo.DeleteExpired(ctx)
	if err != nil {
		return 0, err
	}

	if count > 0 {
		m.logger.Info("cleaned up expired cooldowns", "count", count)
	}

	return count, nil
}

// GetPlayHistory retrieves play history with filters
func (m *Manager) GetPlayHistory(ctx context.Context, opts repository.ListHistoryOptions) ([]models.PlayHistory, error) {
	return m.historyRepo.List(ctx, opts)
}

// GetActiveCooldowns retrieves all active cooldowns
func (m *Manager) GetActiveCooldowns(ctx context.Context) ([]models.MediaCooldown, error) {
	return m.cooldownRepo.List(ctx, repository.ListCooldownOptions{
		ActiveOnly: true,
	})
}

// GetStats returns cooldown statistics
func (m *Manager) GetStats(ctx context.Context) (*CooldownStats, error) {
	activeCount, err := m.cooldownRepo.Count(ctx, repository.ListCooldownOptions{
		ActiveOnly: true,
	})
	if err != nil {
		return nil, err
	}

	expiredCount, err := m.cooldownRepo.Count(ctx, repository.ListCooldownOptions{
		ExpiredOnly: true,
	})
	if err != nil {
		return nil, err
	}

	totalPlays, err := m.historyRepo.Count(ctx, repository.ListHistoryOptions{})
	if err != nil {
		return nil, err
	}

	// Count plays in last 24 hours
	recentPlays, err := m.historyRepo.Count(ctx, repository.ListHistoryOptions{
		Since: time.Now().Add(-24 * time.Hour),
	})
	if err != nil {
		return nil, err
	}

	return &CooldownStats{
		ActiveCooldowns:  activeCount,
		ExpiredCooldowns: expiredCount,
		TotalPlays:       totalPlays,
		RecentPlays:      recentPlays,
	}, nil
}

// getCooldownDays returns the cooldown days for a media type
func (m *Manager) getCooldownDays(mediaType models.MediaType) int {
	switch mediaType {
	case models.MediaTypeMovie:
		return m.config.MovieDays
	case models.MediaTypeSeries:
		return m.config.SeriesDays
	case models.MediaTypeAnime:
		return m.config.AnimeDays
	default:
		return m.config.MovieDays
	}
}

// CooldownStats contains cooldown statistics
type CooldownStats struct {
	ActiveCooldowns  int64
	ExpiredCooldowns int64
	TotalPlays       int64
	RecentPlays      int64
}
