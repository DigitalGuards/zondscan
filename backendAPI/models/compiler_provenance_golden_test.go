package models

import (
	"encoding/json"
	"os"
	"testing"
)

func TestCompilerProvenanceV2GoldenFixture(t *testing.T) {
	raw, err := os.ReadFile("testdata/compiler_provenance_v2.json")
	if err != nil {
		t.Fatalf("read compiler provenance fixture: %v", err)
	}
	var fixture struct {
		CompilerVersion string             `json:"compilerVersion"`
		Provenance      CompilerProvenance `json:"provenance"`
	}
	if err := json.Unmarshal(raw, &fixture); err != nil {
		t.Fatalf("decode compiler provenance fixture: %v", err)
	}
	if got := ClassifyCompilerProvenance(&fixture.Provenance, fixture.CompilerVersion); got != CompilerProvenanceDigestBacked {
		t.Fatalf("fixture classification = %q, want %q", got, CompilerProvenanceDigestBacked)
	}
	want := NativeSandboxCompilerExecutionDigestV2(
		fixture.Provenance.BuildID,
		fixture.Provenance.Components[0].SHA256,
		fixture.Provenance.Components[1].SHA256,
		fixture.Provenance.Components[2].SHA256,
	)
	if fixture.Provenance.ExecutionDigest != want {
		t.Fatalf("fixture execution digest = %q, want %q", fixture.Provenance.ExecutionDigest, want)
	}
}
