package verification

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// localBuildID preserves the historical npm compiler identity. The runtime
// is intentionally unavailable until every executable and JS/WASM component
// that can affect output has complete provenance.
const localBuildID = "0.0.2+commit.3e18e55d.Emscripten.clang"

// clearVerifierEnv blanks every env var loadCompilerSpecs consults so each
// case starts from a known state (t.Setenv restores originals on cleanup).
func clearVerifierEnv(t *testing.T) {
	t.Helper()
	for _, key := range []string{
		"HYPC_COMPILERS",
		"HYPC_COMPILERS_SHA256",
		"HYPC_RUNNER",
		"HYPC_BUILD_ID",
		"HYPC_NODE_BIN",
		"HYPC_BIN",
		"HYPC_SHA256",
		"VERIFIER_MAX_CONCURRENCY",
		"VERIFIER_COMPILE_TIMEOUT",
		"VERIFIER_SOURCE_MAX_BYTES",
		"VERIFIER_COMPILER_MAX_STDOUT_BYTES",
		"VERIFIER_COMPILER_MAX_STDERR_BYTES",
		"VERIFIER_SANDBOX_BIN",
		"VERIFIER_SANDBOX_SHA256",
	} {
		t.Setenv(key, "")
	}
}

func compilerManifestSHA256(data []byte) string {
	digest := sha256.Sum256(data)
	return hex.EncodeToString(digest[:])
}

func setFileCompilerManifest(t *testing.T, path string, data []byte) {
	t.Helper()
	t.Setenv("HYPC_COMPILERS", path)
	t.Setenv("HYPC_COMPILERS_SHA256", compilerManifestSHA256(data))
}

func TestLoadCompilerSpecsInlineArray(t *testing.T) {
	clearVerifierEnv(t)
	t.Setenv("HYPC_COMPILERS", `[
		{"kind":"native","buildId":"current","bin":"/opt/hypc-current","sha256":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","default":true},
		{"kind":"native","buildId":"historical","bin":"/opt/hypc-historical","sha256":"bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"},
		{"kind":"npm","buildId":"`+localBuildID+`","disabled":true}
	]`)

	specs, err := loadCompilerSpecs()
	if err != nil {
		t.Fatalf("load inline manifest: %v", err)
	}
	if len(specs) != 3 {
		t.Fatalf("compiler count = %d, want 3", len(specs))
	}
	if specs[0].Kind != CompilerKindNative || specs[0].BuildID != "current" || !specs[0].Default {
		t.Fatalf("current spec = %+v", specs[0])
	}
	if specs[1].SHA256 != strings.Repeat("b", 64) || specs[1].Default {
		t.Fatalf("historical spec = %+v", specs[1])
	}
	if specs[2].Kind != CompilerKindNPM || !specs[2].Disabled {
		t.Fatalf("disabled npm spec = %+v", specs[2])
	}
}

func TestLoadCompilerSpecsSecureFile(t *testing.T) {
	clearVerifierEnv(t)
	path := filepath.Join(t.TempDir(), "compilers.json")
	manifest := `[{"kind":"native","buildId":"current","bin":"/opt/hypc","sha256":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","default":true}]`
	if err := os.WriteFile(path, []byte(manifest), 0o600); err != nil {
		t.Fatalf("write compiler manifest: %v", err)
	}
	setFileCompilerManifest(t, path, []byte(manifest))

	specs, err := loadCompilerSpecs()
	if err != nil {
		t.Fatalf("load compiler manifest: %v", err)
	}
	if len(specs) != 1 || specs[0].BuildID != "current" {
		t.Fatalf("compiler specs = %+v", specs)
	}
}

