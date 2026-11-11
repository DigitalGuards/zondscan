import { readFileSync } from "node:fs";
import { resolve } from "node:path";

import type { ContractData } from "../types/address";

interface StoredVerificationV3Vector {
  name: string;
  record: Partial<ContractData> & { verificationArtifactDigest: string };
}

/**
 * Schema-v3 records shared with the Go digest tests in
 * backendAPI/models/testdata, so both implementations prove the same bytes.
 */
const fixture = JSON.parse(
  readFileSync(
    resolve(
      process.cwd(),
      "..",
      "backendAPI",
      "models",
      "testdata",
      "verification_record_v3.json",
    ),
    "utf8",
  ),
) as { vectors: StoredVerificationV3Vector[] };

export const STORED_VERIFICATION_V3_VECTORS = fixture.vectors;

export function storedVerificationV3Vector(index: number): StoredVerificationV3Vector["record"] {
  const vector = fixture.vectors.at(index);
  if (vector === undefined) throw new Error(`missing v3 vector ${index}`);
  // A JSON round trip keeps plain objects in this realm; the classifier
  // rejects maps whose prototype is not Object.prototype.
  return JSON.parse(JSON.stringify(vector.record)) as StoredVerificationV3Vector["record"];
}
