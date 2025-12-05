# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Program Director is an AI-powered TV channel programmer written in Go that curates themed programming for Tunarr channels. It uses Ollama (local LLM) to intelligently select media from Radarr/Sonarr based on themes, with database-backed caching and cooldown management.

**Version**: 1.0.0
**Language**: Go 1.23+
**Architecture**: Microservice-ready with HTTP API, Kubernetes support, and GitOps deployment

## Development Commands

### Building and Running

```bash
# Build the application
go build -o program-director .

# Build with version information
go build -ldflags "-X main.version=1.0.0 -X main.commit=$(git rev-parse HEAD) -X main.date=$(date -u +%Y-%m-%dT%H:%M:%SZ)" -o program-director .

# Run locally
./program-director --help

# Run with config file
./program-director -c config.yaml generate --all-themes

# Enable debug logging
./program-director --debug scan
```

### Code Quality

```bash
# Format code
go fmt ./...

# Lint code
golangci-lint run

# Vet code
go vet ./...

# Run all quality checks
go fmt ./... && go vet ./... && golangci-lint run
```

### Testing

```bash
# Run all tests
go test ./...

# Run tests with coverage
go test -cover ./...

# Verbose test output
go test -v ./...

# Run specific test
go test -run TestName ./internal/...

# Coverage report
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out
```

### Docker Build

```bash
# Build local image
docker build -t program-director:dev .

# Build with version info
docker build --build-arg VERSION=1.0.0 --build-arg COMMIT=$(git rev-parse HEAD) -t program-director:dev .

# Run container
docker run --rm \
  -e RADARR_API_KEY=your-key \
  -e SONARR_API_KEY=your-key \
  -v $(pwd)/config.yaml:/app/config/config.yaml \
  -v $(pwd)/data:/app/data \
  program-director:dev generate --all-themes
```

### Application Commands

```bash
# Sync media from Radarr/Sonarr to database
program-director sync                              # Sync all media
program-director sync --movies-only                # Movies only
program-director sync --series-only                # TV shows only
program-director sync --cleanup                    # Remove deleted media

# Scan and display media statistics
program-director scan                              # Show statistics
program-director scan --detailed                   # Detailed breakdown
program-director scan --source radarr              # Specific source only

# Generate playlists
program-director generate --theme sci-fi-night     # Single theme
program-director generate --all-themes             # All themes
program-director generate --theme horror --dry-run # Preview only

# Run as HTTP server with scheduler
program-director serve                             # Start server
program-director serve --port 8080                 # Custom port
program-director serve --enable-scheduler          # Enable cron scheduler
program-director serve --schedule "0 2 * * *"      # Custom schedule

# Explore Trakt.tv content
program-director trakt trending --movies           # Trending movies
program-director trakt trending --shows            # Trending shows
program-director trakt popular --movies --limit 20 # Top 20 popular movies
program-director trakt search "inception"          # Search content

# Show version
program-director version
```

## Architecture

### High-Level Data Flow

```text
┌─────────────────────────────────────────────────────────────────────┐
│                         HTTP API Server (Optional)                   │
│  ┌────────────┐    ┌──────────────┐    ┌────────────────────────┐  │
│  │ REST API   │    │  Scheduler   │    │  Prometheus Metrics    │  │
│  │ Endpoints  │    │  (Cron Jobs) │    │  /metrics              │  │
│  └────────────┘    └──────────────┘    └────────────────────────┘  │
└─────────────────────────────────────────────────────────────────────┘
                                │
                                ▼
┌─────────────────────────────────────────────────────────────────────┐
│                         Core Services                                │
│                                                                       │
│  Radarr/Sonarr → Sync → Database → Generator → [Scorer + LLM] → Tunarr │
│                  Trakt →    ↓                       ↓                 │
│                         Media Cache         Cooldown Manager          │
└─────────────────────────────────────────────────────────────────────┘
```

The workflow:

**CLI Mode:**
1. User runs `sync` command to populate database with media metadata
2. User configures themes in `config.yaml` with channel IDs and criteria
3. User runs `generate` command for themes
4. `Generator` fetches candidates from database (excluding cooldowns)
5. `Scorer` ranks media by similarity to theme (genres, keywords, ratings)
6. Optionally, Ollama LLM refines selection
7. `Generator` applies playlist to Tunarr channel via API
8. `CooldownManager` tracks plays to prevent repetition

