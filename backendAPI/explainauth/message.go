package explainauth

import (
	"fmt"
	"strings"
	"time"

	"backendAPI/models"
)

func challengeRoute(contract string) string {
	return "/contract/explain/" + contract + "?regenerate=1"
}

func canonicalMessage(challenge models.ContractExplainChallenge) []byte {
	fields := []string{
		challenge.Version,
		"action=" + challenge.Action,
		"origin=" + challenge.Origin,
		"chainId=" + challenge.ChainID,
		"method=" + challenge.Method,
		"route=" + challenge.Route,
		"contract=" + challenge.ContractAddress,
		"signer=" + challenge.SignerAddress,
		"nonce=" + challenge.ID,
		"issuedAt=" + challenge.IssuedAt.UTC().Format(time.RFC3339),
		"expiresAt=" + challenge.ExpiresAt.UTC().Format(time.RFC3339),
	}
	return []byte(strings.Join(fields, "\n"))
}

func validateStoredChallenge(challenge models.ContractExplainChallenge, config Config) error {
	if challenge.Version != ChallengeVersion ||
		challenge.Action != RegenerateAction ||
		challenge.Origin != config.origin ||
		challenge.ChainID != config.expectedChainID ||
		challenge.Method != challengeMethod ||
		challenge.Route != challengeRoute(challenge.ContractAddress) ||
		challenge.ID == "" || challenge.ContractAddress == "" || challenge.SignerAddress == "" ||
		challenge.IssuedAt.IsZero() || !challenge.ExpiresAt.Equal(challenge.IssuedAt.Add(ChallengeTTL)) {
		return fmt.Errorf("stored challenge binding mismatch")
	}
	if string(challenge.Message) != string(canonicalMessage(challenge)) {
		return fmt.Errorf("stored challenge message mismatch")
	}
	return nil
}
