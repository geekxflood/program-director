-- Media cooldowns table
CREATE TABLE IF NOT EXISTS media_cooldowns (
    id BIGSERIAL PRIMARY KEY,
    media_id BIGINT NOT NULL REFERENCES media(id) ON DELETE CASCADE,
    cooldown_days INTEGER NOT NULL,
    last_played_at TIMESTAMP NOT NULL,
    can_replay_at TIMESTAMP NOT NULL,

    -- Denormalized for easy querying
    media_title TEXT NOT NULL,
    media_type TEXT NOT NULL,

    -- Only one cooldown per media
    UNIQUE(media_id)
);

-- Indexes for common queries
CREATE INDEX IF NOT EXISTS idx_media_cooldowns_can_replay_at ON media_cooldowns(can_replay_at);
CREATE INDEX IF NOT EXISTS idx_media_cooldowns_media_type ON media_cooldowns(media_type);

-- Index for finding active cooldowns
CREATE INDEX IF NOT EXISTS idx_media_cooldowns_active ON media_cooldowns(can_replay_at)
    WHERE can_replay_at > CURRENT_TIMESTAMP;
