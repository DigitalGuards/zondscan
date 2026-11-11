import { describe, expect, it } from "@jest/globals";

import {
  classifyStoredVerification,
  requiresVerificationRecordSchemaV3,
  sourceBundleDigestV1,
  verificationArtifactDigestV2,
  verificationArtifactDigestV3,
  VERIFICATION_RECORD_SCHEMA_V1,
  VERIFICATION_RECORD_SCHEMA_V2,
  VERIFICATION_RECORD_SCHEMA_V3,
} from "./storedVerification";
import { STORED_VERIFICATION_V2_FIXTURE } from "./storedVerificationV2.fixture";
import {
  STORED_VERIFICATION_V3_VECTORS,
  storedVerificationV3Vector,
} from "./storedVerificationV3.fixture";

describe("verificationArtifactDigestV3", () => {
  it.each(STORED_VERIFICATION_V3_VECTORS.map((vector, index) => [vector.name, index] as const))(
    "matches the backend vector: %s",
    (_name, index) => {
      const record = storedVerificationV3Vector(index);
      expect(record.verificationRecordSchema).toBe(VERIFICATION_RECORD_SCHEMA_V3);
      expect(
        sourceBundleDigestV1(record.contractName ?? "", record.sourceCode ?? "", record.imports ?? {}),
      ).toBe(record.sourceBundleDigest);
      expect(verificationArtifactDigestV3(record)).toBe(record.verificationArtifactDigest);
      expect(classifyStoredVerification(record)).toBe("digest-backed");
    },
  );

  it("is only defined for schema-v3 records", () => {
    expect(verificationArtifactDigestV3(STORED_VERIFICATION_V2_FIXTURE)).toBeNull();
    expect(verificationArtifactDigestV2(storedVerificationV3Vector(0))).toBeNull();
  });
});

describe("classifyStoredVerification for schema v3", () => {
  it.each([
    ["via-IR dropped", { viaIR: false }],
    ["primary path moved", { primarySourcePath: "contracts/Counter.hyp" }],
    ["primary path dropped", { primarySourcePath: undefined }],
    ["schema lowered to v2", { verificationRecordSchema: VERIFICATION_RECORD_SCHEMA_V2 }],
    ["optimizer runs changed", { optimizationRuns: 201 }],
    ["malformed via-IR flag", { viaIR: "true" }],
    ["malformed primary path", { primarySourcePath: 7 }],
  ] as const)("rejects a record whose %s", (_label, override) => {
    expect(
      classifyStoredVerification({ ...storedVerificationV3Vector(0), ...override }),
    ).toBe("invalid-recorded");
  });

  it("rejects a v2 digest namespace on a v3 record", () => {
    const record = storedVerificationV3Vector(0);
    record.verificationArtifactDigest = record.verificationArtifactDigest.replace(".v3:", ".v2:");
    expect(classifyStoredVerification(record)).toBe("invalid-recorded");
  });

  it("rejects the default primary path spelled out", () => {
    const record = storedVerificationV3Vector(1);
    expect(record.primarySourcePath).toBeUndefined();
    expect(record.viaIR).toBe(true);
    expect(
      classifyStoredVerification({ ...record, primarySourcePath: `${record.contractName}.hyp` }),
    ).toBe("invalid-recorded");
  });

  it("rejects a v3 record that carries neither v3 input", () => {
    const record = { ...storedVerificationV3Vector(1), viaIR: false };
    const verificationArtifactDigest = verificationArtifactDigestV3(record);
    expect(verificationArtifactDigest).not.toBeNull();
    expect(classifyStoredVerification({ ...record, verificationArtifactDigest })).toBe(
      "invalid-recorded",
    );
  });

  it.each([
    ["via-IR", { viaIR: true }],
    ["primary path", { primarySourcePath: "src/Main.hyp" }],
  ] as const)("rejects %s on a v2 record", (_label, override) => {
    expect(classifyStoredVerification({ ...STORED_VERIFICATION_V2_FIXTURE, ...override })).toBe(
      "invalid-recorded",
    );
  });

  it("rejects v3 inputs on a v1 record", () => {
    const v1 = {
      ...STORED_VERIFICATION_V2_FIXTURE,
      verificationRecordSchema: VERIFICATION_RECORD_SCHEMA_V1,
      verificationArtifactDigest: undefined,
    };
    expect(classifyStoredVerification(v1)).toBe("legacy-unrecorded");
    expect(classifyStoredVerification({ ...v1, viaIR: true })).toBe("invalid-recorded");
  });

  it("keeps v2 records that spell out absent v3 inputs as false or empty", () => {
    expect(
      classifyStoredVerification({
        ...STORED_VERIFICATION_V2_FIXTURE,
        primarySourcePath: "",
        viaIR: false,
      }),
    ).toBe("digest-backed");
  });
});

describe("requiresVerificationRecordSchemaV3", () => {
  it.each([
    ["Main", "", false, false],
    ["Main", "Main.hyp", false, false],
    ["Main", "src/Main.hyp", false, true],
    ["Main", "", true, true],
  ] as const)("(%s, %p, %p) is %p", (name, path, viaIR, want) => {
    expect(requiresVerificationRecordSchemaV3(name, path, viaIR)).toBe(want);
  });
});
