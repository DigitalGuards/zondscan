import assert from 'node:assert/strict';
import { execFile } from 'node:child_process';
import { randomUUID } from 'node:crypto';
import { once } from 'node:events';
import { mkdtemp, readFile, readdir } from 'node:fs/promises';
import { createServer } from 'node:http';
import { createRequire } from 'node:module';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import test from 'node:test';
import { fileURLToPath } from 'node:url';
import { run as runBackfill } from './receipt-backfill.mjs';

const driverRoot = fileURLToPath(
  new URL('../ExplorerFrontend', import.meta.url),
);
const require = createRequire(join(driverRoot, 'package.json'));
const {
  BSON,
  Binary,
  Decimal128,
  Double,
  Int32,
  Long,
  MongoClient,
  ObjectId,
} = require('mongodb');
const mongoUri = process.env.RECEIPT_BACKFILL_TEST_MONGO_URI;
const replicaUri = process.env.RECEIPT_BACKFILL_TEST_REPLICA_URI;
const genesisHash = `0x${'a'.repeat(64)}`;
const blockHash = `0x${'b'.repeat(64)}`;
const txHash = `0x${'c'.repeat(64)}`;
const collections = ['blocks', 'transactionByAddress', 'transfer', 'untouched'];

async function run(options, hooks) {
  try {
    return await runBackfill(options, hooks);
  } catch (error) {
    if (error instanceof AggregateError) {
      error.message += `: ${error.errors.map((cause) => cause.message).join('; ')}`;
    }
    throw error;
  }
}

function canonical(value) {
  const sort = (item) => {
    if (Array.isArray(item)) return item.map(sort);
    if (!item || typeof item !== 'object') return item;
    return Object.fromEntries(
      Object.keys(item)
        .sort()
        .map((key) => [key, sort(item[key])]),
    );
  };
  return sort(BSON.EJSON.serialize(value, { relaxed: false }));
}

function metadata() {
  return {
    safeLong: Long.fromString('42'),
    unsafeLong: Long.fromString('9007199254740993'),
    int: new Int32(42),
    double: new Double(42),
    decimal: Decimal128.fromString('42.000000000000000001'),
    binary: new Binary(Buffer.from([0, 1, 255])),
    date: new Date('2026-01-01T00:00:00.000Z'),
    objectId: new ObjectId(),
    nullable: null,
    nested: [{ preserved: new Long(7), enabled: true }],
  };
}

