package verification

import (
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"
)

func sourceTreeRequest() VerifyRequest {
	return VerifyRequest{
		ContractName:      "Counter",
		SourceCode:        "import \"../lib/MathLib.hyp\"; contract Counter {}",
		PrimarySourcePath: "contracts/core/Counter.hyp",
		Imports: map[string]string{
			"contracts/lib/MathLib.hyp": "library MathLib {}",
			"contracts/base/Base.hyp":   "abstract contract Base {}",
		},
	}
}

func TestCanonicalizeVerifyRequestAcceptsSourceTree(t *testing.T) {
	raw := sourceTreeRequest()
	raw.PrimarySourcePath = "./contracts//core/Counter.hyp"
	raw.Imports = map[string]string{
		"./contracts/lib/MathLib.hyp":            "library MathLib {}",
		"contracts/core/../base/Base.hyp":        "abstract contract Base {}",
		"node_modules/@scope/pkg/Owned+v2~1.hyp": "contract Owned {}",
	}
	req, err := CanonicalizeVerifyRequest(raw)
	if err != nil {
		t.Fatalf("canonicalize source tree: %v", err)
	}
	if req.PrimarySourcePath != "contracts/core/Counter.hyp" {
		t.Fatalf("primarySourcePath = %q", req.PrimarySourcePath)
	}
	wantImports := map[string]string{
		"contracts/lib/MathLib.hyp":              "library MathLib {}",
		"contracts/base/Base.hyp":                "abstract contract Base {}",
		"node_modules/@scope/pkg/Owned+v2~1.hyp": "contract Owned {}",
	}
	if !reflect.DeepEqual(req.Imports, wantImports) {
		t.Fatalf("imports = %#v, want %#v", req.Imports, wantImports)
	}

	again, err := CanonicalizeVerifyRequest(req)
	if err != nil || !reflect.DeepEqual(again, req) {
		t.Fatalf("canonicalization is not idempotent: %+v, %v", again, err)
	}
}

// The single-file imports flow always accepted file names with spaces, so
// existing users can keep submitting them, in the old layout and in a tree.
func TestCanonicalizeVerifyRequestAcceptsSpacesInPaths(t *testing.T) {
	single, err := CanonicalizeVerifyRequest(VerifyRequest{
		ContractName: "Counter",
		SourceCode:   "import \"./My Token.hyp\"; contract Counter {}",
		Imports:      map[string]string{"./My Token.hyp": "contract MyToken {}"},
	})
	if err != nil {
		t.Fatalf("canonicalize single-file request with a spaced import: %v", err)
	}
	if want := map[string]string{"My Token.hyp": "contract MyToken {}"}; !reflect.DeepEqual(single.Imports, want) {
		t.Fatalf("imports = %#v, want %#v", single.Imports, want)
	}
	input, err := wrapStandardJSON(single)
	if err != nil {
		t.Fatalf("wrap standard JSON: %v", err)
	}
	if input.Sources["My Token.hyp"].Content != "contract MyToken {}" {
		t.Fatalf("spaced import missing from sources: %#v", input.Sources)
	}

	tree, err := CanonicalizeVerifyRequest(VerifyRequest{
		ContractName:      "Counter",
		SourceCode:        "import \"../my lib/Math Lib.hyp\"; contract Counter {}",
		PrimarySourcePath: "./my contracts/core/Counter.hyp",
		Imports:           map[string]string{"my contracts/my lib/Math Lib.hyp": "library MathLib {}"},
	})
	if err != nil {
		t.Fatalf("canonicalize source tree with spaced paths: %v", err)
	}
	if tree.PrimarySourcePath != "my contracts/core/Counter.hyp" {
		t.Fatalf("primarySourcePath = %q", tree.PrimarySourcePath)
	}
	if _, ok := tree.Imports["my contracts/my lib/Math Lib.hyp"]; !ok {
		t.Fatalf("imports = %#v", tree.Imports)
	}
	again, err := CanonicalizeVerifyRequest(tree)
	if err != nil || !reflect.DeepEqual(again, tree) {
		t.Fatalf("canonicalization is not idempotent: %+v, %v", again, err)
	}
}

