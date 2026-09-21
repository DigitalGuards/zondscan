package routes

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"backendAPI/aiexplain"
	"backendAPI/explainauth"

	"github.com/gin-gonic/gin"
)

type fakeContractExplainer struct {
	mu         sync.Mutex
	calls      int
	regenerate []bool
	err        error
}

func (f *fakeContractExplainer) Explain(
	_ context.Context,
	address string,
	regenerate bool,
) (*aiexplain.ExplainResponse, error) {
	f.mu.Lock()
	f.calls++
	f.regenerate = append(f.regenerate, regenerate)
	f.mu.Unlock()
	if f.err != nil {
		return nil, f.err
	}
	return &aiexplain.ExplainResponse{Address: address, Explanation: "ok"}, nil
}

type fakeRetryableExplainError struct {
	kind  error
	delay time.Duration
}

func (e fakeRetryableExplainError) Error() string             { return e.kind.Error() }
func (e fakeRetryableExplainError) Unwrap() error             { return e.kind }
func (e fakeRetryableExplainError) RetryAfter() time.Duration { return e.delay }

func (f *fakeContractExplainer) callCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.calls
}

type fakeRegenerationAuthorizer struct {
	mu             sync.Mutex
	issueResponse  *explainauth.ChallengeResponse
	issueErr       error
	authorizeErr   error
	authorizeCalls int
	singleUse      bool
	used           bool
}

func (f *fakeRegenerationAuthorizer) IssueChallenge(
	context.Context,
	string,
) (*explainauth.ChallengeResponse, error) {
	return f.issueResponse, f.issueErr
}

func (f *fakeRegenerationAuthorizer) AuthorizeRegeneration(
	context.Context,
	string,
	explainauth.AuthorizationRequest,
) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.authorizeCalls++
	if f.authorizeErr != nil {
		return f.authorizeErr
	}
	if f.singleUse && f.used {
		return explainauth.ErrInvalidAuthorization
	}
	f.used = true
	return nil
}

func TestKnownExplainErrorResponseFailsClosedOnVerificationProvenance(t *testing.T) {
	status, body, ok := knownExplainErrorResponse(fmt.Errorf("wrapped: %w", aiexplain.ErrProvenance))
	if !ok || status != http.StatusConflict {
		t.Fatalf("mapping = status %d, body %#v, ok %v", status, body, ok)
	}
	message, _ := body["error"].(string)
	if !strings.Contains(message, "digest-backed compiler and source-bundle provenance") {
		t.Fatalf("error message = %q", message)
	}
}

func TestKnownExplainErrorResponseRejectsConcurrentSourceChange(t *testing.T) {
	status, body, ok := knownExplainErrorResponse(aiexplain.ErrSourceChanged)
	if !ok || status != http.StatusConflict {
		t.Fatalf("mapping = status %d, body %#v, ok %v", status, body, ok)
	}
	message, _ := body["error"].(string)
	if !strings.Contains(message, "source changed") {
		t.Fatalf("error message = %q", message)
	}
}

func TestExplainCostControlErrorsHaveTypedStatusAndRetryAfter(t *testing.T) {
	tests := []struct {
		name   string
		err    error
		status int
	}{
		{name: "single flight", err: aiexplain.ErrGenerationInProgress, status: http.StatusConflict},
		{name: "daily budget", err: aiexplain.ErrDailyBudget, status: http.StatusTooManyRequests},
		{name: "budget unavailable", err: aiexplain.ErrBudgetCooldown, status: http.StatusServiceUnavailable},
		{name: "provider cooldown", err: aiexplain.ErrProviderCooldown, status: http.StatusBadGateway},
		{name: "cache cooldown", err: aiexplain.ErrCacheCooldown, status: http.StatusServiceUnavailable},
		{name: "storage unavailable", err: aiexplain.ErrStorageUnavailable, status: http.StatusServiceUnavailable},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			gin.SetMode(gin.TestMode)
			router := gin.New()
			explainer := &fakeContractExplainer{err: fakeRetryableExplainError{
				kind:  test.err,
				delay: 1500 * time.Millisecond,
			}}
			registerContractExplainRoutes(router, explainer, nil)
			response := performExplainRequest(
				router,
				"/contract/explain/"+testExplainAddress(),
				nil,
			)
			if response.Code != test.status {
				t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
			}
			if got := response.Header().Get("Retry-After"); got != "2" {
				t.Fatalf("Retry-After = %q, want 2", got)
			}
		})
	}
}

