# Testnet network isolation

V2 keeps its existing database, collections, indexes and faucet claim history. V3 uses a new database, a new MongoDB volume, new credentials and separate application processes. Switching networks performs a full navigation between configured origins so server-rendered requests, API calls and browser caches use one deployment identity throughout a page.

The supplied profiles prepare the separation. V3 stays disabled until the published chain identity and reviewed 64-byte artifacts are available. The future V3 origin is a configurable placeholder, not an announcement of an active network.

## Profile checks

Copy `deploy/network/profiles.example.json` to the ignored `profiles.local.json` in the same directory. Set the V2 storage and ports to match the existing installation. Retain its database and volume exactly. Keep the V3 database and volume separate. Populate real RPC, beacon, chain ID, execution genesis hash and beacon genesis root/time pins only after verifying the intended network. Both templates leave all identity pins unset.

Run this from the repository root to check the profiles without opening a network connection or starting a service:

```bash
python3 deploy/network/preflight.py deploy/network/profiles.local.json
python3 -m unittest discover -s deploy/network/tests -v
```

The preflight checks distinct databases, volumes, origins, credential variable names and all eight application/database ports. Service listeners use loopback. An activation check additionally requires explicit execution and beacon identity pins, separate RPC/beacon endpoints and private, distinct credential files. Export URI and credential-file environment variables in a private operator environment. The URI database and port must match the selected profile. Credentials never belong in the profile JSON.

| Profile value | Application environment |
| --- | --- |
| `network` | `EXPLORER_NETWORK=v2` or `v3` |
| `database` | `MONGO_DB_NAME` in every component |
| `chain_id` | `EXPECTED_CHAIN_ID`, canonical positive decimal string |
| `genesis_hash` | `EXPECTED_GENESIS_HASH`, lowercase 32-byte `0x` execution block-zero hash |
| `beacon_genesis_validators_root` | syncer-only `EXPECTED_BEACON_GENESIS_VALIDATORS_ROOT`, nonzero 32-byte `0x` root |
| `beacon_genesis_time` | syncer-only `EXPECTED_BEACON_GENESIS_TIME`, canonical positive decimal uint64 timestamp |
| `mongo_uri_env` value | `MONGOURI` for Go, `DATABASE_URL` for frontend Mongo access |
| `rpc_url` | `NODE_URL`; every entry in `NODE_URLS` must identify the same network |
| `beacon_url` | `BEACONCHAIN_API` |
| `ports.backend` | `HTTP_PORT`, plus the deployment manager's loopback bind |
| `ports.syncer_health` | `HEALTH_PORT`, plus the deployment manager's loopback bind |
| `ports.frontend` | frontend `PORT` and loopback hostname |
| `origin` | public domain and network-selector URL |

Build each frontend with its own network, handler URL and origin settings. A single build must not mix a V2 page origin with a V3 handler or database. Trace, mempool, contract compiler/verifier and faucet settings are also network-specific. Leave V3 faucet spending disabled until it has a separate verified configuration and funding source.

Verify the execution chain ID and block-zero hash against the reviewed network release configuration. Separately verify the configured beacon source's `/eth/v1/beacon/genesis` response: `data.genesis_validators_root` and `data.genesis_time` must match that network's reviewed beacon genesis configuration. The beacon root identifies the genesis validator set; the execution hash identifies execution block zero. Populate each pin from its corresponding source. A pinned syncer requires both beacon pins and validates its beacon feed before accepting validator data. Offline preflight validates the supplied values and their separation; it does not contact either endpoint or establish that the supplied pins are authoritative.

## Prepare empty V3 MongoDB storage

`deploy/network/mongodb-v3.compose.yml` contains only a new MongoDB service. Every service is behind the explicit `prepare-v3` profile. Validation does not start it. The default database is `qrldata-v3`, the volume is `zondscan-testnet-v3-data`, and the only published listener is loopback. It uses MongoDB 8.2 and a dedicated single-member replica set, `zondscan-v3`, to support future transactional ingestion. A single member provides transaction support; it does not provide high availability.

Before provisioning, check that the dedicated volume name is unused. Never attach the existing V2 volume to this service. Create five private files with independently generated contents: administrator username/password, application username/password and a MongoDB replica key. Set their paths through `MONGO_V3_ADMIN_USER_FILE`, `MONGO_V3_ADMIN_PASSWORD_FILE`, `MONGO_V3_USER_FILE`, `MONGO_V3_PASSWORD_FILE` and `MONGO_V3_REPLICA_KEY_FILE`. Files must be owner-readable only. Follow MongoDB's keyfile format requirements. Application credentials must differ from the administrator and from V2 credentials.

The following explicit preparation commands create only V3 database storage. They do not activate the network selector or start frontend, backend or syncer processes:

```bash
docker compose -f deploy/network/mongodb-v3.compose.yml --profile prepare-v3 config --quiet
docker compose -f deploy/network/mongodb-v3.compose.yml --profile prepare-v3 up -d mongodb-v3
docker compose -f deploy/network/mongodb-v3.compose.yml --profile prepare-v3 exec -T mongodb-v3 mongosh --quiet --nodb /opt/network/mongo-init-replica.js
docker compose -f deploy/network/mongodb-v3.compose.yml --profile prepare-v3 ps
```

