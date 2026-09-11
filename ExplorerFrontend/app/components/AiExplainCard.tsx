"use client";

import { useEffect, useMemo, useState } from "react";
import axios, { AxiosError } from "axios";
import type { ContractData } from "../types/address";
import {
  hasAuthoritativeCreatorProvenance,
  parseContractExplainChallenge,
  signContractExplainChallenge,
} from "../lib/contractExplainAuth";
import { canonicalizeQrlAddress } from "../lib/qrlAddress";
import { classifyStoredVerification } from "../lib/storedVerification";
import { getQrlConnect } from "../lib/qrlConnect";
import config from "../../config";

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
 * verified, matching the backend's 403, so the button never appears on
 * unverified contracts and there's no path to spend tokens on bytecode-
 * only addresses.
 */
export default function AiExplainCard(props: AiExplainCardProps): JSX.Element {
  const { contractData } = props;
  return (
    <AiExplainCardRecord
      key={aiExplainRecordIdentity(contractData)}
      {...props}
    />
  );
}

export function aiExplainRecordIdentity(contractData: ContractData): string {
  return [
    contractData.address,
    contractData.creatorAddressProvenance ?? "",
    contractData.verificationRecordSchema ?? "",
    contractData.compilerVersion ?? "",
    contractData.compilerProvenance?.executionDigest ?? "",
    contractData.sourceBundleDigest ?? "",
    contractData.verificationArtifactDigest ?? "",
    contractData.aiExplanationSourceDigest ?? "",
  ].join("\u0000");
}

