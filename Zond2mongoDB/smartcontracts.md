# Smart Contracts and Tokens

## Overview
The system provides comprehensive support for smart contracts and tokens on the Zond network, including detection, storage, and interaction capabilities. This document outlines the key components and processes involved in handling smart contracts and tokens.

## Architecture

### Components
1. **Zond2mongoDB**
   - Detects contract creation transactions
   - Processes contract code and metadata
   - Identifies token contracts (ERC20)
   - Stores contract data in MongoDB

2. **Backend API**
   - Serves contract and token information
   - Provides search and filtering capabilities
   - Handles contract verification
   - Manages token-specific operations

3. **Frontend**
   - Displays contract information
   - Shows token details and balances
   - Supports contract interaction
   - Provides search functionality

## Data Structures

### Contract Storage Model
```go
type ContractInfo struct {
    ContractCreatorAddress []byte // Creator's address
    ContractAddress        []byte // Contract's address
    ContractCode          []byte // Contract bytecode
    TokenName     string        // If it's a token
    TokenSymbol   string        // Token symbol
    TokenDecimals uint8         // Token decimals
    IsToken       bool          // Whether it's a token contract
}
```

### API Response Format
```typescript
interface ContractData {
    _id: string;
    contractCreatorAddress: string;
    contractAddress: string;
    tokenName?: string;
    tokenSymbol?: string;
    tokenDecimals?: number;
    isToken: boolean;
}
```

## Contract Detection and Processing

### Contract Creation Detection
1. Transaction monitoring:
   - Checks for transactions with empty `to` field
   - Verifies contract creation status
   - Extracts contract address from receipt

2. Code verification:
   - Retrieves contract bytecode
   - Validates code presence
   - Stores contract metadata

### Token Detection
1. ERC20 Interface checking:
   - Attempts to call standard ERC20 methods:
     - name()
     - symbol()
     - decimals()
   - Marks contract as token if methods exist

2. Token information retrieval:
```go
// Get token symbol
func GetTokenSymbol(contractAddress string) (string, error) {
    result, err := CallContractMethod(contractAddress, SIG_SYMBOL)
    // Process and return symbol
}

// Get token decimals
func GetTokenDecimals(contractAddress string) (uint8, error) {
    result, err := CallContractMethod(contractAddress, SIG_DECIMALS)
    // Process and return decimals
}
```

## Database Operations

### Contract Storage
```go
// Store new contract
func StoreContract(contractAddress string, status string, isToken bool, 
    name string, symbol string, decimals uint8) error {
    // Create contract document
    // Store in MongoDB
}

// Retrieve contract
func GetContract(address string) (*models.ContractInfo, error) {
    // Fetch from MongoDB
    // Return contract info
}

// Update contract status
func UpdateContractStatus(address string, status string) error {
    // Update status in MongoDB
}
```

### Search and Filtering
```go
func ReturnContracts(page int64, limit int64, search string) ([]models.ContractInfo, int64, error) {
    // Base filter for contracts
    filter := bson.D{}

    // Add search if provided
    if search != "" {
        searchBytes, _ := hex.DecodeString(search)
        filter = bson.D{
            {Key: "$or", Value: bson.A{
                bson.D{{Key: "contractAddress", Value: searchBytes}},
                bson.D{{Key: "contractCreatorAddress", Value: searchBytes}},
            }},
        }
    }
    // Return filtered results with pagination
}
```

## API Endpoints

### GET /contracts
- Returns list of contracts with pagination
- Supports search functionality
- Parameters:
  - page: Page number (default: 0)
  - limit: Items per page (default: 10)
  - search: Search query for addresses

### GET /address/aggregate/:query
- Returns detailed contract information
- Includes:
  - Contract metadata
  - Token information (if applicable)
  - Creator information
  - Transaction history

## Error Handling

### Contract Operations
- Timeout handling (10-second context timeout)
- Validation of addresses
- Contract creation verification
- Token detection fallbacks

### API Responses
- Proper error status codes
- Detailed error messages
- Fallback values for missing data
- Validation of input parameters

## Best Practices

### Contract Interaction
1. Always verify contract existence before operations
2. Use timeouts for RPC calls
3. Validate contract addresses
4. Handle token-specific edge cases

### Data Storage
1. Store binary data as bytes
2. Index contract addresses
3. Maintain contract status
4. Track contract creation details

### Security
1. Validate input addresses
2. Sanitize contract data
3. Rate limit API requests
4. Handle sensitive information properly

## Future Improvements

### Planned Features
1. Contract verification support
2. Advanced token standards (ERC721, ERC1155)
3. Contract interaction interface
4. Token transfer tracking

### Optimization Opportunities
1. Caching frequently accessed contracts
2. Batch processing of token operations
3. Enhanced search capabilities
4. Real-time contract updates

## Monitoring and Maintenance

### Key Metrics
1. Contract creation rate
2. Token contract percentage
3. Contract interaction frequency
4. API endpoint usage

### Health Checks
1. Contract detection accuracy
2. Token information completeness
3. Database performance
4. API response times

## Troubleshooting

### Common Issues
1. Contract creation failure
   - Check transaction status
   - Verify bytecode
   - Validate creation parameters

2. Token detection issues
   - Verify ERC20 interface
   - Check method signatures
   - Validate token data

3. API response problems
   - Check request parameters
   - Verify database connection
   - Validate contract existence

### Debug Tools
1. Contract verification tools
2. Token interface checkers
3. RPC call debuggers
4. Database query analyzers 