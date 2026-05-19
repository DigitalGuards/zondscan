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

// systemPrompt frames the LLM as a smart-contract auditor producing a
// non-technical summary. The wording is deliberately conservative: this
// is a *summary*, not financial advice, and the model should call out
// risks rather than vouch for safety.
const systemPrompt = `You are an expert smart contract analyst. Given the source code of a contract that has been verified on the QRL Zond v2 blockchain, write a concise plain-English summary that a non-developer can understand.

Output Markdown structured as:

**Purpose** — one sentence on what this contract is.
**What it does** — 2-4 bullets describing the main behaviours / functions.
**Who can use it** — note any access control (owner-only, admin, anyone, etc.).
**Risks to watch for** — surface anything that could cost users money: unchecked mints, owner kill switches, upgradeable proxies, missing access control, math overflow, reentrancy patterns, etc. Be specific. If nothing stands out, say so plainly.

Keep the whole answer under 350 words. Never recommend interaction. Never claim the contract is safe. If the source looks like a well-known standard (ERC-20, ERC-721, etc.), say so explicitly.`

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

	source := c.SourceCode
	if len(source) > e.SourceMaxBytes && e.SourceMaxBytes > 0 {
		source = source[:e.SourceMaxBytes] + "\n\n// […truncated for length…]"
	}

	user := buildUserPrompt(c, source)
	text, model, err := e.Client.Generate(ctx, systemPrompt, user)
	if err != nil {
		return nil, fmt.Errorf("anthropic: %w", err)
	}

	now := time.Now().UTC().Format(time.RFC3339)
	if err := db.SaveContractExplanation(address, text, model, now); err != nil {
		// Persistence failure isn't fatal — return the freshly-generated
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
