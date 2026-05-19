'use client';

import { useState } from 'react';
import axios, { AxiosError } from 'axios';
import type { ContractData } from '../types/address';
import config from '../../config';

interface AiExplainCardProps {
  contractData: ContractData;
}

interface ExplainResponse {
  address: string;
  explanation: string;
  generatedAt: string;
  model: string;
  cached: boolean;
}

/**
 * On-demand AI explainer for verified contracts. Posts to
 * /contract/explain/:address and renders the returned Markdown. The first
 * click on a fresh contract spends real Anthropic credit; subsequent reads
 * hit the MongoDB cache for free until someone clicks "Regenerate".
 *
 * Hard render gate: this component bails to a stub if the contract isn't
 * verified — matching the backend's 403 — so the button never appears on
 * unverified contracts and there's no path to spend tokens on bytecode-
 * only addresses.
 */
export default function AiExplainCard({ contractData }: AiExplainCardProps): JSX.Element | null {
  // Pre-load any cached explanation that arrived with the address payload
  // so the user sees the body immediately on page load (no extra round-trip).
  const [explanation, setExplanation] = useState<string | null>(contractData.aiExplanation ?? null);
  const [generatedAt, setGeneratedAt] = useState<string | null>(contractData.aiExplanationAt ?? null);
  const [model, setModel] = useState<string | null>(contractData.aiExplanationModel ?? null);
  const [cached, setCached] = useState<boolean>(Boolean(contractData.aiExplanation));
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  if (!contractData.verified) return null;

  const call = async (regenerate: boolean): Promise<void> => {
    setLoading(true);
    setError(null);
    try {
      const url = `${config.handlerUrl}/contract/explain/${contractData.address}${
        regenerate ? '?regenerate=1' : ''
      }`;
      const r = await axios.post<ExplainResponse>(url);
      setExplanation(r.data.explanation);
      setGeneratedAt(r.data.generatedAt);
      setModel(r.data.model);
      setCached(r.data.cached);
    } catch (e) {
      const ae = e as AxiosError;
      const msg = (ae.response?.data as { error?: string } | undefined)?.error
        ?? (e instanceof Error ? e.message : String(e));
      setError(msg);
    } finally {
      setLoading(false);
    }
  };

  return (
    <div className="rounded-lg border border-border bg-card-gradient p-3 md:p-4 space-y-3">
      <div className="flex items-start justify-between gap-3 flex-wrap">
        <div>
          <div className="text-sm md:text-base font-medium text-gray-200 flex items-center gap-2">
            <span aria-hidden="true">✨</span> AI explanation
          </div>
          <div className="text-xs text-gray-400 mt-0.5">
            {explanation
              ? `Generated ${formatStamp(generatedAt)}${cached ? ' (cached)' : ''}${model ? ` · ${model}` : ''}`
              : 'Have Claude summarise what this verified contract does. Costs a one-time generate per contract.'}
          </div>
        </div>
        <div className="flex gap-2">
          {!explanation && (
            <button
              type="button"
              onClick={() => call(false)}
              disabled={loading}
              className="inline-flex items-center justify-center px-3 py-1.5 rounded-lg bg-accent text-black text-xs font-medium hover:bg-accent-hover transition-colors disabled:opacity-50"
            >
              {loading ? 'Analysing…' : 'Explain with AI'}
            </button>
          )}
          {explanation && (
            <button
              type="button"
              onClick={() => call(true)}
              disabled={loading}
              className="inline-flex items-center justify-center px-3 py-1.5 rounded-lg bg-card-gradient border border-border hover:border-accent text-xs text-gray-300 hover:text-accent transition-colors disabled:opacity-50"
            >
              {loading ? 'Analysing…' : 'Regenerate'}
            </button>
          )}
        </div>
      </div>

      {error && (
        <div className="rounded-md border border-red-500/40 bg-red-500/10 p-2 text-xs text-red-300">
          {error}
        </div>
      )}

      {explanation && (
        <>
          <div className="rounded-md bg-black/40 border border-border p-3 text-xs md:text-sm text-gray-200 font-sans whitespace-pre-wrap break-words leading-relaxed">
            {explanation}
          </div>
          <div className="text-[10px] text-gray-500">
            AI-generated summary. May contain inaccuracies. Not financial advice — verify the
            source code yourself before interacting with this contract.
          </div>
        </>
      )}
    </div>
  );
}

function formatStamp(iso: string | null): string {
  if (!iso) return 'just now';
  try {
    const d = new Date(iso);
    if (Number.isNaN(d.getTime())) return iso;
    return d.toLocaleString();
  } catch {
    return iso;
  }
}
