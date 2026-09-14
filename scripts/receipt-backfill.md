# Receipt and calldata backfill

`receipt-backfill.mjs` repairs receipt accounting and calldata in three explorer collections. It is a standalone maintenance tool; application deployment, syncer replay, and schema migration are separate operations.

## Prerequisites

- Use a qualified Node.js 22 runtime and the installed MongoDB driver resolved from `ExplorerFrontend`, or an explicit `--driver-root`. The tool installs no dependencies. MongoDB must support the CAS expressions used by the tool, including `$getField` and `$arrayElemAt`; `durable` mode requires a replica set or mongos.
- Pin the exact Mongo endpoint, database, schema profile, RPC endpoint, chain ID, genesis hash, inclusive block range, and canonical end-block hash. Both endpoints must be explicit loopback endpoints. Use a finalized end anchor where available. Historical block bodies and receipts must remain retrievable; this repair does not request traces or historical contract state.
- Inspect index readiness: `_id_` on all three collections; a usable `blocks.blockNumberInt` index; a `blocks.result.number` index, including a compound index with that leading field; and companion indexes covering `txHash` plus `blockNumber`. Verify query plans on the actual target. This tool does not create indexes.
- Take coherent archived backups of `blocks`, `transactionByAddress`, and `transfer` while writers are paused. Preserve collection/index metadata, checksum the archives, verify an off-host copy, and restore-test them in an isolated database. Compare exact cursor/document counts and representative BSON values and types; estimated collection counts can be stale.
- Retain enough disk space for backups and the complete journal. Keep journals and runtime profiles private. Collection UUIDs differ after a normal restore, so a production journal cannot be silently redirected to a restored database.
- Hold an external exclusive per-target lock, such as `flock`, throughout apply or rollback and its verification. The same lock must cover every maintenance runner targeting that database. Independently stop and verify **both readers and writers on both schema profiles** before mutation. Pause flags acknowledge these external controls; the CLI does not stop services or acquire the operating-system lock.

## Arguments

All identity arguments are required in every mode and must remain identical across preparation, apply, verification, and rollback.

| Argument                               | Value                                                                                                        |
| -------------------------------------- | ------------------------------------------------------------------------------------------------------------ |
| `--mode`                               | `dry-run` (default), `prepare`, `apply`, `verify`, or `rollback`                                             |
| `--driver-root`                        | Directory containing the installed `mongodb` dependency; defaults to sibling `ExplorerFrontend`              |
| `--rpc-url`                            | Loopback HTTP(S) RPC URL, without URL credentials                                                            |
| `--mongo-uri`                          | Single loopback `mongodb://` endpoint; any URI database must match `--db`                                    |
| `--db`                                 | Explicit database name                                                                                       |
| `--schema`                             | `legacy`: markerless blocks and 20-byte addresses; `durable`: existing complete blocks and 64-byte addresses |
| `--chain-id`                           | Decimal chain ID                                                                                             |
| `--genesis-hash`                       | Expected `0x`-prefixed 32-byte genesis hash                                                                  |
| `--start`, `--end`                     | Inclusive decimal block heights                                                                              |
| `--end-hash`                           | Expected canonical hash at `--end`                                                                           |
| `--journal-dir`                        | Absolute directory path; required except for `dry-run`                                                       |
| `--rpc-delay-ms`                       | Minimum request-start spacing, default `50`, accepted range `0` through `10000`; RPC work is sequential      |
| `--writers-paused`, `--readers-paused` | Both acknowledgements required for `apply` and `rollback`                                                    |
| `--accept-skips`                       | Explicitly permits applying a reviewed plan that excludes blocks                                             |

Replace every placeholder before using this read-only example:

```sh
node scripts/receipt-backfill.mjs --mode dry-run --driver-root ./ExplorerFrontend --rpc-url 'http://127.0.0.1:<rpc-port>' --mongo-uri 'mongodb://127.0.0.1:<mongo-port>/<database>' --db '<database>' --schema '<legacy-or-durable>' --chain-id '<decimal-chain-id>' --genesis-hash '<genesis-hash>' --start '<first-height>' --end '<last-height>' --end-hash '<canonical-end-hash>' --rpc-delay-ms 50
```

## Workflow and recovery

1. Run `dry-run`. It reads RPC and MongoDB and writes no database rows or journal files. Review the summary and every skipped-block reason.
2. Run `prepare` with the same arguments and `--journal-dir` pointing to a **fresh, nonexistent absolute directory**. This performs receipt RPC work ahead of the maintenance window. Per-block raw-BSON beforeimages, expected afterimages, and an index are persisted and fsynced before the manifest is sealed. An interrupted preparation remains unsealed and cannot be applied; start another preparation in a fresh directory.
3. Review the sealed plan, exact counts, exclusions, storage requirements, and backup qualification. Prefer zero skipped blocks. Missing or duplicate companions and invalid stored blocks exclude the entire block. `--accept-skips` requires an explicit operator decision; completion then covers only the accepted plan.
4. Acquire the external per-target lock, pause and verify both readers and writers, then run `apply` with the original arguments, journal directory, and both pause flags. Initial and final chain/anchor checks bracket the preplanned writes; apply performs no per-block receipt RPC. Replica-set apply uses per-block transactions. Standalone apply uses journaled, sequential CAS with conditional rollback on failure.
5. Run `verify` against the same sealed plan before releasing the maintenance window. It requires every planned row to match its afterimage. Recheck operational readiness before restarting services.

The manifest binds the exact runtime source SHA-256, collection UUIDs, endpoints, database, schema, and chain/range identity. Even a source-only formatting change invalidates an existing plan. Keep the qualified artifact unchanged. Apply preflights every plan file and its mutation shape before its first write.

For interrupted apply, retain the journal, re-establish the lock and pause controls, and rerun `apply`. Exact beforeimages and afterimages determine safe progress; an intervening value, BSON type change, or missing row stops the operation. Progress is durably recorded after verified blocks.

Run `rollback` with the same identity arguments, journal, lock, and both pause flags to conditionally restore approved fields from beforeimages. It preserves original field absence and refuses to overwrite intervening changes. Inspect any unresolved conflict before further action. `verify` checks applied afterimages; rollback qualification uses the restored beforeimages and rollback result instead. Retain archives and journals after completion.

## Repair boundary and reuse qualification

Only these fields are eligible:

| Collection             | Fields                                                                                 |
| ---------------------- | -------------------------------------------------------------------------------------- |
| `blocks`               | Within each `result.transactions[i]`: `data`, `status`, `gasUsed`, `effectiveGasPrice` |
| `transactionByAddress` | `paidFees`, `paidFeesWei`, `feeSource`, `receiptStatus`                                |
| `transfer`             | `data`, `status`, `paidFees`                                                           |

Transaction order, `_id`, addresses, submitted gas limits/prices, amounts, signatures, contract metadata, unknown BSON values/types, and ingestion/token markers are preserved. Every receipt must match its transaction and block identity. Fees use actual `gasUsed * effectiveGasPrice`; zero-priced receipts remain exactly zero. `paidFeesWei` is an exact decimal integer string. Legacy `paidFees` remains a BSON double derived from the exact 18-place QRL decimal representation.

Before reuse on larger blocks or a different schema, qualify every serialized forward and reverse MongoDB update command against a conservative **12 MiB** ceiling, including its command wrapper. The CLI's 64 MiB per-plan-file ceiling does not bound the expanded CAS command size. Requalify oversized commands, unsupported BSON forms, and actual query plans before applying. Run the focused unit tests and the isolated standalone/replica-set integration suite for the selected runtime and driver.
