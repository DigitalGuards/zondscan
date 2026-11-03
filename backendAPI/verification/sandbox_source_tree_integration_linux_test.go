//go:build linux

package verification

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// Synthetic multi-directory project written for this test. The primary
// source lives two directories deep and reaches its dependencies through
// "../" imports, the layout the root-level <Name>.hyp flow cannot express.
var sourceTreeIntegrationFixture = map[string]string{
	"contracts/core/Counter.hyp": `// SPDX-License-Identifier: MIT
pragma hyperion >=0.0.2;

import "../lib/MathLib.hyp";
import {Base} from "../base/Base.hyp";

contract Counter is Base {
    function twice(uint256 x) public pure returns (uint256) {
        return MathLib.double(x);
    }
}
`,
	"contracts/lib/MathLib.hyp": `// SPDX-License-Identifier: MIT
pragma hyperion >=0.0.2;

library MathLib {
    function double(uint256 x) internal pure returns (uint256) {
        return x * 2;
    }
}
`,
	"contracts/base/Base.hyp": `// SPDX-License-Identifier: MIT
pragma hyperion >=0.0.2;

abstract contract Base {
    uint256 public stored = 7;
}
`,
}

const sourceTreeIntegrationPrimary = "contracts/core/Counter.hyp"

func sourceTreeIntegrationRequest(viaIR bool) VerifyRequest {
	imports := map[string]string{}
	for unit, content := range sourceTreeIntegrationFixture {
		if unit != sourceTreeIntegrationPrimary {
			imports[unit] = content
		}
	}
	return VerifyRequest{
		ContractName:      "Counter",
		SourceCode:        sourceTreeIntegrationFixture[sourceTreeIntegrationPrimary],
		PrimarySourcePath: sourceTreeIntegrationPrimary,
		Imports:           imports,
		OptimizerEnabled:  true,
		OptimizerRuns:     200,
		ViaIR:             viaIR,
	}
}

// compileSourceTreeWithCLI compiles the fixture the way a deployer would:
// the files on disk and hypc run from the project root. Its runtime code
// stands in for the on-chain code of a contract deployed from that build.
func compileSourceTreeWithCLI(t *testing.T, hypc string, viaIR bool) string {
	t.Helper()
	root := t.TempDir()
	for unit, content := range sourceTreeIntegrationFixture {
		file := filepath.Join(root, filepath.FromSlash(unit))
		if err := os.MkdirAll(filepath.Dir(file), 0o700); err != nil {
			t.Fatalf("create fixture directory: %v", err)
		}
		if err := os.WriteFile(file, []byte(content), 0o600); err != nil {
			t.Fatalf("write fixture source: %v", err)
		}
	}
	args := []string{"--optimize", "--optimize-runs", "200", "--bin-runtime"}
	if viaIR {
		args = append(args, "--via-ir")
	}
	cmd := exec.Command(hypc, append(args, sourceTreeIntegrationPrimary)...)
	cmd.Dir = root
	cmd.Env = []string{}
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("compile fixture with the hypc CLI: %v: %s", err, output)
	}
	header := "======= " + sourceTreeIntegrationPrimary + ":Counter ======="
	_, section, found := strings.Cut(string(output), header)
	if !found {
		t.Fatalf("hypc CLI output has no %s section: %s", header, output)
	}
	_, runtime, found := strings.Cut(section, "Binary of the runtime part:")
	if !found {
		t.Fatalf("hypc CLI output has no runtime binary: %s", section)
	}
	runtime = strings.TrimSpace(runtime)
	if line, _, cut := strings.Cut(runtime, "\n"); cut {
		runtime = strings.TrimSpace(line)
	}
	if runtime == "" {
		t.Fatalf("hypc CLI produced empty runtime code: %s", section)
	}
	return runtime
}

