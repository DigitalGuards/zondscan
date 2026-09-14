#!/usr/bin/env node
import assert from 'node:assert/strict';
import { createHash, randomUUID } from 'node:crypto';
import { createReadStream } from 'node:fs';
import { mkdir, open, readFile, rename, stat } from 'node:fs/promises';
import { createRequire } from 'node:module';
import { dirname, isAbsolute, join, resolve } from 'node:path';
import { createInterface } from 'node:readline';
import { fileURLToPath } from 'node:url';

const SCRIPT = fileURLToPath(import.meta.url);
const COLLECTIONS = ['blocks', 'transactionByAddress', 'transfer'];
const VERSION = 1;
const MAX_PLAN_BYTES = 64 * 1024 * 1024;
const QUANTA = 10n ** 18n;
const MISSING = Symbol('missing');

export class DataError extends Error {}

export function quantity(value, label = 'quantity') {
  if (typeof value !== 'string' || !/^0x(?:0|[1-9a-fA-F][0-9a-fA-F]*)$/.test(value)) {
    throw new DataError(`Invalid ${label}`);
  }
  const result = BigInt(value);
  if (result >= 1n << 256n) throw new DataError(`Oversized ${label}`);
  return result;
}

function hash(value, label = 'hash') {
  if (typeof value !== 'string' || !/^0x[0-9a-fA-F]{64}$/.test(value)) {
    throw new DataError(`Invalid ${label}`);
  }
  return value.toLowerCase();
}

function decimal(value, label) {
  if (!/^(0|[1-9][0-9]*)$/.test(String(value))) throw new Error(`Invalid ${label}`);
  const result = BigInt(value);
  if (result > 9223372036854775807n) throw new Error(`${label} exceeds BSON int64`);
  return result;
}

function address(value, schema, label, allowEmpty = false) {
  if (allowEmpty && (value === '' || value === null)) return '';
  if (typeof value !== 'string') throw new DataError(`Invalid ${label}`);
  const body = value.replace(/^(?:0x|[Qq])/, '');
  const width = schema === 'legacy' ? 40 : 128;
  if (body === value || !new RegExp(`^[0-9a-fA-F]{${width}}$`).test(body)) {
    throw new DataError(`Invalid ${label}`);
  }
  return body.toLowerCase();
}

function same(actual, expected, label) {
  if (actual !== expected) throw new DataError(`${label} mismatch`);
}

export function exactFeeDecimal(wei) {
  return `${wei / QUANTA}.${(wei % QUANTA).toString().padStart(18, '0')}`;
}

function getPath(document, path) {
  let current = document;
  for (const key of path.split('.')) {
    if (current === null || typeof current !== 'object' || !Object.hasOwn(current, key)) return MISSING;
    current = current[key];
  }
  return current;
}

function setPath(document, path, value) {
  const keys = path.split('.');
  let current = document;
  for (const key of keys.slice(0, -1)) {
    if (current[key] === null || typeof current[key] !== 'object') throw new Error('Invalid patch parent');
    current = current[key];
  }
  if (value === MISSING) delete current[keys.at(-1)];
  else current[keys.at(-1)] = value;
}

function digest(bytes) {
  return createHash('sha256').update(bytes).digest('hex');
}

function encoded(document, BSON) {
  return BSON.serialize(document);
}

function decode(raw, BSON) {
  return BSON.deserialize(raw, { promoteValues: false });
}

function sameDocument(left, right, BSON) {
  return encoded(left, BSON).equals(encoded(right, BSON));
}

function decodePlanRow(row, BSON) {
  const beforeRaw = Buffer.from(row.before, 'base64');
  const afterRaw = Buffer.from(row.after, 'base64');
  if (digest(beforeRaw) !== row.beforeHash || digest(afterRaw) !== row.afterHash) throw new Error('Plan BSON checksum mismatch');
  return { before: decode(beforeRaw, BSON), after: decode(afterRaw, BSON) };
}

function makeRow(collection, beforeRaw, patches, BSON) {
  const before = decode(beforeRaw, BSON);
  if (!encoded(before, BSON).equals(beforeRaw)) throw new DataError('Raw BSON cannot be round-tripped without representation changes');
  const after = decode(beforeRaw, BSON);
  // MongoDB processes string update paths lexicographically. Mirror that
  // insertion order while retaining every existing BSON key and value.
  for (const path of Object.keys(patches).sort()) setPath(after, path, patches[path]);
  const afterRaw = encoded(after, BSON);
  return {
    collection,
    paths: Object.keys(patches),
    before: Buffer.from(beforeRaw).toString('base64'),
    after: afterRaw.toString('base64'),
    beforeHash: digest(beforeRaw),
    afterHash: digest(afterRaw),
  };
}

