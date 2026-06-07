package verification

import (
	"os"
	"path/filepath"
	"testing"
)

// localBuildID is the build id reported by the npm @theqrl/hypc bundled in
// verification/runner/node_modules. The integration test below probes the
// real runner, so it only runs where that runner is installed and node is on
// PATH; everywhere else it skips.
const localBuildID = "0.0.2+commit.3e18e55d.Emscripten.clang"

func localRunner(t *testing.T) string {
	t.Helper()
	abs, err := filepath.Abs(filepath.Join("runner", "hypc-runner.js"))
	if err != nil {
		t.Fatalf("abs runner path: %v", err)
	}
	return abs
}

// clearVerifierEnv blanks every env var loadCompilerSpecs consults so each
// case starts from a known state (t.Setenv restores originals on cleanup).
func clearVerifierEnv(t *testing.T) {
	t.Helper()
	for _, k := range []string{"HYPC_COMPILERS", "HYPC_RUNNER", "HYPC_BUILD_ID", "HYPC_NODE_BIN", "HYPC_BIN"} {
		t.Setenv(k, "")
	}
}

func TestLoadCompilerSpecs_InlineArray(t *testing.T) {
	clearVerifierEnv(t)
	t.Setenv("HYPC_COMPILERS", `  [
		{"buildId":"a","runner":"/x/a.js","default":true},
		{"buildId":"b","runner":"/x/b.sh","nodeBin":"/bin/sh","bin":"/usr/bin/hypc-b"}
	]`)

	specs, err := loadCompilerSpecs()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(specs) != 2 {
		t.Fatalf("want 2 specs, got %d", len(specs))
	}
	if specs[0].BuildID != "a" || !specs[0].Default {
		t.Errorf("spec[0] = %+v", specs[0])
	}
	if specs[1].NodeBin != "/bin/sh" || specs[1].Bin != "/usr/bin/hypc-b" {
		t.Errorf("spec[1] = %+v", specs[1])
	}
}

func TestLoadCompilerSpecs_FilePath(t *testing.T) {
	clearVerifierEnv(t)
	path := filepath.Join(t.TempDir(), "compilers.json")
	if err := os.WriteFile(path, []byte(`[{"buildId":"x","runner":"/r.js"}]`), 0o600); err != nil {
		t.Fatalf("write temp manifest: %v", err)
	}
	t.Setenv("HYPC_COMPILERS", path)

	specs, err := loadCompilerSpecs()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(specs) != 1 || specs[0].BuildID != "x" || specs[0].Runner != "/r.js" {
		t.Fatalf("unexpected specs: %+v", specs)
	}
}

func TestLoadCompilerSpecs_LegacyEnv(t *testing.T) {
	clearVerifierEnv(t)
	t.Setenv("HYPC_RUNNER", "/legacy/runner.sh")
	t.Setenv("HYPC_BUILD_ID", "legacy-1")
	t.Setenv("HYPC_NODE_BIN", "/bin/sh")
	t.Setenv("HYPC_BIN", "/opt/hypc")

	specs, err := loadCompilerSpecs()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(specs) != 1 {
		t.Fatalf("want 1 legacy spec, got %d", len(specs))
	}
	got := specs[0]
	if got.BuildID != "legacy-1" || got.Runner != "/legacy/runner.sh" ||
		got.NodeBin != "/bin/sh" || got.Bin != "/opt/hypc" || !got.Default {
		t.Fatalf("legacy spec mismatch: %+v", got)
	}
}

func TestLoadCompilerSpecs_Unconfigured(t *testing.T) {
	clearVerifierEnv(t)
	specs, err := loadCompilerSpecs()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(specs) != 0 {
		t.Fatalf("want no specs when unconfigured, got %+v", specs)
	}
}

func TestLoadCompilerSpecs_MissingFields(t *testing.T) {
	cases := map[string]string{
		"missing runner":  `[{"buildId":"a"}]`,
		"missing buildId": `[{"runner":"/r.js"}]`,
	}
	for name, manifest := range cases {
		t.Run(name, func(t *testing.T) {
			clearVerifierEnv(t)
			t.Setenv("HYPC_COMPILERS", manifest)
			if _, err := loadCompilerSpecs(); err == nil {
				t.Fatalf("expected error for %q", name)
			}
		})
	}
}

// TestRegistry_Integration boots a real Registry against the bundled npm
// hypc runner. It is the end-to-end check that the legacy/multi config paths,
// the version probe, default selection, and Resolve/Info all behave. Skips
// when node or the runner aren't usable so CI without hypc stays green.
func TestRegistry_Integration(t *testing.T) {
	runner := localRunner(t)
	if _, err := os.Stat(runner); err != nil {
		t.Skipf("runner not present: %v", err)
	}

	// Gate on a working probe via the legacy single-build path, which must
	// reproduce the pre-multi-version behavior exactly.
	clearVerifierEnv(t)
	t.Setenv("HYPC_RUNNER", runner)
	t.Setenv("HYPC_BUILD_ID", localBuildID)
	reg, err := NewRegistry()
	if err != nil {
		t.Skipf("hypc runner not usable in this environment: %v", err)
	}

	if got := reg.SupportedBuildIDs(); len(got) != 1 || got[0] != localBuildID {
		t.Fatalf("legacy registry builds = %v", got)
	}
	if reg.Default().BuildID != localBuildID {
		t.Fatalf("legacy default = %q", reg.Default().BuildID)
	}
	// Empty selects default; exact id resolves; unknown id is rejected.
	if c, ok := reg.Resolve(""); !ok || c.BuildID != localBuildID {
		t.Fatalf("Resolve(\"\") = %v ok=%v", c, ok)
	}
	if _, ok := reg.Resolve("0.9.9+nope"); ok {
		t.Fatalf("Resolve(unknown) should be false")
	}
	info := reg.Info()
	if info.BuildID != localBuildID || info.Default != localBuildID || len(info.Compilers) != 1 {
		t.Fatalf("legacy Info() = %+v", info)
	}

	t.Run("skips bad build, default falls back to survivor", func(t *testing.T) {
		clearVerifierEnv(t)
		// First entry is the "preferred default" but reports the wrong id, so
		// it must be skipped; the default then falls back to the good build.
		t.Setenv("HYPC_COMPILERS", `[
			{"buildId":"0.0.0+bogus","runner":"`+runner+`","default":true},
			{"buildId":"`+localBuildID+`","runner":"`+runner+`"}
		]`)
		reg, err := NewRegistry()
		if err != nil {
			t.Fatalf("NewRegistry: %v", err)
		}
		if got := reg.SupportedBuildIDs(); len(got) != 1 || got[0] != localBuildID {
			t.Fatalf("expected only the good build, got %v", got)
		}
		if reg.Default().BuildID != localBuildID || !reg.Default().Default {
			t.Fatalf("default should fall back to the good build, got %+v", reg.Default())
		}
	})

	t.Run("duplicate build ids are de-duped", func(t *testing.T) {
		clearVerifierEnv(t)
		t.Setenv("HYPC_COMPILERS", `[
			{"buildId":"`+localBuildID+`","runner":"`+runner+`"},
			{"buildId":"`+localBuildID+`","runner":"`+runner+`"}
		]`)
		reg, err := NewRegistry()
		if err != nil {
			t.Fatalf("NewRegistry: %v", err)
		}
		if got := reg.SupportedBuildIDs(); len(got) != 1 {
			t.Fatalf("expected dedup to 1 build, got %v", got)
		}
	})
}
