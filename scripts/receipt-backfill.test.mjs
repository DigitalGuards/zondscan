import assert from 'node:assert/strict';
import { createHash } from 'node:crypto';
import { createRequire } from 'node:module';
import { test } from 'node:test';
import { exactFeeDecimal, mutationFor, parseArgs, planBlock, quantity, validateOptions } from './receipt-backfill.mjs';

const require = createRequire(new URL('../ExplorerFrontend/package.json', import.meta.url));
const { BSON, Binary, Double, Int32, Long, ObjectId } = require('mongodb');
const blockHash = `0x${'11'.repeat(32)}`;
const transactionHash = `0x${'22'.repeat(32)}`;
const baseOptions = {
  schema: 'legacy', rpcUrl: 'http://127.0.0.1:12345', mongoUri: 'mongodb://127.0.0.1:12346',
  db: 'repair_fixture', chainId: '17', genesisHash: `0x${'33'.repeat(32)}`,
  start: '1', end: '1', endHash: blockHash,
};

function decode(raw) { return BSON.deserialize(raw, { promoteValues: false }); }
function fixture(schema = 'legacy') {
  const width = schema === 'legacy' ? 40 : 128;
  const from = `Q${'a'.repeat(width)}`;
  const to = `Q${'b'.repeat(width)}`;
  const options = { ...baseOptions, schema };
  const tx = {
    hash: transactionHash, blockHash, blockNumber: '0x1', transactionIndex: '0x0', chainId: '0x11',
    from, to, gas: '0x186a0', gasPrice: '0x999', value: '0x5', nonce: '0x0', input: '0x1234',
  };
  const stored = {
    _id: new ObjectId('111111111111111111111111'), blockNumberInt: Long.fromString('1'),
    untouched: {
      small: Long.fromString('42'), large: Long.fromString('9007199254740993'),
      integer: new Int32(1), double: new Double(1), binary: new Binary(Buffer.from([1, 2, 3])),
      date: new Date('2020-01-01T00:00:00Z'), nullable: null, array: [new Int32(2), 'retained'],
    },
    result: { number: '0x1', hash: blockHash, transactions: [{
      hash: transactionHash, blockhash: blockHash, blocknumber: '0x1', transactionindex: '0x0', chainid: '0x11',
      from, to, gas: tx.gas, gasprice: tx.gasPrice, value: tx.value, nonce: tx.nonce,
      data: '', status: '', signature: 'preserved signature', publickey: 'preserved public key',
    }] },
  };
  if (schema === 'durable') Object.assign(stored, { ingestionState: 'complete', tokenIngestionState: 'pending', tokenAttempts: new Int32(3) });
  const history = { _id: new ObjectId('222222222222222222222222'), txHash: transactionHash, blockNumber: '0x1', from, to, amount: new Double(3), paidFees: new Double(99), metadata: { preserve: true } };
  const transfer = { _id: new ObjectId('333333333333333333333333'), txHash: transactionHash, blockNumber: '0x1', from, to, status: '', paidFees: new Double(99), value: new Double(3), signature: 'keep' };
  return {
    options, BSON, Double, blockRaw: BSON.serialize(stored),
    block: { number: '0x1', hash: blockHash, transactions: [tx] },
    receipts: [{ transactionHash, blockHash, blockNumber: '0x1', transactionIndex: '0x0', from, to, status: '0x0', gasUsed: '0x5208', effectiveGasPrice: '0x7' }],
    histories: [BSON.serialize(history)], transfers: [BSON.serialize(transfer)],
  };
}

for (const schema of ['legacy', 'durable']) {
  test(`${schema} planning preserves BSON metadata and approved exact receipt fields`, () => {
    const source = fixture(schema);
    const plan = planBlock(source);
    assert.equal(plan.rows.length, 3);
    const history = decode(Buffer.from(plan.rows[0].after, 'base64'));
    assert.equal(history.paidFeesWei, '147000');
    assert.equal(history.feeSource, 'receipt');
    assert.equal(history.receiptStatus, '0x0');
    assert.equal(history.paidFees._bsontype, 'Double');
    assert.equal(history.paidFees.valueOf(), Number('0.000000000000147000'));
    const block = decode(Buffer.from(plan.rows[2].after, 'base64'));
    const old = decode(source.blockRaw);
    assert.deepEqual(BSON.serialize(block.untouched), BSON.serialize(old.untouched));
    assert.equal(block.result.transactions[0].gas, '0x186a0');
    assert.equal(block.result.transactions[0].gasprice, '0x999');
    assert.equal(block.result.transactions[0].signature, 'preserved signature');
    assert.equal(block.result.transactions[0].publickey, 'preserved public key');
    assert.equal(block.result.transactions[0].data, '0x1234');
    assert.equal(block.result.transactions[0].gasUsed, '0x5208');
    assert.equal(block.result.transactions[0].effectiveGasPrice, '0x7');
    assert.equal(block.ingestionState, old.ingestionState);
    assert.equal(block.tokenIngestionState, old.tokenIngestionState);
    assert.deepEqual(Object.keys(block.result.transactions[0]).slice(-2), ['effectiveGasPrice', 'gasUsed']);
    for (const row of plan.rows) {
      assert.doesNotThrow(() => mutationFor(row, 'forward', BSON));
      assert.doesNotThrow(() => mutationFor(row, 'reverse', BSON));
    }
  });
}

