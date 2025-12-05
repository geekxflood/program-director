# Program Director

AI-powered TV channel programmer for Tunarr written in Go.

## Overview

Program Director generates themed channel programming in Tunarr by:

1. Syncing media metadata from Radarr and Sonarr to local database
2. Using similarity scoring and LLM (Ollama) to intelligently select content matching themes
3. Applying generated playlists to Tunarr channels with cooldown tracking

## Features

- **AI-Powered Selection**: Uses Ollama LLM for intelligent media curation
- **Theme-Based Playlists**: Configure themes with genres, keywords, and scheduling
- **Database-Backed**: SQLite or PostgreSQL for media caching and cooldown tracking
- **Radarr/Sonarr Integration**: Syncs and caches media metadata locally
- **Tunarr Channels**: Updates channel programming with curated content
- **Cooldown Management**: Prevents media from being replayed too frequently
- **CLI & Server Modes**: Run one-off generations or as a scheduled service

## Installation

### Using Go

```bash
go install github.com/geekxflood/program-director@latest
```

### Using Docker

```bash
docker pull ghcr.io/geekxflood/program-director:latest
```

### Building from source

```bash
git clone https://github.com/geekxflood/program-director.git
cd program-director
go build -o program-director .
```

## Configuration

### Environment Variables

| Variable            | Description                                    | Required |
| ------------------- | ---------------------------------------------- | -------- |
| `RADARR_API_KEY`    | Radarr API key                                 | Yes      |
| `SONARR_API_KEY`    | Sonarr API key                                 | Yes      |
| `RADARR_URL`        | Radarr API URL                                 | No       |
| `SONARR_URL`        | Sonarr API URL                                 | No       |
| `TUNARR_URL`        | Tunarr API URL                                 | No       |
| `OLLAMA_URL`        | Ollama API URL                                 | No       |
| `OLLAMA_MODEL`      | Ollama model name (default: dolphin-llama3:8b) | No       |
| `DB_DRIVER`         | Database driver (postgres/sqlite)              | No       |
| `POSTGRES_HOST`     | PostgreSQL host                                | No       |
| `POSTGRES_PORT`     | PostgreSQL port                                | No       |
| `POSTGRES_DATABASE` | PostgreSQL database name                       | No       |
| `POSTGRES_USER`     | PostgreSQL user                                | No       |
| `POSTGRES_PASSWORD` | PostgreSQL password                            | No       |

### Config File

Copy `configs/config.example.yaml` to `config.yaml` and customize:

```yaml
database:
  driver: "sqlite"  # or "postgres"
  sqlite:
    path: "./data/program-director.db"

radarr:
  url: "http://localhost:7878"
  # api_key from RADARR_API_KEY env var

sonarr:
  url: "http://localhost:8989"
  # api_key from SONARR_API_KEY env var

tunarr:
  url: "http://localhost:8000"

ollama:
  url: "http://localhost:11434"
  model: "dolphin-llama3:8b"
  temperature: 0.7
  num_ctx: 8192

cooldown:
  movie_days: 30
  series_days: 14
  anime_days: 14

themes:
  - name: "sci-fi-night"
    description: "Science fiction themed evening"
    channel_id: "your-tunarr-channel-id"
    schedule: "0 20 * * *"  # 8 PM daily (cron format)
    media_types:
      - "movie"
      - "series"
    genres:
      - "Science Fiction"
    keywords:
      - "space"
      - "future"
    min_rating: 6.0
    max_items: 10
    duration: 300  # minutes
```

## Usage

### CLI Commands

```bash
# Sync media metadata from Radarr/Sonarr
program-director sync

# Scan media library (display stats)
program-director scan

# Generate playlist for a specific theme
program-director generate --theme sci-fi-night

# Generate playlists for all themes
program-director generate --all-themes

# Dry run (preview without applying)
program-director generate --theme sci-fi-night --dry-run

# Run as HTTP server with scheduler
program-director serve

# Show version
program-director version
```

