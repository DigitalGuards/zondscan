package explainauth

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"backendAPI/models"
	"backendAPI/qrladdress"

	cryptomldsa "github.com/theQRL/go-qrllib/crypto/ml_dsa_87"
	qrllibwallet "github.com/theQRL/go-qrllib/wallet"
	"github.com/theQRL/go-qrllib/wallet/common"
	"github.com/theQRL/go-qrllib/wallet/common/descriptor"
)

type connectSigningFixture struct {
	MessageHex string `json:"messageHex"`
	Signature  string `json:"signature"`
	PublicKey  string `json:"publicKey"`
	Signer     string `json:"signer"`
	Digest     string `json:"digest"`
}

type memoryChallengeStore struct {
	mu         sync.Mutex
	challenges map[string]models.ContractExplainChallenge
}

func newMemoryChallengeStore() *memoryChallengeStore {
	return &memoryChallengeStore{challenges: make(map[string]models.ContractExplainChallenge)}
}

func (s *memoryChallengeStore) Create(_ context.Context, challenge models.ContractExplainChallenge) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.challenges[challenge.ID]; exists {
		return errors.New("duplicate challenge")
	}
	s.challenges[challenge.ID] = challenge
	return nil
}

func (s *memoryChallengeStore) Load(
	_ context.Context,
	id string,
) (models.ContractExplainChallenge, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	challenge, found := s.challenges[id]
	return challenge, found, nil
}

func (s *memoryChallengeStore) Consume(
	_ context.Context,
	challenge models.ContractExplainChallenge,
	now time.Time,
) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	stored, found := s.challenges[challenge.ID]
	if !found || !now.Before(stored.ExpiresAt) || !reflect.DeepEqual(stored, challenge) {
		return false, nil
	}
	delete(s.challenges, challenge.ID)
	return true, nil
}

func (s *memoryChallengeStore) get(id string) (models.ContractExplainChallenge, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	challenge, found := s.challenges[id]
	return challenge, found
}

func (s *memoryChallengeStore) set(challenge models.ContractExplainChallenge) {
	s.mu.Lock()
	s.challenges[challenge.ID] = challenge
	s.mu.Unlock()
}

type mutableContractReader struct {
	mu      sync.Mutex
	records map[string]models.ContractInfo
}

func (r *mutableContractReader) ReadContract(
	_ context.Context,
	address string,
) (models.ContractInfo, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.records[address], nil
}

func (r *mutableContractReader) set(address string, record models.ContractInfo) {
	r.mu.Lock()
	r.records[address] = record
	r.mu.Unlock()
}

type serviceHarness struct {
	service   *Service
	store     *memoryChallengeStore
	contracts *mutableContractReader
	now       *time.Time
	chainID   *string
	contract  string
	creator   string
	key       *cryptomldsa.MLDSA87
}

func newServiceHarness(t *testing.T) *serviceHarness {
	t.Helper()
	config, err := NewConfig("https://zondscan.com", "0x539")
	if err != nil {
		t.Fatal(err)
	}
	contract := mustCanonicalAddress(t, "Q"+strings.Repeat("1", 128))
	key, creator := testSigner(t)
	now := time.Date(2026, 8, 27, 20, 0, 0, 0, time.UTC)
	chainID := "0x539"
	store := newMemoryChallengeStore()
	contracts := &mutableContractReader{records: map[string]models.ContractInfo{
		contract: {
			ContractAddress:        contract,
			ContractCreatorAddress: creator,
			CreatorAddressProvenance: models.CreatorAddressProvenanceDirectDeployment,
		},
	}}
	service, err := NewService(config, Dependencies{
		Store:     store,
		Contracts: contracts,
		Chain: ChainIDReaderFunc(func(context.Context) (string, error) {
			return chainID, nil
		}),
		Random: bytes.NewReader(bytes.Repeat([]byte{0xab}, nonceBytes)),
		Now:    func() time.Time { return now },
	})
	if err != nil {
		t.Fatal(err)
	}
	return &serviceHarness{
		service: service, store: store, contracts: contracts, now: &now,
		chainID: &chainID, contract: contract, creator: creator, key: key,
	}
}

