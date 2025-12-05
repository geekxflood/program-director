package sonarr

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/geekxflood/program-director/internal/config"
	"github.com/geekxflood/program-director/pkg/models"
)

// Client is a Sonarr API client
type Client struct {
	baseURL    string
	apiKey     string
	httpClient *http.Client
}

// New creates a new Sonarr client
func New(cfg *config.SonarrConfig) *Client {
	return &Client{
		baseURL: cfg.URL,
		apiKey:  cfg.APIKey,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// Series represents a series from Sonarr API
type Series struct {
	ID             int64    `json:"id"`
	Title          string   `json:"title"`
	Year           int      `json:"year"`
	Overview       string   `json:"overview"`
	Runtime        int      `json:"runtime"`
	Genres         []string `json:"genres"`
	Status         string   `json:"status"`
	Monitored      bool     `json:"monitored"`
	Path           string   `json:"path"`
	SeriesType     string   `json:"seriesType"` // standard, anime, daily
	TVDBID         int64    `json:"tvdbId"`
	IMDBID         string   `json:"imdbId"`
	Ratings        Ratings  `json:"ratings"`
	Statistics     Stats    `json:"statistics"`
}

// Ratings holds rating information
type Ratings struct {
	Value float64 `json:"value"`
	Votes int64   `json:"votes"`
}

// Stats holds series statistics
type Stats struct {
	SeasonCount       int   `json:"seasonCount"`
	EpisodeCount      int   `json:"episodeCount"`
	EpisodeFileCount  int   `json:"episodeFileCount"`
	TotalEpisodeCount int   `json:"totalEpisodeCount"`
	SizeOnDisk        int64 `json:"sizeOnDisk"`
	PercentOfEpisodes float64 `json:"percentOfEpisodes"`
}

// GetSeries retrieves all series from Sonarr
func (c *Client) GetSeries(ctx context.Context) ([]Series, error) {
	req, err := c.newRequest(ctx, "GET", "/api/v3/series", nil)
	if err != nil {
		return nil, err
	}

	var series []Series
	if err := c.do(req, &series); err != nil {
		return nil, fmt.Errorf("failed to get series: %w", err)
	}

	return series, nil
}

// GetSeriesByID retrieves a single series by ID
func (c *Client) GetSeriesByID(ctx context.Context, id int64) (*Series, error) {
	req, err := c.newRequest(ctx, "GET", fmt.Sprintf("/api/v3/series/%d", id), nil)
	if err != nil {
		return nil, err
	}

	var series Series
	if err := c.do(req, &series); err != nil {
		return nil, fmt.Errorf("failed to get series %d: %w", id, err)
	}

	return &series, nil
}

// HealthCheck verifies the Sonarr connection
func (c *Client) HealthCheck(ctx context.Context) error {
	req, err := c.newRequest(ctx, "GET", "/api/v3/system/status", nil)
	if err != nil {
		return err
	}

	var status map[string]interface{}
	if err := c.do(req, &status); err != nil {
		return fmt.Errorf("sonarr health check failed: %w", err)
	}

	return nil
}

// ToMedia converts a Sonarr series to a Media model
func (s *Series) ToMedia() *models.Media {
	// Determine media type based on series type
	mediaType := models.MediaTypeSeries
	if s.SeriesType == "anime" || isAnime(s.Genres) {
		mediaType = models.MediaTypeAnime
	}

	return &models.Media{
		ExternalID: s.ID,
		Source:     models.MediaSourceSonarr,
		MediaType:  mediaType,
		Title:      s.Title,
		Year:       s.Year,
		Overview:   s.Overview,
		Runtime:    s.Runtime,
		Genres:     models.StringSlice(s.Genres),
		IMDBRating: s.Ratings.Value,
		TMDBRating: 0, // Sonarr doesn't provide TMDB rating directly
		IMDBID:     s.IMDBID,
		TVDBID:     s.TVDBID,
		Path:       s.Path,
		HasFile:    s.Statistics.EpisodeFileCount > 0,
		SizeOnDisk: s.Statistics.SizeOnDisk,
		Status:     s.Status,
		Monitored:  s.Monitored,
	}
}

// isAnime checks if the genres indicate anime content
func isAnime(genres []string) bool {
	for _, g := range genres {
		g = strings.ToLower(g)
		if g == "anime" || g == "animation" && containsJapanese(genres) {
			return true
		}
	}
	return false
}

// containsJapanese checks if genres suggest Japanese content
func containsJapanese(genres []string) bool {
	for _, g := range genres {
		g = strings.ToLower(g)
		if strings.Contains(g, "japanese") || strings.Contains(g, "japan") {
			return true
		}
	}
	return false
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
