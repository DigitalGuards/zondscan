"use client";

import type { StoredVerificationStatus } from "../lib/storedVerification";

interface ContractInteractionProvenanceNoticeProps {
  status: StoredVerificationStatus;
  interaction: "Read" | "Write";
}

export default function ContractInteractionProvenanceNotice({
  status,
  interaction,
}: ContractInteractionProvenanceNoticeProps): JSX.Element | null {
  if (status === "digest-backed") return null;

  if (status === "invalid-recorded") {
    return (
      <div
        role="alert"
        data-interaction-provenance-status="invalid-recorded"
        className="rounded-lg border border-error/40 bg-error/10 p-4 text-sm text-text-secondary"
      >
        <div className="font-medium text-error">
          {interaction} interactions blocked
        </div>
        <p className="mt-1">
          This verification record has invalid or inconsistent compiler or
          source-bundle provenance. Its recorded byte-match remains visible as
          historical data, and no contract interaction is exposed.
        </p>
      </div>
    );
  }

  return (
    <div
      role={interaction === "Write" ? "alert" : "status"}
      data-interaction-provenance-status="legacy-unrecorded"
      className="rounded-lg border border-warning/40 bg-warning/10 p-4 text-sm text-text-secondary"
    >
      <div className="font-medium text-warning">
        {interaction === "Write"
          ? "Write interactions blocked"
          : "Legacy verification"}
      </div>
      {interaction === "Write" ? (
        <p className="mt-1">
          This legacy verification lacks the current deployment-bound artifact
          digest. Source and ABI remain visible for review, while wallet pairing
          and transaction signing stay disabled.
        </p>
      ) : (
        <p className="mt-1">
          This legacy verification lacks the current deployment-bound artifact
          digest. Read calls use its recorded ABI as informational data.
        </p>
      )}
    </div>
  );
}
