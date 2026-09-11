#!/usr/bin/env node
/**
 * check-api-docs-drift.mjs
 *
 * Compares the Gin route registrations in the Go backend against the paths
 * documented in app/lib/openapi.json, so CI fails when a route is added,
 * removed, or renamed without a matching spec update.
 *
 * Zero dependencies. All paths resolve relative to this file's location, so
 * the script runs correctly from any working directory.
 *
 * Rules:
 *   - Every .GET(/.POST( registration in the backend route files must
 *     appear in the spec as METHOD /api/<path>, with :param rewritten to
 *     {param}.
 *   - Every /api/* path in the spec must map back to a Go registration.
 *   - The frontend-owned routes (GET /search, GET+POST /faucet/claim) are
 *     exempt from the Go diff and are instead asserted to be present.
 */

import { readFileSync } from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const here = path.dirname(fileURLToPath(import.meta.url));
const frontendRoot = path.resolve(here, '..');
const repoRoot = path.resolve(frontendRoot, '..');

const routeFiles = [
  'backendAPI/routes/routes.go',
  'backendAPI/routes/verification_routes.go',
  'backendAPI/routes/contract_call_route.go',
  'backendAPI/routes/explain_routes.go',
  'backendAPI/routes/qns_route.go',
];

const specPath = path.join(frontendRoot, 'app', 'lib', 'openapi.json');

// Matches `router.GET("/path"` / `router.POST("/path", middleware, ...)`.
// The route files register everything directly on *gin.Engine with a
// literal path string as the first argument, so this pattern covers every
// registration shape they use today.
const registrationRe = /\.\s*(GET|POST|PUT|DELETE|PATCH|HEAD|OPTIONS)\(\s*"([^"]+)"/g;

let failed = false;
const fail = (msg) => {
  failed = true;
  console.error(msg);
};

