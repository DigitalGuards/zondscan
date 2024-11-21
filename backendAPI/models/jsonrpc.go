package models

type JsonRPC struct {
	Jsonrpc string        `json:"jsonrpc"`
	Method  string        `json:"method"`
	Params  []interface{} `json:"params"`
	ID      int           `json:"id"`
}

type Zond struct {
	Jsonrpc   string    `json:"jsonrpc"`
	ID        int       `json:"id"`
	ResultOld ResultOld `json:"result"`
}

type ZondUint64Version struct {
	Jsonrpc string `json:"jsonrpc"`
	ID      int    `json:"id"`
	Result  Result `json:"result"`
}


type Balance struct {
	Jsonrpc string `json:"jsonrpc"`
	ID      int    `json:"id"`
	Result  string `json:"result"`
	Error   Error  `json:"error"`
}

type Error struct {
    Code    int    `json:"code"`
    Message string `json:"message"`
}