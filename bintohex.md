# Binary to Hex Storage Migration Strategy

## Current State
1. **Data Flow**:
   - Input: Hex strings from blockchain (e.g., "0x123...")
   - Storage: Convert to binary ([]byte) and store in MongoDB
   - MongoDB Display: Shows as base64
   - Retrieval: Convert base64 back to binary, then to hex string

2. **Issues**:
   - Multiple conversions: hex -> binary -> base64 -> binary -> hex
   - Redundant storage (e.g., From/FromStr fields)
   - Base64 representation in MongoDB makes debugging harder
   - Extra processing overhead for each read/write

## Migration Plan

### Phase 1: Model Updates
1. **Zond2mongoDB**:
   ```go
   // Change Transfer model fields from []byte to string
   type Transfer struct {
       From            string    `json:"from"`     // Store original hex
       To              string    `json:"to"`       // Store original hex
       TxHash          string    `json:"txHash"`   // Store original hex
       Pk              string    `json:"pk"`       // Store original hex
       Signature       string    `json:"signature"`// Store original hex
       // Remove redundant FromStr/ToStr fields
   }
   ```

2. **BackendAPI**:
   ```go
   // Update models to match Zond2mongoDB changes
   type Transfer struct {
       From            string    `json:"from" bson:"from"`
       To              string    `json:"to" bson:"to"`
       TxHash          string    `json:"txHash" bson:"txHash"`
       // ...other fields
   }
   ```

### Additional Collections to Update

1. **Addresses Collection**:
   ```go
   // Current
   type Address struct {
       ObjectId primitive.ObjectID `bson:"_id"`
       ID       []byte            `json:"id"`      // Currently binary
       Balance  float32           `json:"balance"`
       Nonce    uint64           `json:"nonce"`
   }
   
   // Change to
   type Address struct {
       ObjectId primitive.ObjectID `bson:"_id"`
       ID       string            `json:"id"`      // Store as hex
       Balance  float32           `json:"balance"`
       Nonce    uint64           `json:"nonce"`
   }
   ```

2. **Contracts Collection**:
   ```go
   // Current
   type ContractInfo struct {
       ContractCreatorAddress []byte `json:"contractCreatorAddress" bson:"contractCreatorAddress"`
       ContractAddress        []byte `json:"contractAddress" bson:"contractAddress"`
       ContractCode           []byte `json:"contractCode" bson:"contractCode"`
       // ... other fields
   }
   
   // Change to
   type ContractInfo struct {
       ContractCreatorAddress string `json:"contractCreatorAddress" bson:"contractCreatorAddress"`
       ContractAddress        string `json:"contractAddress" bson:"contractAddress"`
       ContractCode           string `json:"contractCode" bson:"contractCode"`
       // ... other fields
   }
   ```

3. **TransactionByAddress Collection**:
   ```go
   // Current
   type TransactionByAddress struct {
       ID          primitive.ObjectID `bson:"_id"`
       Address     []byte            `json:"Address"`
       From        []byte            `json:"From"`
       To          []byte            `json:"To"`
       TxHash      []byte            `json:"TxHash"`
       // ... other fields
   }
   
   // Change to
   type TransactionByAddress struct {
       ID          primitive.ObjectID `bson:"_id"`
       Address     string            `json:"Address"`
       From        string            `json:"From"`
       To          string            `json:"To"`
       TxHash      string            `json:"TxHash"`
       // ... other fields
   }
   ```

### Phase 2: Code Updates
1. **Zond2mongoDB Changes**:
   - Remove hex.DecodeString calls in processTransactionData
   - Store original hex strings directly
   - Update all database queries to work with hex strings

2. **BackendAPI Changes**:
   - Update queries to work with hex strings
   - Remove binary/hex conversion logic
   - Update response formatting if needed

### Phase 3: Data Migration
Update migration script to handle additional collections:
1. Transfer collection
2. Addresses collection
3. Contracts collection
4. TransactionByAddress collection

Each collection will need:
- Read existing documents
- Convert binary fields to hex strings
- Update documents with new format
- Verify data integrity

### Phase 4: Testing
1. **Test Cases**:
   - Data integrity after migration
   - Query performance
   - API responses
   - Frontend compatibility

2. **Performance Testing**:
   - Database size comparison
   - Query speed comparison
   - API response times

## Benefits
1. Simpler code (no conversions)
2. Better debugging (readable values in MongoDB)
3. Reduced storage (no duplicate fields)
4. Faster processing (no conversions)
5. Direct compatibility with blockchain format

## Risks and Mitigation
1. **Data Loss**:
   - Thorough testing before production
   - Keep backups
   - Run in staging first

2. **Performance**:
   - Benchmark before and after
   - Monitor database size
   - Test with large datasets

3. **Downtime**:
   - Plan migration during low-traffic period
   - Consider rolling update strategy