func TestLoadCompilerSpecsRejectsUnsafeManifestFile(t *testing.T) {
	manifest := []byte(`[{"kind":"native","buildId":"current","bin":"/opt/hypc","sha256":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","default":true}]`)

	t.Run("relative path", func(t *testing.T) {
		clearVerifierEnv(t)
		setFileCompilerManifest(t, "relative-compilers.json", manifest)
		if _, err := loadCompilerSpecs(); err == nil || !strings.Contains(err.Error(), "absolute") {
			t.Fatalf("loadCompilerSpecs error = %v, want absolute-path rejection", err)
		}
	})

	t.Run("symlink", func(t *testing.T) {
		clearVerifierEnv(t)
		dir := t.TempDir()
		target := filepath.Join(dir, "target.json")
		link := filepath.Join(dir, "compilers.json")
		if err := os.WriteFile(target, manifest, 0o600); err != nil {
			t.Fatalf("write target manifest: %v", err)
		}
		if err := os.Symlink(target, link); err != nil {
			t.Fatalf("create manifest symlink: %v", err)
		}
		setFileCompilerManifest(t, link, manifest)
		if _, err := loadCompilerSpecs(); err == nil || !strings.Contains(err.Error(), "symlink") {
			t.Fatalf("loadCompilerSpecs error = %v, want symlink rejection", err)
		}
	})

	for _, mode := range []os.FileMode{0o620, 0o602} {
		mode := mode
		t.Run("writable mode "+mode.String(), func(t *testing.T) {
			clearVerifierEnv(t)
			path := filepath.Join(t.TempDir(), "compilers.json")
			if err := os.WriteFile(path, manifest, 0o600); err != nil {
				t.Fatalf("write compiler manifest: %v", err)
			}
			if err := os.Chmod(path, mode); err != nil {
				t.Fatalf("chmod compiler manifest: %v", err)
			}
			setFileCompilerManifest(t, path, manifest)
			if _, err := loadCompilerSpecs(); err == nil || !strings.Contains(err.Error(), "group or world writable") {
				t.Fatalf("loadCompilerSpecs error = %v, want writable-file rejection", err)
			}
		})
	}

	t.Run("directory", func(t *testing.T) {
		clearVerifierEnv(t)
		setFileCompilerManifest(t, t.TempDir(), manifest)
		if _, err := loadCompilerSpecs(); err == nil || !strings.Contains(err.Error(), "regular file") {
			t.Fatalf("loadCompilerSpecs error = %v, want regular-file rejection", err)
		}
	})
}

func TestLoadCompilerSpecsRequiresTrustedFileManifestDigest(t *testing.T) {
	manifest := []byte(`[{
		"kind":"native",
		"buildId":"current",
		"bin":"/opt/hypc",
		"sha256":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		"default":true
	}]`)
	writeManifest := func(t *testing.T) string {
		t.Helper()
		path := filepath.Join(t.TempDir(), "compilers.json")
		if err := os.WriteFile(path, manifest, 0o600); err != nil {
			t.Fatalf("write compiler manifest: %v", err)
		}
		return path
	}

	t.Run("missing digest", func(t *testing.T) {
		clearVerifierEnv(t)
		t.Setenv("HYPC_COMPILERS", writeManifest(t))
		if _, err := loadCompilerSpecs(); err == nil || !strings.Contains(err.Error(), "HYPC_COMPILERS_SHA256 is required") {
			t.Fatalf("loadCompilerSpecs error = %v, want missing manifest digest rejection", err)
		}
	})

	t.Run("malformed digest", func(t *testing.T) {
		clearVerifierEnv(t)
		t.Setenv("HYPC_COMPILERS", writeManifest(t))
		t.Setenv("HYPC_COMPILERS_SHA256", "not-a-digest")
		if _, err := loadCompilerSpecs(); err == nil || !strings.Contains(err.Error(), "exactly 64 hexadecimal") {
			t.Fatalf("loadCompilerSpecs error = %v, want malformed manifest digest rejection", err)
		}
	})

	t.Run("digest mismatch", func(t *testing.T) {
		clearVerifierEnv(t)
		t.Setenv("HYPC_COMPILERS", writeManifest(t))
		t.Setenv("HYPC_COMPILERS_SHA256", strings.Repeat("0", 64))
		if _, err := loadCompilerSpecs(); err == nil || !strings.Contains(err.Error(), "manifest SHA-256 mismatch") {
			t.Fatalf("loadCompilerSpecs error = %v, want manifest digest mismatch", err)
		}
	})

	t.Run("replacement bytes", func(t *testing.T) {
		clearVerifierEnv(t)
		path := writeManifest(t)
		t.Setenv("HYPC_COMPILERS", path)
		t.Setenv("HYPC_COMPILERS_SHA256", compilerManifestSHA256(manifest))
		if err := os.WriteFile(path, append(append([]byte(nil), manifest...), '\n'), 0o600); err != nil {
			t.Fatalf("replace compiler manifest bytes: %v", err)
		}
		if _, err := loadCompilerSpecs(); err == nil || !strings.Contains(err.Error(), "manifest SHA-256 mismatch") {
			t.Fatalf("loadCompilerSpecs error = %v, want replaced manifest rejection", err)
		}
	})

	t.Run("inline rejects unrelated digest", func(t *testing.T) {
		clearVerifierEnv(t)
		t.Setenv("HYPC_COMPILERS", string(manifest))
		t.Setenv("HYPC_COMPILERS_SHA256", compilerManifestSHA256(manifest))
		if _, err := loadCompilerSpecs(); err == nil || !strings.Contains(err.Error(), "only valid with a file-backed") {
			t.Fatalf("loadCompilerSpecs error = %v, want inline digest rejection", err)
		}
	})

	t.Run("digest without manifest", func(t *testing.T) {
		clearVerifierEnv(t)
		t.Setenv("HYPC_COMPILERS_SHA256", compilerManifestSHA256(manifest))
		if _, err := loadCompilerSpecs(); err == nil || !strings.Contains(err.Error(), "requires a file-backed") {
			t.Fatalf("loadCompilerSpecs error = %v, want orphaned digest rejection", err)
		}
	})
}

