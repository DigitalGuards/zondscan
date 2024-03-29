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
