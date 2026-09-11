package verification

import (
	"reflect"
	"strings"
	"testing"
)

func TestCanonicalizeVerifyRequestPersistsEffectiveInputs(t *testing.T) {
	originalImports := map[string]string{
		"./lib/Alpha.hyp":      "contract Alpha {}",
		"interfaces//Beta.hyp": "interface Beta {}",
	}
	req, err := CanonicalizeVerifyRequest(VerifyRequest{
		ContractName:     "Primary",
		OptimizerEnabled: true,
		OptimizerRuns:    0,
		Imports:          originalImports,
	})
	if err != nil {
		t.Fatalf("canonicalize request: %v", err)
	}
	wantImports := map[string]string{
		"lib/Alpha.hyp":       "contract Alpha {}",
		"interfaces/Beta.hyp": "interface Beta {}",
	}
	if !reflect.DeepEqual(req.Imports, wantImports) {
		t.Fatalf("canonical imports = %#v, want %#v", req.Imports, wantImports)
	}
	if req.OptimizerRuns != defaultOptimizerRuns {
		t.Fatalf("optimizerRuns = %d, want %d", req.OptimizerRuns, defaultOptimizerRuns)
	}
	if _, mutated := originalImports["lib/Alpha.hyp"]; mutated {
		t.Fatalf("input imports map was mutated: %#v", originalImports)
	}

	input, err := wrapStandardJSON(req)
	if err != nil {
		t.Fatalf("wrap standard JSON: %v", err)
	}
	if got := input.Sources[primarySourcePath(req.ContractName)].Content; got != req.SourceCode {
		t.Fatalf("primary source content = %q", got)
	}
	if len(input.Sources) != 3 {
		t.Fatalf("source count = %d, want 3", len(input.Sources))
	}
	if input.Settings.Optimizer == nil || input.Settings.Optimizer.Runs != defaultOptimizerRuns {
		t.Fatalf("optimizer settings = %+v", input.Settings.Optimizer)
	}
}

func TestCanonicalizeVerifyRequestRejectsSourceCollisions(t *testing.T) {
	t.Run("primary source", func(t *testing.T) {
		for _, importPath := range []string{"Primary.hyp", "./Primary.hyp", "lib/../Primary.hyp"} {
			_, err := CanonicalizeVerifyRequest(VerifyRequest{
				ContractName: "Primary",
				Imports:      map[string]string{importPath: "contract Hidden {}"},
			})
			if err == nil || !strings.Contains(err.Error(), "collides with primary source") {
				t.Fatalf("path %q error = %v", importPath, err)
			}
		}
	})

	t.Run("normalized aliases", func(t *testing.T) {
		_, err := CanonicalizeVerifyRequest(VerifyRequest{
			ContractName: "Primary",
			Imports: map[string]string{
				"lib/Alpha.hyp":   "contract AlphaA {}",
				"./lib/Alpha.hyp": "contract AlphaB {}",
			},
		})
		if err == nil || !strings.Contains(err.Error(), "normalize to the same source") {
			t.Fatalf("alias collision error = %v", err)
		}
	})
}

func TestCanonicalizeVerifyRequestRejectsUnsafePathsAndNames(t *testing.T) {
	for _, importPath := range []string{"", "/Absolute.hyp", "../Escape.hyp", "dir\\Windows.hyp", "bad\x00name.hyp"} {
		_, err := CanonicalizeVerifyRequest(VerifyRequest{
			ContractName: "Primary",
			Imports:      map[string]string{importPath: "contract Imported {}"},
		})
		if err == nil {
			t.Fatalf("unsafe path %q was accepted", importPath)
		}
	}
	for _, contractName := range []string{"", "Primary.hyp", "../Primary", "9Primary"} {
		_, err := CanonicalizeVerifyRequest(VerifyRequest{ContractName: contractName})
		if err == nil {
			t.Fatalf("invalid contract name %q was accepted", contractName)
		}
	}
}

func TestCanonicalizeVerifyRequestAcceptsDollarIdentifiers(t *testing.T) {
	for _, contractName := range []string{"$Primary", "Primary$V2"} {
		req, err := CanonicalizeVerifyRequest(VerifyRequest{ContractName: contractName})
		if err != nil {
			t.Fatalf("valid Hyperion contract name %q was rejected: %v", contractName, err)
		}
		if req.ContractName != contractName {
			t.Fatalf("contract name = %q, want %q", req.ContractName, contractName)
		}
	}
}

func TestFindContractBindsToPrimarySourceUnit(t *testing.T) {
	primary := CompiledContract{Metadata: "primary"}
	imported := CompiledContract{Metadata: "imported"}
	out := &StandardJSONOutput{Contracts: map[string]map[string]CompiledContract{
		"Primary.hyp":  {"Primary": primary},
		"Imported.hyp": {"Primary": imported},
	}}

	got, ok := findContract(out, "Primary.hyp", "Primary")
	if !ok || got.Metadata != "primary" {
		t.Fatalf("selected contract = %+v, ok=%v", got, ok)
	}

	delete(out.Contracts["Primary.hyp"], "Primary")
	if got, ok := findContract(out, "Primary.hyp", "Primary"); ok || got != nil {
		t.Fatalf("imported same-name contract was selected: %+v", got)
	}
}
