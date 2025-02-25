package rpc

import (
	"net/http"
)

type MyHTTPClient interface {
	Do(req *http.Request) (*http.Response, error)
}
