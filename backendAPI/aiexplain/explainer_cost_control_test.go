package aiexplain

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"backendAPI/db"
	"backendAPI/models"
	"backendAPI/sourcebundle"
)

type fakeGenerationStore struct {
	mu                   sync.Mutex
	contract             models.ContractInfo
	lease                *models.AIExplanationLease
	nextToken            int
	providerReservations int
	regenReservations    int
	saves                int
	cooldowns            int
	releases             int
	budgetErr            error
	saveErr              error
	lookupErr            error
	firstLookupOverride  *models.ContractInfo
	lookupCalls          int
}

func (f *fakeGenerationStore) ReturnContractCode(string) (models.ContractInfo, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.lookupErr != nil {
		return models.ContractInfo{}, f.lookupErr
	}
	f.lookupCalls++
	if f.lookupCalls == 1 && f.firstLookupOverride != nil {
		return *f.firstLookupOverride, nil
	}
	return f.contract, nil
}

func (f *fakeGenerationStore) AcquireLease(
	_ context.Context,
	_ string,
	_ models.ContractInfo,
	sourceDigest string,
	ttl time.Duration,
	requireCacheMiss bool,
) (models.AIExplanationLease, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if requireCacheMiss && cachedExplanationMatches(f.contract, sourceDigest) {
		return models.AIExplanationLease{}, db.ErrAIExplanationCacheFilled
	}
	if f.lease != nil {
		return models.AIExplanationLease{}, &db.AIExplanationLeaseBusyError{
			RetryAfterDuration: time.Minute,
		}
	}
	f.nextToken++
	lease := models.AIExplanationLease{
		Token:        string(rune('a' + f.nextToken)),
		SourceDigest: sourceDigest,
		VerifiedAt:   f.contract.VerifiedAt,
		AcquiredAt:   time.Now().UTC(),
		ExpiresAt:    time.Now().UTC().Add(ttl),
	}
	f.lease = &lease
	return lease, nil
}

func (f *fakeGenerationStore) ReserveRegenSlot(string, int, time.Duration) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.regenReservations++
	return nil
}

func (f *fakeGenerationStore) ReserveProviderCall(context.Context, int, time.Time) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.budgetErr != nil {
		return f.budgetErr
	}
	f.providerReservations++
	return nil
}

func (f *fakeGenerationStore) SaveExplanation(
	_ context.Context,
	_ string,
	explanation,
	model,
	generatedAt,
	sourceDigest string,
	lease models.AIExplanationLease,
	_ models.ContractInfo,
) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.lease == nil || f.lease.Token != lease.Token {
		return db.ErrSourceBundleChanged
	}
	if f.saveErr != nil {
		return f.saveErr
	}
	f.saves++
	f.contract.AIExplanation = explanation
	f.contract.AIExplanationModel = model
	f.contract.AIExplanationAt = generatedAt
	f.contract.AIExplanationSourceDigest = sourceDigest
	f.lease = nil
	return nil
}

func (f *fakeGenerationStore) CooldownLease(
	_ context.Context,
	_ string,
	lease models.AIExplanationLease,
	_ time.Duration,
) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.lease != nil && f.lease.Token == lease.Token {
		f.cooldowns++
	}
	return nil
}

func (f *fakeGenerationStore) ReleaseLease(
	_ context.Context,
	_ string,
	lease models.AIExplanationLease,
) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.lease != nil && f.lease.Token == lease.Token {
		f.lease = nil
		f.releases++
	}
	return nil
}

type blockingGenerator struct {
	mu      sync.Mutex
	calls   int
	started chan struct{}
	release chan struct{}
	err     error
	once    sync.Once
}

func (g *blockingGenerator) Generate(
	context.Context,
	string,
	string,
) (string, string, error) {
	g.mu.Lock()
	g.calls++
	g.mu.Unlock()
	g.once.Do(func() { close(g.started) })
	if g.release != nil {
		<-g.release
	}
	if g.err != nil {
		return "", "", g.err
	}
	return "explanation", "test-model", nil
}

func (g *blockingGenerator) callCount() int {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.calls
}

