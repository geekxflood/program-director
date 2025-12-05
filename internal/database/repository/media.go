package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/geekxflood/program-director/internal/database"
	"github.com/geekxflood/program-director/pkg/models"
)

// MediaRepository handles media persistence
type MediaRepository struct {
	db database.DB
}

// NewMediaRepository creates a new MediaRepository
func NewMediaRepository(db database.DB) *MediaRepository {
	return &MediaRepository{db: db}
}

// Create inserts a new media record
func (r *MediaRepository) Create(ctx context.Context, m *models.Media) error {
	now := time.Now()
	m.CreatedAt = now
	m.UpdatedAt = now
	m.SyncedAt = now

	query := `
		INSERT INTO media (
			external_id, source, media_type, title, year, overview, runtime,
			genres, imdb_rating, tmdb_rating, popularity,
			imdb_id, tmdb_id, tvdb_id, path, has_file, size_on_disk,
			status, monitored, synced_at, created_at, updated_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7,
			$8, $9, $10, $11,
			$12, $13, $14, $15, $16, $17,
			$18, $19, $20, $21, $22
		) RETURNING id
	`

	genresValue, err := m.Genres.Value()
	if err != nil {
		return fmt.Errorf("failed to marshal genres: %w", err)
	}

	err = r.db.QueryRow(ctx, query,
		m.ExternalID, m.Source, m.MediaType, m.Title, m.Year, m.Overview, m.Runtime,
		genresValue, m.IMDBRating, m.TMDBRating, m.Popularity,
		m.IMDBID, m.TMDBID, m.TVDBID, m.Path, m.HasFile, m.SizeOnDisk,
		m.Status, m.Monitored, m.SyncedAt, m.CreatedAt, m.UpdatedAt,
	).Scan(&m.ID)

	return err
}

// Upsert creates or updates a media record based on external_id and source
func (r *MediaRepository) Upsert(ctx context.Context, m *models.Media) error {
	now := time.Now()
	m.UpdatedAt = now
	m.SyncedAt = now

	query := `
		INSERT INTO media (
			external_id, source, media_type, title, year, overview, runtime,
			genres, imdb_rating, tmdb_rating, popularity,
			imdb_id, tmdb_id, tvdb_id, path, has_file, size_on_disk,
			status, monitored, synced_at, created_at, updated_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7,
			$8, $9, $10, $11,
			$12, $13, $14, $15, $16, $17,
			$18, $19, $20, $21, $22
		)
		ON CONFLICT (external_id, source) DO UPDATE SET
			media_type = EXCLUDED.media_type,
			title = EXCLUDED.title,
			year = EXCLUDED.year,
			overview = EXCLUDED.overview,
			runtime = EXCLUDED.runtime,
			genres = EXCLUDED.genres,
			imdb_rating = EXCLUDED.imdb_rating,
			tmdb_rating = EXCLUDED.tmdb_rating,
			popularity = EXCLUDED.popularity,
			imdb_id = EXCLUDED.imdb_id,
			tmdb_id = EXCLUDED.tmdb_id,
			tvdb_id = EXCLUDED.tvdb_id,
			path = EXCLUDED.path,
			has_file = EXCLUDED.has_file,
			size_on_disk = EXCLUDED.size_on_disk,
			status = EXCLUDED.status,
			monitored = EXCLUDED.monitored,
			synced_at = EXCLUDED.synced_at,
			updated_at = EXCLUDED.updated_at
		RETURNING id, created_at
	`

	genresValue, err := m.Genres.Value()
	if err != nil {
		return fmt.Errorf("failed to marshal genres: %w", err)
	}

	err = r.db.QueryRow(ctx, query,
		m.ExternalID, m.Source, m.MediaType, m.Title, m.Year, m.Overview, m.Runtime,
		genresValue, m.IMDBRating, m.TMDBRating, m.Popularity,
		m.IMDBID, m.TMDBID, m.TVDBID, m.Path, m.HasFile, m.SizeOnDisk,
		m.Status, m.Monitored, m.SyncedAt, now, now,
	).Scan(&m.ID, &m.CreatedAt)

	return err
}

