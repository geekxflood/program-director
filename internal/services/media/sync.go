package media

import (
	"context"
	"log/slog"
	"time"

	"github.com/geekxflood/program-director/internal/clients/radarr"
	"github.com/geekxflood/program-director/internal/clients/sonarr"
	"github.com/geekxflood/program-director/internal/database/repository"
	"github.com/geekxflood/program-director/pkg/models"
)

// SyncService handles media synchronization from Radarr/Sonarr
type SyncService struct {
	radarr    *radarr.Client
	sonarr    *sonarr.Client
	mediaRepo *repository.MediaRepository
	logger    *slog.Logger
}

// NewSyncService creates a new SyncService
func NewSyncService(
	radarrClient *radarr.Client,
	sonarrClient *sonarr.Client,
	mediaRepo *repository.MediaRepository,
	logger *slog.Logger,
) *SyncService {
	return &SyncService{
		radarr:    radarrClient,
		sonarr:    sonarrClient,
		mediaRepo: mediaRepo,
		logger:    logger,
	}
}

// SyncResult contains the results of a sync operation
type SyncResult struct {
	Source    models.MediaSource
	Created   int
	Updated   int
	Deleted   int
	Errors    int
	Duration  time.Duration
}

// SyncAll synchronizes all media from both Radarr and Sonarr
func (s *SyncService) SyncAll(ctx context.Context, cleanup bool) ([]SyncResult, error) {
	var results []SyncResult

	// Sync movies
	movieResult, err := s.SyncMovies(ctx, cleanup)
	if err != nil {
		s.logger.Error("failed to sync movies", "error", err)
	} else {
		results = append(results, *movieResult)
	}

	// Sync series
	seriesResult, err := s.SyncSeries(ctx, cleanup)
	if err != nil {
		s.logger.Error("failed to sync series", "error", err)
	} else {
		results = append(results, *seriesResult)
	}

	return results, nil
}

// SyncMovies synchronizes movies from Radarr
func (s *SyncService) SyncMovies(ctx context.Context, cleanup bool) (*SyncResult, error) {
	start := time.Now()
	result := &SyncResult{
		Source: models.MediaSourceRadarr,
	}

	s.logger.Info("starting movie sync")

	// Fetch all movies from Radarr
	movies, err := s.radarr.GetMovies(ctx)
	if err != nil {
		return nil, err
	}

	s.logger.Info("fetched movies from Radarr", "count", len(movies))

	syncTime := time.Now()

	for _, movie := range movies {
		select {
		case <-ctx.Done():
			return result, ctx.Err()
		default:
		}

		media := movie.ToMedia()
		media.SyncedAt = syncTime

		// Check if exists
		existing, err := s.mediaRepo.GetByExternalID(ctx, media.ExternalID, media.Source)
		if err != nil {
			// Doesn't exist, create
			if err := s.mediaRepo.Upsert(ctx, media); err != nil {
				s.logger.Error("failed to create movie",
					"title", media.Title,
					"error", err,
				)
				result.Errors++
				continue
			}
			result.Created++
		} else {
			// Exists, update
			media.ID = existing.ID
			media.CreatedAt = existing.CreatedAt
			if err := s.mediaRepo.Upsert(ctx, media); err != nil {
				s.logger.Error("failed to update movie",
					"title", media.Title,
					"error", err,
				)
				result.Errors++
				continue
			}
			result.Updated++
		}
	}

	// Cleanup stale entries
	if cleanup {
		deleted, err := s.mediaRepo.DeleteStale(ctx, models.MediaSourceRadarr, syncTime.Add(-time.Minute))
		if err != nil {
			s.logger.Error("failed to cleanup stale movies", "error", err)
		} else {
			result.Deleted = int(deleted)
		}
	}

	result.Duration = time.Since(start)
	s.logger.Info("movie sync complete",
		"created", result.Created,
		"updated", result.Updated,
		"deleted", result.Deleted,
		"errors", result.Errors,
		"duration", result.Duration,
	)

	return result, nil
}

// SyncSeries synchronizes series from Sonarr
func (s *SyncService) SyncSeries(ctx context.Context, cleanup bool) (*SyncResult, error) {
	start := time.Now()
	result := &SyncResult{
		Source: models.MediaSourceSonarr,
	}

	s.logger.Info("starting series sync")

	// Fetch all series from Sonarr
	series, err := s.sonarr.GetSeries(ctx)
	if err != nil {
		return nil, err
	}

	s.logger.Info("fetched series from Sonarr", "count", len(series))

	syncTime := time.Now()

	for _, show := range series {
		select {
		case <-ctx.Done():
			return result, ctx.Err()
		default:
		}

		media := show.ToMedia()
		media.SyncedAt = syncTime

		// Check if exists
		existing, err := s.mediaRepo.GetByExternalID(ctx, media.ExternalID, media.Source)
		if err != nil {
			// Doesn't exist, create
			if err := s.mediaRepo.Upsert(ctx, media); err != nil {
				s.logger.Error("failed to create series",
					"title", media.Title,
					"error", err,
				)
				result.Errors++
				continue
			}
			result.Created++
		} else {
			// Exists, update
			media.ID = existing.ID
			media.CreatedAt = existing.CreatedAt
			if err := s.mediaRepo.Upsert(ctx, media); err != nil {
				s.logger.Error("failed to update series",
					"title", media.Title,
					"error", err,
				)
				result.Errors++
				continue
			}
			result.Updated++
		}
	}

	// Cleanup stale entries
	if cleanup {
		deleted, err := s.mediaRepo.DeleteStale(ctx, models.MediaSourceSonarr, syncTime.Add(-time.Minute))
		if err != nil {
			s.logger.Error("failed to cleanup stale series", "error", err)
		} else {
			result.Deleted = int(deleted)
		}
	}

	result.Duration = time.Since(start)
	s.logger.Info("series sync complete",
		"created", result.Created,
		"updated", result.Updated,
		"deleted", result.Deleted,
		"errors", result.Errors,
		"duration", result.Duration,
	)

	return result, nil
}

// GetStats returns media statistics
func (s *SyncService) GetStats(ctx context.Context) (*MediaStats, error) {
	hasFile := true

	movieCount, err := s.mediaRepo.Count(ctx, repository.ListMediaOptions{
		Source:  models.MediaSourceRadarr,
		HasFile: &hasFile,
	})
	if err != nil {
		return nil, err
	}

	seriesCount, err := s.mediaRepo.Count(ctx, repository.ListMediaOptions{
		Source:    models.MediaSourceSonarr,
		MediaType: models.MediaTypeSeries,
		HasFile:   &hasFile,
	})
	if err != nil {
		return nil, err
	}

	animeCount, err := s.mediaRepo.Count(ctx, repository.ListMediaOptions{
		Source:    models.MediaSourceSonarr,
		MediaType: models.MediaTypeAnime,
		HasFile:   &hasFile,
	})
	if err != nil {
		return nil, err
	}

	return &MediaStats{
		Movies:   movieCount,
		Series:   seriesCount,
		Anime:    animeCount,
		Total:    movieCount + seriesCount + animeCount,
	}, nil
}

// MediaStats contains media catalog statistics
type MediaStats struct {
	Movies int64
	Series int64
	Anime  int64
	Total  int64
}
