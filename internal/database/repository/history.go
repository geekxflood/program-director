package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/geekxflood/program-director/internal/database"
	"github.com/geekxflood/program-director/pkg/models"
)

// HistoryRepository handles play history persistence
type HistoryRepository struct {
	db database.DB
}

// NewHistoryRepository creates a new HistoryRepository
func NewHistoryRepository(db database.DB) *HistoryRepository {
	return &HistoryRepository{db: db}
}

// Create inserts a new play history record
func (r *HistoryRepository) Create(ctx context.Context, h *models.PlayHistory) error {
	if h.PlayedAt.IsZero() {
		h.PlayedAt = time.Now()
	}

	query := `
		INSERT INTO play_history (
			media_id, channel_id, theme_name, played_at, media_title, media_type
		) VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id
	`

	err := r.db.QueryRow(ctx, query,
		h.MediaID, h.ChannelID, h.ThemeName, h.PlayedAt, h.MediaTitle, h.MediaType,
	).Scan(&h.ID)

	return err
}

// GetByID retrieves a play history record by ID
func (r *HistoryRepository) GetByID(ctx context.Context, id int64) (*models.PlayHistory, error) {
	query := `
		SELECT id, media_id, channel_id, theme_name, played_at, media_title, media_type
		FROM play_history WHERE id = $1
	`

	var h models.PlayHistory
	err := r.db.QueryRow(ctx, query, id).Scan(
		&h.ID, &h.MediaID, &h.ChannelID, &h.ThemeName, &h.PlayedAt, &h.MediaTitle, &h.MediaType,
	)
	if err != nil {
		return nil, err
	}
	return &h, nil
}

// List retrieves play history with optional filters
func (r *HistoryRepository) List(ctx context.Context, opts ListHistoryOptions) ([]models.PlayHistory, error) {
	query := `
		SELECT id, media_id, channel_id, theme_name, played_at, media_title, media_type
		FROM play_history WHERE 1=1
	`
	args := make([]interface{}, 0)
	argIndex := 1

	if opts.MediaID > 0 {
		query += fmt.Sprintf(" AND media_id = $%d", argIndex)
		args = append(args, opts.MediaID)
		argIndex++
	}

	if opts.ChannelID != "" {
		query += fmt.Sprintf(" AND channel_id = $%d", argIndex)
		args = append(args, opts.ChannelID)
		argIndex++
	}

	if opts.ThemeName != "" {
		query += fmt.Sprintf(" AND theme_name = $%d", argIndex)
		args = append(args, opts.ThemeName)
		argIndex++
	}

	if !opts.Since.IsZero() {
		query += fmt.Sprintf(" AND played_at >= $%d", argIndex)
		args = append(args, opts.Since)
		argIndex++
	}

	if !opts.Until.IsZero() {
		query += fmt.Sprintf(" AND played_at <= $%d", argIndex)
		args = append(args, opts.Until)
		argIndex++
	}

	// Order by played_at descending by default
	query += " ORDER BY played_at DESC"

	if opts.Limit > 0 {
		query += fmt.Sprintf(" LIMIT $%d", argIndex)
		args = append(args, opts.Limit)
		argIndex++
	}

	if opts.Offset > 0 {
		query += fmt.Sprintf(" OFFSET $%d", argIndex)
		args = append(args, opts.Offset)
	}

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var history []models.PlayHistory
	for rows.Next() {
		var h models.PlayHistory
		err := rows.Scan(
			&h.ID, &h.MediaID, &h.ChannelID, &h.ThemeName, &h.PlayedAt, &h.MediaTitle, &h.MediaType,
		)
		if err != nil {
			return nil, err
		}
		history = append(history, h)
	}

	return history, rows.Err()
}

// GetLastPlayForMedia retrieves the most recent play for a specific media
func (r *HistoryRepository) GetLastPlayForMedia(ctx context.Context, mediaID int64) (*models.PlayHistory, error) {
	query := `
		SELECT id, media_id, channel_id, theme_name, played_at, media_title, media_type
		FROM play_history
		WHERE media_id = $1
		ORDER BY played_at DESC
		LIMIT 1
	`

	var h models.PlayHistory
	err := r.db.QueryRow(ctx, query, mediaID).Scan(
		&h.ID, &h.MediaID, &h.ChannelID, &h.ThemeName, &h.PlayedAt, &h.MediaTitle, &h.MediaType,
	)
	if err != nil {
		return nil, err
	}
	return &h, nil
}

// GetPlayCount returns the number of times a media has been played
func (r *HistoryRepository) GetPlayCount(ctx context.Context, mediaID int64) (int64, error) {
	var count int64
	err := r.db.QueryRow(ctx,
		"SELECT COUNT(*) FROM play_history WHERE media_id = $1",
		mediaID,
	).Scan(&count)
	return count, err
}

// GetRecentlyPlayedMediaIDs returns IDs of media played since the given time
func (r *HistoryRepository) GetRecentlyPlayedMediaIDs(ctx context.Context, since time.Time) ([]int64, error) {
	rows, err := r.db.Query(ctx,
		"SELECT DISTINCT media_id FROM play_history WHERE played_at >= $1",
		since,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var ids []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// Count returns the total number of play history records
func (r *HistoryRepository) Count(ctx context.Context, opts ListHistoryOptions) (int64, error) {
	query := "SELECT COUNT(*) FROM play_history WHERE 1=1"
	args := make([]interface{}, 0)
	argIndex := 1

	if opts.MediaID > 0 {
		query += fmt.Sprintf(" AND media_id = $%d", argIndex)
		args = append(args, opts.MediaID)
		argIndex++
	}

	if opts.ChannelID != "" {
		query += fmt.Sprintf(" AND channel_id = $%d", argIndex)
		args = append(args, opts.ChannelID)
		argIndex++
	}

	if opts.ThemeName != "" {
		query += fmt.Sprintf(" AND theme_name = $%d", argIndex)
		args = append(args, opts.ThemeName)
		argIndex++
	}

	if !opts.Since.IsZero() {
		query += fmt.Sprintf(" AND played_at >= $%d", argIndex)
		args = append(args, opts.Since)
		argIndex++
	}

	var count int64
	err := r.db.QueryRow(ctx, query, args...).Scan(&count)
	return count, err
}

// Delete removes a play history record
func (r *HistoryRepository) Delete(ctx context.Context, id int64) error {
	_, err := r.db.Exec(ctx, "DELETE FROM play_history WHERE id = $1", id)
	return err
}

// DeleteOlderThan removes play history records older than the given time
func (r *HistoryRepository) DeleteOlderThan(ctx context.Context, before time.Time) (int64, error) {
	result, err := r.db.Exec(ctx,
		"DELETE FROM play_history WHERE played_at < $1",
		before,
	)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

// ListHistoryOptions provides filtering options for List
type ListHistoryOptions struct {
	MediaID   int64
	ChannelID string
	ThemeName string
	Since     time.Time
	Until     time.Time
	Limit     int
	Offset    int
}
