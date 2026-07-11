// Package metadata implements the background fetcher for off-chain NFT
// metadata: collection-level (Phase 3a, `contractCode`) and per-token
// (Phase 3b, `tokenMetadata`).
//
// Polls the two work queues, resolves each row's URI through the
// configured IPFS gateway, parses the JSON, and persists the off-chain
// `name` / `description` / `image` / `external_url` fields.
//
// Fetch lifecycle (Phase 3c): a row is fetched when it is fresh (never
// attempted), retryable (previous attempt failed and the exponential
// backoff deadline passed), or stale (previous attempt succeeded longer
// than the refresh TTL ago). The stale track is what lets an on-chain
// tokenURI / contractURI change propagate to the explorer; the retry
// track is what un-bricks tokens whose first IPFS fetch timed out
// before the content was widely pinned.
//
// Design notes
//
//   - We deliberately don't run this on the hot block-processing path. A
//     stuck IPFS gateway must not block block indexing.
//   - Default gateway is the myqrlwallet backend's IPFS proxy
//     (https://qrlwallet.com/api/ipfs/), which already enforces a 10 MiB
//     cap, 8 s timeout, CID validation, and strict CSP on responses. The
//     env var IPFS_GATEWAY_URL overrides for local dev / staging /
//     self-hosting; if blank the service self-disables.
//   - Image URLs are resolved through the same gateway so the frontend can
//     render them via next/image without an additional proxy hop.
//   - A failed fetch records the reason (`fetchError`) plus a backoff
//     deadline, and does NOT clear previously-fetched content fields or
//     the last-success timestamp; a yesterday-good name survives a
//     today-bad gateway.
//   - TTL refreshes of unchanged `ipfs://` URIs skip the gateway round
//     trip entirely: IPFS content is content-addressed, so an identical
//     URI implies identical metadata and only the freshness stamp is
//     bumped.
package metadata

import (
	"QRL2MongoDB/configs"
	"QRL2MongoDB/db"
	"QRL2MongoDB/models"
	"QRL2MongoDB/rpc"
	"context"
	"errors"
	"fmt"
	"math/big"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"
)

// Service is the background fetcher; one instance per syncer process.
type Service struct {
	gatewayURL   string
	httpClient   *http.Client
	pollInterval time.Duration
	batchSize    int
	maxBodyBytes int64
	retryBase    time.Duration
	retryMax     time.Duration
	refreshTTL   time.Duration

	stopOnce sync.Once
	stopCh   chan struct{}
}

// metadataServiceConfig holds the tunables sourced from the environment.
type metadataServiceConfig struct {
	gatewayURL   string
	pollInterval time.Duration
	fetchTimeout time.Duration
	maxBodyBytes int64
	batchSize    int
	retryBase    time.Duration
	retryMax     time.Duration
	refreshTTL   time.Duration
	enabled      bool
}

// envInt parses an integer env var with a fallback default.
func envInt(key string, def int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			return n
		}
	}
	return def
}

// loadMetadataServiceConfig reads the metadata-fetcher tunables from env.
// All values have safe defaults so the service runs out of the box; only
// IPFS_GATEWAY_URL must be explicitly set or default to the wallet
// backend's proxy.
func loadMetadataServiceConfig() metadataServiceConfig {
	cfg := metadataServiceConfig{
		gatewayURL:   strings.TrimSpace(os.Getenv("IPFS_GATEWAY_URL")),
		pollInterval: time.Duration(envInt("METADATA_POLL_INTERVAL_MS", 30000)) * time.Millisecond,
		fetchTimeout: time.Duration(envInt("METADATA_FETCH_TIMEOUT_MS", 8000)) * time.Millisecond,
		maxBodyBytes: int64(envInt("METADATA_MAX_BODY_BYTES", 1<<20)), // 1 MiB
		batchSize:    envInt("METADATA_BATCH_SIZE", 25),
		// Failed fetches retry on an exponential backoff: base * 2^attempts,
		// capped at max. Defaults: 1 min doubling up to 6 h, so a freshly
		// pinned CID that missed its first gateway window resolves within
		// minutes while a permanently dead URI costs four RPC probes a day.
		retryBase: time.Duration(envInt("METADATA_RETRY_BASE_MS", 60_000)) * time.Millisecond,
		retryMax:  time.Duration(envInt("METADATA_RETRY_MAX_MS", 21_600_000)) * time.Millisecond,
		// Successful rows re-fetch after this TTL so on-chain tokenURI /
		// contractURI changes propagate. METADATA_REFRESH_ENABLED=false
		// disables the stale track entirely.
		refreshTTL: time.Duration(envInt("METADATA_REFRESH_TTL_MS", 86_400_000)) * time.Millisecond,
		enabled:    strings.ToLower(os.Getenv("METADATA_FETCHER_ENABLED")) != "false",
	}
	if strings.ToLower(os.Getenv("METADATA_REFRESH_ENABLED")) == "false" {
		cfg.refreshTTL = 0
	}
	// Default to the myqrlwallet backend's IPFS proxy. It already enforces
	// CID validation + size caps + timeouts; pointing zondscan at it gives
	// the indexer the same protections without re-implementing them.
	if cfg.gatewayURL == "" {
		cfg.gatewayURL = "https://qrlwallet.com/api/ipfs/"
	}
	// Normalise: gateway must end in a single trailing slash so
	// resolveMetadataURI can concatenate `<gateway><cid>/<path>` cleanly.
	cfg.gatewayURL = strings.TrimRight(cfg.gatewayURL, "/") + "/"
	return cfg
}