func TestRealStaticHyperionVerifiesSourceTreeThroughSandbox(t *testing.T) {
	sandbox := realSandboxForIntegrationTest(t)
	path := strings.TrimSpace(os.Getenv("VERIFIER_TEST_HYPC_BIN"))
	digest := strings.TrimSpace(os.Getenv("VERIFIER_TEST_HYPC_SHA256"))
	buildID := strings.TrimSpace(os.Getenv("VERIFIER_TEST_HYPC_BUILD_ID"))
	if path == "" || digest == "" || buildID == "" {
		t.Skip("set VERIFIER_TEST_HYPC_BIN, VERIFIER_TEST_HYPC_SHA256, and VERIFIER_TEST_HYPC_BUILD_ID for the real Hyperion probe")
	}
	compiler := compilerWithRealSandbox(t, sandbox, path, digest, buildID)

	compileThroughVerifier := func(t *testing.T, req VerifyRequest) *CompiledContract {
		t.Helper()
		canonical, err := CanonicalizeVerifyRequest(req)
		if err != nil {
			t.Fatalf("canonicalize source tree request: %v", err)
		}
		input, err := wrapStandardJSON(canonical)
		if err != nil {
			t.Fatalf("wrap source tree request: %v", err)
		}
		output, err := compiler.Compile(context.Background(), input)
		if err != nil {
			t.Fatalf("compile source tree through NsJail: %v", err)
		}
		if fatal := output.FatalErrors(); len(fatal) > 0 {
			t.Fatalf("source tree compile errors: %s", formatCompileErrors(fatal))
		}
		contract, ok := findContract(output, effectivePrimarySourcePath(canonical), canonical.ContractName)
		if !ok {
			t.Fatalf("Counter not found in primary unit %q", effectivePrimarySourcePath(canonical))
		}
		if contract.DeployedBytecode().Object == "" {
			t.Fatal("verifier produced empty runtime code")
		}
		return contract
	}

	for _, viaIR := range []bool{true, false} {
		name := "legacy pipeline"
		if viaIR {
			name = "via-IR"
		}
		t.Run(name, func(t *testing.T) {
			contract := compileThroughVerifier(t, sourceTreeIntegrationRequest(viaIR))
			compiled := contract.DeployedBytecode()
			onChain := compileSourceTreeWithCLI(t, path, viaIR)

			// Full equality, CBOR metadata trailer included: the trailer
			// hashes every source path, so this proves the verifier laid the
			// tree out exactly as the deployer's build saw it.
			if !strings.EqualFold(strings.TrimPrefix(compiled.Object, "0x"), onChain) {
				t.Fatalf("sandboxed runtime code differs from the CLI build:\nsandbox %s\ncli     %s", compiled.Object, onChain)
			}
			match, err := Match(compiled.Object, onChain, compiled.ImmutableReferences)
			if err != nil {
				t.Fatalf("match source tree runtime code: %v", err)
			}
			if !match.Matched {
				t.Fatalf("source tree did not verify: %+v", match)
			}

			// The flag must reach the compiler: the other pipeline yields
			// different code and must not verify against this deployment.
			other := compileThroughVerifier(t, sourceTreeIntegrationRequest(!viaIR)).DeployedBytecode()
			mismatch, err := Match(other.Object, onChain, other.ImmutableReferences)
			if err != nil {
				t.Fatalf("match other pipeline: %v", err)
			}
			if mismatch.Matched {
				t.Fatalf("viaIR=%v code verified a viaIR=%v deployment", !viaIR, viaIR)
			}
		})
	}
}

func TestRealStaticHyperionReportsMissingTreeSourceAsCompileError(t *testing.T) {
	sandbox := realSandboxForIntegrationTest(t)
	path := strings.TrimSpace(os.Getenv("VERIFIER_TEST_HYPC_BIN"))
	digest := strings.TrimSpace(os.Getenv("VERIFIER_TEST_HYPC_SHA256"))
	buildID := strings.TrimSpace(os.Getenv("VERIFIER_TEST_HYPC_BUILD_ID"))
	if path == "" || digest == "" || buildID == "" {
		t.Skip("set VERIFIER_TEST_HYPC_BIN, VERIFIER_TEST_HYPC_SHA256, and VERIFIER_TEST_HYPC_BUILD_ID for the real Hyperion probe")
	}
	compiler := compilerWithRealSandbox(t, sandbox, path, digest, buildID)

	// A tree that leaves out one imported file must fail with the
	// compiler's own message. With the import callback enabled, hypc would
	// try to read the file from disk and the sandbox would kill it.
	req := sourceTreeIntegrationRequest(true)
	delete(req.Imports, "contracts/lib/MathLib.hyp")
	input, err := wrapStandardJSON(req)
	if err != nil {
		t.Fatalf("wrap source tree request: %v", err)
	}
	output, err := compiler.Compile(context.Background(), input)
	if err != nil {
		t.Fatalf("missing source surfaced as an infrastructure error: %v", err)
	}
	message := formatCompileErrors(output.FatalErrors())
	if !strings.Contains(message, `"contracts/lib/MathLib.hyp" not found`) {
		t.Fatalf("missing source error = %q", message)
	}
}
