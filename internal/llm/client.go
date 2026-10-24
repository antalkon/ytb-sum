// Package llm is a minimal client for OpenAI-compatible chat completion APIs
// (OpenRouter by default, also Ollama, LM Studio and similar).
package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// Message is a chat message. Content is a string or a []Part.
type Message struct {
	Role    string `json:"role"`
	Content any    `json:"content"`
}

// Part is one element of multimodal message content.
type Part struct {
	Type       string      `json:"type"`
	Text       string      `json:"text,omitempty"`
	InputAudio *InputAudio `json:"input_audio,omitempty"`
}

// InputAudio carries base64-encoded audio.
type InputAudio struct {
	Data   string `json:"data"`
	Format string `json:"format"`
}

// ResponseFormat requests structured JSON output.
type ResponseFormat struct {
	Type       string      `json:"type"`
	JSONSchema *JSONSchema `json:"json_schema,omitempty"`
}

// JSONSchema names a strict schema for ResponseFormat.
type JSONSchema struct {
	Name   string         `json:"name"`
	Strict bool           `json:"strict"`
	Schema map[string]any `json:"schema"`
}

// Request is a chat completion request.
type Request struct {
	Model          string          `json:"model"`
	Models         []string        `json:"models,omitempty"` // OpenRouter fallbacks
	Messages       []Message       `json:"messages"`
	ResponseFormat *ResponseFormat `json:"response_format,omitempty"`
	Temperature    *float64        `json:"temperature,omitempty"`
	MaxTokens      int             `json:"max_tokens,omitempty"`
	Provider       map[string]any  `json:"provider,omitempty"`
	Usage          map[string]any  `json:"usage,omitempty"`
}

// Usage reports token counts and, on OpenRouter, the cost in USD.
type Usage struct {
	PromptTokens     int     `json:"prompt_tokens"`
	CompletionTokens int     `json:"completion_tokens"`
	Cost             float64 `json:"cost"`
}

// Add accumulates u2 into u.
func (u *Usage) Add(u2 Usage) {
	u.PromptTokens += u2.PromptTokens
	u.CompletionTokens += u2.CompletionTokens
	u.Cost += u2.Cost
}

// Response is the parsed completion.
type Response struct {
	Model        string
	Content      string
	FinishReason string
	Usage        Usage
}

// APIError is a non-2xx response from the API.
type APIError struct {
	Status  int
	Message string
}

func (e *APIError) Error() string {
	return fmt.Sprintf("LLM API error %d: %s", e.Status, e.Message)
}

// Client talks to an OpenAI-compatible endpoint.
type Client struct {
	BaseURL    string
	APIKey     string
	HTTP       *http.Client
	MaxRetries int
	// IsOpenRouter enables OpenRouter-only request fields.
	IsOpenRouter bool
	sleep        func(context.Context, time.Duration) error
}

// New returns a Client with sensible retry defaults.
func New(baseURL, apiKey string, hc *http.Client) *Client {
	return &Client{
		BaseURL:      strings.TrimRight(baseURL, "/"),
		APIKey:       apiKey,
		HTTP:         hc,
		MaxRetries:   3,
		IsOpenRouter: strings.Contains(baseURL, "openrouter.ai"),
		sleep:        sleepCtx,
	}
}

// Chat sends req, retrying on rate limits and server errors.
func (c *Client) Chat(ctx context.Context, req Request) (*Response, error) {
	if c.IsOpenRouter {
		req.Usage = map[string]any{"include": true}
		if req.ResponseFormat != nil {
			req.Provider = map[string]any{"require_parameters": true}
		}
	} else {
		req.Models = nil
	}
	body, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}

	var lastErr error
	for attempt := 0; attempt <= c.MaxRetries; attempt++ {
		if attempt > 0 {
			if err := c.sleep(ctx, backoff(attempt, lastErr)); err != nil {
				return nil, err
			}
		}
		resp, err := c.do(ctx, body)
		if err == nil {
			return resp, nil
		}
		lastErr = err
		if !retryable(err) || ctx.Err() != nil {
			return nil, err
		}
	}
	return nil, lastErr
}

