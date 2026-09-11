"use client";

import { classifyStoredVerification } from "../lib/storedVerification";
import type { StoredVerificationInput } from "../lib/storedVerification";

interface VerifiedBadgeProps {
  /** Compact variant fits inside table rows / headers; default sits inline next to titles. */
  size?: "sm" | "md";
  record: StoredVerificationInput;
}

/**
 * Pill rendered next to verified-contract titles. The parent has already
 * established `contractData.verified === true`; this boundary distinguishes
 * exact digest-backed records from legacy and internally inconsistent ones.
 */
export default function VerifiedBadge({
  size = "md",
  record,
}: VerifiedBadgeProps): JSX.Element {
  const state = verifiedBadgeState(record);
  const sizing =
    size === "sm" ? "px-1.5 py-0.5 text-[10px]" : "px-2 py-0.5 text-xs";
  const tone =
    state.status === "digest-backed"
      ? "border-success/40 bg-success/10 text-success"
      : state.status === "legacy-unrecorded"
        ? "border-warning/40 bg-warning/10 text-warning"
        : "border-error/40 bg-error/10 text-error";

  return (
    <span
      title={state.description}
      aria-label={`${state.label}: ${state.description}`}
      data-verification-status={state.status}
      className={`inline-flex items-center gap-1 rounded-full border font-medium ${tone} ${sizing}`}
    >
      <svg
        className={size === "sm" ? "h-2.5 w-2.5" : "h-3 w-3"}
        viewBox="0 0 20 20"
        fill="currentColor"
        aria-hidden="true"
      >
        {state.status === "digest-backed" ? (
          <path
            fillRule="evenodd"
            d="M10 18a8 8 0 1 0 0-16 8 8 0 0 0 0 16Zm3.707-9.293a1 1 0 1 0-1.414-1.414L9 10.586 7.707 9.293a1 1 0 0 0-1.414 1.414l2 2a1 1 0 0 0 1.414 0l4-4Z"
            clipRule="evenodd"
          />
        ) : (
          <path
            fillRule="evenodd"
            d="M8.257 3.099c.765-1.36 2.722-1.36 3.486 0l6.518 11.59C19.011 16.022 18.047 17.667 16.518 17.667H3.482c-1.529 0-2.493-1.645-1.743-2.978l6.518-11.59ZM10 7a1 1 0 0 1 1 1v3a1 1 0 1 1-2 0V8a1 1 0 0 1 1-1Zm0 7.25a1.25 1.25 0 1 0 0-2.5 1.25 1.25 0 0 0 0 2.5Z"
            clipRule="evenodd"
          />
        )}
      </svg>
      {state.label}
    </span>
  );
}

export interface VerifiedBadgeState {
  status: "digest-backed" | "legacy-unrecorded" | "invalid-recorded";
  label: string;
  description: string;
}

export function verifiedBadgeState(
  record: StoredVerificationInput,
): VerifiedBadgeState {
  const status = classifyStoredVerification(record);
  if (status === "digest-backed") {
    return {
      status,
      label: "Verified",
      description:
        "Source, ABI, compiler settings, and canonical deployment identity match the recorded artifact digest.",
    };
  }
  if (status === "legacy-unrecorded") {
    return {
      status,
      label: "Legacy verified",
      description:
        "Source and deployed bytecode matched under a legacy record that lacks the current deployment-bound artifact digest.",
    };
  }
  return {
    status,
    label: "Invalid verification record",
    description:
      "The stored record says source matched deployed bytecode, but its compiler, source bundle, or deployment artifact binding is invalid.",
  };
}
