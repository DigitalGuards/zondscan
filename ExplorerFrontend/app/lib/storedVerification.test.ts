import { describe, expect, it } from "@jest/globals";

import {
  classifyStoredVerification,
  sourceBundleDigestV1,
  verificationArtifactDigestV2,
  VERIFICATION_ARTIFACT_DIGEST_PREFIX_V2,
  VERIFICATION_RECORD_SCHEMA_V1,
  VERIFICATION_RECORD_SCHEMA_V2,
} from "./storedVerification";
import {
  STORED_VERIFICATION_V2_BASE,
  STORED_VERIFICATION_V2_FIXTURE,
} from "./storedVerificationV2.fixture";

describe("sourceBundleDigestV1", () => {
  it("matches the backend Unicode and trailing-newline framing vector", () => {
    const imports = {
      "src/βeta/Math.hyp": "library Math { /* π */ }\n",
      "src/Ångström/Types.hyp": "struct Café { uint256 value; }\n",
    };

    expect(
      sourceBundleDigestV1(
        "München",
        'contract München {\n    string public greeting = "你好";\n}\n',
        imports,
      ),
    ).toBe(
      "qrl.verified-source-bundle.v1:sha256:f8a35959e8d3ba5e9e55b0a563d5563db22b98f9486c9557e84d353ac8524202",
    );
  });
});

describe("verificationArtifactDigestV2", () => {
  it("matches the backend framing vector", () => {
    expect(
      verificationArtifactDigestV2(STORED_VERIFICATION_V2_BASE),
    ).toBe(
      "qrl.contract-verification-artifact.v2:sha256:cf2043bd0715e9b395aee59b3b5cde6ccea131b2f72cd348e980b0d9b541c944",
    );
  });

  it("uses the deployment-bound V2 digest namespace", () => {
    expect(
      verificationArtifactDigestV2(STORED_VERIFICATION_V2_BASE),
    ).toMatch(
      new RegExp(
        `^${VERIFICATION_ARTIFACT_DIGEST_PREFIX_V2.replaceAll(".", "\\.")}[0-9a-f]{64}$`,
      ),
    );
  });

  it("sorts library keys using backend-compatible UTF-8 ordering", () => {
    expect(
      verificationArtifactDigestV2({
        ...STORED_VERIFICATION_V2_BASE,
        libraries: { A: "Qa", Z: "Qz" },
      }),
    ).toBe(STORED_VERIFICATION_V2_FIXTURE.verificationArtifactDigest);
  });

  it("supports the canonical genesis target encoding", () => {
    const genesis = {
      ...STORED_VERIFICATION_V2_BASE,
      creationTransaction: "",
      creationBlockNumber: "0x0",
      genesisContract: true,
    };
    const verificationArtifactDigest = verificationArtifactDigestV2(genesis);

    expect(verificationArtifactDigest).not.toBeNull();
    expect(
      classifyStoredVerification({ ...genesis, verificationArtifactDigest }),
    ).toBe("digest-backed");
  });
});

describe("classifyStoredVerification", () => {
  it("accepts a complete deployment-bound schema-v2 record", () => {
    expect(classifyStoredVerification(STORED_VERIFICATION_V2_FIXTURE)).toBe(
      "digest-backed",
    );
  });

  it("keeps a complete schema-v1 record visible as legacy and untrusted", () => {
    const {
      verificationArtifactDigest: _artifactDigest,
      ...withoutArtifactDigest
    } = STORED_VERIFICATION_V2_FIXTURE;

    expect(
      classifyStoredVerification({
        ...withoutArtifactDigest,
        verificationRecordSchema: VERIFICATION_RECORD_SCHEMA_V1,
      }),
    ).toBe("legacy-unrecorded");
  });

  it("keeps a genuinely absent historical record in the legacy state", () => {
    expect(classifyStoredVerification({ sourceCode: "contract Old {}" })).toBe(
      "legacy-unrecorded",
    );
  });

  it.each([null, "", "qrl.contract-verification-record.v3"])(
    "rejects a present malformed or unknown record schema: %p",
    (verificationRecordSchema) => {
      expect(
        classifyStoredVerification({
          verificationRecordSchema,
          compilerProvenance: null,
          sourceBundleDigest: null,
        }),
      ).toBe("invalid-recorded");
    },
  );

  it("rejects modern trust fields when the record marker is absent", () => {
    const { verificationRecordSchema: _schema, ...unmarked } =
      STORED_VERIFICATION_V2_FIXTURE;
    expect(classifyStoredVerification(unmarked)).toBe("invalid-recorded");
  });

  it("rejects a V1 marker carrying a V2 artifact digest", () => {
    expect(
      classifyStoredVerification({
        ...STORED_VERIFICATION_V2_FIXTURE,
        verificationRecordSchema: VERIFICATION_RECORD_SCHEMA_V1,
      }),
    ).toBe("invalid-recorded");
  });

  it("rejects null or missing V2 trust fields", () => {
    expect(
      classifyStoredVerification({
        ...STORED_VERIFICATION_V2_FIXTURE,
        compilerProvenance: null,
        sourceBundleDigest: null,
        verificationArtifactDigest: null,
      }),
    ).toBe("invalid-recorded");
    expect(
      classifyStoredVerification({
        verificationRecordSchema: VERIFICATION_RECORD_SCHEMA_V2,
      }),
    ).toBe("invalid-recorded");
  });

  it.each([
    ["address", `Q${"f".repeat(128)}`],
    ["creationTransaction", `0x${"f".repeat(64)}`],
    ["creationBlockNumber", "0x11"],
    ["creationBlockHash", `0x${"f".repeat(64)}`],
    ["chainId", "0x53a"],
    ["contractCodeSha256", "f".repeat(64)],
    ["abi", '[{"type":"constructor"}]'],
    ["optimizationEnabled", false],
    ["optimizationRuns", 201],
    ["evmVersion", "cancun"],
    ["constructorArguments", "00"],
    ["libraries", { A: "Qchanged", Z: "Qz" }],
    ["license", "MIT"],
  ] as const)("rejects an artifact whose %s field changed", (field, value) => {
    expect(
      classifyStoredVerification({
        ...STORED_VERIFICATION_V2_FIXTURE,
        [field]: value,
      }),
    ).toBe("invalid-recorded");
  });

  it.each([
    ["noncanonical address", { address: `Q${"A".repeat(128)}` }],
    ["noncanonical block quantity", { creationBlockNumber: "0x010" }],
    ["unsafe optimizer count", { optimizationRuns: Number.MAX_VALUE }],
    ["unknown verification method", { verificationMethod: "partial" }],
    ["malformed libraries", { libraries: { A: 7 } }],
  ])("rejects %s", (_label, override) => {
    expect(
      classifyStoredVerification({
        ...STORED_VERIFICATION_V2_FIXTURE,
        ...override,
      }),
    ).toBe("invalid-recorded");
  });
});
