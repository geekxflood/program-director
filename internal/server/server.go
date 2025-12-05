package server

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"time"

	"github.com/geekxflood/program-director/internal/config"
	"github.com/geekxflood/program-director/internal/database/repository"
	"github.com/geekxflood/program-director/internal/services/cooldown"
	"github.com/geekxflood/program-director/internal/services/media"
	"github.com/geekxflood/program-director/internal/services/playlist"
)

// Server represents the HTTP server
type Server struct {
	config            *config.Config
	logger            *slog.Logger
	httpServer        *http.Server
	mediaRepo         *repository.MediaRepository
	historyRepo       *repository.HistoryRepository
	cooldownRepo      *repository.CooldownRepository
	syncService       *media.SyncService
	playlistGenerator *playlist.Generator
	cooldownManager   *cooldown.Manager
	metricsEnabled    bool
}

// Config holds server configuration
type Config struct {
	Port           int
	MetricsEnabled bool
}

// NewServer creates a new HTTP server instance
func NewServer(
	cfg *config.Config,
	serverCfg *Config,
	mediaRepo *repository.MediaRepository,
	historyRepo *repository.HistoryRepository,
	cooldownRepo *repository.CooldownRepository,
	syncService *media.SyncService,
	playlistGenerator *playlist.Generator,
	cooldownManager *cooldown.Manager,
	logger *slog.Logger,
) *Server {
	return &Server{
		config:            cfg,
		logger:            logger,
		mediaRepo:         mediaRepo,
		historyRepo:       historyRepo,
		cooldownRepo:      cooldownRepo,
		syncService:       syncService,
		playlistGenerator: playlistGenerator,
		cooldownManager:   cooldownManager,
		metricsEnabled:    serverCfg.MetricsEnabled,
	}
}

// Start starts the HTTP server
func (s *Server) Start(ctx context.Context, port int) error {
	mux := http.NewServeMux()

	// Register handlers
	s.registerHandlers(mux)

	addr := fmt.Sprintf("0.0.0.0:%d", port)

	s.httpServer = &http.Server{
		Addr:         addr,
		Handler:      mux,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
		BaseContext:  func(net.Listener) context.Context { return ctx },
	}

	s.logger.Info("HTTP server starting", "address", addr)

	// Start server in goroutine
	errChan := make(chan error, 1)
	go func() {
		if err := s.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errChan <- err
		}
	}()

	// Wait for context cancellation or server error
	select {
	case err := <-errChan:
		return fmt.Errorf("server error: %w", err)
	case <-ctx.Done():
		s.logger.Info("shutting down HTTP server")
		return s.Shutdown(context.Background())
	}
}

// Shutdown gracefully shuts down the HTTP server
func (s *Server) Shutdown(ctx context.Context) error {
	if s.httpServer == nil {
		return nil
	}

	shutdownCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	s.logger.Info("HTTP server shutdown initiated")
	if err := s.httpServer.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("server shutdown failed: %w", err)
	}

	s.logger.Info("HTTP server shutdown complete")
	return nil
}

// registerHandlers registers all HTTP handlers
func (s *Server) registerHandlers(mux *http.ServeMux) {
	// Health check
	mux.HandleFunc("/health", s.handleHealth)
	mux.HandleFunc("/ready", s.handleReady)

	// Metrics
	if s.metricsEnabled {
		mux.HandleFunc("/metrics", s.handleMetrics)
	}

	// API v1 routes
	mux.HandleFunc("/api/v1/media", s.handleMediaList)
	mux.HandleFunc("/api/v1/media/sync", s.handleMediaSync)
	mux.HandleFunc("/api/v1/themes", s.handleThemesList)
	mux.HandleFunc("/api/v1/generate", s.handleGenerateAll)
	mux.HandleFunc("/api/v1/generate/", s.handleGenerateTheme)
	mux.HandleFunc("/api/v1/history", s.handleHistory)
	mux.HandleFunc("/api/v1/cooldowns", s.handleCooldowns)
	mux.HandleFunc("/api/v1/webhooks", s.handleWebhooks)
}