async function fixture(
  t,
  {
    schema = 'legacy',
    status = '0x0',
    price = '0x7',
    transactionCount = 1,
  } = {},
) {
  const uri = schema === 'durable' ? replicaUri : mongoUri;
  const parsed = new URL(uri);
  assert.equal(parsed.protocol, 'mongodb:');
  assert.ok(
    ['127.0.0.1', '[::1]'].includes(parsed.hostname),
    'Integration Mongo must be loopback',
  );
  const database = `receipt_backfill_test_${randomUUID().replaceAll('-', '')}`;
  const client = new MongoClient(uri, {
    promoteValues: false,
    serverSelectionTimeoutMS: 2000,
  });
  await client.connect();
  const db = client.db(database);
  t.after(async () => {
    assert.match(database, /^receipt_backfill_test_[0-9a-f]{32}$/);
    await db.dropDatabase();
    await client.close();
  });
  const addressBytes = schema === 'legacy' ? 20 : 64;
  const from = `Q${'11'.repeat(addressBytes)}`;
  const to = `Q${'22'.repeat(addressBytes)}`;
  const tx = {
    hash: txHash,
    blockHash,
    blockNumber: '0x1',
    transactionIndex: '0x0',
    from,
    to,
    gas: '0x186a0',
    gasPrice: '0x9',
    value: '0x1',
    nonce: '0x0',
    chainId: '0x539',
    input: '0x12345678',
  };
  const receipt = {
    transactionHash: txHash,
    blockHash,
    blockNumber: '0x1',
    transactionIndex: '0x0',
    from,
    to,
    status,
    gasUsed: '0x5208',
    effectiveGasPrice: price,
  };
  await db.collection('blocks').insertOne({
    _id: new ObjectId(),
    blockNumberInt: Long.fromNumber(1),
    metadata: metadata(),
    ...(schema === 'durable'
      ? { ingestionState: 'complete', tokenIngestionState: 'complete' }
      : {}),
    result: {
      number: '0x1',
      hash: blockHash,
      parenthash: genesisHash,
      timestamp: '0x64',
      transactions: [
        {
          hash: txHash,
          blockhash: blockHash,
          blocknumber: '0x1',
          transactionindex: '0x0',
          from,
          to,
          gas: tx.gas,
          gasprice: tx.gasPrice,
          value: tx.value,
          nonce: tx.nonce,
          chainid: tx.chainId,
          data: null,
          status: '0x1',
          metadata: metadata(),
        },
      ],
    },
  });
  await db.collection('transactionByAddress').insertOne({
    _id: new ObjectId(),
    txHash,
    blockNumber: '0x1',
    from,
    to,
    amount: new Double(1e-18),
    amountWei: '1',
    paidFees: new Double(17.25),
    receiptStatus: null,
    metadata: metadata(),
  });
  await db.collection('transfer').insertOne({
    _id: new ObjectId(),
    txHash,
    blockNumber: '0x1',
    from,
    to,
    value: new Double(1e-18),
    paidFees: new Double(17.25),
    status: '0x1',
    metadata: metadata(),
  });
  await db
    .collection('untouched')
    .insertOne({ _id: new ObjectId(), metadata: metadata() });
  const transactions = [tx];
  const receipts = [receipt];
  for (let index = 1; index < transactionCount; index++) {
    const extraHash = `0x${index.toString(16).padStart(64, '0')}`;
    const position = `0x${index.toString(16)}`;
    transactions.push({ ...tx, hash: extraHash, transactionIndex: position });
    receipts.push({
      ...receipt,
      transactionHash: extraHash,
      transactionIndex: position,
    });
    const stored = (await db.collection('blocks').findOne({})).result
      .transactions[0];
    await db.collection('blocks').updateOne(
      {},
      {
        $push: {
          'result.transactions': {
            ...stored,
            hash: extraHash,
            transactionindex: position,
          },
        },
      },
    );
    for (const name of ['transactionByAddress', 'transfer']) {
      const row = await db.collection(name).findOne({ txHash });
      await db
        .collection(name)
        .insertOne({ ...row, _id: new ObjectId(), txHash: extraHash });
    }
  }
  const requests = [];
  const server = createServer(async (request, response) => {
    let body = '';
    for await (const chunk of request) body += chunk;
    const message = JSON.parse(body);
    requests.push(message.method);
    let result;
    if (message.method === 'qrl_chainId') result = '0x539';
    else if (
      message.method === 'qrl_getBlockByNumber' &&
      message.params[0] === '0x0'
    ) {
      result = { number: '0x0', hash: genesisHash, transactions: [] };
    } else if (
      message.method === 'qrl_getBlockByNumber' &&
      message.params[0] === '0x1'
    ) {
      result = {
        number: '0x1',
        hash: blockHash,
        transactions: message.params[1]
          ? transactions
          : transactions.map((item) => item.hash),
      };
    } else if (
      message.method === 'qrl_getTransactionReceipt' &&
      receipts.some((item) => item.transactionHash === message.params[0])
    ) {
      result = receipts.find(
        (item) => item.transactionHash === message.params[0],
      );
    } else {
      response.writeHead(400).end();
      return;
    }
    response.writeHead(200, { 'content-type': 'application/json' });
    response.end(JSON.stringify({ jsonrpc: '2.0', id: message.id, result }));
  });
  server.listen(0, '127.0.0.1');
  await once(server, 'listening');
  t.after(() => new Promise((resolve) => server.close(resolve)));
  const directory = await mkdtemp(
    join(tmpdir(), 'receipt-backfill-integration-'),
  );
  const options = {
    mongoUri: uri,
    db: database,
    driverRoot,
    rpcUrl: `http://127.0.0.1:${server.address().port}`,
    chainId: '1337',
    genesisHash,
    endHash: blockHash,
    start: '1',
    end: '1',
    schema,
    journalDir: join(directory, 'journal'),
    writersPaused: true,
    readersPaused: true,
    rpcDelayMs: 0,
  };
  const snapshot = async () =>
    Object.fromEntries(
      await Promise.all(
        collections.map(async (name) => [
          name,
          canonical(
            await db.collection(name).find({}).sort({ _id: 1 }).toArray(),
          ),
        ]),
      ),
    );
  const rawSnapshot = async () =>
    Object.fromEntries(
      await Promise.all(
        collections.map(async (name) => [
          name,
          (
            await db
              .collection(name)
              .find({}, { raw: true })
              .sort({ _id: 1 })
              .toArray()
          ).map((raw) => Buffer.from(raw)),
        ]),
      ),
    );
  const prepare = async () => {
    const summary = await run({ ...options, mode: 'prepare' });
    assert.equal(
      summary.blocks,
      1,
      'A real Mongo fixture must produce a nonempty plan',
    );
    assert.equal(summary.transactions, transactionCount);
    assert.equal(summary.skippedBlocks, 0);
    return summary;
  };
  return {
    db,
    options,
    snapshot,
    rawSnapshot,
    prepare,
    receipt,
    tx,
    requests,
    directory,
  };
}

