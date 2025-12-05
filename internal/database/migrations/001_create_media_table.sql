-- Media catalog table
CREATE TABLE IF NOT EXISTS media (
    id BIGSERIAL PRIMARY KEY,
    external_id BIGINT NOT NULL,
    source TEXT NOT NULL,
    media_type TEXT NOT NULL,

    -- Basic metadata
    title TEXT NOT NULL,
    year INTEGER,
    overview TEXT,
    runtime INTEGER,

    -- Genres as JSON array
    genres JSONB DEFAULT '[]',

    -- Ratings
    imdb_rating REAL DEFAULT 0,
    tmdb_rating REAL DEFAULT 0,
    popularity REAL DEFAULT 0,

    -- External IDs
    imdb_id TEXT,
    tmdb_id BIGINT,
    tvdb_id BIGINT,

    -- File info
    path TEXT,
    has_file BOOLEAN DEFAULT FALSE,
    size_on_disk BIGINT DEFAULT 0,

    -- Status
    status TEXT,
    monitored BOOLEAN DEFAULT TRUE,

    -- Timestamps
    synced_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Indexes for common queries
CREATE INDEX IF NOT EXISTS idx_media_source ON media(source);
CREATE INDEX IF NOT EXISTS idx_media_media_type ON media(media_type);
CREATE INDEX IF NOT EXISTS idx_media_external_id ON media(external_id, source);
CREATE INDEX IF NOT EXISTS idx_media_title ON media(title);
CREATE INDEX IF NOT EXISTS idx_media_year ON media(year);
CREATE INDEX IF NOT EXISTS idx_media_imdb_id ON media(imdb_id);
CREATE INDEX IF NOT EXISTS idx_media_has_file ON media(has_file);

-- Unique constraint on external_id + source
CREATE UNIQUE INDEX IF NOT EXISTS idx_media_unique ON media(external_id, source);
