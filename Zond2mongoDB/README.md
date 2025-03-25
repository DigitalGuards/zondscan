# Zond2mongoDB

Proof of stake blockchain synchronizer that efficiently transfers QRL blockchain data to MongoDB.

## Overview

This is the Golang implementation of the QRL blockchain synchronizer to MongoDB. It serves as a critical component in the QRL Explorer ecosystem by continuously syncing blockchain data from a Zond node to MongoDB for efficient querying and data access.

## Recent Updates

### Transaction Fee Calculation
- Fixed transaction fee calculation to properly store paid fees in both the `transfer` and `transactionByAddress` collections
- Implemented fallback mechanisms when gas usage data is missing:
  1. First tries to get gas usage from transaction receipt
  2. Falls back to gas limit if receipt isn't available
  3. Ensures fees are never zero for successful transactions
- Improves accuracy of transaction cost data in the explorer

## Project Structure

```
├── bitfield/           # Bitfield operations and big number handling
│   ├── big.go
│   └── bitfield.go
│
├── configs/            # Configuration management
│   ├── const.go       # Constants definitions
│   ├── env.go         # Environment variables handling
│   └── setup.go       # Setup and initialization
│
├── contract/          # Smart contract related functionality
│   ├── call.go        # Contract call implementations
│   ├── config.json    # Contract configuration
│   └── contracts/     # Smart contract definitions
│       ├── ERC20.sol
│       ├── IERC20.sol
│       ├── MyToken.sol
│       └── MyVote.sol
│
├── db/               # Database operations and models
│   ├── bitfield.go
│   ├── blocks.go     # Block data handling
│   ├── blocksize.go  # Block size tracking
│   ├── circulating.go
│   ├── coinbase.go   # Coinbase transaction handling
│   ├── coingecko.go  # CoinGecko integration
│   ├── contracts.go  # Smart contract handling
│   ├── conversion.go # Data conversion utilities
│   ├── db.go        # Core database operations
│   ├── transactions.go
│   ├── validators.go # Validator data management
│   ├── volume.go    # Volume tracking
│   └── zond.go      # Zond node interaction
│
├── fetch/           # External data fetching
│   └── coingecko.go # CoinGecko API integration
│
├── logger/          # Logging functionality
│   └── logger.go
│
├── mock_rpc/        # Mock RPC for testing
│   └── myhttpclient_mock.go
│
├── models/          # Data models and structures
│   ├── bitfield.go
│   ├── block.go
│   ├── coingecko.go
│   ├── contract.go
│   ├── db.go
│   ├── trace.go
│   ├── transactions.go
│   ├── transfer.go
│   ├── validators.go
│   ├── wallet.go
│   └── zond.go
│
├── rpc/            # RPC client implementation
│   ├── calls.go    # RPC call definitions
│   └── client.go   # RPC client
│
├── services/       # Business logic layer
│   └── validator_service.go # Validator data processing and storage
│
└── synchroniser/   # Blockchain synchronization
    └── sync.go     # Core sync logic
```

## Setup

1. Create an `.env` file in the root directory with the following fields:

```env
MONGOURI=mongodb://localhost:27017
NODE_URL=http://localhost:8545
BEACONCHAIN_API=http://beaconnodehttpapi:3500
```

2. Build the application:
```bash
# On Unix-like systems
go build -o syncer main.go

# On Windows
go build -o syncer.exe main.go
```

3. Run the synchronizer:
```bash
# On Unix-like systems
./syncer

# On Windows
./syncer.exe
```

For production deployment, it's recommended to use a process manager like PM2:
```bash
# On Unix-like systems
pm2 start ./syncer --name "synchroniser"

# On Windows
pm2 start ./syncer.exe --name "synchroniser"
```

## Key Components

### Synchroniser
- Core component that manages the blockchain synchronization process
- Handles periodic updates of validators and contract data
- Maintains data consistency across node updates

### Validator System
- **Service Layer** (`services/validator_service.go`): 
  - Handles validator data processing and storage
  - Manages deduplication of validator records
  - Provides clean interfaces for validator operations
- **Data Storage**:
  - Stores all validators in a single document for efficient retrieval
  - Maintains validator status, epochs, and balances
  - Supports pagination for large validator sets
- **Beacon Chain Integration**:
  - Fetches validator data from the beacon chain API
  - Processes validator updates every epoch
  - Tracks validator activation and exit epochs

### Contract System
- **Enhanced Contract Detection**:
  - Automatically detects contract creation transactions
  - Identifies and indexes ERC20 tokens
  - Stores contract metadata and status
- **Contract Storage**:
  - Maintains contract addresses and creation information
  - Tracks contract status and verification
  - Stores token information for ERC20 contracts

### RPC Client
- Handles communication with the Zond node
- Manages beacon chain API interactions
- Provides robust error handling and retry mechanisms

### Database Layer
- Manages MongoDB operations and data persistence
- Implements efficient querying patterns
- Handles atomic updates for consistency

### Models
- Defines comprehensive data structures for blockchain entities
- Supports both legacy and new beacon chain formats
- Includes helper methods for data conversion

## Data Structures

### Validator Storage
```json
{
  "_id": "validators",
  "epoch": "123456",
  "validators": [
    {
      "index": "123",
      "publicKeyHex": "abcdef...",
      "effectiveBalance": "32000000000",
      "activationEpoch": "12346",
      "exitEpoch": "18446744073709551615",
      "withdrawableEpoch": "18446744073709551615",
      "slashed": false,
      "isLeader": true
    }
  ],
  "updatedAt": "1683924000"
}
```

### Contract Storage
```json
{
  "address": "abcdef...",
  "status": "1",
  "isToken": true,
  "name": "Token Name",
  "symbol": "TKN",
  "decimals": 18,
  "creatorAddress": "abcdef...",
  "creationTransaction": "abcdef...",
  "updatedAt": "1683924000"
}
```

### Data Format Notes
- Numeric values (epochs, balances, timestamps) are stored as decimal strings
- Dilithium public keys and addresses are stored in hex format WITHOUT "0x" prefix
- Timestamps are stored as Unix timestamps in decimal format
- Contract-related addresses and hashes are stored in hex format WITHOUT "0x" prefix

## License

This project is licensed under the MIT License - see the LICENSE file for details.
