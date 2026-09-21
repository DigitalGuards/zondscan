package aiexplain

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"backendAPI/db"
	"backendAPI/models"
	"backendAPI/sourcebundle"
)

// Common errors callers can switch on to return the right HTTP status.
var (
	ErrNotFound       = errors.New("contract not found")
	ErrNotVerified    = errors.New("contract is not verified")
	ErrSourceTooLarge = errors.New("contract source exceeds size cap")
	ErrProvenance     = errors.New("verification provenance is not digest-backed")
	ErrSourceChanged  = errors.New("contract source bundle changed")
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
	// GenerationLeaseTTL exceeds the route deadline plus FailureCooldown.
	// The original lease preserves a full cooldown when a follow-up Mongo
	// cooldown write encounters a partial outage.
	GenerationLeaseTTL = 3 * time.Minute
	FailureCooldown    = 2 * time.Minute
)

// Explainer is the orchestrator: it enforces the verified-only gate, runs
// the Anthropic call, and persists the result back to the contract record
// so subsequent reads are free.
type Explainer struct {
	Client modelGenerator
	Store  generationStore

	// SourceMaxBytes caps how much of the complete verified source bundle we
	// ship to Anthropic. The historical name is retained for configuration
	// compatibility; the implementation counts runes to preserve valid UTF-8.
	SourceMaxBytes int

	// DailyProviderCallLimit is shared through a Mongo UTC-day counter. The
	// configured value is validated during Init.
	DailyProviderCallLimit int

	Now func() time.Time
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
	store := e.Store
	if store == nil {
		store = mongoGenerationStore{}
	}
	now := time.Now
	if e.Now != nil {
		now = e.Now
	}

	c, err := store.ReturnContractCode(address)
	if err != nil {
		return nil, &retryableError{
			kind:       ErrStorageUnavailable,
			cause:      fmt.Errorf("lookup contract: %w", err),
			retryAfter: FailureCooldown,
		}
	}
	if c.ContractAddress == "" {
		return nil, ErrNotFound
	}
	if !c.Verified || c.SourceCode == "" {
		return nil, ErrNotVerified
	}
	if !explanationRecordIsDigestBacked(c) {
		return nil, ErrProvenance
	}

	fullSourceBundle := buildSourceBundle(c)
	sourceDigest := sourcebundle.Digest(c.ContractName, c.SourceCode, c.Imports)

	if !regenerate && cachedExplanationMatches(c, sourceDigest) {
		return cachedExplainResponse(c), nil
	}

	lease, err := store.AcquireLease(
		ctx,
		address,
		c,
		sourceDigest,
		GenerationLeaseTTL,
		!regenerate,
	)
	if err != nil {
		var busy *db.AIExplanationLeaseBusyError
		switch {
		case errors.As(err, &busy):
			return nil, &retryableError{
				kind:       ErrGenerationInProgress,
				cause:      err,
				retryAfter: busy.RetryAfterDuration,
			}
		case errors.Is(err, db.ErrSourceBundleChanged):
			return nil, ErrSourceChanged
		case errors.Is(err, db.ErrAIExplanationCacheFilled):
			fresh, freshErr := store.ReturnContractCode(address)
			if freshErr != nil {
				return nil, &retryableError{
					kind:       ErrStorageUnavailable,
					cause:      fmt.Errorf("reload concurrent AI explanation cache: %w", freshErr),
					retryAfter: FailureCooldown,
				}
			}
			if cachedExplanationMatches(fresh, sourceDigest) {
				return cachedExplainResponse(fresh), nil
			}
			if fresh.VerifiedAt != c.VerifiedAt || fresh.SourceBundleDigest != sourceDigest {
				return nil, ErrSourceChanged
			}
			return nil, &retryableError{
				kind:       ErrGenerationInProgress,
				cause:      err,
				retryAfter: time.Second,
			}
		default:
			return nil, &retryableError{
				kind:       ErrStorageUnavailable,
				cause:      fmt.Errorf("acquire AI explanation lease: %w", err),
				retryAfter: FailureCooldown,
			}
		}
	}
	releaseLease := func() {
		_ = store.ReleaseLease(context.WithoutCancel(ctx), address, lease)
	}
	cooldownLease := func() {
		_ = store.CooldownLease(
			context.WithoutCancel(ctx),
			address,
			lease,
			FailureCooldown,
		)
	}

	// Regen path: reserve a slot atomically before spending Anthropic
	// credit. ReserveRegenSlot returns ErrRegenCap when the rolling 7-day
	// cap is hit; we surface that as a typed error the HTTP layer maps to
	// 429. Initial generations skip this check, only regenerate=true
	// passes through here.
	if regenerate {
		if err := store.ReserveRegenSlot(address, RegenLimitPerWindow, RegenWindow); err != nil {
			releaseLease()
			if errors.Is(err, db.ErrAIRegenCap) {
				return nil, ErrRegenCap
			}
			return nil, &retryableError{
				kind:       ErrStorageUnavailable,
				cause:      fmt.Errorf("reserve regen slot: %w", err),
				retryAfter: FailureCooldown,
			}
		}
	}

	dailyLimit := e.DailyProviderCallLimit
	if dailyLimit <= 0 {
		dailyLimit = DefaultDailyProviderCallLimit
	}
	if err := store.ReserveProviderCall(ctx, dailyLimit, now().UTC()); err != nil {
		var budget *db.AIProviderDailyBudgetError
		if errors.As(err, &budget) {
			releaseLease()
			return nil, &retryableError{
				kind:       ErrDailyBudget,
				cause:      err,
				retryAfter: budget.RetryAfterDuration,
			}
		}
		cooldownLease()
		return nil, &retryableError{
			kind:       ErrBudgetCooldown,
			cause:      fmt.Errorf("reserve AI provider call budget: %w", err),
			retryAfter: FailureCooldown,
		}
	}

	// The primary source is always first and imports use canonical filename
	// order. Apply one cap to the aggregate so imports cannot bypass the
	// existing source budget.
	source := capSourceBundle(fullSourceBundle, e.SourceMaxBytes)

	user := buildUserPrompt(c, source)
	text, model, err := e.Client.Generate(ctx, systemPrompt, user)
	if err != nil {
		cooldownLease()
		return nil, &retryableError{
			kind:       ErrProviderCooldown,
			cause:      fmt.Errorf("anthropic: %w", err),
			retryAfter: FailureCooldown,
		}
	}

	generatedAt := now().UTC().Format(time.RFC3339)
	if err := store.SaveExplanation(
		ctx,
		address,
		text,
		model,
		generatedAt,
		sourceDigest,
		lease,
		c,
	); err != nil {
		if errors.Is(err, db.ErrSourceBundleChanged) {
			return nil, ErrSourceChanged
		}
		cooldownLease()
		return nil, &retryableError{
			kind:       ErrCacheCooldown,
			cause:      err,
			retryAfter: FailureCooldown,
		}
	}

	return &ExplainResponse{
		Address:     c.ContractAddress,
		Explanation: text,
		GeneratedAt: generatedAt,
		Model:       model,
		Cached:      false,
	}, nil
}

