# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Program Director is an AI-powered TV channel programmer that curates themed Custom Shows for Tunarr. It uses LangChain with Ollama (local LLM) to intelligently select media from Radarr/Sonarr based on themes.

## Development Commands

### Environment Setup

```bash
# Create and activate virtual environment
python -m venv .venv
source .venv/bin/activate  # On Windows: .venv\Scripts\activate

# Install with dev dependencies
pip install -e ".[dev]"
```

### Code Quality

```bash
# Lint and format code
ruff check .                  # Check for linting issues
ruff format .                 # Auto-format code
ruff check . --fix           # Auto-fix issues where possible

# Type checking
mypy program_director/        # Run type checker (strict mode enabled)

# Run tests
pytest                        # Run test suite
pytest -v                     # Verbose output
pytest -k test_name          # Run specific test
```

### Docker Build

```bash
# Build local image
docker build -t program-director:dev .

# Run container
docker run --rm \
  -e RADARR_API_KEY=your-key \
  -e SONARR_API_KEY=your-key \
  -v /path/to/config.yaml:/app/config/config.yaml \
  program-director:dev generate --all-themes
```

### Application Commands

```bash
# Generate playlists
program-director generate --theme sci-fi-night     # Single theme
program-director generate --all-themes             # All themes
program-director generate --theme horror --dry-run # Preview only

# Scan media library (displays stats)
program-director scan

# List configured themes
program-director themes
```

## Architecture

### High-Level Data Flow

```text
CLI → Config → Agent → [MediaLibrary, LLM, TunarrClient]
                        ↓           ↓          ↓
                    Radarr/     Ollama      Tunarr
                    Sonarr               Custom Shows
```

The workflow:

1. User invokes CLI command
2. Config loaded from YAML + environment variables
3. `PlaylistAgent` orchestrates the workflow
4. `MediaLibrary` fetches and caches media metadata from Radarr/Sonarr
5. Agent generates LLM prompt with theme + media summary
6. Ollama LLM selects matching content (returns JSON)
7. `TunarrClient` creates/updates Custom Show

### Module Responsibilities

**[config.py](program_director/config.py)** - Configuration management

- Loads YAML config and merges with environment variables
- Pydantic models for validation: `AgentConfig`, `ThemeConfig`, `OllamaConfig`, `TunarrConfig`, `RadarrConfig`, `SonarrConfig`
- Environment variables override config file values
- Default config path: `/app/config/config.yaml` (Docker-friendly)

**[scanner.py](program_director/scanner.py)** - Media metadata retrieval

- `MediaLibrary`: Unified interface combining Radarr + Sonarr
  - Lazy-loads and caches movies/series/anime
  - `get_media_summary()`: Generates markdown table of top-rated content for LLM context
  - `search_by_theme()`: Keyword-based search in genres/overviews
- `RadarrClient`: Fetches movies via REST API
- `SonarrClient`: Fetches TV shows and anime
- Uses httpx for async-capable HTTP with 30s timeout
- Implements context manager pattern for resource cleanup

**[agent.py](program_director/agent.py)** - LLM orchestration

- `PlaylistAgent`: Main workflow coordinator
  - Initializes ChatOllama with 8192 token context window
  - `generate_playlist()`: LLM-powered selection based on theme
  - `get_selected_titles()`: Extracts titles from LLM suggestion
  - `apply_playlist()`: Creates/updates Custom Show (idempotent)
- `PlaylistSuggestion`: Pydantic model for LLM JSON output
- System prompt instructs LLM to curate with specific criteria (7.0+ ratings, variety, duration targets)
- Temperature: 0.7 for balanced creativity

**[tunarr_client.py](program_director/tunarr_client.py)** - Tunarr API

- REST API client for Custom Shows
- Methods: `get_custom_shows()`, `get_custom_show()`, `create_custom_show()`, `update_custom_show()`, `delete_custom_show()`
- Custom Shows store programs (movies, episodes, tracks) for channel scheduling
- Uses httpx with 30s timeout and context manager pattern