**Server Mode:**
1. User runs `serve` command to start HTTP server
2. Optional cron scheduler triggers automatic generation per theme
3. HTTP API exposes all operations (sync, generate, scan, history, cooldowns)
4. Prometheus metrics available at `/metrics` endpoint
5. Health checks at `/health` and `/ready` for Kubernetes
6. Graceful shutdown on SIGINT/SIGTERM signals

**Trakt Integration:**
- Explore trending and popular movies/shows
- Search for content
- Retrieve detailed media information
- Optional - no API key required for basic features

### Module Responsibilities

**[cmd/root.go](cmd/root.go)** - CLI framework

- Cobra-based command structure
- Configuration loading with Viper
- Global flags: `--config`, `--debug`, `--db-driver`
- Subcommands: `generate`, `sync`, `scan`, `serve`, `trakt`, `version`
- Persistent pre-run hook for config initialization

**[cmd/generate.go](cmd/generate.go)** - Playlist generation command

- Flags: `--theme`, `--all-themes`, `--dry-run`
- Validates theme selection
- Initializes services and orchestrates generation
- Handles graceful shutdown on SIGINT/SIGTERM

**[cmd/sync.go](cmd/sync.go)** - Media synchronization command

- Flags: `--movies-only`, `--series-only`, `--cleanup`
- Fetches media from Radarr/Sonarr APIs
- Stores/updates media metadata in database
- Reports sync statistics (new, updated, unchanged)
- Optional cleanup of deleted media

**[cmd/scan.go](cmd/scan.go)** - Media library inspection

- Flags: `--detailed`, `--source`
- Displays database statistics
- Shows media counts by type, genre, rating
- Useful for verifying sync results

**[cmd/serve.go](cmd/serve.go)** - HTTP server mode

- Flags: `--port`, `--enable-scheduler`, `--schedule`, `--dry-run`
- Runs HTTP server on configured port (default: 8080)
- Optional built-in cron scheduler for automatic generation
- Initializes all services (database, clients, sync, scorer, generator)
- Graceful shutdown support with context cancellation
- Health and readiness check endpoints

**[cmd/trakt.go](cmd/trakt.go)** - Trakt.tv integration

- Subcommands: `trending`, `popular`, `search`
- Flags: `--movies`, `--shows`, `--limit`
- Explores trending and popular content
- Searches for movies and TV shows
- Displays detailed media information
- Optional - no API key validation required

**[internal/config/config.go](internal/config/config.go)** - Configuration management

- Viper-based configuration with YAML file + environment variables
- Structures: `Config`, `DatabaseConfig`, `RadarrConfig`, `SonarrConfig`, `TunarrConfig`, `TraktConfig`, `OllamaConfig`, `ThemeConfig`, `CooldownConfig`, `ServerConfig`
- Environment variables override YAML values with `PROGRAMDIR_` prefix
- Validation ensures required fields present
- Default config search paths: `.`, `./configs`, `/etc/program-director`, `~/.config/program-director`
- Trakt config optional (client_id and client_secret)

**[internal/database/](internal/database/)** - Database layer

- `database.go`: Interface definition and factory function
- `sqlite.go`: SQLite implementation using modernc.org/sqlite
- `postgres.go`: PostgreSQL implementation using pgx/v5
- `repository/media.go`: Media CRUD operations
- `repository/history.go`: Playback history tracking
- `repository/cooldown.go`: Cooldown queries
- Schema migrations embedded in code
- Connection pooling and context support

**[internal/clients/radarr/client.go](internal/clients/radarr/client.go)** - Radarr API

- HTTP client for Radarr v3 API
- Methods: `GetMovies()`, `GetMovie(id)`, `GetQueue()`, `GetSystemStatus()`
- Filters to only movies with files available
- 30-second timeout, API key authentication

**[internal/clients/sonarr/client.go](internal/clients/sonarr/client.go)** - Sonarr API

- HTTP client for Sonarr v3 API
- Methods: `GetSeries()`, `GetSeason(id)`, `GetEpisodes(seriesID)`, `GetQueue()`, `GetSystemStatus()`
- Distinguishes between anime and regular series
- Filters to only content with files available

**[internal/clients/tunarr/client.go](internal/clients/tunarr/client.go)** - Tunarr API

