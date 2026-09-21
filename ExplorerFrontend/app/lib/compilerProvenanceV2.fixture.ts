import { readFileSync } from "node:fs";
import { resolve } from "node:path";

import type { CompilerProvenance } from "../types/address";

const fixture = JSON.parse(
  readFileSync(
    resolve(
      process.cwd(),
      "..",
      "backendAPI",
      "models",
      "testdata",
      "compiler_provenance_v2.json",
    ),
    "utf8",
  ),
) as {
  compilerVersion: string;
  provenance: CompilerProvenance;
};

export const COMPILER_PROVENANCE_V2_BUILD_ID = fixture.compilerVersion;
export const COMPILER_PROVENANCE_V2_FIXTURE = fixture.provenance;
