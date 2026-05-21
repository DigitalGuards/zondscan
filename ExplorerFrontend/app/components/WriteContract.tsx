'use client';

import { useMemo, useState } from 'react';
import Link from 'next/link';
import * as zondAbi from '@theqrl/web3-zond-abi';
import ConnectButton from './ConnectButton';
import type { QRLConnectProvider } from '../lib/qrlConnect';
import type { ContractData } from '../types/address';

interface AbiInput {
  name: string;
  type: string;
  components?: AbiInput[];
}

interface AbiFunction {
  type: 'function';
  name: string;
  stateMutability: 'view' | 'pure' | 'nonpayable' | 'payable';
  inputs: AbiInput[];
  outputs?: { name: string; type: string }[];
}

interface WriteContractProps {
  contractData: ContractData;
}

/**
 * One collapsible card per nonpayable/payable ABI function. Each card encodes
 * its calldata client-side via @theqrl/web3-zond-abi and asks the user's
 * paired wallet to broadcast the transaction over the QRL Connect relay. The
 * wallet returns the broadcast tx hash, which we link to the local explorer.
 *
 * Wallet submits, the explorer never sees the signed payload and never
 * touches a node directly for write paths. Matches the protocol used by the
 * dApp example and keeps the explorer trust boundary narrow.
 */
export default function WriteContract({ contractData }: WriteContractProps): JSX.Element {
  const [account, setAccount] = useState<string | null>(null);
  const [provider, setProvider] = useState<QRLConnectProvider | null>(null);

  const writeFns = useMemo(() => {
    if (!contractData.verified || !contractData.abi) return [] as AbiFunction[];
    try {
      const parsed = JSON.parse(contractData.abi) as AbiFunction[];
      return parsed.filter(
        f =>
          f.type === 'function' &&
          (f.stateMutability === 'nonpayable' || f.stateMutability === 'payable'),
      );
    } catch {
      return [] as AbiFunction[];
    }
  }, [contractData.abi, contractData.verified]);

  if (!contractData.verified) {
    return (
      <div className="rounded-lg border border-border bg-card-gradient p-4 text-sm text-gray-300">
        Verify this contract first to enable Write interactions.
      </div>
    );
  }
  if (writeFns.length === 0) {
    return (
      <div className="rounded-lg border border-border bg-card-gradient p-4 text-sm text-gray-300">
        This contract has no state-changing functions to call.
      </div>
    );
  }

  return (
    <div className="space-y-3 md:space-y-4">
      <div className="rounded-lg border border-border bg-card-gradient p-3 md:p-4 flex flex-col sm:flex-row gap-3 sm:items-center sm:justify-between">
        <div>
          <div className="text-xs md:text-sm font-medium text-gray-200">Wallet pairing</div>
          <div className="text-xs text-gray-400 mt-0.5">
            Connect MyQRLWallet to sign and broadcast state-changing calls.
          </div>
        </div>
        <ConnectButton onAccount={setAccount} onProvider={setProvider} />
      </div>

      <div className="space-y-2 md:space-y-3">
        {writeFns.map((fn, idx) => (
          <WriteFunctionCard
            key={`${fn.name}-${idx}-${fn.inputs.map(i => i.type).join(',')}`}
            fn={fn}
            address={contractData.address}
            account={account}
            provider={provider}
          />
        ))}
      </div>
    </div>
  );
}