- HTTP client for Tunarr REST API
- Methods: `GetChannels()`, `GetChannel(id)`, `GetMediaSources()`, `SetProgramming()`, `GetProgramming()`
- Creates/updates channel programming lineups
- Handles Plex media source integration

**[internal/clients/ollama/client.go](internal/clients/ollama/client.go)** - Ollama LLM

- HTTP client for Ollama API
- Methods: `Generate()`, `Chat()`, `ListModels()`
- Streaming and non-streaming response support
- Configurable temperature, context window

**[internal/clients/trakt/client.go](internal/clients/trakt/client.go)** - Trakt.tv API

- HTTP client for Trakt API v2
- Methods: `GetMovie()`, `GetShow()`, `GetTrendingMovies()`, `GetTrendingShows()`, `GetPopularMovies()`, `GetPopularShows()`, `Search()`
- API key authentication via headers (trakt-api-key, trakt-api-version)
- 30-second timeout
- Returns rich metadata: ratings, genres, runtime, year, IDs (TMDB, IMDB, Trakt)

**[internal/services/media/sync.go](internal/services/media/sync.go)** - Media synchronization

- `Syncer` service coordinates Radarr/Sonarr fetching
- Converts API responses to database models
- Batch inserts/updates for performance
- Reports sync statistics (new, updated, unchanged)

**[internal/services/similarity/scorer.go](internal/services/similarity/scorer.go)** - Content scoring

- `Scorer` ranks media against theme criteria
- Scoring factors:
  - Genre overlap (weighted heavily)
  - Keyword matches in title/overview
  - Rating proximity to `min_rating`
  - Year (prefer recent content)
- Returns sorted list of `MediaWithScore` candidates
- Respects `max_items` and `duration` targets

**[internal/services/playlist/generator.go](internal/services/playlist/generator.go)** - Playlist generation

- `Generator` orchestrates the full generation workflow
- `Generate()`: Single theme generation
- `GenerateAll()`: All themes with context cancellation
- Steps:
  1. Get cooldown exclusions
  2. Find candidates via Scorer
  3. Build playlist
  4. Apply to Tunarr (if not dry-run)
  5. Record plays and cooldowns
- Returns `GenerationResult` with stats and errors

**[internal/services/cooldown/manager.go](internal/services/cooldown/manager.go)** - Cooldown tracking

- `Manager` prevents media replay too soon
- `RecordPlay()`: Logs media playback
- `GetActiveCooldownMediaIDs()`: Returns media IDs to exclude
- Cooldown periods configurable by type (movies, series, anime)
- Database-backed for persistence across runs

**[internal/server/server.go](internal/server/server.go)** - HTTP server

- `Server` struct manages HTTP server lifecycle
- Methods: `Start()`, `Stop()`, `setupRoutes()`
- Integrates all services (database, sync, generator, cooldown)
- Graceful shutdown with configurable timeout
- Context cancellation support
- Configurable port (default: 8080)

**[internal/server/handlers.go](internal/server/handlers.go)** - HTTP handlers

- RESTful API endpoints (12 handlers)
- Health checks: `GET /health`, `GET /ready`
- Metrics: `GET /metrics` (Prometheus format)
- Media: `GET /api/v1/media`, `POST /api/v1/media/sync`
- Themes: `GET /api/v1/themes`, `POST /api/v1/generate`
- History: `GET /api/v1/history`
- Cooldowns: `GET /api/v1/cooldowns`
- JSON responses with success/error structure
- Proper HTTP status codes and error handling

**[internal/scheduler/scheduler.go](internal/scheduler/scheduler.go)** - Cron scheduler

- `Scheduler` struct wraps robfig/cron/v3
- Methods: `Start()`, `Stop()`, `GetStatus()`, `GetNextRun()`
- Configurable cron expressions (default: "0 2 * * *")
- Automatic playlist generation per theme schedule
- Panic recovery and error logging
- Integration with serve command
- Context-aware shutdown

**[pkg/models/media.go](pkg/models/media.go)** - Domain models

- `Media`: Core media entity (ID, title, year, genres, ratings, runtime, path, etc.)
- `MediaWithScore`: Media + similarity score + match reason
- `Playlist`: Collection of media items for a theme
- Shared across all services

## HTTP API Endpoints

