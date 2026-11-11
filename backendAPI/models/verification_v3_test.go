package models

import (
	"strings"
	"testing"
)

func v3TestTarget() VerificationTarget {
	return VerificationTarget{
		Address:             "Q" + strings.Repeat("a", 128),
		CreationTransaction: "0x" + strings.Repeat("b", 64),
		CreationBlockNumber: "0x10",
		CreationBlockHash:   "0x" + strings.Repeat("c", 64),
		ChainID:             "0x539",
		DeployedCodeSHA256:  strings.Repeat("d", 64),
	}
}

func v3TestResult() VerificationResult {
	return VerificationResult{
		Abi:                 `[{"type":"function","name":"value"}]`,
		ContractName:        "Main",
		CompilerVersion:     "current-q128",
		CompilerProvenance:  testCompilerProvenance(),
		OptimizationEnabled: true,
		OptimizationRuns:    200,
		VerificationMethod:  "full-source",
	}
}

func TestDeploymentBoundVerificationRecordKeepsDefaultLayoutOnV2(t *testing.T) {
	target := v3TestTarget()
	sourceDigest := "qrl.verified-source-bundle.v1:sha256:" + strings.Repeat("e", 64)
	for name, result := range map[string]VerificationResult{
		"no primary path": v3TestResult(),
		"default path spelled out": func() VerificationResult {
			r := v3TestResult()
			r.PrimarySourcePath = "Main.hyp"
			return r
		}(),
	} {
		t.Run(name, func(t *testing.T) {
			schema, digest := DeploymentBoundVerificationRecord(target, result, sourceDigest)
			if schema != VerificationRecordSchemaV2 {
				t.Fatalf("schema = %q, want v2", schema)
			}
			if want := VerificationArtifactDigestV2(target, v3TestResult(), sourceDigest); digest != want {
				t.Fatalf("digest = %q, want the unchanged v2 digest %q", digest, want)
			}
		})
	}
}

func TestDeploymentBoundVerificationRecordUsesV3ForNewCompileInputs(t *testing.T) {
	target := v3TestTarget()
	sourceDigest := "qrl.verified-source-bundle.v1:sha256:" + strings.Repeat("e", 64)

	nested := v3TestResult()
	nested.PrimarySourcePath = "contracts/core/Main.hyp"
	viaIR := v3TestResult()
	viaIR.ViaIR = true
	both := nested
	both.ViaIR = true

	seen := map[string]string{
		"v2": VerificationArtifactDigestV2(target, v3TestResult(), sourceDigest),
	}
	for name, result := range map[string]VerificationResult{
		"nested path": nested,
		"via-IR":      viaIR,
		"both":        both,
	} {
		schema, digest := DeploymentBoundVerificationRecord(target, result, sourceDigest)
		if schema != VerificationRecordSchemaV3 || !IsVerificationArtifactDigestV3(digest) {
			t.Fatalf("%s: schema=%q digest=%q, want a v3 record", name, schema, digest)
		}
		if IsVerificationArtifactDigestV2(digest) {
			t.Fatalf("%s: v3 digest also parses as v2: %q", name, digest)
		}
		for previousName, previous := range seen {
			if previous == digest {
				t.Fatalf("%s and %s share artifact digest %q", name, previousName, digest)
			}
		}
		seen[name] = digest
	}

	// The default path written out and left empty bind the same effective
	// compiler input, so they must not produce two digests for one artifact.
	defaultSpelled := viaIR
	defaultSpelled.PrimarySourcePath = "Main.hyp"
	if VerificationArtifactDigestV3(target, defaultSpelled, sourceDigest) != seen["via-IR"] {
		t.Fatal("the default primary path changed the v3 digest")
	}
}

func TestCanonicalPrimarySourcePath(t *testing.T) {
	for _, tc := range []struct {
		name, path, want string
	}{
		{"Main", "", ""},
		{"Main", "Main.hyp", ""},
		{"Main", "src/Main.hyp", "src/Main.hyp"},
		{"Main", "Other.hyp", "Other.hyp"},
	} {
		if got := CanonicalPrimarySourcePath(tc.name, tc.path); got != tc.want {
			t.Fatalf("CanonicalPrimarySourcePath(%q, %q) = %q, want %q", tc.name, tc.path, got, tc.want)
		}
		if got := EffectivePrimarySourcePath(tc.name, tc.want); tc.want == "" && got != "Main.hyp" {
			t.Fatalf("EffectivePrimarySourcePath(%q, %q) = %q", tc.name, tc.want, got)
		}
	}
}
