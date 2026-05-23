package aiexplain

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// anthropicEndpoint is the Messages API endpoint. Pinned by the SDK
// version contract, Anthropic guarantees stability per `anthropic-version`
// header below, so we don't have to follow API churn.
const anthropicEndpoint = "https://api.anthropic.com/v1/messages"

// anthropicVersion is the dated stable API version contract. Anthropic
// keeps old versions live; bump this header only when intentionally
// adopting new fields.
const anthropicVersion = "2023-06-01"

// defaultModel is the Claude Haiku 4.5 model id. Haiku is the right tier
// for contract explanation: fast, cheap, and the input is well-structured
// source code so reasoning depth is rarely the bottleneck.
const defaultModel = "claude-haiku-4-5"

// Client wraps the Anthropic Messages API for the contract-explanation
// flow. The apiKey + httpClient are immutable after construction so this
// type is safe for concurrent use.
type Client struct {
	apiKey     string
	model      string
	maxTokens  int
	httpClient *http.Client
}

// NewClient builds a Client. apiKey is required; pass empty model to use
// the default. The HTTP client uses a tight per-call timeout, Anthropic
// is usually under 5s for Haiku but a slow run shouldn't tie up the gin
// goroutine for minutes.
func NewClient(apiKey, model string, maxTokens int) *Client {
	if model == "" {
		model = defaultModel
	}
	if maxTokens <= 0 {
		maxTokens = 1500
	}
	return &Client{
		apiKey:     apiKey,
		model:      model,
		maxTokens:  maxTokens,
		httpClient: &http.Client{Timeout: 30 * time.Second},
	}
}

// Generate issues a single Messages.create call and returns the text of
// the first content block. ctx governs the full lifecycle: cancel it from
// the caller side to abort.
//
// We don't stream, the response is short enough that buffering is fine
// and avoids the SSE plumbing overhead.
func (c *Client) Generate(ctx context.Context, system, user string) (string, string, error) {
	body, err := json.Marshal(anthropicRequest{
		Model:     c.model,
		MaxTokens: c.maxTokens,
		System:    system,
		Messages:  []anthropicMessage{{Role: "user", Content: user}},
	})
	if err != nil {
		return "", "", fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, anthropicEndpoint, bytes.NewReader(body))
	if err != nil {
		return "", "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", c.apiKey)
	req.Header.Set("anthropic-version", anthropicVersion)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", "", fmt.Errorf("anthropic HTTP: %w", err)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", "", fmt.Errorf("anthropic body: %w", err)
	}

	var parsed anthropicResponse
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return "", "", fmt.Errorf("anthropic decode: %w (status %d, body %q)", err, resp.StatusCode, truncate(string(raw), 200))
	}
	if parsed.Error != nil {
		return "", "", fmt.Errorf("anthropic %s: %s", parsed.Error.Type, parsed.Error.Message)
	}
	if resp.StatusCode != http.StatusOK {
		return "", "", fmt.Errorf("anthropic HTTP %d: %s", resp.StatusCode, truncate(string(raw), 200))
	}

	for _, block := range parsed.Content {
		if block.Type == "text" && block.Text != "" {
			return block.Text, parsed.Model, nil
		}
	}
	return "", parsed.Model, fmt.Errorf("anthropic returned no text content")
}

// truncate caps the displayed length of a log message safely against
// multi-byte UTF-8 input, slicing a string by byte index could split a
// codepoint and produce invalid UTF-8 in logs.
func truncate(s string, n int) string {
	runes := []rune(s)
	if len(runes) <= n {
		return s
	}
	return string(runes[:n]) + "…"
}