func TestExplanationLeaseSingleFlightsAllPaidGeneration(t *testing.T) {
	for _, regenerate := range []bool{false, true} {
		t.Run(map[bool]string{false: "initial", true: "regeneration"}[regenerate], func(t *testing.T) {
			store := &fakeGenerationStore{contract: testDigestBackedContract()}
			generator := &blockingGenerator{
				started: make(chan struct{}),
				release: make(chan struct{}),
			}
			explainer := &Explainer{
				Client:                 generator,
				Store:                  store,
				DailyProviderCallLimit: 10,
			}

			primaryResult := make(chan error, 1)
			go func() {
				_, err := explainer.Explain(context.Background(), store.contract.ContractAddress, regenerate)
				primaryResult <- err
			}()
			<-generator.started

			const concurrent = 12
			errorsByCaller := make(chan error, concurrent)
			var callers sync.WaitGroup
			for i := 0; i < concurrent; i++ {
				callers.Add(1)
				go func() {
					defer callers.Done()
					_, err := explainer.Explain(
						context.Background(),
						store.contract.ContractAddress,
						regenerate,
					)
					errorsByCaller <- err
				}()
			}
			callers.Wait()
			close(errorsByCaller)
			for err := range errorsByCaller {
				if !errors.Is(err, ErrGenerationInProgress) {
					t.Fatalf("concurrent error = %v, want ErrGenerationInProgress", err)
				}
			}
			close(generator.release)
			if err := <-primaryResult; err != nil {
				t.Fatalf("primary Explain: %v", err)
			}

			store.mu.Lock()
			providerReservations := store.providerReservations
			regenReservations := store.regenReservations
			saves := store.saves
			store.mu.Unlock()
			if generator.callCount() != 1 || providerReservations != 1 || saves != 1 {
				t.Fatalf(
					"provider calls = %d, budget reservations = %d, saves = %d",
					generator.callCount(),
					providerReservations,
					saves,
				)
			}
			wantRegenReservations := 0
			if regenerate {
				wantRegenReservations = 1
			}
			if regenReservations != wantRegenReservations {
				t.Fatalf("regen reservations = %d, want %d", regenReservations, wantRegenReservations)
			}
		})
	}
}

func TestProviderAndCacheFailuresRetainCooldownLease(t *testing.T) {
	tests := []struct {
		name         string
		generatorErr error
		saveErr      error
		wantKind     error
	}{
		{name: "provider", generatorErr: errors.New("provider unavailable"), wantKind: ErrProviderCooldown},
		{name: "cache", saveErr: errors.New("mongo unavailable"), wantKind: ErrCacheCooldown},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			store := &fakeGenerationStore{contract: testDigestBackedContract(), saveErr: test.saveErr}
			generator := &blockingGenerator{
				started: make(chan struct{}),
				err:     test.generatorErr,
			}
			explainer := &Explainer{
				Client:                 generator,
				Store:                  store,
				DailyProviderCallLimit: 10,
			}
			_, err := explainer.Explain(context.Background(), store.contract.ContractAddress, false)
			if !errors.Is(err, test.wantKind) {
				t.Fatalf("first error = %v, want %v", err, test.wantKind)
			}
			if delay, ok := RetryAfter(err); !ok || delay != FailureCooldown {
				t.Fatalf("retry = %s, %v", delay, ok)
			}
			_, err = explainer.Explain(context.Background(), store.contract.ContractAddress, false)
			if !errors.Is(err, ErrGenerationInProgress) {
				t.Fatalf("second error = %v, want active cooldown lease", err)
			}
			store.mu.Lock()
			cooldowns := store.cooldowns
			store.mu.Unlock()
			if generator.callCount() != 1 || cooldowns != 1 {
				t.Fatalf("provider calls = %d, cooldowns = %d", generator.callCount(), cooldowns)
			}
		})
	}
}

func TestDailyBudgetRejectionReleasesUnspentLease(t *testing.T) {
	store := &fakeGenerationStore{
		contract:  testDigestBackedContract(),
		budgetErr: &db.AIProviderDailyBudgetError{RetryAfterDuration: 3 * time.Hour},
	}
	generator := &blockingGenerator{started: make(chan struct{})}
	explainer := &Explainer{
		Client:                 generator,
		Store:                  store,
		DailyProviderCallLimit: 10,
	}
	_, err := explainer.Explain(context.Background(), store.contract.ContractAddress, false)
	if !errors.Is(err, ErrDailyBudget) {
		t.Fatalf("error = %v, want ErrDailyBudget", err)
	}
	if delay, ok := RetryAfter(err); !ok || delay != 3*time.Hour {
		t.Fatalf("retry = %s, %v", delay, ok)
	}
	store.mu.Lock()
	releases := store.releases
	lease := store.lease
	store.mu.Unlock()
	if releases != 1 || lease != nil || generator.callCount() != 0 {
		t.Fatalf("releases = %d, lease = %#v, provider calls = %d", releases, lease, generator.callCount())
	}
}

