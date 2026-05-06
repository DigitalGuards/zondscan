# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

QRL Proof-of-Stake Explorer - A blockchain explorer for the Quantum Resistant Ledger (QRL) Zond network. Three main components sync blockchain data, serve it via REST API, and display it in a web UI.

## Architecture

```
┌─────────────────┐     ┌──────────────────┐     ┌───────────────┐     ┌──────────────┐
│  QRL Zond Node  │────▶│  QRL2MongoDB    │────▶│   MongoDB     │◀────│  backendAPI  │
│  (RPC :8545)    │     │  (synchronizer)  │     │  (qrldata-z)  │     │ (REST :8081) │
└─────────────────┘     └──────────────────┘     └───────────────┘     └──────────────┘
                                                                              │
                                                                              ▼
                                                                    ┌──────────────────┐
                                                                    │ ExplorerFrontend │
                                                                    │  (Next.js :3000) │
                                                                    └──────────────────┘
```

Note: Backend uses port 8081 locally since beacon chain occupies 8080. Production typically uses 8080 or proxied through nginx.

## Build & Run Commands

### Frontend (ExplorerFrontend/)
```bash
npm install           # Install dependencies
npm run dev           # Development server (port 3000)
npm run build         # Production build
npm start             # Production server
npm test              # Run tests
npm run lint          # Run linting
```

### Backend API (backendAPI/)
```bash
go mod download                    # Install dependencies
go build -o backendAPI main.go     # Build executable
./backendAPI                       # Run server (port 8080)
go test ./...                      # Run tests
```

### Synchronizer (QRL2MongoDB/)
```bash
go mod download                     # Install dependencies
go build -o synchroniser main.go    # Build executable
./synchroniser                      # Run synchronizer
```

### Full Stack Deployment (with PM2)
```bash
./deploy.sh                         # Full setup (Linux/macOS)
./deploy-windowsgitbash.sh          # Full setup (Windows Git Bash)
```

### Update Scripts
```bash
./update-backend.sh                 # Rebuild and restart backend + syncer
./update-frontend.sh                # Rebuild and restart frontend
```

### Docker Compose
```bash
docker compose up -d                # Start all services (MongoDB, backend, frontend, syncer)
docker compose ps                   # Check container status
docker compose logs -f syncer       # Follow syncer logs
docker compose down                 # Stop all services
```

Services run on: Frontend :3000, Backend :8082, MongoDB :27018

### Kubernetes
```bash
./scripts/k8s-deploy.sh             # Deploy to Kubernetes cluster
./scripts/k8s-deploy.sh --delete    # Remove deployment
kubectl get pods -n zond-explorer   # Check pod status
```

Key K8s files:
- `k8s/configmap.yaml` - Environment config (NODE_URL, BEACONCHAIN_API)
- `k8s/secrets.yaml` - MongoDB credentials (base64 encoded)
- `k8s/ingress.yaml` - External routing (update domain for production)
- `k8s/mongodb/statefulset.yaml` - MongoDB with persistent storage

## Key Environment Variables

