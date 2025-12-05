package models

import (
	"encoding/json"
	"time"
)

// MediaType represents the type of media
type MediaType string

const (
	MediaTypeMovie  MediaType = "movie"
	MediaTypeSeries MediaType = "series"
	MediaTypeAnime  MediaType = "anime"
)

// MediaSource represents where the media metadata came from
type MediaSource string

const (
	MediaSourceRadarr MediaSource = "radarr"
	MediaSourceSonarr MediaSource = "sonarr"
)

// Media represents a media item in the local catalog
type Media struct {
	ID         int64       `json:"id" db:"id"`
	ExternalID int64       `json:"external_id" db:"external_id"` // ID in source system (Radarr/Sonarr)
	Source     MediaSource `json:"source" db:"source"`
	MediaType  MediaType   `json:"media_type" db:"media_type"`

	// Basic metadata
	Title    string  `json:"title" db:"title"`
	Year     int     `json:"year" db:"year"`
	Overview string  `json:"overview" db:"overview"`
	Runtime  int     `json:"runtime" db:"runtime"` // in minutes

	// Genres stored as JSON array
	Genres     StringSlice `json:"genres" db:"genres"`

	// Ratings
	IMDBRating  float64 `json:"imdb_rating" db:"imdb_rating"`
	TMDBRating  float64 `json:"tmdb_rating" db:"tmdb_rating"`
	Popularity  float64 `json:"popularity" db:"popularity"`

	// External IDs
	IMDBID string `json:"imdb_id" db:"imdb_id"`
	TMDBID int64  `json:"tmdb_id" db:"tmdb_id"`
	TVDBID int64  `json:"tvdb_id" db:"tvdb_id"`

	// File info
	Path       string `json:"path" db:"path"`
	HasFile    bool   `json:"has_file" db:"has_file"`
	SizeOnDisk int64  `json:"size_on_disk" db:"size_on_disk"`

	// Status
	Status    string `json:"status" db:"status"`
	Monitored bool   `json:"monitored" db:"monitored"`

	// Timestamps
	SyncedAt  time.Time `json:"synced_at" db:"synced_at"`
	CreatedAt time.Time `json:"created_at" db:"created_at"`
	UpdatedAt time.Time `json:"updated_at" db:"updated_at"`
}

// StringSlice is a helper type for JSON arrays in the database
type StringSlice []string

// Scan implements sql.Scanner for StringSlice
func (s *StringSlice) Scan(src interface{}) error {
	if src == nil {
		*s = nil
		return nil
	}

	var data []byte
	switch v := src.(type) {
	case []byte:
		data = v
	case string:
		data = []byte(v)
	}

	return json.Unmarshal(data, s)
}

// Value implements driver.Valuer for StringSlice
func (s StringSlice) Value() (interface{}, error) {
	if s == nil {
		return nil, nil
	}
	return json.Marshal(s)
}

// PlayHistory represents a record of when media was played
type PlayHistory struct {
	ID        int64     `json:"id" db:"id"`
	MediaID   int64     `json:"media_id" db:"media_id"`
	ChannelID string    `json:"channel_id" db:"channel_id"`
	ThemeName string    `json:"theme_name" db:"theme_name"`
	PlayedAt  time.Time `json:"played_at" db:"played_at"`

	// Denormalized for easy querying
	MediaTitle string    `json:"media_title" db:"media_title"`
	MediaType  MediaType `json:"media_type" db:"media_type"`
}

// MediaCooldown tracks when media can be replayed
type MediaCooldown struct {
	ID            int64     `json:"id" db:"id"`
	MediaID       int64     `json:"media_id" db:"media_id"`
	CooldownDays  int       `json:"cooldown_days" db:"cooldown_days"`
	LastPlayedAt  time.Time `json:"last_played_at" db:"last_played_at"`
	CanReplayAt   time.Time `json:"can_replay_at" db:"can_replay_at"`

	// Denormalized
	MediaTitle string    `json:"media_title" db:"media_title"`
	MediaType  MediaType `json:"media_type" db:"media_type"`
}

// IsOnCooldown returns true if the media is still on cooldown
func (c *MediaCooldown) IsOnCooldown() bool {
	return time.Now().Before(c.CanReplayAt)
}

// DaysRemaining returns the number of days until cooldown expires
func (c *MediaCooldown) DaysRemaining() int {
	if !c.IsOnCooldown() {
		return 0
	}
	remaining := time.Until(c.CanReplayAt)
	return int(remaining.Hours() / 24)
}

// MediaWithScore represents media with a similarity/relevance score
type MediaWithScore struct {
	Media
	Score       float64  `json:"score"`
	MatchReason string   `json:"match_reason"`
}

// Channel represents a Tunarr channel
type Channel struct {
	ID             string `json:"id"`
	Number         int    `json:"number"`
	Name           string `json:"name"`
	Icon           string `json:"icon"`
	GroupTitle     string `json:"groupTitle"`
	ProgramCount   int    `json:"programCount"`
	Duration       int64  `json:"duration"`
}

// Program represents a program in a Tunarr channel lineup
type Program struct {
	ID           string    `json:"id"`
	Title        string    `json:"title"`
	Duration     int64     `json:"duration"` // milliseconds
	Type         string    `json:"type"`     // content, flex, redirect
	SourceType   string    `json:"sourceType"`
	ExternalID   string    `json:"externalId"`
	ScheduleTime time.Time `json:"scheduleTime"`
}

// Playlist represents a generated playlist
type Playlist struct {
	ThemeName   string           `json:"theme_name"`
	ChannelID   string           `json:"channel_id"`
	GeneratedAt time.Time        `json:"generated_at"`
	Items       []MediaWithScore `json:"items"`
	TotalScore  float64          `json:"total_score"`
	Duration    int              `json:"duration"` // Total duration in minutes
}
