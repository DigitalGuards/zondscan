import { describe, expect, it } from "@jest/globals";

import {
  classifyCompilerProvenance,
  nativeSandboxCompilerExecutionDigestV2,
} from "./compilerProvenance";
import {
  COMPILER_PROVENANCE_V2_BUILD_ID,
  COMPILER_PROVENANCE_V2_FIXTURE,
} from "./compilerProvenanceV2.fixture";

describe("nativeSandboxCompilerExecutionDigestV2", () => {
  it("matches the backend-owned golden fixture", () => {
    const [hypc, nsjail, policy] = COMPILER_PROVENANCE_V2_FIXTURE.components;

    expect(
      nativeSandboxCompilerExecutionDigestV2(
        COMPILER_PROVENANCE_V2_BUILD_ID,
        hypc.sha256,
        nsjail.sha256,
        policy.sha256,
      ),
    ).toBe(COMPILER_PROVENANCE_V2_FIXTURE.executionDigest);
  });
});

describe("classifyCompilerProvenance", () => {
  it("accepts the exact backend schema-v2 component identity", () => {
    expect(
      classifyCompilerProvenance(
        COMPILER_PROVENANCE_V2_FIXTURE,
        COMPILER_PROVENANCE_V2_BUILD_ID,
      ),
    ).toEqual({
      status: "digest-backed",
      provenance: COMPILER_PROVENANCE_V2_FIXTURE,
    });
  });

  it.each([
    ["schema v1", { schema: "qrl.contract-compiler-provenance.v1" }],
    ["unknown schema", { schema: "qrl.contract-compiler-provenance.v3" }],
    ["wrong kind", { kind: "npm" }],
    ["wrong build", { buildId: "different" }],
    ["unbound digest", { executionDigest: "c".repeat(64) }],
  ])("rejects %s", (_label, mutation) => {
    expect(
      classifyCompilerProvenance(
        { ...COMPILER_PROVENANCE_V2_FIXTURE, ...mutation },
        COMPILER_PROVENANCE_V2_BUILD_ID,
      ),
    ).toEqual({ status: "invalid-recorded" });
  });

  it.each([
    ["missing component", COMPILER_PROVENANCE_V2_FIXTURE.components.slice(0, 2)],
    [
      "extra component",
      [
        ...COMPILER_PROVENANCE_V2_FIXTURE.components,
        { name: "runtime", sha256: "d".repeat(64) },
      ],
    ],
    [
      "wrong order",
      [
        COMPILER_PROVENANCE_V2_FIXTURE.components[1],
        COMPILER_PROVENANCE_V2_FIXTURE.components[0],
        COMPILER_PROVENANCE_V2_FIXTURE.components[2],
      ],
    ],
    [
      "uppercase digest",
      [
        {
          ...COMPILER_PROVENANCE_V2_FIXTURE.components[0],
          sha256: COMPILER_PROVENANCE_V2_FIXTURE.components[0].sha256.toUpperCase(),
        },
        ...COMPILER_PROVENANCE_V2_FIXTURE.components.slice(1),
      ],
    ],
  ])("rejects %s", (_label, components) => {
    expect(
      classifyCompilerProvenance(
        { ...COMPILER_PROVENANCE_V2_FIXTURE, components },
        COMPILER_PROVENANCE_V2_BUILD_ID,
      ),
    ).toEqual({ status: "invalid-recorded" });
  });

  it("keeps absent historical provenance separate from malformed values", () => {
    expect(
      classifyCompilerProvenance(undefined, "historical"),
    ).toEqual({ status: "legacy-unrecorded" });
    expect(classifyCompilerProvenance(null, "historical")).toEqual({
      status: "invalid-recorded",
    });
  });
});
