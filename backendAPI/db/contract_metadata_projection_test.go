package db

import "testing"

func TestContractMetadataProjectionKeepsTrustMetadataWithoutSourceBytes(t *testing.T) {
	excluded := make(map[string]bool)
	for _, element := range contractMetadataProjection() {
		value, ok := element.Value.(int)
		if !ok || value != 0 {
			t.Fatalf("projection %q = %#v, want exclusion value 0", element.Key, element.Value)
		}
		excluded[element.Key] = true
	}

	for _, field := range []string{"verificationRecordSchema", "compilerVersion", "compilerProvenance", "sourceBundleDigest", "abi"} {
		if excluded[field] {
			t.Fatalf("trust metadata field %q was projected out", field)
		}
	}
	for _, field := range []string{"sourceCode", "imports", "contractCode", "aiExplanation"} {
		if !excluded[field] {
			t.Fatalf("heavy field %q was not projected out", field)
		}
	}
}