func TestCanonicalizeVerifyRequestStoresDefaultPrimaryPathAsEmpty(t *testing.T) {
	for _, primary := range []string{"", "Counter.hyp", "./Counter.hyp", "lib/../Counter.hyp"} {
		req, err := CanonicalizeVerifyRequest(VerifyRequest{
			ContractName:      "Counter",
			PrimarySourcePath: primary,
		})
		if err != nil {
			t.Fatalf("primary %q: %v", primary, err)
		}
		if req.PrimarySourcePath != "" {
			t.Fatalf("primary %q canonicalized to %q, want empty", primary, req.PrimarySourcePath)
		}
		if got := effectivePrimarySourcePath(req); got != "Counter.hyp" {
			t.Fatalf("effective primary = %q", got)
		}
	}
}

func TestCanonicalizeVerifyRequestRejectsUnsafeSourceTreePaths(t *testing.T) {
	long := strings.Repeat("a/", maxSourcePathBytes/2) + "X.hyp"
	deep := strings.Repeat("d/", maxSourcePathSegments) + "X.hyp"
	cases := []struct {
		name    string
		primary string
		path    string
		wantErr string
	}{
		{"traversal", "", "../lib/X.hyp", "escapes the source root"},
		{"nested traversal", "", "contracts/../../X.hyp", "escapes the source root"},
		{"bare parent", "", "..", "escapes the source root"},
		{"dot", "", ".", "escapes the source root"},
		{"absolute", "", "/etc/passwd", "must be relative"},
		{"empty", "", "", "is empty"},
		{"backslash", "", `contracts\X.hyp`, "forward slashes"},
		{"drive letter", "", "C:/X.hyp", "unsupported character"},
		{"url scheme", "", "https://example.com/X.hyp", "unsupported character"},
		{"NUL", "", "lib/X\x00.hyp", "unsupported character"},
		{"newline", "", "lib/X\n.hyp", "unsupported character"},
		{"tab", "", "my\tlib/X.hyp", "unsupported character"},
		{"DEL", "", "lib/X\x7f.hyp", "unsupported character"},
		{"traversal with spaces", "", "my lib/../../My Token.hyp", "escapes the source root"},
		{"absolute with spaces", "", "/my lib/My Token.hyp", "must be relative"},
		{"direction override", "", "lib/X\u202ephy.hyp", "unsupported character"},
		{"look-alike", "", "lib/\u0425.hyp", "unsupported character"},
		{"too long", "", long, "longer than"},
		{"too deep", "", deep, "path segments"},
		{"primary traversal", "../Counter.hyp", "lib/X.hyp", "escapes the source root"},
		{"primary absolute", "/contracts/Counter.hyp", "lib/X.hyp", "must be relative"},
		{"primary traversal with spaces", "my contracts/../../Counter.hyp", "lib/X.hyp", "escapes the source root"},
		{"primary control character", "contracts/Count\ver.hyp", "lib/X.hyp", "unsupported character"},
		{"primary not hyp", "contracts/Counter.sol", "lib/X.hyp", "must name a .hyp file"},
		{"primary bare extension", "contracts/.hyp", "lib/X.hyp", "must name a .hyp file"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := CanonicalizeVerifyRequest(VerifyRequest{
				ContractName:      "Counter",
				PrimarySourcePath: tc.primary,
				Imports:           map[string]string{tc.path: "contract Imported {}"},
			})
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("error = %v, want %q", err, tc.wantErr)
			}
		})
	}
}

