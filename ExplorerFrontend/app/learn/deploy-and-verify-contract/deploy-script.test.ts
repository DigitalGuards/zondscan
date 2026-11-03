/**
 * Runs the tutorial's deploy script with the installed @theqrl/web3 against a
 * fake node, so a web3 upgrade that changes how the fee pair is filled, or
 * that starts refusing the transaction the page tells readers to send, fails
 * here.
 *
 * The values pinned below are the ones @theqrl/web3 1.0.3 signed for the same
 * node answers when the sample still passed a legacy gasPrice, which 1.0.3
 * dropped (measured side by side): tip = web3's default 2.5 Shor,
 * cap = 2 * baseFee + tip.
 */
import { createServer } from 'node:http';
import type { AddressInfo } from 'node:net';
import { createContext, runInContext } from 'node:vm';
import * as web3Package from '@theqrl/web3';
import { MLDSA87 } from '@theqrl/wallet.js';
import { TransactionFactory } from '@theqrl/web3-qrl-accounts';
import { CURRENT_EXPLORER_CHAIN_ID_HEX } from '../../lib/navigation';
import { deployScript } from './deploy-script';

const PUBLIC_RPC = 'https://qrlwallet.com/api/qrl-rpc/testnet';
const BASE_FEE = BigInt('100000000000');
const WEB3_DEFAULT_TIP = BigInt('2500000000');
const ESTIMATED_GAS = BigInt(200000);
const BYTECODE = '0x6080604052';
const CREATED = `Q${'c0'.repeat(64)}`;

interface FakeNode {
  url: string;
  methods: string[];
  broadcasts: string[];
  close: () => Promise<void>;
}

function rpcRequests(body: string): { id: unknown; method: string; params: unknown[] }[] {
  const parsed: unknown = JSON.parse(body);
  const list: unknown[] = Array.isArray(parsed) ? parsed : [parsed];
  return list.map((entry) => {
    if (typeof entry !== 'object' || entry === null) throw new Error('fixture: bad request');
    const { id, method, params } = entry as { id: unknown; method: unknown; params: unknown };
    if (typeof method !== 'string' || !Array.isArray(params)) {
      throw new Error('fixture: bad request');
    }
    return { id, method, params };
  });
}

async function startFakeNode(chainId: bigint): Promise<FakeNode> {
  const methods: string[] = [];
  const broadcasts: string[] = [];
  const receipts = new Map<string, unknown>();
  const answer = (method: string, params: unknown[]): unknown => {
    switch (method) {
      case 'qrl_chainId':
        return `0x${chainId.toString(16)}`;
      case 'net_version':
        return chainId.toString();
      case 'qrl_estimateGas':
        return `0x${ESTIMATED_GAS.toString(16)}`;
      case 'qrl_getTransactionCount':
        return '0x3';
      case 'qrl_blockNumber':
        return '0x11';
      case 'qrl_call':
        return '0x';
      case 'qrl_getBlockByNumber':
        return {
          number: '0x10',
          hash: `0x${'11'.repeat(32)}`,
          parentHash: `0x${'00'.repeat(32)}`,
          baseFeePerGas: `0x${BASE_FEE.toString(16)}`,
          gasLimit: '0x1c9c380',
          gasUsed: '0x0',
          timestamp: '0x1',
          transactions: [],
        };
      case 'qrl_sendRawTransaction': {
        const raw = params[0];
        if (typeof raw !== 'string') throw new Error('fixture: bad broadcast');
        broadcasts.push(raw);
        const hash = web3Package.utils.keccak256(raw);
        receipts.set(hash, {
          transactionHash: hash,
          transactionIndex: '0x0',
          blockHash: `0x${'22'.repeat(32)}`,
          blockNumber: '0x11',
          from: `Q${'00'.repeat(64)}`,
          to: null,
          cumulativeGasUsed: '0x5208',
          gasUsed: '0x5208',
          effectiveGasPrice: `0x${BASE_FEE.toString(16)}`,
          contractAddress: CREATED,
          logs: [],
          logsBloom: `0x${'00'.repeat(256)}`,
          status: '0x1',
          type: '0x2',
        });
        return hash;
      }
      case 'qrl_getTransactionReceipt':
        return receipts.get(String(params[0])) ?? null;
      case 'qrl_gasPrice':
        return `0x${BASE_FEE.toString(16)}`;
      default:
        throw new Error(`fixture: unexpected ${method}`);
    }
  };
  const server = createServer((request, response) => {
    let body = '';
    request.on('data', (chunk: Buffer) => {
      body += chunk.toString('utf8');
    });
    request.on('end', () => {
      const replies = rpcRequests(body).map(({ id, method, params }) => {
        methods.push(method);
        try {
          return { jsonrpc: '2.0', id, result: answer(method, params) };
        } catch (error) {
          return { jsonrpc: '2.0', id, error: { code: -32601, message: String(error) } };
        }
      });
      response.writeHead(200, { 'content-type': 'application/json' });
      response.end(JSON.stringify(replies.length === 1 ? replies[0] : replies));
    });
  });
  await new Promise<void>((resolve) => {
    server.listen(0, '127.0.0.1', resolve);
  });
  const { port } = server.address() as AddressInfo;
  return {
    url: `http://127.0.0.1:${String(port)}`,
    methods,
    broadcasts,
    close: () =>
      new Promise<void>((resolve) => {
        server.close(() => {
          resolve();
        });
      }),
  };
}

