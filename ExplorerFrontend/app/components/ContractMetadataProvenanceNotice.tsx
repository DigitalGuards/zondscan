import type {
  ContractMeta,
  ContractMetadataProvenanceStatus,
} from "../types/transaction";

export function classifyContractMetadata(
  contract?: ContractMeta,
): ContractMetadataProvenanceStatus | null {
  if (!contract?.verified) return null;
  if (contract.provenanceStatus === "digest-backed") return "digest-backed";
  if (
    contract.provenanceStatus === undefined ||
    contract.provenanceStatus === "legacy-unrecorded"
  ) {
    return "legacy-unrecorded";
  }
  return "invalid-recorded";
}

export function trustedContractMetadataABI(
  contract?: ContractMeta,
): string | undefined {
  const status = classifyContractMetadata(contract);
  if (status === null || status === "invalid-recorded") return undefined;
  return typeof contract?.abi === "string" && contract.abi.length > 0
    ? contract.abi
    : undefined;
}

export default function ContractMetadataProvenanceNotice({
  contract,
}: {
  contract?: ContractMeta;
}): JSX.Element | null {
  const status = classifyContractMetadata(contract);
  if (status === null || status === "digest-backed") return null;

  const legacy = status === "legacy-unrecorded";
  return (
    <div
      role={legacy ? "status" : "alert"}
      data-contract-metadata-provenance-status={status}
      className={`rounded-md border p-2 text-xs text-text-secondary ${
        legacy
          ? "border-warning/30 bg-warning/10"
          : "border-error/30 bg-error/10"
      }`}
    >
      {legacy
        ? "Legacy verified ABI: compiler and source-bundle provenance were not recorded. Decoding is informational."
        : "Stored ABI blocked: verification provenance is invalid. Raw transaction data remains available."}
    </div>
  );
}
