package routes

import (
	"backendAPI/db"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestAddressAggregateCacheKeyCanonicalizesAliases(t *testing.T) {
	body := strings.Repeat("a", 128)
	want := "addr:Q" + body + ":2:25"
	for _, address := range []string{
		"Q" + body,
		"q" + body,
		"0x" + body,
		"0X" + body,
		qip55ChecksumA,
	} {
		if got := addressAggregateCacheKey(address, 2, 25); got != want {
			t.Errorf("addressAggregateCacheKey(%q) = %q, want %q", address, got, want)
		}
	}
}

func TestRespondAddressAggregateErrorUsesRetryable503ForStaleBalance(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)

	respondAddressAggregateError(ctx, errors.Join(errors.New("wrapped"), db.ErrStaleIndexedBalance))

	if recorder.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusServiceUnavailable)
	}
	if got := recorder.Header().Get("Retry-After"); got != "5" {
		t.Fatalf("Retry-After = %q, want 5", got)
	}
}
