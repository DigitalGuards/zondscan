// Package services / metadata_service.go
//
// Background fetcher for collection-level off-chain NFT metadata (Phase 3a).
//
// Polls `contractCode` for NFT contracts that have a `metadataURI` populated
// by the classifier but no `metadataFetchedAt` yet (or a previous fetch
// error), resolves the URI through the configured IPFS gateway, parses the
// JSON, and persists the off-chain `name` / `description` / `image` /
// `external_url` fields via `db.UpdateContractMetadata`.
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
//   - The fetcher updates `metadataFetchError` on transient failures so an
//     operator can see what went wrong without grepping logs. The error
//     does NOT clear previously-fetched content fields; a yesterday-good
//     name survives a today-bad gateway.
package synchroniser

import (
	"QRL2MongoDB/configs"
	"QRL2MongoDB/db"
	"QRL2MongoDB/models"
	"QRL2MongoDB/rpc"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"net"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
	"syscall"
	"time"

	"go.uber.org/zap"
)

// MetadataService is the background fetcher; one instance per syncer process.
type MetadataService struct {
	gatewayURL   string
	httpClient   *http.Client
	pollInterval time.Duration
	batchSize    int
	maxBodyBytes int64

	stopCh chan struct{}
}

// metadataServiceConfig holds the tunables sourced from the environment.
type metadataServiceConfig struct {
	gatewayURL   string
	pollInterval time.Duration
	fetchTimeout time.Duration
	maxBodyBytes int64
	batchSize    int
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
		enabled:      strings.ToLower(os.Getenv("METADATA_FETCHER_ENABLED")) != "false",
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

// NewMetadataService constructs (but does not start) a metadata fetcher.
func NewMetadataService() *MetadataService {
	cfg := loadMetadataServiceConfig()
	if !cfg.enabled {
		configs.Logger.Warn("Metadata fetcher service disabled by env (METADATA_FETCHER_ENABLED=false)")
		return nil
	}
	return &MetadataService{
		gatewayURL:   cfg.gatewayURL,
		httpClient:   newSafeHTTPClient(cfg.fetchTimeout),
		pollInterval: cfg.pollInterval,
		batchSize:    cfg.batchSize,
		maxBodyBytes: cfg.maxBodyBytes,
		stopCh:       make(chan struct{}),
	}
}

// newSafeHTTPClient builds an http.Client whose Transport refuses to connect
// to private / loopback / link-local / multicast / unspecified IPs. The
// Dialer's Control hook fires after DNS resolution and before the socket
// connects, so it also catches DNS-rebinding attacks where a hostname
// resolves to a public IP at validation time but a private one when the
// dial actually runs.
//
// This is the SSRF perimeter for the metadata fetcher: any URL whose host
// resolves to a forbidden range gets a connection error from net.Dial,
// surfaced as a normal HTTP error to fetchBody. We never reach the
// dangerous endpoint.
func newSafeHTTPClient(timeout time.Duration) *http.Client {
	dialer := &net.Dialer{
		Timeout:   timeout,
		KeepAlive: 30 * time.Second,
		Control: func(network, address string, _ syscall.RawConn) error {
			host, _, err := net.SplitHostPort(address)
			if err != nil {
				return err
			}
			ip := net.ParseIP(host)
			if ip == nil {
				return fmt.Errorf("ssrf-guard: unresolvable host %q", host)
			}
			if isForbiddenIP(ip) {
				return fmt.Errorf("ssrf-guard: refusing to connect to %s", ip.String())
			}
			return nil
		},
	}
	return &http.Client{
		Timeout: timeout,
		Transport: &http.Transport{
			DialContext:         dialer.DialContext,
			TLSHandshakeTimeout: timeout,
			// Disable connection reuse so a previously-allowed IP can't be
			// re-targeted by a later DNS rebind for the same hostname.
			DisableKeepAlives: true,
		},
	}
}

// extraBlockedCIDRs lists ranges Go's net.IP.IsPrivate / IsLoopback /
// IsLinkLocal* don't cover but that we still don't want a server-side
// fetcher hitting. Kept as a package var so tests can extend it.
var extraBlockedCIDRs = mustParseCIDRs(
	"100.64.0.0/10",   // CGNAT
	"192.0.0.0/24",    // IETF Protocol Assignments
	"198.18.0.0/15",   // Network interconnect device benchmark
	"192.0.2.0/24",    // TEST-NET-1
	"198.51.100.0/24", // TEST-NET-2
	"203.0.113.0/24",  // TEST-NET-3
	"240.0.0.0/4",     // Reserved
	"fc00::/7",        // IPv6 ULA
	"fe80::/10",       // IPv6 link-local
	"100::/64",        // IPv6 discard prefix
	"2001:db8::/32",   // IPv6 docs prefix
)

func mustParseCIDRs(s ...string) []*net.IPNet {
	nets := make([]*net.IPNet, 0, len(s))
	for _, c := range s {
		_, n, err := net.ParseCIDR(c)
		if err != nil {
			panic(fmt.Sprintf("invalid extra-blocked CIDR %q: %v", c, err))
		}
		nets = append(nets, n)
	}
	return nets
}

// isForbiddenIP returns true if `ip` is in a range the metadata fetcher must
// never connect to. Combines Go's stdlib helpers with explicit additional
// CIDRs (CGNAT, IETF reserved, IPv6 ULA, etc) that the stdlib doesn't flag.
func isForbiddenIP(ip net.IP) bool {
	if ip == nil {
		return true
	}
	if ip.IsLoopback() ||
		ip.IsPrivate() ||
		ip.IsLinkLocalUnicast() ||
		ip.IsLinkLocalMulticast() ||
		ip.IsMulticast() ||
		ip.IsUnspecified() ||
		ip.IsInterfaceLocalMulticast() {
		return true
	}
	for _, n := range extraBlockedCIDRs {
		if n.Contains(ip) {
			return true
		}
	}
	return false
}

// Start launches the polling goroutine. Idempotent in the sense that
// calling Start twice will spawn two pollers; callers should construct one
// service per process. The caller-passed context cancels the loop in
// addition to Stop().
func (s *MetadataService) Start(ctx context.Context) {
	if s == nil {
		return
	}
	configs.Logger.Info("Starting metadata fetcher service",
		zap.String("gateway", s.gatewayURL),
		zap.Duration("pollInterval", s.pollInterval),
		zap.Int64("maxBodyBytes", s.maxBodyBytes),
		zap.Int("batchSize", s.batchSize))

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

// Stop signals the polling goroutine to exit. Safe to call multiple times.
func (s *MetadataService) Stop() {
	if s == nil {
		return
	}
	select {
	case <-s.stopCh:
		// already closed
	default:
		close(s.stopCh)
	}
}

// tick runs one batch of metadata fetches. Errors per row are logged and
// recorded on the contractCode document; the tick itself never fails.
//
// Phase 3b: every tick also processes a batch of per-token metadata stubs
// alongside the collection-level batch, sharing the same poll interval +
// HTTP client + SSRF guard. The two work queues are independent (no
// cross-row dependencies), so a slow contract-URI fetch doesn't block
// per-token progress and vice versa.
func (s *MetadataService) tick(ctx context.Context) {
	s.tickContracts(ctx)
	s.tickTokens(ctx)
}

func (s *MetadataService) tickContracts(ctx context.Context) {
	rows, err := db.GetContractsAwaitingMetadata(ctx, s.batchSize)
	if err != nil {
		configs.Logger.Warn("metadata fetcher: failed to read contract work queue", zap.Error(err))
		return
	}
	if len(rows) == 0 {
		return
	}
	configs.Logger.Info("metadata fetcher: processing contract batch", zap.Int("rows", len(rows)))

	for _, c := range rows {
		select {
		case <-ctx.Done():
			return
		case <-s.stopCh:
			return
		default:
		}
		s.fetchOne(ctx, c.Address, c.MetadataURI)
	}
}

// tickTokens runs one batch of per-tokenID metadata fetches. For each stub
// the fetcher calls tokenURI(id) or uri(id) depending on the contract's
// recorded standard, resolves the resulting URI through the gateway, parses
// the JSON, and writes the result. Same "preserve last-good" semantics as
// the contract path.
func (s *MetadataService) tickTokens(ctx context.Context) {
	rows, err := db.GetTokensAwaitingMetadata(ctx, s.batchSize)
	if err != nil {
		configs.Logger.Warn("metadata fetcher: failed to read token work queue", zap.Error(err))
		return
	}
	if len(rows) == 0 {
		return
	}
	configs.Logger.Info("metadata fetcher: processing token batch", zap.Int("rows", len(rows)))

	for _, r := range rows {
		select {
		case <-ctx.Done():
			return
		case <-s.stopCh:
			return
		default:
		}
		s.fetchOneToken(ctx, r)
	}
}

// fetchOneToken resolves the URI for one (contract, tokenID), parses the
// metadata JSON, and persists the outcome. Same three-step pipeline as
// fetchOne: resolve scheme -> HTTP GET (SSRF-guarded) -> JSON parse.
func (s *MetadataService) fetchOneToken(ctx context.Context, stub models.TokenMetadata) {
	id, ok := new(big.Int).SetString(stub.TokenID, 10)
	if !ok {
		s.recordTokenFailure(ctx, stub.ContractAddress, stub.TokenID, "parse tokenID: not a decimal integer")
		return
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
		s.recordTokenFailure(ctx, stub.ContractAddress, stub.TokenID,
			fmt.Sprintf("unsupported standard %q", stub.TokenStandard))
		return
	}
	if uriErr != nil {
		// Transport error: preserve state, retry on the next tick (because
		// fetchedAt + fetchError stay empty).
		configs.Logger.Debug("token URI probe transport error; preserving state",
			zap.String("contract", stub.ContractAddress),
			zap.String("tokenID", stub.TokenID),
			zap.Error(uriErr))
		return
	}
	if uri == "" {
		s.recordTokenFailure(ctx, stub.ContractAddress, stub.TokenID, "contract returned no URI")
		return
	}

	resolved, err := resolveMetadataURI(uri, s.gatewayURL)
	if err != nil {
		s.recordTokenFailure(ctx, stub.ContractAddress, stub.TokenID, fmt.Sprintf("resolve: %v", err))
		return
	}

	body, fetchErr := s.fetchBody(ctx, resolved)
	if fetchErr != nil {
		s.recordTokenFailure(ctx, stub.ContractAddress, stub.TokenID, fmt.Sprintf("fetch: %v", fetchErr))
		return
	}

	parsed, parseErr := parseTokenMetadataJSON(body)
	if parseErr != nil {
		s.recordTokenFailure(ctx, stub.ContractAddress, stub.TokenID, fmt.Sprintf("parse: %v", parseErr))
		return
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
		"",
	); err != nil {
		configs.Logger.Warn("metadata fetcher: token persist failed",
			zap.String("contract", stub.ContractAddress),
			zap.String("tokenID", stub.TokenID),
			zap.Error(err))
		return
	}
	configs.Logger.Info("metadata fetcher: token stored",
		zap.String("contract", stub.ContractAddress),
		zap.String("tokenID", stub.TokenID),
		zap.String("name", parsed.Name),
		zap.Bool("hasImage", imageResolved != ""),
		zap.Int("attributes", len(parsed.Attributes)))
}

func (s *MetadataService) recordTokenFailure(ctx context.Context, contract, tokenID, reason string) {
	if err := db.UpdateTokenMetadata(ctx, contract, tokenID, "", "", "", "", "", nil, "", reason); err != nil {
		configs.Logger.Warn("metadata fetcher: failed to record token fetch error",
			zap.String("contract", contract),
			zap.String("tokenID", tokenID),
			zap.String("reason", reason),
			zap.Error(err))
		return
	}
	configs.Logger.Warn("metadata fetcher: token fetch failed",
		zap.String("contract", contract),
		zap.String("tokenID", tokenID),
		zap.String("reason", reason))
}

// fetchOne resolves and parses one contract's metadata URI, persisting the
// outcome (success or failure) on the contractCode document.
func (s *MetadataService) fetchOne(ctx context.Context, address, uri string) {
	resolved, err := resolveMetadataURI(uri, s.gatewayURL)
	if err != nil {
		s.recordFailure(ctx, address, fmt.Sprintf("resolve: %v", err))
		return
	}

	configs.Logger.Debug("metadata fetcher: fetching",
		zap.String("address", address),
		zap.String("uri", uri),
		zap.String("resolved", resolved))

	body, fetchErr := s.fetchBody(ctx, resolved)
	if fetchErr != nil {
		s.recordFailure(ctx, address, fmt.Sprintf("fetch: %v", fetchErr))
		return
	}

	parsed, parseErr := parseMetadataJSON(body)
	if parseErr != nil {
		s.recordFailure(ctx, address, fmt.Sprintf("parse: %v", parseErr))
		return
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
				zap.String("address", address),
				zap.String("image", parsed.Image),
				zap.Error(err))
		}
	}

	now := time.Now().UTC().Format(time.RFC3339)
	if err := db.UpdateContractMetadata(
		ctx,
		address,
		parsed.Name,
		parsed.Description,
		imageResolved,
		parsed.ExternalURL,
		now,
		"", // clear any prior error on success
	); err != nil {
		configs.Logger.Warn("metadata fetcher: persist failed",
			zap.String("address", address),
			zap.Error(err))
		return
	}

	configs.Logger.Info("metadata fetcher: stored",
		zap.String("address", address),
		zap.String("name", parsed.Name),
		zap.Bool("hasImage", imageResolved != ""))
}

