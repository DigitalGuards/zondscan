package qns

import (
	"backendAPI/db"
	"backendAPI/qrladdress"
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"os"
	"strings"
)

const (
	registryEnv = "QNS_REGISTRY_ADDRESS"
	chainIDEnv  = "QNS_EXPECTED_CHAIN_ID"
)

var (
	ErrNotConfigured       = errors.New("QNS resolution is not configured")
	ErrInvalidConfig       = errors.New("invalid QNS configuration")
	ErrChainMismatch       = errors.New("QNS chain ID mismatch")
	ErrRegistryUnavailable = errors.New("QNS registry code is unavailable")
	ErrRPC                 = errors.New("QNS RPC failure")
	ErrMalformedResponse   = errors.New("malformed QNS RPC response")
	ErrInvalidAddress      = errors.New("invalid QNS address response")
)

// Config is an environment-derived QNS deployment target. Fields stay
// private so callers cannot bypass the validation performed by LoadConfig.
type Config struct {
	registryAddress string
	expectedChainID string
}

// ConfigFromEnv reads the optional QNS deployment configuration. Both values
// are required to enable resolution; an unset deployment remains a clear,
// fail-closed service state.
func ConfigFromEnv() (Config, error) {
	return LoadConfig(os.Getenv)
}

// LoadConfig is ConfigFromEnv with an injectable environment lookup for tests.
func LoadConfig(getenv func(string) string) (Config, error) {
	registryRaw := strings.TrimSpace(getenv(registryEnv))
	chainRaw := strings.TrimSpace(getenv(chainIDEnv))
	if registryRaw == "" || chainRaw == "" {
		return Config{}, fmt.Errorf("%w: set %s and %s", ErrNotConfigured, registryEnv, chainIDEnv)
	}
	registry, ok := qrladdress.Canonicalize(registryRaw)
	if !ok || isZeroAddress(registry) {
		return Config{}, fmt.Errorf("%w: %s must be a non-zero QIP-55 address", ErrInvalidConfig, registryEnv)
	}
	chainID, err := normalizeChainID(chainRaw)
	if err != nil {
		return Config{}, fmt.Errorf("%w: %s: %v", ErrInvalidConfig, chainIDEnv, err)
	}
	return Config{registryAddress: registry, expectedChainID: chainID}, nil
}

// CacheKey identifies one deployment without exposing configuration fields to
// the route package. Address casing aliases collapse to one key.
func (c Config) CacheKey() string {
	return strings.ToLower(c.registryAddress) + ":" + c.expectedChainID
}

func (c Config) valid() bool {
	return c.registryAddress != "" && c.expectedChainID != ""
}

// RPCFunc matches db.NodeRPC and permits deterministic mocked protocol tests.
type RPCFunc func(context.Context, string, []interface{}) (json.RawMessage, *db.RPCError, error)

type Client struct {
	call RPCFunc
}

func NewClient(call RPCFunc) *Client {
	return &Client{call: call}
}

// Deployment is returned only after chain ID and registry bytecode checks
// pass. ResolveName rejects its zero value, keeping the two-step cached route
// flow fail closed.
type Deployment struct {
	registryAddress string
	expectedChainID string
}

type MissingRecord string

const (
	MissingNone     MissingRecord = ""
	MissingResolver MissingRecord = "resolver"
	MissingAddress  MissingRecord = "address"
)

type Resolution struct {
	Name    string
	Address string
	Missing MissingRecord
}

// CheckDeployment confirms the connected node is on the configured chain and
// that the configured registry currently has non-zero bytecode.
func (c *Client) CheckDeployment(ctx context.Context, config Config) (Deployment, error) {
	if c == nil || c.call == nil || !config.valid() {
		return Deployment{}, ErrNotConfigured
	}

	chainResult, err := c.rpcString(ctx, "qrl_chainId", []interface{}{})
	if err != nil {
		return Deployment{}, err
	}
	actualChainID, err := normalizeChainID(chainResult)
	if err != nil {
		return Deployment{}, fmt.Errorf("%w: qrl_chainId", ErrMalformedResponse)
	}
	if actualChainID != config.expectedChainID {
		return Deployment{}, fmt.Errorf(
			"%w: expected %s, got %s",
			ErrChainMismatch,
			config.expectedChainID,
			actualChainID,
		)
	}

	code, err := c.rpcString(
		ctx,
		"qrl_getCode",
		[]interface{}{config.registryAddress, "latest"},
	)
	if err != nil {
		return Deployment{}, err
	}
	hasCode, err := hasNonZeroCode(code)
	if err != nil {
		return Deployment{}, err
	}
	if !hasCode {
		return Deployment{}, ErrRegistryUnavailable
	}

	return Deployment{
		registryAddress: config.registryAddress,
		expectedChainID: config.expectedChainID,
	}, nil
}

