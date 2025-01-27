# Transaction Endpoint Fix Analysis

## Current Issue
The `/txs` endpoint is returning `null` for transactions while still showing the correct total count and latest block. This indicates that the transactions are in the database but not being properly retrieved/decoded.

## Investigation

### 1. MongoDB Schema (Zond2mongoDB)
Looking at `Zond2mongoDB/db/transactions.go`, transactions are stored in MongoDB using the `TransactionByAddressCollection` function with this schema:

```go
doc := primitive.D{
    {Key: "txType", Value: txType},      // string
    {Key: "from", Value: from},          // string
    {Key: "to", Value: to},              // string
    {Key: "txHash", Value: hash},        // string
    {Key: "timeStamp", Value: timeStamp},// string
    {Key: "amount", Value: amount},      // float32
    {Key: "paidFees", Value: paidFees},  // float32
}
```

### 2. Current Model (backendAPI)
The current TransactionByAddress model in `backendAPI/models/transactionbyaddress.go`:

```go
type TransactionByAddress struct {
    ID          primitive.ObjectID `bson:"_id"`
    InOut       uint64             `json:"InOut"`
    TxType      uint32             `json:"TxType"`
    Address     string             `json:"Address" bson:"Address"`
    From        string             `json:"From" bson:"From"`
    To          string             `json:"To" bson:"To"`
    TxHash      string             `json:"TxHash" bson:"TxHash"`
    TimeStamp   uint64             `json:"TimeStamp"`
    Amount      float32            `json:"Amount"`
    PaidFees    float32            `json:"PaidFees"`
    BlockNumber uint64             `json:"BlockNumber"`
}
```

### 3. Implementation Issues

1. Field Type Mismatches:
   - TxType: stored as string in MongoDB but uint32 in model
   - TimeStamp: stored as string in MongoDB but uint64 in model
   - InOut: stored as int in MongoDB but uint64 in model
   - BlockNumber: not stored in MongoDB but expected in model

2. Field Name Case Mismatches:
   - MongoDB: lowercase field names (txType, from, to, txHash, timeStamp)
   - Model: uppercase field names (TxType, From, To, TxHash, TimeStamp)

3. Collection access:
   - Using direct collection variable instead of `configs.GetCollection(configs.DB, "transactions_by_address")`

## Required Changes

1. Update TransactionByAddress model in `backendAPI/models/transactionbyaddress.go`:
```go
type TransactionByAddress struct {
    ID          primitive.ObjectID `bson:"_id,omitempty"`
    InOut       int               `bson:"inOut" json:"InOut"`
    TxType      string            `bson:"txType" json:"TxType"`
    From        string            `bson:"from" json:"From"`
    To          string            `bson:"to" json:"To"`
    TxHash      string            `bson:"txHash" json:"TxHash"`
    TimeStamp   string            `bson:"timeStamp" json:"TimeStamp"`
    Amount      float32           `bson:"amount" json:"Amount"`
    PaidFees    float32           `bson:"paidFees" json:"PaidFees"`
}
```

2. Update ReturnTransactionsNetwork in `backendAPI/db/transaction.go`:
```go
func ReturnTransactionsNetwork(page int) ([]models.TransactionByAddress, error) {
    ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
    var transactions []models.TransactionByAddress
    defer cancel()

    limit := 5

    projection := primitive.D{
        {Key: "inOut", Value: 1},
        {Key: "txType", Value: 1},
        {Key: "from", Value: 1},
        {Key: "to", Value: 1},
        {Key: "txHash", Value: 1},
        {Key: "timeStamp", Value: 1},
        {Key: "amount", Value: 1},
        {Key: "paidFees", Value: 1},
    }

    opts := options.Find().
        SetProjection(projection).
        SetSort(primitive.D{{Key: "timeStamp", Value: -1}})

    if page == 0 {
        page = 1
    }
    opts.SetSkip(int64((page - 1) * limit))
    opts.SetLimit(int64(limit))

    results, err := configs.GetCollection(configs.DB, "transactions_by_address").Find(ctx, primitive.D{}, opts)
    if err != nil {
        return nil, fmt.Errorf("failed to query transactions: %v", err)
    }

    defer results.Close(ctx)
    for results.Next(ctx) {
        var singleTransaction models.TransactionByAddress
        if err = results.Decode(&singleTransaction); err != nil {
            return nil, fmt.Errorf("failed to decode transaction: %v", err)
        }
        transactions = append(transactions, singleTransaction)
    }

    return transactions, nil
}
```

## Migration Notes
1. No database migration needed - we're adapting our models to match existing data
2. The changes are backward compatible
3. Frontend already handles these fields correctly
4. We're removing BlockNumber since it's not stored in MongoDB

## Testing Plan
1. Test `/txs` endpoint with different page numbers
2. Verify transaction fields match the previous format
3. Check pagination works correctly
4. Verify sorting by timestamp is correct
5. Ensure frontend still works with the updated field types (string instead of uint)