func TestBudgetStorageFailureRetainsCooldownLease(t *testing.T) {
	store := &fakeGenerationStore{
		contract:  testDigestBackedContract(),
		budgetErr: errors.New("mongo unavailable"),
	}
	generator := &blockingGenerator{started: make(chan struct{})}
	explainer := &Explainer{
		Client:                 generator,
		Store:                  store,
		DailyProviderCallLimit: 10,
	}
	_, err := explainer.Explain(context.Background(), store.contract.ContractAddress, false)
	if !errors.Is(err, ErrBudgetCooldown) {
		t.Fatalf("error = %v, want ErrBudgetCooldown", err)
	}
	store.mu.Lock()
	cooldowns := store.cooldowns
	lease := store.lease
	store.mu.Unlock()
	if cooldowns != 1 || lease == nil || generator.callCount() != 0 {
		t.Fatalf("cooldowns = %d, lease = %#v, provider calls = %d", cooldowns, lease, generator.callCount())
	}
}

func TestInitialLookupStorageFailureIsRetryableServiceError(t *testing.T) {
	store := &fakeGenerationStore{lookupErr: errors.New("mongo unavailable")}
	explainer := &Explainer{
		Client: &blockingGenerator{started: make(chan struct{})},
		Store:  store,
	}
	_, err := explainer.Explain(context.Background(), "Qcontract", false)
	if !errors.Is(err, ErrStorageUnavailable) {
		t.Fatalf("error = %v, want ErrStorageUnavailable", err)
	}
	if delay, ok := RetryAfter(err); !ok || delay != FailureCooldown {
		t.Fatalf("retry = %s, %v", delay, ok)
	}
}

func TestConcurrentCacheFillPreventsLateDuplicateProviderCall(t *testing.T) {
	current := testDigestBackedContract()
	current.AIExplanation = "cached explanation"
	current.AIExplanationAt = "2026-08-27T01:03:00Z"
	current.AIExplanationModel = "test-model"
	current.AIExplanationSourceDigest = current.SourceBundleDigest
	stale := current
	stale.AIExplanation = ""
	stale.AIExplanationAt = ""
	stale.AIExplanationModel = ""
	stale.AIExplanationSourceDigest = ""
	store := &fakeGenerationStore{
		contract:            current,
		firstLookupOverride: &stale,
	}
	generator := &blockingGenerator{started: make(chan struct{})}
	explainer := &Explainer{
		Client:                 generator,
		Store:                  store,
		DailyProviderCallLimit: 10,
	}
	response, err := explainer.Explain(context.Background(), current.ContractAddress, false)
	if err != nil {
		t.Fatalf("Explain: %v", err)
	}
	if !response.Cached || response.Explanation != current.AIExplanation {
		t.Fatalf("response = %#v", response)
	}
	if generator.callCount() != 0 {
		t.Fatalf("provider calls = %d, want 0", generator.callCount())
	}
}

func testDigestBackedContract() models.ContractInfo {
	const buildID = "hypc-q128-test"
	source := "contract Primary {}"
	digest := sourcebundle.Digest("Primary", source, nil)
	hypcSHA256 := strings.Repeat("a", 64)
	nsjailSHA256 := strings.Repeat("b", 64)
	policySHA256 := strings.Repeat("c", 64)
	return models.ContractInfo{
		ContractAddress:          "Q" + strings.Repeat("1", 128),
		Verified:                 true,
		VerificationRecordSchema: models.VerificationRecordSchemaV1,
		VerifiedAt:               "2026-08-27T01:02:03.000000004Z",
		SourceCode:               source,
		ContractName:             "Primary",
		CompilerVersion:          buildID,
		SourceBundleDigest:       digest,
		CompilerProvenance: &models.CompilerProvenance{
			Schema:  models.CompilerProvenanceSchemaV2,
			Kind:    "native",
			BuildID: buildID,
			ExecutionDigest: models.NativeSandboxCompilerExecutionDigestV2(
				buildID,
				hypcSHA256,
				nsjailSHA256,
				policySHA256,
			),
			Components: []models.CompilerProvenanceComponent{
				{Name: "hypc", SHA256: hypcSHA256},
				{Name: "nsjail", SHA256: nsjailSHA256},
				{Name: "policy", SHA256: policySHA256},
			},
		},
	}
}
