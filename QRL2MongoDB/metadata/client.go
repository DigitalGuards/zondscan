package metadata

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"syscall"
	"time"
)

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

// errGatewayRateLimited is returned by fetchBody on HTTP 429. It is NOT a
// row failure: the row's state is preserved (no fetchError, no backoff
// bump) and the tick aborts its remaining batch, because once the gateway
// has rate-limited us every subsequent request this window will fail the
// same way and would pointlessly stamp failure churn onto healthy rows.
// The default gateway (the wallet backend's IPFS proxy) allows
// 60 req/min/IP, below this fetcher's worst-case budget of two 25-row
// batches per 30s tick, so 429s are expected during backlog drains.
var errGatewayRateLimited = errors.New("gateway rate limited (HTTP 429)")

// fetchBody issues an HTTP GET with the configured timeout and size cap.
func (s *Service) fetchBody(ctx context.Context, resolvedURL string) ([]byte, error) {
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

	if resp.StatusCode == http.StatusTooManyRequests {
		return nil, errGatewayRateLimited
	}
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