func TestCanonicalizeVerifyRequestRejectsSourceTreeAliases(t *testing.T) {
	cases := []struct {
		name    string
		imports map[string]string
		wantErr string
	}{
		{
			name:    "primary collision",
			imports: map[string]string{"contracts/lib/../core/Counter.hyp": "x"},
			wantErr: "collides with primary source",
		},
		{
			name: "duplicate after normalization",
			imports: map[string]string{
				"contracts/lib/MathLib.hyp":   "a",
				"./contracts/lib/MathLib.hyp": "b",
			},
			wantErr: "normalize to the same source",
		},
		{
			name: "case-only duplicate",
			imports: map[string]string{
				"contracts/lib/MathLib.hyp": "a",
				"contracts/lib/mathlib.hyp": "b",
			},
			wantErr: "differ only by letter case",
		},
		{
			name:    "case-only primary collision",
			imports: map[string]string{"Contracts/Core/Counter.hyp": "a"},
			wantErr: "differ only by letter case",
		},
		{
			name: "case-only duplicate with spaces",
			imports: map[string]string{
				"contracts/lib/My Token.hyp": "a",
				"contracts/lib/my token.hyp": "b",
			},
			wantErr: "differ only by letter case",
		},
		{
			name:    "root import under the primary label",
			imports: map[string]string{"Counter.hyp": "a"},
			wantErr: "reserves for the primary source",
		},
		{
			name:    "root import under the primary label with dot segments",
			imports: map[string]string{"contracts/../Counter.hyp": "a"},
			wantErr: "reserves for the primary source",
		},
		{
			name:    "root import under the primary label in another case",
			imports: map[string]string{"counter.hyp": "a"},
			wantErr: "reserves for the primary source",
		},
		{
			name: "file used as directory",
			imports: map[string]string{
				"contracts/lib.hyp":          "a",
				"contracts/lib.hyp/Math.hyp": "b",
			},
			wantErr: "used as a file and as a directory",
		},
		{
			name:    "primary used as directory",
			imports: map[string]string{"contracts/core/Counter.hyp/Shadow.hyp": "a"},
			wantErr: "used as a file and as a directory",
		},
		{
			name: "file used as directory with different case",
			imports: map[string]string{
				"contracts/Lib":          "a",
				"contracts/lib/Math.hyp": "b",
			},
			wantErr: "used as a file and as a directory",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := sourceTreeRequest()
			req.Imports = tc.imports
			_, err := CanonicalizeVerifyRequest(req)
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("error = %v, want %q", err, tc.wantErr)
			}
		})
	}
}

// Only the root name is reserved: a nested import may share the primary's
// file name, and the default layout keeps its original collision error.
func TestCanonicalizeVerifyRequestReservesPrimaryLabelOnlyAtTheRoot(t *testing.T) {
	req := sourceTreeRequest()
	req.Imports = map[string]string{"contracts/legacy/Counter.hyp": "contract Counter {}"}
	if _, err := CanonicalizeVerifyRequest(req); err != nil {
		t.Fatalf("nested import sharing the primary file name was rejected: %v", err)
	}

	_, err := CanonicalizeVerifyRequest(VerifyRequest{
		ContractName: "Counter",
		Imports:      map[string]string{"./Counter.hyp": "contract Counter {}"},
	})
	if err == nil || !strings.Contains(err.Error(), "collides with primary source") {
		t.Fatalf("default layout collision error = %v", err)
	}
}

func TestCanonicalizeVerifyRequestLimitsSourceCount(t *testing.T) {
	imports := make(map[string]string, maxSourceFiles)
	for i := 0; i < maxSourceFiles-1; i++ {
		imports[fmt.Sprintf("lib/F%03d.hyp", i)] = "contract F {}"
	}
	req := sourceTreeRequest()
	req.Imports = imports
	if _, err := CanonicalizeVerifyRequest(req); err != nil {
		t.Fatalf("request at the source limit was rejected: %v", err)
	}
	imports["lib/Overflow.hyp"] = "contract F {}"
	if _, err := CanonicalizeVerifyRequest(req); err == nil || !strings.Contains(err.Error(), "too many source files") {
		t.Fatalf("request over the source limit error = %v", err)
	}
}

