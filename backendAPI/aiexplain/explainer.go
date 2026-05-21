package aiexplain

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"backendAPI/db"
	"backendAPI/models"
)

// Common errors callers can switch on to return the right HTTP status.
var (
	ErrNotFound       = errors.New("contract not found")
	ErrNotVerified    = errors.New("contract is not verified")
	ErrSourceTooLarge = errors.New("contract source exceeds size cap")
	ErrRegenCap       = errors.New("regeneration cap reached")
)

// RegenLimitPerWindow is the hard cap on regenerations per contract per
// rolling 7-day window. Initial generations don't count; only ?regenerate=1
// calls consume slots. Combined with the cache (subsequent reads short-
// circuit to MongoDB) this bounds the Anthropic spend per contract to
// 5 calls per week regardless of how often the page is hit.
const (
	RegenLimitPerWindow = 5
	RegenWindow         = 7 * 24 * time.Hour
)

// Explainer is the orchestrator: it enforces the verified-only gate, runs
// the Anthropic call, and persists the result back to the contract record
// so subsequent reads are free.
type Explainer struct {
	Client *Client

	// SourceMaxBytes caps how much source we ship to Anthropic. Larger
	// contracts get truncated with a clear "…" marker so the LLM still
	// sees the prelude (which carries most of the structural signal).
	SourceMaxBytes int
}

// systemPrompt frames the LLM as a neutral describer. No safety analysis,
// no implementation feedback, no opinions, just factual observations
// about what the contract is and what it does. The summary is intentionally
// short so it complements rather than competes with the source / ABI panels.
const systemPrompt = `You are summarising a verified smart contract on the QRL Zond v2 blockchain. Produce a short factual description of what the contract is. Do NOT include opinions, safety analysis, risk assessments, recommendations, access-control discussion, or implementation feedback. Just neutral observations.

Output Markdown with only these sections:

**Purpose**, one sentence describing what this contract is.
**What it does**, 2-4 short bullets, descriptive and factual only.
**Standard**, only when the contract clearly matches a well-known token standard (ERC-20, ERC-721, ERC-1155, etc.); name the standard. Omit this section otherwise.

Keep the entire answer under 150 words. Never add sections on risks, security, access control, audit notes, or implementation quality.`

// Explain returns the cached explanation when present, or generates a fresh
// one via Anthropic and persists it before returning. Forces a fresh call
// when regenerate=true.
func (e *Explainer) Explain(ctx context.Context, address string, regenerate bool) (*ExplainResponse, error) {
	c, err := db.ReturnContractCode(address)
	if err != nil {
		return nil, fmt.Errorf("lookup contract: %w", err)
	}
	if c.ContractAddress == "" {
		return nil, ErrNotFound
	}
	if !c.Verified || c.SourceCode == "" {
		return nil, ErrNotVerified
	}

	if !regenerate && c.AIExplanation != "" {
		return &ExplainResponse{
			Address:     c.ContractAddress,
			Explanation: c.AIExplanation,
			GeneratedAt: c.AIExplanationAt,
			Model:       c.AIExplanationModel,
			Cached:      true,
		}, nil
	}

	// Regen path: reserve a slot atomically before spending Anthropic
	// credit. ReserveRegenSlot returns ErrRegenCap when the rolling 7-day
	// cap is hit; we surface that as a typed error the HTTP layer maps to
	// 429. Initial generations skip this check, only regenerate=true
	// passes through here.
	if regenerate {
		if err := db.ReserveAIRegenSlot(address, RegenLimitPerWindow, RegenWindow); err != nil {
			if errors.Is(err, db.ErrAIRegenCap) {
				return nil, ErrRegenCap
			}
			return nil, fmt.Errorf("reserve regen slot: %w", err)
		}
	}

	// Truncate by rune count so a UTF-8 codepoint can't be split mid-byte
	// (which would produce an invalid string and confuse the LLM tokenizer).
	// SourceMaxBytes is interpreted as a *rune* cap; bytes ≈ runes for the
	// ASCII-dominant Hyperion source we expect, with a small safety margin
	// for non-ASCII string literals in the source.
	source := c.SourceCode
	if e.SourceMaxBytes > 0 {
		runes := []rune(c.SourceCode)
		if len(runes) > e.SourceMaxBytes {
			source = string(runes[:e.SourceMaxBytes]) + "\n\n// […truncated for length…]"
		}
	}

	user := buildUserPrompt(c, source)
	text, model, err := e.Client.Generate(ctx, systemPrompt, user)
	if err != nil {
		return nil, fmt.Errorf("anthropic: %w", err)
	}

	now := time.Now().UTC().Format(time.RFC3339)
	if err := db.SaveContractExplanation(address, text, model, now); err != nil {
		// Persistence failure isn't fatal, return the freshly-generated
		// answer so the user gets something. Just log via the caller.
		return &ExplainResponse{
			Address:     c.ContractAddress,
			Explanation: text,
			GeneratedAt: now,
			Model:       model,
			Cached:      false,
		}, fmt.Errorf("cache write failed: %w", err)
	}

	return &ExplainResponse{
		Address:     c.ContractAddress,
		Explanation: text,
		GeneratedAt: now,
		Model:       model,
		Cached:      false,
	}, nil
}

// buildUserPrompt assembles the per-contract message. We embed the
// contract name + address + compiler + license as a header so the model
// can ground its answer in concrete facts, then the source itself.
func buildUserPrompt(c models.ContractInfo, source string) string {
	var b strings.Builder
	b.WriteString("Contract metadata:\n")
	fmt.Fprintf(&b, "- address: %s\n", c.ContractAddress)
	if c.ContractName != "" {
		fmt.Fprintf(&b, "- name: %s\n", c.ContractName)
	}
	if c.CompilerVersion != "" {
		fmt.Fprintf(&b, "- compiler: %s\n", c.CompilerVersion)
	}
	if c.License != "" {
		fmt.Fprintf(&b, "- license: %s\n", c.License)
	}
	b.WriteString("\nSource code:\n```hyperion\n")
	b.WriteString(source)
	if !strings.HasSuffix(source, "\n") {
		b.WriteString("\n")
	}
	b.WriteString("```\n")
	return b.String()
}
