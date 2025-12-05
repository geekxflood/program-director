package ollama

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

// Client is an Ollama API client
type Client struct {
	baseURL     string
	model       string
	temperature float64
	numCtx      int
	httpClient  *http.Client
}

// New creates a new Ollama client
func New(cfg *config.OllamaConfig) *Client {
	return &Client{
		baseURL:     cfg.URL,
		model:       cfg.Model,
		temperature: cfg.Temperature,
		numCtx:      cfg.NumCtx,
		httpClient: &http.Client{
			Timeout: 5 * time.Minute, // LLM requests can take a while
		},
	}
}

// ChatRequest represents a chat completion request
type ChatRequest struct {
	Model       string        `json:"model"`
	Messages    []ChatMessage `json:"messages"`
	Stream      bool          `json:"stream"`
	Options     Options       `json:"options,omitempty"`
	Format      string        `json:"format,omitempty"` // "json" for JSON output
}

// ChatMessage represents a message in the conversation
type ChatMessage struct {
	Role    string `json:"role"` // system, user, assistant
	Content string `json:"content"`
}

// Options holds model options
type Options struct {
	Temperature float64 `json:"temperature,omitempty"`
	NumCtx      int     `json:"num_ctx,omitempty"`
	NumPredict  int     `json:"num_predict,omitempty"`
	TopK        int     `json:"top_k,omitempty"`
	TopP        float64 `json:"top_p,omitempty"`
}

// ChatResponse represents the response from chat completion
type ChatResponse struct {
	Model     string      `json:"model"`
	CreatedAt string      `json:"created_at"`
	Message   ChatMessage `json:"message"`
	Done      bool        `json:"done"`
	TotalDuration  int64 `json:"total_duration"`
	LoadDuration   int64 `json:"load_duration"`
	PromptEvalCount int  `json:"prompt_eval_count"`
	EvalCount       int  `json:"eval_count"`
	EvalDuration    int64 `json:"eval_duration"`
}

// GenerateRequest represents a text generation request
type GenerateRequest struct {
	Model    string  `json:"model"`
	Prompt   string  `json:"prompt"`
	System   string  `json:"system,omitempty"`
	Stream   bool    `json:"stream"`
	Options  Options `json:"options,omitempty"`
	Format   string  `json:"format,omitempty"`
}

// GenerateResponse represents the response from text generation
type GenerateResponse struct {
	Model     string `json:"model"`
	CreatedAt string `json:"created_at"`
	Response  string `json:"response"`
	Done      bool   `json:"done"`
}

// Chat performs a chat completion request
func (c *Client) Chat(ctx context.Context, messages []ChatMessage) (*ChatResponse, error) {
	req := ChatRequest{
		Model:    c.model,
		Messages: messages,
		Stream:   false,
		Options: Options{
			Temperature: c.temperature,
			NumCtx:      c.numCtx,
		},
	}

	return c.doChat(ctx, &req)
}

// ChatWithJSON performs a chat completion request expecting JSON output
func (c *Client) ChatWithJSON(ctx context.Context, messages []ChatMessage) (*ChatResponse, error) {
	req := ChatRequest{
		Model:    c.model,
		Messages: messages,
		Stream:   false,
		Format:   "json",
		Options: Options{
			Temperature: c.temperature,
			NumCtx:      c.numCtx,
		},
	}

	return c.doChat(ctx, &req)
}

// Generate performs a text generation request
func (c *Client) Generate(ctx context.Context, prompt string, system string) (*GenerateResponse, error) {
	req := GenerateRequest{
		Model:  c.model,
		Prompt: prompt,
		System: system,
		Stream: false,
		Options: Options{
			Temperature: c.temperature,
			NumCtx:      c.numCtx,
		},
	}

	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := c.newRequest(ctx, "POST", "/api/generate", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}

	var resp GenerateResponse
	if err := c.do(httpReq, &resp); err != nil {
		return nil, fmt.Errorf("failed to generate: %w", err)
	}

	return &resp, nil
}

// GenerateWithJSON performs a text generation request expecting JSON output
func (c *Client) GenerateWithJSON(ctx context.Context, prompt string, system string) (*GenerateResponse, error) {
	req := GenerateRequest{
		Model:  c.model,
		Prompt: prompt,
		System: system,
		Stream: false,
		Format: "json",
		Options: Options{
			Temperature: c.temperature,
			NumCtx:      c.numCtx,
		},
	}

	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := c.newRequest(ctx, "POST", "/api/generate", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}

	var resp GenerateResponse
	if err := c.do(httpReq, &resp); err != nil {
		return nil, fmt.Errorf("failed to generate: %w", err)
	}

	return &resp, nil
}

// ListModels lists available models
func (c *Client) ListModels(ctx context.Context) ([]ModelInfo, error) {
	req, err := c.newRequest(ctx, "GET", "/api/tags", nil)
	if err != nil {
		return nil, err
	}

	var resp struct {
		Models []ModelInfo `json:"models"`
	}
	if err := c.do(req, &resp); err != nil {
		return nil, fmt.Errorf("failed to list models: %w", err)
	}

	return resp.Models, nil
}

// ModelInfo represents information about a model
type ModelInfo struct {
	Name       string `json:"name"`
	ModifiedAt string `json:"modified_at"`
	Size       int64  `json:"size"`
	Digest     string `json:"digest"`
}

// HealthCheck verifies the Ollama connection and model availability
func (c *Client) HealthCheck(ctx context.Context) error {
	models, err := c.ListModels(ctx)
	if err != nil {
		return fmt.Errorf("ollama health check failed: %w", err)
	}

	// Check if configured model is available
	modelFound := false
	for _, m := range models {
		if m.Name == c.model {
			modelFound = true
			break
		}
	}

	if !modelFound {
		return fmt.Errorf("configured model %s not found in Ollama", c.model)
	}

	return nil
}

// doChat executes a chat completion request
func (c *Client) doChat(ctx context.Context, req *ChatRequest) (*ChatResponse, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := c.newRequest(ctx, "POST", "/api/chat", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}

	var resp ChatResponse
	if err := c.do(httpReq, &resp); err != nil {
		return nil, fmt.Errorf("failed to chat: %w", err)
	}

	return &resp, nil
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