function WriteFunctionCard({
  fn,
  address,
  account,
  provider,
}: {
  fn: AbiFunction;
  address: string;
  account: string | null;
  provider: QRLConnectProvider | null;
}) {
  const [values, setValues] = useState<string[]>(() => fn.inputs.map(() => ''));
  const [value, setValue] = useState<string>('');
  const [txHash, setTxHash] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [rejected, setRejected] = useState(false);
  const [loading, setLoading] = useState(false);
  const [open, setOpen] = useState(false);

  const setVal = (i: number, v: string) =>
    setValues(cur => cur.map((x, j) => (j === i ? v : x)));

  const onWrite = async () => {
    if (!provider || !account) {
      setError('Connect a wallet first');
      return;
    }
    setLoading(true);
    setError(null);
    setTxHash(null);
    setRejected(false);
    try {
      const args = fn.inputs.map((input, i) => parseArg(values[i] ?? '', input.type));
      const data = zondAbi.encodeFunctionCall(fn as never, args as never[]);

      const tx: Record<string, string> = {
        from: account,
        to: address,
        data,
      };
      if (fn.stateMutability === 'payable' && value.trim() !== '') {
        tx.value = toHexWei(value.trim());
      }

      const result = (await provider.request({
        method: 'qrl_sendTransaction',
        params: [tx],
      })) as string;
      setTxHash(result);
    } catch (e) {
      // EIP-1193 user-rejection: code 4001.
      const code = (e as { code?: number } | undefined)?.code;
      if (code === 4001) {
        setRejected(true);
        setError('Request rejected in wallet');
      } else {
        setError(e instanceof Error ? e.message : String(e));
      }
    } finally {
      setLoading(false);
    }
  };

  const sig = `${fn.name}(${fn.inputs.map(i => `${i.type}${i.name ? ' ' + i.name : ''}`).join(', ')})`;
  const disabled = loading || !account || !provider;

  return (
    <div className="rounded-lg border border-border bg-card-gradient">
      <button
        type="button"
        onClick={() => setOpen(o => !o)}
        aria-expanded={open}
        className="w-full flex items-center justify-between gap-2 px-3 py-2 text-left"
      >
        <span className="font-mono text-xs md:text-sm text-gray-200 break-all">
          {sig}
          {fn.stateMutability === 'payable' && (
            <span className="ml-2 text-[10px] text-amber-300 align-middle">payable</span>
          )}
        </span>
        <span className="text-xs text-gray-500">{open ? '–' : '+'}</span>
      </button>
      {open && (
        <div className="border-t border-border p-3 space-y-3">
          {fn.inputs.map((input, i) => (
            <div key={i}>
              <div className="text-xs text-gray-400 mb-1">
                {input.name ? `${input.name} ` : ''}
                <span className="font-mono text-gray-500">({input.type})</span>
              </div>
              <input
                value={values[i] ?? ''}
                onChange={e => setVal(i, e.target.value)}
                placeholder={placeholderFor(input.type)}
                className="form-input font-mono text-xs"
              />
            </div>
          ))}

          {fn.stateMutability === 'payable' && (
            <div>
              <div className="text-xs text-gray-400 mb-1">
                value <span className="font-mono text-gray-500">(QRL)</span>
              </div>
              <input
                value={value}
                onChange={e => setValue(e.target.value)}
                placeholder="0.0"
                className="form-input font-mono text-xs"
              />
            </div>
          )}

          <button
            type="button"
            onClick={onWrite}
            disabled={disabled}
            className="inline-flex items-center justify-center px-3 py-1.5 rounded-lg bg-accent text-black text-xs font-medium hover:bg-accent-hover transition-colors disabled:opacity-50"
          >
            {loading ? 'Awaiting wallet…' : account ? 'Write' : 'Connect wallet to write'}
          </button>

          {error && (
            <div
              className={`rounded-md border p-2 text-xs ${
                rejected
                  ? 'border-orange-500/40 bg-orange-500/10 text-orange-300'
                  : 'border-red-500/40 bg-red-500/10 text-red-300'
              }`}
            >
              {error}
            </div>
          )}

          {txHash && !error && (
            <div className="rounded-md border border-green-500/40 bg-green-500/10 p-2 text-xs text-green-300">
              <div className="mb-1">Broadcast:</div>
              <Link href={`/tx/${txHash}`} className="font-mono text-accent hover:text-accent-hover break-all">
                {txHash}
              </Link>
            </div>
          )}
        </div>
      )}
    </div>
  );
}

// parseArg converts a user-typed string into the right JS shape for
// @theqrl/web3-zond-abi.encodeFunctionCall. Mirrors ReadContract's parser
// so the Read/Write tabs accept identical input formats.
function parseArg(raw: string, type: string): unknown {
  const t = raw.trim();
  if (t === '') {
    if (type === 'bool') return false;
    if (type.startsWith('uint') || type.startsWith('int')) return '0';
    return '';
  }
  if (type === 'bool') return t.toLowerCase() === 'true' || t === '1';
  if (type.endsWith(']') || type === 'tuple') {
    try {
      return JSON.parse(t);
    } catch {
      throw new Error(`expected JSON for ${type}: e.g. [1,2,3]`);
    }
  }
  if (type.startsWith('uint') || type.startsWith('int')) {
    return t;
  }
  return t;
}

function placeholderFor(type: string): string {
  if (type === 'address') return 'Q…';
  if (type === 'bool') return 'true / false';
  if (type.startsWith('uint') || type.startsWith('int')) return '0';
  if (type === 'string') return 'text';
  if (type.startsWith('bytes')) return '0x…';
  if (type.endsWith(']') || type === 'tuple') return 'JSON: [1,2,3]';
  return '';
}

// toHexWei converts a decimal QRL string ("1.25") into 0x-prefixed wei.
// Avoids float drift by splitting on the decimal point and padding the
// fractional side to 18 digits before BigInt arithmetic.
function toHexWei(qrl: string): string {
  const [whole, frac = ''] = qrl.split('.');
  const fracPadded = (frac + '0'.repeat(18)).slice(0, 18);
  const combined = `${whole}${fracPadded}`.replace(/^0+/, '') || '0';
  return '0x' + BigInt(combined).toString(16);
}
