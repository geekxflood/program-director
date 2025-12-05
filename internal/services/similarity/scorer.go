package similarity

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sort"
	"strings"

	"github.com/geekxflood/program-director/internal/clients/ollama"
	"github.com/geekxflood/program-director/internal/config"
	"github.com/geekxflood/program-director/internal/database/repository"
	"github.com/geekxflood/program-director/pkg/models"
)

// Scorer handles content similarity scoring
type Scorer struct {
	mediaRepo *repository.MediaRepository
	ollama    *ollama.Client
	logger    *slog.Logger
}

// NewScorer creates a new Scorer
func NewScorer(
	mediaRepo *repository.MediaRepository,
	ollamaClient *ollama.Client,
	logger *slog.Logger,
) *Scorer {
	return &Scorer{
		mediaRepo: mediaRepo,
		ollama:    ollamaClient,
		logger:    logger,
	}
}

// FindCandidates finds media candidates matching a theme
func (s *Scorer) FindCandidates(ctx context.Context, theme *config.ThemeConfig, excludeIDs []int64) ([]models.MediaWithScore, error) {
	// Phase 1: Genre-based filtering
	candidates, err := s.filterByGenre(ctx, theme, excludeIDs)
	if err != nil {
		return nil, fmt.Errorf("genre filter failed: %w", err)
	}

	s.logger.Debug("genre filter results",
		"theme", theme.Name,
		"candidates", len(candidates),
	)

	if len(candidates) == 0 {
		return nil, nil
	}

	// Phase 2: LLM refinement on top candidates
	if len(candidates) > 20 && s.ollama != nil {
		refined, err := s.refinWithLLM(ctx, theme, candidates[:min(50, len(candidates))])
		if err != nil {
			s.logger.Warn("LLM refinement failed, using genre scores",
				"error", err,
			)
		} else {
			candidates = refined
		}
	}

	// Sort by score descending
	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].Score > candidates[j].Score
	})

	// Limit results
	maxItems := theme.MaxItems
	if maxItems == 0 {
		maxItems = 20
	}
	if len(candidates) > maxItems {
		candidates = candidates[:maxItems]
	}

	return candidates, nil
}

// filterByGenre performs initial filtering based on genre matching
func (s *Scorer) filterByGenre(ctx context.Context, theme *config.ThemeConfig, excludeIDs []int64) ([]models.MediaWithScore, error) {
	var mediaTypes []models.MediaType

	// Determine which media types to include
	for _, mt := range theme.MediaTypes {
		switch strings.ToLower(mt) {
		case "movie", "movies":
			mediaTypes = append(mediaTypes, models.MediaTypeMovie)
		case "series", "shows", "tv":
			mediaTypes = append(mediaTypes, models.MediaTypeSeries)
		case "anime":
			mediaTypes = append(mediaTypes, models.MediaTypeAnime)
		}
	}

	// If no specific types, include all
	if len(mediaTypes) == 0 {
		mediaTypes = []models.MediaType{models.MediaTypeMovie, models.MediaTypeSeries, models.MediaTypeAnime}
	}

	var candidates []models.MediaWithScore

	for _, mediaType := range mediaTypes {
		// Fetch media matching genres
		media, err := s.mediaRepo.ListByGenres(ctx, theme.Genres, mediaType, excludeIDs)
		if err != nil {
			return nil, err
		}

		for _, m := range media {
			// Skip if below minimum rating
			if theme.MinRating > 0 && m.IMDBRating < theme.MinRating {
				continue
			}

			// Calculate genre score
			score := s.calculateGenreScore(m.Genres, theme.Genres)

			// Add keyword bonus
			if len(theme.Keywords) > 0 {
				score += s.calculateKeywordScore(m.Title, m.Overview, theme.Keywords)
			}

			// Add rating bonus
			if m.IMDBRating > 0 {
				score += m.IMDBRating / 20 // Small bonus for highly rated content
			}

			candidates = append(candidates, models.MediaWithScore{
				Media:       m,
				Score:       score,
				MatchReason: fmt.Sprintf("Genre match: %.0f%%", score*100),
			})
		}
	}

	return candidates, nil
}

