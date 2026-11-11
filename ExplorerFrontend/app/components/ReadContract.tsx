'use client';

import { useMemo, useState } from 'react';
import axios from 'axios';
import * as zondAbi from '@theqrl/web3-qrl-abi';
import type { ContractData } from '../types/address';
import { assertVm64AbiSupport } from '../lib/vm64Abi';
import { decodeContractRead, parseContractArg } from '../lib/contractCall';
import { parseAbiFunctions, type AbiFunction, isContractCallResponse } from '../lib/contractAbi';
import { isRecord, errorMessage, InvalidInputError } from '../lib/guards';
import config from '../../config';
import { classifyStoredVerification } from '../lib/storedVerification';
import ContractInteractionProvenanceNotice from './ContractInteractionProvenanceNotice';

interface ReadContractProps {
  contractData: ContractData;
}

/**
 * Renders one collapsible card per view/pure ABI function. Each card has
 * inputs for the function's parameters, a Call button, and a result area
 * that decodes the bytes returned by the backend's /contract/call proxy.
 *
 * ABI encode + decode runs entirely client-side through the installed codec.
 * The runtime capability gate requires the QIP-55 64-byte word layout. The
 * backend never sees decoded values, it just forwards the hex
 * calldata to the node and returns hex.
 */
export default function ReadContract({ contractData }: ReadContractProps): JSX.Element {
  const verificationStatus = useMemo(
    () => classifyStoredVerification(contractData),
    [contractData]
  );
  const readFns = useMemo<AbiFunction[]>(() => {
    if (!contractData.verified || verificationStatus === 'invalid-recorded' || !contractData.abi) {
      return [];
    }
    try {
      const parsed = parseAbiFunctions(contractData.abi);
      return parsed.filter((f) => f.stateMutability === 'view' || f.stateMutability === 'pure');
    } catch {
      return [];
    }
  }, [contractData.abi, contractData.verified, verificationStatus]);

  if (!contractData.verified) {
    return (
      <div className="rounded-lg border border-border bg-card-gradient p-4 text-sm text-text-secondary">
        Verify this contract first to enable Read interactions.
      </div>
    );
  }
  if (verificationStatus === 'invalid-recorded') {
    return <ContractInteractionProvenanceNotice status={verificationStatus} interaction="Read" />;
  }
  if (readFns.length === 0) {
    return (
      <div className="space-y-3">
        <ContractInteractionProvenanceNotice status={verificationStatus} interaction="Read" />
        <div className="rounded-lg border border-border bg-card-gradient p-4 text-sm text-text-secondary">
          This contract has no view/pure functions to read.
        </div>
      </div>
    );
  }
  return (
    <div className="space-y-3">
      <ContractInteractionProvenanceNotice status={verificationStatus} interaction="Read" />
      <div className="space-y-2 md:space-y-3">
        {readFns.map((fn, idx) => (
          <ReadFunctionCard
            key={`${fn.name}-${idx}-${fn.inputs.map((i) => i.type).join(',')}`}
            fn={fn}
            address={contractData.address}
          />
        ))}
      </div>
    </div>
  );
}

function ReadFunctionCard({ fn, address }: { fn: AbiFunction; address: string }) {
  const [values, setValues] = useState<string[]>(() => fn.inputs.map(() => ''));
  const [result, setResult] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [reverted, setReverted] = useState(false);
  const [rawReply, setRawReply] = useState<string | null>(null);
  const [loading, setLoading] = useState(false);
  const [open, setOpen] = useState(false);

  const setVal = (i: number, v: string) => {
    setValues((cur) => cur.map((x, j) => (j === i ? v : x)));
  };

  const onCall = async () => {
    setLoading(true);
    setError(null);
    setResult(null);
    setReverted(false);
    setRawReply(null);
    try {
      assertVm64AbiSupport();
      const args = fn.inputs.map((input, i) => parseContractArg(values[i] ?? '', input.type));
      const data = zondAbi.encodeFunctionCall(fn, args);

      const resp = await axios.post<unknown>(`${config.handlerUrl}/contract/call`, {
        to: address,
        data,
      });

      if (!isContractCallResponse(resp.data))
        throw new InvalidInputError('Invalid contract call response');
      if (resp.data.error) {
        setError(resp.data.error);
        setReverted(Boolean(resp.data.reverted));
        return;
      }
      if (!resp.data.result) {
        setError('No result returned by node');
        return;
      }
      const outputs = fn.outputs ?? [];
      if (outputs.length === 0) {
        setResult('(void)');
        return;
      }
      const read = decodeContractRead(outputs, resp.data.result);
      if (read.decoded) {
        setResult(read.text);
      } else {
        setError(read.error);
        setRawReply(read.raw);
      }
    } catch (e) {
      const payload: unknown = axios.isAxiosError(e) ? e.response?.data : undefined;
      setError(
        isRecord(payload) && typeof payload.error === 'string' ? payload.error : errorMessage(e)
      );
    } finally {
      setLoading(false);
    }
  };

  const sig = `${fn.name}(${fn.inputs.map((i) => `${i.type}${i.name ? ' ' + i.name : ''}`).join(', ')})`;

  return (
    <div className="rounded-lg border border-border bg-card-gradient">
      <button
        type="button"
        onClick={() => {
          setOpen((o) => !o);
        }}
        aria-expanded={open}
        className="w-full flex items-center justify-between gap-2 px-3 py-2 text-left"
      >
        <span className="font-mono text-xs md:text-sm text-text-primary break-all">{sig}</span>
        <span className="text-xs text-text-muted">{open ? '–' : '+'}</span>
      </button>
      {open && (
        <div className="border-t border-border p-3 space-y-3">
          {fn.inputs.map((input, i) => (
            <div key={i}>
              <div className="text-xs text-text-secondary mb-1">
                {input.name ? `${input.name} ` : ''}
                <span className="font-mono text-text-muted">({input.type})</span>
              </div>
              <input
                value={values[i] ?? ''}
                onChange={(e) => {
                  setVal(i, e.target.value);
                }}
                placeholder={placeholderFor(input.type)}
                className="form-input font-mono text-xs"
              />
            </div>
          ))}
          <button
            type="button"
            onClick={() => {
              void onCall();
            }}
            disabled={loading}
            className="inline-flex items-center justify-center px-3 py-1.5 rounded-lg bg-accent text-background text-xs font-medium hover:bg-accent-hover transition-colors disabled:opacity-50"
          >
            {loading ? 'Calling…' : 'Call'}
          </button>

          {error && (
            <div
              className={`rounded-md border p-2 text-xs ${
                reverted
                  ? 'border-orange-500/40 bg-orange-500/10 text-orange-300'
                  : 'border-red-500/40 bg-red-500/10 text-red-300'
              }`}
            >
              {reverted ? 'execution reverted: ' : ''}
              {error}
              {rawReply !== null && (
                <div className="mt-2 font-mono break-all text-text-secondary">
                  Raw reply: {rawReply}
                </div>
              )}
            </div>
          )}

          {result !== null && !error && (
            <pre className="rounded-md bg-background-tertiary border border-border p-3 font-mono text-xs text-text-secondary whitespace-pre-wrap break-words">
              {result}
            </pre>
          )}
        </div>
      )}
    </div>
  );
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
