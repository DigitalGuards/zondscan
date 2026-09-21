// Package explainauth authenticates contract creators before an AI
// explanation regeneration can consume provider credit.
package explainauth

import (
	"context"
	"errors"
	"io"
	"time"

	"backendAPI/models"
)

const (
	ChallengeVersion = "ZONDSCAN-AUTH-v1"
	SchemeVersion    = "QRL-SIGN-MSG-v1"
	RegenerateAction = "contract.explain.regenerate"
	ChallengeTTL     = 5 * time.Minute

	challengeMethod = "POST"
)

var (
	ErrNotConfigured        = errors.New("AI explanation authorization is not configured")
	ErrInvalidConfig        = errors.New("invalid AI explanation authorization configuration")
	ErrContractNotFound     = errors.New("contract not found")
	ErrCreatorUnavailable   = errors.New("contract creator is unavailable")
	ErrInvalidAuthorization = errors.New("invalid, expired, or already used authorization proof")
	ErrUnavailable          = errors.New("AI explanation authorization is unavailable")
)

// Config contains the trusted values committed into every authorization
// challenge. NewConfig validates and canonicalizes both fields.
type Config struct {
	origin          string
	expectedChainID string
}

// ChallengeResponse is safe to return to the browser. The browser signs
// MessageHex verbatim and never reconstructs the canonical text.
type ChallengeResponse struct {
	ChallengeID string `json:"challengeId"`
	MessageHex  string `json:"messageHex"`
	Signer      string `json:"signer"`
	Contract    string `json:"contract"`
	Origin      string `json:"origin"`
	ChainID     string `json:"chainId"`
	ExpiresAt   string `json:"expiresAt"`
}

// SignedMessageProof is the exact rich response returned by qrl_signMessage.
type SignedMessageProof struct {
	Signature     string `json:"signature"`
	PublicKey     string `json:"publicKey"`
	Descriptor    string `json:"descriptor"`
	Signer        string `json:"signer"`
	Digest        string `json:"digest"`
	SchemeVersion string `json:"schemeVersion"`
}

// AuthorizationRequest is submitted only for ?regenerate=1 calls.
type AuthorizationRequest struct {
	ChallengeID string             `json:"challengeId"`
	Proof       SignedMessageProof `json:"proof"`
}

// ChallengeStore persists shared single-use challenges. Consume must be an
// atomic compare-and-delete operation and must enforce ExpiresAt > now.
type ChallengeStore interface {
	Create(context.Context, models.ContractExplainChallenge) error
	Load(context.Context, string) (models.ContractExplainChallenge, bool, error)
	Consume(context.Context, models.ContractExplainChallenge, time.Time) (bool, error)
}

// ContractReader resolves the current contract record and creator.
type ContractReader interface {
	ReadContract(context.Context, string) (models.ContractInfo, error)
}

type ContractReaderFunc func(context.Context, string) (models.ContractInfo, error)

func (f ContractReaderFunc) ReadContract(ctx context.Context, address string) (models.ContractInfo, error) {
	return f(ctx, address)
}

// ChainIDReader returns the connected execution node's current chain ID.
type ChainIDReader interface {
	ChainID(context.Context) (string, error)
}

type ChainIDReaderFunc func(context.Context) (string, error)

func (f ChainIDReaderFunc) ChainID(ctx context.Context) (string, error) {
	return f(ctx)
}

// Dependencies are injected so authorization and replay tests remain local.
type Dependencies struct {
	Store     ChallengeStore
	Contracts ContractReader
	Chain     ChainIDReader
	Random    io.Reader
	Now       func() time.Time
}
