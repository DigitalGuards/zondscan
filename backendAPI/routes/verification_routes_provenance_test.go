package routes

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"backendAPI/verification"

	"github.com/gin-gonic/gin"
)

func serveAlreadyVerifiedResult(t *testing.T, already bool, err error) *httptest.ResponseRecorder {
	t.Helper()
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/check", func(c *gin.Context) {
		if respondAlreadyVerified(c, "Qtest", already, err) {
			return
		}
		c.Status(http.StatusNoContent)
	})
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/check", nil))
	return recorder
}

func TestRespondAlreadyVerifiedRejectsInvalidStoredRecord(t *testing.T) {
	recorder := serveAlreadyVerifiedResult(t, false, verification.ErrInvalidStoredVerification)
	if recorder.Code != http.StatusConflict {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusConflict)
	}
	want := `{"error":"stored verification record is invalid; operator repair required"}`
	if got := recorder.Body.String(); got != want {
		t.Fatalf("body = %s, want %s", got, want)
	}
}

func TestRespondAlreadyVerifiedPreservesIdempotencyAndFailsClosedOnReadErrors(t *testing.T) {
	if recorder := serveAlreadyVerifiedResult(t, true, nil); recorder.Code != http.StatusOK {
		t.Fatalf("already-verified status = %d, want %d", recorder.Code, http.StatusOK)
	}
	if recorder := serveAlreadyVerifiedResult(t, false, errors.New("database unavailable")); recorder.Code != http.StatusInternalServerError {
		t.Fatalf("read-error status = %d, want %d", recorder.Code, http.StatusInternalServerError)
	}
}
