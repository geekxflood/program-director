# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Program Director is an AI-powered TV channel programmer written in Go that curates themed programming for Tunarr channels. It uses Ollama (local LLM) to intelligently select media from Radarr/Sonarr based on themes, with database-backed caching and cooldown management.

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
program-director sync

# Scan and display media statistics
program-director scan

# Generate playlists
program-director generate --theme sci-fi-night     # Single theme
program-director generate --all-themes             # All themes
program-director generate --theme horror --dry-run # Preview only

# Run as HTTP server with scheduler
program-director serve

# Show version
program-director version
```

## Architecture

### High-Level Data Flow

```text
Radarr/Sonarr → Sync → Database → Generator → [Scorer + LLM] → Tunarr
                         ↓                          ↓
                   Media Cache              Cooldown Manager
```

The workflow:

1. User runs `sync` command to populate database with media metadata
2. User configures themes in `config.yaml` with channel IDs and criteria
3. User runs `generate` command for themes
4. `Generator` fetches candidates from database (excluding cooldowns)
5. `Scorer` ranks media by similarity to theme (genres, keywords, ratings)
6. Optionally, Ollama LLM refines selection
7. `Generator` applies playlist to Tunarr channel via API
8. `CooldownManager` tracks plays to prevent repetition

### Module Responsibilities

**[cmd/root.go](cmd/root.go)** - CLI framework

- Cobra-based command structure
- Configuration loading with Viper
- Global flags: `--config`, `--debug`, `--db-driver`
- Subcommands: `generate`, `sync`, `scan`, `serve`, `version`
- Persistent pre-run hook for config initialization

**[cmd/generate.go](cmd/generate.go)** - Playlist generation command

- Flags: `--theme`, `--all-themes`, `--dry-run`
- Validates theme selection
- Initializes services and orchestrates generation
- Handles graceful shutdown on SIGINT/SIGTERM

**[cmd/sync.go](cmd/sync.go)** - Media synchronization command

- Fetches media from Radarr/Sonarr APIs
- Stores/updates media metadata in database
- Reports sync statistics

**[cmd/scan.go](cmd/scan.go)** - Media library inspection

- Displays database statistics
- Shows media counts by type, genre, rating
- Useful for verifying sync results

**[cmd/serve.go](cmd/serve.go)** - HTTP server mode

- Runs HTTP server on configured port
- Optional built-in cron scheduler for automatic generation
- Graceful shutdown support
- Health check endpoints

**[internal/config/config.go](internal/config/config.go)** - Configuration management

- Viper-based configuration with YAML file + environment variables
- Structures: `Config`, `DatabaseConfig`, `RadarrConfig`, `SonarrConfig`, `TunarrConfig`, `OllamaConfig`, `ThemeConfig`, `CooldownConfig`, `ServerConfig`
- Environment variables override YAML values
- Validation ensures required fields present
- Default config search paths: `.`, `./configs`, `/etc/program-director`, `~/.config/program-director`

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

**[pkg/models/media.go](pkg/models/media.go)** - Domain models

- `Media`: Core media entity (ID, title, year, genres, ratings, runtime, path, etc.)
- `MediaWithScore`: Media + similarity score + match reason
- `Playlist`: Collection of media items for a theme
- Shared across all services

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

## Code Style

- **Language**: Go 1.22+
- **Formatting**: `go fmt` (enforced)
- **Linting**: golangci-lint (recommended)
- **Imports**: Group stdlib, external, internal
- **Naming**: Follow Go conventions (MixedCaps, acronyms all-caps)
- **Errors**: Wrap with context using `fmt.Errorf` with `%w`
- **Logging**: Structured logging with `log/slog`

## Testing

- Framework: Go standard `testing` package
- Currently minimal test coverage - needs expansion
- When adding tests:
  - Place in `*_test.go` files alongside source
  - Use table-driven tests for multiple cases
  - Mock external dependencies (Radarr, Sonarr, Tunarr, Ollama)
  - Use `testify/assert` for assertions (optional)

## CI/CD

GitHub Actions workflow (`.github/workflows/docker.yml`):

- **Build**: Multi-platform Docker images (linux/amd64, linux/arm64)
- **Test**: Run `go test ./...`
- **Lint**: Run `golangci-lint`
- **Registry**: GHCR (ghcr.io/geekxflood/program-director)
- **Tags**: `latest`, semver (`1.0.0`, `1.0`, `1`), SHA, PR numbers

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
- `log/slog` - Structured logging (stdlib)
- Standard library for HTTP, JSON, context, etc.

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
