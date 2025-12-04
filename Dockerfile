FROM python:3.12-slim

WORKDIR /app

# Copy all project files (pyproject.toml references README.md)
COPY pyproject.toml README.md LICENSE ./
COPY playlist_agent/ playlist_agent/

# Install dependencies
RUN pip install --no-cache-dir .

# Create non-root user
RUN useradd -r -u 1000 playlist-agent && \
    chown -R playlist-agent:playlist-agent /app

USER playlist-agent

# Default command
CMD ["python", "-m", "playlist_agent.cli", "--help"]