type retryAfterError struct {
	*APIError
	after time.Duration
}

func (c *Client) do(ctx context.Context, body []byte) (*Response, error) {
	hreq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.BaseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	hreq.Header.Set("Content-Type", "application/json")
	if c.APIKey != "" {
		hreq.Header.Set("Authorization", "Bearer "+c.APIKey)
	}
	if c.IsOpenRouter {
		hreq.Header.Set("X-Title", "ytb-sum")
	}
	hresp, err := c.HTTP.Do(hreq)
	if err != nil {
		return nil, err
	}
	defer hresp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(hresp.Body, 32<<20))
	if err != nil {
		return nil, err
	}

	var parsed struct {
		Model   string `json:"model"`
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
			FinishReason string `json:"finish_reason"`
		} `json:"choices"`
		Usage Usage `json:"usage"`
		Error *struct {
			Code    any    `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	jsonErr := json.Unmarshal(data, &parsed)

	if hresp.StatusCode/100 != 2 || parsed.Error != nil {
		msg := strings.TrimSpace(string(data))
		if parsed.Error != nil && parsed.Error.Message != "" {
			msg = parsed.Error.Message
		}
		status := hresp.StatusCode
		if status/100 == 2 {
			status = errorCode(parsed.Error.Code)
		}
		apiErr := &APIError{Status: status, Message: truncate(msg, 500)}
		if ra := hresp.Header.Get("Retry-After"); ra != "" {
			if s, err := strconv.Atoi(ra); err == nil {
				return nil, &retryAfterError{apiErr, time.Duration(s) * time.Second}
			}
		}
		return nil, apiErr
	}
	if jsonErr != nil {
		return nil, fmt.Errorf("decode LLM response: %w", jsonErr)
	}
	if len(parsed.Choices) == 0 {
		return nil, &APIError{Status: 502, Message: "empty choices in response"}
	}
	ch := parsed.Choices[0]
	return &Response{
		Model:        parsed.Model,
		Content:      ch.Message.Content,
		FinishReason: ch.FinishReason,
		Usage:        parsed.Usage,
	}, nil
}

func errorCode(code any) int {
	switch v := code.(type) {
	case float64:
		return int(v)
	case string:
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return 502
}

func retryable(err error) bool {
	var ra *retryAfterError
	if errors.As(err, &ra) {
		return true
	}
	var api *APIError
	if errors.As(err, &api) {
		return api.Status == 408 || api.Status == 429 || api.Status >= 500
	}
	// Network errors (timeouts, resets) are worth another try.
	return !errors.Is(err, context.Canceled) && !errors.Is(err, context.DeadlineExceeded)
}

func backoff(attempt int, err error) time.Duration {
	var ra *retryAfterError
	if errors.As(err, &ra) && ra.after > 0 && ra.after <= time.Minute {
		return ra.after
	}
	return time.Duration(1<<(attempt-1)) * 2 * time.Second
}

func sleepCtx(ctx context.Context, d time.Duration) error {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}

func truncate(s string, n int) string {
	if len(s) > n {
		return s[:n] + "…"
	}
	return s
}

// Hint turns common API failures into actionable advice.
func Hint(err error) string {
	var api *APIError
	if !errors.As(err, &api) {
		return ""
	}
	switch api.Status {
	case 401:
		return "check the API key (OPENROUTER_API_KEY or `ytb-sum config show`)"
	case 402:
		return "OpenRouter balance is too low — top up at https://openrouter.ai/settings/credits"
	case 404:
		if strings.Contains(api.Message, "requested parameters") {
			return "this model has no provider supporting structured JSON output — pick another --model"
		}
		return "model not found — check --model against https://openrouter.ai/models"
	case 429:
		return "rate limited — wait a minute or pick another model"
	}
	return ""
}
