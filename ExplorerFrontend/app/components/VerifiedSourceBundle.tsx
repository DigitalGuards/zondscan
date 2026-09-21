"use client";

import { useMemo, useState } from "react";

import type { ContractData } from "../types/address";
import {
  classifyStoredVerification,
  classifyVerifiedImports,
  sourceBundleDigestV1,
} from "../lib/storedVerification";
import type {
  StoredVerificationStatus,
  VerifiedImport,
} from "../lib/storedVerification";
import CopyButton from "./CopyButton";

interface VerifiedSourceBundleProps {
  contractData: ContractData;
}

export { classifyVerifiedImports, sourceBundleDigestV1 };

export function orderedVerifiedImports(value: unknown): VerifiedImport[] {
  return classifyVerifiedImports(value).files;
}

export default function VerifiedSourceBundle({
  contractData,
}: VerifiedSourceBundleProps): JSX.Element {
  const importsState = useMemo(
    () => classifyVerifiedImports(contractData.imports),
    [contractData.imports],
  );
  const verifiedImports = importsState.files;
  const verificationStatus = useMemo(
    () => classifyStoredVerification(contractData),
    [contractData],
  );

  return (
    <div className="space-y-3">
      {contractData.sourceCode ? (
        <SourceFilePanel
          title="Contract source"
          source={contractData.sourceCode}
          copyLabel="Copy source"
          defaultExpanded
        />
      ) : null}

      <SourceBundleStatus
        verificationStatus={verificationStatus}
        importCount={verifiedImports.length}
        hasPrimarySource={Boolean(contractData.sourceCode)}
        importsValid={importsState.valid}
      />

      {verifiedImports.length > 0 ? (
        <section
          aria-label={
            verificationStatus === "digest-backed"
              ? "Verified imported sources"
              : "Recorded imported sources"
          }
          className="space-y-2"
        >
          <div className="flex items-center justify-between gap-2">
            <div className="text-xs md:text-sm text-text-secondary">
              {verificationStatus === "digest-backed"
                ? "Verified imports"
                : "Recorded imports"}
            </div>
            <div className="font-mono text-xs text-text-secondary">
              {verifiedImports.length}{" "}
              {verifiedImports.length === 1 ? "file" : "files"}
            </div>
          </div>
          {verifiedImports.map(([filename, source]) => (
            <SourceFilePanel
              key={filename}
              title={filename}
              source={source}
              copyLabel={`Copy import ${filename}`}
              canonicalImport
            />
          ))}
        </section>
      ) : null}
    </div>
  );
}

function SourceBundleStatus({
  verificationStatus,
  importCount,
  hasPrimarySource,
  importsValid,
}: {
  verificationStatus: StoredVerificationStatus;
  importCount: number;
  hasPrimarySource: boolean;
  importsValid: boolean;
}): JSX.Element {
  if (verificationStatus === "legacy-unrecorded") {
    return (
      <div
        role="status"
        data-source-bundle-status="legacy-unrecorded"
        className="rounded-lg border border-warning/30 bg-warning/10 p-3 text-xs md:text-sm text-text-secondary"
      >
        This legacy verification lacks the current deployment-bound artifact
        digest. Its recorded source bundle remains available for review.
      </div>
    );
  }

  if (
    verificationStatus === "invalid-recorded" ||
    !hasPrimarySource ||
    !importsValid
  ) {
    return (
      <div
        role="status"
        data-source-bundle-status="invalid-recorded"
        className="rounded-lg border border-error/30 bg-error/10 p-3 text-xs md:text-sm text-text-secondary"
      >
        Source-bundle and deployment identity cannot be established from this
        verification record.
      </div>
    );
  }

  if (importCount === 0) {
    return (
      <div
        role="status"
        data-source-bundle-status="digest-backed-single-file"
        className="rounded-lg border border-success/30 bg-success/10 p-3 text-xs md:text-sm text-text-secondary"
      >
        Single-file verification: no imports recorded.
      </div>
    );
  }

  return (
    <div
      role="status"
      data-source-bundle-status="digest-backed-multi-file"
      className="rounded-lg border border-success/30 bg-success/10 p-3 text-xs md:text-sm text-text-secondary"
    >
      Verified source bundle includes {importCount} recorded import{" "}
      {importCount === 1 ? "file" : "files"}.
    </div>
  );
}

function SourceFilePanel({
  title,
  source,
  copyLabel,
  defaultExpanded = false,
  canonicalImport = false,
}: {
  title: string;
  source: string;
  copyLabel: string;
  defaultExpanded?: boolean;
  canonicalImport?: boolean;
}): JSX.Element {
  const [expanded, setExpanded] = useState(defaultExpanded);

  return (
    <article
      className={
        canonicalImport
          ? "rounded-lg border border-border bg-card-gradient p-3"
          : ""
      }
    >
      <div className="flex items-center justify-between gap-2 mb-2">
        <div className="min-w-0 text-xs md:text-sm text-text-secondary">
          {canonicalImport ? (
            <code
              data-testid="verified-import-filename"
              className="block break-all text-text-primary"
            >
              {title}
            </code>
          ) : (
            title
          )}
        </div>
        <div className="flex shrink-0 gap-2">
          <button
            type="button"
            onClick={() => setExpanded((current) => !current)}
            aria-expanded={expanded}
            className="inline-flex items-center px-3 py-1.5 rounded-lg bg-card-gradient border border-border hover:border-accent text-sm text-text-secondary hover:text-accent transition-colors"
          >
            {expanded ? "Collapse" : "Expand"}
          </button>
          <CopyButton value={source} label={copyLabel} />
        </div>
      </div>
      <pre
        data-testid={canonicalImport ? "verified-import-source" : undefined}
        className={`rounded-lg bg-background-tertiary border border-border p-3 font-mono text-xs text-text-secondary overflow-x-auto whitespace-pre transition-[max-height] duration-200 ${
          expanded
            ? "max-h-[36rem] overflow-y-auto"
            : "max-h-24 overflow-hidden"
        }`}
      >
        {source}
      </pre>
    </article>
  );
}
