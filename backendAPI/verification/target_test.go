package verification

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strings"
	"testing"

	"backendAPI/models"
)

func validVerificationTarget() models.VerificationTarget {
	return models.VerificationTarget{
		Address:             "Q" + strings.Repeat("a", 128),
		CreationTransaction: "0x" + strings.Repeat("b", 64),
		CreationBlockNumber: "0x10",
		CreationBlockHash:   "0x" + strings.Repeat("c", 64),
		ChainID:             "0x539",
		DeployedCodeSHA256:  strings.Repeat("d", 64),
	}
}

func TestRuntimeCodeSHA256CanonicalizesHexRepresentation(t *testing.T) {
	wantBytes := sha256.Sum256([]byte{0xab, 0xcd})
	want := hex.EncodeToString(wantBytes[:])
	for _, code := range []string{"0xabcd", "0xABCD", "ABCD"} {
		got, err := RuntimeCodeSHA256(code)
		if err != nil || got != want {
			t.Fatalf("RuntimeCodeSHA256(%q) = %q, %v; want %q", code, got, err, want)
		}
	}
	for _, code := range []string{"", "0x", "0x0", "0xzz"} {
		if _, err := RuntimeCodeSHA256(code); err == nil {
			t.Fatalf("RuntimeCodeSHA256(%q) unexpectedly succeeded", code)
		}
	}
}

func TestValidateVerificationTargetRequiresCanonicalIdentity(t *testing.T) {
	if err := validateVerificationTarget(validVerificationTarget()); err != nil {
		t.Fatalf("valid target rejected: %v", err)
	}

	for name, mutate := range map[string]func(*models.VerificationTarget){
		"short address":           func(target *models.VerificationTarget) { target.Address = "Qabc" },
		"uppercase address":       func(target *models.VerificationTarget) { target.Address = "Q" + strings.Repeat("A", 128) },
		"noncanonical block":      func(target *models.VerificationTarget) { target.CreationBlockNumber = "0x010" },
		"block hash lacks prefix": func(target *models.VerificationTarget) { target.CreationBlockHash = strings.Repeat("c", 64) },
		"uppercase transaction":   func(target *models.VerificationTarget) { target.CreationTransaction = "0x" + strings.Repeat("B", 64) },
		"malformed code digest":   func(target *models.VerificationTarget) { target.DeployedCodeSHA256 = strings.Repeat("z", 64) },
	} {
		t.Run(name, func(t *testing.T) {
			target := validVerificationTarget()
			mutate(&target)
			if err := validateVerificationTarget(target); !errors.Is(err, ErrVerificationTargetIncomplete) {
				t.Fatalf("validation error = %v, want incomplete target", err)
			}
		})
	}
}

func TestValidateVerificationTargetAllowsGenesisWithoutTransaction(t *testing.T) {
	target := validVerificationTarget()
	target.GenesisContract = true
	target.CreationTransaction = ""
	target.CreationBlockNumber = "0x0"
	if err := validateVerificationTarget(target); err != nil {
		t.Fatalf("genesis target rejected: %v", err)
	}
}
