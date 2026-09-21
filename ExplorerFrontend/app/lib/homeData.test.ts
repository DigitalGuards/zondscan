import { describe, expect, it } from '@jest/globals';
import { initialHomeData, loadHomeData, type HomeData } from './homeData';

function deferred() {
  let resolve!: (body: Record<string, unknown>) => void;
  let reject!: (reason: Error) => void;
  const promise = new Promise<Record<string, unknown>>((yes, no) => {
    resolve = yes;
    reject = no;
  });
  return { promise, resolve, reject };
}

const fixtures: Record<string, Record<string, unknown>> = {
  '/overview': {
    validatorCount: 24,
    marketcap: 100,
    circulating: '65000000',
    status: { dataInitialized: true },
  },
  '/latestblock': { blockNumber: 1234 },
  '/txs?page=1': { total: 9876 },
  '/epoch': { headEpoch: '42' },
  '/blocks?page=1&limit=10': {
    blocks: [{ number: '0x4d2', timestamp: '0x1', hash: 'block', miner: '', transactions: [] }],
  },
  '/epochs?page=1&limit=1': { epochs: [{ totalStaked: '1000000000' }] },
  '/gas/summary': { avgGasPriceHex: '0x1' },
  '/transactions': { response: [{ TxHash: 'first', TimeStamp: 1, From: '', To: '', Amount: '1' }] },
};

function snapshot() {
  let data = initialHomeData();
  let updates = 0;
  return {
    read: () => data,
    updates: () => updates,
    publish: (update: (previous: HomeData) => HomeData) => {
      data = update(data);
      updates++;
    },
  };
}

describe('independent homepage snapshot readiness', () => {
  it('publishes stats and both tables before the exact count resolves', async () => {
    const total = deferred();
    const state = snapshot();
    const poll = loadHomeData(
      (path) => (path === '/txs?page=1' ? total.promise : Promise.resolve(fixtures[path])),
      new AbortController().signal,
      state.publish
    );
    await Promise.resolve();
    expect(state.read().status.totalTransactions).toBe('loading');
    expect(state.read().totalTransactions).toBeNull();
    expect(state.read().status.overview).toBe('ready');
    expect(state.read().validatorCount).toBe(24);
    expect(state.read().blockHeight).toBe(1234);
    expect(state.read().blocks).toHaveLength(1);
    expect(state.read().txs).toHaveLength(1);
    total.resolve({ total: 9876543 });
    await poll;
    expect(state.read().totalTransactions).toBe(9876543);
    expect(state.read().status.totalTransactions).toBe('ready');
  });

  it('does not infer initialization or zero stats before the overview responds', async () => {
    const overview = deferred();
    const state = snapshot();
    const poll = loadHomeData(
      (path) => (path === '/overview' ? overview.promise : Promise.resolve(fixtures[path])),
      new AbortController().signal,
      state.publish
    );
    await Promise.resolve();
    expect(state.read().status.overview).toBe('loading');
    expect(state.read().dataInitialized).toBeNull();
    expect(state.read().validatorCount).toBeNull();
    expect(state.read().status.txs).toBe('ready');
    overview.resolve({ status: { dataInitialized: false }, validatorCount: 0 });
    await poll;
    expect(state.read().dataInitialized).toBe(false);
    expect(state.read().validatorCount).toBe(0);
  });

  it('keeps failed first responses unknown and allows the next poll to recover', async () => {
    const state = snapshot();
    await loadHomeData(
      () => Promise.reject(new Error('offline')),
      new AbortController().signal,
      state.publish
    );
    expect(Object.values(state.read().status).every((status) => status === 'error')).toBe(true);
    expect(state.read().totalTransactions).toBeNull();
    expect(state.read().blocks).toBeNull();
    expect(state.read().dataInitialized).toBeNull();
    await loadHomeData(
      (path) => Promise.resolve(fixtures[path]),
      new AbortController().signal,
      state.publish
    );
    expect(Object.values(state.read().status).every((status) => status === 'ready')).toBe(true);
    expect(state.read().totalTransactions).toBe(9876);
  });

  it('preserves successful data during a pending or failed background refresh', async () => {
    const state = snapshot();
    await loadHomeData(
      (path) => Promise.resolve(fixtures[path]),
      new AbortController().signal,
      state.publish
    );
    const before = state.read();
    const pending = deferred();
    const poll = loadHomeData(() => pending.promise, new AbortController().signal, state.publish);
    expect(state.read()).toBe(before);
    pending.reject(new Error('offline'));
    await poll;
    expect(state.read().totalTransactions).toBe(before.totalTransactions);
    expect(state.read().blocks).toBe(before.blocks);
    expect(state.read().txs).toBe(before.txs);
    expect(state.read().status.blocks).toBe('error');
  });

  it('ignores late responses after cancellation, including superseded polls', async () => {
    const state = snapshot();
    const stale = deferred();
    const controller = new AbortController();
    const oldPoll = loadHomeData(() => stale.promise, controller.signal, state.publish);
    controller.abort();
    await loadHomeData(
      (path) => Promise.resolve(fixtures[path]),
      new AbortController().signal,
      state.publish
    );
    const current = state.read();
    const updates = state.updates();
    stale.resolve({ total: 1, blockNumber: 1, validatorCount: 1, blocks: [], response: [] });
    await oldPoll;
    expect(state.read()).toBe(current);
    expect(state.updates()).toBe(updates);
  });

  it('forwards the same cancellation signal to every request and ignores cancelled failures', async () => {
    const state = snapshot();
    const controller = new AbortController();
    const pending = deferred();
    const signals: AbortSignal[] = [];
    const poll = loadHomeData(
      (_path, signal) => {
        signals.push(signal);
        return pending.promise;
      },
      controller.signal,
      state.publish
    );
    expect(signals).toHaveLength(8);
    expect(signals.every((signal) => signal === controller.signal)).toBe(true);
    controller.abort();
    pending.reject(new Error('cancelled'));
    await poll;
    expect(state.updates()).toBe(0);
  });

  it('distinguishes valid zero counts and empty tables from missing response fields', async () => {
    const state = snapshot();
    await loadHomeData(() => Promise.resolve({}), new AbortController().signal, state.publish);
    expect(state.read().totalTransactions).toBeNull();
    expect(state.read().status.totalTransactions).toBe('error');
    expect(state.read().blocks).toBeNull();
    await loadHomeData(
      (path) =>
        Promise.resolve({ ...fixtures[path], total: 0, blockNumber: 0, blocks: [], response: [] }),
      new AbortController().signal,
      state.publish
    );
    expect(state.read().totalTransactions).toBe(0);
    expect(state.read().blockHeight).toBe(0);
    expect(state.read().blocks).toEqual([]);
    expect(state.read().txs).toEqual([]);
  });

  it('keeps recent transactions deduplicated and limited without using them as the total count', async () => {
    const state = snapshot();
    const rows = Array.from({ length: 12 }, (_, index) => ({ TxHash: `tx${index}` }));
    await loadHomeData(
      (path) =>
        Promise.resolve(
          path === '/transactions' ? { response: [null, ...rows, ...rows] } : fixtures[path]
        ),
      new AbortController().signal,
      state.publish
    );
    expect(state.read().txs).toHaveLength(8);
    expect(state.read().txs?.map((tx) => tx.TxHash)).toEqual(
      rows.slice(0, 8).map((tx) => tx.TxHash)
    );
    expect(state.read().totalTransactions).toBe(9876);
  });
});
