package handler

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

func TestCORSMiddlewareRejectsDisallowedPOSTOriginBeforeHandler(t *testing.T) {
	gin.SetMode(gin.TestMode)
	called := false
	router := gin.New()
	router.Use(corsMiddleware([]string{"https://zondscan.com"}))
	router.POST("/cost", func(c *gin.Context) {
		called = true
		c.Status(http.StatusNoContent)
	})

	request := httptest.NewRequest(http.MethodPost, "/cost", nil)
	request.Header.Set("Origin", "https://attacker.example")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)

	if response.Code != http.StatusForbidden {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	if called {
		t.Fatal("disallowed cross-origin POST reached the route handler")
	}
}

func TestCORSMiddlewareAllowsTrustedBrowserAndOriginlessClients(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(corsMiddleware([]string{"https://zondscan.com"}))
	router.POST("/cost", func(c *gin.Context) { c.Status(http.StatusNoContent) })

	for _, test := range []struct {
		name   string
		origin string
		acao   string
	}{
		{name: "site", origin: "https://zondscan.com", acao: "https://zondscan.com"},
		{name: "extension", origin: "chrome-extension://install-id", acao: "chrome-extension://install-id"},
		{name: "server client"},
	} {
		t.Run(test.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodPost, "/cost", nil)
			if test.origin != "" {
				request.Header.Set("Origin", test.origin)
			}
			response := httptest.NewRecorder()
			router.ServeHTTP(response, request)
			if response.Code != http.StatusNoContent {
				t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
			}
			if got := response.Header().Get("Access-Control-Allow-Origin"); got != test.acao {
				t.Fatalf("Access-Control-Allow-Origin = %q, want %q", got, test.acao)
			}
		})
	}
}

func TestCORSMiddlewareRejectsDisallowedPOSTPreflight(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(corsMiddleware([]string{"https://zondscan.com"}))

	request := httptest.NewRequest(http.MethodOptions, "/cost", nil)
	request.Header.Set("Origin", "https://attacker.example")
	request.Header.Set("Access-Control-Request-Method", http.MethodPost)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)

	if response.Code != http.StatusForbidden {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
}

func TestHTTPServerBoundsHeaderBodyAndIdleReads(t *testing.T) {
	handler := http.HandlerFunc(func(http.ResponseWriter, *http.Request) {})
	server := newHTTPServer("127.0.0.1:0", handler)
	if server.Handler == nil || server.Addr != "127.0.0.1:0" {
		t.Fatalf("server = %#v", server)
	}
	if server.ReadHeaderTimeout != 5*time.Second ||
		server.ReadTimeout != 15*time.Second ||
		server.IdleTimeout != 60*time.Second {
		t.Fatalf("read timeouts = header %s, body %s, idle %s",
			server.ReadHeaderTimeout,
			server.ReadTimeout,
			server.IdleTimeout,
		)
	}
	if server.WriteTimeout < 60*time.Second {
		t.Fatalf("write timeout = %s, want at least 60s", server.WriteTimeout)
	}
	if server.MaxHeaderBytes != 1<<20 {
		t.Fatalf("MaxHeaderBytes = %d", server.MaxHeaderBytes)
	}
}
