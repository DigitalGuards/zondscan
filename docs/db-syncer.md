# QRL2MongoDB - Blockchain Database Synchronizer

Comprehensive documentation for the QRL Zond blockchain synchronizer that powers [zondscan.com](https://zondscan.com). This service connects to a QRL Zond node via JSON-RPC, fetches blocks and transactions, and writes them to MongoDB (database: `qrldata-z`).

**Source location:** `QRL2MongoDB/`

---

## Table of Contents

1. [Architecture Overview](#1-architecture-overview)
2. [Sync Process](#2-sync-process)
3. [MongoDB Collections and Schemas](#3-mongodb-collections-and-schemas)
4. [RPC Calls](#4-rpc-calls)
5. [Token Transfer Detection and Indexing](#5-token-transfer-detection-and-indexing)
6. [Contract Detection and Metadata Extraction](#6-contract-detection-and-metadata-extraction)
7. [Validator Data Synchronization](#7-validator-data-synchronization)
8. [Mempool / Pending Transaction Handling](#8-mempool--pending-transaction-handling)
9. [Configuration](#9-configuration)
10. [How to Build and Run](#10-how-to-build-and-run)
11. [Reindexing Scripts](#11-reindexing-scripts)
12. [Key Implementation Details](#12-key-implementation-details)

---

## 1. Architecture Overview

```
                                    QRL2MongoDB
                                    ============

 +-----------------+          +-------------------------------+          +----------------+
 | QRL Zond Node   |  JSON-   |  main.go                      |          |   MongoDB      |
 | (Execution)     |  RPC     |    |                          |          | (qrldata-z)    |
 | :8545           |<-------->|    +-- synchroniser/           |--------->|                |
 +-----------------+          |    |     sync.go               |  Insert  | - blocks       |
                              |    |     producer_consumer.go  |  Upsert  | - transfer     |
 +-----------------+          |    |     pending_sync.go       |  Update  | - addresses    |
 | Beacon Chain    |  HTTP    |    |     periodic_tasks.go     |          | - contractCode |
 | API             |  REST    |    |     gap_detection.go      |          | - tokenTransfers|
 | :3500           |<-------->|    |     token_sync.go         |          | - validators   |
 +-----------------+          |    +-- db/                     |          | - ...18+ more  |
                              |    +-- rpc/                    |          +----------------+
 +-----------------+          |    +-- services/               |
 | CoinGecko API   |  HTTPS   |    +-- fetch/                 |
 | (market data)   |<-------->|    +-- configs/                |
 +-----------------+          +-------------------------------+
                                        |
                                  Health endpoint
                                  :8081/health
```

### Data Flow

MongoDB must provide multi-document transactions through a replica set or `mongos`. The syncer validates the topology with the `hello` command before creating collections or starting background workers. A standalone `mongod` is rejected because reorg rollback updates the indexed chain, companion records, and sync high-water mark as one transaction.

Exactly one syncer process may target a database. The process acquires a renewable lease in `syncer_leases` before starting writers and terminates if lease renewal fails. A new generation can acquire only a fresh or cleanly released lease. TTL expiry alone never grants takeover because a suspended old process could resume an in-flight write. Deployment configuration must keep one syncer replica and use a no-overlap rollout strategy.

Crash recovery is fail closed. First compute-fence the old process and host so they cannot resume, then inspect the exact expired lease `owner` and `generation`, and acknowledge that identity:

```bash
cd QRL2MongoDB
MONGOURI='mongodb://<replica-set>/?replicaSet=rs0' go run ./cmd/recover-syncer-lease --owner '<inspected-owner>' --generation <inspected-generation> --compute-fenced
```

The command rejects live leases and identity mismatches. Do not use it merely because a health check is failing. A pre-upgrade lease row that lacks `releasedGeneration` also requires this compute-fenced acknowledgement after the old binary is stopped.

### Block companion migration and rollout order

The current API and syncer fail closed on markerless, pending, unknown-state, missing-height, duplicate-height, or conflicting block rows. They also require the named unique `blockNumberInt_desc_idx` index. Existing databases must complete the offline companion migration before either new binary serves traffic:

```bash
cd QRL2MongoDB
MONGOURI='mongodb://<replica-set>/?replicaSet=rs0' go run ./cmd/reindex-block-companions
MONGOURI='mongodb://<replica-set>/?replicaSet=rs0' NODE_URLS='http://<reviewed-node>:8545' go run ./cmd/reindex-block-companions --execute
```

Use this order: stop the old backend and every writer, verify the old syncer released its lease, take a recoverable database backup, run the dry inventory, run `--execute`, require the final canonical audit attestation and unique-height index, deploy both binaries, then start the syncer. `--limit N` creates a resumable checkpoint; keep public traffic stopped until an unlimited final run reports that the migration gate is clear. The command acquires and renews the same exclusive lease as the normal syncer.

The migration rejects every repeated height, including rows with the same hash. Its final unlimited pass upgrades the historical non-unique height index only after the canonical audit succeeds. The syncer then initializes the independent token scan state. Blocks with transactions enter `tokenIngestionState: "pending"`; empty blocks become `complete`. Token endpoints fail closed while pending blocks replay. Exact-block token classification and balance reads require a node that retains the historical state selected by block hash. A pruned node can leave the oldest token claim retrying and intentionally stall the complete prefix. For an uninterrupted token API rollout, wait until no complete blocks with transactions remain missing or pending token state before starting the backend.

1. **Initial Sync**: On startup, the syncer determines the last synced block from the `sync_state` collection. It then fetches all blocks from that point to the chain head using a producer/consumer pattern with concurrent block fetching.

2. **Continuous Monitoring**: After the initial sync, it enters a polling loop (every 30 seconds) that checks for new blocks and processes them individually or in batches depending on how far behind it is.

3. **Parallel Services**: Several background services run concurrently:
   - Mempool sync (every 1 second)
   - Market data updates (every 30 minutes)
   - Validator updates (every 6 hours)
   - Gap detection (every 5 minutes)
   - Wallet count sync (every 4 hours)
   - Contract reprocessing (every 1 hour)
   - Durable token block retry drain (immediately, then every 30 seconds)

### Receipt accounting and calldata compatibility

Receipt enrichment validates transaction hash, canonical block number/hash, execution status, gas consumption within the transaction limit, and effective gas price before block insertion. The block transaction retains `status`, `gasUsed`, and `effectiveGasPrice`. Companion fees are computed from those retained receipt quantities; missing accounting remains retryable. Zero effective gas price produces an exact zero fee. Newly written history rows carry `feeSource: "receipt"` and `receiptStatus`.

RPC calldata is decoded from `input` and stored under the existing BSON `data` key. Old BSON rows remain readable. The block API exposes `input` and preserves `data` as a compatibility alias. Retrying an incomplete block refreshes its transaction payload from the fetched block and validated receipts before companion completion. Already complete historical blocks are preserved.

Previously indexed `paidFees` and `paidFeesWei` may contain gas-limit estimates. The API omits their `PaidFees` field until receipt provenance is established. It can use a matching live receipt to enrich an individual historical transaction response without modifying stored rows. Missing gas, fee, or execution status is displayed as unavailable. This release performs no automatic replay or retention change.

For a later historical repair, first snapshot the database and record the exact canonical range while writers are stopped. Inventory complete blocks whose transactions lack receipt accounting or calldata, then refetch each selected block and its matching receipts using an explicit bounded range and an exclusive writer lease. A reviewed repair must atomically update the block transaction payload and corresponding fee/status companion fields only when the stored block number/hash still match; it must preserve unrelated token, contract, and verification state. Validate row counts, identities, fee products, and a resume checkpoint before activation. The existing `reindex-block-companions` command skips complete blocks, so it is not a repair command for these historical gaps. Use the separately invoked [receipt and calldata backfill tool](../scripts/receipt-backfill.md), following its topology, backup, pause, journal, and verification requirements. Preserve completion markers and existing history throughout the repair.

### Module Structure

| Directory | Purpose |
|-----------|---------|
| `main.go` | Entry point, signal handling, health server |
| `synchroniser/` | Core sync logic, periodic tasks, token processing |
| `db/` | All MongoDB read/write operations |
| `rpc/` | Zond node JSON-RPC client |
| `services/` | Validator data processing and storage |
| `configs/` | MongoDB connection, collection references, constants |
| `models/` | Go struct definitions for all data types |
| `fetch/` | External API clients (CoinGecko) |
| `validation/` | Hex string and address validation |
| `utils/` | Hex math utilities |
| `logger/` | Structured logging (zap) configuration |
| `scripts/` | Python reindexing utilities |

---

## 2. Sync Process

### 2.1 Startup Sequence (`main.go`)

```
main()
  |-- Connect to MongoDB and validate transaction topology
  |-- Acquire and renew the exclusive syncer lease
  |-- Require the block companion attestation and unique height index
  |-- Initialize and verify token collections, indexes, and legacy ordering backfill
  |-- Start health check server (:$HEALTH_PORT/health)
  |-- StartPendingTransactionSync()    <-- background goroutine
  |-- Sync()                           <-- blocks until caught up, then continuous
```

### 2.2 Initial Batch Sync (`synchroniser/sync.go`)

The `Sync()` function performs the initial catch-up:

1. **Determine starting block**: Reads the last synced block from `sync_state` collection. Falls back to finding the latest block in the `blocks` collection. If empty, starts from genesis (`0x0`).

2. **Store initial sync start**: Records `0x1` in `sync_initial_state` collection (used later for token processing range).

3. **Get chain head**: Calls `qrl_blockNumber` to get the latest block on the network (with retry up to 5 times with exponential backoff).

4. **Batch processing**: Uses a producer/consumer pattern to process blocks in parallel:
   - Batch size: **64** blocks (normal) or **128** blocks (when >1000 blocks behind)
   - Max concurrent producers: **8**
   - Channel buffer: **32** producer channels

5. **Post-sync tasks** (after initial sync completes):
   - Calculate daily transaction volume
   - Drain durable token block work for the synced range
   - Start wallet count sync service
   - Start contract reprocessing service
   - Enter continuous block monitoring

### 2.3 Producer/Consumer Pattern (`synchroniser/producer_consumer.go`)

```
Sync()
  |
  |-- Creates buffered channel of producer channels (cap 32)
  |-- Starts single consumer goroutine
  |-- Creates producers for each block range
  |
  +-- producer(start, end) -> <-chan Data
  |     |-- Acquires semaphore token (max 8 concurrent)
  |     |-- For each block in range:
  |     |     |-- Skip if block already exists in DB
  |     |     |-- Sleep (reduced RPC delay for bulk: 5-7ms)
  |     |     |-- Fetch block with 3 retries (100ms backoff)
  |     |     |-- Track failed blocks for later retry
  |     |     |-- Update transaction statuses
  |     |     |-- Accumulate block data and numbers
  |     |-- Send accumulated Data to channel
  |     |-- Release semaphore token
  |
  +-- consumer(ch <-chan (<-chan Data))
        |-- For each producer channel:
        |     |-- Spawn goroutine to consume Data
        |     |-- InsertManyBlockDocuments()
        |     |-- ProcessTransactions() for each block
        |     |-- Track processed blocks for gap detection
        |     |-- Atomically track highest processed block
        |-- After all producers done:
              |-- Force update sync state to highest block
              |-- Check for gaps in processed blocks
```

**Data struct:**
```go
type Data struct {
    blockData    []interface{}   // Block documents
    blockNumbers []int           // Corresponding block numbers
}
```

### 2.4 Single Block Insertion / Continuous Monitoring (`synchroniser/periodic_tasks.go`)

After the initial sync, `singleBlockInsertion()` starts four concurrent tickers:

| Task | Interval | Description |
|------|----------|-------------|
| Block processing | 30 seconds | Check for new blocks, process individually or batch |
| Data updates | 30 minutes | CoinGecko price, wallet count, volume, block sizes |
| Validator updates | 6 hours | Fetch validators from beacon chain API |
| Gap detection | 5 minutes | Find and fill missing blocks (after 1min initial delay) |

**Block processing logic** (`processBlockPeriodically`):
- If more than **64 blocks behind** (BatchSyncThreshold): uses `batchSync()` for parallel fetching
- If fewer than 64 blocks behind: processes blocks one-by-one with `processSubsequentBlocks()`
- After processing, runs token transfer processing for the new blocks

**Single block processing** (`processSubsequentBlocks`):
1. Fetches block from node (3 retries, 500ms backoff)
2. Verifies parent hash matches the previous block in DB
3. If parent hash mismatch: rolls back and resyncs from parent (chain reorg handling)
4. Inserts block document and processes transactions
5. Updates pending transaction statuses
6. Stores the block number as the last known synced block

### 2.5 Gap Detection and Filling (`synchroniser/gap_detection.go`)

Gaps can occur during concurrent batch processing if individual block fetches fail. The system has three layers of gap protection:

1. **During batch sync**: After batch completes, `detectGaps()` queries the DB for the expected block range and identifies missing block numbers (limited to last 1000 blocks).

2. **Periodic gap detection**: Every 5 minutes, scans the last 1000 blocks for gaps.

3. **Failed block tracking**: A `sync.Map` tracks blocks that failed to sync with attempt counts and timestamps. Blocks are retried up to 3 times (`GapRetryAttempts`).

```go
type FailedBlock struct {
    BlockNumber string
    Attempts    int
    LastError   error
    LastAttempt time.Time
}
```

---

## 3. MongoDB Collections and Schemas

Database name: **`qrldata-z`**

### 3.1 Core Data Collections

#### `blocks`
Stores complete block data as fetched from the Zond node.

```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "result": {
    "baseFeePerGas": "0x...",
    "gasLimit": "0x...",
    "gasUsed": "0x...",
    "hash": "0x...",
    "number": "0x1a",
    "parentHash": "0x...",
    "receiptsRoot": "0x...",
    "stateRoot": "0x...",
    "timestamp": "0x...",
    "transactions": [
      {
        "blockHash": "0x...",
        "blockNumber": "0x1a",
        "from": "Q...",
        "gas": "0x...",
        "gasPrice": "0x...",
        "hash": "0x...",
        "nonce": "0x...",
        "to": "Q...",
        "transactionIndex": "0x0",
        "type": "0x0",
        "value": "0x...",
        "chainId": "0x...",
        "signature": "...",
        "publicKey": "...",
        "data": "0x...",
        "status": "0x1"
      }
    ],
    "transactionsRoot": "0x...",
    "size": "0x...",
    "withdrawals": [],
    "withdrawalsRoot": "0x..."
  }
}
```

All numeric values are stored as hex strings with `0x` prefix. Block documents are the full JSON-RPC response including `jsonrpc` and `id` fields. The top-level `blockNumberInt` has a required unique descending index. Core companion writes and token effects have separate durable state:

| Field | Type | Description |
|-------|------|-------------|
| `blockNumberInt` | int64 | Canonical numeric height, unique across the collection |
| `ingestionState` | string | `pending` until all core block companions are durable, then `complete` |
| `companionsCompletedAt` | date | Core companion completion time |
| `tokenIngestionState` | string | `pending` until exact-block token effects complete, then `complete` |
| `tokenProcessingToken` | string | Random ownership token for the current token claim |
| `tokenProcessingUntil` | date | Mongo server-time crash-reclaim deadline |
| `tokenAttempts` | int | Durable token processing attempt count |
| `tokenNextAttemptAt` | date | Mongo server-time retry eligibility |
| `lastTokenError` | string | Bounded diagnostic from the last token failure |
| `tokenCompletedAt` | date | Exact-block token processing completion time |

#### `transfer`
Individual transaction records with derived values. One document per transaction.

| Field | Type | Description |
|-------|------|-------------|
| `blockNumber` | string | Hex block number |
| `blockTimestamp` | string | Hex unix timestamp |
| `from` | string | Sender address (lowercase) |
| `to` | string | Recipient address (lowercase, absent for contract creation) |
| `txHash` | string | Transaction hash |
| `pk` | string | Public key |
| `signature` | string | Transaction signature |
| `nonce` | string | Hex nonce |
| `value` | float64 | Amount in QRL (converted from wei) |
| `status` | string | Hex status (`0x1` = success) |
| `size` | string | Block size (hex) |
| `paidFees` | float64 | Transaction fee in QRL |
| `contractAddress` | string | Contract address (for creation txs, replaces `to`) |
| `data` | string | Transaction input data |

**Index**: `blockTimestamp` descending.

#### `transactionByAddress`
Compact transaction index for address-based lookups.

| Field | Type | Description |
|-------|------|-------------|
| `txType` | string | Transaction type |
| `from` | string | Sender address (lowercase) |
| `to` | string | Recipient address (lowercase) |
| `txHash` | string | Transaction hash |
| `timeStamp` | string | Block timestamp (hex) |
| `amount` | float64 | Value in QRL |
| `paidFees` | float64 | Fees in QRL |
| `blockNumber` | string | Block number (hex) |

#### `internalTransactionByAddress`
Internal transactions from `debug_traceTransaction` calls.

| Field | Type | Description |
|-------|------|-------------|
| `type` | string | Transaction type (e.g., "CALL") |
| `callType` | string | Call type (e.g., "delegatecall") |
| `hash` | string | Transaction hash |
| `from` | string | Internal caller (lowercase, Q-prefix) |
| `to` | string | Internal callee (lowercase, Q-prefix) |
| `input` | string | Input data (hex) |
| `output` | string | Output data (hex) |
| `traceAddress` | []int | Trace position |
| `value` | float64 | Value in QRL |
| `gas` | string | Gas limit (hex) |
| `gasUsed` | string | Gas used (hex) |
| `addressFunctionIdentifier` | string | Extracted function target address |
| `amountFunctionIdentifier` | string | Extracted function amount (hex) |
| `blockTimestamp` | string | Block timestamp (hex) |

#### `addresses`
Wallet and contract address balances.

| Field | Type | Description |
|-------|------|-------------|
| `id` | string | Address (lowercase) |
| `balance` | float64 | Current balance in QRL |
| `isContract` | bool | Whether address is a contract |

### 3.2 Contract and Token Collections

#### `contractCode`
Smart contract deployments and token metadata.

| Field | Type | Description |
|-------|------|-------------|
| `address` | string | Contract address (lowercase) |
| `status` | string | Deployment status (hex, `0x1` = success) |
| `isToken` | bool | Whether contract is an ERC20 token |
| `name` | string | Token name (if ERC20) |
| `symbol` | string | Token symbol (if ERC20) |
| `decimals` | uint8 | Token decimals (if ERC20) |
| `totalSupply` | string | Token total supply (decimal string) |
| `contractCode` | string | Bytecode (hex) |
| `creatorAddress` | string | Deployer address (lowercase) |
| `creationTransaction` | string | Deployment tx hash |
| `creationBlockNumber` | string | Deployment block (hex) |
| `updatedAt` | string | ISO 8601 timestamp |
| `maxSupply` | string | (Optional) Custom max supply |
| `maxWalletAmount` | string | (Optional) Custom max wallet size |
| `maxTxLimit` | string | (Optional) Custom max tx amount |

#### `tokenTransfers`
ERC-20, ERC-721, and ERC-1155 token transfer events extracted from transaction logs.

| Field | Type | Description |
|-------|------|-------------|
| `contractAddress` | string | Token contract address (lowercase) |
| `from` | string | Sender address (lowercase) |
| `to` | string | Recipient address (lowercase) |
| `amount` | string | Transfer amount (hex or decimal) |
| `blockNumber` | string | Block number (hex) |
| `blockHash` | string | Exact canonical block hash that emitted the event |
| `blockNumberInt` | int64 | Numeric block height used for ordering |
| `txHash` | string | Transaction hash |
| `logIndex` | string | Canonical event position within the block |
| `tokenID` | string | Decimal NFT token ID, empty for ERC-20 |
| `timestamp` | string | Block timestamp (hex) |
| `tokenSymbol` | string | Token symbol |
| `tokenDecimals` | uint8 | Token decimals |
| `tokenName` | string | Token name |
| `transferType` | string | `"direct"` or `"event"` |

**Indexes**: `(contractAddress, blockNumber)`, `(from, blockNumber)`, `(to, blockNumber)`, numeric `blockNumberInt`, NFT history, and `(txHash, contractAddress, logIndex, tokenID)` unique.

#### `tokenBalances`
Current token holder balances per contract.

| Field | Type | Description |
|-------|------|-------------|
| `contractAddress` | string | Token contract address (lowercase) |
| `holderAddress` | string | Holder address (lowercase) |
| `balance` | string | Current balance (decimal string via RPC) |
| `blockNumber` | string | Last updated block (hex) |
| `blockNumberInt` | int64 | Numeric height of the exact observation |
| `blockHash` | string | Exact block hash used for historical RPC reads |
| `tokenID` | string | Decimal NFT token ID, empty for ERC-20 |
| `tokenStandard` | string | `ERC-20`, `ERC-721`, or `ERC-1155` |
| `balanceStale` | bool | Hidden from public readers while reconciliation is pending |
| `updatedAt` | string | ISO 8601 timestamp |

**Index**: `(contractAddress, holderAddress, tokenID)` unique, plus standard and reconciliation query indexes.

**Schema validation** enforced on this collection (see `configs/setup.go`).

#### `tokenEventDeadLetters`
Durable audit records for structurally valid logs from confirmed token contracts whose event payload cannot be decoded canonically.

| Field | Type | Description |
|-------|------|-------------|
| `blockNumber` | string | Exact source height (hex) |
| `blockNumberInt` | int64 | Numeric source height used for rollback |
| `blockHash` | string | Exact source block hash |
| `txHash` | string | Exact source transaction hash |
| `logIndex` | string | Canonical event position within the block |
| `emitter` | string | Confirmed token contract address |
| `topic0` | string | Transfer event signature |
| `tokenStandard` | string | Confirmed token standard used for decoding |
| `reason` | string | Bounded deterministic rejection reason |
| `firstSeenAt` | date | Mongo server time when first quarantined |
| `lastSeenAt` | date | Mongo server time of the latest replay |

**Indexes**: `(blockHash, txHash, logIndex)` unique and `(blockNumberInt, blockHash)` for rollback. Startup fails closed if these indexes cannot be created. Reorg rollback deletes dead letters owned by the orphaned block.

#### `pending_token_contracts`
Queue for contracts awaiting token detection processing.

| Field | Type | Description |
|-------|------|-------------|
| `contractAddress` | string | Contract address |
| `txHash` | string | Transaction hash |
| `blockNumber` | string | Block number (hex) |
| `blockHash` | string | Canonical block hash bound to the queued transaction |
| `blockTimestamp` | string | Block timestamp (hex) |
| `processed` | bool | Whether this has been processed |
| `processing` | bool | Whether one worker currently owns the item |
| `processingToken` | string | Random ownership token required for completion |
| `processingUntil` | date | Mongo server-time claim deadline |
| `attempts` | int | Durable processing attempt count |
| `nextAttemptAt` | date | Mongo server-time retry eligibility |
| `lastError` | string | Bounded diagnostic from the last failed attempt |
| `createdAt` | date | Mongo server time when the queue identity was first seen |
| `updatedAt` | date | Mongo server time of the latest queue mutation |
| `processedAt` | date | Mongo server time when processing completed |

**Indexes**: `(contractAddress, txHash)` unique, queue claim and retry indexes, plus a 30-day TTL on completed `processedAt` records.

### 3.3 Validator Collections

#### `validators`
Single document containing all current validators.

| Field | Type | Description |
|-------|------|-------------|
| `_id` | string | Always `"validators"` |
| `epoch` | string | Current epoch (decimal) |
| `updatedAt` | string | Unix timestamp |
| `validators` | array | Array of ValidatorRecord objects |

Each **ValidatorRecord**:

| Field | Type | Description |
|-------|------|-------------|
| `index` | string | Validator index (decimal) |
| `publicKeyHex` | string | Public key (hex, converted from base64) |
| `withdrawalCredentialsHex` | string | Withdrawal credentials (hex) |
| `effectiveBalance` | string | Effective balance (decimal string) |
| `slashed` | bool | Slashing status |
| `activationEligibilityEpoch` | string | Eligibility epoch (decimal) |
| `activationEpoch` | string | Activation epoch (decimal) |
| `exitEpoch` | string | Exit epoch (decimal) |
| `withdrawableEpoch` | string | Withdrawable epoch (decimal) |
| `slotNumber` | string | Assigned slot (decimal) |
| `isLeader` | bool | Whether this validator is a slot leader |

#### `validatorHistory`
Per-epoch validator statistics.

| Field | Type | Description |
|-------|------|-------------|
| `epoch` | string | Epoch number (decimal, unique key) |
| `timestamp` | int64 | Unix timestamp |
| `validatorsCount` | int | Total validators |
| `activeCount` | int | Active validators |
| `pendingCount` | int | Pending validators |
| `exitedCount` | int | Exited validators |
| `slashedCount` | int | Slashed validators |
| `totalStaked` | string | Sum of effective balances (decimal) |

#### `epoch_info`
Current beacon chain head information.

| Field | Type | Description |
|-------|------|-------------|
| `_id` | string | Always `"current"` |
| `headEpoch` | string | Current head epoch |
| `headSlot` | string | Current head slot |
| `finalizedEpoch` | string | Last finalized epoch |
| `justifiedEpoch` | string | Last justified epoch |
| `finalizedSlot` | string | Last finalized slot |
| `justifiedSlot` | string | Last justified slot |
| `updatedAt` | int64 | Unix timestamp |

### 3.4 Analytics Collections

#### `coingecko`
Current QRL market data (single document, upserted).

| Field | Type | Description |
|-------|------|-------------|
| `marketCapUSD` | float32 | Market cap in USD |
| `priceUSD` | float32 | Current price in USD |
| `volumeUSD` | float32 | 24h trading volume in USD |
| `lastUpdated` | Date | Last update timestamp |

**Schema validation** enforced.

#### `priceHistory`
Historical price snapshots for charts.

| Field | Type | Description |
|-------|------|-------------|
| `timestamp` | Date | Snapshot time |
| `priceUSD` | float32 | Price in USD |
| `marketCapUSD` | float32 | Market cap in USD |
| `volumeUSD` | float32 | 24h volume in USD |

**Index**: `timestamp` descending.

#### `walletCount`
Total non-contract address count.

| Field | Type | Description |
|-------|------|-------------|
| `_id` | string | `"current_count"` |
| `count` | int64 | Number of non-contract addresses |
| `timestamp` | Date | Last update time |

**Schema validation** enforced.

#### `dailyTransactionsVolume`
24-hour transaction volume.

| Field | Type | Description |
|-------|------|-------------|
| `type` | string | `"daily_volume"` |
| `volume` | float64 | Total QRL transferred in 24h |
| `timestamp` | string | Latest block timestamp (hex) |
| `transferCount` | int | Number of transfers |

**Schema validation** enforced.

#### `totalCirculatingQuanta` / `totalCirculatingSupply`
Total circulating supply.

| Field | Type | Description |
|-------|------|-------------|
| `_id` | string | `"totalBalance"` |
| `circulating` | string | Total balance as decimal string |

#### `averageBlockSize` / `blockSize`
Block size history for charts. Rebuilt periodically from `blocks` collection via aggregation pipeline.

| Field | Type | Description |
|-------|------|-------------|
| `blockNumber` | string | Block number (hex) |
| `timestamp` | string | Block timestamp (hex) |
| `size` | string | Block size (hex) |
| `transactionCount` | int | Number of transactions |

### 3.5 Sync State Collections

#### `sync_state`
Tracks the synchronizer's progress.

| Field | Type | Description |
|-------|------|-------------|
| `_id` | string | `"last_synced_block"` |
| `block_number` | string | Hex block number of last processed block |

#### `sync_initial_state`
Records the starting block of the initial sync (used for token processing range).

| Field | Type | Description |
|-------|------|-------------|
| `_id` | string | `"initial_sync_start"` |
| `block_number` | string | Hex block number (typically `"0x1"`) |

#### `syncer_leases`
Coordinates the exclusive chain-indexer writer across processes.

| Field | Type | Description |
|-------|------|-------------|
| `_id` | string | `"chain-indexer"` |
| `owner` | string | Random process identity holding the lease |
| `generation` | long | Monotonic ownership generation |
| `expiresAt` | date | UTC lease expiry renewed by the active syncer |
| `releasedAt` | date | Mongo server time of a clean or compute-fenced release |
| `releasedGeneration` | long | Generation whose release was acknowledged |

#### `balance_reconciliations`
Durable reorg work for native, ERC-20, ERC-721, and ERC-1155 snapshots that must be refreshed from canonical RPC state.

| Field | Type | Description |
|-------|------|-------------|
| `_id` | string | Deterministic hash of the balance identity |
| `kind` | string | `native`, `erc20`, `erc721`, or `erc1155` |
| `address` | string | Native address when `kind` is `native` |
| `contractAddress` | string | Token contract for token kinds |
| `holderAddress` | string | Holder for ERC-20 and ERC-1155 |
| `tokenID` | string | Canonical decimal uint256 for ERC-721 and ERC-1155 |
| `rollbackTo` | string | Reorg rollback target that created or refreshed the work |
| `availableAt` | date | Mongo server-time retry or crash-reclaim eligibility |
| `claimToken` | string | Random ownership token required for failure or completion |
| `claimExpiresAt` | date | Server-time claim deadline |
| `attempts` | int | Durable failed-attempt count |
| `lastError` | string | Bounded diagnostic from the last failed attempt |

The queue uses fair availability ordering and bounded exponential backoff, so persistent RPC failures cannot starve newer invalidations. Public readers hide the stale snapshots until an ownership-safe refresh completes.

### 3.6 Mempool Collection

#### `pending_transactions`
Transactions currently in the node's mempool.

| Field | Type | Description |
|-------|------|-------------|
| `_id` | string | Transaction hash |
| `from` | string | Sender address |
| `to` | string | Recipient address |
| `value` | string | Value (hex) |
| `gas` | string | Gas limit (hex) |
| `gasPrice` | string | Gas price (hex) |
| `maxFeePerGas` | string | EIP-1559 max fee (optional) |
| `maxPriorityFeePerGas` | string | EIP-1559 priority fee (optional) |
| `input` | string | Input data |
| `nonce` | string | Nonce (hex) |
| `type` | string | Transaction type |
| `chainId` | string | Chain ID |
| `lastSeen` | Date | Last time seen in mempool |
| `status` | string | `"pending"`, `"mined"`, or `"dropped"` |
| `createdAt` | Date | First seen timestamp |

### 3.7 Other Collections

#### `coinbase`
Block proposer rewards.

| Field | Type | Description |
|-------|------|-------------|
| `blockhash` | string | Block hash |
| `blocknumber` | uint64 | Block number |
| `from` | string | Proposer address |
| `blockproposerreward` | uint64 | Proposer reward |
| `attestorreward` | uint64 | Attestor reward |
| `feereward` | uint64 | Fee reward |

---

## 4. RPC Calls

The synchronizer communicates with two external APIs:

### 4.1 Zond Execution Layer (JSON-RPC via `NODE_URL`)

All calls go through a shared HTTP client with connection pooling (100 max idle connections, 30s timeout).

| RPC Method | File | Purpose |
|------------|------|---------|
| `qrl_blockNumber` | `rpc/calls.go` | Get latest block number |
| `qrl_getBlockByNumber` | `rpc/calls.go` | Fetch full block with transactions (`true` for full tx objects) |
| `qrl_getTransactionReceipt` | `rpc/calls.go`, `rpc/tokenscalls.go` | Get tx receipt (contract address, status, logs) |
| `qrl_getBalance` | `rpc/calls.go` | Get address balance |
| `qrl_getCode` | `rpc/calls.go` | Get contract bytecode |
| `qrl_call` | `rpc/calls.go`, `rpc/tokenscalls.go` | Call contract method (read-only) |
| `qrl_getLogs` | `rpc/calls.go` | Get Transfer logs using an exact block-hash selector |
| `qrl_getTransactionByHash` | `rpc/calls.go` | Get transaction details by hash |
| `debug_traceTransaction` | `rpc/calls.go` | Trace internal calls (callTracer) |
| `txpool_content` | `rpc/pending.go` | Get mempool pending/queued transactions |

### 4.2 Beacon Chain API (HTTP REST via `BEACONCHAIN_API`)

| Endpoint | File | Purpose |
|----------|------|---------|
| `GET /qrl/v1alpha1/validators` | `rpc/calls.go` | Fetch validator list (paginated, up to 3 pages) |
| `GET /qrl/v1alpha1/beacon/chainhead` | `rpc/calls.go` | Get current chain head (epoch, slot, finality) |

### 4.3 CoinGecko API

| Endpoint | File | Purpose |
|----------|------|---------|
| `GET /api/v3/coins/quantum-resistant-ledger` | `fetch/coingecko.go` | Market data (price, market cap, volume) |

### 4.4 ERC20 Token Method Calls (`rpc/tokenscalls.go`)

These are made via `qrl_call` to detect and query ERC20 tokens:

| Method Signature | Function | Purpose |
|-----------------|----------|---------|
| `0x06fdde03` | `name()` | Get token name |
| `0x95d89b41` | `symbol()` | Get token symbol |
| `0x313ce567` | `decimals()` | Get token decimals |
| `0x70a08231` | `balanceOf(address)` | Get token balance for holder |
| `0x18160ddd` | `totalSupply()` | Get total token supply |
| `0x32668b54` | `maxSupply()` | Custom: max supply |
| `0x94303c2d` | `maxTxAmount()` | Custom: max transaction amount |
| `0x41d3014e` | `maxWalletSize()` | Custom: max wallet size |
| `0x8da5cb5b` | `owner()` | Custom: contract owner |

---

## 5. Token Transfer Detection and Indexing

Token transfer detection is a two-phase process that runs after block sync.

### 5.1 Phase 1: Queue Potential Token Contracts

During `ProcessTransactions()` for each block:
1. For each transaction, `processContracts()` checks if it's a contract creation (empty `to` field) or interaction with an existing contract.
2. If a contract is involved, `QueuePotentialTokenContract()` writes an entry to the `pending_token_contracts` collection with `processed: false`.

### 5.2 Phase 2: Process Queued Contracts

`ProcessTokenTransfersFromTransactions()` drains the legacy per-transaction queue after transaction processing:
1. Atomically claims one eligible unprocessed entry with a random token, Mongo server-time deadline, and incremented attempt count.
2. Checks that the queued block is still represented by exactly one canonical `ingestionState: "complete"` block and that the receipt identity matches it.
3. Marks the legacy item processed after exact receipt and canonical block validation. Transfer, classification, metadata, and balance effects are owned exclusively by the durable block-level worker, which avoids two queues mutating the same projection out of order.
4. Failure releases the item with bounded exponential backoff. A periodic worker retries eligible items even when the chain head is idle.

### 5.3 Durable Block-Level Token Processing

`ProcessBlockTokenTransfers()` receives one exact complete block and runs behind a durable claim stored on that block:

1. Claim one `tokenIngestionState: "pending"` block using Mongo server time, a random owner token, and a crash-reclaim deadline.
2. Call `qrl_getLogs` with `blockHash` and the Transfer, TransferSingle, and TransferBatch signatures.
3. Validate every returned log identity before any token effect is written: `removed` is false, block number and hash match, the transaction belongs to the stored block, transaction and log indexes are canonical and unique, and the emitter is a valid QIP-55 address.
4. Classify each emitter against the exact block state. RPC and Mongo failures remain retryable errors. A successful probe with no recognized token standard skips that emitter's matching ABI-shaped logs.
5. Decode every log for confirmed token standards using canonical ERC-20, ERC-721, and ERC-1155 payload rules. Deterministic shape or decode failures are idempotently written to `tokenEventDeadLetters`, then later block heights can continue. A dead-letter write failure remains retryable and keeps the block pending.
6. After planning all logs, idempotently upsert transfer rows and refresh token balance, ownership, and metadata state using exact-block RPC calls.
7. Reload the exact unique block and compare its transaction snapshot. Mark token ingestion complete only while the same claim token and block identity are still owned.

A failure releases the claim with bounded exponential backoff. A crash leaves the claim reclaimable after its server-time lease. Claims are strictly height ordered: an older retry stalls later token effects so historical replay cannot overwrite a newer balance or ownership projection. The startup worker drains work immediately and every 30 seconds, including while the chain head is quiet. Balance reconciliation also waits for this queue to reach a complete prefix. Reorg rollback deletes the owning orphan block and its transfer rows, then queues affected balance identities for reconciliation.

### 5.4 Transfer Event Signature

```
keccak256("Transfer(address,address,uint256)")
= 0xddf252ad1be2c89b69c2b068fc378daa952ba7f163c4a11628f55a4df523b3ef
```

Log topics layout:
- `topics[0]`: event signature hash in the high 32 bytes of a 64-byte VM word, followed by 32 zero bytes
- `topics[1]`: complete 64-byte `from` address
- `topics[2]`: complete 64-byte `to` address
- `data`: Transfer amount in one 64-byte ABI word

### 5.5 Post-Initial-Sync Token Processing (`synchroniser/token_sync.go`)

After the initial block sync completes, `ProcessTokensAfterInitialSync()` queries complete blocks by numeric `blockNumberInt` and requests their durable token claims in batches. The independent startup worker also continuously drains pending or crash-reclaimable claims, so restart and quiet-head recovery do not depend on another block arriving.

---

## 6. Contract Detection and Metadata Extraction

### 6.1 New Contract Detection (`db/contracts.go`)

When a transaction has an empty `to` field (contract creation):
1. Calls `qrl_getTransactionReceipt` to get the deployed contract address and status.
2. Calls `qrl_getCode` to fetch the contract bytecode.
3. Calls `GetTokenInfo()` which sequentially tries `name()`, `symbol()`, and `decimals()`. If all three succeed, the contract is flagged as an ERC20 token.
4. If it's a token, fetches `totalSupply()`.
5. Stores everything in the `contractCode` collection via `StoreContract()`.

### 6.2 Existing Contract Detection

When a transaction targets an existing address, `IsAddressContract()`:
1. Checks the `contractCode` collection in MongoDB.
2. If not found, calls `qrl_getCode` via RPC.
3. If code exists (not `0x` or empty), it's a contract - stores it with token detection.

### 6.3 Contract Reprocessing (`db/contracts.go`)

`StartContractReprocessingJob()` runs every 1 hour:
- Queries for contracts with missing information (empty code, tokens without total supply, non-tokens without name/symbol).
- Re-fetches data from the node for each incomplete contract.
- Preserves existing creation information (creator address, creation tx, creation block).

### 6.4 Token Detection (`db/token_detection.go`)

The `DetectToken()` function provides a clean API:
```go
func DetectToken(contractAddress string) TokenDetectionResult {
    name, symbol, decimals, isToken := rpc.GetTokenInfo(contractAddress)
    // Also fetches totalSupply if isToken
}
```

`EnsureTokenInDatabase()` is the consolidated function that:
1. Detects if the contract is a token via RPC.
2. Gets or creates the contract entry in MongoDB.
3. Preserves existing creation information if updating.
4. Returns the contract info and whether it's a token.

---

## 7. Validator Data Synchronization

### 7.1 Sync Flow

Every 6 hours (`updateValidatorsPeriodically`):

1. **Get chain head**: `GET /qrl/v1alpha1/beacon/chainhead` returns current epoch, slot, and finality info. Stored in the `epoch_info` collection.

2. **Fetch validators**: `GET /qrl/v1alpha1/validators` with pagination (up to 3 pages). Each page contains a list of validators with their details.

3. **Store validators** (`services/validator_service.go`):
   - Converts base64-encoded public keys and withdrawal credentials to hex.
   - Determines leader status (simplified: `index % 128 == 0`).
   - Merges with existing validators (updates mutable fields, adds new ones).
   - All validators stored in a single document with `_id: "validators"`.

4. **Store history** (`services/validator_service.go`):
   - Computes per-epoch statistics: active, pending, exited, slashed counts.
   - Calculates total staked by summing effective balances.
   - Upserts into `validatorHistory` keyed by epoch.

### 7.2 Epoch Calculation

```go
currentEpoch = latestBlockNumber / 128  // 128 slots per epoch
```

### 7.3 Validator Status Logic

```go
func GetValidatorStatus(activationEpoch, exitEpoch string, slashed bool, currentEpoch int64) string {
    if slashed { return "slashed" }
    if activation > currentEpoch { return "pending" }
    if exit <= currentEpoch { return "exited" }
    return "active"
}
```

---

## 8. Mempool / Pending Transaction Handling

### 8.1 Three Concurrent Services

Started by `StartPendingTransactionSync()` at application startup:

| Service | Interval | Function |
|---------|----------|----------|
| Mempool sync | 1 second | `syncMempool()` |
| Old tx cleanup | 1 hour | `CleanupOldPendingTransactions(24h)` |
| Pending verification | 5 minutes | `verifyPendingTransactions()` |

### 8.2 Mempool Sync (`syncMempool`)

1. Calls `txpool_content` RPC (uses `MEMPOOL_NODE_URL` if set, else `NODE_URL`).
2. Parses the nested response format: `{pending: {address: {nonce: tx}}, queued: {address: {nonce: tx}}}`.
3. Upserts each transaction into `pending_transactions` with `status: "pending"`.
4. Processes both `pending` and `queued` pools.

### 8.3 Pending Transaction Lifecycle

```
txpool_content -> UpsertPendingTransaction (status: "pending")
                       |
                       v
                 Block mined with tx
                       |
                       v
         UpdatePendingTransactionsInBlock (status: "mined")
                       |
                       v
         verifyPendingTransactions -> DeletePendingTransaction
                                     (if receipt exists)
                       |
                       v (if not mined)
         CleanupOldPendingTransactions
         (delete if lastSeen > 24 hours ago)
```

### 8.4 Block-Level Pending Update

When a new block is processed (`UpdatePendingTransactionsInBlock`):
1. Creates a map of all transaction hashes in the block.
2. Queries all pending transactions.
3. For any match, updates `status` to `"mined"` and records the block number.

---

## 9. Configuration

### 9.1 Environment Variables

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| `MONGOURI` | Yes | -- | Transaction-capable MongoDB URI without the database name, for example `mongodb://localhost:27017/?replicaSet=rs0` |
| `NODE_URL` | Yes | `http://localhost:8545` | Zond execution layer JSON-RPC endpoint |
| `MEMPOOL_NODE_URL` | No | Falls back to `NODE_URL` | Separate RPC for mempool access |
| `ENABLE_DEBUG_TRACE` | No | `false` | Enable indexing of native transfers made by contract sub-calls |
| `TRACE_NODE_URL` | No | Primary `NODE_URL` | Private debug-enabled HTTP(S) or WS(S) endpoint used for transaction traces |
| `BEACONCHAIN_API` | Yes | `http://localhost:3500` | Beacon chain HTTP API endpoint |
| `HEALTH_PORT` | No | `8083` | Health endpoint port. Docker and Kubernetes set this to `8081`. |
| `RPC_DELAY_MS` | No | `50` | Delay between RPC calls in ms |
| `RPC_DELAY_JITTER_MS` | No | `26` | Random jitter added to RPC delay |

### 9.2 Sync Constants (`synchroniser/producer_consumer.go`)

| Constant | Value | Description |
|----------|-------|-------------|
| `DefaultBatchSize` | 64 | Normal batch size for block fetching |
| `LargeBatchSize` | 128 | Batch size when >1000 blocks behind |
| `BatchSyncThreshold` | 64 | Blocks behind before switching to batch mode |
| `LargeSyncThreshold` | 1000 | Blocks behind before using large batch size |
| `MaxProducerConcurrency` | 8 | Max concurrent block-fetching goroutines |

### 9.3 Mempool Constants (`synchroniser/pending_sync.go`)

| Constant | Value | Description |
|----------|-------|-------------|
| `MEMPOOL_SYNC_INTERVAL` | 1 second | Mempool polling frequency |
| `CLEANUP_INTERVAL` | 1 hour | Old pending tx cleanup frequency |
| `VERIFY_PENDING_INTERVAL` | 5 minutes | Pending tx verification frequency |
| `MAX_PENDING_AGE` | 24 hours | Max age before pending tx is cleaned up |

### 9.4 Gap Detection Constants (`synchroniser/gap_detection.go`)

| Constant | Value | Description |
|----------|-------|-------------|
| `MaxGapDetectionBlocks` | 1000 | Maximum blocks to scan for gaps |
| `GapRetryAttempts` | 3 | Max retry attempts for failed blocks |

### 9.5 Token Sync Defaults (`synchroniser/token_sync.go`)

| Setting | Value | Description |
|---------|-------|-------------|
| `BatchSize` | 10 | Blocks per token processing batch |
| `BatchDelayMs` | 86 | Delay between batches (ms) |
| `QueryTimeoutSec` | 30 | Timeout for block queries (seconds) |

### 9.6 Other Constants (`configs/const.go`)

| Constant | Value | Description |
|----------|-------|-------------|
| `QUANTA` | 1e18 | Wei-to-QRL divisor |
| `QRLZeroAddress` | `Q000...000` | Zero address (128 hex chars + Q) |
| `LOG_FILENAME` | `zond_sync.log` | Log file name (in `logs/` directory) |

---

## 10. How to Build and Run

### 10.1 Prerequisites

- Go 1.24+
- MongoDB replica set or sharded cluster with a writable primary. The database is created automatically.
- QRL Zond node (execution layer + beacon chain)

The native `deploy.sh` path expects a writable single-node replica set named `rs0` on `127.0.0.1:27017`. It validates this topology before stopping services or offering database cleanup. The script leaves topology migration to an explicit maintenance procedure.

For a new native single-node installation, configure `mongod` with `replication.replSetName: rs0`, keep its network bind restricted to the intended hosts, start it, and initialize it once:

```bash
mongosh --quiet "mongodb://127.0.0.1:27017/admin?directConnection=true" \
  --eval 'rs.initiate({_id:"rs0",members:[{_id:0,host:"127.0.0.1:27017"}]})'

mongosh --quiet "mongodb://127.0.0.1:27017/admin?replicaSet=rs0" \
  --eval 'const s=rs.status(); printjson({set:s.set,myState:s.myState,ok:s.ok})'
```

Back up and schedule maintenance before converting an existing standalone deployment. Review the advertised member hostname from every application host before initializing it. Native application URIs must use the same replica-set name.

`deploy.sh` preserves existing component `.env` files. Before deploying an existing installation, update the backend and frontend database URIs to include `/qrldata-z?replicaSet=rs0`, and update the syncer URI to include `/?replicaSet=rs0`.

### 10.2 Build

```bash
cd QRL2MongoDB

# Download dependencies
go mod download

# Build
go build -o synchroniser main.go
```

### 10.3 Configure

Create a `.env` file from the example:

```bash
cp .env.example .env
```

Edit `.env`:
```env
MONGOURI=mongodb://localhost:27017/?replicaSet=rs0
NODE_URL=http://localhost:8545
MEMPOOL_NODE_URL=http://localhost:8545
BEACONCHAIN_API=http://localhost:3500
```

### 10.4 Run

```bash
./synchroniser
```

The synchronizer will:
1. Connect to MongoDB and create/validate all collections with indexes and schema validators.
2. Start the health check server on port 8081.
3. Start mempool sync (background).
4. Begin block synchronization from the last known block.
5. After catching up, enter continuous monitoring mode.

Logs are written to both stdout and `logs/zond_sync.log`.

### 10.5 Docker

```bash
# Build
docker build -t zond-syncer .

# Run
docker run -e 'MONGOURI=mongodb://mongo:27017/?replicaSet=rs0' \
           -e NODE_URL=http://node:8545 \
           -e BEACONCHAIN_API=http://beacon:3500 \
           zond-syncer
```

The Dockerfile uses a two-stage build (Go 1.24-alpine builder, alpine runner) with a non-root user (UID 1000).

### 10.6 Kubernetes

The MongoDB StatefulSet starts `mongod` with `--replSet rs0`. Its readiness probe initializes an uninitialized database with the stable `mongodb-0.mongodb.zond-explorer.svc.cluster.local` member name, then reports ready only when that exact member is the writable `rs0` primary. The probe rejects existing replica configurations with another set name or member identity. The headless Service publishes the pod before readiness so that stable member name resolves during initialization.

The syncer Deployment uses `replicas: 1` and the `Recreate` strategy. Keep both settings. The in-database lease provides a second exclusivity check and causes another syncer process to fail during startup.

The `/health` endpoint uses `HEALTH_PORT` (`8083` by default; deployments set `8081`) for liveness/readiness probes.

---

## 11. Reindexing Scripts

Located in `scripts/`, these Python scripts are used for one-time reindexing operations when the database needs to be repaired or backfilled.

### 11.1 `reindex_contracts.py`

**Purpose**: Scans the `transfer` collection for contract creation transactions (those with a `contractAddress` field) and rebuilds the `contractCode` collection.

**What it does**:
1. Connects to MongoDB and the Zond RPC node.
2. Queries `transfer` collection for documents with `contractAddress`.
3. For each contract creation:
   - Gets the contract bytecode via `qrl_getCode`.
   - Calls `name()`, `symbol()`, `decimals()` to detect ERC20 tokens.
   - Upserts the contract data into `contractCode`.

**Usage**:
```bash
cd scripts
pip install -r requirements.txt
MONGOURI=mongodb://localhost:27017/?replicaSet=rs0 NODE_URL=http://localhost:8545 python reindex_contracts.py
```

### 11.2 `reindex_tokens.py`

**Purpose**: Rebuilds the `tokenTransfers` and `tokenBalances` collections by replaying Transfer event logs from the blockchain.

**What it does**:
1. Creates indexes on `tokenTransfers` collection.
2. Updates contracts with missing `creationBlockNumber` by looking up their creation transaction receipt.
3. For each token contract in `contractCode` where `isToken: true`:
   - Fetches Transfer event logs from the creation block to the latest block (in batches of 50 blocks).
   - Parses event topics to extract `from`, `to`, and `amount`.
   - Stores transfers in `tokenTransfers` (skips duplicates).
   - Calculates running balances and updates `tokenBalances`.

**Dependencies** (in addition to `requirements.txt`):
- `web3` (for hex-to-int conversion)

**Usage**:
```bash
cd scripts
pip install -r requirements.txt
pip install web3
MONGOURI=mongodb://localhost:27017/?replicaSet=rs0 NODE_URL=http://localhost:8545 python reindex_tokens.py
```

---

## 12. Key Implementation Details

### 12.1 Concurrency Model

- **Exclusive process lease** in `syncer_leases` permits one active syncer generation for a database. The two-minute lease is renewed every 40 seconds. Clean release is required before a new generation can acquire, so TTL expiry cannot create split brain. Crash recovery requires explicit compute fencing and exact owner/generation acknowledgement.
- **Chain mutation mutex** serializes local block insertion, companion writes, contract reprocessing, sync-state updates, and rollback.
- **Producer/consumer pattern** for batch block fetching. Producers run as goroutines, limited to 8 concurrent via a channel-based semaphore (`producerSem`).
- **Atomic operations** (`sync/atomic`) used to track the highest processed block number across goroutines.
- **Mutex** (`sync.Mutex`) protects sync state updates during consumer processing.
- **sync.Map** used for failed block tracking (lock-free concurrent map).
- **WaitGroups** coordinate goroutine completion.

### 12.2 RPC Rate Limiting

Two delay modes:
- **Normal**: 50ms + random(0-26ms) jitter between calls.
- **Bulk sync**: 1/10th of normal (min 5ms) for faster initial sync.

Delays are configurable via `RPC_DELAY_MS` and `RPC_DELAY_JITTER_MS` environment variables.

### 12.3 Retry Logic

| Operation | Retries | Backoff |
|-----------|---------|---------|
| Get latest block (initial) | 5 | Exponential (1s, 2s, 4s, 8s, 16s) |
| Block fetch (producer) | 3 | Linear (100ms, 200ms, 300ms) |
| Block fetch (single) | 3 | Linear (500ms, 1000ms, 1500ms) |
| Periodic tasks | 5 | Exponential (1s, 2s, 4s, 8s, 16s) |
| Token balance RPC | 3 | Linear (500ms, 1000ms, 1500ms) |
| CoinGecko fetch | 3 | Exponential with jitter (30s base, max 5min) |

### 12.4 Chain Reorg Handling

In `processSubsequentBlocks()`:
1. After fetching a new block, checks if `block.parentHash` matches the hash of the previous block stored in MongoDB.
2. If there's a mismatch, calls `Rollback()` which:
   - Captures the removed blocks, transactions, and created contracts in one MongoDB transaction snapshot.
   - Deletes the blocks and their append-only companion records, contract trust rows, and pending verification jobs.
   - Removes only mined pending-transaction tombstones for orphaned transaction hashes so canonical mempool entries can return.
   - Marks affected native and ERC-20/721/1155 snapshots stale and enqueues canonical RPC reconciliation in the same transaction. Native identities include transaction and internal-call endpoints, created contracts, the block miner, and withdrawal beneficiaries.
   - Clears contract flags on affected address records and resets the sync high-water mark to the rollback target.
3. Returns the parent block number so the sync loop reprocesses from there.

Rollback requires MongoDB transactions. Startup rejects a standalone topology so a reorg cannot begin a partial multi-collection cleanup.

### 12.5 Address Format

QRL Zond uses two address formats:
- **Legacy**: `0x` prefix (standard Ethereum format)
- **Zond native**: `Q` prefix

The synchronizer normalizes addresses:
- Stored in MongoDB as **lowercase** (via `strings.ToLower()`).
- The `validation` package handles both formats.
- `ConvertToQAddress()` converts `0x` to `Q` prefix.

### 12.6 Hex Number Handling

All block numbers, timestamps, gas values, and amounts from the Zond node come as non-zero-padded hex strings (e.g., `0x1a`, not `0x0000001a`). The `utils` package provides:
- `HexToInt()` / `IntToHex()` - Conversion using `math/big`.
- `CompareHexNumbers()` - Proper numeric comparison (not lexicographic).
- `AddHexNumbers()` / `SubtractHexNumbers()` - Hex arithmetic.

This is critical because MongoDB lexicographic comparison of hex strings produces incorrect results for different-length strings (e.g., `0x9` > `0x10` lexicographically). The `getBlocksWithTransactions()` function in `token_sync.go` handles this by filtering in Go rather than MongoDB.

### 12.7 Transaction Fee Calculation

For each transaction:
1. Gets `gasPrice` from the transaction.
2. Gets `gasUsed` from `debug_traceTransaction` or falls back to the transaction receipt's `gasUsed`, or the gas limit.
3. Fee = `gasPrice * gasUsed / 1e18` (converted from wei to QRL).
4. If fee is zero for a successful transaction (`status: 0x1`), sets a minimum fee of `0.000001 QRL`.

### 12.8 Logging

Uses [uber-go/zap](https://github.com/uber-go/zap) structured logging:
- Console encoding with custom time format (`Jan  2 15:04:05`).
- Writes to both `logs/zond_sync.log` and stdout.
- Debug level enabled.
- All periodic tasks have panic recovery with automatic restart after 5 seconds.

### 12.9 Health Check

A simple HTTP health endpoint at `/health` (default port 8083) returns:
```json
{"status":"ok"}
```
Used for Kubernetes liveness/readiness probes and Docker health checks.

### 12.10 Duplicate Prevention

- **Blocks**: fetched height, envelope identity, deterministic ordering, parent continuity, and same-height identity are validated before durable insertion. Any duplicate height is rejected, and `blockNumberInt_desc_idx` enforces uniqueness. Post-write identity reads classify the confirmed prefix after partial or uncertain batch outcomes.
- **Block visibility**: `ingestionState: "pending"` remains hidden. The exact block becomes `complete` only after every companion write succeeds.
- **Token transfer visibility**: the owning block must be uniquely core-complete and token-complete, and the transfer `blockHash` must exactly match that block. Hashless legacy rows remain hidden until replay repairs them.
- **Token transfers**: `(txHash, contractAddress, logIndex, tokenID)` is the idempotent event identity.
- **Token balances**: `(contractAddress, holderAddress, tokenID)` compound identity with standard-aware upsert.
- **Sync state**: Only updates if new block number is higher than existing.
- **Pending contracts**: canonical `(contractAddress, txHash)` compound identity with owner-token claims.

### 12.11 Collection Initialization

`configs.ConnectDB()` performs a read-only connection and topology check. After the exclusive lease is acquired, `configs.BootstrapDB()`:
1. Creates collections with JSON schema validators for `dailyTransactionsVolume`, `coingecko`, `priceHistory`, `walletCount`, `totalCirculatingSupply`, and `tokenBalances`.
2. Creates compound and unique indexes for `blocks`, `tokenBalances`, `pending_token_contracts`, `tokenTransfers`, `tokenEventDeadLetters`, `priceHistory`, and `transfer`. An existing non-unique block-height index is upgraded only by the offline migration after its duplicate audit.
3. Initializes the `sync_state` collection with `block_number: "0x0"` if empty.
4. Initializes CoinGecko collection with zero values.