**Frontend (.env):**
- `DATABASE_URL` - MongoDB connection string
- `NEXT_PUBLIC_HANDLER_URL` - Backend API URL (e.g., http://localhost:8080)
- `NEXT_PUBLIC_DOMAIN_NAME` - Frontend domain

**Backend (.env):**
- `MONGOURI` - MongoDB connection string
- `NODE_URL` - Zond RPC endpoint (e.g., http://localhost:8545)
- `HTTP_PORT` - API port (default :8080)

**Synchronizer (.env):**
- `MONGOURI` - MongoDB connection string (without database name)
- `NODE_URL` - Zond RPC endpoint for block sync
- `MEMPOOL_NODE_URL` - (Optional) Separate RPC endpoint for mempool/pending tx detection. Falls back to NODE_URL if not set. Useful when using a public RPC for blocks but local node for mempool.
- `BEACONCHAIN_API` - Beacon chain HTTP API endpoint

## MongoDB Collections (database: qrldata-z)

Core data:
- `blocks` - Block headers and transactions
- `transfer` - Individual transaction records
- `transactionByAddress` - Indexed transactions by address
- `internalTransactionByAddress` - Internal transactions (contract calls)
- `addresses` - Wallet balances and metadata
- `pending_transactions` - Mempool transactions (status: pending/mined)
- `validators` - Single document containing all validators per epoch
- `validatorHistory` - Historical validator counts per epoch
- `contractCode` - Smart contract deployments and token metadata

Token system:
- `tokenTransfers` - ERC20 token transfer events (from Transfer event logs)
- `tokenBalances` - Token holder balances per contract address

Analytics:
- `coingecko` - Market price data
- `walletCount` - Total wallet metrics
- `dailyTransactionsVolume` - Volume tracking
- `totalCirculatingQuanta` - Supply tracking
- `blockSize` - Block size history for charts
- `syncState` - Tracks current sync progress (lastSyncedBlock)

## Code Organization

### Frontend (Next.js 15 App Router)
- `app/` - Pages and API routes
- `app/components/` - Shared components (SearchBar, Sidebar, AreaChart)
- `app/lib/helpers.ts` - Formatting and conversion utilities
- Pattern: Server Components (`page.tsx`) fetch data; Client Components (`*-client.tsx`) handle interactivity

### Hosted dApp Example (`/dapp-example`)
- The route at `/dapp-example` is a static bundle baked in at build time from `DigitalGuards/myqrlwallet-connect` (folder: `example/`). It's served by Next directly from `public/dapp-example/`. See `ExplorerFrontend/scripts/README.md` for the full pipeline, env overrides, and troubleshooting.
- Pipeline: `ExplorerFrontend/scripts/build-dapp-example.sh` is wired via the `prebuild` npm hook, so every `npm run build` (and `update-frontend.sh`) clones the connect repo fresh and rebuilds the example with `--base=/dapp-example/`.
- **Gotcha:** `next dev` does NOT trigger `prebuild`, so `/dapp-example` 404s in dev until `npm run build` has been run at least once. When debugging a local 404, check `public/dapp-example/index.html` exists before anything else.
- The `next.config.js` rewrite maps `/dapp-example` → `/dapp-example/index.html` (Next doesn't auto-serve `index.html` for directory requests under `public/`). Don't remove it.
- Sidebar link uses a plain `<a>` with hard navigation (`Sidebar.tsx`) because the target is a static asset, not an App Router route — don't convert it to `<Link>`.

### Backend API (Go + Gin)
- `configs/` - Environment and MongoDB setup
- `db/` - Database query functions (one file per entity type)
- `handler/` - Request processing and middleware
- `models/` - Data structures
- `routes/routes.go` - All REST API endpoints

### Synchronizer (Go)
- `synchroniser/sync.go` - Main sync loop, batch processing, block insertion
- `synchroniser/pending_sync.go` - Mempool transaction sync (every 5s)
- `db/` - Database operations for syncing
- `rpc/` - Zond node RPC client
- `services/validator_service.go` - Validator data processing

## Sync Constants (QRL2MongoDB)

```go
DefaultBatchSize       = 64    // Normal batch size
LargeBatchSize         = 128   // When >1000 blocks behind
BatchSyncThreshold     = 64    // Triggers batch mode
MaxProducerConcurrency = 8     // Parallel block fetchers
MEMPOOL_SYNC_INTERVAL  = 5s    // Pending tx polling
MAX_PENDING_AGE        = 24h   // Pending tx cleanup threshold
```

## Recent Fixes

1. **Token Transfer Display (Added 2026-01-07)**: Added token transfer information to transaction pages. The `/tx/:hash` endpoint now includes `tokenTransfer` data from the `tokenTransfers` collection. Frontend displays token name, amount, and addresses for ERC20 transfers. Pending transactions decode input data to show token transfer details before confirmation.

2. **Pending Transaction Lifecycle (Fixed 2026-01-03)**: Fixed bug where mined transactions still showed as pending. The issue was in `backendAPI/routes/routes.go` - the `/pending-transaction/:hash` endpoint was returning mined transactions instead of returning 404. Also updated `ExplorerFrontend/app/tx/[query]/page.tsx` to check `status === 'pending'` before showing pending view.

3. **Transaction Fee Calculation (Fixed)**: Fixed transaction fee calculation to properly store paid fees. Implemented fallback mechanisms when gas usage data is missing (receipt → gas limit → ensure non-zero for successful txs).

## Areas Needing Work

1. **UI/UX Improvements**: Enhance the frontend design for a sleeker, more modern look and better user experience (e.g., improving chart visualizations, simplifying navigation, or ensuring responsive design on mobile devices).
2. **Smart Contract Features**: Expand smart contract support - better contract interaction views, verified contract source display, and ABI decoding.
3. **Validator Enhancements**: Improve validator-related features - detailed validator pages, staking analytics, and attestation tracking.

## Testing Against Local Node

A local QRL Zond node is available for testing. Compare:
- Node RPC (localhost:8545) - source of truth for blockchain data
- MongoDB logs - what the syncer has stored
- Syncer logs - operational status of sync process

## Git Workflow

- **Branching Strategy**: Use feature branches for all new work. Create your branch from the `dev` branch.
- **Never commit directly to `main` or `dev`** - all changes must go through Pull Requests (PRs).
- **PR Process (for features/fixes)**:
  1. Create a new branch from `dev` (e.g., `git checkout -b feat/new-feature dev`).
  2. Commit and push changes to your feature branch.
  3. Create a PR from your feature branch to `dev`.
  4. Wait for Gemini's automated review (3-10 minutes depending on PR size) and address all comments.
  5. Merge the PR into `dev` after it's approved.
- **Release Process (dev to main)**:
  1. Periodically, create a PR from `dev` to `main` to release new features.
  2. This PR should also be reviewed before merging.

## Commit Convention

Use conventional commits: `feat:`, `fix:`, `perf:`, `docs:`, `chore:`, `test:`

## Data Format Notes

- Numeric values stored as hex strings with "0x" prefix in blocks/transactions
- Addresses and hashes stored in hex format (WITH "0x" prefix in most places)
- Timestamps are Unix timestamps in decimal or hex format
- Validator data stores epochs/balances as decimal strings
