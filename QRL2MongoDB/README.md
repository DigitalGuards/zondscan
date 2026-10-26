# QRL2MongoDB

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
ENABLE_DEBUG_TRACE=false               # Set true to index internal native transfers
# TRACE_NODE_URL=ws://127.0.0.1:8546   # Private debug-enabled RPC endpoint
BEACONCHAIN_API=http://beaconnodehttpapi:3500
```

**Note:** `MEMPOOL_NODE_URL` is optional. If not set, it falls back to `NODE_URL`. This is useful when using a public RPC for block sync but a local node (with txpool access) for mempool detection.

### Internal transaction tracing

Internal native transfers require `debug_traceTransaction` with `callTracer`.
Public RPC endpoints should keep the `debug` namespace disabled because traces
are resource intensive. Run a separate loopback-only WebSocket listener on the
execution node and configure the syncer to use it:

```text
--ws --ws.addr=127.0.0.1 --ws.port=8546 --ws.api=debug
```

```env
ENABLE_DEBUG_TRACE=true
TRACE_NODE_URL=ws://127.0.0.1:8546
```

`TRACE_NODE_URL` accepts `http`, `https`, `ws`, and `wss`. When it is absent,
the syncer uses the primary `NODE_URL` for compatibility. With tracing enabled,
startup fails if the selected endpoint does not expose `debug`; this prevents a
configuration change from silently creating permanent gaps in internal
transactions.

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

### Syncer lease recovery

The syncer holds the exclusive `chain-indexer` lease in MongoDB's
`syncer_leases` collection. A new generation requires a release acknowledgement.
Expiry alone leaves the lease held because a suspended process could resume
and continue writing.

New owners use the format
`pid-<pid>-<random>;machine=<sha256-prefix>;boot=<boot-id>`. The machine field
contains the first 32 hexadecimal characters of the SHA-256 digest of the
trimmed `/etc/machine-id`; the boot field comes from
`/proc/sys/kernel/random/boot_id`. Both values must be readable and valid.
Otherwise, the owner retains the `pid-<pid>-<random>` format and recovery
requires an operator. Each machine must have a unique machine ID.

After a refused acquisition, the syncer reads the lease once. It automatically
acknowledges an expired lease when the recorded machine identity matches its
own and the recorded boot identity differs from the current boot. This proves
that the old process belonged to a previous boot of the same machine. MongoDB
server time determines expiry. The acknowledgement matches the exact owner and
generation and checks expiry again, then the syncer attempts normal acquisition
once. A log entry records the old owner, generation, and reboot reason. The
existing acquisition filter keeps concurrent syncers from both taking the lease.

Old-format or unknown owners, a different machine, the same boot, an unexpired
lease, unavailable local identity, and recovery read or update errors retain
the existing lease-held error. An unexpired lease can become eligible after
expiry on a subsequent startup. Other cases require operator recovery after
proving that the old process cannot execute again. A same-boot process crash
requires that manual proof.

To acknowledge an expired lease manually, inspect its exact `owner` and
`generation` in `syncer_leases`, establish the compute fence, and run:

```bash
go run ./cmd/recover-syncer-lease \
  --owner '<inspected-owner>' \
  --generation '<inspected-generation>' \
  --compute-fenced
