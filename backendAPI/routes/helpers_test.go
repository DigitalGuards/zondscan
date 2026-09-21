package routes

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"backendAPI/models"
	"backendAPI/sourcebundle"

	"github.com/gin-gonic/gin"
)

// The 400 bodies below are part of the public API surface: the frontend
// (and any third-party consumer) matches on them, so they are asserted
// byte-for-byte against the recorded response.

func newGuardRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)
	guardRouter := gin.New()
	guardRouter.GET("/addr/:address", func(c *gin.Context) {
		addr, ok := requireAddressParam(c, "address")
		if !ok {
			return
		}
		c.JSON(http.StatusOK, gin.H{"address": addr})
	})
	guardRouter.GET("/tx/:hash", func(c *gin.Context) {
		hash, ok := requireTxHashParam(c, "hash")
		if !ok {
			return
		}
		c.JSON(http.StatusOK, gin.H{"hash": hash})
	})
	return guardRouter
}

func TestContractInfoPayloadGatesStoredABIByCompilerProvenance(t *testing.T) {
	address := "Q" + strings.Repeat("a", 128)
	const (
		buildID        = "0.2.0-develop.2026.8.27+commit.6f862206.mod.Linux.g++"
		compilerSHA256 = "fe8e2344dbd902d6fc8c8cbb24114378c2de3a996b0d58f642d303c8bf30e930"
		nsjailSHA256   = "cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc"
		policySHA256   = "cc2c6d14e943c9b4b9252e69c2fbf9a5a4313938cd561594a255746393baaa8c"
	)
	valid := &models.CompilerProvenance{
		Schema:  models.CompilerProvenanceSchemaV2,
		Kind:    "native",
		BuildID: buildID,
		ExecutionDigest: models.NativeSandboxCompilerExecutionDigestV2(
			buildID,
			compilerSHA256,
			nsjailSHA256,
			policySHA256,
		),
		Components: []models.CompilerProvenanceComponent{
			{Name: "hypc", SHA256: compilerSHA256},
			{Name: "nsjail", SHA256: nsjailSHA256},
			{Name: "policy", SHA256: policySHA256},
		},
	}
	contract := models.ContractInfo{
		ContractAddress:          address,
		Verified:                 true,
		VerificationRecordSchema: models.VerificationRecordSchemaV1,
		ContractName:             "Main",
		SourceCode:               "contract Main {}",
		Abi:                      `[{"type":"function","name":"read"}]`,
		CompilerVersion:          buildID,
		CompilerProvenance:       valid,
	}
	contract.SourceBundleDigest = sourcebundle.Digest(contract.ContractName, contract.SourceCode, contract.Imports)

	t.Run("digest-backed includes ABI", func(t *testing.T) {
		compact := contract
		compact.ContractName = ""
		compact.SourceCode = ""
		compact.Imports = nil
		payload := contractInfoPayload(map[string]models.ContractInfo{address: compact}, address)
		if payload["provenanceStatus"] != models.CompilerProvenanceDigestBacked || payload["abi"] != contract.Abi {
			t.Fatalf("digest-backed payload = %#v", payload)
		}
	})

	for name, digest := range map[string]string{
		"missing source digest":   "",
		"malformed source digest": sourcebundle.Version + ":sha256:nope",
	} {
		t.Run(name+" suppresses ABI", func(t *testing.T) {
			invalid := contract
			invalid.SourceBundleDigest = digest
			payload := contractInfoPayload(map[string]models.ContractInfo{address: invalid}, address)
			if payload["provenanceStatus"] != models.CompilerProvenanceInvalidRecorded {
				t.Fatalf("invalid digest payload status = %#v", payload)
			}
			if _, ok := payload["abi"]; ok {
				t.Fatalf("invalid digest payload exposed ABI: %#v", payload)
			}
		})
	}

	t.Run("legacy includes ABI with explicit status", func(t *testing.T) {
		legacy := contract
		legacy.VerificationRecordSchema = ""
		legacy.CompilerProvenance = nil
		legacy.SourceBundleDigest = ""
		payload := contractInfoPayload(map[string]models.ContractInfo{address: legacy}, address)
		if payload["provenanceStatus"] != models.CompilerProvenanceLegacyUnrecorded || payload["abi"] != contract.Abi {
			t.Fatalf("legacy payload = %#v", payload)
		}
	})

	t.Run("mixed legacy and current fields suppress ABI", func(t *testing.T) {
		mixed := contract
		mixed.CompilerProvenance = nil
		payload := contractInfoPayload(map[string]models.ContractInfo{address: mixed}, address)
		if payload["provenanceStatus"] != models.CompilerProvenanceInvalidRecorded {
			t.Fatalf("mixed payload status = %#v", payload)
		}
		if _, ok := payload["abi"]; ok {
			t.Fatalf("mixed payload exposed ABI: %#v", payload)
		}
	})

	t.Run("invalid suppresses ABI", func(t *testing.T) {
		invalid := contract
		broken := *valid
		broken.ExecutionDigest = strings.Repeat("c", 64)
		invalid.CompilerProvenance = &broken
		payload := contractInfoPayload(map[string]models.ContractInfo{address: invalid}, address)
		if payload["provenanceStatus"] != models.CompilerProvenanceInvalidRecorded {
			t.Fatalf("invalid payload status = %#v", payload)
		}
		if _, ok := payload["abi"]; ok {
			t.Fatalf("invalid payload exposed ABI: %#v", payload)
		}
	})

	t.Run("unverified includes neither status nor ABI", func(t *testing.T) {
		unverified := contract
		unverified.Verified = false
		payload := contractInfoPayload(map[string]models.ContractInfo{address: unverified}, address)
		if _, ok := payload["provenanceStatus"]; ok {
			t.Fatalf("unverified payload exposed provenance status: %#v", payload)
		}
		if _, ok := payload["abi"]; ok {
			t.Fatalf("unverified payload exposed ABI: %#v", payload)
		}
	})
}

