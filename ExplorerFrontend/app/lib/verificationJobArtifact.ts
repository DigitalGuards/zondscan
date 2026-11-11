import {
  classifyStoredVerification,
  classifyVerifiedImports,
  requiresVerificationRecordSchemaV3,
  sourceBundleDigestV1,
  VERIFICATION_RECORD_SCHEMA_V2,
  VERIFICATION_RECORD_SCHEMA_V3,
} from "./storedVerification";
import type { StoredVerificationStatus } from "./storedVerification";

export interface VerificationJobTarget {
  address?: unknown;
  creationTransaction?: unknown;
  creationBlockNumber?: unknown;
  creationBlockHash?: unknown;
  chainId?: unknown;
  deployedCodeSha256?: unknown;
  genesisContract?: unknown;
}

export interface VerificationJobPayload {
  sourceCode?: unknown;
  contractName?: unknown;
  compilerVersion?: unknown;
  compilerProvenance?: unknown;
  optimizationEnabled?: unknown;
  optimizationRuns?: unknown;
  evmVersion?: unknown;
  constructorArguments?: unknown;
  libraries?: unknown;
  imports?: unknown;
  primarySourcePath?: unknown;
  viaIR?: unknown;
  license?: unknown;
  verificationMethod?: unknown;
}

export interface VerificationJobResult {
  abi?: unknown;
  bytecodeHash?: unknown;
  artifactDigest?: unknown;
  compilerProvenance?: unknown;
  verifiedAt?: unknown;
}

export interface VerificationJobArtifactInput {
  jobId?: unknown;
  status?: unknown;
  address?: unknown;
  target?: VerificationJobTarget | null;
  payload?: VerificationJobPayload | null;
  result?: VerificationJobResult | null;
}

export function classifyVerificationJobArtifact(
  job: VerificationJobArtifactInput,
): StoredVerificationStatus | null {
  if (job.status !== "success") return null;
  if (!job.target || !job.payload || !job.result) {
    return "invalid-recorded";
  }

  const importsState = classifyVerifiedImports(job.payload.imports);
  if (
    typeof job.payload.contractName !== "string" ||
    typeof job.payload.sourceCode !== "string" ||
    !importsState.valid ||
    job.address !== job.target.address ||
    job.result.bytecodeHash !== job.target.deployedCodeSha256 ||
    !sameCompilerProvenance(
      job.payload.compilerProvenance,
      job.result.compilerProvenance,
    )
  ) {
    return "invalid-recorded";
  }

  // The backend persists the v2 record shape unless the job compiled a
  // nested primary path or via-IR, so the job selects its schema by the same
  // rule. Malformed field types fall through to classification, which
  // rejects them.
  const primarySourcePath =
    typeof job.payload.primarySourcePath === "string" ? job.payload.primarySourcePath : "";
  const viaIR = job.payload.viaIR === true;
  const verificationRecordSchema = requiresVerificationRecordSchemaV3(
    job.payload.contractName,
    primarySourcePath,
    viaIR,
  )
    ? VERIFICATION_RECORD_SCHEMA_V3
    : VERIFICATION_RECORD_SCHEMA_V2;

  const imports = Object.fromEntries(importsState.files);
  return classifyStoredVerification({
    verificationRecordSchema,
    address: job.target.address,
    creationTransaction: job.target.creationTransaction,
    creationBlockNumber: job.target.creationBlockNumber,
    creationBlockHash: job.target.creationBlockHash,
    chainId: job.target.chainId,
    contractCodeSha256: job.target.deployedCodeSha256,
    genesisContract: job.target.genesisContract,
    sourceCode: job.payload.sourceCode,
    contractName: job.payload.contractName,
    compilerVersion: job.payload.compilerVersion,
    compilerProvenance: job.result.compilerProvenance,
    imports,
    sourceBundleDigest: sourceBundleDigestV1(
      job.payload.contractName,
      job.payload.sourceCode,
      imports,
    ),
    abi: job.result.abi,
    optimizationEnabled: job.payload.optimizationEnabled,
    optimizationRuns: job.payload.optimizationRuns,
    evmVersion: job.payload.evmVersion,
    constructorArguments: job.payload.constructorArguments,
    libraries: job.payload.libraries,
    primarySourcePath: job.payload.primarySourcePath,
    viaIR: job.payload.viaIR,
    license: job.payload.license,
    verificationMethod: job.payload.verificationMethod,
    verificationArtifactDigest: job.result.artifactDigest,
  });
}

function sameCompilerProvenance(left: unknown, right: unknown): boolean {
  if (left === undefined || right === undefined) return false;
  try {
    return JSON.stringify(left) === JSON.stringify(right);
  } catch {
    return false;
  }
}