function withoutRepairedFields(snapshot) {
  const result = structuredClone(snapshot);
  for (const block of result.blocks) {
    for (const tx of block.result.transactions) {
      for (const key of ['data', 'status', 'gasUsed', 'effectiveGasPrice'])
        delete tx[key];
    }
  }
  for (const row of result.transactionByAddress) {
    for (const key of ['paidFees', 'paidFeesWei', 'feeSource', 'receiptStatus'])
      delete row[key];
  }
  for (const row of result.transfer) {
    for (const key of ['data', 'status', 'paidFees']) delete row[key];
  }
  return result;
}

const enabled = { skip: !mongoUri, concurrency: false, timeout: 30000 };

test(
  'eleven transactions preserve numeric array positions and byte-exact rollback',
  enabled,
  async (t) => {
    const f = await fixture(t, { transactionCount: 11 });
    const before = await f.snapshot();
    const beforeRaw = await f.rawSnapshot();
    await f.prepare();
    await run({ ...f.options, mode: 'apply' });
    await run({ ...f.options, mode: 'verify' });
    assert.deepEqual(
      withoutRepairedFields(await f.snapshot()),
      withoutRepairedFields(before),
    );
    const transactions = (await f.db.collection('blocks').findOne({})).result
      .transactions;
    for (const index of [2, 10]) {
      assert.equal(
        transactions[index].transactionindex,
        `0x${index.toString(16)}`,
      );
      assert.equal(transactions[index].gasUsed, '0x5208');
      assert.equal(transactions[index].effectiveGasPrice, '0x7');
    }
    await run({ ...f.options, mode: 'rollback' });
    assert.deepEqual(await f.rawSnapshot(), beforeRaw);
  },
);

test(
  'real Mongo dry-run and prepare leave all BSON documents unchanged',
  enabled,
  async (t) => {
    const f = await fixture(t);
    const before = await f.snapshot();
    const beforeRaw = await f.rawSnapshot();
    const dry = await run({ ...f.options, mode: 'dry-run' });
    assert.equal(dry.blocks, 1);
    assert.equal(dry.skippedBlocks, 0);
    assert.deepEqual(await readdir(f.directory), []);
    assert.deepEqual(await f.snapshot(), before);
    await f.prepare();
    assert.deepEqual(await f.snapshot(), before);
    assert.deepEqual(await f.rawSnapshot(), beforeRaw);
    assert.ok(!(await readdir(f.options.journalDir)).includes('progress.json'));
  },
);