func serve(t *testing.T, guardRouter *gin.Engine, path string) *httptest.ResponseRecorder {
	t.Helper()
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	guardRouter.ServeHTTP(w, req)
	return w
}

func TestRequireAddressParam(t *testing.T) {
	guardRouter := newGuardRouter()

	t.Run("invalid address emits exact 400 body", func(t *testing.T) {
		cases := []string{
			"/addr/not-an-address",
			"/addr/" + repeat("a", 128),       // bare hex, missing prefix
			"/addr/Q" + repeat("a", 127),      // one char short
			"/addr/0x" + repeat("a", 64),      // tx hash, not an address
			"/addr/" + "Q" + repeat("z", 128), // non-hex body
		}
		want := `{"error":"invalid address; expected Q or 0x followed by 128 hex chars"}`
		for _, path := range cases {
			w := serve(t, guardRouter, path)
			if w.Code != http.StatusBadRequest {
				t.Errorf("%s: status = %d, want %d", path, w.Code, http.StatusBadRequest)
			}
			if got := w.Body.String(); got != want {
				t.Errorf("%s: body = %s, want %s", path, got, want)
			}
		}
	})

	t.Run("valid aliases return a canonical response address", func(t *testing.T) {
		for _, addr := range []string{
			"Q" + repeat("a", 128),
			"q" + repeat("A", 128),
			"0x" + repeat("1", 128),
			"0X" + repeat("a", 128),
			qip55ChecksumA,
		} {
			w := serve(t, guardRouter, "/addr/"+addr)
			if w.Code != http.StatusOK {
				t.Errorf("%s: status = %d, want %d", addr, w.Code, http.StatusOK)
			}
			wantAddress := qip55ChecksumA
			if strings.Contains(addr, "1") {
				wantAddress = "Q" + repeat("1", 128)
			}
			want := `{"address":"` + wantAddress + `"}`
			if got := w.Body.String(); got != want {
				t.Errorf("%s: body = %s, want %s", addr, got, want)
			}
		}
	})
}

