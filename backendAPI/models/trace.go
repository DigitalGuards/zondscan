package models

import "go.mongodb.org/mongo-driver/bson/primitive"

type TraceResult struct {
	ID                        primitive.ObjectID `bson:"_id"`
	Type                      []byte             `json:"Type"`
	CallType                  []byte             `json:"CallType"`
	Hash                      []byte             `json:"Hash"`
	From                      []byte             `json:"From"`
	Gas                       uint64             `json:"Gas"`
	GasUsed                   uint64             `json:"GasUsed"`
	To                        []byte             `json:"To"`
	Input                     uint64             `json:"Input"`
	Output                    uint64             `json:"Output"`
	Calls                     []Call             `json:"Calls"`
	Value                     float32            `json:"Value"`
	TraceAddress              []int              `json:"TraceAddress"`
	InOut                     uint64             `json:"InOut"`
	Address                   []byte             `json:"Address"`
	AddressFunctionIdentifier []byte             `json:"AddressFunctionIdentifier"`
	AmountFunctionIdentifier  uint64             `json:"AmountFunctionIdentifier"`
	BlockTimeStamp            uint64             `json:"BlockTimestamp"`
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

// InternalTx mirrors a `internalTransactionByAddress` document as the
// syncer actually writes it (db/transactions.go:InternalTransactionByAddressCollection):
// the strings are all stored as canonical Go strings, despite the legacy
// TraceResult above declaring its fields as []byte. We use this struct
// for the /tx/:hash response so BSON decoding survives the round trip.
//
// Internal txs are the per-call entries the EVM produced under one
// outer transaction; opcode-level CALL / DELEGATECALL / STATICCALL,
// surfaced in the tx page so users can see what the contract actually
// did beyond top-level Transfer events.
type InternalTx struct {
	Type                      string `json:"type" bson:"type"`
	CallType                  string `json:"callType" bson:"callType"`
	Hash                      string `json:"hash" bson:"hash"`
	From                      string `json:"from" bson:"from"`
	To                        string `json:"to" bson:"to"`
	Input                     string `json:"input" bson:"input"`
	Output                    string `json:"output" bson:"output"`
	TraceAddress              []int  `json:"traceAddress" bson:"traceAddress"`
	// Value lives as float64 in mongo (legacy schema choice). For the API
	// we re-stringify in the route handler so the frontend can format it
	// consistently with everything else.
	Value                     float64 `json:"value" bson:"value"`
	Gas                       string  `json:"gas" bson:"gas"`
	GasUsed                   string  `json:"gasUsed" bson:"gasUsed"`
	AddressFunctionIdentifier string  `json:"addressFunctionIdentifier" bson:"addressFunctionIdentifier"`
	AmountFunctionIdentifier  string  `json:"amountFunctionIdentifier" bson:"amountFunctionIdentifier"`
	BlockTimestamp            string  `json:"blockTimestamp" bson:"blockTimestamp"`
}
