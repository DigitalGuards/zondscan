"use client";

import {
  classifyCompilerProvenance,
  nativeSandboxCompilerExecutionDigestV2,
} from "../lib/compilerProvenance";
import type { CompilerProvenanceState } from "../lib/compilerProvenance";
import {
  VERIFICATION_RECORD_SCHEMA_V1,
  VERIFICATION_RECORD_SCHEMA_V2,
} from "../lib/storedVerification";
import CopyButton from "./CopyButton";

export {
  classifyCompilerProvenance,
  nativeSandboxCompilerExecutionDigestV2,
};
export type { CompilerProvenanceState };

interface CompilerProvenanceDetailsProps {
  verificationRecordSchema?: unknown;
  provenance: unknown;
  compilerVersion?: string;
}

export default function CompilerProvenanceDetails({
  verificationRecordSchema,
  provenance,
  compilerVersion,
}: CompilerProvenanceDetailsProps): JSX.Element {
  const compilerState = classifyCompilerProvenance(provenance, compilerVersion);
  const state: CompilerProvenanceState =
    verificationRecordSchema === undefined && provenance === undefined
      ? compilerState
      : verificationRecordSchema === VERIFICATION_RECORD_SCHEMA_V1 ||
          verificationRecordSchema === VERIFICATION_RECORD_SCHEMA_V2
        ? compilerState.status === "legacy-unrecorded"
          ? { status: "invalid-recorded" }
          : compilerState
        : { status: "invalid-recorded" };

  if (state.status === "legacy-unrecorded") {
    return (
      <section
        aria-label="Compiler provenance"
        data-provenance-status="legacy-unrecorded"
        className="sm:col-span-2 rounded-lg border border-warning/30 bg-warning/10 p-3"
      >
        <div className="text-text-secondary">Compiler provenance</div>
        <div className="font-mono font-medium text-warning">
          legacy-unrecorded
        </div>
        <p className="mt-1 text-text-secondary">
          This historical verification has no recorded compiler artifact digest.
        </p>
      </section>
    );
  }

  if (state.status === "invalid-recorded") {
    return (
      <section
        aria-label="Compiler provenance"
        data-provenance-status="invalid-recorded"
        className="sm:col-span-2 rounded-lg border border-error/30 bg-error/10 p-3"
      >
        <div className="text-text-secondary">Compiler provenance</div>
        <div className="font-mono font-medium text-error">invalid-recorded</div>
        <p className="mt-1 text-text-secondary">
          The stored provenance is incomplete or conflicts with the recorded
          compiler version.
        </p>
      </section>
    );
  }

  const { provenance: recorded } = state;
  return (
    <section
      aria-label="Compiler provenance"
      data-provenance-status="digest-backed"
      className="sm:col-span-2 rounded-lg border border-success/30 bg-success/10 p-3"
    >
      <div className="mb-2 flex flex-wrap items-center justify-between gap-2">
        <div className="text-text-secondary">Compiler provenance</div>
        <div className="font-mono font-medium text-success">digest-backed</div>
      </div>
      <dl className="grid grid-cols-1 gap-2 sm:grid-cols-2">
        <ProvenanceRow label="Schema" value={recorded.schema} />
        <ProvenanceRow label="Kind" value={recorded.kind} />
        <ProvenanceRow label="Build ID" value={recorded.buildId} />
        <ProvenanceRow
          label="Execution digest"
          value={recorded.executionDigest}
          copyLabel="Copy execution digest"
        />
        {recorded.components.map((component) => (
          <ProvenanceRow
            key={component.name}
            label={`${component.name} SHA-256`}
            value={component.sha256}
            copyLabel={`Copy ${component.name} SHA-256`}
          />
        ))}
      </dl>
    </section>
  );
}

function ProvenanceRow({
  label,
  value,
  copyLabel,
}: {
  label: string;
  value: string;
  copyLabel?: string;
}): JSX.Element {
  return (
    <div>
      <dt className="text-text-secondary">{label}</dt>
      <dd className="flex min-w-0 items-start gap-1.5">
        <span className="min-w-0 break-all font-mono text-text-primary">
          {value}
        </span>
        {copyLabel && (
          <span className="shrink-0">
            <CopyButton value={value} label={copyLabel} size="sm" />
          </span>
        )}
      </dd>
    </div>
  );
}