func TestGetBalanceRejectsInvalidAddresses(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.POST("/getBalance", handleGetBalance)
	want := `{"error":"invalid address; expected Q or 0x followed by 128 hex chars"}`

	for _, address := range []string{
		"Q" + repeat("a", 127),
		"Q" + repeat("a", 129),
		"Q" + repeat("Ab", 64),
		"not-an-address",
	} {
		form := url.Values{"address": {address}}
		req := httptest.NewRequest(http.MethodPost, "/getBalance", strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		if w.Code != http.StatusBadRequest {
			t.Errorf("%s: status = %d, want %d", address, w.Code, http.StatusBadRequest)
		}
		if got := w.Body.String(); got != want {
			t.Errorf("%s: body = %s, want %s", address, got, want)
		}
	}
}

// parseStandardFilter derives its message from the allowed set instead of
// keeping one literal per route, so pin both historical wordings here.
func TestParseStandardFilterErrorBodies(t *testing.T) {
	gin.SetMode(gin.TestMode)
	guardRouter := gin.New()
	guardRouter.GET("/three", func(c *gin.Context) {
		if _, ok := parseStandardFilter(c, "ERC-20", "ERC-721", "ERC-1155"); !ok {
			return
		}
		c.Status(http.StatusOK)
	})
	guardRouter.GET("/two", func(c *gin.Context) {
		if _, ok := parseStandardFilter(c, "ERC-721", "ERC-1155"); !ok {
			return
		}
		c.Status(http.StatusOK)
	})

	cases := []struct {
		path string
		want string
	}{
		{"/three?standard=bogus", `{"error":"invalid standard; expected one of ERC-20, ERC-721, ERC-1155"}`},
		{"/two?standard=ERC-20", `{"error":"invalid standard; expected ERC-721 or ERC-1155"}`},
	}
	for _, tc := range cases {
		w := serve(t, guardRouter, tc.path)
		if w.Code != http.StatusBadRequest {
			t.Errorf("%s: status = %d, want %d", tc.path, w.Code, http.StatusBadRequest)
		}
		if got := w.Body.String(); got != tc.want {
			t.Errorf("%s: body = %s, want %s", tc.path, got, tc.want)
		}
	}
	// Absent and valid params both pass through.
	for _, path := range []string{"/three", "/three?standard=ERC-20", "/two?standard=ERC-1155"} {
		if w := serve(t, guardRouter, path); w.Code != http.StatusOK {
			t.Errorf("%s: status = %d, want %d", path, w.Code, http.StatusOK)
		}
	}
}

func TestRequireTxHashParam(t *testing.T) {
	guardRouter := newGuardRouter()

	t.Run("invalid hash emits exact 400 body", func(t *testing.T) {
		cases := []string{
			"/tx/nothash",
			"/tx/" + repeat("a", 64),   // missing 0x prefix
			"/tx/0x" + repeat("a", 63), // one char short
			"/tx/0x" + repeat("a", 65), // one char long
			"/tx/Q" + repeat("a", 128), // address, not a hash
		}
		want := `{"error":"invalid transaction hash; expected 0x + 64 hex chars"}`
		for _, path := range cases {
			w := serve(t, guardRouter, path)
			if w.Code != http.StatusBadRequest {
				t.Errorf("%s: status = %d, want %d", path, w.Code, http.StatusBadRequest)
			}
			if got := w.Body.String(); got != want {
				t.Errorf("%s: body = %s, want %s", path, got, want)
			}
		}
	})

	t.Run("valid hash passes through unmodified", func(t *testing.T) {
		hash := "0x" + repeat("Ab", 32)
		w := serve(t, guardRouter, "/tx/"+hash)
		if w.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
		}
		want := `{"hash":"` + hash + `"}`
		if got := w.Body.String(); got != want {
			t.Errorf("body = %s, want %s", got, want)
		}
	})
}