func (h *serviceHarness) issueAndSign(t *testing.T) (*ChallengeResponse, AuthorizationRequest) {
	t.Helper()
	response, err := h.service.IssueChallenge(context.Background(), h.contract)
	if err != nil {
		t.Fatalf("IssueChallenge: %v", err)
	}
	challenge, found := h.store.get(response.ChallengeID)
	if !found {
		t.Fatal("challenge was not stored")
	}
	messageBytes, err := hex.DecodeString(strings.TrimPrefix(response.MessageHex, "0x"))
	if err != nil || !bytes.Equal(messageBytes, challenge.Message) {
		t.Fatalf("messageHex does not match stored challenge bytes: %v", err)
	}
	digest := messageDigest(challenge.Message)
	signature, err := h.key.SignDeterministic([]byte(SchemeVersion), digest[:])
	if err != nil {
		t.Fatalf("SignDeterministic: %v", err)
	}
	publicKey := h.key.GetPK()
	request := AuthorizationRequest{
		ChallengeID: response.ChallengeID,
		Proof: SignedMessageProof{
			Signature:     "0x" + hex.EncodeToString(signature[:]),
			PublicKey:     "0x" + hex.EncodeToString(publicKey[:]),
			Descriptor:    "0x010000",
			Signer:        h.creator,
			Digest:        "0x" + hex.EncodeToString(digest[:]),
			SchemeVersion: SchemeVersion,
		},
	}
	return response, request
}

func TestServiceIssuesBoundChallengeAndAuthorizesOnce(t *testing.T) {
	harness := newServiceHarness(t)
	response, request := harness.issueAndSign(t)
	if response.Contract != harness.contract || response.Signer != harness.creator ||
		response.Origin != "https://zondscan.com" || response.ChainID != "0x539" {
		t.Fatalf("response = %#v", response)
	}
	if err := harness.service.AuthorizeRegeneration(context.Background(), harness.contract, request); err != nil {
		t.Fatalf("AuthorizeRegeneration: %v", err)
	}
	if err := harness.service.AuthorizeRegeneration(context.Background(), harness.contract, request); !errors.Is(err, ErrInvalidAuthorization) {
		t.Fatalf("replay error = %v", err)
	}
}

func TestVerifySignedMessageAcceptsConnectCanonicalVector(t *testing.T) {
	// Pinned from myqrlwallet-connect/src/signing/__fixtures__/canonical.json.
	fixtureBytes, err := os.ReadFile("testdata/qrl_sign_message_v1.json")
	if err != nil {
		t.Fatal(err)
	}
	var fixture connectSigningFixture
	if err := json.Unmarshal(fixtureBytes, &fixture); err != nil {
		t.Fatal(err)
	}
	message, err := hex.DecodeString(strings.TrimPrefix(fixture.MessageHex, "0x"))
	if err != nil {
		t.Fatal(err)
	}
	challenge := models.ContractExplainChallenge{
		SignerAddress: fixture.Signer,
		Message:       message,
	}
	proof := SignedMessageProof{
		Signature:     fixture.Signature,
		PublicKey:     fixture.PublicKey,
		Descriptor:    "0x010000",
		Signer:        fixture.Signer,
		Digest:        fixture.Digest,
		SchemeVersion: SchemeVersion,
	}
	if err := verifySignedMessage(challenge, proof); err != nil {
		t.Fatalf("verify Connect canonical qrl_signMessage vector: %v", err)
	}
}

func TestInvalidProofDoesNotConsumeChallenge(t *testing.T) {
	harness := newServiceHarness(t)
	_, request := harness.issueAndSign(t)
	valid := request
	request.Proof.Digest = "0x" + strings.Repeat("00", digestBytes)
	if err := harness.service.AuthorizeRegeneration(context.Background(), harness.contract, request); !errors.Is(err, ErrInvalidAuthorization) {
		t.Fatalf("tampered proof error = %v", err)
	}
	if err := harness.service.AuthorizeRegeneration(context.Background(), harness.contract, valid); err != nil {
		t.Fatalf("valid proof after tamper: %v", err)
	}
}

func TestTamperedSigningFieldsDoNotConsumeChallenge(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*AuthorizationRequest)
	}{
		{
			name: "signer",
			mutate: func(request *AuthorizationRequest) {
				request.Proof.Signer = "Q" + strings.Repeat("3", 128)
			},
		},
		{
			name: "descriptor",
			mutate: func(request *AuthorizationRequest) {
				request.Proof.Descriptor = "0x010001"
			},
		},
		{
			name: "public key",
			mutate: func(request *AuthorizationRequest) {
				request.Proof.PublicKey = "0x" + strings.Repeat("00", cryptomldsa.CRYPTO_PUBLIC_KEY_BYTES)
			},
		},
		{
			name: "signature",
			mutate: func(request *AuthorizationRequest) {
				request.Proof.Signature = "0x" + strings.Repeat("00", cryptomldsa.CRYPTO_BYTES)
			},
		},
		{
			name: "digest",
			mutate: func(request *AuthorizationRequest) {
				request.Proof.Digest = "0x" + strings.Repeat("00", digestBytes)
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			harness := newServiceHarness(t)
			_, request := harness.issueAndSign(t)
			valid := request
			test.mutate(&request)
			if err := harness.service.AuthorizeRegeneration(context.Background(), harness.contract, request); !errors.Is(err, ErrInvalidAuthorization) {
				t.Fatalf("tampered proof error = %v", err)
			}
			if err := harness.service.AuthorizeRegeneration(context.Background(), harness.contract, valid); err != nil {
				t.Fatalf("valid proof after tamper: %v", err)
			}
		})
	}
}

