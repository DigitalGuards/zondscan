# Quanta Explorer Backend API

The backend API component of the Quanta Explorer application, written in Go. This server provides API endpoints for the frontend to fetch blockchain data and interact with the QRL Zond network.

## Project Structure

```
backendAPI/
├── configs/           # Configuration management
│   ├── const.go      # Constants and configuration values
│   ├── env.go        # Environment variable handling
│   └── setup.go      # Application setup and initialization
├── db/               # Database operations
│   ├── address.go    # Address and wallet operations
│   ├── block.go      # Block-related operations
│   ├── contract.go   # Smart contract operations
│   ├── db.go         # Database package declaration
│   ├── db_test.go    # Database tests
│   ├── pending.go    # Pending transaction operations
│   ├── stats.go      # Statistics and utility functions
│   ├── token.go      # Token balance and transfer queries
│   ├── transaction.go # Transaction operations
│   └── validator.go  # Validator operations
├── handler/          # Request handlers
│   └── handler.go    # HTTP request handlers
├── models/           # Data models
│   ├── address.go    # Address-related structures
│   ├── blocksize.go  # Block size tracking
│   ├── coinbase.go   # Coinbase transaction models
│   ├── coingecko.go  # CoinGecko price data models
│   ├── contract.go   # Smart contract models
│   ├── jsonrpc.go    # JSON-RPC request/response structures
│   ├── trace.go      # Transaction trace models
│   ├── transactionbyaddress.go  # Address transaction models
│   ├── transfer.go   # Token transfer models
│   ├── validator.go  # Validator data models
│   ├── wallet.go     # Wallet-related structures
│   └── zond.go       # Zond-specific models
├── resources/        # Static resources
│   └── favicon.ico
├── routes/           # API route definitions
│   └── routes.go     # Route handlers and middleware
├── main.go          # Application entry point
├── go.mod           # Go module definition
└── go.sum           # Go module checksums
```

## Key Components

### Configs
Handles all configuration-related functionality including:
- Environment variable management
- Constants and configuration values
- Application setup and initialization

### Database (db/)
Manages database operations and interactions, organized into logical modules:
- address.go: Handles all address and wallet-related database operations
- block.go: Manages block-related queries and operations
- contract.go: Handles smart contract interactions and queries
- pending.go: Manages pending transaction operations
- stats.go: Provides statistics and utility functions
- transaction.go: Handles all transaction-related operations
- validator.go: Manages validator-related queries
- db_test.go: Contains database operation tests

### Handlers (handler/)
Contains HTTP request handlers that process incoming requests and return appropriate responses.

### Models (models/)
Defines data structures for:
- Blockchain addresses
- Block information
- Transactions
- Smart contracts
- Validators
- Wallet data
- Price data from CoinGecko
- JSON-RPC communication

### Routes (routes/)
Defines API endpoints and their corresponding handlers:
- Route registration
- Middleware configuration
- Request routing

## Environment Configuration

The API uses two environment files:
- `.env.development` for development environment
- `.env.production` for production environment

### Development Environment Variables
| VARIABLE | VALUE |
| ------ | ------ |
| GIN_MODE | release |
| MONGOURI | mongodb://localhost:27017/qrldata-z?readPreference=primary |
| HTTP_PORT | :8080 |
| NODE_URL | http://localhost:8545 |

## Getting Started

1. Ensure Go is installed on your system
2. Clone the repository
3. Navigate to the backendAPI directory
4. Install dependencies:
   ```bash
   go mod download
   ```
5. Set up environment files:
   ```bash
   touch .env.development .env.production
   ```
6. Build the application:
   ```bash
   # On Unix-like systems
   go build -o backendAPI main.go

   # On Windows
   go build -o backendAPI.exe main.go
   ```

For production deployment, use PM2:
```bash
# On Unix-like systems
pm2 start ./backendAPI --name "handler"

# On Windows
pm2 start ./backendAPI.exe --name "handler"
```

## Development

- Follow Go best practices and conventions
- Ensure proper error handling
- Write tests for new functionality
- Update documentation when adding new features

## API Documentation

### Overview & Statistics
| Endpoint | Method | Description |
|----------|--------|-------------|
| `/overview` | GET | Network statistics (market cap, price, wallet count, circulating supply, validators, contracts) |
| `/latestblock` | GET | Current block height |
| `/debug/blocks` | GET | Debug endpoint showing total blocks and latest block |

### Blocks
| Endpoint | Method | Description |
|----------|--------|-------------|
| `/blocks` | GET | Paginated block list. Query: `page`, `limit` |
| `/block/:query` | GET | Single block by number (decimal or 0x hex) |
| `/blocksizes` | GET | Historical block size data for charts |

### Transactions
| Endpoint | Method | Description |
|----------|--------|-------------|
| `/txs` | GET | Paginated network transactions. Query: `page` |
| `/tx/:query` | GET | Transaction details by hash. Includes `tokenTransfer` if ERC20, `contractCreated` if deployment |
| `/transactions` | GET | Latest transactions (limited) |
| `/coinbase/:query` | GET | Coinbase transaction details |

### Pending Transactions
| Endpoint | Method | Description |
|----------|--------|-------------|
| `/pending-transactions` | GET | Paginated mempool transactions. Query: `page`, `limit` |
| `/pending-transaction/:hash` | GET | Single pending transaction. Returns 404 if mined or not found |

### Addresses
| Endpoint | Method | Description |
|----------|--------|-------------|
| `/address/aggregate/:query` | GET | Full address data (balance, rank, transactions, internal txs, contract code) |
| `/address/:address/transactions` | GET | Paginated address transactions. Query: `page`, `limit` |
| `/address/:address/tokens` | GET | Token balances held by address (for wallet integration) |
| `/getBalance` | POST | Get address balance. Form: `address` |
| `/richlist` | GET | Top addresses by balance |
| `/walletdistribution/:query` | GET | Wallet distribution statistics |

### Tokens (ERC20)
| Endpoint | Method | Description |
|----------|--------|-------------|
| `/token/:address/info` | GET | Token metadata (name, symbol, decimals, total supply, holder count) |
| `/token/:address/holders` | GET | Paginated token holders. Query: `page`, `limit` (max 100) |
| `/token/:address/transfers` | GET | Paginated token transfer history. Query: `page`, `limit` (max 100) |

### Contracts
| Endpoint | Method | Description |
|----------|--------|-------------|
| `/contracts` | GET | Paginated contracts. Query: `page`, `limit`, `search`, `isToken` (optional filter) |

### Validators
| Endpoint | Method | Description |
|----------|--------|-------------|
| `/validators` | GET | Paginated validator list. Query: `page_token` |
| `/validator/:id` | GET | Individual validator by index or public key |
| `/validators/stats` | GET | Validator statistics (total, active, slashed) |
| `/validators/history` | GET | Historical validator counts. Query: `limit` (default 100) |
| `/epoch` | GET | Current epoch information |

### Response Format

All endpoints return JSON. Paginated endpoints include:
```json
{
  "data": [...],
  "total": 1234,
  "page": 1,
  "limit": 10
}
```

Error responses:
```json
{
  "error": "Error message"
}
```