// ResolveName performs resolver(bytes32) against the checked registry and
// addr(bytes32) against the returned resolver using exact QRVM64 ABI words.
func (c *Client) ResolveName(
	ctx context.Context,
	deployment Deployment,
	name string,
) (Resolution, error) {
	if c == nil || c.call == nil || deployment.registryAddress == "" || deployment.expectedChainID == "" {
		return Resolution{}, ErrNotConfigured
	}
	normalized, err := NormalizeName(name)
	if err != nil {
		return Resolution{}, err
	}
	node := Namehash(normalized)

	resolverResult, err := c.qrlCall(
		ctx,
		deployment.registryAddress,
		resolverCallData(node),
	)
	if err != nil {
		return Resolution{}, err
	}
	resolver, zero, err := decodeAddressWord(resolverResult)
	if err != nil {
		return Resolution{}, err
	}
	if zero {
		return Resolution{Name: normalized, Missing: MissingResolver}, nil
	}

	addressResult, err := c.qrlCall(ctx, resolver, addrCallData(node))
	if err != nil {
		return Resolution{}, err
	}
	address, zero, err := decodeAddressWord(addressResult)
	if err != nil {
		return Resolution{}, err
	}
	if zero {
		return Resolution{Name: normalized, Missing: MissingAddress}, nil
	}
	return Resolution{Name: normalized, Address: address}, nil
}

func (c *Client) qrlCall(ctx context.Context, to, data string) (string, error) {
	return c.rpcString(ctx, "qrl_call", []interface{}{
		map[string]interface{}{"to": to, "data": data},
		"latest",
	})
}

func (c *Client) rpcString(
	ctx context.Context,
	method string,
	params []interface{},
) (string, error) {
	raw, rpcErr, err := c.call(ctx, method, params)
	if err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return "", ctxErr
		}
		return "", fmt.Errorf("%w: %s transport", ErrRPC, method)
	}
	if rpcErr != nil {
		return "", fmt.Errorf("%w: %s returned code %d", ErrRPC, method, rpcErr.Code)
	}
	var result string
	if err := json.Unmarshal(raw, &result); err != nil {
		return "", fmt.Errorf("%w: %s result", ErrMalformedResponse, method)
	}
	return result, nil
}

func normalizeChainID(value string) (string, error) {
	if value == "" {
		return "", errors.New("empty chain ID")
	}
	base := 10
	digits := value
	if strings.HasPrefix(value, "0x") || strings.HasPrefix(value, "0X") {
		base = 16
		digits = value[2:]
	}
	if digits == "" {
		return "", errors.New("empty chain ID")
	}
	for _, c := range []byte(digits) {
		valid := c >= '0' && c <= '9'
		if base == 16 {
			valid = valid || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')
		}
		if !valid {
			return "", errors.New("chain ID is not a positive integer")
		}
	}
	n, ok := new(big.Int).SetString(digits, base)
	if !ok || n.Sign() <= 0 {
		return "", errors.New("chain ID is not a positive integer")
	}
	return n.String(), nil
}

func hasNonZeroCode(value string) (bool, error) {
	if !strings.HasPrefix(value, "0x") {
		return false, ErrMalformedResponse
	}
	body := value[2:]
	if body == "" || isZeroHex(body) {
		return false, nil
	}
	if len(body)%2 != 0 {
		return false, ErrMalformedResponse
	}
	code, err := hex.DecodeString(body)
	if err != nil {
		return false, ErrMalformedResponse
	}
	return len(code) > 0, nil
}

func decodeAddressWord(value string) (address string, zero bool, err error) {
	if value == "0x" {
		return "", true, nil
	}
	if !strings.HasPrefix(value, "0x") || len(value) != 2+qrladdress.HexLength {
		return "", false, ErrMalformedResponse
	}
	// qrl_call returns raw ABI bytes serialized as hex. The serializer's
	// letter casing is not an address checksum, so normalize it before
	// deriving the public QIP-55 text form.
	body := strings.ToLower(value[2:])
	if _, err := hex.DecodeString(body); err != nil {
		return "", false, ErrMalformedResponse
	}
	if isZeroHex(body) {
		return "", true, nil
	}
	canonical, ok := qrladdress.Canonicalize("Q" + body)
	if !ok {
		return "", false, ErrInvalidAddress
	}
	return canonical, false, nil
}

func isZeroAddress(address string) bool {
	if len(address) < 2 {
		return false
	}
	return isZeroHex(address[1:])
}

func isZeroHex(value string) bool {
	for _, c := range []byte(value) {
		if c != '0' {
			return false
		}
	}
	return value != ""
}