// calculateGenreScore calculates how well media genres match theme genres
func (s *Scorer) calculateGenreScore(mediaGenres models.StringSlice, themeGenres []string) float64 {
	if len(themeGenres) == 0 {
		return 0.5 // Neutral score if no genres specified
	}

	matches := 0
	for _, mg := range mediaGenres {
		mgLower := strings.ToLower(mg)
		for _, tg := range themeGenres {
			if strings.Contains(mgLower, strings.ToLower(tg)) ||
				strings.Contains(strings.ToLower(tg), mgLower) {
				matches++
				break
			}
		}
	}

	// Score based on percentage of theme genres matched
	return float64(matches) / float64(len(themeGenres))
}

// calculateKeywordScore calculates keyword match score
func (s *Scorer) calculateKeywordScore(title, overview string, keywords []string) float64 {
	if len(keywords) == 0 {
		return 0
	}

	text := strings.ToLower(title + " " + overview)
	matches := 0

	for _, kw := range keywords {
		if strings.Contains(text, strings.ToLower(kw)) {
			matches++
		}
	}

	return float64(matches) / float64(len(keywords)) * 0.3 // Max 30% bonus from keywords
}

// refinWithLLM uses the LLM to refine and score candidates
func (s *Scorer) refinWithLLM(ctx context.Context, theme *config.ThemeConfig, candidates []models.MediaWithScore) ([]models.MediaWithScore, error) {
	// Build media summary for LLM
	var mediaSummary strings.Builder
	mediaSummary.WriteString("Media candidates:\n")
	for i, c := range candidates {
		mediaSummary.WriteString(fmt.Sprintf("%d. \"%s\" (%d) - Genres: %s - Rating: %.1f\n",
			i+1, c.Title, c.Year, strings.Join(c.Genres, ", "), c.IMDBRating))
		if c.Overview != "" && len(c.Overview) > 200 {
			mediaSummary.WriteString(fmt.Sprintf("   %s...\n", c.Overview[:200]))
		} else if c.Overview != "" {
			mediaSummary.WriteString(fmt.Sprintf("   %s\n", c.Overview))
		}
	}

	systemPrompt := `You are a TV programming assistant that selects content for themed channels.
You must respond ONLY with valid JSON in this exact format:
{
  "rankings": [
    {"index": 1, "score": 0.95, "reason": "brief reason"},
    {"index": 2, "score": 0.80, "reason": "brief reason"}
  ]
}

Score each item from 0.0 to 1.0 based on how well it fits the theme.
Include ALL items in your rankings.
Only output JSON, no other text.`

	userPrompt := fmt.Sprintf(`Theme: %s
Description: %s
Target genres: %s
Keywords: %s

%s

Rank ALL items by how well they fit this theme. Output JSON only.`,
		theme.Name,
		theme.Description,
		strings.Join(theme.Genres, ", "),
		strings.Join(theme.Keywords, ", "),
		mediaSummary.String(),
	)

	messages := []ollama.ChatMessage{
		{Role: "system", Content: systemPrompt},
		{Role: "user", Content: userPrompt},
	}

	resp, err := s.ollama.ChatWithJSON(ctx, messages)
	if err != nil {
		return nil, err
	}

	// Parse LLM response
	var result struct {
		Rankings []struct {
			Index  int     `json:"index"`
			Score  float64 `json:"score"`
			Reason string  `json:"reason"`
		} `json:"rankings"`
	}

	if err := json.Unmarshal([]byte(resp.Message.Content), &result); err != nil {
		s.logger.Warn("failed to parse LLM response",
			"error", err,
			"response", resp.Message.Content,
		)
		return nil, err
	}

	// Update scores based on LLM rankings
	for _, ranking := range result.Rankings {
		idx := ranking.Index - 1 // Convert to 0-based
		if idx >= 0 && idx < len(candidates) {
			// Blend genre score with LLM score
			originalScore := candidates[idx].Score
			llmScore := ranking.Score
			candidates[idx].Score = (originalScore*0.3 + llmScore*0.7) // Weight LLM higher
			candidates[idx].MatchReason = ranking.Reason
		}
	}

	return candidates, nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
