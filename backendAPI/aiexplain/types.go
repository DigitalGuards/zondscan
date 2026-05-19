package aiexplain

// ExplainResponse is the body returned by POST /contract/explain/:address.
// Cached=true means the body came straight from MongoDB without re-calling
// the Anthropic API; useful for the frontend to surface a "regenerate"
// affordance.
type ExplainResponse struct {
	Address     string `json:"address"`
	Explanation string `json:"explanation"`
	GeneratedAt string `json:"generatedAt"`
	Model       string `json:"model"`
	Cached      bool   `json:"cached"`
}

// Anthropic Messages API request shape. We only need a tiny slice of the
// surface: model, max_tokens, system, single user message.
type anthropicRequest struct {
	Model     string             `json:"model"`
	MaxTokens int                `json:"max_tokens"`
	System    string             `json:"system,omitempty"`
	Messages  []anthropicMessage `json:"messages"`
}

type anthropicMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// Anthropic Messages API response — minimal projection. We only ever read
// the first text block; if Anthropic changes shape, the unmarshalling will
// just produce an empty Content string and the caller surfaces an error.
type anthropicResponse struct {
	Content []anthropicContent `json:"content"`
	Model   string             `json:"model"`
	Error   *anthropicError    `json:"error,omitempty"`
}

type anthropicContent struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type anthropicError struct {
	Type    string `json:"type"`
	Message string `json:"message"`
}