// 1. Collect routes from the Go sources.
const goRoutes = new Set();
for (const rel of routeFiles) {
  const file = path.join(repoRoot, rel);
  let src;
  try {
    src = readFileSync(file, 'utf8');
  } catch (err) {
    fail(`ERROR: cannot read ${rel}: ${err.message}`);
    continue;
  }
  // Strip line comments so commented-out route registrations never count as
  // live routes.
  src = src.replace(/^[ \t]*\/\/.*$/gm, '');
  // The parser assumes no group prefixes. If someone introduces
  // router.Group(), this check must learn to resolve the prefix instead of
  // silently producing wrong paths.
  if (/\.Group\(/.test(src)) {
    fail(
      `ERROR: ${rel} uses router.Group(); update scripts/check-api-docs-drift.mjs to resolve group prefixes before relying on it.`,
    );
    continue;
  }
  for (const m of src.matchAll(registrationRe)) {
    const method = m[1].toUpperCase();
    const goPath = m[2].replace(/:([A-Za-z0-9_]+)/g, '{$1}');
    goRoutes.add(`${method} /api${goPath}`);
  }
}

// 2. Collect routes from the spec.
let spec;
try {
  spec = JSON.parse(readFileSync(specPath, 'utf8'));
} catch (err) {
  console.error(`ERROR: cannot parse ${specPath}: ${err.message}`);
  process.exit(1);
}

// Compiler provenance is part of both the discovery API and persisted
// verification records. Keep these exact references covered alongside route
// drift so a schema-only regression cannot silently erase artifact identity.
const provenanceRef = '#/components/schemas/CompilerProvenance';
const compilerProvenanceSchemaV2 = 'qrl.contract-compiler-provenance.v2';
const nativeCompilerComponentNames = ['hypc', 'nsjail', 'policy'];
const sourceBundleDigestPattern = '^qrl\\.verified-source-bundle\\.v1:sha256:[0-9a-f]{64}$';
const verificationArtifactDigestPattern =
  '^qrl\\.contract-verification-artifact\\.v2:sha256:[0-9a-f]{64}$';
const verificationRecordSchemaV1 = 'qrl.contract-verification-record.v1';
const verificationRecordSchemaV2 = 'qrl.contract-verification-record.v2';
const contractInfoSchema = spec.components?.schemas?.ContractInfo;
const contractProvenanceSchema = contractInfoSchema?.properties?.compilerProvenance;
const compilerProvenanceSchema = spec.components?.schemas?.CompilerProvenance;
const compilerComponentsSchema = compilerProvenanceSchema?.properties?.components;
const compilerComponentNames = compilerComponentsSchema?.prefixItems?.map(
  (item) => item?.allOf?.[1]?.properties?.name?.const,
);
const compilerInfo = spec.paths?.['/api/contract/compiler-info']?.get?.responses?.['200']
  ?.content?.['application/json']?.schema?.properties;
if (
  compilerInfo?.kind?.type !== 'string' ||
  compilerInfo?.sha256?.type !== 'string' ||
  compilerInfo?.provenance?.$ref !== provenanceRef ||
  compilerInfo?.compilers?.items?.properties?.provenance?.$ref !== provenanceRef
) {
  fail('Compiler info OpenAPI schema is missing exact compiler provenance fields.');
}
const jobProperties = spec.paths?.['/api/contract/verify/{jobId}']?.get?.responses?.['200']
  ?.content?.['application/json']?.schema?.properties;
const verifyRequestProperties = spec.paths?.['/api/contract/verify']?.post?.requestBody?.content?.[
  'application/json'
]?.schema?.properties;
const jobTarget = jobProperties?.target;
const jobResult = jobProperties?.result?.properties;
if (
  jobProperties?.payload?.properties?.compilerProvenance?.$ref !== provenanceRef ||
  jobProperties?.result?.properties?.compilerProvenance?.$ref !== provenanceRef ||
  jobResult?.artifactDigest?.pattern !== verificationArtifactDigestPattern ||
  jobTarget?.properties?.address?.pattern !== '^Q[0-9a-f]{128}$' ||
  jobTarget?.properties?.deployedCodeSha256?.pattern !== '^[0-9a-f]{64}$' ||
  !jobTarget?.required?.includes('creationBlockHash') ||
  !jobTarget?.required?.includes('chainId') ||
  !jobTarget?.required?.includes('deployedCodeSha256') ||
  jobProperties?.payload?.properties?.verificationMethod?.const !== 'full-source' ||
  jobProperties?.payload?.properties?.imports?.additionalProperties?.type !== 'string' ||
  verifyRequestProperties?.imports?.additionalProperties?.type !== 'string' ||
  verifyRequestProperties?.contractName?.pattern !== '^[A-Za-z_$][A-Za-z0-9_$]*$'
) {
  fail('Verification OpenAPI schema is missing compiler provenance or canonical imports.');
}
if (
  !contractProvenanceSchema?.anyOf?.some((entry) => entry?.$ref === provenanceRef) ||
  !contractInfoSchema?.properties?.verificationRecordSchema?.enum?.includes(
    verificationRecordSchemaV1,
  ) ||
  !contractInfoSchema?.properties?.verificationRecordSchema?.enum?.includes(
    verificationRecordSchemaV2,
  ) ||
  contractInfoSchema?.properties?.verificationArtifactDigest?.pattern !==
    verificationArtifactDigestPattern ||
  contractInfoSchema?.properties?.contractCodeSha256?.pattern !== '^[0-9a-f]{64}$' ||
  !contractInfoSchema?.allOf?.some(
    (entry) =>
      entry?.if?.properties?.verificationRecordSchema?.const === verificationRecordSchemaV2 &&
      entry?.then?.required?.includes('verificationArtifactDigest') &&
      entry?.then?.required?.includes('creationBlockHash') &&
      entry?.then?.required?.includes('chainId'),
  ) ||
  contractInfoSchema?.properties?.imports?.additionalProperties?.type !==
    'string' ||
  contractInfoSchema?.properties?.sourceBundleDigest?.pattern !==
    sourceBundleDigestPattern ||
  contractInfoSchema?.properties?.aiExplanationSourceDigest?.pattern !==
    sourceBundleDigestPattern ||
  compilerProvenanceSchema?.properties?.schema?.const !== compilerProvenanceSchemaV2 ||
  compilerProvenanceSchema?.properties?.kind?.const !== 'native' ||
  compilerProvenanceSchema?.properties?.executionDigest?.pattern !== '^[0-9a-f]{64}$' ||
  compilerComponentsSchema?.minItems !== nativeCompilerComponentNames.length ||
  compilerComponentsSchema?.maxItems !== nativeCompilerComponentNames.length ||
  compilerComponentsSchema?.items !== false ||
  JSON.stringify(compilerComponentNames) !== JSON.stringify(nativeCompilerComponentNames)
) {
  fail('ContractInfo OpenAPI schema is missing exact sandbox, source-bundle, or deployment-artifact identity.');
}
const explainOperation = spec.paths?.['/api/contract/explain/{address}']?.post;
const challengeOperation = spec.paths?.['/api/contract/explain/{address}/challenge']?.post;
const qip55AddressPattern = '^Q[0-9a-fA-F]{128}$';
const explainAddressParameter = explainOperation?.parameters?.find(
  (parameter) => parameter?.name === 'address',
);
const regenerateParameter = explainOperation?.parameters?.find(
  (parameter) => parameter?.name === 'regenerate',
);
if (
  explainOperation?.requestBody?.content?.['application/json']?.schema?.$ref !==
    '#/components/schemas/ContractExplainAuthorizationRequest' ||
  explainAddressParameter?.schema?.pattern !== qip55AddressPattern ||
  regenerateParameter?.schema?.const !== '1' ||
  !explainOperation?.responses?.['401'] ||
  !explainOperation?.responses?.['409'] ||
  !explainOperation?.responses?.['413']
) {
  fail('Contract explanation OpenAPI schema is missing authorization or provenance responses.');
}
if (
  challengeOperation?.responses?.['200']?.content?.['application/json']?.schema?.$ref !==
    '#/components/schemas/ContractExplainChallenge' ||
  challengeOperation?.parameters?.find((parameter) => parameter?.name === 'address')?.schema
    ?.pattern !== qip55AddressPattern ||
  spec.components?.schemas?.ContractExplainChallenge?.properties?.signer?.pattern !==
    qip55AddressPattern ||
  spec.components?.schemas?.ContractExplainChallenge?.properties?.contract?.pattern !==
    qip55AddressPattern ||
  spec.components?.schemas?.ContractExplainChallenge?.properties?.chainId?.pattern !==
    '^0x[1-9a-f][0-9a-f]*$' ||
  spec.components?.schemas?.ContractExplainSignedMessageProof?.properties?.descriptor?.const !==
    '0x010000' ||
  spec.components?.schemas?.ContractExplainSignedMessageProof?.properties?.schemeVersion?.const !==
    'QRL-SIGN-MSG-v1'
) {
  fail('Contract explanation challenge OpenAPI schema is incomplete.');
}

const httpMethods = new Set(['get', 'post', 'put', 'delete', 'patch', 'head', 'options', 'trace']);
const specApiRoutes = new Set();
const specFrontendRoutes = new Set();
for (const [specRoutePath, operations] of Object.entries(spec.paths ?? {})) {
  for (const method of Object.keys(operations)) {
    if (!httpMethods.has(method.toLowerCase())) continue;
    const key = `${method.toUpperCase()} ${specRoutePath}`;
    if (specRoutePath === '/api' || specRoutePath.startsWith('/api/')) {
      specApiRoutes.add(key);
    } else {
      specFrontendRoutes.add(key);
    }
  }
}

// 3. Diff backend routes against the spec.
const missingFromSpec = [...goRoutes].filter((r) => !specApiRoutes.has(r)).sort();
const staleInSpec = [...specApiRoutes].filter((r) => !goRoutes.has(r)).sort();

if (missingFromSpec.length > 0) {
  fail('Backend routes missing from app/lib/openapi.json:');
  for (const r of missingFromSpec) fail(`  + ${r}`);
}
if (staleInSpec.length > 0) {
  fail('Spec paths with no matching backend route (stale or misspelled):');
  for (const r of staleInSpec) fail(`  - ${r}`);
}

// 4. Assert the frontend-owned routes are documented.
const frontendOwned = ['GET /search', 'GET /faucet/claim', 'POST /faucet/claim'];
const missingFrontend = frontendOwned.filter((r) => !specFrontendRoutes.has(r)).sort();
const unknownFrontend = [...specFrontendRoutes].filter((r) => !frontendOwned.includes(r)).sort();

if (missingFrontend.length > 0) {
  fail('Frontend-owned routes missing from app/lib/openapi.json:');
  for (const r of missingFrontend) fail(`  + ${r}`);
}
if (unknownFrontend.length > 0) {
  fail('Spec documents non-/api paths this check does not know about (add them to frontendOwned if intentional):');
  for (const r of unknownFrontend) fail(`  ? ${r}`);
}

if (failed) {
  console.error(
    `\nDrift detected: ${goRoutes.size} backend routes vs ${specApiRoutes.size} documented /api paths.`,
  );
  process.exit(1);
}

console.log(
  `API docs in sync: ${goRoutes.size} backend routes + ${specFrontendRoutes.size} frontend routes documented.`,
);
