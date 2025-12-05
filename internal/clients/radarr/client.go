package radarr

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/geekxflood/program-director/internal/config"
	"github.com/geekxflood/program-director/pkg/models"
)

// Client is a Radarr API client
type Client struct {
	baseURL    string
	apiKey     string
	httpClient *http.Client
}

// New creates a new Radarr client
func New(cfg *config.RadarrConfig) *Client {
	return &Client{
		baseURL: cfg.URL,
		apiKey:  cfg.APIKey,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// Movie represents a movie from Radarr API
type Movie struct {
	ID            int64    `json:"id"`
	Title         string   `json:"title"`
	Year          int      `json:"year"`
	Overview      string   `json:"overview"`
	Runtime       int      `json:"runtime"`
	Genres        []string `json:"genres"`
	Status        string   `json:"status"`
	Monitored     bool     `json:"monitored"`
	Path          string   `json:"path"`
	HasFile       bool     `json:"hasFile"`
	SizeOnDisk    int64    `json:"sizeOnDisk"`
	IMDBID        string   `json:"imdbId"`
	TMDBID        int64    `json:"tmdbId"`
	Ratings       Ratings  `json:"ratings"`
	MovieFile     *MovieFile `json:"movieFile,omitempty"`
	Popularity    float64  `json:"popularity"`
}

// Ratings holds rating information
type Ratings struct {
	IMDB    Rating `json:"imdb"`
	TMDB    Rating `json:"tmdb"`
	RottenTomatoes Rating `json:"rottenTomatoes"`
}

// Rating holds individual rating values
type Rating struct {
	Value float64 `json:"value"`
	Votes int64   `json:"votes"`
}

// MovieFile holds movie file information
type MovieFile struct {
	ID       int64  `json:"id"`
	Path     string `json:"path"`
	Size     int64  `json:"size"`
	Quality  Quality `json:"quality"`
}

// Quality holds quality information
type Quality struct {
	Quality QualityInfo `json:"quality"`
}

// QualityInfo holds quality details
type QualityInfo struct {
	ID         int    `json:"id"`
	Name       string `json:"name"`
	Resolution int    `json:"resolution"`
}

// GetMovies retrieves all movies from Radarr
func (c *Client) GetMovies(ctx context.Context) ([]Movie, error) {
	req, err := c.newRequest(ctx, "GET", "/api/v3/movie", nil)
	if err != nil {
		return nil, err
	}

	var movies []Movie
	if err := c.do(req, &movies); err != nil {
		return nil, fmt.Errorf("failed to get movies: %w", err)
	}

	return movies, nil
}

// GetMovie retrieves a single movie by ID
func (c *Client) GetMovie(ctx context.Context, id int64) (*Movie, error) {
	req, err := c.newRequest(ctx, "GET", fmt.Sprintf("/api/v3/movie/%d", id), nil)
	if err != nil {
		return nil, err
	}

	var movie Movie
	if err := c.do(req, &movie); err != nil {
		return nil, fmt.Errorf("failed to get movie %d: %w", id, err)
	}

	return &movie, nil
}

// HealthCheck verifies the Radarr connection
func (c *Client) HealthCheck(ctx context.Context) error {
	req, err := c.newRequest(ctx, "GET", "/api/v3/system/status", nil)
	if err != nil {
		return err
	}

	var status map[string]interface{}
	if err := c.do(req, &status); err != nil {
		return fmt.Errorf("radarr health check failed: %w", err)
	}

	return nil
}

// ToMedia converts a Radarr movie to a Media model
func (m *Movie) ToMedia() *models.Media {
	return &models.Media{
		ExternalID: m.ID,
		Source:     models.MediaSourceRadarr,
		MediaType:  models.MediaTypeMovie,
		Title:      m.Title,
		Year:       m.Year,
		Overview:   m.Overview,
		Runtime:    m.Runtime,
		Genres:     models.StringSlice(m.Genres),
		IMDBRating: m.Ratings.IMDB.Value,
		TMDBRating: m.Ratings.TMDB.Value,
		Popularity: m.Popularity,
		IMDBID:     m.IMDBID,
		TMDBID:     m.TMDBID,
		Path:       m.Path,
		HasFile:    m.HasFile,
		SizeOnDisk: m.SizeOnDisk,
		Status:     m.Status,
		Monitored:  m.Monitored,
	}
}

// newRequest creates a new HTTP request with API key header
func (c *Client) newRequest(ctx context.Context, method, path string, body io.Reader) (*http.Request, error) {
	u, err := url.Parse(c.baseURL + path)
	if err != nil {
		return nil, fmt.Errorf("invalid URL: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, method, u.String(), body)
	if err != nil {
		return nil, err
	}

	req.Header.Set("X-Api-Key", c.apiKey)
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
