package sourcebundle

import (
	"strings"
	"testing"
)

func TestRenderAndDigestAreDeterministicAcrossMapOrder(t *testing.T) {
	left := map[string]string{
		"z/Zeta.hyp":  "contract Zeta {}",
		"a/Alpha.hyp": "contract Alpha {}",
	}
	right := map[string]string{
		"a/Alpha.hyp": "contract Alpha {}",
		"z/Zeta.hyp":  "contract Zeta {}",
	}
	leftFrame := Render("Primary", "contract Primary {}", left)
	rightFrame := Render("Primary", "contract Primary {}", right)
	if leftFrame != rightFrame {
		t.Fatalf("rendered frames differ:\nleft:\n%s\nright:\n%s", leftFrame, rightFrame)
	}
	leftDigest := Digest("Primary", "contract Primary {}", left)
	rightDigest := Digest("Primary", "contract Primary {}", right)
	if leftDigest != rightDigest {
		t.Fatalf("digests differ: %q != %q", leftDigest, rightDigest)
	}
	if !strings.HasPrefix(leftDigest, Version+":sha256:") {
		t.Fatalf("digest %q does not carry source-bundle version", leftDigest)
	}
}

func TestDigestChangesWithEveryBundleIdentityInput(t *testing.T) {
	baseline := Digest("Primary", "contract Primary {}", map[string]string{
		"lib/A.hyp": "contract A {}",
	})
	variants := []string{
		Digest("Renamed", "contract Primary {}", map[string]string{"lib/A.hyp": "contract A {}"}),
		Digest("Primary", "contract Primary { function f() {} }", map[string]string{"lib/A.hyp": "contract A {}"}),
		Digest("Primary", "contract Primary {}", map[string]string{"lib/B.hyp": "contract A {}"}),
		Digest("Primary", "contract Primary {}", map[string]string{"lib/A.hyp": "contract A { function f() {} }"}),
	}
	for index, variant := range variants {
		if variant == baseline {
			t.Fatalf("variant %d retained baseline digest %q", index, baseline)
		}
	}
}

func TestDigestDistinguishesTrailingNewlineHiddenByPromptRender(t *testing.T) {
	withoutNewline := "contract Primary {}"
	withNewline := withoutNewline + "\n"
	if Render("Primary", withoutNewline, nil) != Render("Primary", withNewline, nil) {
		t.Fatal("test setup no longer produces the same prompt rendering")
	}
	if Digest("Primary", withoutNewline, nil) == Digest("Primary", withNewline, nil) {
		t.Fatal("exact source digest collapsed a trailing-newline difference")
	}
}

func TestDigestDistinguishesInjectedPromptFileBoundary(t *testing.T) {
	primary := "contract Primary {}"
	importSource := "contract A {}"
	injectedPrimary := primary + "\n\n// File: \"lib/A.hyp\" (import)\n" + importSource + "\n"
	imports := map[string]string{"lib/A.hyp": importSource}
	if Render("Primary", injectedPrimary, nil) != Render("Primary", primary, imports) {
		t.Fatal("test setup no longer produces the same prompt rendering")
	}
	if Digest("Primary", injectedPrimary, nil) == Digest("Primary", primary, imports) {
		t.Fatal("exact source digest collapsed an injected file boundary")
	}
}

func TestDigestCrossLanguageUnicodeVector(t *testing.T) {
	imports := map[string]string{
		"src/βeta/Math.hyp":      "library Math { /* π */ }\n",
		"src/Ångström/Types.hyp": "struct Café { uint256 value; }\n",
	}
	const expected = "qrl.verified-source-bundle.v1:sha256:f8a35959e8d3ba5e9e55b0a563d5563db22b98f9486c9557e84d353ac8524202"
	if got := Digest(
		"München",
		"contract München {\n    string public greeting = \"你好\";\n}\n",
		imports,
	); got != expected {
		t.Fatalf("cross-language source digest = %q, want %q", got, expected)
	}
}