export function planBlock({ blockRaw, block, receipts, histories, transfers, options, BSON, Double }) {
  const stored = decode(blockRaw, BSON);
  const height = quantity(stored.result?.number, 'stored block number');
  const blockHash = hash(stored.result?.hash, 'stored block hash');
  same(height.toString(), stored.blockNumberInt?.toString(), 'stored numeric height');
  same(quantity(block.number, 'RPC block number'), height, 'canonical height');
  same(hash(block.hash), blockHash, 'canonical block');
  if (options.schema === 'durable') same(stored.ingestionState, 'complete', 'durable completion');
  else if (Object.hasOwn(stored, 'ingestionState')) throw new DataError('Legacy block unexpectedly has an ingestion marker');
  if (!Array.isArray(stored.result.transactions) || !Array.isArray(block.transactions)) throw new DataError('Invalid transaction array');
  same(stored.result.transactions.length, block.transactions.length, 'transaction count');
  same(receipts.length, block.transactions.length, 'receipt count');
  if (block.transactions.length > 4096) throw new DataError('Transaction block exceeds repair bound');
  const patches = {};
  const rows = [];
  for (let index = 0; index < block.transactions.length; index++) {
    const tx = block.transactions[index];
    const old = stored.result.transactions[index];
    const receipt = receipts[index];
    const txHash = hash(tx.hash, 'transaction hash');
    same(hash(old.hash), txHash, 'stored transaction order');
    same(hash(old.blockhash), blockHash, 'stored transaction block');
    same(quantity(old.blocknumber), height, 'stored transaction height');
    same(quantity(old.transactionindex), BigInt(index), 'stored transaction index');
    same(hash(tx.blockHash), blockHash, 'RPC transaction block');
    same(quantity(tx.blockNumber), height, 'RPC transaction height');
    same(quantity(tx.transactionIndex), BigInt(index), 'RPC transaction index');
    if (tx.chainId !== undefined) same(quantity(tx.chainId), BigInt(options.chainId), 'transaction chain');
    const from = address(tx.from, options.schema, 'transaction sender');
    const to = address(tx.to, options.schema, 'transaction recipient', true);
    same(address(old.from, options.schema, 'stored sender'), from, 'stored sender');
    same(address(old.to, options.schema, 'stored recipient', true), to, 'stored recipient');
    same(quantity(old.gas), quantity(tx.gas), 'stored gas limit');
    same(quantity(old.value), quantity(tx.value), 'stored value');
    same(quantity(old.nonce), quantity(tx.nonce), 'stored nonce');
    if (!receipt || typeof receipt !== 'object') throw new DataError('Missing mined receipt');
    same(hash(receipt.transactionHash), txHash, 'receipt transaction');
    same(hash(receipt.blockHash), blockHash, 'receipt block');
    same(quantity(receipt.blockNumber), height, 'receipt height');
    same(quantity(receipt.transactionIndex), BigInt(index), 'receipt index');
    same(address(receipt.from, options.schema, 'receipt sender'), from, 'receipt sender');
    same(address(receipt.to, options.schema, 'receipt recipient', true), to, 'receipt recipient');
    if (!['0x0', '0x1'].includes(receipt.status)) throw new DataError('Invalid execution status');
    const gasUsed = quantity(receipt.gasUsed, 'receipt gas used');
    if (gasUsed > quantity(tx.gas, 'gas limit')) throw new DataError('Receipt gas exceeds transaction limit');
    const price = quantity(receipt.effectiveGasPrice, 'receipt effective price');
    if (typeof tx.input !== 'string' || !/^0x(?:[0-9a-fA-F]{2})*$/.test(tx.input)) throw new DataError('Invalid calldata');
    const fee = gasUsed * price;
    const legacyFee = Number(exactFeeDecimal(fee));
    if (!Number.isFinite(legacyFee)) throw new DataError('Legacy fee exceeds finite double');
    for (const [field, value] of Object.entries({ data: tx.input, status: receipt.status, gasUsed: receipt.gasUsed, effectiveGasPrice: receipt.effectiveGasPrice })) {
      patches[`result.transactions.${index}.${field}`] = value;
    }
    const historyRaw = histories[index];
    const transferRaw = transfers[index];
    if (!historyRaw || !transferRaw) throw new DataError('Missing transaction companion');
    for (const [raw, label] of [[historyRaw, 'history'], [transferRaw, 'transfer']]) {
      const companion = decode(raw, BSON);
      if (!Object.hasOwn(companion, '_id')) throw new DataError(`Missing ${label} id`);
      same(hash(companion.txHash), txHash, `${label} hash`);
      same(quantity(companion.blockNumber), height, `${label} block`);
      same(address(companion.from, options.schema, `${label} sender`), from, `${label} sender`);
      if (label === 'transfer' && to === '' && companion.contractAddress !== undefined) {
        same(address(companion.contractAddress, options.schema, 'stored created contract'), address(receipt.contractAddress, options.schema, 'receipt created contract'), 'created contract');
      } else {
        same(address(companion.to, options.schema, `${label} recipient`, true), to, `${label} recipient`);
      }
    }
    rows.push(makeRow('transactionByAddress', historyRaw, {
      paidFees: new Double(legacyFee), paidFeesWei: fee.toString(), feeSource: 'receipt', receiptStatus: receipt.status,
    }, BSON));
    rows.push(makeRow('transfer', transferRaw, { data: tx.input, status: receipt.status, paidFees: new Double(legacyFee) }, BSON));
  }
  rows.push(makeRow('blocks', blockRaw, patches, BSON));
  return { version: VERSION, height: height.toString(), hash: blockHash, transactions: block.transactions.length, rows };
}

