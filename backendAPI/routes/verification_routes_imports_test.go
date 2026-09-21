package routes

import (
	"reflect"
	"testing"

	"backendAPI/models"
	"backendAPI/verification"
)

func TestVerificationJobPayloadPersistsCanonicalEffectiveInputs(t *testing.T) {
	req, err := verification.CanonicalizeVerifyRequest(verification.VerifyRequest{
		SourceCode:       "contract Primary {}",
		ContractName:     "Primary",
		OptimizerEnabled: true,
		OptimizerRuns:    0,
		Imports: map[string]string{
			"./lib/Owned.hyp": "contract Owned {}",
		},
	})
	if err != nil {
		t.Fatalf("canonicalize request: %v", err)
	}
	comp := &verification.Compiler{
		BuildID: "current-q128",
		Provenance: models.CompilerProvenance{
			Schema:  "qrl.contract-compiler-provenance.v1",
			Kind:    verification.CompilerKindNative,
			BuildID: "current-q128",
		},
	}

	payload := verificationJobPayload(req, comp)
	wantImports := map[string]string{"lib/Owned.hyp": "contract Owned {}"}
	if !reflect.DeepEqual(payload.Imports, wantImports) {
		t.Fatalf("payload imports = %#v, want %#v", payload.Imports, wantImports)
	}
	if payload.OptimizationRuns != 200 {
		t.Fatalf("payload optimizerRuns = %d, want 200", payload.OptimizationRuns)
	}
}