```

The operator command supports every owner format and machine identity. It
requires an expired lease and an exact owner/generation match; a changed lease
must be inspected again.

## Key Components

### Beacon chain timing and validator history

Startup reads `/qrl/v1/config/spec` (`SLOTS_PER_EPOCH` and `SECONDS_PER_SLOT`)
and `/qrl/v1/beacon/genesis` (`genesis_time`) from `BEACONCHAIN_API`. String
numbers are validated, requests have a 10-second timeout and an 8 MiB response
cap, and startup discovery has a 25-second deadline. The leased writer saves
numeric `slotsPerEpoch`, `secondsPerSlot` and `genesisTime` in `sync_state` under
`_id: "chain_timing"`. A beacon outage logs a warning and uses the stored clock.
Without a stored clock, execution indexing starts while beacon workers wait.
Discovery retries every 30 seconds, persists timing, and wakes the waiting
workers as soon as it succeeds. It also verifies a cached clock after recovery.
An existing clock or slot-assignment genesis that disagrees causes a hard startup
failure, or orderly shutdown if discovered during retry. Invalid stored timing
also fails closed. Retry and waiting workers honor shutdown and drain before
the chain-indexer lease is released.

The backend reads this document for epoch slot ranges and execution timestamp
mapping. `GET /epoch` (public `/api/epoch`) exposes the same three timing fields
alongside the beacon head and finality. The frontend uses them for block epochs,
validator ages, and network examples in Learn. The staking calculator and
standard-parameter explanations retain qrysm mainnet economics: 128 slots of
60 seconds per epoch. Execution block heights remain independent of beacon slots
across missed proposals. Gas and pending transaction ETA responses keep serving
their available data when timing is missing; timing fields are null when neither
recent block intervals nor stored timing can supply a value.

A live worker refreshes `/qrl/v1/beacon/headers/head` and
`/qrl/v1/beacon/states/head/finality_checkpoints` every 30 seconds. The current
epoch is `headSlot / slotsPerEpoch`. It labels the live validator set fetched
at that head slot with this epoch. The live set has one document per validator
and is kept separately from historical summaries.

The history worker fetches `/qrl/v1/beacon/states/{epoch * slotsPerEpoch}/validators`
for every epoch from zero through the finalized checkpoint, inclusive. Each
`validator_history` record has `source: "beacon-state"`, a decimal-string `epoch`,
numeric `epochInt`, and timestamp
`genesisTime + epoch * slotsPerEpoch * secondsPerSlot`. Counts use the beacon
statuses in four exclusive buckets: pending, active, exited (including withdrawal
statuses), and slashed. The slashed flag takes precedence throughout a validator's
lifecycle. `totalStaked` sums every effective balance exactly in Shor.

The worker starts at the newest finalized epoch and walks backward to zero.
It handles at most 16 epochs per pass and checks finality between passes. A
separate descending catch-up range prioritizes new finalized epochs before
continuing the older backlog. `sync_state` document `_id: "validator_history"`
stores `backfillNext`, `newestScheduled`, `catchupNext`, and `catchupTarget`;
`-1` marks a finished descending range. Each summary is upserted by epoch before
progress is saved, so a crash between writes safely replays that epoch. A unique
epoch index prevents duplicate records. Unavailable states, malformed responses,
and transient failures retain the cursor for retry after 30 seconds. Historical
state access through epoch zero is required for a complete rebuild. Newly
finalized epochs can still fill while an older state remains unavailable.

All beacon workers share a one-request-per-second limiter. History and cursor
writes require the chain-indexer lease and use write deadlines ending before
lease expiry. Shutdown cancels requests and drains registered writers before
releasing the lease. Once caught up, the history worker polls every 30 seconds.

On the first run, the API immediately excludes old unmarked records from both
the epoch list and validator history. As the worker reaches each old epoch, its
upsert replaces the old summary and adds the marker. No collection deletion or
block reindex is needed. Recent pages fill first, usually within tens of seconds.
Visitors see a growing list ordered by chain epoch; epoch details show the known
clock time and dashes for unavailable validator summaries, including recent
unfinalized epochs. Known zero counts remain zero. Epoch pages refresh every
30 seconds, and proposer duties fill independently.

For roughly 5,000 epochs, allow about 1.5 to 2 hours when proposer duties are
already caught up, or about 3 to 4 hours when both walkers have a full backlog.
These estimates assume healthy sub-second beacon responses. Retries, fallback
duty lookups, slow storage and missing historical states extend the duration.
Operators can follow the cursor fields and the count of `source: "beacon-state"`
records, plus retry logs containing the affected epoch. Start the updated syncer
before serving the updated API and frontend so chain timing is available.

### Finalized slot proposer assignments

The leased syncer writes `slotAssignments` documents with numeric `epoch` and
`slot`, decimal-string `validatorIndex`, and `0x`-prefixed hex `pubkey`. The
`slot_unique` index enforces one assignment per slot. An epoch index supports
the API lookup. Slot counts and durations come from `chain_timing`.

On first start, the worker copies the validated `chain_timing` genesis into
its `sync_state` cursor under `_id: "slot_assignments"`. It starts its `nextEpoch` cursor
at zero and checks epochs in ascending order. Only explicit unavailable-state
responses from the historical duty sources advance an unavailable epoch;
transport failures, rate limits, invalid responses, and incomplete duty sets
keep the cursor for retry. `firstAvailableEpoch` records the earliest complete
duty set and `unavailableEpochs` counts skipped unavailable epochs. On the node
checked for this change, both APIs served historical epochs including epoch
zero. Slot zero has no proposer duty, so its stored validator fields are empty.

The primary source is `/qrl/v1/validator/duties/proposer/{epoch}`. The fallback
is `/qrl/v1alpha1/validators/assignments?epoch=N`, with pagination and pubkeys
resolved through `/qrl/v1/beacon/states/head/validators/{index}`. Validator
indices retain their pubkeys after exit. Every HTTP request shares a one-request
per-second limit, a 10-second timeout, and an 8 MiB response cap. Pagination is
limited to 100 pages of 100 validators. Each pass handles up to 16 epochs.

The worker indexes duties through the finalized checkpoint epoch, inclusive,
refreshes the checkpoint between batches, and polls every 30 seconds
when caught up or retrying. It upserts the full validated duty set before saving
`nextEpoch`. A restart replays an interrupted epoch safely. A node with pruned
history can leave older slots with unknown proposers.

Index creation, assignment writes, and cursor updates require the active
`chain-indexer` lease and the existing chain mutation lock. Write deadlines end
before lease expiry. The worker participates in shutdown cancellation and
draining before the lease is released.

The backend derives execution block slots from
`(execution timestamp - genesisTime) / secondsPerSlot`, the timestamp relation
enforced by consensus. Startup creates `slot_timestamp_ingestion_idx` on
`blocks` for epoch slot timestamp lookups. This reads existing block rows
without a block reindex. Start the updated syncer before serving the updated
epoch endpoint so the beacon slot clock is available. Proposer fields appear
progressively as backfill advances; block mapping works once the clock is saved.

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
  - Stores one document per validator, keyed by its beacon index
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
