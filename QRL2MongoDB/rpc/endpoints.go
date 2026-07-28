package rpc

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"
)

// endpointSelector keeps an in-memory health view across the configured
// JSON-RPC endpoints and routes outbound RPC calls accordingly. It is a
// best-effort, single-process construct: state lives only in this binary, and
// rotation reacts to live request signals (no separate health-poll goroutine).
//
// Endpoint priority order is determined by the env var `NODE_URLS` (comma
// separated). For backwards compatibility, if `NODE_URLS` is not set, a single
// `NODE_URL` is used.
type endpointSelector struct {
	mu     sync.RWMutex
	states []*endpointState
}

type endpointState struct {
	url                 string
	consecutiveFailures int
	down                bool
	downSince           time.Time
}

const (
	// downAfterFailures: consecutive failed attempts that flip an endpoint to
	// the "down" pool. Smaller than the backend's because the syncer's
	// per-endpoint retry already does 3 attempts before reporting failure.
	downAfterFailures = 1
	// downCooldown: how long to skip a down endpoint before letting it back
	// into the regular rotation. Successful health-check on retry resets it.
	downCooldown = 60 * time.Second
)

var (
	selectorOnce sync.Once
	selector     *endpointSelector
)

// Endpoints returns the global selector, initialising it from env vars on
// first use. Callers that need to read or mutate selector state should go
// through this.
func Endpoints() *endpointSelector {
	selectorOnce.Do(func() {
		selector = newEndpointSelectorFromEnv()
	})
	return selector
}

func newEndpointSelectorFromEnv() *endpointSelector {
	urls := parseEndpointList(os.Getenv("NODE_URLS"), os.Getenv("NODE_URL"))
	s := &endpointSelector{}
	for _, u := range urls {
		s.states = append(s.states, &endpointState{url: u})
	}
	if len(s.states) == 0 {
		zap.L().Warn("[rpc-endpoints] no NODE_URL or NODE_URLS configured")
	} else {
		urlList := make([]string, 0, len(s.states))
		for _, st := range s.states {
			urlList = append(urlList, st.url)
		}
		zap.L().Info("[rpc-endpoints] initialised",
			zap.Strings("urls", urlList),
			zap.Int("count", len(s.states)))
	}
	return s
}

// parseEndpointList prefers a comma-separated list. It falls back to the
// single-URL legacy form if the list is empty.
func parseEndpointList(list, single string) []string {
	if list != "" {
		var urls []string
		for _, p := range strings.Split(list, ",") {
			p = strings.TrimSpace(p)
			if p != "" {
				urls = append(urls, p)
			}
		}
		if len(urls) > 0 {
			return urls
		}
	}
	if single != "" {
		return []string{single}
	}
	return nil
}

// orderedAttempts returns the endpoint URLs in the order callers should try
// them: healthy first, then any whose cooldown expired, then anything still
// in-cooldown as a last resort. Returning all configured endpoints ensures we
// never give up just because every endpoint is currently flagged down.
func (s *endpointSelector) orderedAttempts() []*endpointState {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if len(s.states) == 0 {
		return nil
	}
	now := time.Now()
	healthy := make([]*endpointState, 0, len(s.states))
	cooled := make([]*endpointState, 0, len(s.states))
	stillDown := make([]*endpointState, 0, len(s.states))
	for _, st := range s.states {
		if !st.down {
			healthy = append(healthy, st)
			continue
		}
		if now.Sub(st.downSince) >= downCooldown {
			cooled = append(cooled, st)
		} else {
			stillDown = append(stillDown, st)
		}
	}
	out := make([]*endpointState, 0, len(s.states))
	out = append(out, healthy...)
	out = append(out, cooled...)
	out = append(out, stillDown...)
	return out
}

// markFailed bumps the failure counter; flips an endpoint to "down" once it
// crosses downAfterFailures.
func (s *endpointSelector) markFailed(url string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, st := range s.states {
		if st.url != url {
			continue
		}
		st.consecutiveFailures++
		if !st.down && st.consecutiveFailures >= downAfterFailures {
			st.down = true
			st.downSince = time.Now()
			zap.L().Error("[rpc-endpoints] endpoint flipped to down",
				zap.String("url", url),
				zap.Int("failures", st.consecutiveFailures))
		}
		return
	}
}

// markSucceeded resets failure counters and exits the down state.
func (s *endpointSelector) markSucceeded(url string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, st := range s.states {
		if st.url != url {
			continue
		}
		if st.down {
			zap.L().Info("[rpc-endpoints] endpoint recovered",
				zap.String("url", url),
				zap.Duration("downFor", time.Since(st.downSince)))
		}
		st.consecutiveFailures = 0
		st.down = false
		return
	}
}

// CurrentURL returns the first preferred endpoint URL, useful for diagnostic
// logging and for callers that just need a string.
func (s *endpointSelector) CurrentURL() string {
	attempts := s.orderedAttempts()
	if len(attempts) == 0 {
		return ""
	}
	return attempts[0].url
}

// PrimaryURL returns the first endpoint as configured (env order), regardless
// of health state. Use for primary-only methods (e.g. txpool_*, debug_*) that
// must not fail over to a public/foundation node which may not expose them.
func (s *endpointSelector) PrimaryURL() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if len(s.states) == 0 {
		return ""
	}
	return s.states[0].url
}