// NewService constructs (but does not start) a metadata fetcher.
func NewService() *Service {
	cfg := loadMetadataServiceConfig()
	if !cfg.enabled {
		configs.Logger.Warn("Metadata fetcher service disabled by env (METADATA_FETCHER_ENABLED=false)")
		return nil
	}
	return &Service{
		gatewayURL:   cfg.gatewayURL,
		httpClient:   newSafeHTTPClient(cfg.fetchTimeout),
		pollInterval: cfg.pollInterval,
		batchSize:    cfg.batchSize,
		maxBodyBytes: cfg.maxBodyBytes,
		retryBase:    cfg.retryBase,
		retryMax:     cfg.retryMax,
		refreshTTL:   cfg.refreshTTL,
		stopCh:       make(chan struct{}),
	}
}

// nextRetryDelay computes the exponential-backoff delay after a failure:
// base * 2^retryCount, capped at max. `retryCount` is the number of
// failures BEFORE the current one (0 for the first failure => base delay).
// The doubling loop is bounded by max, so large stored counts can't
// overflow the duration.
func nextRetryDelay(base, max time.Duration, retryCount int) time.Duration {
	if base <= 0 {
		base = time.Minute
	}
	if max < base {
		max = base
	}
	d := base
	for i := 0; i < retryCount && d < max; i++ {
		d *= 2
	}
	if d > max {
		d = max
	}
	return d
}

// Start launches the polling goroutine. Idempotent in the sense that
// calling Start twice will spawn two pollers; callers should construct one
// service per process. The caller-passed context cancels the loop in
// addition to Stop().
func (s *Service) Start(ctx context.Context) {
	if s == nil {
		return
	}
	configs.Logger.Info("Starting metadata fetcher service",
		zap.String("gateway", s.gatewayURL),
		zap.Duration("pollInterval", s.pollInterval),
		zap.Int64("maxBodyBytes", s.maxBodyBytes),
		zap.Int("batchSize", s.batchSize),
		zap.Duration("retryBase", s.retryBase),
		zap.Duration("retryMax", s.retryMax),
		zap.Duration("refreshTTL", s.refreshTTL))

	go func() {
		// First tick immediately so the first batch lands without waiting
		// for the full pollInterval after startup.
		s.tick(ctx)
		t := time.NewTicker(s.pollInterval)
		defer t.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-s.stopCh:
				return
			case <-t.C:
				s.tick(ctx)
			}
		}
	}()
}

// Stop signals the polling goroutine to exit. Safe to call multiple times,
// including concurrently: sync.Once guarantees the stop channel is closed
// exactly once (the previous check-then-act select could double-close under
// a concurrent-Stop race).
func (s *Service) Stop() {
	if s == nil {
		return
	}
	s.stopOnce.Do(func() {
		close(s.stopCh)
	})
}

// tick runs one batch of metadata fetches. Errors per row are logged and
// recorded on the row; the tick itself never fails.
//
// Phase 3b: every tick also processes a batch of per-token metadata stubs
// alongside the collection-level batch, sharing the same poll interval +
// HTTP client + SSRF guard. The two work queues are independent (no
// cross-row dependencies), so a slow contract-URI fetch doesn't block
// per-token progress and vice versa. The exception is a gateway 429: it
// aborts the whole tick (both batches), because the rate limit is shared
// and hammering on regardless would stamp spurious failures.
func (s *Service) tick(ctx context.Context) {
	if rateLimited := s.tickContracts(ctx); rateLimited {
		configs.Logger.Warn("metadata fetcher: skipping token batch this tick (gateway rate limited)")
		return
	}
	s.tickTokens(ctx)
}

