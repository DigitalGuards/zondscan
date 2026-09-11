package explainauth

import (
	"strings"
	"testing"
	"time"

	"backendAPI/models"
)

func TestCanonicalMessageGolden(t *testing.T) {
	contract := "Q" + strings.Repeat("1", 128)
	signer := "Q" + strings.Repeat("2", 128)
	challenge := models.ContractExplainChallenge{
		ID:              strings.Repeat("ab", nonceBytes),
		Version:         ChallengeVersion,
		Action:          RegenerateAction,
		Origin:          "https://zondscan.com",
		ChainID:         "0x539",
		Method:          challengeMethod,
		Route:           challengeRoute(contract),
		ContractAddress: contract,
		SignerAddress:   signer,
		IssuedAt:        time.Date(2026, 8, 27, 20, 0, 0, 0, time.UTC),
		ExpiresAt:       time.Date(2026, 8, 27, 20, 5, 0, 0, time.UTC),
	}
	want := strings.Join([]string{
		"ZONDSCAN-AUTH-v1",
		"action=contract.explain.regenerate",
		"origin=https://zondscan.com",
		"chainId=0x539",
		"method=POST",
		"route=/contract/explain/" + contract + "?regenerate=1",
		"contract=" + contract,
		"signer=" + signer,
		"nonce=" + challenge.ID,
		"issuedAt=2026-08-27T20:00:00Z",
		"expiresAt=2026-08-27T20:05:00Z",
	}, "\n")
	got := string(canonicalMessage(challenge))
	if got != want {
		t.Fatalf("canonical message:\n%s\nwant:\n%s", got, want)
	}
	if strings.HasSuffix(got, "\n") {
		t.Fatal("canonical message has a trailing newline")
	}
}

func TestValidateStoredChallengeRejectsCrossRouteBinding(t *testing.T) {
	contract := "Q" + strings.Repeat("1", 128)
	challenge := models.ContractExplainChallenge{
		ID:              strings.Repeat("ab", nonceBytes),
		Version:         ChallengeVersion,
		Action:          RegenerateAction,
		Origin:          "https://zondscan.com",
		ChainID:         "0x539",
		Method:          challengeMethod,
		Route:           "/contract/verify",
		ContractAddress: contract,
		SignerAddress:   "Q" + strings.Repeat("2", 128),
		IssuedAt:        time.Date(2026, 8, 27, 20, 0, 0, 0, time.UTC),
		ExpiresAt:       time.Date(2026, 8, 27, 20, 5, 0, 0, time.UTC),
	}
	challenge.Message = canonicalMessage(challenge)
	config, err := NewConfig("https://zondscan.com", "0x539")
	if err != nil {
		t.Fatal(err)
	}
	if err := validateStoredChallenge(challenge, config); err == nil {
		t.Fatal("cross-route challenge validated")
	}
}
