package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/geekxflood/program-director/internal/config"
	"github.com/geekxflood/program-director/internal/database/repository"
	"github.com/geekxflood/program-director/pkg/models"
)

// Response helpers
type errorResponse struct {
	Error   string `json:"error"`
	Message string `json:"message,omitempty"`
}

type successResponse struct {
	Success bool        `json:"success"`
	Data    interface{} `json:"data,omitempty"`
	Message string      `json:"message,omitempty"`
}

// writeJSON writes a JSON response
func writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

// writeError writes an error response
func writeError(w http.ResponseWriter, status int, err error, message string) {
	writeJSON(w, status, errorResponse{
		Error:   err.Error(),
		Message: message,
	})
}

// Health check handler
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, fmt.Errorf("method not allowed"), "")
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"status":    "healthy",
		"timestamp": time.Now().Format(time.RFC3339),
		"version":   "1.0.0",
	})
}

// Ready check handler
func (s *Server) handleReady(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, fmt.Errorf("method not allowed"), "")
		return
	}

	// Check database connectivity
	ctx := r.Context()
	_, err := s.mediaRepo.Count(ctx, repository.ListMediaOptions{Limit: 1})
	if err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]interface{}{
			"status":  "not ready",
			"message": "database not accessible",
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"status":    "ready",
		"timestamp": time.Now().Format(time.RFC3339),
	})
}

// Metrics handler (Prometheus format)
func (s *Server) handleMetrics(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, fmt.Errorf("method not allowed"), "")
		return
	}

	ctx := r.Context()

	// Get counts
	hasFile := true
	movieCount, _ := s.mediaRepo.Count(ctx, repository.ListMediaOptions{
		MediaType: models.MediaTypeMovie,
		HasFile:   &hasFile,
	})
	seriesCount, _ := s.mediaRepo.Count(ctx, repository.ListMediaOptions{
		MediaType: models.MediaTypeSeries,
		HasFile:   &hasFile,
	})
	animeCount, _ := s.mediaRepo.Count(ctx, repository.ListMediaOptions{
		MediaType: models.MediaTypeAnime,
		HasFile:   &hasFile,
	})
	historyCount, _ := s.historyRepo.Count(ctx, repository.ListHistoryOptions{})
	cooldownCount, _ := s.cooldownRepo.CountActive(ctx)

	w.Header().Set("Content-Type", "text/plain; version=0.0.4")
	fmt.Fprintf(w, "# HELP program_director_media_total Total number of media items by type\n")
	fmt.Fprintf(w, "# TYPE program_director_media_total gauge\n")
	fmt.Fprintf(w, "program_director_media_total{type=\"movie\"} %d\n", movieCount)
	fmt.Fprintf(w, "program_director_media_total{type=\"series\"} %d\n", seriesCount)
	fmt.Fprintf(w, "program_director_media_total{type=\"anime\"} %d\n", animeCount)
	fmt.Fprintf(w, "\n")

	fmt.Fprintf(w, "# HELP program_director_history_plays_total Total number of plays recorded\n")
	fmt.Fprintf(w, "# TYPE program_director_history_plays_total counter\n")
	fmt.Fprintf(w, "program_director_history_plays_total %d\n", historyCount)
	fmt.Fprintf(w, "\n")

	fmt.Fprintf(w, "# HELP program_director_cooldowns_active Number of media items on cooldown\n")
	fmt.Fprintf(w, "# TYPE program_director_cooldowns_active gauge\n")
	fmt.Fprintf(w, "program_director_cooldowns_active %d\n", cooldownCount)
	fmt.Fprintf(w, "\n")

	fmt.Fprintf(w, "# HELP program_director_themes_configured Number of configured themes\n")
	fmt.Fprintf(w, "# TYPE program_director_themes_configured gauge\n")
	fmt.Fprintf(w, "program_director_themes_configured %d\n", len(s.config.Themes))
}

// Media list handler
func (s *Server) handleMediaList(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, fmt.Errorf("method not allowed"), "")
		return
	}

	ctx := r.Context()

	// Parse query parameters
	mediaType := r.URL.Query().Get("type")
	hasFile := true

	opts := repository.ListMediaOptions{
		HasFile: &hasFile,
		Limit:   100,
	}

	if mediaType != "" {
		opts.MediaType = models.MediaType(mediaType)
	}

	media, err := s.mediaRepo.List(ctx, opts)
	if err != nil {
		s.logger.Error("failed to list media", "error", err)
		writeError(w, http.StatusInternalServerError, err, "failed to query media")
		return
	}

	writeJSON(w, http.StatusOK, successResponse{
		Success: true,
		Data: map[string]interface{}{
			"media": media,
			"count": len(media),
		},
	})
}

// Media sync handler
func (s *Server) handleMediaSync(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, fmt.Errorf("method not allowed"), "")
		return
	}

	ctx := r.Context()
	cleanup := r.URL.Query().Get("cleanup") == "true"

	s.logger.Info("media sync triggered via API", "cleanup", cleanup)

	// Sync movies
	movieResult, err := s.syncService.SyncMovies(ctx, cleanup)
	if err != nil {
		s.logger.Error("movie sync failed", "error", err)
		writeError(w, http.StatusInternalServerError, err, "movie sync failed")
		return
	}

	// Sync series
	seriesResult, err := s.syncService.SyncSeries(ctx, cleanup)
	if err != nil {
		s.logger.Error("series sync failed", "error", err)
		writeError(w, http.StatusInternalServerError, err, "series sync failed")
		return
	}

	writeJSON(w, http.StatusOK, successResponse{
		Success: true,
		Data: map[string]interface{}{
			"movies": map[string]interface{}{
				"created": movieResult.Created,
				"updated": movieResult.Updated,
				"deleted": movieResult.Deleted,
				"errors":  movieResult.Errors,
			},
			"series": map[string]interface{}{
				"created": seriesResult.Created,
				"updated": seriesResult.Updated,
				"deleted": seriesResult.Deleted,
				"errors":  seriesResult.Errors,
			},
		},
		Message: "sync completed successfully",
	})
}

