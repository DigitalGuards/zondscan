package explainauth

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"
	"time"

	"backendAPI/models"
	"backendAPI/qrladdress"

	cryptomldsa "github.com/theQRL/go-qrllib/crypto/ml_dsa_87"
	qrllibwallet "github.com/theQRL/go-qrllib/wallet"
	"github.com/theQRL/go-qrllib/wallet/common"
	"github.com/theQRL/go-qrllib/wallet/common/descriptor"
	"golang.org/x/crypto/sha3"
)

const (
	nonceBytes      = 32
	digestBytes     = 64
	descriptorBytes = 3
)

type Service struct {
	config    Config
	store     ChallengeStore
	contracts ContractReader
	chain     ChainIDReader
	random    io.Reader
	now       func() time.Time
}

var (
	defaultMu      sync.RWMutex
	defaultService *Service
)

func NewService(config Config, dependencies Dependencies) (*Service, error) {
	if config.origin == "" || config.expectedChainID == "" {
		return nil, ErrNotConfigured
	}
	if dependencies.Store == nil || dependencies.Contracts == nil || dependencies.Chain == nil {
		return nil, fmt.Errorf("%w: store, contract reader, and chain reader are required", ErrInvalidConfig)
	}
	if dependencies.Random == nil {
		dependencies.Random = rand.Reader
	}
	if dependencies.Now == nil {
		dependencies.Now = time.Now
	}
	return &Service{
		config:    config,
		store:     dependencies.Store,
		contracts: dependencies.Contracts,
		chain:     dependencies.Chain,
		random:    dependencies.Random,
		now:       dependencies.Now,
	}, nil
}

func SetDefault(service *Service) {
	defaultMu.Lock()
	defaultService = service
	defaultMu.Unlock()
}

func Default() *Service {
	defaultMu.RLock()
	defer defaultMu.RUnlock()
	return defaultService
}

func (s *Service) IssueChallenge(ctx context.Context, address string) (*ChallengeResponse, error) {
	contract, ok := qrladdress.Canonicalize(address)
	if !ok {
		return nil, ErrContractNotFound
	}
	if err := s.ensureExpectedChain(ctx); err != nil {
		return nil, err
	}
	creator, err := s.currentCreator(ctx, contract)
	if err != nil {
		return nil, err
	}

	nonce := make([]byte, nonceBytes)
	if _, err := io.ReadFull(s.random, nonce); err != nil {
		return nil, fmt.Errorf("%w: generate challenge nonce: %v", ErrUnavailable, err)
	}
	now := s.now().UTC().Truncate(time.Second)
	challenge := models.ContractExplainChallenge{
		ID:              hex.EncodeToString(nonce),
		Version:         ChallengeVersion,
		Action:          RegenerateAction,
		Origin:          s.config.origin,
		ChainID:         s.config.expectedChainID,
		Method:          challengeMethod,
		Route:           challengeRoute(contract),
		ContractAddress: contract,
		SignerAddress:   creator,
		IssuedAt:        now,
		ExpiresAt:       now.Add(ChallengeTTL),
	}
	challenge.Message = canonicalMessage(challenge)
	if err := s.store.Create(ctx, challenge); err != nil {
		return nil, fmt.Errorf("%w: persist challenge: %v", ErrUnavailable, err)
	}
	return &ChallengeResponse{
		ChallengeID: challenge.ID,
		MessageHex:  "0x" + hex.EncodeToString(challenge.Message),
		Signer:      creator,
		Contract:    contract,
		Origin:      challenge.Origin,
		ChainID:     challenge.ChainID,
		ExpiresAt:   challenge.ExpiresAt.Format(time.RFC3339),
	}, nil
}

func (s *Service) AuthorizeRegeneration(ctx context.Context, address string, request AuthorizationRequest) error {
	contract, ok := qrladdress.Canonicalize(address)
	if !ok || validateAuthorizationEncoding(request) != nil {
		return ErrInvalidAuthorization
	}
	challenge, found, err := s.store.Load(ctx, request.ChallengeID)
	if err != nil {
		return fmt.Errorf("%w: load challenge: %v", ErrUnavailable, err)
	}
	if !found || challenge.ContractAddress != contract || validateStoredChallenge(challenge, s.config) != nil {
		return ErrInvalidAuthorization
	}
	now := s.now().UTC()
	if !now.Before(challenge.ExpiresAt) {
		return ErrInvalidAuthorization
	}
	if err := s.ensureExpectedChain(ctx); err != nil {
		return err
	}
	creator, err := s.currentCreator(ctx, contract)
	if err != nil {
		if errors.Is(err, ErrUnavailable) {
			return err
		}
		return ErrInvalidAuthorization
	}
	if creator != challenge.SignerAddress {
		return ErrInvalidAuthorization
	}
	if err := verifySignedMessage(challenge, request.Proof); err != nil {
		return ErrInvalidAuthorization
	}

	consumed, err := s.store.Consume(ctx, challenge, now)
	if err != nil {
		return fmt.Errorf("%w: consume challenge: %v", ErrUnavailable, err)
	}
	if !consumed {
		return ErrInvalidAuthorization
	}
	return nil
}

func (s *Service) ensureExpectedChain(ctx context.Context) error {
	chainID, err := s.chain.ChainID(ctx)
	if err != nil {
		return fmt.Errorf("%w: query chain ID: %v", ErrUnavailable, err)
	}
	normalized, err := NormalizeChainID(chainID)
	if err != nil || normalized != s.config.expectedChainID {
		return fmt.Errorf("%w: execution chain does not match configured chain", ErrUnavailable)
	}
	return nil
}