function bsonType(value) {
  if (value === null) return 'null';
  if (Array.isArray(value)) return 'array';
  if (value instanceof Date) return 'date';
  return ({ Int32: 'int', Long: 'long', Double: 'double', Decimal128: 'decimal', ObjectId: 'objectId', Binary: 'binData', BSONRegExp: 'regex', Timestamp: 'timestamp', MinKey: 'minKey', MaxKey: 'maxKey' })[value?._bsontype] ?? ({ string: 'string', boolean: 'bool', object: 'object' })[typeof value];
}

// Equality alone treats BSON numeric types as equivalent. Pair every captured
// leaf with a type fence and retain explicit missing-versus-null predicates.
function expressionPath(document, path) {
  let current = document;
  let expression = '$$ROOT';
  for (const key of path.split('.')) {
    expression = Array.isArray(current)
      ? { $arrayElemAt: [expression, Number(key)] }
      : { $getField: { field: { $literal: key }, input: expression } };
    current = current?.[key];
  }
  return expression;
}

function exactPredicates(document, path, clauses) {
  const value = getPath(document, path);
  if (value === MISSING) {
    clauses.push({ [path]: { $exists: false } });
    return;
  }
  const type = bsonType(value);
  if (!type) throw new Error(`Unsupported BSON type at ${path}`);
  clauses.push({ [path]: { $exists: true } });
  clauses.push({ [path]: { $eq: value } });
  clauses.push({ $expr: { $eq: [{ $type: expressionPath(document, path) }, type] } });
  if (type === 'array' || type === 'object') {
    for (const key of Object.keys(value)) {
      if (key.includes('.') || key.startsWith('$')) throw new Error('Unsupported BSON key in CAS');
      exactPredicates(document, `${path}.${key}`, clauses);
    }
  }
}

export function mutationFor(row, direction, BSON) {
  if (!COLLECTIONS.includes(row.collection)) throw new Error('Unapproved collection');
  const { before, after } = decodePlanRow(row, BSON);
  const expected = direction === 'forward' ? before : after;
  const desired = direction === 'forward' ? after : before;
  const allowed = row.collection === 'blocks' ? /^result\.transactions\.[0-9]+\.(data|status|gasUsed|effectiveGasPrice)$/ : row.collection === 'transactionByAddress' ? /^(paidFees|paidFeesWei|feeSource|receiptStatus)$/ : /^(data|status|paidFees)$/;
  if (!Array.isArray(row.paths) || !row.paths.every((path) => allowed.test(path))) throw new Error('Unapproved patch field');
  const rebuilt = decode(encoded(before, BSON), BSON);
  for (const path of [...row.paths].sort()) setPath(rebuilt, path, getPath(after, path));
  if (!sameDocument(rebuilt, after, BSON)) throw new Error('Afterimage changes fields outside its approved patch');
  const clauses = [];
  // The full captured roots fence identities, arrays, unknown fields and BSON
  // types. Only approved leaf paths are ever written, including rollback.
  for (const key of new Set([...Object.keys(before), ...Object.keys(after)])) {
    if (key.includes('.') || key.startsWith('$')) throw new Error('Unsupported BSON root key in CAS');
    exactPredicates(expected, key, clauses);
  }
  clauses.push({ $expr: { $eq: [{ $size: { $objectToArray: '$$ROOT' } }, Object.keys(expected).length] } });
  const update = { $set: {}, $unset: {} };
  for (const path of row.paths) {
    const value = getPath(desired, path);
    if (value === MISSING) update.$unset[path] = '';
    else update.$set[path] = value;
  }
  if (!Object.keys(update.$set).length) delete update.$set;
  if (!Object.keys(update.$unset).length) delete update.$unset;
  return { filter: { $and: clauses }, update, expected, desired };
}