func TestInitialExplanationRemainsPublicWithoutAuthorizationService(t *testing.T) {
	router, explainer := testExplainRouter(nil)
	response := performExplainRequest(router, "/contract/explain/"+testExplainAddress(), nil)
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	if explainer.callCount() != 1 {
		t.Fatalf("explainer calls = %d", explainer.callCount())
	}
}

func TestInitialExplanationRequiresEmptyBody(t *testing.T) {
	router, explainer := testExplainRouter(nil)
	for name, body := range map[string][]byte{
		"json object": []byte(`{}`),
		"whitespace":  []byte(" \n"),
	} {
		t.Run(name, func(t *testing.T) {
			response := performExplainRequest(
				router,
				"/contract/explain/"+testExplainAddress(),
				body,
			)
			if response.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
			}
		})
	}
	if explainer.callCount() != 0 {
		t.Fatalf("explainer calls = %d", explainer.callCount())
	}
}

func TestRegenerationFailsClosedWithoutAuthorizationService(t *testing.T) {
	router, explainer := testExplainRouter(nil)
	response := performExplainRequest(
		router,
		"/contract/explain/"+testExplainAddress()+"?regenerate=1",
		validExplainAuthorizationBody(t),
	)
	if response.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	if explainer.callCount() != 0 {
		t.Fatalf("explainer calls = %d", explainer.callCount())
	}
}

func TestRegenerationAuthenticatesBeforeCallingExplainer(t *testing.T) {
	authorizer := &fakeRegenerationAuthorizer{}
	router, explainer := testExplainRouter(authorizer)
	response := performExplainRequest(
		router,
		"/contract/explain/"+testExplainAddress()+"?regenerate=1",
		validExplainAuthorizationBody(t),
	)
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	if authorizer.authorizeCalls != 1 || explainer.callCount() != 1 {
		t.Fatalf("authorize calls = %d, explain calls = %d", authorizer.authorizeCalls, explainer.callCount())
	}
	if len(explainer.regenerate) != 1 || !explainer.regenerate[0] {
		t.Fatalf("regenerate arguments = %#v", explainer.regenerate)
	}
}

func TestRegenerationRejectsInvalidProofBeforeCallingExplainer(t *testing.T) {
	authorizer := &fakeRegenerationAuthorizer{authorizeErr: explainauth.ErrInvalidAuthorization}
	router, explainer := testExplainRouter(authorizer)
	response := performExplainRequest(
		router,
		"/contract/explain/"+testExplainAddress()+"?regenerate=1",
		validExplainAuthorizationBody(t),
	)
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	if explainer.callCount() != 0 {
		t.Fatalf("explainer calls = %d", explainer.callCount())
	}
}

func TestRegenerationRejectsOversizedAndAmbiguousBodies(t *testing.T) {
	authorizer := &fakeRegenerationAuthorizer{}
	router, explainer := testExplainRouter(authorizer)
	path := "/contract/explain/" + testExplainAddress() + "?regenerate=1"

	oversized := bytes.Repeat([]byte("x"), int(maxExplainAuthorizationBodyBytes)+1)
	response := performExplainRequest(router, path, oversized)
	if response.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("oversized status = %d", response.Code)
	}

	valid := string(validExplainAuthorizationBody(t))
	duplicate := strings.Replace(valid, `{"challengeId":`, `{"challengeId":"`+strings.Repeat("a", 64)+`","challengeId":`, 1)
	response = performExplainRequest(router, path, []byte(duplicate))
	if response.Code != http.StatusBadRequest {
		t.Fatalf("duplicate status = %d, body = %s", response.Code, response.Body.String())
	}
	if authorizer.authorizeCalls != 0 || explainer.callCount() != 0 {
		t.Fatalf("authorize calls = %d, explain calls = %d", authorizer.authorizeCalls, explainer.callCount())
	}
}

func TestRegenerationRequiresExactCanonicalQuery(t *testing.T) {
	for _, query := range []string{
		"regenerate=true",
		"regenerate=1&regenerate=1",
		"regenerate=1&extra=1",
		"extra=1&regenerate=1",
	} {
		t.Run(query, func(t *testing.T) {
			authorizer := &fakeRegenerationAuthorizer{}
			router, explainer := testExplainRouter(authorizer)
			response := performExplainRequest(
				router,
				"/contract/explain/"+testExplainAddress()+"?"+query,
				validExplainAuthorizationBody(t),
			)
			if response.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
			}
			if authorizer.authorizeCalls != 0 || explainer.callCount() != 0 {
				t.Fatalf("authorize calls = %d, explain calls = %d", authorizer.authorizeCalls, explainer.callCount())
			}
		})
	}
}

