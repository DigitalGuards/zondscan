import {
  classifyStoredVerification,
  classifyVerifiedImports,
  sourceBundleDigestV1,
  VERIFICATION_RECORD_SCHEMA_V2,
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

  const imports = Object.fromEntries(importsState.files);
  return classifyStoredVerification({
    verificationRecordSchema: VERIFICATION_RECORD_SCHEMA_V2,
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
