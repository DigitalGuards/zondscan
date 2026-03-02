# ZondScan Backend API Documentation

Comprehensive documentation for the ZondScan backend API server -- a Go + Gin REST API that serves blockchain data for the QRL Zond network explorer at zondscan.com.

## Table of Contents

1. [Architecture Overview](#architecture-overview)
2. [REST API Endpoints](#rest-api-endpoints)
3. [Database Collections & Query Patterns](#database-collections--query-patterns)
4. [Data Models](#data-models)
5. [Configuration](#configuration)
6. [Build & Run](#build--run)
7. [Key Implementation Details](#key-implementation-details)

---

## Architecture Overview

```
                                    ┌──────────────────────────┐
                                    │   QRL Zond Node (RPC)    │
                                    │   http://localhost:8545   │
                                    └────────────┬─────────────┘
                                                 │
                                    (zond_getBalance calls)
                                                 │
┌──────────────┐    REST API     ┌───────────────▼──────────────┐     Reads      ┌───────────────┐
│  Frontend    │◀───────────────▶│     backendAPI (Go + Gin)    │◀──────────────▶│   MongoDB     │
│  Next.js     │   :8080/:8081   │                              │                │  qrldata-z    │
└──────────────┘                 │  handler/ - Middleware/CORS  │                └───────────────┘
                                 │  routes/  - Endpoint routing │
                                 │  db/      - Query functions  │
                                 │  models/  - Data structures  │
                                 │  configs/ - DB + env setup   │
                                 └──────────────────────────────┘
```

### Component Responsibilities

| Directory | Purpose |
|-----------|---------|
| `main.go` | Entry point. Sets up logging (stdout + `backendAPI.log`), panic recovery, and calls `handler.RequestHandler()`. |
| `configs/` | MongoDB connection (`setup.go`), collection references and constants (`const.go`), environment variable loading (`env.go`). |
| `handler/` | Gin router initialization, CORS config, middleware (panic recovery, request latency logging), TLS/HTTP mode selection. |
| `routes/` | All REST endpoint definitions and request/response handling. |
| `db/` | Database query functions organized by entity: addresses, blocks, transactions, contracts, tokens, validators, stats, pending transactions. |
| `models/` | Go struct definitions for all data entities. |

### Request Flow

1. HTTP request arrives at Gin router
2. Passes through middleware: Logger -> Recovery -> Monitor -> CORS
3. Matched route handler in `routes/routes.go`
4. Handler calls one or more `db/` functions
5. `db/` functions query MongoDB collections (configured in `configs/const.go`)
6. Results decoded into `models/` structs
7. JSON response returned to client

---

## REST API Endpoints

### Health & Overview

#### `GET /health`
Kubernetes health check probe.

**Response:**
```json
{ "status": "ok" }
```

#### `GET /overview`
Dashboard overview with market data and network stats.

**Response:**
```json
{
  "marketcap": 1000000000000000000,
  "currentPrice": 1000.0,
  "countwallets": 1500,
  "circulating": "65000000",
  "volume": 42,
  "tradingVolume": 50000.0,
  "validatorCount": 128,
  "contractCount": 35,
  "status": {
    "syncing": true,
    "dataInitialized": true
  }
}
```

**Notes:**
- `circulating` defaults to `"65000000"` when unavailable.
- `status.dataInitialized` is `true` if any non-zero data exists.
- All numeric fields default to `0` when data is unavailable.

---

### Price Data

#### `GET /price-history?interval=24h`
Historical price data for charts and wallet apps.

**Query Parameters:**
| Param | Default | Valid Values |
|-------|---------|-------------|
| `interval` | `24h` | `4h`, `12h`, `24h`, `7d`, `30d`, `all` |

**Response:**
```json
{
  "data": [
    {
      "timestamp": "2026-02-24T10:00:00Z",
      "priceUSD": 1.23,
      "marketCapUSD": 80000000,
      "volumeUSD": 50000
    }
  ],
  "interval": "24h",
  "count": 48
}
```

**Interval to data point mapping:**
| Interval | Max Points | Approximate Granularity |
|----------|-----------|------------------------|
| `4h` | 8 | ~30 min |
| `12h` | 24 | ~30 min |
| `24h` | 48 | ~30 min |
| `7d` | 336 | ~30 min |
| `30d` | 1440 | ~30 min |
| `all` | unlimited | all stored data |

---

### Blocks

#### `GET /blocks?page=1&limit=5`
Paginated list of latest blocks.

**Query Parameters:**
| Param | Default | Description |
|-------|---------|-------------|
| `page` | `1` | Page number (1-indexed) |
| `limit` | `5` | Blocks per page |

**Response:**
```json
{
  "blocks": [
    {
      "baseFeePerGas": "0x3b9aca00",
      "gasLimit": "0x1c9c380",
      "gasUsed": "0x5208",
      "hash": "0xabc...",
      "number": "0x1a4",
      "timestamp": "0x65abc123",
      "transactions": [...]
    }
  ],
  "total": 1500
}
```

**Notes:**
- Total is capped at `300 * limit` pages maximum.
- Sorted by timestamp descending (newest first).
- Projection includes: `number`, `timestamp`, `hash`, `transactions`.

#### `GET /block/:query`
Single block by number. Accepts both decimal (`420`) and hex (`0x1a4`) formats.

**Response:**
```json
{
  "block": {
    "jsonrpc": "2.0",
    "id": 1,
    "result": {
      "baseFeePerGas": "0x3b9aca00",
      "gasLimit": "0x1c9c380",
      "hash": "0xabc...",
      "number": "0x1a4",
      "timestamp": "0x65abc123",
      "transactions": [...],
      "withdrawals": [...]
    }
  }
}
```

**Notes:** Falls back to zero-padded hex (`0x01a4`) if initial lookup fails.

#### `GET /latestblock`
Returns the latest synced block number (from `sync_state` collection).

**Response:**
```json
{ "blockNumber": 420 }
```

#### `GET /blocksizes`
Returns historical average block size data for charts.

**Response:**
```json
{
  "response": [
    { "timestamp": "...", "size": 1234 }
  ]
}
```

#### `GET /debug/blocks`
Debug endpoint returning total block count and latest block number.

**Response:**
```json
{
  "total_blocks": 1500,
  "latest_block": 420
}
```

---

### Transactions

#### `GET /txs?page=1`
Paginated network-wide transactions list.

**Query Parameters:**
| Param | Required | Description |
|-------|----------|-------------|
| `page` | Yes | Page number (1-indexed) |

**Response:**
```json
{
  "txs": [
    {
      "InOut": 0,
      "TxType": "transfer",
      "From": "Z2019ea...",
      "To": "Z5a330c...",
      "TxHash": "0xabc...",
      "TimeStamp": "1706123456",
      "Amount": "1.500000000000000000",
      "PaidFees": "0.000021000000000000",
      "BlockNumber": "420"
    }
  ],
  "total": 5000,
  "latestBlock": 420
}
```

**Notes:**
- Fixed page size of 5 transactions per page.
- Amount and PaidFees are serialized with 18 decimal places.
- BlockNumber is converted from hex to decimal in JSON output.

#### `GET /tx/:query`
Single transaction by hash.

**Response:**
```json
{
  "response": {
    "blockNumber": "0x1a4",
    "blockTimestamp": "0x65abc123",
    "from": "Z2019ea...",
    "to": "Z5a330c...",
    "txHash": "0xabc...",
    "value": "0x1bc16d674ec80000",
    "gasUsed": "0x5208",
    "gasPrice": "0x3b9aca00",
    "nonce": "0x0",
    "signature": "...",
    "pk": "..."
  },
  "latestBlock": 420,
  "contractCreated": {
    "address": "Z5a330c...",
    "isToken": true,
    "name": "MyToken",
    "symbol": "MTK",
    "decimals": 18
  },
  "tokenTransfer": {
    "contractAddress": "Z5a330c...",
    "from": "Z2019ea...",
    "to": "Zaaabbb...",
    "amount": "1000000000000000000",
    "tokenName": "MyToken",
    "tokenSymbol": "MTK",
    "tokenDecimals": 18
  }
}
```

**Notes:**
- `contractCreated` is included only if the transaction deployed a contract.
- `tokenTransfer` is included only if the transaction is an ERC20 transfer.
- First looks up the transaction in the `blocks` collection (by matching `result.transactions[].hash`), then falls back to the `transfer` collection.

#### `GET /transactions`
Returns all latest transactions (no pagination -- full scan).

**Response:**
```json
{
  "response": [
    {
      "InOut": 0,
      "TxType": "transfer",
      "TxHash": "0xabc...",
      "TimeStamp": "1706123456",
      "Amount": "1.500000000000000000",
      "PaidFees": "0.000021000000000000"
    }
  ]
}
```

#### `GET /coinbase/:query`
Coinbase transaction lookup (uses the same `ReturnSingleTransfer` function as `/tx`).

**Response:**
```json
{
  "response": { ... }
}
```

---

### Addresses

#### `GET /address/aggregate/:query`
Aggregated address data: balance, rank, transactions, internal transactions, and contract code.

**Path Parameter:** Address with `Z` prefix (e.g., `Z2019ea08f4e24201b98f9154906da4b924a04892`)

**Response:**
```json
{
  "address": {
    "id": "z2019ea08f4e24201b98f9154906da4b924a04892",
    "balance": 100.5,
    "nonce": 42
  },
  "transactions_count": 150,
  "rank": 5,
  "transactions_by_address": [
    {
      "InOut": 0,
      "TxType": "transfer",
      "From": "Z2019ea...",
      "To": "Z5a330c...",
      "TxHash": "0xabc...",
      "TimeStamp": "1706123456",
      "Amount": "1.500000000000000000",
      "PaidFees": "0.000021000000000000",
      "BlockNumber": "420"
    }
  ],
  "internal_transactions_by_address": [...],
  "contract_code": {
    "address": "Z2019ea...",
    "creatorAddress": "Z5a330c...",
    "contractCode": "0x606060...",
    "isToken": false
  },
  "latestBlock": 420
}
```

**Notes:**
- Normalizes `z` prefix to `Z` on input.
- If the address is not found in the `addresses` collection, it queries the Zond node RPC (`zond_getBalance`) and creates a new entry.
- Transaction queries use case-insensitive regex matching.
- Internal transactions query the `internalTransactionByAddress` collection using hex-decoded byte-level matching.

#### `GET /address/:address/transactions?page=1&limit=5`
Paginated non-zero-amount transactions for an address.

**Query Parameters:**
| Param | Default | Description |
|-------|---------|-------------|
| `page` | `1` | Page number |
| `limit` | `5` | Results per page |

**Response:**
```json
{
  "transactions": [...],
  "total": 50,
  "page": 1,
  "limit": 5
}
```

**Notes:** Filters to only transactions where `amount > 0`.

#### `GET /address/:address/tokens`
All ERC20 token balances held by an address. Designed for wallet integration (e.g., qrlwallet auto-discovery).

**Response:**
```json
{
  "address": "Z2019ea...",
  "tokens": [
    {
      "contractAddress": "Z5a330c...",
      "holderAddress": "Z2019ea...",
      "balance": "1000000000000000000",
      "blockNumber": "0x1a4",
      "name": "MyToken",
      "symbol": "MTK",
      "decimals": 18
    }
  ],
  "count": 1
}
```

**Notes:**
- Uses MongoDB aggregation pipeline with `$lookup` to join `tokenBalances` with `contractCode` for metadata.
- Sorted by balance descending (highest value tokens first).
- Searches both `Z` and `z` prefix variants of the address.

#### `POST /getBalance`
Get balance for an address directly from the Zond node RPC.

**Request Body (form-encoded):**
```
address=Z2019ea08f4e24201b98f9154906da4b924a04892
```

**Response:**
```json
{ "balance": 100.5 }
```

**Notes:** Makes a live `zond_getBalance` RPC call to the Zond node. Balance is returned in QRL (divided by 1e18 from wei).

#### `GET /walletdistribution/:query`
Count wallets with balance greater than the specified threshold (in units of 1e12 wei).

**Example:** `/walletdistribution/1000` counts wallets with balance > 1000 * 1e12 wei.

**Response:**
```json
{ "response": 42 }
```

#### `GET /richlist`
Top 50 addresses by balance.

**Response:**
```json
{
  "richlist": [
    { "id": "z2019ea...", "balance": 1000000.5, "nonce": 100 }
  ]
}
```

---

### Contracts

#### `GET /contracts?page=0&limit=10&search=&isToken=true`
Paginated list of deployed smart contracts.

**Query Parameters:**
| Param | Default | Description |
|-------|---------|-------------|
| `page` | `0` | Page number (0-indexed) |
| `limit` | `10` | Results per page |
| `search` | (none) | Search by contract address, creator address, or token name |
| `isToken` | (none) | Filter: `true` for tokens only, `false` for non-tokens only |

**Response:**
```json
{
  "response": [
    {
      "address": "Z5a330c...",
      "creatorAddress": "Z2019ea...",
      "contractCode": "0x606060...",
      "creationTransaction": "0xabc...",
      "creationBlockNumber": "0x1a4",
      "isToken": true,
      "name": "MyToken",
      "symbol": "MTK",
      "decimals": 18,
      "totalSupply": "1000000000000000000000",
      "status": "verified"
    }
  ],
  "total": 35
}
```

**Notes:**
- Search is case-insensitive.
- Addresses in the response are normalized to uppercase `Z` prefix.
- Sorted by `_id` descending (latest first).

---

### Tokens

#### `GET /token/:address/info`
Summary information about a specific ERC20 token.

**Response:**
```json
{
  "contractAddress": "Z5a330c...",
  "name": "MyToken",
  "symbol": "MTK",
  "decimals": 18,
  "totalSupply": "1000000000000000000000",
  "holderCount": 42,
  "transferCount": 150,
  "creatorAddress": "Z2019ea...",
  "creationTxHash": "0xabc...",
  "creationBlock": "0x1a4"
}
```

#### `GET /token/:address/holders?page=0&limit=25`
Paginated token holder list sorted by balance descending.

**Query Parameters:**
| Param | Default | Max |
|-------|---------|-----|
| `page` | `0` | -- |
| `limit` | `25` | `100` |

**Response:**
```json
{
  "contractAddress": "Z5a330c...",
  "holders": [
    {
      "contractAddress": "Z5a330c...",
      "holderAddress": "Z2019ea...",
      "balance": "500000000000000000000"
    }
  ],
  "totalHolders": 42,
  "page": 0,
  "limit": 25
}
```

**Notes:** Uses aggregation pipeline with `$toDecimal` for proper numeric sorting of string balances.

#### `GET /token/:address/transfers?page=0&limit=25`
Paginated token transfer history.

**Query Parameters:**
| Param | Default | Max |
|-------|---------|-----|
| `page` | `0` | -- |
| `limit` | `25` | `100` |

**Response:**
```json
{
  "contractAddress": "Z5a330c...",
  "transfers": [
    {
      "contractAddress": "Z5a330c...",
      "from": "Z2019ea...",
      "to": "Zaaabbb...",
      "amount": "1000000000000000000",
      "blockNumber": "0x1a4",
      "txHash": "0xabc...",
      "timestamp": "1706123456",
      "tokenSymbol": "MTK",
      "tokenDecimals": 18,
      "tokenName": "MyToken",
      "transferType": "transfer"
    }
  ],
  "totalTransfers": 150,
  "page": 0,
  "limit": 25
}
```

---

### Validators

#### `GET /validators?page_token=`
List all validators with status and staking info.

**Query Parameters:**
| Param | Default | Description |
|-------|---------|-------------|
| `page_token` | (none) | Pagination token (currently unused in implementation) |

**Response:**
```json
{
  "validators": [
    {
      "index": "0",
      "address": "0xabc123...",
      "status": "active",
      "age": 100,
      "stakedAmount": "10000000000000",
      "isActive": true
    }
  ],
  "totalStaked": "1280000000000000"
}
```

**Notes:**
- Status is computed from activation/exit epochs relative to current epoch: `active`, `pending`, `exited`, `slashed`.
- Current epoch = `latestBlockNumber / 128`.
- All validators are stored in a single MongoDB document with `_id: "validators"`.

#### `GET /validator/:id`
Individual validator details by index or public key hex.

**Response:**
```json
{
  "index": "0",
  "publicKeyHex": "0xabc123...",
  "withdrawalCredentialsHex": "0xdef456...",
  "effectiveBalance": "10000000000000",
  "slashed": false,
  "activationEligibilityEpoch": "0",
  "activationEpoch": "0",
  "exitEpoch": "18446744073709551615",
  "withdrawableEpoch": "18446744073709551615",
  "status": "active",
  "age": 100,
  "currentEpoch": "100"
}
```

#### `GET /validators/stats`
Aggregated validator statistics.

**Response:**
```json
{
  "totalValidators": 128,
  "activeCount": 120,
  "pendingCount": 5,
  "exitedCount": 2,
  "slashedCount": 1,
  "totalStaked": "1280000000000000",
  "currentEpoch": "100"
}
```

#### `GET /validators/history?limit=100`
Historical validator count data for charts.

**Query Parameters:**
| Param | Default | Description |
|-------|---------|-------------|
| `limit` | `100` | Max records to return |

**Response:**
```json
{
  "history": [
    {
      "epoch": "99",
      "timestamp": 1706123456,
      "validatorsCount": 128,
      "activeCount": 120,
      "pendingCount": 5,
      "exitedCount": 2,
      "slashedCount": 1,
      "totalStaked": "1280000000000000"
    }
  ]
}
```

#### `GET /epoch`
Current epoch information.

**Response:**
```json
{
  "headEpoch": "100",
  "headSlot": "12800",
  "finalizedEpoch": "98",
  "justifiedEpoch": "99",
  "slotsPerEpoch": 128,
  "secondsPerSlot": 60,
  "slotInEpoch": 50,
  "timeToNextEpoch": 4680,
  "updatedAt": 1706123456
}
```

**Notes:**
- `slotsPerEpoch` = 128, `secondsPerSlot` = 60 (constants).
- `timeToNextEpoch` is computed as `(128 - slotInEpoch) * 60` seconds.

---

### Pending Transactions

#### `GET /pending-transactions?page=1&limit=10`
Paginated list of pending (mempool) transactions.

**Query Parameters:**
| Param | Default | Description |
|-------|---------|-------------|
| `page` | `1` | Page number (1-indexed) |
| `limit` | `10` | Results per page |

**Response:**
```json
{
  "transactions": [
    {
      "hash": "0xabc...",
      "from": "Z2019ea...",
      "to": "Z5a330c...",
      "value": "0x1bc16d674ec80000",
      "gas": "0x5208",
      "gasPrice": "0x3b9aca00",
      "nonce": "0x0",
      "status": "pending",
      "lastSeen": 1706123456,
      "createdAt": 1706123400
    }
  ],
  "total": 3,
  "page": 1,
  "limit": 10,
  "totalPages": 1
}
```

**Notes:** Excludes transactions with status `"mined"`. Timestamps are serialized as Unix timestamps.

#### `GET /pending-transaction/:hash`
Single pending transaction by hash. Implements lifecycle management:
1. If found in `pending_transactions` with status `"mined"`, deletes it and returns 404 with `"status": "mined"`.
2. If not found in pending, checks the `transfer` collection for a mined transaction.
3. If found as mined, returns `"status": "mined"` with block number.
4. If not found anywhere, returns 404 with an explanation that the tx may have been dropped.

---

## Database Collections & Query Patterns

All collections are in the `qrldata-z` database. Collection references are initialized as package-level variables in `configs/const.go`.

### Core Collections

| Collection | Purpose | Key Query Patterns |
|------------|---------|-------------------|
| `blocks` | Full block data (header + transactions) | Find by `result.number` (hex), `result.hash`; sorted by `result.timestamp` desc |
| `transfer` | Individual transaction records | Find by `txHash` (byte array) |
| `transactionByAddress` | Indexed transactions by address | Find by `from`/`to` (case-insensitive regex); sorted by `timeStamp` desc |
| `internalTransactionByAddress` | Internal transactions (contract calls) | Find by `from`/`to` (hex-decoded bytes); sorted by `blockTimestamp` desc |
| `addresses` | Wallet balances and metadata | Find by `id` (lowercase hex string); sorted by `balance` desc for richlist |
| `pending_transactions` | Mempool transactions | Find by `_id` (hash); filter `status != "mined"`; sorted by `createdAt` desc |
| `sync_state` | Sync progress tracking | Find by `_id: "last_synced_block"` to get `block_number` |

### Contract & Token Collections

| Collection | Purpose | Key Query Patterns |
|------------|---------|-------------------|
| `contractCode` | Smart contract deployments & token metadata | Find by `address` (both Z/z prefix); search by `name` regex; filter by `isToken` |
| `tokenBalances` | Token holder balances per contract | Find by `holderAddress` or `contractAddress` (both prefix variants); aggregation with `$lookup` to `contractCode` |
| `tokenTransfers` | ERC20 transfer events | Find by `contractAddress` or `txHash`; sorted by `blockNumber` desc |

### Analytics & Market Collections

| Collection | Purpose | Key Query Patterns |
|------------|---------|-------------------|
| `coingecko` | CoinGecko market data (price, market cap, volume) | `FindOne` with empty filter |
| `priceHistory` | Historical price snapshots | Filter by `timestamp >= since`; sorted by `timestamp` desc; limit by interval |
| `walletCount` | Total wallet count | Find by `_id: "current_count"` |
| `dailyTransactionsVolume` | Daily transaction volume | `FindOne` with empty filter |
| `totalCirculatingSupply` | Circulating supply | `FindOne` with empty filter |
| `averageBlockSize` | Block size history | Full scan sorted by `timestamp` asc |

### Validator Collections

| Collection | Purpose | Key Query Patterns |
|------------|---------|-------------------|
| `validators` | Single document containing all validators per epoch | Find by `_id: "validators"` |
| `validator_history` | Historical validator counts per epoch | Full scan sorted by `epoch` desc; limited |
| `epoch_info` | Current epoch state | Find by `_id: "current"` |

### Indexes

Created on startup if missing:

**`blocks` collection:**
- `result_number_timestamp`: `{ result.number: -1, result.timestamp: 1 }`
- `result_hash`: `{ result.hash: 1 }`

**`transactionByAddress` collection:**
- `timestamp_desc`: `{ timeStamp: -1 }`
- `tx_hash`: `{ txHash: 1 }`

### Initialized Collections

On startup, `initializeCollections()` creates default documents (via `$setOnInsert` + upsert) for:
- `walletCount`: `{ _id: "current_count", count: 0 }`
- `dailyTransactionsVolume`: `{ volume: 0 }`
- `totalCirculatingSupply`: `{ circulating: "0" }`
- `coingecko`: `{ marketCapUSD: 1e18, priceUSD: 1000, lastUpdated: <now> }`

---

## Data Models

### Address
```go
type Address struct {
    ObjectId primitive.ObjectID `bson:"_id"`
    ID       string             `json:"id"`       // Lowercase hex with z prefix
    Balance  float64            `json:"balance"`   // QRL units (not wei)
    Nonce    uint64             `json:"nonce"`
}
```

### Block (Result)
```go
type Result struct {
    BaseFeePerGas    string        `json:"baseFeePerGas"`    // Hex
    GasLimit         string        `json:"gasLimit"`         // Hex
    GasUsed          string        `json:"gasUsed"`          // Hex
    Hash             string        `json:"hash"`             // 0x-prefixed
    Number           string        `json:"number"`           // Hex
    ParentHash       string        `json:"parentHash"`
    Timestamp        string        `json:"timestamp"`        // Hex Unix timestamp
    Transactions     []Transaction `json:"transactions"`
    Withdrawals      []Withdrawal  `json:"withdrawals"`
    Size             string        `json:"size"`             // Hex
    Miner            string        `json:"miner"`
    // ... additional fields
}
```

### Transaction (in block)
```go
type Transaction struct {
    BlockHash   string `json:"blockHash"`
    BlockNumber string `json:"blockNumber"`
    From        string `json:"from"`
    Gas         string `json:"gas"`
    GasPrice    string `json:"gasPrice"`
    Hash        string `json:"hash"`
    Nonce       string `json:"nonce"`
    To          string `json:"to"`
    Value       string `json:"value"`
    Signature   string `json:"signature"`
    PublicKey   string `json:"publicKey"`
    Data        string `json:"data"`
    Status      string `json:"status"`
}
```

### TransactionByAddress
```go
type TransactionByAddress struct {
    InOut       int     `json:"InOut"`        // 0=outgoing, 1=incoming
    TxType      string  `json:"TxType"`
    Address     string  `json:"Address"`      // Counterparty address
    From        string  `json:"From"`
    To          string  `json:"To"`
    TxHash      string  `json:"TxHash"`
    TimeStamp   string  `json:"TimeStamp"`    // Decimal Unix timestamp
    Amount      float64 `json:"-"`            // Serialized as string with 18 decimals
    PaidFees    float64 `json:"-"`            // Serialized as string with 18 decimals
    BlockNumber string  `json:"BlockNumber"`  // Hex -> decimal in JSON
}
```

**Note:** Custom `MarshalJSON` converts `Amount` and `PaidFees` to `"%.18f"` format and `BlockNumber` from hex to decimal string.

### Transfer
```go
type Transfer struct {
    BlockNumber    string `bson:"blockNumber"`    // Hex
    BlockTimestamp string `bson:"blockTimestamp"`  // Hex
    From           string `bson:"from"`
    To             string `bson:"to"`
    TxHash         string `bson:"txHash"`
    Value          string `bson:"value"`           // Hex wei
    GasUsed        string `bson:"gasUsed"`         // Hex
    GasPrice       string `bson:"gasPrice"`        // Hex
    Nonce          string `bson:"nonce"`            // Hex
    Signature      string `bson:"signature"`
    Pk             string `bson:"pk"`
    Size           string `bson:"size"`             // Hex
}
```

### ContractInfo
```go
type ContractInfo struct {
    ContractCreatorAddress string `json:"creatorAddress" bson:"creatorAddress"`
    ContractAddress        string `json:"address" bson:"address"`
    ContractCode           string `json:"contractCode" bson:"contractCode"`
    CreationTransaction    string `json:"creationTransaction" bson:"creationTransaction"`
    CreationBlockNumber    string `json:"creationBlockNumber" bson:"creationBlockNumber"`
    IsToken                bool   `json:"isToken" bson:"isToken"`
    Status                 string `json:"status" bson:"status"`
    TokenDecimals          uint8  `json:"decimals" bson:"decimals"`
    TokenName              string `json:"name" bson:"name"`
    TokenSymbol            string `json:"symbol" bson:"symbol"`
    TotalSupply            string `json:"totalSupply" bson:"totalSupply"`
    UpdatedAt              string `json:"updatedAt" bson:"updatedAt"`
}
```

### TokenBalance
```go
type TokenBalance struct {
    ContractAddress string `json:"contractAddress" bson:"contractAddress"`
    HolderAddress   string `json:"holderAddress" bson:"holderAddress"`
    Balance         string `json:"balance" bson:"balance"`          // Raw integer string
    BlockNumber     string `json:"blockNumber" bson:"blockNumber"`
    Name            string `json:"name,omitempty"`                   // Via aggregation
    Symbol          string `json:"symbol,omitempty"`                 // Via aggregation
    Decimals        int    `json:"decimals,omitempty"`               // Via aggregation
}
```

### TokenTransfer
```go
type TokenTransfer struct {
    ContractAddress string `json:"contractAddress" bson:"contractAddress"`
    From            string `json:"from" bson:"from"`
    To              string `json:"to" bson:"to"`
    Amount          string `json:"amount" bson:"amount"`
    BlockNumber     string `json:"blockNumber" bson:"blockNumber"`
    TxHash          string `json:"txHash" bson:"txHash"`
    Timestamp       string `json:"timestamp" bson:"timestamp"`
    TokenSymbol     string `json:"tokenSymbol" bson:"tokenSymbol"`
    TokenDecimals   int    `json:"tokenDecimals" bson:"tokenDecimals"`
    TokenName       string `json:"tokenName" bson:"tokenName"`
    TransferType    string `json:"transferType" bson:"transferType"`
}
```

### Validator Models
```go
// Storage format (single document in MongoDB)
type ValidatorStorage struct {
    ID         string            `bson:"_id"`         // Always "validators"
    Epoch      string            `bson:"epoch"`
    Validators []ValidatorRecord `bson:"validators"`
    UpdatedAt  string            `bson:"updatedAt"`
}

type ValidatorRecord struct {
    Index                      string `bson:"index"`
    PublicKeyHex               string `bson:"publicKeyHex"`
    WithdrawalCredentialsHex   string `bson:"withdrawalCredentialsHex"`
    EffectiveBalance           string `bson:"effectiveBalance"`   // Decimal string
    Slashed                    bool   `bson:"slashed"`
    ActivationEligibilityEpoch string `bson:"activationEligibilityEpoch"`
    ActivationEpoch            string `bson:"activationEpoch"`
    ExitEpoch                  string `bson:"exitEpoch"`
    WithdrawableEpoch          string `bson:"withdrawableEpoch"`
}

// API response format
type Validator struct {
    Index        string `json:"index"`
    Address      string `json:"address"`       // PublicKeyHex
    Status       string `json:"status"`        // active/pending/exited/slashed
    Age          int64  `json:"age"`           // Epochs since activation
    StakedAmount string `json:"stakedAmount"`
    IsActive     bool   `json:"isActive"`
}
```

### CoinGecko / Price
```go
type CoinGecko struct {
    MarketCapUSD float64   `bson:"marketCapUSD"`
    PriceUSD     float64   `bson:"priceUSD"`
    VolumeUSD    float64   `bson:"volumeUSD"`
    LastUpdated  time.Time `bson:"lastUpdated"`
}

type PriceHistory struct {
    Timestamp    time.Time `json:"timestamp"`
    PriceUSD     float64   `json:"priceUSD"`
    MarketCapUSD float64   `json:"marketCapUSD"`
    VolumeUSD    float64   `json:"volumeUSD"`
}
```

### PendingTransaction
```go
type PendingTransaction struct {
    Hash      string    `json:"hash" bson:"_id"`      // TX hash is the document ID
    From      string    `json:"from" bson:"from"`
    To        string    `json:"to,omitempty"`
    Value     string    `json:"value" bson:"value"`
    Gas       string    `json:"gas" bson:"gas"`
    GasPrice  string    `json:"gasPrice" bson:"gasPrice"`
    Nonce     string    `json:"nonce" bson:"nonce"`
    Input     string    `json:"input" bson:"input"`
    Status    string    `json:"status" bson:"status"`    // "pending", "mined", "dropped"
    LastSeen  time.Time `json:"lastSeen"`                // Serialized as Unix timestamp
    CreatedAt time.Time `json:"createdAt"`               // Serialized as Unix timestamp
}
```

---

## Configuration

### Environment Variables

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| `MONGOURI` | Yes | -- | MongoDB connection string (e.g., `mongodb://localhost:27017`) |
| `NODE_URL` | No | `http://127.0.0.1:8545` | Zond node RPC endpoint (used for `zond_getBalance`) |
| `APP_ENV` | No | `development` | Environment mode. Set to `production` for HTTPS. |
| `HTTP_PORT` | No | `:8080` | HTTP listen port (development mode) |
| `HTTPS_PORT` | No* | -- | HTTPS listen port (production mode; required if `APP_ENV=production`) |
| `CERT_PATH` | No* | -- | TLS certificate file path (required if `APP_ENV=production`) |
| `KEY_PATH` | No* | -- | TLS private key file path (required if `APP_ENV=production`) |

### .env File

The API loads environment variables from a `.env` file in the working directory. The filename is `.env` + `$APP_ENV` (e.g., `.envproduction`, `.envdevelopment`). If `MONGOURI` is already set as an environment variable (e.g., via Docker), the `.env` file is skipped.

Minimal `.env` example:
```
MONGOURI=mongodb://localhost:27017
NODE_URL=http://localhost:8545
HTTP_PORT=:8081
```

### MongoDB Setup

- **Database name:** `qrldata-z`
- **Connection:** Via `MONGOURI` environment variable
- **Connection singleton:** Uses `sync.Once` to ensure single connection across the application
- **Timeout:** 10-second context timeout on connection
- The database and collections are populated by the `Zond2mongoDB` synchronizer component (not the API).

---

## Build & Run

### Local Development

```bash
cd backendAPI

# Install dependencies
go mod download

# Build
go build -o backendAPI main.go

# Run (ensure .env or env vars are set)
./backendAPI
```

The server starts on `HTTP_PORT` (default `:8080`) in development mode.

### Docker

```bash
# Build image
docker build -t zondscan-backend .

# Run container
docker run -d \
  -p 8080:8080 \
  -e MONGOURI=mongodb://host.docker.internal:27017 \
  -e NODE_URL=http://host.docker.internal:8545 \
  zondscan-backend
```

The Dockerfile uses a multi-stage build:
1. **Builder stage:** `golang:1.24-alpine` -- compiles with static linking (`CGO_ENABLED=0`)
2. **Production stage:** `alpine:latest` -- runs as non-root user (UID 1000)

### Production (HTTPS)

```bash
APP_ENV=production \
HTTPS_PORT=:443 \
CERT_PATH=/etc/ssl/cert.pem \
KEY_PATH=/etc/ssl/key.pem \
MONGOURI=mongodb://localhost:27017 \
./backendAPI
```

### With PM2

```bash
# Via the deploy script at the repo root
./deploy.sh
```

---

## Key Implementation Details

### Address Normalization

QRL Zond addresses use a `Z` prefix (instead of Ethereum's `0x`). The codebase handles multiple address formats:

- **Storage in MongoDB:** Most addresses are stored with lowercase `z` prefix by the synchronizer.
- **API input:** Accepts both `Z` and `z` prefixes. Routes normalize `z` to `Z` on input.
- **Database queries:** Use both variants via `normalizeAddressBoth()` which returns `["z...", "Z..."]`.
- **Case-insensitive matching:** Transaction lookups use MongoDB regex with the `i` option.
- **RPC calls:** `GetBalance` ensures uppercase `Z` prefix for Zond node RPC.

### Pagination Patterns

Two pagination styles are used:

**1-indexed (page starts at 1):**
- `/txs?page=1` -- skip = `(page - 1) * limit`
- `/pending-transactions?page=1&limit=10`
- `/address/:addr/transactions?page=1&limit=5`

**0-indexed (page starts at 0):**
- `/contracts?page=0&limit=10` -- skip = `page * limit`
- `/token/:addr/holders?page=0&limit=25`
- `/token/:addr/transfers?page=0&limit=25`

Maximum limit is enforced at 100 for token holder and transfer endpoints.

### Token Handling

Token data comes from three linked collections:

1. **`contractCode`** -- Contract metadata including `isToken`, `name`, `symbol`, `decimals`, `totalSupply`
2. **`tokenBalances`** -- Per-holder balances with `contractAddress` + `holderAddress` + `balance`
3. **`tokenTransfers`** -- Transfer events with full context (from, to, amount, tx hash)

The `/address/:addr/tokens` endpoint uses a MongoDB aggregation pipeline:
```
tokenBalances (match holderAddress)
  -> $addFields (lowercase contractAddress)
  -> $lookup (join contractCode on lowercase address match)
  -> $unwind (flatten joined array)
  -> $project (select fields + token metadata)
  -> $addFields ($toDecimal for balance sorting)
  -> $sort (balance descending)
```

### Hex/Decimal Conversions

- Block numbers stored as hex strings (`"0x1a4"`) in MongoDB. Converted to decimal (`420`) in API responses where appropriate.
- Balances stored as hex wei strings. Converted to float64 QRL (divided by 1e18) for address balances.
- `TransactionByAddress.Amount` and `PaidFees` are `float64` internally but serialized as strings with 18 decimal places (`"1.500000000000000000"`).
- `TransactionByAddress.BlockNumber` is converted from hex to decimal in the custom JSON marshaler.

### Validator Status Computation

Validators are stored in a single document. Status is computed at query time:

```
if slashed -> "slashed"
if activationEpoch > currentEpoch -> "pending"
if exitEpoch <= currentEpoch -> "exited"
else -> "active"
```

- `currentEpoch = latestBlockNumber / 128`
- `FAR_FUTURE_EPOCH = "18446744073709551615"` (uint64 max) indicates a validator has not exited.
- Age in epochs = `currentEpoch - activationEpoch`.

### Error Handling & Resilience

- All database queries use 10-second (or 15-second for token aggregations) context timeouts.
- The handler includes a custom recovery middleware that catches panics and returns HTTP 500.
- A monitor middleware logs request latency for all endpoints.
- Main function writes panic stack traces to `crash_<timestamp>.log` files.
- Application log output goes to both stdout and `backendAPI.log`.
- Null arrays are converted to empty arrays (`[]`) before returning JSON responses.

### CORS Configuration

```go
cors.Config{
    AllowOrigins:     []string{"*"},
    AllowMethods:     []string{"GET", "POST"},
    AllowHeaders:     []string{"Origin", "Content-Length", "Content-Type", "Authorization"},
    AllowCredentials: true,
    MaxAge:           12 * time.Hour,
}
```

### Constants

```go
const QUANTA float64 = 1000000000000000000  // 1 QRL = 1e18 wei
const SlotsPerEpoch = 128
const SecondsPerSlot = 60
```

### Dependencies

| Package | Version | Purpose |
|---------|---------|---------|
| `gin-gonic/gin` | v1.9.1 | HTTP framework |
| `gin-contrib/cors` | v1.6.0 | CORS middleware |
| `go-playground/validator` | v10.19.0 | Input validation |
| `joho/godotenv` | v1.5.1 | .env file loading |
| `mongo-driver` | v1.8.4 | MongoDB driver |