func TestConcurrentRouteReplayCallsExplainerOnce(t *testing.T) {
	authorizer := &fakeRegenerationAuthorizer{singleUse: true}
	router, explainer := testExplainRouter(authorizer)
	path := "/contract/explain/" + testExplainAddress() + "?regenerate=1"
	body := validExplainAuthorizationBody(t)
	start := make(chan struct{})
	statuses := make(chan int, 2)
	for i := 0; i < 2; i++ {
		go func() {
			<-start
			statuses <- performExplainRequest(router, path, body).Code
		}()
	}
	close(start)
	got := []int{<-statuses, <-statuses}
	if !((got[0] == http.StatusOK && got[1] == http.StatusUnauthorized) ||
		(got[1] == http.StatusOK && got[0] == http.StatusUnauthorized)) {
		t.Fatalf("statuses = %#v", got)
	}
	if explainer.callCount() != 1 {
		t.Fatalf("explainer calls = %d", explainer.callCount())
	}
}

func TestChallengeRouteReturnsExpectedWireShape(t *testing.T) {
	want := &explainauth.ChallengeResponse{
		ChallengeID: strings.Repeat("a", 64),
		MessageHex:  "0x0102",
		Signer:      testExplainAddress(),
		Contract:    testExplainAddress(),
		Origin:      "https://zondscan.com",
		ChainID:     "0x539",
		ExpiresAt:   "2026-08-27T20:05:00Z",
	}
	authorizer := &fakeRegenerationAuthorizer{issueResponse: want}
	router, _ := testExplainRouter(authorizer)
	response := performExplainRequest(
		router,
		"/contract/explain/"+testExplainAddress()+"/challenge",
		nil,
	)
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	var got explainauth.ChallengeResponse
	if err := json.Unmarshal(response.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got != *want {
		t.Fatalf("response = %#v, want %#v", got, *want)
	}
	if response.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("Cache-Control = %q", response.Header().Get("Cache-Control"))
	}
}

func TestChallengeRouteRejectsQueryParameters(t *testing.T) {
	authorizer := &fakeRegenerationAuthorizer{}
	router, explainer := testExplainRouter(authorizer)
	response := performExplainRequest(
		router,
		"/contract/explain/"+testExplainAddress()+"/challenge?extra=1",
		nil,
	)
	if response.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	if explainer.callCount() != 0 {
		t.Fatalf("explainer calls = %d", explainer.callCount())
	}
}

func TestChallengeRouteRequiresEmptyBody(t *testing.T) {
	authorizer := &fakeRegenerationAuthorizer{}
	router, explainer := testExplainRouter(authorizer)
	response := performExplainRequest(
		router,
		"/contract/explain/"+testExplainAddress()+"/challenge",
		[]byte(" \n"),
	)
	if response.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	if explainer.callCount() != 0 {
		t.Fatalf("explainer calls = %d", explainer.callCount())
	}
}

func TestAuthorizationUnavailableMapsToServiceUnavailable(t *testing.T) {
	authorizer := &fakeRegenerationAuthorizer{authorizeErr: fmt.Errorf("wrapped: %w", explainauth.ErrUnavailable)}
	router, explainer := testExplainRouter(authorizer)
	response := performExplainRequest(
		router,
		"/contract/explain/"+testExplainAddress()+"?regenerate=1",
		validExplainAuthorizationBody(t),
	)
	if response.Code != http.StatusServiceUnavailable || explainer.callCount() != 0 {
		t.Fatalf("status = %d, explain calls = %d", response.Code, explainer.callCount())
	}
}

func testExplainRouter(authorizer regenerationAuthorizer) (*gin.Engine, *fakeContractExplainer) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	explainer := &fakeContractExplainer{}
	registerContractExplainRoutes(router, explainer, authorizer)
	return router, explainer
}

func performExplainRequest(router *gin.Engine, path string, body []byte) *httptest.ResponseRecorder {
	request := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(body))
	if body != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	return response
}

func validExplainAuthorizationBody(t *testing.T) []byte {
	t.Helper()
	request := explainauth.AuthorizationRequest{
		ChallengeID: strings.Repeat("a", 64),
		Proof: explainauth.SignedMessageProof{
			Signature:     "0x" + strings.Repeat("11", 4627),
			PublicKey:     "0x" + strings.Repeat("22", 2592),
			Descriptor:    "0x010000",
			Signer:        testExplainAddress(),
			Digest:        "0x" + strings.Repeat("33", 64),
			SchemeVersion: explainauth.SchemeVersion,
		},
	}
	body, err := json.Marshal(request)
	if err != nil {
		t.Fatal(err)
	}
	return body
}

func testExplainAddress() string {
	return "Q" + strings.Repeat("1", 128)
}