export function validateOptions(input) {
  const options = { mode: 'dry-run', driverRoot: resolve(dirname(SCRIPT), '../ExplorerFrontend'), rpcDelayMs: 50, ...input };
  if (!['dry-run', 'prepare', 'apply', 'verify', 'rollback'].includes(options.mode)) throw new Error('Invalid mode');
  if (!['legacy', 'durable'].includes(options.schema)) throw new Error('Explicit --schema legacy|durable is required');
  for (const key of ['rpcUrl', 'mongoUri', 'db', 'chainId', 'genesisHash', 'start', 'end', 'endHash']) {
    if (options[key] === undefined || options[key] === '') throw new Error(`Missing required option ${key}`);
  }
  const rpc = new URL(options.rpcUrl);
  if (!['http:', 'https:'].includes(rpc.protocol) || !['127.0.0.1', '[::1]', 'localhost'].includes(rpc.hostname) || rpc.username || rpc.password || rpc.hash) throw new Error('RPC must be an explicit loopback HTTP endpoint without credentials');
  const mongo = new URL(options.mongoUri);
  if (mongo.protocol !== 'mongodb:' || !['127.0.0.1', '[::1]', 'localhost'].includes(mongo.hostname) || mongo.hash) throw new Error('MongoDB must be a single explicit loopback endpoint');
  if (!/^[A-Za-z0-9_-]+$/.test(options.db)) throw new Error('Invalid database name');
  if (mongo.pathname !== '' && mongo.pathname !== '/' && mongo.pathname !== `/${options.db}`) throw new Error('Mongo URI database does not match --db');
  options.chainId = decimal(options.chainId, 'chain id').toString();
  options.start = decimal(options.start, 'start').toString();
  options.end = decimal(options.end, 'end').toString();
  if (BigInt(options.start) > BigInt(options.end)) throw new Error('Start exceeds end');
  options.genesisHash = hash(options.genesisHash, 'genesis hash');
  options.endHash = hash(options.endHash, 'end hash');
  options.rpcDelayMs = Number(options.rpcDelayMs);
  if (!Number.isInteger(options.rpcDelayMs) || options.rpcDelayMs < 0 || options.rpcDelayMs > 10000) throw new Error('Invalid RPC delay');
  if (options.mode !== 'dry-run' && (!options.journalDir || !isAbsolute(options.journalDir))) throw new Error('An absolute --journal-dir is required');
  if (['apply', 'rollback'].includes(options.mode) && !options.writersPaused) throw new Error('--writers-paused acknowledgement is required');
  if (['apply', 'rollback'].includes(options.mode) && !options.readersPaused) throw new Error('--readers-paused acknowledgement is required for maintenance writes');
  options.mongoIdentity = `${mongo.hostname}:${mongo.port || '27017'}/${options.db}?replicaSet=${mongo.searchParams.get('replicaSet') || ''}`;
  options.rpcIdentity = rpc.href;
  return options;
}

function rpcClient(options) {
  let previous = 0;
  let id = 0;
  return async (method, params = []) => {
    const delay = options.rpcDelayMs - (Date.now() - previous);
    if (delay > 0) await new Promise((done) => setTimeout(done, delay));
    previous = Date.now();
    const requestId = ++id;
    let response;
    try {
      response = await fetch(options.rpcUrl, {
        method: 'POST', headers: { 'content-type': 'application/json' },
        body: JSON.stringify({ jsonrpc: '2.0', id: requestId, method, params }),
        signal: AbortSignal.timeout(10000), redirect: 'error',
      });
      if (!response.ok) throw new Error('HTTP failure');
      const reader = response.body.getReader();
      const chunks = [];
      let size = 0;
      for (;;) {
        const { done, value } = await reader.read();
        if (done) break;
        size += value.byteLength;
        if (size > 32 * 1024 * 1024) { await reader.cancel(); throw new Error('RPC response too large'); }
        chunks.push(value);
      }
      const envelope = JSON.parse(Buffer.concat(chunks).toString('utf8'));
      if (envelope.jsonrpc !== '2.0' || envelope.id !== requestId || envelope.error || envelope.result === null || envelope.result === undefined) throw new Error('Invalid RPC response');
      return envelope.result;
    } catch {
      throw new Error(`RPC ${method} failed; no target writes authorized by this response`);
    }
  };
}

