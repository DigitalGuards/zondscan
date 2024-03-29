package models

type TraceResponse struct {
	JsonRPC string      `json:"jsonrpc"`
	ID      int         `json:"id"`
	Result  TraceResult `json:"result"`
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
	TraceAddress []int  `json:"traceAddress"`
}

type Call struct {
	From    string `json:"from"`
	Gas     string `json:"gas"`
	GasUsed string `json:"gasUsed"`
	To      string `json:"to"`
	Input   string `json:"input"`
	Value   string `json:"value"`
	Type    string `json:"type"`
}