function AiExplainCardRecord({
  contractData,
}: AiExplainCardProps): JSX.Element | null {
  const verificationStatus = useMemo(
    () => classifyStoredVerification(contractData),
    [contractData],
  );
  const creatorAuthorizationAvailable = hasAuthoritativeCreatorProvenance(
    contractData.creatorAddressProvenance,
  );
  const cachedExplanationIsBound =
    verificationStatus === "digest-backed" &&
    typeof contractData.sourceBundleDigest === "string" &&
    contractData.aiExplanationSourceDigest === contractData.sourceBundleDigest;
  // Pre-load any cached explanation that arrived with the address payload
  // so the user sees the body immediately on page load (no extra round-trip).
  const [explanation, setExplanation] = useState<string | null>(
    cachedExplanationIsBound ? (contractData.aiExplanation ?? null) : null,
  );
  const [generatedAt, setGeneratedAt] = useState<string | null>(
    cachedExplanationIsBound ? (contractData.aiExplanationAt ?? null) : null,
  );
  const [model, setModel] = useState<string | null>(
    cachedExplanationIsBound ? (contractData.aiExplanationModel ?? null) : null,
  );
  const [cached, setCached] = useState<boolean>(
    cachedExplanationIsBound && Boolean(contractData.aiExplanation),
  );
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  // Owner-only regen gate. Lazily snapshots any stored wallet session at
  // mount and updates when the wallet's accountsChanged event fires.
  const [isOwner, setIsOwner] = useState<boolean>(false);

  useEffect(() => {
    if (verificationStatus !== "digest-backed" || !creatorAuthorizationAvailable) return;
    if (typeof window === "undefined") return;
    const creator = canonicalizeQrlAddress(contractData.creatorAddress);
    if (!creator) return;
    let qrl;
    try {
      qrl = getQrlConnect();
    } catch {
      // SSR path or environment without window.crypto.
      return;
    }
    const check = (): void => {
      const accounts = qrl.getAccounts();
      const account =
        accounts.length === 1 ? canonicalizeQrlAddress(accounts[0]) : null;
      setIsOwner(account === creator);
    };
    check();
    const onChange = (): void => check();
    qrl.on("accountsChanged", onChange);
    qrl.on("connect", onChange);
    qrl.on("disconnect", onChange);
    return () => {
      qrl.off("accountsChanged", onChange);
      qrl.off("connect", onChange);
      qrl.off("disconnect", onChange);
    };
  }, [contractData.creatorAddress, creatorAuthorizationAvailable, verificationStatus]);

  if (!contractData.verified) return null;

  if (verificationStatus !== "digest-backed") {
    const legacy = verificationStatus === "legacy-unrecorded";
    return (
      <div
        role={legacy ? "status" : "alert"}
        data-ai-explain-status={verificationStatus}
        className={`rounded-lg border p-3 text-xs md:text-sm text-text-secondary ${
          legacy
            ? "border-warning/30 bg-warning/10"
            : "border-error/30 bg-error/10"
        }`}
      >
        {legacy
          ? "AI explanation unavailable: this legacy verification lacks the current deployment-bound artifact digest."
          : "AI explanation unavailable: compiler, source-bundle, or deployment provenance is invalid."}
      </div>
    );
  }

  const call = async (regenerate: boolean): Promise<void> => {
    setLoading(true);
    setError(null);
    try {
      const endpoint = `${config.handlerUrl}/contract/explain/${contractData.address}`;
      const url = `${endpoint}${
        regenerate ? "?regenerate=1" : ""
      }`;
      let body: unknown;
      if (regenerate) {
        if (typeof window === "undefined") {
          throw new Error("Contract creator authorization requires a browser wallet");
        }
        const creator = canonicalizeQrlAddress(contractData.creatorAddress);
        const contract = canonicalizeQrlAddress(contractData.address);
        if (!creatorAuthorizationAvailable || !creator || !contract) {
          throw new Error("Contract creator authorization is unavailable");
        }
        const challengeResponse = await axios.post<unknown>(
          `${endpoint}/challenge`,
        );
        const challenge = parseContractExplainChallenge(
          challengeResponse.data,
          {
            contract,
            signer: creator,
            origin: window.location.origin,
          },
        );
        body = await signContractExplainChallenge(getQrlConnect(), challenge);
      }
      const r = await axios.post<ExplainResponse>(url, body);
      setExplanation(r.data.explanation);
      setGeneratedAt(r.data.generatedAt);
      setModel(r.data.model);
      setCached(r.data.cached);
    } catch (e) {
      const ae = e as AxiosError;
      const msg =
        (ae.response?.data as { error?: string } | undefined)?.error ??
        (e instanceof Error ? e.message : String(e));
      setError(msg);
    } finally {
      setLoading(false);
    }
  };

  return (
    <div className="rounded-lg border border-border bg-card-gradient p-3 md:p-4 space-y-3">
      <div className="flex items-start justify-between gap-3 flex-wrap">
        <div>
          <div className="text-sm md:text-base font-medium text-text-primary flex items-center gap-2">
            <span aria-hidden="true">✨</span> AI explanation
          </div>
          <div className="text-xs text-text-secondary mt-0.5">
            {explanation
              ? `Generated ${formatStamp(generatedAt)}${cached ? " (cached)" : ""}${model ? ` · ${model}` : ""}`
              : "Have Claude summarise what this verified contract does. Costs a one-time generate per contract."}
          </div>
        </div>
        <div className="flex gap-2">
          {!explanation && (
            <button
              type="button"
              onClick={() => call(false)}
              disabled={loading}
              className="inline-flex items-center justify-center px-3 py-1.5 rounded-lg bg-accent text-background text-xs font-medium hover:bg-accent-hover transition-colors disabled:opacity-50"
            >
              {loading ? "Analysing…" : "Explain with AI"}
            </button>
          )}
          {explanation && creatorAuthorizationAvailable && isOwner && (
            <button
              type="button"
              onClick={() => call(true)}
              disabled={loading}
              className="inline-flex items-center justify-center px-3 py-1.5 rounded-lg bg-card-gradient border border-border hover:border-accent text-xs text-text-secondary hover:text-accent transition-colors disabled:opacity-50"
              title="Regenerate (limited to 5 per 7-day window)"
            >
              {loading ? "Analysing…" : "Regenerate"}
            </button>
          )}
          {explanation && (!creatorAuthorizationAvailable || !isOwner) && (
            <span className="text-[10px] text-text-muted self-center">
              {creatorAuthorizationAvailable
                ? "Only the contract creator can regenerate."
                : "Creator authorization requires deployment evidence."}
            </span>
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
          <div className="rounded-md bg-black/40 border border-border p-3 text-xs md:text-sm text-text-primary font-sans whitespace-pre-wrap break-words leading-relaxed">
            {explanation}
          </div>
          <div className="text-[10px] text-text-muted">
            AI-generated summary. May contain inaccuracies. Not financial
            advice, verify the source code yourself before interacting with this
            contract.
          </div>
        </>
      )}
    </div>
  );
}

function formatStamp(iso: string | null): string {
  if (!iso) return "just now";
  try {
    const d = new Date(iso);
    if (Number.isNaN(d.getTime())) return iso;
    return d.toLocaleString();
  } catch {
    return iso;
  }
}