When running in server mode (`program-director serve`), the following REST API endpoints are available:

### Health & Monitoring

- `GET /health` - Health check (always returns 200 OK with version info)
- `GET /ready` - Readiness check (checks database connectivity)
- `GET /metrics` - Prometheus metrics (media counts, generation stats)

### Media Management

- `GET /api/v1/media` - List all media from database
  - Query params: `type` (movie/series/anime), `limit`, `offset`
  - Returns: Array of media objects with metadata

- `POST /api/v1/media/sync` - Trigger media synchronization
  - Body: `{"movies": true, "series": true, "cleanup": false}`
  - Returns: Sync statistics (new, updated, unchanged)

### Theme & Generation

- `GET /api/v1/themes` - List all configured themes
  - Returns: Theme configurations with counts

- `POST /api/v1/generate` - Generate playlists
  - Body: `{"theme": "theme-name", "dry_run": false}` or `{"all_themes": true}`
  - Returns: Generation results with statistics

### History & Cooldowns

- `GET /api/v1/history` - Get play history
  - Query params: `channel_id`, `theme`, `limit`
  - Returns: Array of play history records

- `GET /api/v1/cooldowns` - Get active cooldowns
  - Returns: Array of media IDs currently on cooldown

### Response Format

All API responses follow this structure:

```json
{
  "success": true,
  "data": { ... },
  "error": null
}
```

Error responses:

```json
{
  "success": false,
  "data": null,
  "error": "Error message"
}
```

## Key Implementation Details

### Configuration Priority

1. YAML config file provides base values
2. Environment variables override YAML (with `PROGRAMDIR_` prefix)
3. CLI flags override both (e.g., `--debug`, `--db-driver`)

Required environment variables:
- `RADARR_API_KEY` - Required for Radarr access
- `SONARR_API_KEY` - Required for Sonarr access

All other settings have defaults or can be set in YAML.

### Database Schema

The application uses three main tables:

- `media`: Stores all movies, series, anime with metadata
- `play_history`: Tracks when media was played on which channel
- `cooldowns`: Computed view of media on cooldown

Migrations run automatically on startup.

### LLM Integration (Optional)

The Ollama client is available for advanced selection logic:

1. Scorer provides initial candidate set
2. LLM can refine based on nuanced criteria (plot themes, mood, pacing)
3. Falls back to scored results if LLM unavailable

Currently, scoring alone is sufficient for most use cases.

### Idempotent Generation

The generator applies programming to Tunarr channels:

- Overwrites existing channel programming
- Re-running the same theme updates rather than duplicates
- Cooldowns prevent same content across multiple runs

### Resource Management

All services implement proper cleanup:

- Database connections use connection pooling
- HTTP clients have timeouts (30s default)
- Context cancellation propagates through call chains
- Graceful shutdown on signals

## Kubernetes Deployment

### Helm Chart

The project includes a comprehensive Helm chart in `charts/program-director/`:

**Chart Structure:**
- `Chart.yaml` - Chart metadata (version 1.0.0, appVersion 1.0.0)
- `values.yaml` - Default configuration
- `values-production.yaml` - Production overrides
- `values-staging.yaml` - Staging overrides
- `templates/` - Kubernetes manifests

**Key Features:**
- Deployment with security contexts (non-root, read-only filesystem)
- Service (ClusterIP, NodePort, or LoadBalancer)
- ConfigMap for application configuration
- Secret for sensitive data (API keys)
- PersistentVolumeClaim for SQLite data
- Ingress with TLS support
- ServiceMonitor for Prometheus Operator
- HorizontalPodAutoscaler for auto-scaling
- Health and readiness probes

**Installation:**

```bash
# Install with default values
helm install program-director ./charts/program-director

# Install with custom values
helm install program-director ./charts/program-director \
  --values ./charts/program-director/values-production.yaml

# Upgrade existing installation
helm upgrade program-director ./charts/program-director

# Uninstall
helm uninstall program-director
```

### ArgoCD GitOps

The `argocd/applicationset.yaml` provides multi-environment deployment:

**Features:**
- List generator for production and staging environments
- Automatic sync with prune and self-heal
- Retry logic with exponential backoff
- Environment-specific value files
- Notification support

**Deployment:**

