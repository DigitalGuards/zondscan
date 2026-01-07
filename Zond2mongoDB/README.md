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
│   ├── token_detection.go  # ERC20 token detection via RPC
│   ├── tokenbalances.go    # Token holder balance tracking
│   ├── tokentransfers.go   # Token transfer event processing
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
    ├── sync.go         # Core sync logic
    └── pending_sync.go # Mempool transaction sync (every 5s)
```

## Setup

1. Create an `.env` file in the root directory with the following fields:

```env
MONGOURI=mongodb://localhost:27017
NODE_URL=http://localhost:8545
MEMPOOL_NODE_URL=http://localhost:8545  # Optional: separate endpoint for mempool detection
BEACONCHAIN_API=http://beaconnodehttpapi:3500
```

**Note:** `MEMPOOL_NODE_URL` is optional. If not set, it falls back to `NODE_URL`. This is useful when using a public RPC for block sync but a local node (with txpool access) for mempool detection.

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

### Token System
The synchronizer tracks ERC20 token activity across the network:

- **Token Detection** (`db/token_detection.go`):
  - Detects tokens by calling ERC20 methods (name, symbol, decimals)
  - Validates contracts implement the ERC20 interface
  - Fetches total supply for token contracts

- **Transfer Tracking** (`db/tokentransfers.go`):
  - Parses Transfer event logs from transaction receipts
  - Event signature: `0xddf252ad1be2c89b69c2b068fc378daa952ba7f163c4a11628f55a4df523b3ef`
  - Stores from/to addresses, amounts, and enriches with token metadata
  - Links transfers to their transaction hash and block

- **Balance Tracking** (`db/tokenbalances.go`):
  - Maintains real-time token balances for all holders
  - Updates balances on each transfer (increment recipient, decrement sender)
  - Supports querying all tokens held by an address

- **Collections**:
  - `tokenTransfers`: Individual transfer events with full metadata
  - `tokenBalances`: Current balance per holder per token contract

### Pending Transaction Sync
The synchronizer monitors the mempool for pending transactions:

- **Polling** (`synchroniser/pending_sync.go`):
  - Polls `txpool_content` every 5 seconds
  - Uses `MEMPOOL_NODE_URL` (falls back to `NODE_URL`)
  - Requires local node access (public RPCs typically block txpool)

- **Lifecycle**:
  - New transactions stored with `status: "pending"`
  - Transactions updated to `status: "mined"` when included in a block
  - Old pending transactions (>24h) are cleaned up

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

### Token Transfer Storage
```json
{
  "txHash": "0xabc123...",
  "blockNumber": "0x1a2b3c",
  "contractAddress": "0xtoken...",
  "from": "0xsender...",
  "to": "0xrecipient...",
  "amount": "1000000000000000000",
  "tokenName": "My Token",
  "tokenSymbol": "MTK",
  "tokenDecimals": 18,
  "timestamp": "1683924000"
}
```

### Token Balance Storage
```json
{
  "holderAddress": "0xholder...",
  "contractAddress": "0xtoken...",
  "balance": "5000000000000000000",
  "tokenName": "My Token",
  "tokenSymbol": "MTK",
  "tokenDecimals": 18,
  "lastUpdated": "2024-01-07T12:00:00Z"
}
```

### Pending Transaction Storage
```json
{
  "hash": "0xtxhash...",
  "from": "0xsender...",
  "to": "0xrecipient...",
  "value": "0x1bc16d674ec80000",
  "gas": "0x5208",
  "gasPrice": "0x3b9aca00",
  "nonce": "0x5",
  "input": "0x...",
  "status": "pending",
  "firstSeen": "2024-01-07T12:00:00Z"
}
```

### Data Format Notes
- Numeric values (epochs, balances, timestamps) are stored as decimal strings
- Dilithium public keys and addresses are stored in hex format WITHOUT "0x" prefix
- Timestamps are stored as Unix timestamps in decimal format
- Contract-related addresses and hashes are stored in hex format WITHOUT "0x" prefix

## License

This project is licensed under the MIT License - see the LICENSE file for details.