async function verifyNetwork(rpc, options) {
  same(quantity(await rpc('qrl_chainId')), BigInt(options.chainId), 'RPC chain identity');
  const genesis = await rpc('qrl_getBlockByNumber', ['0x0', false]);
  same(quantity(genesis.number), 0n, 'genesis height');
  same(hash(genesis.hash), options.genesisHash, 'genesis identity');
  const anchor = await rpc('qrl_getBlockByNumber', [`0x${BigInt(options.end).toString(16)}`, false]);
  same(quantity(anchor.number), BigInt(options.end), 'anchor height');
  same(hash(anchor.hash), options.endHash, 'anchor identity');
}

async function syncDirectory(directory) {
  const handle = await open(directory, 'r');
  try { await handle.sync(); } finally { await handle.close(); }
}

async function durableJSON(file, value) {
  const temp = `${file}.${randomUUID()}.tmp`;
  const handle = await open(temp, 'wx', 0o600);
  try { await handle.writeFile(JSON.stringify(value)); await handle.sync(); } finally { await handle.close(); }
  await rename(temp, file);
  await syncDirectory(dirname(file));
}

async function appendIndex(directory, record) {
  const handle = await open(join(directory, 'index.jsonl'), 'a', 0o600);
  try { await handle.writeFile(`${JSON.stringify(record)}\n`); await handle.sync(); } finally { await handle.close(); }
  await syncDirectory(directory);
}

async function* indexRecords(directory) {
  const stream = createReadStream(join(directory, 'index.jsonl'));
  const lines = createInterface({ input: stream, crlfDelay: Infinity });
  try { for await (const line of lines) { if (line) yield JSON.parse(line); } } finally { lines.close(); stream.destroy(); }
}

async function fileDigest(file) {
  const sum = createHash('sha256');
  for await (const chunk of createReadStream(file)) sum.update(chunk);
  return sum.digest('hex');
}

function binding(options, collectionIds, scriptHash) {
  return Object.fromEntries(Object.entries({ version: VERSION, scriptHash, collectionIds, mongoIdentity: options.mongoIdentity, rpcIdentity: options.rpcIdentity, db: options.db, chainId: options.chainId, genesisHash: options.genesisHash, start: options.start, end: options.end, endHash: options.endHash, schema: options.schema }));
}

async function collectionIdentities(db, BSON) {
  const identities = {};
  for (const name of COLLECTIONS) {
    const rows = await db.listCollections({ name }, { nameOnly: false }).toArray();
    if (rows.length !== 1 || !rows[0].info?.uuid) throw new Error(`Missing collection UUID: ${name}`);
    identities[name] = encoded({ uuid: rows[0].info.uuid }, BSON).toString('base64');
  }
  return identities;
}

async function readRow(db, collection, id, BSON, session) {
  const raw = await db.collection(collection).findOne({ _id: id }, { raw: true, session, maxTimeMS: 10000 });
  return raw ? decode(Buffer.from(raw), BSON) : null;
}

async function observeRow(db, row, BSON, session) {
  const { before, after } = decodePlanRow(row, BSON);
  const current = await readRow(db, row.collection, before._id, BSON, session);
  if (current && sameDocument(current, before, BSON)) return 'before';
  if (current && sameDocument(current, after, BSON)) return 'after';
  throw new Error(`Intervening or missing ${row.collection} row; conditional repair stopped`);
}

async function mutateRow(db, row, direction, BSON, session) {
  const mutation = mutationFor(row, direction, BSON);
  const result = await db.collection(row.collection).updateOne(mutation.filter, mutation.update, { session, maxTimeMS: 10000, ...(session ? {} : { writeConcern: { w: 'majority', j: true } }) });
  if (result.matchedCount?.toString() !== '1') throw new Error(`CAS conflict in ${row.collection}`);
  const current = await readRow(db, row.collection, mutation.expected._id, BSON, session);
  if (!current || !sameDocument(current, mutation.desired, BSON)) throw new Error(`Post-write verification failed in ${row.collection}`);
}