// AllURLs returns every configured URL in env order. Used for /health and
// startup logs.
func (s *endpointSelector) AllURLs() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]string, 0, len(s.states))
	for _, st := range s.states {
		out = append(out, st.url)
	}
	return out
}

// ProbeChainHeadCtx is a lightweight health probe used by the /health
// handler: it issues `qrl_blockNumber` with a single attempt per endpoint
// and no retry budget, and returns the parsed block height. It runs
// synchronously and honours ctx cancellation/deadline for every endpoint
// attempt, so the handler can bound the probe with the request context
// without leaking a goroutine when the client disconnects.
func ProbeChainHeadCtx(ctx context.Context) (uint64, error) {
	body, err := doNodeRPCCtx(ctx, []byte(`{"jsonrpc":"2.0","method":"qrl_blockNumber","params":[],"id":1}`))
	if err != nil {
		return 0, err
	}
	var resp struct {
		Result string `json:"result"`
		Error  *struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return 0, fmt.Errorf("unmarshal qrl_blockNumber: %w", err)
	}
	if resp.Error != nil {
		return 0, fmt.Errorf("rpc error %d: %s", resp.Error.Code, resp.Error.Message)
	}
	if !strings.HasPrefix(resp.Result, "0x") {
		return 0, fmt.Errorf("unexpected qrl_blockNumber result: %q", resp.Result)
	}
	height, err := strconv.ParseUint(resp.Result[2:], 16, 64)
	if err != nil {
		return 0, fmt.Errorf("parse height: %w", err)
	}
	return height, nil
}

// doNodeRPC posts a JSON-RPC body across the endpoint rotation, honouring
// ctx for every attempt. firstRetries is the retry budget for the preferred
// endpoint (transient network blips against the primary recover without
// burning a failover slot); subsequent endpoints get a single attempt each.
//
// Returns the response body bytes on success. The caller is responsible for
// any application-level (JSON-RPC error field) checks; this helper only
// treats transport-layer failures as failover triggers.
func doNodeRPC(ctx context.Context, reqBody []byte, firstRetries int) ([]byte, error) {
	sel := Endpoints()
	attempts := sel.orderedAttempts()
	if len(attempts) == 0 {
		return nil, fmt.Errorf("no node endpoints configured (set NODE_URLS or NODE_URL)")
	}

	var lastErr error
	for i, st := range attempts {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		retries := 1
		if i == 0 {
			retries = firstRetries
		}
		body, err := postWithRetry(ctx, st.url, reqBody, retries)
		if err == nil {
			sel.markSucceeded(st.url)
			return body, nil
		}
		sel.markFailed(st.url)
		lastErr = err
		if i < len(attempts)-1 {
			zap.L().Warn("[rpc-endpoints] rotating to next endpoint",
				zap.String("failedUrl", st.url),
				zap.Error(err),
				zap.String("nextUrl", attempts[i+1].url))
		}
	}
	return nil, fmt.Errorf("all node endpoints exhausted; last error: %w", lastErr)
}

// doNodeRPCCtx posts with a single attempt per endpoint, used by the health
// probe so cancellation/deadline propagate to the in-flight HTTP request.
func doNodeRPCCtx(ctx context.Context, reqBody []byte) ([]byte, error) {
	return doNodeRPC(ctx, reqBody, 1)
}

// DoNodeRPC posts a JSON-RPC body to the first healthy node endpoint with a
// 3-attempt exponential-backoff budget, rotating to the next endpoint on
// failure.
func DoNodeRPC(reqBody []byte) ([]byte, error) {
	return doNodeRPC(context.Background(), reqBody, 3)
}

// postWithRetry executes the request against a single URL with the given
// retry budget and the existing exponential-backoff schedule (1s, 2s, 4s),
// honouring ctx for the request and the backoff waits.
// Returns the response body on success.
func postWithRetry(ctx context.Context, url string, reqBody []byte, retries int) ([]byte, error) {
	if retries < 1 {
		retries = 1
	}
	var lastErr error
	for attempt := 0; attempt < retries; attempt++ {
		req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(reqBody))
		if err != nil {
			lastErr = err
			continue
		}
		req.Header.Set("Content-Type", "application/json")

		resp, err := GetHTTPClient().Do(req)
		if err == nil && resp != nil {
			if resp.StatusCode >= 200 && resp.StatusCode < 300 {
				body, readErr := io.ReadAll(resp.Body)
				resp.Body.Close()
				if readErr == nil {
					return body, nil
				}
				lastErr = readErr
			} else {
				lastErr = fmt.Errorf("HTTP %d", resp.StatusCode)
				resp.Body.Close()
			}
		} else {
			lastErr = err
			if resp != nil {
				resp.Body.Close()
			}
		}

		if attempt < retries-1 {
			backoff := time.Duration(1<<uint(attempt)) * time.Second
			zap.L().Warn("RPC call failed, retrying...",
				zap.Error(lastErr),
				zap.Int("retry", attempt+1),
				zap.Duration("backoff", backoff),
				zap.String("url", url))
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(backoff):
			}
		}
	}
	return nil, lastErr
}