async function runSample(script: string, url: string) {
  expect(script).toContain(PUBLIC_RPC);
  const logs: string[] = [];
  const errors: unknown[] = [];
  const sandboxProcess: { env: { MNEMONIC: string }; exitCode?: number } = {
    env: { MNEMONIC: MLDSA87.newWallet().getMnemonic() },
  };
  const modules: Record<string, unknown> = {
    '@theqrl/web3': web3Package,
    '@theqrl/wallet.js': { MLDSA87 },
    './Counter.json': { abi: [], bytecode: BYTECODE },
  };
  const context = createContext({
    require: (name: string) => modules[name],
    process: sandboxProcess,
    console: {
      log: (...parts: unknown[]) => logs.push(parts.map(String).join(' ')),
      error: (error: unknown) => errors.push(error),
    },
  });
  await runInContext(script.replace(PUBLIC_RPC, url), context);
  return { logs, errors, exitCode: sandboxProcess.exitCode };
}

function explorerChainId(): bigint {
  if (CURRENT_EXPLORER_CHAIN_ID_HEX === null) throw new Error('explorer network has no chain id');
  return BigInt(CURRENT_EXPLORER_CHAIN_ID_HEX);
}

describe('deploy-and-verify tutorial script', () => {
  it('checks the explorer network chain id and sends a type 2 transaction', () => {
    expect(deployScript).toContain(`if (chainId !== ${explorerChainId().toString()}n)`);
    expect(deployScript).toContain("type: '0x2',");
    expect(deployScript).not.toMatch(/gasPrice/);
  });

  it('deploys with the fee pair web3 fills from the latest block', async () => {
    const node = await startFakeNode(explorerChainId());
    try {
      const run = await runSample(deployScript, node.url);
      expect(run.errors).toEqual([]);
      expect(run.exitCode).toBeUndefined();
      expect(run.logs).toEqual([`Contract address: ${CREATED}`]);
      expect(node.broadcasts).toHaveLength(1);
      const signed = TransactionFactory.fromSerializedData(
        web3Package.utils.hexToBytes(node.broadcasts[0] ?? '0x')
      );
      expect(signed.type).toBe(2);
      expect(signed.toJSON()).toMatchObject({
        chainId: `0x${explorerChainId().toString(16)}`,
        nonce: '0x3',
        gasLimit: `0x${((ESTIMATED_GAS * BigInt(12)) / BigInt(10)).toString(16)}`,
        maxPriorityFeePerGas: `0x${WEB3_DEFAULT_TIP.toString(16)}`,
        maxFeePerGas: `0x${(BigInt(2) * BASE_FEE + WEB3_DEFAULT_TIP).toString(16)}`,
        data: BYTECODE,
      });
      expect(signed.toJSON().to).toBeUndefined();
      // The fee pair comes from the block alone: no gas price is asked for.
      expect(node.methods).not.toContain('qrl_gasPrice');
    } finally {
      await node.close();
    }
  }, 30000);

  it('stops before signing on another chain', async () => {
    const node = await startFakeNode(BigInt(1337));
    try {
      const run = await runSample(deployScript, node.url);
      expect(run.exitCode).toBe(1);
      expect(run.errors).toHaveLength(1);
      expect(String(run.errors[0])).toContain('unexpected chain: 1337');
      expect(node.broadcasts).toEqual([]);
    } finally {
      await node.close();
    }
  }, 30000);

  it('is refused by the installed web3 with the legacy gasPrice the page used to show', async () => {
    const legacy = deployScript
      .replace("    type: '0x2',\n", '')
      .replace(
        '    gas: (estimated * 12n) / 10n, // 20% headroom\n',
        '    gas: (estimated * 12n) / 10n, // 20% headroom\n    gasPrice: await web3.qrl.getGasPrice(),\n'
      );
    expect(legacy).toContain('gasPrice: await web3.qrl.getGasPrice(),');
    expect(legacy).not.toContain("type: '0x2'");
    const node = await startFakeNode(explorerChainId());
    try {
      const run = await runSample(legacy, node.url);
      expect(run.exitCode).toBe(1);
      expect(run.errors).toHaveLength(1);
      expect(run.errors[0]).toBeInstanceOf(web3Package.InvalidPropertiesForTransactionTypeError);
      expect(node.broadcasts).toEqual([]);
    } finally {
      await node.close();
    }
  }, 30000);
});