// tickContracts processes one contract batch. Returns true when the batch
// was aborted because the gateway rate-limited us.
func (s *Service) tickContracts(ctx context.Context) bool {
	rows, err := db.GetContractsAwaitingMetadata(ctx, s.batchSize, time.Now(), s.refreshTTL)
	if err != nil {
		configs.Logger.Warn("metadata fetcher: failed to read contract work queue", zap.Error(err))
		return false
	}
	if len(rows) == 0 {
		return false
	}
	configs.Logger.Info("metadata fetcher: processing contract batch", zap.Int("rows", len(rows)))

	for i, c := range rows {
		select {
		case <-ctx.Done():
			return false
		case <-s.stopCh:
			return false
		default:
		}
		if err := s.fetchOne(ctx, c); errors.Is(err, errGatewayRateLimited) {
			configs.Logger.Warn("metadata fetcher: contract batch aborted (gateway rate limited)",
				zap.Int("processed", i),
				zap.Int("batch", len(rows)))
			return true
		}
	}
	return false
}

// tickTokens runs one batch of per-tokenID metadata fetches. For each stub
// the fetcher calls tokenURI(id) or uri(id) depending on the contract's
// recorded standard, resolves the resulting URI through the gateway, parses
// the JSON, and writes the result. Same "preserve last-good" semantics as
// the contract path.
func (s *Service) tickTokens(ctx context.Context) {
	rows, err := db.GetTokensAwaitingMetadata(ctx, s.batchSize, time.Now(), s.refreshTTL)
	if err != nil {
		configs.Logger.Warn("metadata fetcher: failed to read token work queue", zap.Error(err))
		return
	}
	if len(rows) == 0 {
		return
	}
	configs.Logger.Info("metadata fetcher: processing token batch", zap.Int("rows", len(rows)))

	for i, r := range rows {
		select {
		case <-ctx.Done():
			return
		case <-s.stopCh:
			return
		default:
		}
		if err := s.fetchOneToken(ctx, r); errors.Is(err, errGatewayRateLimited) {
			configs.Logger.Warn("metadata fetcher: token batch aborted (gateway rate limited)",
				zap.Int("processed", i),
				zap.Int("batch", len(rows)))
			return
		}
	}
}