func (s *Service) currentCreator(ctx context.Context, contract string) (string, error) {
	record, err := s.contracts.ReadContract(ctx, contract)
	if err != nil {
		return "", fmt.Errorf("%w: read contract: %v", ErrUnavailable, err)
	}
	if record.ContractAddress == "" {
		return "", ErrContractNotFound
	}
	recordAddress, ok := qrladdress.Canonicalize(record.ContractAddress)
	if !ok || recordAddress != contract {
		return "", ErrContractNotFound
	}
	if !models.IsAuthoritativeCreatorAddressProvenance(record.CreatorAddressProvenance) {
		return "", ErrCreatorUnavailable
	}
	creator, ok := qrladdress.Canonicalize(record.ContractCreatorAddress)
	if !ok || isZeroAddress(creator) {
		return "", ErrCreatorUnavailable
	}
	return creator, nil
}

func isZeroAddress(address string) bool {
	return len(address) == qrladdress.Length && strings.Trim(address[1:], "0") == ""
}

func validateAuthorizationEncoding(request AuthorizationRequest) error {
	if len(request.ChallengeID) != nonceBytes*2 {
		return fmt.Errorf("challengeId must be %d lowercase hex characters", nonceBytes*2)
	}
	for _, r := range request.ChallengeID {
		if (r < '0' || r > '9') && (r < 'a' || r > 'f') {
			return fmt.Errorf("challengeId must be lowercase hex")
		}
	}
	if request.Proof.SchemeVersion != SchemeVersion {
		return fmt.Errorf("unsupported signing scheme")
	}
	if request.Proof.Descriptor != "0x010000" {
		return fmt.Errorf("invalid ML-DSA-87 descriptor")
	}
	if !qrladdress.IsValidCanonicalInput(request.Proof.Signer) {
		return fmt.Errorf("invalid signer")
	}
	if _, err := decodeFixedHex(request.Proof.Signature, cryptomldsa.CRYPTO_BYTES); err != nil {
		return fmt.Errorf("invalid signature: %w", err)
	}
	if _, err := decodeFixedHex(request.Proof.PublicKey, cryptomldsa.CRYPTO_PUBLIC_KEY_BYTES); err != nil {
		return fmt.Errorf("invalid public key: %w", err)
	}
	if _, err := decodeFixedHex(request.Proof.Digest, digestBytes); err != nil {
		return fmt.Errorf("invalid digest: %w", err)
	}
	return nil
}

func verifySignedMessage(challenge models.ContractExplainChallenge, proof SignedMessageProof) error {
	signer, ok := qrladdress.Canonicalize(proof.Signer)
	if !ok || signer != challenge.SignerAddress {
		return fmt.Errorf("signer mismatch")
	}

	descriptorValue, err := decodeFixedHex(proof.Descriptor, descriptorBytes)
	if err != nil || !bytes.Equal(descriptorValue, []byte{0x01, 0x00, 0x00}) {
		return fmt.Errorf("invalid descriptor")
	}
	publicKey, err := decodeFixedHex(proof.PublicKey, cryptomldsa.CRYPTO_PUBLIC_KEY_BYTES)
	if err != nil {
		return err
	}
	desc, err := descriptor.FromBytes(descriptorValue)
	if err != nil || !desc.IsValid() {
		return fmt.Errorf("invalid descriptor")
	}
	addressBytes, err := qrllibwallet.GetAddressFromPKAndDescriptor(publicKey, desc)
	if err != nil {
		return fmt.Errorf("derive signer: %w", err)
	}
	derived := common.ToChecksumAddress(addressBytes)
	if derived != challenge.SignerAddress {
		return fmt.Errorf("public key does not derive expected signer")
	}

	digest := messageDigest(challenge.Message)
	providedDigest, err := decodeFixedHex(proof.Digest, digestBytes)
	if err != nil || subtle.ConstantTimeCompare(digest[:], providedDigest) != 1 {
		return fmt.Errorf("digest mismatch")
	}
	signatureBytes, err := decodeFixedHex(proof.Signature, cryptomldsa.CRYPTO_BYTES)
	if err != nil {
		return err
	}
	var signature [cryptomldsa.CRYPTO_BYTES]byte
	copy(signature[:], signatureBytes)
	var publicKeyArray [cryptomldsa.CRYPTO_PUBLIC_KEY_BYTES]byte
	copy(publicKeyArray[:], publicKey)
	if !cryptomldsa.Verify([]byte(SchemeVersion), digest[:], signature, &publicKeyArray) {
		return fmt.Errorf("signature verification failed")
	}
	return nil
}

func messageDigest(message []byte) [digestBytes]byte {
	shake := sha3.NewShake256()
	_, _ = shake.Write([]byte(SchemeVersion))
	_, _ = shake.Write(message)
	var digest [digestBytes]byte
	_, _ = shake.Read(digest[:])
	return digest
}

func decodeFixedHex(value string, size int) ([]byte, error) {
	if len(value) != 2+size*2 || !strings.HasPrefix(value, "0x") {
		return nil, fmt.Errorf("expected 0x-prefixed %d-byte hex", size)
	}
	decoded, err := hex.DecodeString(value[2:])
	if err != nil || len(decoded) != size {
		return nil, fmt.Errorf("expected 0x-prefixed %d-byte hex", size)
	}
	return decoded, nil
}