```bash
# Apply ApplicationSet
kubectl apply -f argocd/applicationset.yaml

# Verify applications
argocd app list
argocd app get program-director-production
```

### Configuration

**Required Secrets:**
```yaml
radarr:
  apiKey: "your-radarr-api-key"
sonarr:
  apiKey: "your-sonarr-api-key"
```

**Optional Configuration:**
```yaml
trakt:
  clientId: "your-trakt-client-id"
  clientSecret: "your-trakt-client-secret"
```

**Database:**
- SQLite: Use PersistentVolume for data persistence
- PostgreSQL: Configure external database connection

## Code Style

- **Language**: Go 1.23+
- **Formatting**: `go fmt` (enforced)
- **Linting**: golangci-lint (recommended)
- **Imports**: Group stdlib, external, internal
- **Naming**: Follow Go conventions (MixedCaps, acronyms all-caps)
- **Errors**: Wrap with context using `fmt.Errorf` with `%w`
- **Logging**: Structured logging with `log/slog`

## Testing

### Testing Strategy

- **Framework**: Go standard `testing` package
- **Coverage**: 15-42% coverage across new features
- **Approach**: Table-driven tests with httptest for HTTP clients

### Existing Tests

**Trakt Client** (`internal/clients/trakt/client_test.go`) - 34.5% coverage:
- `TestNew` - Client initialization
- `TestGetMovie` - Movie retrieval with mock server
- `TestGetTrendingMovies` - Trending movies endpoint
- `TestSearch` - Search functionality
- `TestDoRequestError` - Error handling

**Scheduler** (`internal/scheduler/scheduler_test.go`) - 42.6% coverage:
- `TestNewScheduler` - Scheduler creation
- `TestGetStatus` - Status reporting
- `TestSchedulerStartStop` - Lifecycle management
- `TestGetNextRun` - Schedule calculation

**HTTP Handlers** (`internal/server/handlers_test.go`) - 5.9% coverage:
- `TestWriteJSON` - JSON response helper
- `TestHandleHealth` - Health check endpoint
- `TestHandleThemesList` - Themes listing endpoint

### Adding Tests

When adding tests:
- Place in `*_test.go` files alongside source
- Use table-driven tests for multiple cases
- Mock external dependencies with httptest.NewServer
- Use httptest.NewRecorder for HTTP handler testing
- Test both success and error paths
- Verify HTTP status codes and response bodies

## CI/CD

### GitHub Actions Workflows

**Build & Test** (`.github/workflows/docker.yml`):
- **Trigger**: Push to main, pull requests, tags
- **Build**: Multi-platform Docker images (linux/amd64, linux/arm64)
- **Test**: Run `go test ./...` with race detection
- **Lint**: Run `golangci-lint` with 20+ linters
- **Security**: Run `gosec` with SARIF upload
- **Coverage**: Upload to Codecov
- **Registry**: GHCR (ghcr.io/geekxflood/program-director)
- **Tags**: `latest`, semver (`1.0.0`, `1.0`, `1`), SHA, PR numbers

**Release** (`.github/workflows/release.yml`):
- **Trigger**: Git tags matching `v*.*.*` (semantic versioning)
- **Extract**: Version from tag name
- **Build**: Multi-architecture Docker images with version labels
- **Release Notes**: Extracted from CHANGELOG.md for the version
- **GitHub Release**: Automatically created with notes
- **SBOM**: Generated and uploaded as release artifact
- **Docker Tags**: Version-specific (`v1.0.0`) and `latest`

### Release Process

See [RELEASING.md](RELEASING.md) for comprehensive release documentation.

**Quick Release:**
1. Update CHANGELOG.md with new version section
2. Update Helm chart version in `charts/program-director/Chart.yaml`
3. Commit changes: `git commit -m "chore: Prepare release v1.1.0"`
4. Create tag: `git tag -a v1.1.0 -m "Release v1.1.0"`
5. Push: `git push && git push origin v1.1.0`
6. GitHub Actions will automatically build and release

## External Dependencies

**Required Services**:

- **Ollama**: Local LLM runtime (default model: `dolphin-llama3:8b`) - optional
- **Radarr**: Movie library manager (API key required)
- **Sonarr**: TV/anime library manager (API key required)
- **Tunarr**: IPTV server for custom channels

**Key Libraries**:

