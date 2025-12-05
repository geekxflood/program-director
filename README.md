# Program Director

AI-powered TV channel programmer for Tunarr using LangChain and Ollama.

## Overview

Program Director generates themed Custom Shows in Tunarr by:

1. Querying Radarr and Sonarr for available media with full metadata
2. Using an LLM (Ollama) to intelligently select content matching a theme
3. Creating Custom Shows in Tunarr for the curated content

## Features

- **AI-Powered Selection**: Uses LangChain with Ollama for intelligent media curation
- **Theme-Based Playlists**: Configure themes with keywords for matching
- **Radarr/Sonarr Integration**: Fetches rich metadata (genres, ratings, runtime, overviews)
- **Tunarr Custom Shows**: Automatically creates and updates Custom Shows
- **CLI Interface**: Easy command-line usage for manual or scheduled execution

## Installation

### Using pip

```bash
pip install .
```

### Using Docker

```bash
docker pull ghcr.io/geekxflood/program-director:latest
```

## Configuration

### Environment Variables

| Variable         | Description                                    | Required |
| ---------------- | ---------------------------------------------- | -------- |
| `OLLAMA_URL`     | Ollama API URL                                 | Yes      |
| `OLLAMA_MODEL`   | Ollama model name (default: dolphin-llama3:8b) | No       |
| `TUNARR_URL`     | Tunarr API URL                                 | Yes      |
| `RADARR_URL`     | Radarr API URL                                 | Yes      |
| `RADARR_API_KEY` | Radarr API key                                 | Yes      |
| `SONARR_URL`     | Sonarr API URL                                 | Yes      |
| `SONARR_API_KEY` | Sonarr API key                                 | Yes      |

### Config File

Create a `config.yaml`:

```yaml
ollama:
  url: "http://localhost:11434"
  model: "dolphin-llama3:8b"

tunarr:
  url: "http://localhost:8000"

radarr:
  url: "http://localhost:7878"
  # api_key from environment variable

sonarr:
  url: "http://localhost:8989"
  # api_key from environment variable

themes:
  - name: "sci-fi-night"
    description: "Science fiction themed evening"
    duration: 180
    keywords:
      - "Science Fiction"
      - "space"
      - "alien"

  - name: "horror-marathon"
    description: "Horror movie marathon"
    duration: 240
    keywords:
      - "Horror"
      - "Thriller"
```

## Usage

### CLI Commands

```bash
# Generate playlist for a specific theme
program-director generate --theme sci-fi-night

# Generate playlists for all themes
program-director generate --all-themes

# Dry run (preview without applying)
program-director generate --theme sci-fi-night --dry-run

# Scan media library
program-director scan

# List configured themes
program-director themes
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
  ghcr.io/geekxflood/program-director:latest \
  python -m program_director.cli generate --all-themes
```

## Architecture

```txt
┌─────────────┐     ┌─────────────┐     ┌─────────────┐
│   Radarr    │────▶│  Program    │────▶│   Tunarr    │
│   (Movies)  │     │  Director   │     │  (Custom    │
└─────────────┘     │             │     │   Shows)    │
                    │  LangChain  │     └─────────────┘
┌─────────────┐     │      +      │
│   Sonarr    │────▶│   Ollama    │
│ (TV/Anime)  │     │             │
└─────────────┘     └─────────────┘
```

## Development

### Setup

```bash
# Create virtual environment
python -m venv .venv
source .venv/bin/activate

# Install with dev dependencies
pip install -e ".[dev]"
```

### Linting

```bash
ruff check .
ruff format .
mypy program_director/
```

### Testing

```bash
pytest
```

## License

MIT License - see [LICENSE](LICENSE) for details.

## Related Projects

- [Tunarr](https://github.com/chrisbenincasa/tunarr) - IPTV server for custom channels
- [Radarr](https://github.com/Radarr/Radarr) - Movie management
- [Sonarr](https://github.com/Sonarr/Sonarr) - TV show management
- [Ollama](https://github.com/ollama/ollama) - Local LLM runtime
- [LangChain](https://github.com/langchain-ai/langchain) - LLM framework