test(
  'standalone apply, verify, repeat and rollback preserve BSON types and metadata',
  enabled,
  async (t) => {
    const f = await fixture(t);
    const before = await f.snapshot();
    const beforeRaw = await f.rawSnapshot();
    await f.prepare();
    await run({ ...f.options, mode: 'apply' });
    const after = await f.snapshot();
    assert.deepEqual(
      withoutRepairedFields(after),
      withoutRepairedFields(before),
    );
    const row = await f.db.collection('transactionByAddress').findOne({});
    assert.equal(row.paidFeesWei, '147000');
    assert.equal(row.paidFees._bsontype, 'Double');
    assert.equal(row.receiptStatus, '0x0');
    assert.equal(row.feeSource, 'receipt');
    const block = await f.db.collection('blocks').findOne({});
    assert.equal(Object.hasOwn(block, 'ingestionState'), false);
    assert.equal(block.blockNumberInt._bsontype, 'Long');
    assert.equal(block.result.transactions[0].data, f.tx.input);
    assert.equal(block.result.transactions[0].gasUsed, '0x5208');
    await run({ ...f.options, mode: 'verify' });
    await run({ ...f.options, mode: 'apply' });
    assert.deepEqual(await f.snapshot(), after);
    await run({ ...f.options, mode: 'rollback' });
    assert.deepEqual(
      await f.rawSnapshot(),
      beforeRaw,
      'Rollback restores byte-exact BSON documents',
    );
    assert.deepEqual(
      await f.snapshot(),
      before,
      'Rollback restores null, absence and exact BSON numeric types',
    );
  },
);

test(
  'zero-priced successful receipt remains exactly zero',
  enabled,
  async (t) => {
    const f = await fixture(t, { status: '0x1', price: '0x0' });
    await f.prepare();
    await run({ ...f.options, mode: 'apply' });
    const row = await f.db.collection('transactionByAddress').findOne({});
    assert.equal(row.paidFeesWei, '0');
    assert.equal(row.paidFees.valueOf(), 0);
    assert.equal(row.receiptStatus, '0x1');
  },
);

test(
  'missing or duplicate companions are recorded exceptions with no writes',
  enabled,
  async (t) => {
    for (const kind of ['missing', 'duplicate']) {
      const f = await fixture(t);
      if (kind === 'missing') await f.db.collection('transfer').deleteOne({});
      else {
        const row = await f.db.collection('transfer').findOne({});
        await f.db
          .collection('transfer')
          .insertOne({ ...row, _id: new ObjectId() });
      }
      const before = await f.snapshot();
      const summary = await run({ ...f.options, mode: 'prepare' });
      assert.equal(summary.blocks, 0);
      assert.equal(summary.skippedBlocks, 1);
      await assert.rejects(
        run({ ...f.options, mode: 'apply' }),
        /accept-skips/,
      );
      await run({ ...f.options, mode: 'apply', acceptSkips: true });
      assert.deepEqual(await f.snapshot(), before);
    }
  },
);

test(
  'an ordinary mutation failure rolls back its committed standalone prefix',
  enabled,
  async (t) => {
    const f = await fixture(t);
    const before = await f.snapshot();
    const beforeRaw = await f.rawSnapshot();
    await f.prepare();
    await assert.rejects(
      run(
        { ...f.options, mode: 'apply' },
        {
          beforeMutation({ index }) {
            if (index === 1) throw new Error('Injected ordinary interruption');
          },
        },
      ),
      /Injected ordinary interruption/,
    );
    assert.deepEqual(await f.snapshot(), before);
    assert.deepEqual(await f.rawSnapshot(), beforeRaw);
    await run({ ...f.options, mode: 'apply' });
    await run({ ...f.options, mode: 'verify' });
  },
);

test(
  'real CAS preserves a concurrently changed value and refuses third-state rollback',
  enabled,
  async (t) => {
    for (const stopAt of [0, 1]) {
      const f = await fixture(t);
      const before = await f.snapshot();
      const conflictingCollection =
        stopAt === 0 ? 'transactionByAddress' : 'transfer';
      await f.prepare();
      await assert.rejects(
        run(
          { ...f.options, mode: 'apply' },
          {
            async beforeMutation({ collection, index }) {
              if (index === stopAt)
                await f.db
                  .collection(collection)
                  .updateOne({}, { $set: { paidFees: new Double(99) } });
            },
          },
        ),
        /Intervening|CAS conflict/,
      );
      assert.equal(
        (
          await f.db.collection(conflictingCollection).findOne({})
        ).paidFees.valueOf(),
        99,
      );
      const after = await f.snapshot();
      delete before[conflictingCollection][0].paidFees;
      delete after[conflictingCollection][0].paidFees;
      assert.deepEqual(
        after,
        before,
        'Other committed rows are rolled back despite the conflict',
      );
      await assert.rejects(
        run({ ...f.options, mode: 'rollback' }),
        /Intervening/,
      );
    }
  },
);