- `github.com/spf13/cobra` - CLI framework
- `github.com/spf13/viper` - Configuration management
- `github.com/jackc/pgx/v5` - PostgreSQL driver
- `modernc.org/sqlite` - Pure Go SQLite
- `github.com/robfig/cron/v3` - Cron scheduler
- `log/slog` - Structured logging (stdlib)
- Standard library for HTTP, JSON, context, net/http, etc.

## Important Constraints

1. **API timeouts**: 30 seconds for all external HTTP calls
2. **Database**: SQLite for single-instance, PostgreSQL for multi-instance
3. **Radarr/Sonarr**: Only fetches media with files (excludes monitored-but-missing)
4. **Tunarr**: Requires channel IDs in theme configuration (get from Tunarr API/UI)
5. **Cooldowns**: Prevents immediate replay but doesn't guarantee even distribution

## Common Patterns

### Adding a New Command

1. Create `cmd/mycommand.go`
2. Define `mycommandCmd` with `cobra.Command`
3. Add to `init()` in `cmd/root.go`: `rootCmd.AddCommand(mycommandCmd)`
4. Implement `RunE` function with context and service initialization

### Adding a New Theme

Edit `config.yaml`:

```yaml
themes:
  - name: "theme-name"           # Unique identifier (kebab-case)
    description: "Description"   # Used in LLM prompt
    channel_id: "uuid"           # Tunarr channel ID
    schedule: "0 20 * * *"       # Cron expression (for serve mode)
    media_types:
      - "movie"
      - "series"
    genres:
      - "Genre1"
      - "Genre2"
    keywords:
      - "keyword1"
    min_rating: 6.0
    max_items: 10
    duration: 300                # Target duration in minutes
```

### Enabling Trakt Integration

Edit `config.yaml`:

```yaml
trakt:
  client_id: "your-trakt-client-id"          # Get from https://trakt.tv/oauth/applications
  client_secret: "your-trakt-client-secret"  # Optional for basic features
```

Or set environment variables:

```bash
export PROGRAMDIR_TRAKT_CLIENT_ID="your-client-id"
export PROGRAMDIR_TRAKT_CLIENT_SECRET="your-client-secret"
```

Then use Trakt commands:

```bash
# Explore trending content
program-director trakt trending --movies --limit 20

# Search for content
program-director trakt search "inception"

# Get popular shows
program-director trakt popular --shows
```

### Extending the Scorer

Edit `internal/services/similarity/scorer.go`:

- Add new scoring factors (e.g., actor matching, director preferences)
- Adjust weights in score calculation
- Update `MatchReason` to explain score

### Debugging Generation

Use `--dry-run` to preview without applying:

```bash
program-director --debug generate --theme sci-fi-night --dry-run
```

This will:
- Show all scoring details in debug logs
- Display selected media with scores
- Skip Tunarr API calls and cooldown recording

## Database Queries

Common manual queries for debugging:

```sql
-- View all media
SELECT id, title, year, media_type, rating, genres FROM media;

-- View recent plays
SELECT m.title, ph.channel_id, ph.theme_name, ph.played_at
FROM play_history ph
JOIN media m ON ph.media_id = m.id
ORDER BY ph.played_at DESC
LIMIT 20;

-- Check cooldowns
SELECT m.title, m.media_type, ph.played_at, ph.channel_id
FROM media m
JOIN play_history ph ON m.id = ph.media_id
WHERE ph.played_at > datetime('now', '-30 days')
ORDER BY ph.played_at DESC;

-- Media by genre
SELECT genres, COUNT(*) FROM media GROUP BY genres ORDER BY COUNT(*) DESC;
```

## Troubleshooting

**"No candidates found for theme"**
- Check theme criteria aren't too restrictive
- Run `program-director scan` to verify media in database
- Check `min_rating`, `genres`, `keywords` in theme config

**"Failed to connect to database"**
- Verify database driver configured correctly
- For PostgreSQL, check connection string and credentials
- For SQLite, ensure data directory is writable

**"Radarr/Sonarr API error"**
- Verify API keys are correct
- Check URLs are reachable from application
- Ensure Radarr/Sonarr versions are v3+

**"Channel not found in Tunarr"**
- Verify `channel_id` in theme config matches Tunarr
- Get channel IDs via: `curl http://tunarr:8000/api/channels`
- Or check Tunarr UI channel settings