func TestLoadCompilerSpecsLegacyNativeEnv(t *testing.T) {
	clearVerifierEnv(t)
	t.Setenv("HYPC_RUNNER", "/legacy/runner.sh")
	t.Setenv("HYPC_BUILD_ID", "legacy-native")
	t.Setenv("HYPC_NODE_BIN", "/usr/bin/node")
	t.Setenv("HYPC_BIN", "/opt/hypc")
	t.Setenv("HYPC_SHA256", strings.Repeat("b", 64))

	specs, err := loadCompilerSpecs()
	if err != nil {
		t.Fatalf("load legacy native compiler: %v", err)
	}
	if len(specs) != 1 {
		t.Fatalf("compiler count = %d, want 1", len(specs))
	}
	got := specs[0]
	if got.Kind != CompilerKindNative || got.BuildID != "legacy-native" || got.Bin != "/opt/hypc" ||
		got.SHA256 != strings.Repeat("b", 64) || got.Runner != "" || got.NodeBin != "" || !got.Default {
		t.Fatalf("legacy native spec = %+v", got)
	}
}

func TestLoadCompilerSpecsLegacyNPMFailsClosed(t *testing.T) {
	clearVerifierEnv(t)
	t.Setenv("HYPC_BUILD_ID", localBuildID)
	t.Setenv("HYPC_RUNNER", "/opt/hypc-runner.js")
	t.Setenv("HYPC_NODE_BIN", "/usr/bin/node")

	specs, err := loadCompilerSpecs()
	if err != nil {
		t.Fatalf("load legacy npm compiler: %v", err)
	}
	if len(specs) != 1 || specs[0].Kind != CompilerKindNPM {
		t.Fatalf("legacy npm specs = %+v", specs)
	}
	if _, err := NewRegistry(); err == nil || !strings.Contains(err.Error(), "complete runtime provenance") {
		t.Fatalf("NewRegistry error = %v, want npm provenance rejection", err)
	}
}

func TestLoadCompilerSpecsUnconfigured(t *testing.T) {
	clearVerifierEnv(t)
	specs, err := loadCompilerSpecs()
	if err != nil {
		t.Fatalf("load unconfigured compiler specs: %v", err)
	}
	if len(specs) != 0 {
		t.Fatalf("compiler specs = %+v, want none", specs)
	}
}

func TestLoadCompilerSpecsRejectsMalformedOrAmbiguousManifest(t *testing.T) {
	cases := map[string]string{
		"missing kind":    `[{"buildId":"a","default":true}]`,
		"missing buildId": `[{"kind":"native","default":true}]`,
		"missing default": `[{"kind":"native","buildId":"a"}]`,
		"disabled default": `[
			{"kind":"native","buildId":"a","disabled":true,"default":true}
		]`,
		"two defaults": `[
			{"kind":"native","buildId":"a","default":true},
			{"kind":"native","buildId":"b","default":true}
		]`,
		"duplicate buildId": `[
			{"kind":"native","buildId":"a","default":true},
			{"kind":"native","buildId":"a"}
		]`,
		"duplicate object member": `[
			{"kind":"native","kind":"npm","buildId":"a","default":true}
		]`,
		"case-insensitive duplicate object member": `[
			{"kind":"native","Kind":"npm","buildId":"a","default":true}
		]`,
		"unicode-folded duplicate object member": `[
			{"kind":"native","buildId":"a","sha256":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","\u017fha256":"bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb","default":true}
		]`,
		"unknown field": `[
			{"kind":"native","buildId":"a","sha265":"typo","default":true}
		]`,
		"trailing data": `[
			{"kind":"native","buildId":"a","default":true}
		] true`,
	}
	for name, manifest := range cases {
		t.Run(name, func(t *testing.T) {
			clearVerifierEnv(t)
			t.Setenv("HYPC_COMPILERS", manifest)
			if _, err := loadCompilerSpecs(); err == nil {
				t.Fatalf("expected manifest rejection for %q", name)
			}
		})
	}
}