// fetchOneToken resolves the URI for one (contract, tokenID), parses the
// metadata JSON, and persists the outcome. Same three-step pipeline as
// fetchOne: resolve scheme -> HTTP GET (SSRF-guarded) -> JSON parse.
//
// The on-chain URI is re-read on every attempt, so a TTL refresh picks up
// tokenURI changes; when the URI is unchanged and content-addressed the
// gateway round trip is skipped and only the freshness stamp is bumped.
//
// Returns errGatewayRateLimited when the gateway 429'd (row state
// preserved; the caller aborts the batch), nil otherwise.
func (s *Service) fetchOneToken(ctx context.Context, stub models.TokenMetadata) error {
	id, ok := new(big.Int).SetString(stub.TokenID, 10)
	if !ok {
		s.recordTokenFailure(ctx, stub, "parse tokenID: not a decimal integer")
		return nil
	}

	var (
		uri    string
		uriErr error
	)
	switch stub.TokenStandard {
	case rpc.StandardERC721:
		uri, uriErr = rpc.GetTokenURI(stub.ContractAddress, id)
	case rpc.StandardERC1155:
		uri, uriErr = rpc.GetERC1155URI(stub.ContractAddress, id)
	default:
		s.recordTokenFailure(ctx, stub,
			fmt.Sprintf("unsupported standard %q", stub.TokenStandard))
		return nil
	}
	if uriErr != nil {
		// Transport error (node down / flaky): preserve state and let the
		// row re-enter the queue on a later tick unchanged. Not a fetch
		// failure, so no backoff bookkeeping.
		configs.Logger.Debug("token URI probe transport error; preserving state",
			zap.String("contract", stub.ContractAddress),
			zap.String("tokenID", stub.TokenID),
			zap.Error(uriErr))
		return nil
	}
	if uri == "" {
		// Includes reveal-style collections that set the URI later; the
		// retry track re-probes on backoff until it appears.
		s.recordTokenFailure(ctx, stub, "contract returned no URI")
		return nil
	}

	// TTL-refresh fast path: `stub.URI` is fetcher-owned and only ever
	// written on a successful fetch, so when the on-chain URI still equals
	// it AND the URI is content-addressed, the stored content is provably
	// current: skip the gateway and just bump fetchedAt. This also
	// self-heals a retry row whose refresh failed on a gateway blip (the
	// bump clears the error), because the content for an unchanged
	// immutable URI cannot have drifted.
	if stub.FetchedAt != "" && uri == stub.URI && isImmutableURI(uri) {
		now := time.Now().UTC().Format(time.RFC3339)
		if err := db.UpdateTokenMetadata(ctx, stub.ContractAddress, stub.TokenID,
			"", "", "", "", "", nil, now); err == nil {
			configs.Logger.Debug("metadata fetcher: token freshness bumped (immutable URI unchanged)",
				zap.String("contract", stub.ContractAddress),
				zap.String("tokenID", stub.TokenID))
		}
		return nil
	}

	resolved, err := resolveMetadataURI(uri, s.gatewayURL)
	if err != nil {
		s.recordTokenFailure(ctx, stub, fmt.Sprintf("resolve: %v", err))
		return nil
	}

	body, fetchErr := s.fetchBody(ctx, resolved)
	if fetchErr != nil {
		if errors.Is(fetchErr, errGatewayRateLimited) {
			// Not a row failure: preserve state, abort the batch.
			return errGatewayRateLimited
		}
		s.recordTokenFailure(ctx, stub, fmt.Sprintf("fetch: %v", fetchErr))
		return nil
	}

	parsed, parseErr := parseTokenMetadataJSON(body)
	if parseErr != nil {
		s.recordTokenFailure(ctx, stub, fmt.Sprintf("parse: %v", parseErr))
		return nil
	}

	imageResolved := ""
	if parsed.Image != "" {
		if img, err := resolveMetadataURI(parsed.Image, s.gatewayURL); err == nil {
			imageResolved = img
		} else {
			configs.Logger.Debug("token image URI unresolved",
				zap.String("contract", stub.ContractAddress),
				zap.String("tokenID", stub.TokenID),
				zap.String("image", parsed.Image),
				zap.Error(err))
		}
	}

	now := time.Now().UTC().Format(time.RFC3339)
	if err := db.UpdateTokenMetadata(
		ctx,
		stub.ContractAddress,
		stub.TokenID,
		uri,
		parsed.Name,
		parsed.Description,
		imageResolved,
		parsed.ExternalURL,
		parsed.Attributes,
		now,
	); err != nil {
		configs.Logger.Warn("metadata fetcher: token persist failed",
			zap.String("contract", stub.ContractAddress),
			zap.String("tokenID", stub.TokenID),
			zap.Error(err))
		return nil
	}
	configs.Logger.Info("metadata fetcher: token stored",
		zap.String("contract", stub.ContractAddress),
		zap.String("tokenID", stub.TokenID),
		zap.String("name", parsed.Name),
		zap.Bool("hasImage", imageResolved != ""),
		zap.Int("attributes", len(parsed.Attributes)))
	return nil
}

// recordTokenFailure persists a failed attempt with its backoff schedule.
// The stored retryCount is the count of failures so far; the delay for the
// deadline is derived from the PREVIOUS count so the first failure waits
// exactly retryBase.
func (s *Service) recordTokenFailure(ctx context.Context, stub models.TokenMetadata, reason string) {
	delay := nextRetryDelay(s.retryBase, s.retryMax, stub.RetryCount)
	nextRetryAt := time.Now().UTC().Add(delay).Format(time.RFC3339)
	if err := db.MarkTokenMetadataFetchFailed(ctx, stub.ContractAddress, stub.TokenID,
		reason, stub.RetryCount+1, nextRetryAt); err != nil {
		configs.Logger.Warn("metadata fetcher: failed to record token fetch error",
			zap.String("contract", stub.ContractAddress),
			zap.String("tokenID", stub.TokenID),
			zap.String("reason", reason),
			zap.Error(err))
		return
	}
	configs.Logger.Warn("metadata fetcher: token fetch failed",
		zap.String("contract", stub.ContractAddress),
		zap.String("tokenID", stub.TokenID),
		zap.String("reason", reason),
		zap.Int("retryCount", stub.RetryCount+1),
		zap.String("nextRetryAt", nextRetryAt))
}