test('zero-priced failed execution remains an exact zero fee and known empty input', () => {
  const source = fixture();
  source.receipts[0].effectiveGasPrice = '0x0';
  source.block.transactions[0].input = '0x';
  const plan = planBlock(source);
  const history = decode(Buffer.from(plan.rows[0].after, 'base64'));
  const transfer = decode(Buffer.from(plan.rows[1].after, 'base64'));
  assert.equal(history.paidFeesWei, '0');
  assert.equal(history.paidFees.valueOf(), 0);
  assert.equal(history.receiptStatus, '0x0');
  assert.equal(transfer.data, '0x');
});

test('large exact fee converts through one decimal-to-double boundary', () => {
  const source = fixture();
  source.receipts[0].effectiveGasPrice = '0x123456789abcdef123456789';
  const fee = 21000n * BigInt(source.receipts[0].effectiveGasPrice);
  const history = decode(Buffer.from(planBlock(source).rows[0].after, 'base64'));
  assert.equal(history.paidFeesWei, fee.toString());
  assert.equal(history.paidFees.valueOf(), Number(exactFeeDecimal(fee)));
});

test('receipt identity, quantities and calldata are validated before planning', async (t) => {
  const cases = [
    ['hash', (s) => { s.receipts[0].transactionHash = `0x${'44'.repeat(32)}`; }],
    ['block', (s) => { s.receipts[0].blockHash = `0x${'44'.repeat(32)}`; }],
    ['height', (s) => { s.receipts[0].blockNumber = '0x2'; }],
    ['index', (s) => { s.receipts[0].transactionIndex = '0x1'; }],
    ['sender', (s) => { s.receipts[0].from = `Q${'c'.repeat(40)}`; }],
    ['recipient', (s) => { s.receipts[0].to = `Q${'c'.repeat(40)}`; }],
    ['status', (s) => { s.receipts[0].status = '0x2'; }],
    ['over limit', (s) => { s.receipts[0].gasUsed = '0x186a1'; }],
    ['missing gas', (s) => { delete s.receipts[0].gasUsed; }],
    ['missing price', (s) => { delete s.receipts[0].effectiveGasPrice; }],
    ['oversized price', (s) => { s.receipts[0].effectiveGasPrice = `0x1${'0'.repeat(64)}`; }],
    ['missing input', (s) => { delete s.block.transactions[0].input; }],
    ['odd input', (s) => { s.block.transactions[0].input = '0x1'; }],
    ['transaction chain', (s) => { s.block.transactions[0].chainId = '0x12'; }],
    ['transaction count', (s) => { s.block.transactions.push(s.block.transactions[0]); }],
    ['missing receipt', (s) => { s.receipts = []; }],
    ['missing companion', (s) => { s.histories = []; }],
  ];
  for (const [name, mutate] of cases) await t.test(name, () => {
    const source = fixture(); mutate(source);
    assert.throws(() => planBlock(source));
  });
});

test('durable planning rejects incomplete blocks without changing markers', () => {
  const source = fixture('durable');
  const before = decode(source.blockRaw);
  before.ingestionState = 'pending';
  source.blockRaw = BSON.serialize(before);
  assert.throws(() => planBlock(source), /completion/);
});

test('legacy planning rejects schema mixing and compares address encodings without rewriting', () => {
  const source = fixture();
  source.receipts[0].from = source.receipts[0].from.replace('Q', '0x');
  const plan = planBlock(source);
  assert.equal(decode(Buffer.from(plan.rows[2].after, 'base64')).result.transactions[0].from, `Q${'a'.repeat(40)}`);
  const stored = decode(source.blockRaw);
  stored.ingestionState = 'complete';
  source.blockRaw = BSON.serialize(stored);
  assert.throws(() => planBlock(source), /Legacy block/);
});

