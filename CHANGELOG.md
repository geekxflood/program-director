# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [1.0.0] - 2025-12-05

### Added

#### Core Application
- Complete rewrite from Python to Go for improved performance and deployment
- Multi-architecture Docker support (amd64, arm64)
- Database support for both SQLite and PostgreSQL
- Graceful shutdown handling with context cancellation
- Version information embedded at build time

#### CLI Commands
- `generate` - Generate themed playlists for Tunarr channels
  - Single theme or all themes generation
  - Dry-run mode for preview without applying
  - Result reporting with statistics
- `sync` - Synchronize media catalog from Radarr/Sonarr
  - Selective sync (movies only, series only, or all)
  - Cleanup option for removed media
- `scan` - Display media library statistics
  - Detailed information mode
  - Source-specific scanning
- `serve` - HTTP server mode with RESTful API and scheduler
  - Configurable HTTP port
  - Optional cron scheduler for automated generation
  - Prometheus metrics endpoint
  - Health and readiness checks
- `trakt` - Trakt.tv media exploration commands
  - Trending movies and shows
  - Popular content discovery
  - Search functionality
- `version` - Display version and build information

#### Structured Logging
- Comprehensive structured logging using stdlib `log/slog`
- Dual output formats: human-readable text and JSON
- Debug mode with source file and line information
- Custom time formatting for readability
- Global context (version, app name) in all logs
- Following Prometheus/Alertmanager patterns
- Proper log levels: DEBUG, INFO, WARN, ERROR

#### Configuration
- YAML-based configuration with environment variable overrides
- Viper for flexible configuration management
- Validation for all required settings
- Support for multiple themes with detailed criteria
- Cooldown configuration per media type
- Server configuration for HTTP mode

#### Services
- **Playlist Generator**: Orchestrates theme-based playlist generation
- **Similarity Scorer**: Ranks media by genre, keywords, and ratings
- **Cooldown Manager**: Prevents media replay too soon
- **Media Sync**: Fetches and caches metadata from Radarr/Sonarr

#### API Clients
- **Radarr Client**: Movie library integration
- **Sonarr Client**: TV/anime library integration
- **Tunarr Client**: Channel programming management
- **Trakt Client**: Media metadata and trending content integration
  - Full API v2 support
  - Trending and popular content
  - Search functionality
  - Movie and show details
- **Ollama Client**: LLM integration for intelligent selection

#### HTTP API Server
- RESTful API endpoints for all operations
- Health and readiness checks
- Prometheus metrics export
- Media listing and synchronization endpoints
- Theme management and generation endpoints
- Play history and cooldown tracking endpoints
- Webhook support for automation
- Graceful shutdown with timeout
- JSON response format
- Configurable timeouts and ports

#### Scheduler
- Cron-based automated playlist generation
- Configurable schedule per theme
- Panic recovery and logging
- Manual trigger support
- Status reporting (next run, job count)
- Integration with serve command
- Default schedule: daily at 2 AM

#### Database
- Abstract database interface supporting multiple drivers
- SQLite implementation for single-instance deployments
- PostgreSQL implementation for production deployments
- Automatic schema migrations
- Repository pattern for data access
- Connection pooling and context support

#### Testing
- Unit tests for configuration validation
- Unit tests for model structures
- Test coverage reporting via Codecov
- Table-driven test patterns

#### Kubernetes & Deployment
- Comprehensive Helm chart for Kubernetes deployment
  - Full manifest templates (Deployment, Service, ConfigMap, Secret, etc.)
  - PersistentVolumeClaim for SQLite data
  - Ingress with TLS support
  - ServiceMonitor for Prometheus Operator
  - HorizontalPodAutoscaler support
  - Security contexts (non-root, read-only filesystem)
  - Resource requests and limits
  - Health probes configuration
- ArgoCD ApplicationSet for GitOps deployment
  - Multi-environment support (production, staging)
  - Automatic sync and self-healing
  - Retry logic with exponential backoff
  - Notification support
- Environment-specific values files
  - Production configuration with HA
  - Staging configuration for testing
- Complete Helm chart documentation

#### CI/CD
- GitHub Actions workflow for automated builds
- Multi-platform Docker image builds (amd64, arm64)
- Automated testing with race detection and coverage
- golangci-lint for comprehensive linting (20+ linters)
- gosec for security scanning with SARIF upload
- Docker image publishing to GHCR
- Release workflow triggered by semver tags
- Automatic release note generation from CHANGELOG
- SBOM generation and upload

### Changed
- **Breaking**: Migrated from Python to Go - incompatible with previous Python version
- **Breaking**: New configuration format (YAML-based)
- **Breaking**: Database schema redesigned for Go implementation
- Improved performance and memory efficiency
- Better error handling with wrapped errors
- Enhanced observability with structured logging

### Removed
- Python implementation and dependencies
- Legacy Python-specific features
- Old configuration format

### Fixed
- Configuration loading now properly handles defaults
- Database connection properly closes on shutdown
- Graceful signal handling for clean shutdowns

### Security
- Non-root Docker user (UID 1000)
- API key validation at startup
- Secure database connection string handling
- No secrets in logs

## [0.x.x] - Python Era

Previous Python-based versions are no longer maintained. See git history for details.

[Unreleased]: https://github.com/geekxflood/program-director/compare/v1.0.0...HEAD
[1.0.0]: https://github.com/geekxflood/program-director/releases/tag/v1.0.0