func TestWrapStandardJSONLaysOutSourceTreeAndViaIR(t *testing.T) {
	req := sourceTreeRequest()
	req.ViaIR = true
	req.Libraries = map[string]string{"MathLib": "Q" + strings.Repeat("1", 128)}
	input, err := wrapStandardJSON(req)
	if err != nil {
		t.Fatalf("wrap standard JSON: %v", err)
	}
	wantSources := map[string]string{
		"contracts/core/Counter.hyp": req.SourceCode,
		"contracts/lib/MathLib.hyp":  "library MathLib {}",
		"contracts/base/Base.hyp":    "abstract contract Base {}",
	}
	if len(input.Sources) != len(wantSources) {
		t.Fatalf("sources = %#v", input.Sources)
	}
	for unit, content := range wantSources {
		if input.Sources[unit].Content != content {
			t.Fatalf("source %q = %q, want %q", unit, input.Sources[unit].Content, content)
		}
	}
	if _, exists := input.Sources["Counter.hyp"]; exists {
		t.Fatal("primary source was also written at the root")
	}
	if !input.Settings.ViaIR {
		t.Fatal("viaIR was not passed to the compiler settings")
	}
	if _, ok := input.Settings.Libraries["contracts/core/Counter.hyp"]; !ok || len(input.Settings.Libraries) != 1 {
		t.Fatalf("libraries were not keyed by the primary source: %#v", input.Settings.Libraries)
	}

	payload, err := json.Marshal(input)
	if err != nil {
		t.Fatalf("marshal standard JSON: %v", err)
	}
	if !strings.Contains(string(payload), `"viaIR":true`) {
		t.Fatalf("standard JSON lacks viaIR: %s", payload)
	}
}

func TestWrapStandardJSONDefaultInputIsUnchanged(t *testing.T) {
	input, err := wrapStandardJSON(VerifyRequest{
		ContractName: "Counter",
		SourceCode:   "contract Counter {}",
	})
	if err != nil {
		t.Fatalf("wrap standard JSON: %v", err)
	}
	payload, err := json.Marshal(input)
	if err != nil {
		t.Fatalf("marshal standard JSON: %v", err)
	}
	// This is the exact payload the single-file flow sent before source trees
	// and via-IR existed. Any change here changes every existing verification.
	const want = `{"language":"Hyperion","sources":{"Counter.hyp":{"content":"contract Counter {}"}},` +
		`"settings":{"outputSelection":{"*":{"*":["abi","zvm.deployedBytecode.object",` +
		`"zvm.deployedBytecode.immutableReferences","qrvm.deployedBytecode.object",` +
		`"qrvm.deployedBytecode.immutableReferences","metadata"]}}}}`
	if string(payload) != want {
		t.Fatalf("default standard JSON changed:\n got %s\nwant %s", payload, want)
	}
}

func TestCompilerCheckRequestSizeRejectsOversizedTreesBeforeEnqueue(t *testing.T) {
	req := sourceTreeRequest()
	input, err := wrapStandardJSON(req)
	if err != nil {
		t.Fatalf("wrap standard JSON: %v", err)
	}
	payload, err := json.Marshal(input)
	if err != nil {
		t.Fatalf("marshal standard JSON: %v", err)
	}

	if err := (&Compiler{MaxStdin: len(payload)}).CheckRequestSize(req); err != nil {
		t.Fatalf("request at the limit was rejected: %v", err)
	}
	err = (&Compiler{MaxStdin: len(payload) - 1}).CheckRequestSize(req)
	if !errors.Is(err, ErrPayloadTooLarge) || !strings.Contains(err.Error(), "above the limit") {
		t.Fatalf("oversized request error = %v", err)
	}
	if err := (&Compiler{}).CheckRequestSize(req); err != nil {
		t.Fatalf("unlimited compiler rejected the request: %v", err)
	}

	bad := req
	bad.PrimarySourcePath = "../Counter.hyp"
	if err := (&Compiler{MaxStdin: 1 << 20}).CheckRequestSize(bad); err == nil {
		t.Fatal("invalid request passed the size check")
	}
}