async function rollbackBlock(db, plan, BSON, hooks = {}) {
  const failures = [];
  for (const row of [...plan.rows].reverse()) {
    try {
      const state = await observeRow(db, row, BSON);
      if (state === 'after' && row.beforeHash !== row.afterHash) {
        await hooks.beforeRollbackMutation?.({ collection: row.collection, height: plan.height });
        await mutateRow(db, row, 'reverse', BSON);
      }
    } catch (error) {
      failures.push(error);
    }
  }
  if (failures.length) throw new AggregateError(failures, 'Conditional rollback left conflicting or unresolved rows; inspect journal before resuming');
}

async function applyBlock(client, db, plan, BSON, options, hooks, useTransactions) {
  // Check every companion before the first write. On resume, a journaled
  // partial block may contain a mixture of exact before and exact after rows.
  for (const row of plan.rows) await observeRow(db, row, BSON);
  const apply = async (session) => {
    for (let index = 0; index < plan.rows.length; index++) {
      const row = plan.rows[index];
      const state = await observeRow(db, row, BSON, session);
      if (state === 'after' || row.beforeHash === row.afterHash) continue;
      await hooks.beforeMutation?.({ collection: row.collection, index, height: plan.height });
      await mutateRow(db, row, 'forward', BSON, session);
    }
  };
  try {
    if (useTransactions) {
      const session = client.startSession();
      try {
        await session.withTransaction(() => apply(session), { readConcern: { level: 'snapshot' }, writeConcern: { w: 'majority', j: true }, maxCommitTimeMS: 10000, timeoutMS: 30000 });
      } finally { await session.endSession(); }
    } else await apply();
  } catch (error) {
    // A lost acknowledgement can mean the write committed. Re-read exact
    // images before deciding. If observation itself fails, preserve the WAL
    // and stop; a later resume can safely resolve the uncertainty.
    let allAfter = true;
    for (const row of plan.rows) {
      try {
        const state = await observeRow(db, row, BSON);
        if (state !== 'after' && row.beforeHash !== row.afterHash) allAfter = false;
      } catch { allAfter = false; }
    }
    if (allAfter) return;
    try { await rollbackBlock(db, plan, BSON, hooks); }
    catch (rollbackError) { throw new AggregateError([error, rollbackError], 'Repair failed with conditional rollback conflicts; journal retained'); }
    throw error;
  }
  for (const row of plan.rows) {
    const state = await observeRow(db, row, BSON);
    if (state !== 'after' && row.beforeHash !== row.afterHash) throw new Error('Block did not reach its complete afterimage');
  }
}

async function readPlan(directory, record) {
  if (!/^block-[0-9]+\.json$/.test(record.file)) throw new Error('Invalid journal filename');
  const file = join(directory, record.file);
  if ((await stat(file)).size > MAX_PLAN_BYTES) throw new Error('Oversized journal block');
  const bytes = await readFile(file);
  if (digest(bytes) !== record.digest) throw new Error('Journal block checksum mismatch');
  const plan = JSON.parse(bytes);
  if (plan.height !== record.height || plan.hash !== record.hash) throw new Error('Journal block identity mismatch');
  if (plan.version !== VERSION || !Number.isInteger(plan.transactions) || plan.transactions < 1 || !Array.isArray(plan.rows) || plan.rows.length !== plan.transactions * 2 + 1) throw new Error('Invalid journal block shape');
  return plan;
}

async function preflightPlans(directory, manifest, options, BSON) {
  let previous = -1n;
  const totals = { blocks: 0, transactions: 0, skippedBlocks: 0, changedRows: 0 };
  for await (const record of indexRecords(directory)) {
    const height = decimal(record.height, 'journal height');
    if (height <= previous || height < BigInt(options.start) || height > BigInt(options.end)) throw new Error('Journal heights are unordered or outside the pinned range');
    previous = height;
    if (record.kind === 'skip') { totals.skippedBlocks++; continue; }
    if (record.kind !== 'block') throw new Error('Invalid journal record kind');
    const plan = await readPlan(directory, record);
    for (const row of plan.rows) {
      mutationFor(row, 'forward', BSON);
      mutationFor(row, 'reverse', BSON);
    }
    totals.blocks++;
    totals.transactions += plan.transactions;
    totals.changedRows += plan.rows.filter((row) => row.beforeHash !== row.afterHash).length;
  }
  for (const key of Object.keys(totals)) same(totals[key], manifest.summary[key], `sealed ${key} count`);
}

