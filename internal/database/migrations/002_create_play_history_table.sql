-- Play history table
CREATE TABLE IF NOT EXISTS play_history (
    id BIGSERIAL PRIMARY KEY,
    media_id BIGINT NOT NULL REFERENCES media(id) ON DELETE CASCADE,
    channel_id TEXT NOT NULL,
    theme_name TEXT NOT NULL,
    played_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,

    -- Denormalized for easy querying
    media_title TEXT NOT NULL,
    media_type TEXT NOT NULL
);

-- Indexes for common queries
CREATE INDEX IF NOT EXISTS idx_play_history_media_id ON play_history(media_id);
CREATE INDEX IF NOT EXISTS idx_play_history_channel_id ON play_history(channel_id);
CREATE INDEX IF NOT EXISTS idx_play_history_theme_name ON play_history(theme_name);
CREATE INDEX IF NOT EXISTS idx_play_history_played_at ON play_history(played_at);

-- Index for finding recent plays for a specific media
CREATE INDEX IF NOT EXISTS idx_play_history_media_played ON play_history(media_id, played_at DESC);
