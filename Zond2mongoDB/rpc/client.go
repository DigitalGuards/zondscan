package rpc

import (
	"net/http"
	"time"
)

// Package-level HTTP client with connection pooling and timeouts
var httpClient = &http.Client{
	Timeout: 30 * time.Second,
	Transport: &http.Transport{
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: 100,
		IdleConnTimeout:     90 * time.Second,
		DisableCompression:  true,
	},
}

// GetHTTPClient returns the package-level HTTP client
func GetHTTPClient() *http.Client {
	return httpClient
}

type MyHTTPClient interface {
	Do(req *http.Request) (*http.Response, error)
}
