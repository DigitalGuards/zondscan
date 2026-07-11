package routes

import (
	"net/http"
	"net/http/httptest"
	"testing"

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
			"/addr/" + repeat("a", 40),       // bare hex, missing prefix
			"/addr/Q" + repeat("a", 39),      // one char short
			"/addr/0x" + repeat("a", 64),     // tx hash, not an address
			"/addr/" + "Q" + repeat("z", 40), // non-hex body
		}
		want := `{"error":"invalid address; expected Q or 0x followed by 40 hex chars"}`
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

	t.Run("valid address passes through unmodified", func(t *testing.T) {
		for _, addr := range []string{
			"Q" + repeat("a", 40),
			"q" + repeat("A", 40),
			"0x" + repeat("1", 40),
		} {
			w := serve(t, guardRouter, "/addr/"+addr)
			if w.Code != http.StatusOK {
				t.Errorf("%s: status = %d, want %d", addr, w.Code, http.StatusOK)
			}
			want := `{"address":"` + addr + `"}`
			if got := w.Body.String(); got != want {
				t.Errorf("%s: body = %s, want %s", addr, got, want)
			}
		}
	})
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
			"/tx/Q" + repeat("a", 40),  // address, not a hash
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