func (s *MetadataService) recordFailure(ctx context.Context, address, reason string) {
	// Pass empty content args so the existing name/image survives a transient
	// gateway error.
	if err := db.UpdateContractMetadata(ctx, address, "", "", "", "", "", reason); err != nil {
		configs.Logger.Warn("metadata fetcher: failed to record fetch error",
			zap.String("address", address),
			zap.String("reason", reason),
			zap.Error(err))
		return
	}
	configs.Logger.Warn("metadata fetcher: fetch failed",
		zap.String("address", address),
		zap.String("reason", reason))
}

// fetchBody issues an HTTP GET with the configured timeout and size cap.
func (s *MetadataService) fetchBody(ctx context.Context, resolvedURL string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, resolvedURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json,text/plain,*/*")
	req.Header.Set("User-Agent", "zondscan-metadata-fetcher/3a")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	limited := io.LimitReader(resp.Body, s.maxBodyBytes+1)
	body, err := io.ReadAll(limited)
	if err != nil {
		return nil, err
	}
	if int64(len(body)) > s.maxBodyBytes {
		return nil, fmt.Errorf("body exceeds %d bytes", s.maxBodyBytes)
	}
	return body, nil
}

// metadataDocument is the minimal subset of the NFT metadata schema we
// care about for Phase 3a (collection level). Unknown fields are ignored.
// Each field is optional; the parser coerces wrong-typed fields to the
// empty string so one bad entry can't fail the whole record.
type metadataDocument struct {
	Name        string `json:"-"`
	Description string `json:"-"`
	Image       string `json:"-"`
	ExternalURL string `json:"-"`
}

// tokenMetadataDocument extends metadataDocument with the per-token
// `attributes` array (Phase 3b). Values are coerced to strings for
// storage uniformity since the OpenSea spec allows mixed types
// (numeric, string, object) and downstream consumers can re-parse.
type tokenMetadataDocument struct {
	Name        string
	Description string
	Image       string
	ExternalURL string
	Attributes  []models.TokenAttribute
}

// parseMetadataJSON does a defensive JSON parse: unmarshal into a generic
// map, then pull out the string fields we care about, ignoring any non-
// string variants. This lets a malformed `description: ["foo", "bar"]`
// reduce to an empty description rather than failing the entire record.
//
// Two sentinel cases for which we still return an empty doc + nil error
// rather than a JSON-shape error:
//
//   - the JSON literal `null`, which unmarshals into a nil map; reading
//     `raw[key]` from a nil map is safe in Go but the explicit check keeps
//     the intent obvious and silences static analysis warnings;
//   - top-level scalars or arrays, where Unmarshal fails with a type
//     mismatch error - those still fail loudly.
func parseMetadataJSON(body []byte) (metadataDocument, error) {
	var raw map[string]interface{}
	if err := json.Unmarshal(body, &raw); err != nil {
		return metadataDocument{}, err
	}
	stringField := func(key string) string {
		// Reading from a nil map is safe in Go, but the explicit branch
		// avoids any ambiguity for future readers.
		if raw == nil {
			return ""
		}
		v, ok := raw[key]
		if !ok || v == nil {
			return ""
		}
		s, ok := v.(string)
		if !ok {
			return ""
		}
		return strings.TrimSpace(s)
	}
	doc := metadataDocument{
		Name:        stringField("name"),
		Description: stringField("description"),
		Image:       stringField("image"),
		ExternalURL: stringField("external_url"),
	}
	return doc, nil
}

// parseTokenMetadataJSON is the Phase 3b variant that also extracts the
// `attributes` array (OpenSea convention: each entry has `trait_type`,
// `value`, optionally `display_type`). Anything that isn't a clean array
// of objects is dropped silently, one bad NFT shouldn't fail the rest.
//
// Uses json.Decoder.UseNumber() rather than vanilla json.Unmarshal so
// trait values that are large integers (e.g. token IDs as attribute
// values, common in game NFT schemas) keep their full precision instead
// of being rounded through float64 at 2^53.
func parseTokenMetadataJSON(body []byte) (tokenMetadataDocument, error) {
	dec := json.NewDecoder(bytes.NewReader(body))
	dec.UseNumber()
	var raw map[string]interface{}
	if err := dec.Decode(&raw); err != nil {
		return tokenMetadataDocument{}, err
	}
	stringField := func(key string) string {
		if raw == nil {
			return ""
		}
		v, ok := raw[key]
		if !ok || v == nil {
			return ""
		}
		s, ok := v.(string)
		if !ok {
			return ""
		}
		return strings.TrimSpace(s)
	}

	out := tokenMetadataDocument{
		Name:        stringField("name"),
		Description: stringField("description"),
		Image:       stringField("image"),
		ExternalURL: stringField("external_url"),
	}

	// attributes: []{trait_type, value, display_type?}
	if raw != nil {
		if attrsAny, ok := raw["attributes"]; ok {
			if attrs, isArr := attrsAny.([]interface{}); isArr {
				const maxAttrs = 64 // defensive: stop a malicious doc from blowing memory
				for i, a := range attrs {
					if i >= maxAttrs {
						break
					}
					m, isMap := a.(map[string]interface{})
					if !isMap {
						continue
					}
					attr := models.TokenAttribute{
						TraitType:   stringifyAttrField(m["trait_type"]),
						Value:       stringifyAttrField(m["value"]),
						DisplayType: stringifyAttrField(m["display_type"]),
					}
					if attr.TraitType == "" && attr.Value == "" {
						continue // empty attr, skip
					}
					out.Attributes = append(out.Attributes, attr)
				}
			}
		}
	}
	return out, nil
}

// stringifyAttrField coerces an arbitrary JSON value to a string for
// storage uniformity. Strings pass through trimmed; numbers (preserved as
// json.Number via Decoder.UseNumber so we keep full precision for big
// uint256-shaped trait values) pass through verbatim; bools become
// "true"/"false"; nil and objects collapse to "".
//
// Older callers may still feed float64 values when the JSON came from
// json.Unmarshal instead of json.Decoder.UseNumber; the float64 branch
// is kept for that compatibility path.
func stringifyAttrField(v interface{}) string {
	switch t := v.(type) {
	case nil:
		return ""
	case string:
		return strings.TrimSpace(t)
	case bool:
		if t {
			return "true"
		}
		return "false"
	case json.Number:
		// Decoder.UseNumber path: keep the original textual form, no
		// float64 round-trip. Covers ints past 2^53.
		return t.String()
	case float64:
		// Vanilla Unmarshal fallback; preserve integer-ness when possible.
		if t == float64(int64(t)) {
			return strconv.FormatInt(int64(t), 10)
		}
		return strconv.FormatFloat(t, 'f', -1, 64)
	default:
		return ""
	}
}

// CID validation mirrors the wallet backend's IPFS proxy so callers fail
// fast on malformed CIDs instead of round-tripping to the gateway.
var (
	cidV0Re = regexp.MustCompile(`^Qm[1-9A-HJ-NP-Za-km-z]{44}$`)
	cidV1Re = regexp.MustCompile(`^b[A-Za-z2-7]{58,}$`)
)

func looksLikeCID(s string) bool {
	return cidV0Re.MatchString(s) || cidV1Re.MatchString(s)
}

// resolveMetadataURI normalises a metadata URI into a fetchable HTTPS URL.
//
// Supported schemes:
//
//   - https://...           returned as-is.
//   - http://...            returned as-is (legacy support; the syncer is
//     server-side so plain HTTP is acceptable).
//   - ipfs://CID[/path]     -> gatewayURL + CID + "/" + path
//   - <bare CID>[/path]     -> gatewayURL + CID + "/" + path  (some
//     contracts emit the bare CID without scheme).
//
// Anything else (ar://, ipns://, data:, file:, javascript:) returns an
// error. The wallet backend's IPFS proxy already validates CID + path
// shape, so we reuse the same regexes here to fail fast.
func resolveMetadataURI(uri, gatewayURL string) (string, error) {
	uri = strings.TrimSpace(uri)
	if uri == "" {
		return "", fmt.Errorf("empty URI")
	}

	// https / http: return as-is. We don't fetch via the IPFS gateway in
	// this case, that's intentional, the gateway only knows how to
	// resolve CIDs.
	if strings.HasPrefix(uri, "https://") || strings.HasPrefix(uri, "http://") {
		// Validate it's parseable so we don't pass garbage to http.NewRequest.
		if _, err := url.Parse(uri); err != nil {
			return "", fmt.Errorf("malformed http(s) URI: %w", err)
		}
		return uri, nil
	}

	// ipfs://CID[/path]
	if strings.HasPrefix(uri, "ipfs://") {
		rest := strings.TrimPrefix(uri, "ipfs://")
		// Some contracts emit `ipfs://ipfs/CID`; collapse it.
		rest = strings.TrimPrefix(rest, "ipfs/")
		return joinGateway(gatewayURL, rest)
	}

	// Bare CID with optional path.
	first := strings.SplitN(uri, "/", 2)[0]
	if looksLikeCID(first) {
		return joinGateway(gatewayURL, uri)
	}

	return "", fmt.Errorf("unsupported URI scheme: %s", truncate(uri, 80))
}

// joinGateway pastes the path onto the gateway URL. The gateway is
// guaranteed (by loadMetadataServiceConfig) to end in "/", so a clean
// concatenation is correct.
func joinGateway(gateway, path string) (string, error) {
	if path == "" {
		return "", fmt.Errorf("empty CID/path")
	}
	// Sanity: the CID portion must validate.
	first := strings.SplitN(path, "/", 2)[0]
	if !looksLikeCID(first) {
		return "", fmt.Errorf("invalid CID: %s", truncate(first, 80))
	}
	return gateway + path, nil
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