// buildUserPrompt assembles the per-contract message. We embed the
// contract name + address + compiler + license as a header so the model
// can ground its answer in concrete facts, then the verified source bundle.
func buildUserPrompt(c models.ContractInfo, sourceBundle string) string {
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
	b.WriteString("\nVerified source bundle:\n```hyperion\n")
	b.WriteString(sourceBundle)
	if !strings.HasSuffix(sourceBundle, "\n") {
		b.WriteString("\n")
	}
	b.WriteString("```\n")
	return b.String()
}

func buildSourceBundle(c models.ContractInfo) string {
	return sourcebundle.Render(c.ContractName, c.SourceCode, c.Imports)
}

func cachedExplanationMatches(c models.ContractInfo, sourceDigest string) bool {
	return c.AIExplanation != "" &&
		c.AIExplanationSourceDigest == sourceDigest &&
		c.SourceBundleDigest == sourceDigest
}

func cachedExplainResponse(c models.ContractInfo) *ExplainResponse {
	return &ExplainResponse{
		Address:     c.ContractAddress,
		Explanation: c.AIExplanation,
		GeneratedAt: c.AIExplanationAt,
		Model:       c.AIExplanationModel,
		Cached:      true,
	}
}

func explanationRecordIsDigestBacked(c models.ContractInfo) bool {
	return sourcebundle.ClassifyStoredVerification(c) == models.CompilerProvenanceDigestBacked
}

func capSourceBundle(sourceBundle string, maxRunes int) string {
	if maxRunes <= 0 {
		return sourceBundle
	}
	runes := []rune(sourceBundle)
	if len(runes) <= maxRunes {
		return sourceBundle
	}

	marker := []rune("\n\n// [truncated for length]")
	if maxRunes <= len(marker) {
		return string(runes[:maxRunes])
	}
	return string(runes[:maxRunes-len(marker)]) + string(marker)
}