// GetByID retrieves a media record by ID
func (r *MediaRepository) GetByID(ctx context.Context, id int64) (*models.Media, error) {
	query := `
		SELECT id, external_id, source, media_type, title, year, overview, runtime,
			genres, imdb_rating, tmdb_rating, popularity,
			imdb_id, tmdb_id, tvdb_id, path, has_file, size_on_disk,
			status, monitored, synced_at, created_at, updated_at
		FROM media WHERE id = $1
	`

	var m models.Media
	err := r.db.QueryRow(ctx, query, id).Scan(
		&m.ID, &m.ExternalID, &m.Source, &m.MediaType, &m.Title, &m.Year, &m.Overview, &m.Runtime,
		&m.Genres, &m.IMDBRating, &m.TMDBRating, &m.Popularity,
		&m.IMDBID, &m.TMDBID, &m.TVDBID, &m.Path, &m.HasFile, &m.SizeOnDisk,
		&m.Status, &m.Monitored, &m.SyncedAt, &m.CreatedAt, &m.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &m, nil
}

// GetByExternalID retrieves a media record by external ID and source
func (r *MediaRepository) GetByExternalID(ctx context.Context, externalID int64, source models.MediaSource) (*models.Media, error) {
	query := `
		SELECT id, external_id, source, media_type, title, year, overview, runtime,
			genres, imdb_rating, tmdb_rating, popularity,
			imdb_id, tmdb_id, tvdb_id, path, has_file, size_on_disk,
			status, monitored, synced_at, created_at, updated_at
		FROM media WHERE external_id = $1 AND source = $2
	`

	var m models.Media
	err := r.db.QueryRow(ctx, query, externalID, source).Scan(
		&m.ID, &m.ExternalID, &m.Source, &m.MediaType, &m.Title, &m.Year, &m.Overview, &m.Runtime,
		&m.Genres, &m.IMDBRating, &m.TMDBRating, &m.Popularity,
		&m.IMDBID, &m.TMDBID, &m.TVDBID, &m.Path, &m.HasFile, &m.SizeOnDisk,
		&m.Status, &m.Monitored, &m.SyncedAt, &m.CreatedAt, &m.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &m, nil
}

// List retrieves media with optional filters
func (r *MediaRepository) List(ctx context.Context, opts ListMediaOptions) ([]models.Media, error) {
	query := `
		SELECT id, external_id, source, media_type, title, year, overview, runtime,
			genres, imdb_rating, tmdb_rating, popularity,
			imdb_id, tmdb_id, tvdb_id, path, has_file, size_on_disk,
			status, monitored, synced_at, created_at, updated_at
		FROM media WHERE 1=1
	`
	args := make([]interface{}, 0)
	argIndex := 1

	if opts.Source != "" {
		query += fmt.Sprintf(" AND source = $%d", argIndex)
		args = append(args, opts.Source)
		argIndex++
	}

	if opts.MediaType != "" {
		query += fmt.Sprintf(" AND media_type = $%d", argIndex)
		args = append(args, opts.MediaType)
		argIndex++
	}

	if opts.HasFile != nil {
		query += fmt.Sprintf(" AND has_file = $%d", argIndex)
		args = append(args, *opts.HasFile)
		argIndex++
	}

	if opts.MinRating > 0 {
		query += fmt.Sprintf(" AND imdb_rating >= $%d", argIndex)
		args = append(args, opts.MinRating)
		argIndex++
	}

	// Order by
	if opts.OrderBy != "" {
		query += " ORDER BY " + opts.OrderBy
	} else {
		query += " ORDER BY title"
	}

	// Limit
	if opts.Limit > 0 {
		query += fmt.Sprintf(" LIMIT $%d", argIndex)
		args = append(args, opts.Limit)
		argIndex++
	}

	// Offset
	if opts.Offset > 0 {
		query += fmt.Sprintf(" OFFSET $%d", argIndex)
		args = append(args, opts.Offset)
	}

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var media []models.Media
	for rows.Next() {
		var m models.Media
		err := rows.Scan(
			&m.ID, &m.ExternalID, &m.Source, &m.MediaType, &m.Title, &m.Year, &m.Overview, &m.Runtime,
			&m.Genres, &m.IMDBRating, &m.TMDBRating, &m.Popularity,
			&m.IMDBID, &m.TMDBID, &m.TVDBID, &m.Path, &m.HasFile, &m.SizeOnDisk,
			&m.Status, &m.Monitored, &m.SyncedAt, &m.CreatedAt, &m.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}
		media = append(media, m)
	}

	return media, rows.Err()
}

// ListByGenres retrieves media that has any of the specified genres
func (r *MediaRepository) ListByGenres(ctx context.Context, genres []string, mediaType models.MediaType, excludeIDs []int64) ([]models.Media, error) {
	// Build genre condition
	genreConditions := ""
	args := make([]interface{}, 0)
	argIndex := 1

	for i, genre := range genres {
		if i > 0 {
			genreConditions += " OR "
		}
		genreConditions += fmt.Sprintf("genres LIKE $%d", argIndex)
		args = append(args, "%"+genre+"%")
		argIndex++
	}

	query := fmt.Sprintf(`
		SELECT id, external_id, source, media_type, title, year, overview, runtime,
			genres, imdb_rating, tmdb_rating, popularity,
			imdb_id, tmdb_id, tvdb_id, path, has_file, size_on_disk,
			status, monitored, synced_at, created_at, updated_at
		FROM media
		WHERE has_file = true AND (%s)
	`, genreConditions)

	if mediaType != "" {
		query += fmt.Sprintf(" AND media_type = $%d", argIndex)
		args = append(args, mediaType)
		argIndex++
	}

	// Exclude specific IDs (e.g., already on cooldown)
	if len(excludeIDs) > 0 {
		query += " AND id NOT IN ("
		for i, id := range excludeIDs {
			if i > 0 {
				query += ","
			}
			query += fmt.Sprintf("$%d", argIndex)
			args = append(args, id)
			argIndex++
		}
		query += ")"
	}

	query += " ORDER BY imdb_rating DESC, popularity DESC LIMIT 100"

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var media []models.Media
	for rows.Next() {
		var m models.Media
		err := rows.Scan(
			&m.ID, &m.ExternalID, &m.Source, &m.MediaType, &m.Title, &m.Year, &m.Overview, &m.Runtime,
			&m.Genres, &m.IMDBRating, &m.TMDBRating, &m.Popularity,
			&m.IMDBID, &m.TMDBID, &m.TVDBID, &m.Path, &m.HasFile, &m.SizeOnDisk,
			&m.Status, &m.Monitored, &m.SyncedAt, &m.CreatedAt, &m.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}
		media = append(media, m)
	}

	return media, rows.Err()
}

// Count returns the total number of media records
func (r *MediaRepository) Count(ctx context.Context, opts ListMediaOptions) (int64, error) {
	query := "SELECT COUNT(*) FROM media WHERE 1=1"
	args := make([]interface{}, 0)
	argIndex := 1

	if opts.Source != "" {
		query += fmt.Sprintf(" AND source = $%d", argIndex)
		args = append(args, opts.Source)
		argIndex++
	}

	if opts.MediaType != "" {
		query += fmt.Sprintf(" AND media_type = $%d", argIndex)
		args = append(args, opts.MediaType)
		argIndex++
	}

	if opts.HasFile != nil {
		query += fmt.Sprintf(" AND has_file = $%d", argIndex)
		args = append(args, *opts.HasFile)
	}

	var count int64
	err := r.db.QueryRow(ctx, query, args...).Scan(&count)
	return count, err
}

// Delete removes a media record
func (r *MediaRepository) Delete(ctx context.Context, id int64) error {
	_, err := r.db.Exec(ctx, "DELETE FROM media WHERE id = $1", id)
	return err
}

// DeleteStale removes media that hasn't been synced since the given time
func (r *MediaRepository) DeleteStale(ctx context.Context, source models.MediaSource, beforeTime time.Time) (int64, error) {
	result, err := r.db.Exec(ctx,
		"DELETE FROM media WHERE source = $1 AND synced_at < $2",
		source, beforeTime,
	)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

// ListMediaOptions provides filtering options for List
type ListMediaOptions struct {
	Source    models.MediaSource
	MediaType models.MediaType
	HasFile   *bool
	MinRating float64
	OrderBy   string
	Limit     int
	Offset    int
}