**[cli.py](program_director/cli.py)** - Command-line interface

- Typer-based CLI with three commands: `generate`, `scan`, `themes`
- Rich formatting for tables and colored output
- Validates required API keys and themes
- Handles resource cleanup with `agent.close()`

## Key Implementation Details

### LLM Prompting Strategy

The agent provides the LLM with:

1. Markdown table of top 100 movies / 50 shows / 50 anime (with ratings, genres, runtime)
2. Genre statistics for context
3. Theme name, description, keywords, and target duration
4. Specific instructions: prefer 7.0+ ratings, ensure variety, match duration target
5. Enforces JSON schema output with `JsonOutputParser`

### Idempotent Custom Show Creation

The agent checks if a custom show with the same name exists:

- If exists: updates the existing custom show
- If not: creates a new custom show
- This ensures re-running the same theme updates rather than duplicates

### Configuration Priority

1. Config file (`config.yaml`) provides base values
2. Environment variables override file values
3. Pydantic provides defaults where appropriate

Required environment variables:

- `RADARR_API_KEY` - Required for Radarr access
- `SONARR_API_KEY` - Required for Sonarr access

Optional overrides:

- `OLLAMA_URL`, `OLLAMA_MODEL`
- `TUNARR_URL`, `RADARR_URL`, `SONARR_URL`

### Resource Management

All API clients implement proper cleanup:

- `MediaLibrary` uses context manager (`__enter__`/`__exit__`)
- httpx clients properly closed
- `PlaylistAgent.close()` ensures all resources released

## Code Style

- **Line length**: 100 characters
- **Formatting**: Ruff (replaces black/isort)
- **Type checking**: MyPy strict mode
- **Linting rules**: E, F, I, N, W, UP, B
- **Python version**: >=3.11 (uses modern type hints)

## Testing

- Framework: pytest with pytest-asyncio
- Currently no tests in repository
- When adding tests, place in `tests/` directory at root level

## CI/CD

GitHub Actions workflow (`.github/workflows/docker.yml`):

- **Build**: Multi-platform Docker images (linux/amd64, linux/arm64)
- **Lint**: Ruff check and format validation
- **Registry**: GHCR (ghcr.io/geekxflood/program-director)
- **Tags**: `latest`, semver (`1.0.0`, `1.0`, `1`), SHA, PR numbers

## External Dependencies

**Required Services**:

- **Ollama**: Local LLM runtime (default model: `dolphin-llama3:8b`)
- **Radarr**: Movie library manager (API key required)
- **Sonarr**: TV/anime library manager (API key required)
- **Tunarr**: IPTV server for custom channels

**Key Libraries**:

- `langchain>=0.3.0` - LLM orchestration framework
- `langchain-ollama>=0.2.0` - Ollama integration
- `httpx>=0.27.0` - Modern HTTP client
- `pydantic>=2.0` - Data validation
- `typer>=0.12.0` - CLI framework
- `rich>=13.0` - Terminal formatting

## Important Constraints

1. **API timeouts**: 30 seconds for Radarr/Sonarr requests
2. **LLM context window**: 8192 tokens
3. **Docker user**: Non-root user `program-director` (UID 1000)
4. **Radarr/Sonarr**: Only fetches media with files (excludes monitored-but-missing)

## Common Patterns

### Adding a New Theme

Edit `config.yaml`:

```yaml
themes:
  - name: "theme-name"      # Use kebab-case, max 50 chars
    description: "Theme description for LLM"
    duration: 180           # Target runtime in minutes
    keywords:               # For genre/overview matching
      - "Keyword1"
      - "Keyword2"
```

### Debugging LLM Output

Use `--dry-run` to preview LLM suggestions without applying:

```bash
program-director generate --theme sci-fi-night --dry-run
```

### Modifying LLM Behavior

Edit system prompt in [agent.py](program_director/agent.py:74-107):

- Selection criteria (ratings, variety, runtime)
- Output format requirements
- Context information provided to LLM
