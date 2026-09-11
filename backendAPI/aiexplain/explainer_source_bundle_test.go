package aiexplain

import (
	"strings"
	"testing"
	"unicode/utf8"

	"backendAPI/models"
	"backendAPI/sourcebundle"
)

func TestBuildSourceBundleUsesPrimaryThenSortedCanonicalImports(t *testing.T) {
	contract := models.ContractInfo{
		ContractName: "Primary",
		SourceCode:   "contract Primary {}",
		Imports: map[string]string{
			"z/Zeta.hyp":  "contract Zeta {}",
			"a/Alpha.hyp": "contract Alpha {}",
		},
	}

	bundle := buildSourceBundle(contract)
	primaryIndex := strings.Index(bundle, `// File: "Primary.hyp" (primary)`)
	alphaIndex := strings.Index(bundle, `// File: "a/Alpha.hyp" (import)`)
	zetaIndex := strings.Index(bundle, `// File: "z/Zeta.hyp" (import)`)
	if primaryIndex != 0 || alphaIndex <= primaryIndex || zetaIndex <= alphaIndex {
		t.Fatalf("source bundle order is not primary, alpha, zeta:\n%s", bundle)
	}
	for _, source := range []string{"contract Primary {}", "contract Alpha {}", "contract Zeta {}"} {
		if strings.Count(bundle, source) != 1 {
			t.Fatalf("source %q count = %d in bundle:\n%s", source, strings.Count(bundle, source), bundle)
		}
	}
}

func TestCapSourceBundleAppliesOneUTF8SafeAggregateLimit(t *testing.T) {
	bundle := buildSourceBundle(models.ContractInfo{
		ContractName: "Primary",
		SourceCode:   strings.Repeat("p", 80),
		Imports: map[string]string{
			"lib/Unicode.hyp": strings.Repeat("λ", 80),
		},
	})

	const limit = 96
	got := capSourceBundle(bundle, limit)
	if !utf8.ValidString(got) {
		t.Fatal("capped bundle is not valid UTF-8")
	}
	if runeCount := utf8.RuneCountInString(got); runeCount != limit {
		t.Fatalf("capped rune count = %d, want %d", runeCount, limit)
	}
	if !strings.HasSuffix(got, "// [truncated for length]") {
		t.Fatalf("capped bundle is missing truncation marker: %q", got)
	}
}

func TestCapSourceBundleLeavesSmallBundleExact(t *testing.T) {
	bundle := buildSourceBundle(models.ContractInfo{
		ContractName: "Primary",
		SourceCode:   "contract Primary {}",
	})
	if got := capSourceBundle(bundle, len([]rune(bundle))); got != bundle {
		t.Fatalf("uncapped bundle changed:\n%s", got)
	}
}

func TestCachedExplanationMatchesExactSourceBundleOnly(t *testing.T) {
	digest := sourcebundle.Digest("Primary", "current bundle", nil)
	contract := models.ContractInfo{
		AIExplanation:             "current explanation",
		SourceBundleDigest:        digest,
		AIExplanationSourceDigest: digest,
	}
	if !cachedExplanationMatches(contract, digest) {
		t.Fatal("matching digest-backed cache was rejected")
	}

	contract.AIExplanationSourceDigest = sourcebundle.Digest("Primary", "old bundle", nil)
	if cachedExplanationMatches(contract, digest) {
		t.Fatal("cache generated from an old bundle was accepted")
	}

	contract.AIExplanationSourceDigest = ""
	contract.SourceBundleDigest = ""
	if cachedExplanationMatches(contract, digest) {
		t.Fatal("legacy cache without a source digest was accepted")
	}
}

func TestExplanationRecordRequiresDigestBackedCompilerAndSourceBundle(t *testing.T) {
	const buildID = "current-q128"
	hypcSHA256 := strings.Repeat("a", 64)
	nsjailSHA256 := strings.Repeat("b", 64)
	policySHA256 := strings.Repeat("c", 64)
	contract := models.ContractInfo{
		VerificationRecordSchema: models.VerificationRecordSchemaV1,
		ContractName:             "Primary",
		SourceCode:               "contract Primary {}",
		CompilerVersion:          buildID,
		SourceBundleDigest:       sourcebundle.Digest("Primary", "contract Primary {}", nil),
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
	if !explanationRecordIsDigestBacked(contract) {
		t.Fatal("complete digest-backed record was rejected")
	}

	legacy := contract
	legacy.CompilerProvenance = nil
	if explanationRecordIsDigestBacked(legacy) {
		t.Fatal("legacy-unrecorded compiler provenance was accepted")
	}

	invalid := contract
	invalid.CompilerProvenance = &models.CompilerProvenance{
		Schema:          models.CompilerProvenanceSchemaV2,
		Kind:            "native",
		BuildID:         buildID,
		ExecutionDigest: strings.Repeat("b", 64),
		Components:      contract.CompilerProvenance.Components,
	}
	if explanationRecordIsDigestBacked(invalid) {
		t.Fatal("invalid-recorded compiler provenance was accepted")
	}

	undigested := contract
	undigested.SourceBundleDigest = ""
	if explanationRecordIsDigestBacked(undigested) {
		t.Fatal("record without a source-bundle digest was accepted")
	}

	mismatched := contract
	mismatched.SourceBundleDigest = sourcebundle.Digest("Primary", "different source", nil)
	if explanationRecordIsDigestBacked(mismatched) {
		t.Fatal("record with a mismatched source-bundle digest was accepted")
	}
}