async function prepare(db, rpc, options, driver, target) {
  const { BSON, Double, Long } = driver;
  const execute = options.mode === 'prepare';
  if (execute) {
    await mkdir(options.journalDir, { mode: 0o700 });
    await syncDirectory(dirname(options.journalDir));
    await durableJSON(join(options.journalDir, 'manifest.json'), { binding: target, state: 'preparing' });
    const handle = await open(join(options.journalDir, 'index.jsonl'), 'wx', 0o600);
    try { await handle.sync(); } finally { await handle.close(); }
    await syncDirectory(options.journalDir);
  }
  const summary = { mode: options.mode, blocks: 0, transactions: 0, skippedBlocks: 0, changedRows: 0 };
  const cursor = db.collection('blocks').find({ blockNumberInt: { $gte: Long.fromString(options.start), $lte: Long.fromString(options.end) }, 'result.transactions.0': { $exists: true } }, { raw: true, sort: { blockNumberInt: 1, _id: 1 }, batchSize: 16 });
  let lastHeight = null;
  try {
    for await (const raw of cursor) {
      const stored = decode(Buffer.from(raw), BSON);
      const height = stored.blockNumberInt?.toString();
      if (height === lastHeight) continue;
      lastHeight = height;
      let record;
      try {
        decimal(height, 'stored height');
        const sameHeight = await db.collection('blocks').countDocuments({ $or: [{ blockNumberInt: stored.blockNumberInt }, { 'result.number': stored.result.number }] }, { limit: 2, maxTimeMS: 10000 });
        if (sameHeight.toString() !== '1') throw new DataError('Duplicate stored block height');
        if (options.schema === 'durable' && stored.ingestionState !== 'complete') throw new DataError('Durable block is not complete');
        const block = await rpc('qrl_getBlockByNumber', [`0x${BigInt(height).toString(16)}`, true]);
        if (!Array.isArray(block.transactions)) throw new Error('RPC full block omitted transactions');
        if (hash(block.hash) !== hash(stored.result?.hash)) throw new DataError('Stored block is no longer canonical');
        const receipts = [];
        const histories = [];
        const transfers = [];
        for (const tx of block.transactions) {
          const txHash = hash(tx.hash);
          const find = async (collection) => {
            const found = await db.collection(collection).find({ txHash: { $regex: `^${txHash}$`, $options: 'i' }, blockNumber: stored.result.number }, { raw: true, limit: 2, batchSize: 2, maxTimeMS: 10000 }).toArray();
            if (found.length !== 1) throw new DataError(`Expected exactly one ${collection} companion`);
            return Buffer.from(found[0]);
          };
          histories.push(await find('transactionByAddress'));
          transfers.push(await find('transfer'));
          receipts.push(await rpc('qrl_getTransactionReceipt', [txHash]));
        }
        const plan = planBlock({ blockRaw: Buffer.from(raw), block, receipts, histories, transfers, options, BSON, Double });
        for (const row of plan.rows) {
          mutationFor(row, 'forward', BSON);
          mutationFor(row, 'reverse', BSON);
        }
        const file = `block-${height}.json`;
        const bytes = Buffer.from(JSON.stringify(plan));
        if (bytes.length > MAX_PLAN_BYTES) throw new DataError('Block plan exceeds bounded journal size');
        record = { kind: 'block', height, hash: plan.hash, file, digest: digest(bytes) };
        if (execute) await durableJSON(join(options.journalDir, file), plan);
        summary.blocks++;
        summary.transactions += plan.transactions;
        summary.changedRows += plan.rows.filter((row) => row.beforeHash !== row.afterHash).length;
      } catch (error) {
        if (!(error instanceof DataError)) throw error;
        summary.skippedBlocks++;
        record = { kind: 'skip', height, reason: error.message };
        console.log(JSON.stringify({ skippedBlock: height, reason: error.message }));
      }
      if (execute) await appendIndex(options.journalDir, record);
    }
  } finally { await cursor.close(); }
  await verifyNetwork(rpc, options);
  if (execute) await durableJSON(join(options.journalDir, 'manifest.json'), { binding: target, state: 'prepared', summary, indexHash: await fileDigest(join(options.journalDir, 'index.jsonl')) });
  return summary;
}