test(
  'real CAS detects a type-only numeric metadata change',
  enabled,
  async (t) => {
    const f = await fixture(t);
    await f.prepare();
    await assert.rejects(
      run(
        { ...f.options, mode: 'apply' },
        {
          async beforeMutation({ collection, index }) {
            if (index === 0)
              await f.db.collection(collection).updateOne(
                {},
                {
                  $set: { 'metadata.safeLong': new Double(42) },
                },
              );
          },
        },
      ),
      /Intervening|CAS conflict/,
    );
    const row = await f.db.collection('transactionByAddress').findOne({});
    assert.equal(row.metadata.safeLong._bsontype, 'Double');
    assert.equal(row.metadata.safeLong.valueOf(), 42);
    assert.equal(Object.hasOwn(row, 'feeSource'), false);
  },
);

test(
  'SIGKILL at each standalone prefix resumes from durable BSON beforeimages',
  enabled,
  async (t) => {
    for (const stopAt of [0, 1, 2]) {
      const f = await fixture(t);
      const before = await f.snapshot();
      const beforeRaw = await f.rawSnapshot();
      await f.prepare();
      const source = `import {run} from ${JSON.stringify(new URL('./receipt-backfill.mjs', import.meta.url).href)};
      await run(${JSON.stringify({ ...f.options, mode: 'apply' })}, {
        beforeMutation({index}) { if(index === ${stopAt}) process.kill(process.pid, 'SIGKILL'); }
      });`;
      const error = await new Promise((resolve) => {
        execFile(
          process.execPath,
          ['--input-type=module', '-e', source],
          { timeout: 10000 },
          resolve,
        );
      });
      assert.equal(error?.signal, 'SIGKILL');
      assert.ok(
        !(await readdir(f.options.journalDir)).includes('progress.json'),
      );
      if (stopAt > 0) assert.notDeepEqual(await f.snapshot(), before);
      await run({ ...f.options, mode: 'apply' });
      await run({ ...f.options, mode: 'verify' });
      const progress = JSON.parse(
        await readFile(join(f.options.journalDir, 'progress.json'), 'utf8'),
      );
      assert.equal(progress.complete, true);
      await run({ ...f.options, mode: 'rollback' });
      assert.deepEqual(await f.snapshot(), before);
      assert.deepEqual(await f.rawSnapshot(), beforeRaw);
    }
  },
);

test(
  'durable transactional failure is atomic and successful apply retains completion markers',
  {
    skip: !replicaUri,
    concurrency: false,
    timeout: 30000,
  },
  async (t) => {
    const f = await fixture(t, { schema: 'durable' });
    const before = await f.snapshot();
    const beforeRaw = await f.rawSnapshot();
    await f.prepare();
    await assert.rejects(
      run(
        { ...f.options, mode: 'apply' },
        {
          async beforeMutation({ index }) {
            if (index === 1) {
              assert.deepEqual(
                await f.snapshot(),
                before,
                'Uncommitted writes stay invisible to a separate client',
              );
              throw new Error('Injected transaction interruption');
            }
          },
        },
      ),
      /Injected transaction interruption/,
    );
    assert.deepEqual(await f.snapshot(), before);
    assert.deepEqual(await f.rawSnapshot(), beforeRaw);
    await run(
      { ...f.options, mode: 'apply' },
      {
        async beforeMutation({ index }) {
          if (index === 1) assert.deepEqual(await f.snapshot(), before);
        },
      },
    );
    await run({ ...f.options, mode: 'verify' });
    const after = await f.snapshot();
    assert.deepEqual(
      withoutRepairedFields(after),
      withoutRepairedFields(before),
    );
    assert.equal(after.blocks[0].ingestionState, 'complete');
    await run({ ...f.options, mode: 'rollback' });
    assert.deepEqual(await f.snapshot(), before);
    assert.deepEqual(await f.rawSnapshot(), beforeRaw);
  },
);