func TestConcurrentReplayHasExactlyOneWinner(t *testing.T) {
	harness := newServiceHarness(t)
	_, request := harness.issueAndSign(t)
	start := make(chan struct{})
	results := make(chan error, 2)
	for i := 0; i < 2; i++ {
		go func() {
			<-start
			results <- harness.service.AuthorizeRegeneration(context.Background(), harness.contract, request)
		}()
	}
	close(start)
	var successes, replays int
	for i := 0; i < 2; i++ {
		err := <-results
		switch {
		case err == nil:
			successes++
		case errors.Is(err, ErrInvalidAuthorization):
			replays++
		default:
			t.Fatalf("authorization error = %v", err)
		}
	}
	if successes != 1 || replays != 1 {
		t.Fatalf("successes=%d replays=%d", successes, replays)
	}
}

func TestAuthorizationRejectsExpiredCrossContractAndChangedCreator(t *testing.T) {
	t.Run("expired", func(t *testing.T) {
		harness := newServiceHarness(t)
		_, request := harness.issueAndSign(t)
		*harness.now = harness.now.Add(ChallengeTTL)
		if err := harness.service.AuthorizeRegeneration(context.Background(), harness.contract, request); !errors.Is(err, ErrInvalidAuthorization) {
			t.Fatalf("error = %v", err)
		}
	})

	t.Run("cross contract", func(t *testing.T) {
		harness := newServiceHarness(t)
		_, request := harness.issueAndSign(t)
		other := mustCanonicalAddress(t, "Q"+strings.Repeat("3", 128))
		if err := harness.service.AuthorizeRegeneration(context.Background(), other, request); !errors.Is(err, ErrInvalidAuthorization) {
			t.Fatalf("error = %v", err)
		}
	})

	t.Run("creator changed", func(t *testing.T) {
		harness := newServiceHarness(t)
		_, request := harness.issueAndSign(t)
		_, replacement := testSignerWithSeed(t, 0x44)
		harness.contracts.set(harness.contract, models.ContractInfo{
			ContractAddress:        harness.contract,
			ContractCreatorAddress: replacement,
			CreatorAddressProvenance: models.CreatorAddressProvenanceDirectDeployment,
		})
		if err := harness.service.AuthorizeRegeneration(context.Background(), harness.contract, request); !errors.Is(err, ErrInvalidAuthorization) {
			t.Fatalf("error = %v", err)
		}
	})
}

func TestChallengeRejectsHeuristicOrUnclassifiedCreatorIdentity(t *testing.T) {
	for _, provenance := range []string{
		"",
		models.CreatorAddressProvenanceMintHeuristic,
		models.CreatorAddressProvenanceCreateTraceCaller,
		models.CreatorAddressProvenanceGenesis,
		models.CreatorAddressProvenanceUnclassified,
	} {
		t.Run(provenance, func(t *testing.T) {
			harness := newServiceHarness(t)
			harness.contracts.set(harness.contract, models.ContractInfo{
				ContractAddress:          harness.contract,
				ContractCreatorAddress:   harness.creator,
				CreatorAddressProvenance: provenance,
			})
			if _, err := harness.service.IssueChallenge(context.Background(), harness.contract);
				!errors.Is(err, ErrCreatorUnavailable) {
				t.Fatalf("provenance %q error = %v", provenance, err)
			}
		})
	}
}

func TestChallengeAcceptsAuthoritativeCreatorIdentity(t *testing.T) {
	for _, provenance := range []string{
		models.CreatorAddressProvenanceDirectDeployment,
		models.CreatorAddressProvenanceCreateTraceOuter,
	} {
		t.Run(provenance, func(t *testing.T) {
			harness := newServiceHarness(t)
			harness.contracts.set(harness.contract, models.ContractInfo{
				ContractAddress:          harness.contract,
				ContractCreatorAddress:   harness.creator,
				CreatorAddressProvenance: provenance,
			})
			if _, err := harness.service.IssueChallenge(context.Background(), harness.contract); err != nil {
				t.Fatalf("provenance %q rejected: %v", provenance, err)
			}
		})
	}
}

