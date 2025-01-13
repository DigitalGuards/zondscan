package rpc

import (
	"Zond2mongoDB/configs"
	L "Zond2mongoDB/logger"
	"net/http"
)

var logger = L.FileLogger(configs.Filename)

type MyHTTPClient interface {
	Do(req *http.Request) (*http.Response, error)
}
