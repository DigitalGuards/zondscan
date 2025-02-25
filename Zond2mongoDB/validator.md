# Validator System

## Overview
The validator system tracks active validators in the Zond network. It consists of three main components:
1. **Zond2mongoDB**: Fetches and stores validator data from the blockchain
2. **Backend API**: Serves validator data to the frontend
3. **Frontend**: Displays validator information to users

## Architecture

### Service Layer (`services/validator_service.go`)
- Handles validator data processing and storage
- Manages deduplication of validator records
- Provides clean interfaces for validator operations:
  - `StoreValidators`: Stores and deduplicates validator data
  - `GetValidators`: Retrieves all validators
  - `GetValidatorByPublicKey`: Fetches specific validator details

### Database Layer (`db/validators.go`)
- Manages low-level database operations
- Handles validator document updates
- Provides query interfaces for validator retrieval

### RPC Layer (`rpc/calls.go`)
- Fetches validator data from beacon chain API
- Handles pagination of validator requests
- Manages API rate limiting and error handling

## Data Flow

### 1. Data Fetching
```
Beacon Chain API
      ↓
RPC Layer (GetValidators)
      ↓
Service Layer (StoreValidators)
      ↓
MongoDB Storage
```

### 2. Data Retrieval
```
MongoDB Storage
      ↓
Service Layer (GetValidators)
      ↓
Backend API
      ↓
Frontend Display
```

## Data Structures

### MongoDB Storage Format
```json
{
  "_id": "validators",
  "epoch": "123456",
  "validators": [
    {
      "index": "123",
      "publicKeyHex": "abcdef...",
      "withdrawalCredentialsHex": "...",
      "effectiveBalance": "32000000000",
      "slashed": false,
      "activationEligibilityEpoch": "12345",
      "activationEpoch": "12346",
      "exitEpoch": "18446744073709551615",
      "withdrawableEpoch": "18446744073709551615",
      "slotNumber": "123",
      "isLeader": true
    }
  ],
  "updatedAt": "1683924000"
}
```

### API Response Format
```json
{
  "validators": [
    {
      "address": "abcdef...",
      "uptime": 100.0,
      "age": 1234,
      "stakedAmount": "32000000000",
      "isActive": true
    }
  ],
  "totalStaked": "3200000000000",
  "epoch": "123456"
}
```

## Data Format Notes

### Numeric Values
- All numeric values (epochs, balances, timestamps) are stored as decimal strings
- This makes them easier to process and display without constant conversion
- Maintains precision for large numbers without floating-point issues

### Public Keys
- Dilithium public keys are stored in hex format WITHOUT "0x" prefix
- This allows direct use by the frontend for address conversion
- Withdrawal credentials also stored in hex format WITHOUT "0x" prefix

### Timestamps
- All timestamps are stored as Unix timestamps in decimal format
- Makes it easier to perform date/time calculations and display

## Epoch Calculation
- Each epoch consists of 128 slots
- Each slot takes 60 seconds (1 minute)
- Each epoch is 128 * 60 = 7,680 seconds (2.13 hours)
- Current epoch calculation: `blockNumber / 128`

## Validator States

### Active State Calculation
```go
isActive := activationEpoch <= currentEpoch && currentEpoch < exitEpoch
```

### Age Calculation
```go
age := currentEpoch - activationEpoch  // if activationEpoch <= currentEpoch
```

## Synchronization

### Initial Sync
1. Fetches current epoch from latest block
2. Retrieves all validators in pages (250 per page)
3. Stores validators with deduplication
4. Updates epoch and timestamp

### Periodic Updates
- Runs every 30 minutes via synchronizer
- Only updates if epoch has changed
- Maintains existing validator data
- Adds new validators as they join
- Updates validator states and balances

## Error Handling

### RPC Errors
- Retries on temporary failures
- Logs detailed error information
- Maintains last known good state

### Storage Errors
- Atomic updates for consistency
- Rollback on partial failures
- Logs validation errors

## Monitoring

### Key Metrics
- Total validator count
- Active validator count
- Total staked amount
- Sync status and health

### Logging
- Validator updates
- Sync progress
- Error conditions
- Performance metrics

## API Endpoints

### GET /validators
- Returns list of validators with details
- Supports pagination
- Includes total staked amount
- Calculates validator status

### GET /validators/count
- Returns total and active validator counts
- Used for dashboard metrics
- Fast response for basic stats

## Best Practices

### Data Consistency
- Use single document for all validators
- Implement proper deduplication
- Maintain atomic updates
- Validate data before storage

### Performance
- Index on publicKeyHex
- Batch updates where possible
- Cache frequently accessed data
- Use projection in queries

### Security
- Validate input data
- Sanitize API responses
- Rate limit API requests
- Handle sensitive data properly

## Staking
- Each validator stakes 40,000 QUANTA (shown in Wei: 40000000000000000000000)
- Total staked amount is calculated by multiplying individual stake by number of validators

## Future Improvements
1. Calculate actual validator uptime from historical data (currently hardcoded to 100%)
2. Track validator performance metrics
3. Implement slashing detection
4. Add validator rewards tracking

TODO: implement frontend decoding of validator public key to address using wallet.js 