// fetchOne resolves and parses one contract's metadata URI, persisting the
// outcome (success or failure) on the contractCode document.
//
// The on-chain contractURI() is re-probed on every attempt (mirroring the
// token path re-reading tokenURI) so a setContractURI change propagates on
// TTL refreshes and retries alike; if the probe transport-fails or returns
// empty, the classifier-stored URI is used.
//
// Unlike fetchOneToken there is NO immutable-URI fast path here: the stored
// metadataURI is classifier-owned and may have been rewritten after the
// last successful fetch, so "URI unchanged" does not imply "stored content
// matches". Collection rows are one per contract; the occasional full
// re-fetch is cheap.
//
// Returns errGatewayRateLimited when the gateway 429'd (row state
// preserved; the caller aborts the batch), nil otherwise.
func (s *Service) fetchOne(ctx context.Context, c models.ContractInfo) error {
	uri := c.MetadataURI
	if fresh, err := rpc.GetContractURI(c.Address); err == nil && fresh != "" {
		uri = fresh
	}

	resolved, err := resolveMetadataURI(uri, s.gatewayURL)
	if err != nil {
		s.recordFailure(ctx, c, fmt.Sprintf("resolve: %v", err))
		return nil
	}

	configs.Logger.Debug("metadata fetcher: fetching",
		zap.String("address", c.Address),
		zap.String("uri", uri),
		zap.String("resolved", resolved))

	body, fetchErr := s.fetchBody(ctx, resolved)
	if fetchErr != nil {
		if errors.Is(fetchErr, errGatewayRateLimited) {
			// Not a row failure: preserve state, abort the batch.
			return errGatewayRateLimited
		}
		s.recordFailure(ctx, c, fmt.Sprintf("fetch: %v", fetchErr))
		return nil
	}

	parsed, parseErr := parseMetadataJSON(body)
	if parseErr != nil {
		s.recordFailure(ctx, c, fmt.Sprintf("parse: %v", parseErr))
		return nil
	}

	// Image URLs are resolved through the same gateway so the persisted
	// value is directly renderable via next/image. If the image is an
	// HTTPS URL we keep it as-is; the next/image allow-list takes care of
	// any extra origins.
	imageResolved := ""
	if parsed.Image != "" {
		if img, err := resolveMetadataURI(parsed.Image, s.gatewayURL); err == nil {
			imageResolved = img
		} else {
			// Image URI couldn't be resolved (unsupported scheme, etc).
			// We still record the rest of the metadata; the missing image
			// just shows as a placeholder in the UI.
			configs.Logger.Debug("metadata fetcher: image URI unresolved",
				zap.String("address", c.Address),
				zap.String("image", parsed.Image),
				zap.Error(err))
		}
	}

	// Persist the re-probed URI only when it actually changed; the
	// classifier remains the primary writer of metadataURI.
	uriForWrite := ""
	if uri != c.MetadataURI {
		uriForWrite = uri
	}

	now := time.Now().UTC().Format(time.RFC3339)
	if err := db.UpdateContractMetadata(
		ctx,
		c.Address,
		uriForWrite,
		parsed.Name,
		parsed.Description,
		imageResolved,
		parsed.ExternalURL,
		now,
	); err != nil {
		configs.Logger.Warn("metadata fetcher: persist failed",
			zap.String("address", c.Address),
			zap.Error(err))
		return nil
	}

	configs.Logger.Info("metadata fetcher: stored",
		zap.String("address", c.Address),
		zap.String("name", parsed.Name),
		zap.Bool("hasImage", imageResolved != ""))
	return nil
}

// recordFailure persists a failed contract-level attempt with its backoff
// schedule. Existing content fields and metadataFetchedAt survive so the
// explorer keeps serving last-good data.
func (s *Service) recordFailure(ctx context.Context, c models.ContractInfo, reason string) {
	delay := nextRetryDelay(s.retryBase, s.retryMax, c.MetadataRetryCount)
	nextRetryAt := time.Now().UTC().Add(delay).Format(time.RFC3339)
	if err := db.MarkContractMetadataFetchFailed(ctx, c.Address, reason, c.MetadataRetryCount+1, nextRetryAt); err != nil {
		configs.Logger.Warn("metadata fetcher: failed to record fetch error",
			zap.String("address", c.Address),
			zap.String("reason", reason),
			zap.Error(err))
		return
	}
	configs.Logger.Warn("metadata fetcher: fetch failed",
		zap.String("address", c.Address),
		zap.String("reason", reason),
		zap.Int("retryCount", c.MetadataRetryCount+1),
		zap.String("nextRetryAt", nextRetryAt))
}