export async function run(input, hooks = {}) {
  const options = validateOptions(input);
  const require = createRequire(join(resolve(options.driverRoot), 'package.json'));
  const driver = require('mongodb');
  const { BSON, MongoClient } = driver;
  const client = new MongoClient(options.mongoUri, { promoteValues: false, serverSelectionTimeoutMS: 10000, connectTimeoutMS: 10000, socketTimeoutMS: 15000, maxPoolSize: 2, minPoolSize: 0 });
  try {
    await client.connect();
    const db = client.db(options.db);
    const hello = await client.db('admin').command({ hello: 1 });
    const useTransactions = !!hello.setName || hello.msg === 'isdbgrid';
    if (options.schema === 'durable' && !useTransactions) throw new Error('Durable schema requires replica-set or mongos topology');
    const ids = await collectionIdentities(db, BSON);
    const target = binding(options, ids, digest(await readFile(SCRIPT)));
    const rpc = rpcClient(options);
    await verifyNetwork(rpc, options);
    if (['dry-run', 'prepare'].includes(options.mode)) return await prepare(db, rpc, options, driver, target);
    const manifest = JSON.parse(await readFile(join(options.journalDir, 'manifest.json'), 'utf8'));
    assert.deepEqual(manifest.binding, target, 'Journal target, UUID, range, or tool identity changed');
    if (manifest.state !== 'prepared') throw new Error('Only a fully prepared sealed journal can be applied');
    if (await fileDigest(join(options.journalDir, 'index.jsonl')) !== manifest.indexHash) throw new Error('Journal index checksum mismatch');
    if (options.mode === 'apply' && manifest.summary.skippedBlocks > 0 && !options.acceptSkips) throw new Error('Prepared plan excludes blocks; review index and pass --accept-skips explicitly');
    await preflightPlans(options.journalDir, manifest, options, BSON);
    const summary = { mode: options.mode, blocks: 0, transactions: 0, skippedBlocks: manifest.summary.skippedBlocks };
    let previous = -1n;
    for await (const record of indexRecords(options.journalDir)) {
      const height = decimal(record.height, 'journal height');
      if (height <= previous || height < BigInt(options.start) || height > BigInt(options.end)) throw new Error('Journal heights are unordered or outside the pinned range');
      previous = height;
      if (record.kind === 'skip') continue;
      if (record.kind !== 'block') throw new Error('Invalid journal record kind');
      const plan = await readPlan(options.journalDir, record);
      if (options.mode === 'apply') await applyBlock(client, db, plan, BSON, options, hooks, useTransactions);
      else if (options.mode === 'rollback') await rollbackBlock(db, plan, BSON, hooks);
      else {
        for (const row of plan.rows) {
          const state = await observeRow(db, row, BSON);
          if (state !== 'after' && row.beforeHash !== row.afterHash) throw new Error(`Unrepaired row at block ${plan.height}`);
        }
      }
      summary.blocks++;
      summary.transactions += plan.transactions;
      if (options.mode !== 'verify') await durableJSON(join(options.journalDir, 'progress.json'), { binding: target, ...summary, lastHeight: plan.height });
    }
    await verifyNetwork(rpc, options);
    assert.deepEqual(await collectionIdentities(db, BSON), ids, 'Target collection identity changed during maintenance');
    if (options.mode !== 'verify') await durableJSON(join(options.journalDir, 'progress.json'), { binding: target, ...summary, complete: true });
    return summary;
  } finally { await client.close(); }
}

export function parseArgs(args) {
  const options = {};
  const boolean = new Set(['writers-paused', 'readers-paused', 'accept-skips']);
  const known = new Set(['mode', 'driver-root', 'rpc-url', 'mongo-uri', 'db', 'chain-id', 'genesis-hash', 'start', 'end', 'end-hash', 'schema', 'journal-dir', 'rpc-delay-ms', ...boolean]);
  for (let index = 0; index < args.length; index++) {
    const name = args[index].replace(/^--/, '');
    if (!args[index].startsWith('--') || !known.has(name)) throw new Error(`Unknown option: ${args[index]}`);
    const key = name.replace(/-([a-z])/g, (_, letter) => letter.toUpperCase());
    if (Object.hasOwn(options, key)) throw new Error(`Duplicate option: ${name}`);
    if (boolean.has(name)) options[key] = true;
    else {
      if (args[index + 1] === undefined || args[index + 1].startsWith('--')) throw new Error(`Missing value: ${name}`);
      options[key] = args[++index];
    }
  }
  return options;
}

if (process.argv[1] && resolve(process.argv[1]) === SCRIPT) {
  run(parseArgs(process.argv.slice(2))).then((summary) => console.log(JSON.stringify(summary))).catch((error) => {
    // Driver errors can contain a configured URI. Keep CLI diagnostics bounded
    // to reviewed messages; operational details remain in the private journal.
    console.error(error instanceof DataError || error.name === 'AssertionError' ? error.message.split('\n')[0] : String(error.message).replace(/mongodb:\/\/\S+/g, '[MongoDB endpoint]'));
    process.exitCode = 1;
  });
}
