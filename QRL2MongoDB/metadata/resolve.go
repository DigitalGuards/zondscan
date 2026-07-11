package metadata

import (
	"fmt"
	"net/url"
	"regexp"
	"strings"
)

// CID validation mirrors the wallet backend's IPFS proxy so callers fail
// fast on malformed CIDs instead of round-tripping to the gateway.
var (
	cidV0Re = regexp.MustCompile(`^Qm[1-9A-HJ-NP-Za-km-z]{44}$`)
	cidV1Re = regexp.MustCompile(`^b[A-Za-z2-7]{58,}$`)
)

func looksLikeCID(s string) bool {
	return cidV0Re.MatchString(s) || cidV1Re.MatchString(s)
}

// isImmutableURI reports whether a metadata URI is content-addressed
// (`ipfs://` or a bare CID), meaning identical URI implies identical
// content and a TTL refresh can skip the gateway round trip. http(s)
// URIs are mutable and always re-fetch.
func isImmutableURI(uri string) bool {
	uri = strings.TrimSpace(uri)
	if strings.HasPrefix(uri, "ipfs://") {
		return true
	}
	return looksLikeCID(strings.SplitN(uri, "/", 2)[0])
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
