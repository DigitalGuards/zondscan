import { describe, expect, it } from "@jest/globals";
import { renderToStaticMarkup } from "react-dom/server";

import CompilerProvenanceDetails, {
  classifyCompilerProvenance,
  nativeSandboxCompilerExecutionDigestV2,
} from "./CompilerProvenanceDetails";
import {
  VERIFICATION_RECORD_SCHEMA_V1,
  VERIFICATION_RECORD_SCHEMA_V2,
} from "../lib/storedVerification";
import {
  COMPILER_PROVENANCE_V2_BUILD_ID,
  COMPILER_PROVENANCE_V2_FIXTURE,
} from "../lib/compilerProvenanceV2.fixture";

const BUILD_ID = COMPILER_PROVENANCE_V2_BUILD_ID;
const provenance = COMPILER_PROVENANCE_V2_FIXTURE;
const EXECUTION_DIGEST = provenance.executionDigest;

describe("CompilerProvenanceDetails", () => {
  it("matches the backend schema-v2 execution digest test vector", () => {
    expect(
      nativeSandboxCompilerExecutionDigestV2(
        BUILD_ID,
        provenance.components[0].sha256,
        provenance.components[1].sha256,
        provenance.components[2].sha256,
      ),
    ).toBe(EXECUTION_DIGEST);
  });

  it("surfaces every digest-backed compiler identity field", () => {
    const html = renderToStaticMarkup(
      <CompilerProvenanceDetails
        verificationRecordSchema={VERIFICATION_RECORD_SCHEMA_V2}
        provenance={provenance}
        compilerVersion={BUILD_ID}
      />,
    );

    expect(html).toContain('data-provenance-status="digest-backed"');
    expect(html).toContain(">digest-backed<");
    expect(html).toContain(BUILD_ID);
    expect(html).toContain(EXECUTION_DIGEST);
    for (const component of provenance.components) {
      expect(html).toContain(component.sha256);
      expect(html).toContain(
        `aria-label="Copy ${component.name} SHA-256"`,
      );
    }
    expect(html).toContain('aria-label="Copy execution digest"');
    expect(html).not.toContain("legacy-unrecorded");
  });

  it("labels verified historical records with no provenance as legacy-unrecorded", () => {
    const html = renderToStaticMarkup(
      <CompilerProvenanceDetails
        provenance={undefined}
        compilerVersion="historical"
      />,
    );

    expect(html).toContain('data-provenance-status="legacy-unrecorded"');
    expect(html).toContain(">legacy-unrecorded<");
    expect(html).toContain("no recorded compiler artifact digest");
    expect(html).not.toContain(EXECUTION_DIGEST);
  });

  it("fails closed when recorded provenance conflicts with compilerVersion", () => {
    const state = classifyCompilerProvenance(provenance, "different-build");
    const html = renderToStaticMarkup(
      <CompilerProvenanceDetails
        verificationRecordSchema={VERIFICATION_RECORD_SCHEMA_V2}
        provenance={provenance}
        compilerVersion="different-build"
      />,
    );

    expect(state).toEqual({ status: "invalid-recorded" });
    expect(html).toContain('data-provenance-status="invalid-recorded"');
    expect(html).toContain(">invalid-recorded<");
    expect(html).not.toContain(EXECUTION_DIGEST);
  });

  it("fails closed when compilerVersion is absent", () => {
    expect(classifyCompilerProvenance(provenance)).toEqual({
      status: "invalid-recorded",
    });
  });

  it("treats explicit null provenance as a malformed recorded value", () => {
    expect(classifyCompilerProvenance(null, BUILD_ID)).toEqual({
      status: "invalid-recorded",
    });
  });

  it("renders absent provenance under schema v1 as invalid", () => {
    const html = renderToStaticMarkup(
      <CompilerProvenanceDetails
        verificationRecordSchema={VERIFICATION_RECORD_SCHEMA_V1}
        provenance={undefined}
        compilerVersion={BUILD_ID}
      />,
    );

    expect(html).toContain('data-provenance-status="invalid-recorded"');
    expect(html).toContain("text-error");
  });

  it("fails closed when the execution digest does not bind the recorded fields", () => {
    const mismatched = { ...provenance, executionDigest: "c".repeat(64) };

    expect(classifyCompilerProvenance(mismatched, BUILD_ID)).toEqual({
      status: "invalid-recorded",
    });
  });
});
