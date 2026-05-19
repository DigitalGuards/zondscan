'use client';

import { useMemo, useState } from 'react';
import axios, { AxiosError } from 'axios';
import * as zondAbi from '@theqrl/web3-zond-abi';
import type { ContractData } from '../types/address';
import config from '../../config';

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

interface ReadContractProps {
  contractData: ContractData;
}

/**
 * Renders one collapsible card per view/pure ABI function. Each card has
 * inputs for the function's parameters, a Call button, and a result area
 * that decodes the bytes returned by the backend's /contract/call proxy.
 *
 * ABI encode + decode runs entirely client-side via @theqrl/web3-zond-abi
 * — the backend never sees decoded values, it just forwards the hex
 * calldata to the node and returns hex.
 */
export default function ReadContract({ contractData }: ReadContractProps): JSX.Element {
  const readFns = useMemo(() => {
    if (!contractData.verified || !contractData.abi) return [] as AbiFunction[];
    try {
      const parsed = JSON.parse(contractData.abi) as AbiFunction[];
      return parsed.filter(
        f => f.type === 'function' && (f.stateMutability === 'view' || f.stateMutability === 'pure'),
      );
    } catch {
      return [] as AbiFunction[];
    }
  }, [contractData.abi, contractData.verified]);

  if (!contractData.verified) {
    return (
      <div className="rounded-lg border border-border bg-card-gradient p-4 text-sm text-gray-300">
        Verify this contract first to enable Read interactions.
      </div>
    );
  }
  if (readFns.length === 0) {
    return (
      <div className="rounded-lg border border-border bg-card-gradient p-4 text-sm text-gray-300">
        This contract has no view/pure functions to read.
      </div>
    );
  }
  return (
    <div className="space-y-2 md:space-y-3">
      {readFns.map((fn, idx) => (
        <ReadFunctionCard
          key={`${fn.name}-${idx}-${fn.inputs.map(i => i.type).join(',')}`}
          fn={fn}
          address={contractData.address}
        />
      ))}
    </div>
  );
}

function ReadFunctionCard({ fn, address }: { fn: AbiFunction; address: string }) {
  const [values, setValues] = useState<string[]>(() => fn.inputs.map(() => ''));
  const [result, setResult] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [reverted, setReverted] = useState(false);
  const [loading, setLoading] = useState(false);
  const [open, setOpen] = useState(false);

  const setVal = (i: number, v: string) =>
    setValues(cur => cur.map((x, j) => (j === i ? v : x)));

  const onCall = async () => {
    setLoading(true);
    setError(null);
    setResult(null);
    setReverted(false);
    try {
      const args = fn.inputs.map((input, i) => parseArg(values[i] ?? '', input.type));
      const data = zondAbi.encodeFunctionCall(fn as never, args as never[]);

      const resp = await axios.post<{ result?: string; error?: string; reverted?: boolean }>(
        `${config.handlerUrl}/contract/call`,
        { to: address, data },
      );

      if (resp.data.error) {
        setError(resp.data.error);
        setReverted(Boolean(resp.data.reverted));
        return;
      }
      if (!resp.data.result) {
        setError('No result returned by node');
        return;
      }
      const outTypes = (fn.outputs ?? []).map(o => o.type);
      if (outTypes.length === 0) {
        setResult('(void)');
        return;
      }
      const decoded = zondAbi.decodeParameters(outTypes as never[], resp.data.result);
      setResult(formatDecoded(decoded, fn.outputs ?? []));
    } catch (e) {
      const ae = e as AxiosError;
      const msg = (ae.response?.data as { error?: string } | undefined)?.error
        ?? (e instanceof Error ? e.message : String(e));
      setError(msg);
    } finally {
      setLoading(false);
    }
  };

  const sig = `${fn.name}(${fn.inputs.map(i => `${i.type}${i.name ? ' ' + i.name : ''}`).join(', ')})`;

  return (
    <div className="rounded-lg border border-border bg-card-gradient">
      <button
        type="button"
        onClick={() => setOpen(o => !o)}
        aria-expanded={open}
        className="w-full flex items-center justify-between gap-2 px-3 py-2 text-left"
      >
        <span className="font-mono text-xs md:text-sm text-gray-200 break-all">{sig}</span>
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
          <button
            type="button"
            onClick={onCall}
            disabled={loading}
            className="inline-flex items-center justify-center px-3 py-1.5 rounded-lg bg-accent text-black text-xs font-medium hover:bg-accent-hover transition-colors disabled:opacity-50"
          >
            {loading ? 'Calling…' : 'Call'}
          </button>

          {error && (
            <div className={`rounded-md border p-2 text-xs ${
              reverted
                ? 'border-orange-500/40 bg-orange-500/10 text-orange-300'
                : 'border-red-500/40 bg-red-500/10 text-red-300'
            }`}>
              {reverted ? 'execution reverted: ' : ''}{error}
            </div>
          )}

          {result !== null && !error && (
            <pre className="rounded-md bg-black/40 border border-border p-3 font-mono text-xs text-gray-300 whitespace-pre-wrap break-words">
              {result}
            </pre>
          )}
        </div>
      )}
    </div>
  );
}

// parseArg converts a user-typed string into the right JS shape for
// @theqrl/web3-zond-abi.encodeFunctionCall. Keep this minimal — anything
// fancy (tuples, fixed arrays) the user can express as JSON.
function parseArg(raw: string, type: string): unknown {
  const t = raw.trim();
  if (t === '') {
    if (type === 'bool') return false;
    if (type.startsWith('uint') || type.startsWith('int')) return '0';
    return '';
  }
  if (type === 'bool') return t === 'true' || t === '1';
  if (type.endsWith(']') || type === 'tuple') {
    try { return JSON.parse(t); } catch {
      throw new Error(`expected JSON for ${type}: e.g. [1,2,3]`);
    }
  }
  if (type.startsWith('uint') || type.startsWith('int')) {
    // Hand string through — web3-zond-abi accepts strings for big ints.
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

// formatDecoded renders @theqrl/web3-zond-abi.decodeParameters output for
// human consumption. The Result object carries both `0,1,…` index keys
// and any named keys, plus a `__length__`. We flatten to a stable shape,
// and Q-normalise any address-typed value because the 0.3.x ABI decoder
// emits the legacy Z prefix that nothing else in zondscan uses.
function formatDecoded(decoded: unknown, outputs: { name: string; type: string }[]): string {
  const out: Record<string, unknown> = {};
  outputs.forEach((o, i) => {
    const v = (decoded as Record<string, unknown>)[String(i)];
    out[o.name && o.name !== '' ? o.name : `_${i}`] = qNormalise(v, o.type);
  });
  return JSON.stringify(out, replacer, 2);
}

function qNormalise(v: unknown, type: string): unknown {
  if (type === 'address' && typeof v === 'string' && /^Z[0-9a-fA-F]{40}$/.test(v)) {
    return 'Q' + v.slice(1);
  }
  if (type.startsWith('address[') && Array.isArray(v)) {
    return v.map(x => qNormalise(x, 'address'));
  }
  return v;
}

function replacer(_k: string, v: unknown): unknown {
  return typeof v === 'bigint' ? v.toString() : v;
}
