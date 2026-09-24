// Package beaconprofile verifies the syncer's independently configured beacon
// source. Execution genesis hashes and beacon genesis roots are separate pins.
package beaconprofile

import (
	"QRL2MongoDB/networkprofile"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync/atomic"
	"time"
)

type Profile struct {
	Source      string
	GenesisRoot string
	GenesisTime string
}

var active atomic.Pointer[Profile]

func Parse(getenv func(string) string, pinned bool) (*Profile, error) {
	if !pinned {
		return nil, nil
	}
	profile := &Profile{Source: strings.TrimRight(strings.TrimSpace(getenv("BEACONCHAIN_API")), "/"), GenesisTime: strings.TrimSpace(getenv("EXPECTED_BEACON_GENESIS_TIME"))}
	parsed, err := url.Parse(profile.Source)
	if err != nil || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.RawQuery != "" || parsed.Fragment != "" {
		return nil, errors.New("pinned syncer requires a valid BEACONCHAIN_API HTTP(S) base URL")
	}
	profile.GenesisRoot, err = networkprofile.NormalizeGenesisHash(strings.TrimSpace(getenv("EXPECTED_BEACON_GENESIS_VALIDATORS_ROOT")))
	if err != nil {
		return nil, errors.New("pinned syncer requires EXPECTED_BEACON_GENESIS_VALIDATORS_ROOT as a nonzero 32-byte hash")
	}
	timestamp, err := strconv.ParseUint(profile.GenesisTime, 10, 64)
	if err != nil || timestamp == 0 || strconv.FormatUint(timestamp, 10) != profile.GenesisTime {
		return nil, errors.New("pinned syncer requires EXPECTED_BEACON_GENESIS_TIME as a positive canonical decimal")
	}
	return profile, nil
}

// Configure runs before any MongoDB bootstrap. Unpinned v2 deployments retain
// their existing beacon behavior and require no new settings.
func Configure(ctx context.Context, getenv func(string) string, pinned bool) error {
	profile, err := Parse(getenv, pinned)
	if err != nil {
		return err
	}
	if profile != nil {
		if err := verify(ctx, profile); err != nil {
			return err
		}
	}
	active.Store(profile)
	return nil
}

func GuardSource(ctx context.Context, source string) error {
	profile := active.Load()
	if profile == nil {
		return nil
	}
	if strings.TrimRight(source, "/") != profile.Source {
		return errors.New("beacon source changed after network validation")
	}
	return verify(ctx, profile)
}

func verify(ctx context.Context, profile *Profile) error {
	probeCtx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()
	request, err := http.NewRequestWithContext(probeCtx, http.MethodGet, profile.Source+"/eth/v1/beacon/genesis", nil)
	if err != nil {
		return errors.New("invalid beacon identity request")
	}
	client := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	response, err := client.Do(request)
	if err != nil {
		return errors.New("beacon identity transport unavailable")
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return errors.New("beacon genesis endpoint unavailable")
	}
	body, err := io.ReadAll(io.LimitReader(response.Body, (64<<10)+1))
	if err != nil || len(body) > 64<<10 {
		return errors.New("beacon genesis response unreadable or too large")
	}
	var envelope struct {
		Data struct {
			Time string `json:"genesis_time"`
			Root string `json:"genesis_validators_root"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &envelope); err != nil {
		return errors.New("invalid beacon genesis response")
	}
	root, err := networkprofile.NormalizeGenesisHash(envelope.Data.Root)
	if err != nil || root != profile.GenesisRoot || envelope.Data.Time != profile.GenesisTime {
		return errors.New("beacon network identity mismatch")
	}
	return nil
}