Wait for initial database/user creation before running the replica initialization command. It can be rerun safely for the same replica-set identity. The application user receives `readWrite` only on `qrldata-v3`; administrator credentials stay with operators. For applications outside this MongoDB container, use the mapped loopback port with `replicaSet=zondscan-v3&directConnection=true&authSource=qrldata-v3`. The single-member advertised hostname is container-local, so `directConnection=true` is required for host clients.

Initialization scripts only run for a new empty volume. Changing a secret file later does not rotate a stored MongoDB password. Perform credential rotation explicitly using a reviewed operator procedure.

## Preserve the complete V2 database

`mongo_archive.py` creates a compressed archive of every collection in the selected database, including faucet claims, pending transactions, sync checkpoints, token ingestion state and network identity. It records collection/index inventories, document counts, timestamps, archive size and SHA-256. The output directory and archive are private. The tool uses a temporary owner-only Database Tools configuration file to keep URI credentials out of command arguments and console errors.

Keep backups outside public source control and copy the verified archive to separately protected backup storage. Application configuration, database users/roles, secrets, node state and deployment configuration need a separate private backup. A database archive does not include those external resources.

For a consistent standalone archive, pause every database writer for the entire dump, including the synchronizer, faucet requests and verification writes. An operator-managed MongoDB write lock is another way to quiesce writes, provided it has a tested unlock/watchdog procedure. The tool does not acquire a lock or stop services. Select `writers-quiesced` only after this has been established. It checks that the before/after inventories agree.

With active writers, select `hot-standalone`. Its manifest explicitly records that collections can reflect different moments and there is no point-in-time guarantee. MongoDB documents quiescing writes or a replica-set oplog-aware backup for consistency. This tool performs a single-database logical archive and does not replay an oplog. See [MongoDB backup guidance](https://www.mongodb.com/docs/manual/tutorial/backup-and-restore-tools/).

Set `MONGO_BACKUP_URI` privately to the source loopback endpoint, omitting the URI database path. The database is supplied separately:

```bash
python3 deploy/network/mongo_archive.py backup --database qrldata-z --output deploy/network/backups/v2-snapshot --consistency writers-quiesced
```

Do a restore probe before relying on the backup. Start a separate disposable MongoDB server using a fresh volume and a different loopback port. Set `MONGO_RESTORE_URI` privately to that server, also omitting the database path. The tool requires an empty destination whose name starts with `restore_probe_`, verifies the archive hash, preserves all collection/index definitions and checks source counts for a quiesced snapshot. It uses `--bypassDocumentValidation` during the isolated restore so historical records that predate a current validator can be recovered. The restored validator definitions are retained and checked. It never passes `--drop` or restores directly into a live V2/V3 database.

```bash
python3 deploy/network/mongo_archive.py restore-probe --database restore_probe_v2_snapshot --input deploy/network/backups/v2-snapshot
```

Disable the TTL monitor in the isolated restore server while comparing inventories if the archive contains expiring records. A hot backup can restore successfully with counts that differ from the live source; the probe records that distinction. Preserve the probe report with the archive. Restore users/roles and operator configuration through the separate private recovery process.

## V3 activation gate

The offline gate is an operator checklist with artifact integrity checks. Its readiness manifest is a local attestation from completed release validation; it does not establish protocol support by itself. Populate it only after testing address encoding/decoding, ABI calls, logs/topics, balances, token transfers and transactional ingestion against the pinned V3 network.

Set V3 `enabled` to `true` in the private profile only when ready to activate. Supply a private readiness JSON with `networkId: "v3"`, `addressBytes: 64`, `identityVerified: true`, canonical `chainId`, exact execution `genesisHash`, `beaconGenesisValidatorsRoot`, `beaconGenesisTime`, `transactional_ingestion: true`, and an `artifacts` object containing `frontend`, `backend` and `syncer`. The readiness beacon pair must match the profile's validated beacon pins; the timestamp remains a decimal string. Each artifact entry requires a local `path`, its `sha256`, `address_bytes: 64` and `network_isolation: true`. Use an immutable frontend release archive as the frontend artifact.

```bash
python3 deploy/network/preflight.py deploy/network/profiles.local.json --activate v3 --readiness deploy/network/readiness.local.json
```

The gate rejects disabled profiles, missing pins, shared resources, a shared beacon genesis root/time pair, legacy capabilities and changed artifacts. Activation still requires the application's startup identity checks and a verified `GET /network` result: `networkId`, `chainId`, `genesisHash`, `addressBytes` and `identityVerified`. The MongoDB `explorerNetwork` singleton `_id: "identity"` must agree. These shared fields describe the execution network. Beacon pins stay in the syncer's configuration and local release readiness manifest. An unverified legacy marker requires explicit adoption before pinning. Do not copy the V2 marker into V3.

At cutover, retain V2 as its own deployment. If the old chain retires, stop V2 writers and serve its preserved data in archive mode. Disable write-producing endpoints such as faucet claims and verification submissions in that archive deployment. Record the final indexed block and snapshot hash, verify representative blocks, transactions, addresses and faucet claim history, then enable the separately validated V3 selector target. Keep V2 recovery artifacts and its volume until a deliberate retention decision is made.
