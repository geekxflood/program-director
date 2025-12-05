package tunarr

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/geekxflood/program-director/internal/config"
)

// Client is a Tunarr API client
type Client struct {
	baseURL    string
	httpClient *http.Client
}

// New creates a new Tunarr client
func New(cfg *config.TunarrConfig) *Client {
	return &Client{
		baseURL: cfg.URL,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// Channel represents a Tunarr channel
type Channel struct {
	ID             string         `json:"id"`
	Number         int            `json:"number"`
	Name           string         `json:"name"`
	Icon           ChannelIcon    `json:"icon"`
	GroupTitle     string         `json:"groupTitle"`
	ProgramCount   int            `json:"programCount"`
	Duration       int64          `json:"duration"`
	StreamerSource string         `json:"steamerSource"`
}

// ChannelIcon holds channel icon information
type ChannelIcon struct {
	Path    string `json:"path"`
	Width   int    `json:"width"`
	Height  int    `json:"height"`
}

// Program represents a program in a channel lineup
type Program struct {
	ID           string `json:"id,omitempty"`
	Type         string `json:"type"`          // content, flex, redirect
	Duration     int64  `json:"duration"`      // milliseconds
	PersistTime  bool   `json:"persistTime,omitempty"`

	// For content type
	ExternalSourceType string       `json:"externalSourceType,omitempty"` // plex, jellyfin
	ExternalSourceName string       `json:"externalSourceName,omitempty"`
	ExternalKey        string       `json:"externalKey,omitempty"`
	PlexFilePath       string       `json:"plexFilePath,omitempty"`

	// Additional metadata
	Title       string `json:"title,omitempty"`
	Summary     string `json:"summary,omitempty"`
	Rating      string `json:"rating,omitempty"`
	Year        int    `json:"year,omitempty"`
}

// Programming represents the programming lineup for a channel
type Programming struct {
	Type     string    `json:"type"` // manual, random, etc.
	Programs []Program `json:"programs"`
}

// MediaSource represents a media source (Plex/Jellyfin)
type MediaSource struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	Type         string `json:"type"` // plex, jellyfin
	URI          string `json:"uri"`
	AccessToken  string `json:"accessToken,omitempty"`
}

// PlexLibrary represents a Plex library
type PlexLibrary struct {
	Key       string `json:"key"`
	Type      string `json:"type"`
	Title     string `json:"title"`
	UUID      string `json:"uuid"`
}

// PlexMedia represents media from Plex
type PlexMedia struct {
	RatingKey    string `json:"ratingKey"`
	Key          string `json:"key"`
	Type         string `json:"type"` // movie, episode
	Title        string `json:"title"`
	Summary      string `json:"summary"`
	Year         int    `json:"year"`
	Duration     int64  `json:"duration"` // milliseconds
	ContentRating string `json:"contentRating"`
}

// GetChannels retrieves all channels
func (c *Client) GetChannels(ctx context.Context) ([]Channel, error) {
	req, err := c.newRequest(ctx, "GET", "/api/channels", nil)
	if err != nil {
		return nil, err
	}

	var channels []Channel
	if err := c.do(req, &channels); err != nil {
		return nil, fmt.Errorf("failed to get channels: %w", err)
	}

	return channels, nil
}

// GetChannel retrieves a single channel by ID
func (c *Client) GetChannel(ctx context.Context, id string) (*Channel, error) {
	req, err := c.newRequest(ctx, "GET", fmt.Sprintf("/api/channels/%s", id), nil)
	if err != nil {
		return nil, err
	}

	var channel Channel
	if err := c.do(req, &channel); err != nil {
		return nil, fmt.Errorf("failed to get channel %s: %w", id, err)
	}

	return &channel, nil
}

// GetProgramming retrieves the programming for a channel
func (c *Client) GetProgramming(ctx context.Context, channelID string) (*Programming, error) {
	req, err := c.newRequest(ctx, "GET", fmt.Sprintf("/api/channels/%s/programming", channelID), nil)
	if err != nil {
		return nil, err
	}

	var programming Programming
	if err := c.do(req, &programming); err != nil {
		return nil, fmt.Errorf("failed to get programming for channel %s: %w", channelID, err)
	}

	return &programming, nil
}

// SetProgramming sets the programming for a channel
func (c *Client) SetProgramming(ctx context.Context, channelID string, programming *Programming) error {
	body, err := json.Marshal(programming)
	if err != nil {
		return fmt.Errorf("failed to marshal programming: %w", err)
	}

	req, err := c.newRequest(ctx, "POST", fmt.Sprintf("/api/channels/%s/programming", channelID), bytes.NewReader(body))
	if err != nil {
		return err
	}

	if err := c.do(req, nil); err != nil {
		return fmt.Errorf("failed to set programming for channel %s: %w", channelID, err)
	}

	return nil
}

// GetMediaSources retrieves all configured media sources
func (c *Client) GetMediaSources(ctx context.Context) ([]MediaSource, error) {
	req, err := c.newRequest(ctx, "GET", "/api/media-sources", nil)
	if err != nil {
		return nil, err
	}

	var sources []MediaSource
	if err := c.do(req, &sources); err != nil {
		return nil, fmt.Errorf("failed to get media sources: %w", err)
	}

	return sources, nil
}

// GetPlexLibraries retrieves libraries from a Plex media source
func (c *Client) GetPlexLibraries(ctx context.Context, sourceID string) ([]PlexLibrary, error) {
	req, err := c.newRequest(ctx, "GET", fmt.Sprintf("/api/plex/%s/libraries", sourceID), nil)
	if err != nil {
		return nil, err
	}

	var libraries []PlexLibrary
	if err := c.do(req, &libraries); err != nil {
		return nil, fmt.Errorf("failed to get Plex libraries: %w", err)
	}

	return libraries, nil
}

// GetPlexLibraryMedia retrieves media from a Plex library
func (c *Client) GetPlexLibraryMedia(ctx context.Context, sourceID, libraryKey string) ([]PlexMedia, error) {
	req, err := c.newRequest(ctx, "GET", fmt.Sprintf("/api/plex/%s/libraries/%s/media", sourceID, libraryKey), nil)
	if err != nil {
		return nil, err
	}

	var media []PlexMedia
	if err := c.do(req, &media); err != nil {
		return nil, fmt.Errorf("failed to get Plex library media: %w", err)
	}

	return media, nil
}

// SearchPlexMedia searches for media in Plex
func (c *Client) SearchPlexMedia(ctx context.Context, sourceID, query string) ([]PlexMedia, error) {
	path := fmt.Sprintf("/api/plex/%s/search?query=%s", sourceID, url.QueryEscape(query))
	req, err := c.newRequest(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}

	var media []PlexMedia
	if err := c.do(req, &media); err != nil {
		return nil, fmt.Errorf("failed to search Plex media: %w", err)
	}

	return media, nil
}

// HealthCheck verifies the Tunarr connection
func (c *Client) HealthCheck(ctx context.Context) error {
	req, err := c.newRequest(ctx, "GET", "/api/version", nil)
	if err != nil {
		return err
	}

	var version map[string]interface{}
	if err := c.do(req, &version); err != nil {
		return fmt.Errorf("tunarr health check failed: %w", err)
	}

	return nil
}

// newRequest creates a new HTTP request
func (c *Client) newRequest(ctx context.Context, method, path string, body io.Reader) (*http.Request, error) {
	u, err := url.Parse(c.baseURL + path)
	if err != nil {
		return nil, fmt.Errorf("invalid URL: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, method, u.String(), body)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")

	return req, nil
}

// do executes an HTTP request and decodes the JSON response
func (c *Client) do(req *http.Request, v interface{}) error {
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("API error: status %d, body: %s", resp.StatusCode, string(body))
	}

	if v != nil {
		if err := json.NewDecoder(resp.Body).Decode(v); err != nil {
			return fmt.Errorf("failed to decode response: %w", err)
		}
	}

	return nil
}