### Docker Usage

```bash
docker run --rm \
  -e RADARR_API_KEY=your-key \
  -e SONARR_API_KEY=your-key \
  -e OLLAMA_URL=http://ollama:11434 \
  -e TUNARR_URL=http://tunarr:8000 \
  -e RADARR_URL=http://radarr:7878 \
  -e SONARR_URL=http://sonarr:8989 \
  -v /path/to/config.yaml:/app/config/config.yaml \
  -v /path/to/data:/app/data \
  ghcr.io/geekxflood/program-director:latest \
  generate --all-themes
```

### Docker Compose Example

```yaml
version: '3.8'
services:
  program-director:
    image: ghcr.io/geekxflood/program-director:latest
    environment:
      - RADARR_API_KEY=${RADARR_API_KEY}
      - SONARR_API_KEY=${SONARR_API_KEY}
    volumes:
      - ./config.yaml:/app/config/config.yaml
      - ./data:/app/data
    command: serve
```

## Architecture

```txt
┌─────────────┐     ┌─────────────────────────────┐     ┌─────────────┐
│   Radarr    │────▶│    Program Director (Go)    │────▶│   Tunarr    │
│   (Movies)  │     │                             │     │  (Channels) │
└─────────────┘     │  ┌──────────────────────┐  │     └─────────────┘
                    │  │  Media Sync Service  │  │
┌─────────────┐     │  └──────────────────────┘  │
│   Sonarr    │────▶│           ▼                 │
│ (TV/Anime)  │     │  ┌──────────────────────┐  │
└─────────────┘     │  │ Database (SQLite/PG) │  │
                    │  └──────────────────────┘  │
┌─────────────┐     │           ▼                 │
│   Ollama    │◀────│  ┌──────────────────────┐  │
│  (LLM AI)   │     │  │ Playlist Generator   │  │
└─────────────┘     │  │ + Similarity Scorer  │  │
                    │  └──────────────────────┘  │
                    │           ▼                 │
                    │  ┌──────────────────────┐  │
                    │  │  Cooldown Manager    │  │
                    │  └──────────────────────┘  │
                    └─────────────────────────────┘
```

### Key Components

- **Media Sync**: Periodically fetches media metadata from Radarr/Sonarr
- **Database**: Caches media and tracks playback history/cooldowns
- **Similarity Scorer**: Ranks media by genre, keyword, and rating matching
- **LLM Integration**: Uses Ollama for intelligent content selection
- **Playlist Generator**: Builds themed lineups respecting cooldowns
- **Cooldown Manager**: Prevents repetitive content across channels

## Development

### Setup

```bash
# Clone repository
git clone https://github.com/geekxflood/program-director.git
cd program-director

# Install dependencies
go mod download

# Build
go build -o program-director .

# Run locally
./program-director --help
```

### Testing

```bash
# Run all tests
go test ./...

# With coverage
go test -cover ./...

# Verbose output
go test -v ./...
```

### Code Quality

```bash
# Format code
go fmt ./...

# Lint
golangci-lint run

# Vet
go vet ./...
```

## License

MIT License - see [LICENSE](LICENSE) for details.

## Workflow

1. **Initial Sync**: Run `program-director sync` to populate the database with your media
2. **Configure Themes**: Edit `config.yaml` with your Tunarr channel IDs and theme definitions
3. **Generate Playlists**: Run `program-director generate --all-themes` to create programming
4. **Schedule Updates**: Use `program-director serve` to run as a service with automatic scheduling
5. **Monitor**: Check logs and database for playlist history and cooldown tracking

## Related Projects

- [Tunarr](https://github.com/chrisbenincasa/tunarr) - IPTV server for custom channels
- [Radarr](https://github.com/Radarr/Radarr) - Movie management
- [Sonarr](https://github.com/Sonarr/Sonarr) - TV show management
- [Ollama](https://github.com/ollama/ollama) - Local LLM runtime
