package models

type JsonRPC struct {
	Jsonrpc string        `json:"jsonrpc"`
	Method  string        `json:"method"`
	Params  []interface{} `json:"params"`
	ID      int           `json:"id"`
}

type ZondUint64Version struct {
	Jsonrpc string `json:"jsonrpc"`
	ID      int    `json:"id"`
	Result  Result `json:"result"`
}
