package models

import (
	"strings"
	"testing"
)

const (
	testCompilerBuildID        = "0.2.0-develop.2026.8.27+commit.6f862206.mod.Linux.g++"
	testCompilerSHA256         = "fe8e2344dbd902d6fc8c8cbb24114378c2de3a996b0d58f642d303c8bf30e930"
	testNsJailSHA256           = "cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc"
	testSandboxPolicySHA256    = "cc2c6d14e943c9b4b9252e69c2fbf9a5a4313938cd561594a255746393baaa8c"
	testExecutionDigestV1      = "6a2c91cf64b79e2367e96738b26a5c695479089b4043f60ce3d447ee8568cea5"
	testSandboxExecutionDigest = "fd84c18ce627b6e90bd2bc482e2033283ff7e8d2aeeb422a96be2f2921e669bf"
)

func validCompilerProvenance() *CompilerProvenance {
	return &CompilerProvenance{
		Schema:          CompilerProvenanceSchemaV2,
		Kind:            "native",
		BuildID:         testCompilerBuildID,
		ExecutionDigest: testSandboxExecutionDigest,
		Components: []CompilerProvenanceComponent{
			{Name: "hypc", SHA256: testCompilerSHA256},
			{Name: "nsjail", SHA256: testNsJailSHA256},
			{Name: "policy", SHA256: testSandboxPolicySHA256},
		},
	}
}

func TestNativeCompilerExecutionDigestV1MatchesCrossLanguageVector(t *testing.T) {
	if got := NativeCompilerExecutionDigestV1(testCompilerBuildID, testCompilerSHA256); got != testExecutionDigestV1 {
		t.Fatalf("execution digest = %q, want %q", got, testExecutionDigestV1)
	}
}

func TestNativeSandboxCompilerExecutionDigestV2MatchesCrossLanguageVector(t *testing.T) {
	got := NativeSandboxCompilerExecutionDigestV2(
		testCompilerBuildID,
		testCompilerSHA256,
		testNsJailSHA256,
		testSandboxPolicySHA256,
	)
	if got != testSandboxExecutionDigest {
		t.Fatalf("sandbox execution digest = %q, want %q", got, testSandboxExecutionDigest)
	}
}

func TestClassifyCompilerProvenance(t *testing.T) {
	if got := ClassifyCompilerProvenance(nil, "historical"); got != CompilerProvenanceLegacyUnrecorded {
		t.Fatalf("nil provenance = %q, want legacy-unrecorded", got)
	}
	if got := ClassifyCompilerProvenance(validCompilerProvenance(), testCompilerBuildID); got != CompilerProvenanceDigestBacked {
		t.Fatalf("valid provenance = %q, want digest-backed", got)
	}

	cases := map[string]func(*CompilerProvenance){
		"missing compiler version":  func(*CompilerProvenance) {},
		"build mismatch":            func(p *CompilerProvenance) { p.BuildID = "different" },
		"v1 lacks sandbox identity": func(p *CompilerProvenance) { p.Schema = CompilerProvenanceSchemaV1 },
		"unknown schema":            func(p *CompilerProvenance) { p.Schema = "qrl.contract-compiler-provenance.v3" },
		"wrong kind":                func(p *CompilerProvenance) { p.Kind = "npm" },
		"wrong component":           func(p *CompilerProvenance) { p.Components[0].Name = "runner" },
		"wrong component order": func(p *CompilerProvenance) {
			p.Components[0], p.Components[1] = p.Components[1], p.Components[0]
		},
		"extra component": func(p *CompilerProvenance) {
			p.Components = append(p.Components, CompilerProvenanceComponent{Name: "node", SHA256: testCompilerSHA256})
		},
		"uppercase component digest": func(p *CompilerProvenance) { p.Components[0].SHA256 = strings.ToUpper(testCompilerSHA256) },
		"unbound execution digest":   func(p *CompilerProvenance) { p.ExecutionDigest = strings.Repeat("c", 64) },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			provenance := validCompilerProvenance()
			mutate(provenance)
			compilerVersion := testCompilerBuildID
			if name == "missing compiler version" {
				compilerVersion = ""
			}
			if got := ClassifyCompilerProvenance(provenance, compilerVersion); got != CompilerProvenanceInvalidRecorded {
				t.Fatalf("classification = %q, want invalid-recorded", got)
			}
		})
	}
}