test('contract creation validates the preserved companion creation address', () => {
  const source = fixture();
  const created = `Q${'c'.repeat(40)}`;
  source.block.transactions[0].to = null;
  source.receipts[0].to = null;
  source.receipts[0].contractAddress = created;
  const stored = decode(source.blockRaw);
  stored.result.transactions[0].to = '';
  source.blockRaw = BSON.serialize(stored);
  const history = decode(source.histories[0]); history.to = '';
  source.histories[0] = BSON.serialize(history);
  const transfer = decode(source.transfers[0]); delete transfer.to; transfer.contractAddress = created;
  source.transfers[0] = BSON.serialize(transfer);
  assert.doesNotThrow(() => planBlock(source));
  source.receipts[0].contractAddress = `Q${'d'.repeat(40)}`;
  assert.throws(() => planBlock(source), /created contract/);
});

test('CAS uses explicit array indexing and preserves absent-versus-null rollback', () => {
  const source = fixture();
  const transfer = decode(source.transfers[0]); transfer.data = null;
  source.transfers[0] = BSON.serialize(transfer);
  const plan = planBlock(source);
  const blockMutation = mutationFor(plan.rows[2], 'forward', BSON);
  const filter = JSON.stringify(BSON.EJSON.serialize(blockMutation.filter, { relaxed: false }));
  assert.match(filter, /\$arrayElemAt/);
  assert.match(filter, /\$getField/);
  assert.match(filter, /\$type/);
  assert.match(filter, /\$exists/);
  assert.ok(!filter.includes('"$type":"$result.transactions.0'));
  const reverseTransfer = mutationFor(plan.rows[1], 'reverse', BSON);
  assert.equal(reverseTransfer.update.$set.data, null);
  const reverseHistory = mutationFor(plan.rows[0], 'reverse', BSON);
  assert.equal(reverseHistory.update.$unset.feeSource, '');
  assert.equal(reverseHistory.update.$unset.paidFeesWei, '');
});

test('plan checksums and patch allowlist reject corrupted or expanded afterimages', () => {
  const row = planBlock(fixture()).rows[0];
  assert.throws(() => mutationFor({ ...row, beforeHash: 'bad' }, 'forward', BSON), /checksum/);
  assert.throws(() => mutationFor({ ...row, paths: [...row.paths, 'amount'] }, 'forward', BSON), /Unapproved/);
  const after = decode(Buffer.from(row.after, 'base64')); after.amount = new Double(900);
  const raw = BSON.serialize(after);
  assert.throws(() => mutationFor({ ...row, after: raw.toString('base64'), afterHash: createHash('sha256').update(raw).digest('hex') }, 'forward', BSON), /outside/);
});

test('options default to read-only and require exact identity, loopback and pause gates', async (t) => {
  assert.equal(validateOptions(baseOptions).mode, 'dry-run');
  for (const key of ['db', 'chainId', 'genesisHash', 'start', 'end', 'endHash']) await t.test(`missing ${key}`, () => {
    const options = { ...baseOptions }; delete options[key];
    assert.throws(() => validateOptions(options), /Missing/);
  });
  assert.throws(() => validateOptions({ ...baseOptions, rpcUrl: 'https://example.com' }), /loopback/);
  assert.throws(() => validateOptions({ ...baseOptions, mongoUri: 'mongodb://example.com' }), /loopback/);
  assert.throws(() => validateOptions({ ...baseOptions, mongoUri: 'mongodb://127.0.0.1/other' }), /database/);
  assert.throws(() => validateOptions({ ...baseOptions, start: '2' }), /Start/);
  const write = { ...baseOptions, mode: 'apply', journalDir: '/tmp/repair-fixture' };
  assert.throws(() => validateOptions(write), /writers-paused/);
  assert.throws(() => validateOptions({ ...write, writersPaused: true }), /readers-paused/);
  assert.throws(() => validateOptions({ ...write, schema: 'durable', writersPaused: true }), /readers-paused/);
});

test('CLI refuses unknown, duplicate and valueless options', () => {
  assert.deepEqual(parseArgs(['--mode', 'prepare', '--writers-paused', '--readers-paused', '--accept-skips']), { mode: 'prepare', writersPaused: true, readersPaused: true, acceptSkips: true });
  assert.throws(() => parseArgs(['--force']), /Unknown/);
  assert.throws(() => parseArgs(['--mode', 'verify', '--mode', 'apply']), /Duplicate/);
  assert.throws(() => parseArgs(['--mode']), /Missing/);
});

test('quantity parser rejects noncanonical or oversized fields', () => {
  assert.equal(quantity('0x0'), 0n);
  for (const value of ['', '0x', '0x00', '-1', '0x-1', `0x1${'0'.repeat(64)}`]) assert.throws(() => quantity(value));
});
