package networkprofile

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"
)

const maxIdentityResponse = 2 << 20

// Sources includes every explicitly configured RPC transport. NODE_URL can
// still be used by API/maintenance even when NODE_URLS is also configured.
func Sources(getenv func(string) string) []string {
	var sources []string
	seen := map[string]bool{}
	for _, key := range []string{"NODE_URL", "NODE_URLS", "MEMPOOL_NODE_URL", "TRACE_NODE_URL"} {
		for _, source := range strings.Split(getenv(key), ",") {
			source = strings.TrimSpace(source)
			if source != "" && !seen[source] {
				sources = append(sources, source)
				seen[source] = true
			}
		}
	}
	return sources
}

// ValidateSources checks chain ID and genesis independently. Network labels or
// chain IDs alone cannot distinguish resets that reuse the same chain ID.
func ValidateSources(ctx context.Context, profile Profile, sources []string) error {
	if !profile.Pinned {
		return nil
	}
	if len(sources) == 0 || len(sources) > 16 {
		return errors.New("pinned network requires 1-16 configured RPC sources")
	}
	for i, source := range sources {
		sourceCtx, cancel := context.WithTimeout(ctx, 8*time.Second)
		err := validateSource(sourceCtx, profile.Identity, source)
		cancel()
		if err != nil {
			return fmt.Errorf("RPC source %d identity check failed: %w", i+1, err)
		}
	}
	return nil
}

func validateSource(ctx context.Context, expected Identity, source string) error {
	var chainID string
	if err := requestIdentity(ctx, source, "qrl_chainId", []interface{}{}, &chainID); err != nil {
		return err
	}
	canonicalID, err := NormalizeChainID(chainID)
	if err != nil || canonicalID != expected.ChainID {
		return errors.New("chain ID mismatch")
	}
	var block struct {
		Hash string `json:"hash"`
	}
	if err := requestIdentity(ctx, source, "qrl_getBlockByNumber", []interface{}{"0x0", false}, &block); err != nil {
		return err
	}
	genesis, err := NormalizeGenesisHash(block.Hash)
	if err != nil || genesis != expected.GenesisHash {
		return errors.New("genesis hash mismatch")
	}
	return nil
}

func requestIdentity(ctx context.Context, source, method string, params []interface{}, result interface{}) error {
	parsed, err := url.Parse(source)
	if err != nil || parsed.Host == "" || parsed.Fragment != "" {
		return errors.New("invalid RPC transport")
	}
	payload, _ := json.Marshal(map[string]interface{}{"jsonrpc": "2.0", "id": 1, "method": method, "params": params})
	var body []byte
	switch parsed.Scheme {
	case "http", "https":
		request, err := http.NewRequestWithContext(ctx, http.MethodPost, source, bytes.NewReader(payload))
		if err != nil {
			return errors.New("invalid RPC request")
		}
		request.Header.Set("Content-Type", "application/json")
		client := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
		response, err := client.Do(request)
		if err != nil {
			return errors.New("RPC transport unavailable")
		}
		defer response.Body.Close()
		if response.StatusCode != http.StatusOK {
			return errors.New("RPC identity request rejected")
		}
		body, err = io.ReadAll(io.LimitReader(response.Body, maxIdentityResponse+1))
		if err != nil || len(body) > maxIdentityResponse {
			return errors.New("RPC identity response unreadable or too large")
		}
	case "ws", "wss":
		connection, _, err := websocket.DefaultDialer.DialContext(ctx, source, nil)
		if err != nil {
			return errors.New("RPC transport unavailable")
		}
		defer connection.Close()
		stop := context.AfterFunc(ctx, func() { connection.Close() })
		defer stop()
		connection.SetReadLimit(maxIdentityResponse)
		if deadline, ok := ctx.Deadline(); ok {
			connection.SetWriteDeadline(deadline)
			connection.SetReadDeadline(deadline)
		}
		if err := connection.WriteMessage(websocket.TextMessage, payload); err != nil {
			return errors.New("RPC identity request failed")
		}
		_, body, err = connection.ReadMessage()
		if err != nil {
			return errors.New("RPC identity response unreadable or too large")
		}
	default:
		return errors.New("RPC identity checks require HTTP(S) or WS(S)")
	}
	var response struct {
		JSONRPC string          `json:"jsonrpc"`
		ID      int             `json:"id"`
		Result  json.RawMessage `json:"result"`
		Error   json.RawMessage `json:"error"`
	}
	if err := json.Unmarshal(body, &response); err != nil || response.JSONRPC != "2.0" || response.ID != 1 || len(response.Result) == 0 || string(response.Result) == "null" || (len(response.Error) > 0 && string(response.Error) != "null") {
		return errors.New("RPC identity method unavailable or response invalid")
	}
	if err := json.Unmarshal(response.Result, result); err != nil {
		return errors.New("RPC identity result invalid")
	}
	return nil
}

var runtimeProfile atomic.Pointer[Profile]

// Activate installs the immutable profile after database binding. Runtime
// callers probe around each execution request so a reset endpoint cannot
// return data that is accepted under the previous startup identity.
func Activate(profile Profile) { runtimeProfile.Store(&profile) }

func GuardSource(ctx context.Context, source string) error {
	profile := runtimeProfile.Load()
	if profile == nil || !profile.Pinned {
		return nil
	}
	probeCtx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()
	return validateSource(probeCtx, profile.Identity, source)
}