func TestAuthorizationFailsClosedOnChainMismatch(t *testing.T) {
	harness := newServiceHarness(t)
	_, request := harness.issueAndSign(t)
	*harness.chainID = "0x1"
	err := harness.service.AuthorizeRegeneration(context.Background(), harness.contract, request)
	if !errors.Is(err, ErrUnavailable) {
		t.Fatalf("error = %v, want ErrUnavailable", err)
	}
	if _, found := harness.store.get(request.ChallengeID); !found {
		t.Fatal("chain mismatch consumed the challenge")
	}
}

func TestAuthorizationReportsCurrentCreatorLookupOutageAsUnavailable(t *testing.T) {
	harness := newServiceHarness(t)
	_, request := harness.issueAndSign(t)
	service, err := NewService(harness.service.config, Dependencies{
		Store: harness.store,
		Contracts: ContractReaderFunc(func(context.Context, string) (models.ContractInfo, error) {
			return models.ContractInfo{}, errors.New("mongo unavailable")
		}),
		Chain: ChainIDReaderFunc(func(context.Context) (string, error) {
			return *harness.chainID, nil
		}),
		Now: func() time.Time { return *harness.now },
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := service.AuthorizeRegeneration(context.Background(), harness.contract, request); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("error = %v, want ErrUnavailable", err)
	}
	if _, found := harness.store.get(request.ChallengeID); !found {
		t.Fatal("creator lookup outage consumed the challenge")
	}
}

func TestChallengeCannotCrossConfiguredOriginOrChain(t *testing.T) {
	tests := []struct {
		name    string
		origin  string
		chainID string
	}{
		{name: "origin", origin: "https://www.zondscan.com", chainID: "0x539"},
		{name: "chain", origin: "https://zondscan.com", chainID: "0x1"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			harness := newServiceHarness(t)
			_, request := harness.issueAndSign(t)
			config, err := NewConfig(test.origin, test.chainID)
			if err != nil {
				t.Fatal(err)
			}
			other, err := NewService(config, Dependencies{
				Store:     harness.store,
				Contracts: harness.contracts,
				Chain: ChainIDReaderFunc(func(context.Context) (string, error) {
					return test.chainID, nil
				}),
				Now: func() time.Time { return *harness.now },
			})
			if err != nil {
				t.Fatal(err)
			}
			if err := other.AuthorizeRegeneration(context.Background(), harness.contract, request); !errors.Is(err, ErrInvalidAuthorization) {
				t.Fatalf("cross-binding error = %v", err)
			}
			if err := harness.service.AuthorizeRegeneration(context.Background(), harness.contract, request); err != nil {
				t.Fatalf("original binding after rejection: %v", err)
			}
		})
	}
}

func TestStoredMessageTamperFailsClosedWithoutConsumption(t *testing.T) {
	harness := newServiceHarness(t)
	response, request := harness.issueAndSign(t)
	challenge, found := harness.store.get(response.ChallengeID)
	if !found {
		t.Fatal("challenge missing")
	}
	challenge.Message = append(challenge.Message, '\n')
	harness.store.set(challenge)
	if err := harness.service.AuthorizeRegeneration(context.Background(), harness.contract, request); !errors.Is(err, ErrInvalidAuthorization) {
		t.Fatalf("error = %v", err)
	}
	if _, found := harness.store.get(response.ChallengeID); !found {
		t.Fatal("tampered stored challenge was consumed")
	}
}

func testSigner(t *testing.T) (*cryptomldsa.MLDSA87, string) {
	t.Helper()
	return testSignerWithSeed(t, 0x22)
}

func testSignerWithSeed(t *testing.T, seedByte byte) (*cryptomldsa.MLDSA87, string) {
	t.Helper()
	var seed [cryptomldsa.SEED_BYTES]byte
	for i := range seed {
		seed[i] = seedByte
	}
	key, err := cryptomldsa.NewMLDSA87FromSeed(seed)
	if err != nil {
		t.Fatal(err)
	}
	publicKey := key.GetPK()
	desc, err := descriptor.FromBytes([]byte{0x01, 0x00, 0x00})
	if err != nil {
		t.Fatal(err)
	}
	address, err := qrllibwallet.GetAddressFromPKAndDescriptor(publicKey[:], desc)
	if err != nil {
		t.Fatal(err)
	}
	return key, common.ToChecksumAddress(address)
}

func mustCanonicalAddress(t *testing.T, value string) string {
	t.Helper()
	canonical, ok := qrladdress.Canonicalize(value)
	if !ok {
		t.Fatalf("invalid test address %q", value)
	}
	return canonical
}
