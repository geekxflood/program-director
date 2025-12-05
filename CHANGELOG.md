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
- `serve` - HTTP server mode (prepared for future implementation)
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
- **Ollama Client**: LLM integration for intelligent selection

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

#### CI/CD
- GitHub Actions workflow for automated builds
- Multi-platform Docker image builds
- Automated testing with race detection
- Linting with `go fmt` and `go vet`
- Docker image publishing to GHCR
- Semantic versioning support

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
