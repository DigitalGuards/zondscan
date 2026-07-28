package models

type TraceResponse struct {
	JsonRPC string      `json:"jsonrpc"`
	ID      int         `json:"id"`
	Result  TraceResult `json:"result"`
	// Error is the JSON-RPC level error (e.g. "historical state not
	// available in path scheme yet" on non-archive nodes). Without this
	// field a failed trace unmarshals into a zero Result and reads as "no
	// internal calls", which is silent data loss.
	Error *TraceRPCError `json:"error"`
}

type TraceRPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type TraceResult struct {
	Type         string `json:"type"`
	CallType     string `json:"callType"`
	Hash         string `json:"Hash"`
	From         string `json:"from"`
	Gas          string `json:"gas"`
	GasUsed      string `json:"gasUsed"`
	To           string `json:"to"`
	Input        string `json:"input"`
	Output       string `json:"output"`
	Calls        []Call `json:"calls"`
	Value        string `json:"value"`
	Error        string `json:"error"`
	TraceAddress []int  `json:"traceAddress"`
}

// Call is one frame of the callTracer call tree. Frames nest recursively
// via Calls; a non-empty Error marks the frame (and its whole subtree)
// as reverted.
type Call struct {
	From    string `json:"from"`
	Gas     string `json:"gas"`
	GasUsed string `json:"gasUsed"`
	To      string `json:"to"`
	Input   string `json:"input"`
	Output  string `json:"output"`
	Value   string `json:"value"`
	Type    string `json:"type"`
	Error   string `json:"error"`
	Calls   []Call `json:"calls"`
}