// Themes list handler
func (s *Server) handleThemesList(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, fmt.Errorf("method not allowed"), "")
		return
	}

	writeJSON(w, http.StatusOK, successResponse{
		Success: true,
		Data: map[string]interface{}{
			"themes": s.config.Themes,
			"count":  len(s.config.Themes),
		},
	})
}

// Generate all playlists handler
func (s *Server) handleGenerateAll(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, fmt.Errorf("method not allowed"), "")
		return
	}

	ctx := r.Context()
	dryRun := r.URL.Query().Get("dry_run") == "true"

	s.logger.Info("generating all playlists via API", "dry_run", dryRun)

	results, err := s.playlistGenerator.GenerateAll(ctx, s.config.Themes, dryRun)
	if err != nil {
		s.logger.Error("playlist generation failed", "error", err)
		writeError(w, http.StatusInternalServerError, err, "generation failed")
		return
	}

	// Convert results to JSON-friendly format
	var resultData []map[string]interface{}
	for _, result := range results {
		data := map[string]interface{}{
			"theme":      result.ThemeName,
			"channel_id": result.ChannelID,
			"generated":  result.Generated,
			"item_count": result.ItemCount,
			"duration":   result.Duration.String(),
		}
		if result.Error != nil {
			data["error"] = result.Error.Error()
		}
		resultData = append(resultData, data)
	}

	writeJSON(w, http.StatusOK, successResponse{
		Success: true,
		Data: map[string]interface{}{
			"results": resultData,
			"count":   len(results),
		},
		Message: "playlist generation completed",
	})
}

// Generate specific theme handler
func (s *Server) handleGenerateTheme(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, fmt.Errorf("method not allowed"), "")
		return
	}

	// Extract theme name from path
	themeName := strings.TrimPrefix(r.URL.Path, "/api/v1/generate/")
	if themeName == "" {
		writeError(w, http.StatusBadRequest, fmt.Errorf("theme name required"), "")
		return
	}

	// Find theme
	var themeConfig *config.ThemeConfig
	for i := range s.config.Themes {
		if s.config.Themes[i].Name == themeName {
			themeConfig = &s.config.Themes[i]
			break
		}
	}

	if themeConfig == nil {
		writeError(w, http.StatusNotFound, fmt.Errorf("theme not found"), "")
		return
	}

	ctx := r.Context()
	dryRun := r.URL.Query().Get("dry_run") == "true"

	s.logger.Info("generating playlist via API",
		"theme", themeName,
		"dry_run", dryRun,
	)

	result := s.playlistGenerator.Generate(ctx, themeConfig, dryRun)

	data := map[string]interface{}{
		"theme":      result.ThemeName,
		"channel_id": result.ChannelID,
		"generated":  result.Generated,
		"item_count": result.ItemCount,
		"duration":   result.Duration.String(),
	}
	if result.Error != nil {
		data["error"] = result.Error.Error()
	}

	writeJSON(w, http.StatusOK, successResponse{
		Success: true,
		Data:    data,
		Message: "playlist generation completed",
	})
}

// History handler
func (s *Server) handleHistory(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, fmt.Errorf("method not allowed"), "")
		return
	}

	ctx := r.Context()

	history, err := s.historyRepo.List(ctx, repository.ListHistoryOptions{
		Limit: 100,
	})
	if err != nil {
		s.logger.Error("failed to list history", "error", err)
		writeError(w, http.StatusInternalServerError, err, "failed to query history")
		return
	}

	writeJSON(w, http.StatusOK, successResponse{
		Success: true,
		Data: map[string]interface{}{
			"history": history,
			"count":   len(history),
		},
	})
}

// Cooldowns handler
func (s *Server) handleCooldowns(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, fmt.Errorf("method not allowed"), "")
		return
	}

	ctx := r.Context()

	cooldowns, err := s.cooldownRepo.List(ctx, repository.ListCooldownOptions{
		ActiveOnly: true,
		Limit:      100,
	})
	if err != nil {
		s.logger.Error("failed to list cooldowns", "error", err)
		writeError(w, http.StatusInternalServerError, err, "failed to query cooldowns")
		return
	}

	writeJSON(w, http.StatusOK, successResponse{
		Success: true,
		Data: map[string]interface{}{
			"cooldowns": cooldowns,
			"count":     len(cooldowns),
		},
	})
}

// Webhooks handler
func (s *Server) handleWebhooks(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, fmt.Errorf("method not allowed"), "")
		return
	}

	// Parse webhook payload
	var payload map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeError(w, http.StatusBadRequest, err, "invalid JSON payload")
		return
	}

	s.logger.Info("webhook received", "payload", payload)

	// TODO: Implement webhook processing logic
	// For now, just acknowledge receipt

	writeJSON(w, http.StatusOK, successResponse{
		Success: true,
		Message: "webhook received",
	})
}
