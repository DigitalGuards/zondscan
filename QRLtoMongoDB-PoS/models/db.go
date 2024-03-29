package models

type TransactionByAddress struct {
	TxType    int     `bson:"txType"`
	TimeStamp int64   `bson:"timeStamp"`
	Amount    float32 `bson:"amount"`
}
