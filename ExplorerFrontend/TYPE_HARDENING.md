# TypeScript hardening

`npm run build` runs lint and typecheck in its prebuild hook. The repository CI already runs this command for ExplorerFrontend, so these checks are required without changing another package or the repository workflow.

The flat ESLint config enables the type-aware `strict-type-checked` preset. Type assertions, explicit `any`, non-null assertions, and all TypeScript comment directives are errors. Const assertions are allowed. Inline ESLint configuration is disabled. Test files have an explicit override for assertions, explicit `any`, and non-null assertions. Numeric template expressions follow the shared reference configuration.

Untrusted arrays must pass through `isArray`, which exposes unknown elements. Axios calls must request an unknown response and validate it before use. The new guards reject malformed input or drop invalid entries. The contract ABI codec accepts the guarded types directly, so this package requires no library assertion wrapper.

## Compiler flags

Enabled package-wide: `strict`, `noImplicitOverride`, `noImplicitReturns`, `noFallthroughCasesInSwitch`, `noUnusedLocals`, and `noUnusedParameters`.

Two flags remain deferred. Measured independently with the final source and the other enabled flags, `noUncheckedIndexedAccess` produces 91 diagnostics and `exactOptionalPropertyTypes` produces 46. Their initial counts were 98 and 47. They need further migration across UI props, indexed accesses, and test fixtures.

## Remaining lint debt

`eslint-ratchet.json` is generated from the full ruleset and records exact file/rule/count entries. It contains 221 pairs across 105 files, totaling 593 findings. The flat config exposes these pairs as warnings for editors. `npm run lint` independently checks the full error ruleset and rejects new pairs, increased counts, parsing failures, inline suppression comments, and stale baseline entries. New TypeScript files receive the full ruleset.

After removing findings, run `npm run lint:ratchet` and review the JSON diff. CI and normal lint/build commands never regenerate this file. Increasing baseline counts requires explicit review.

The remaining production assertions are the two casts in `app/address/[query]/address-page-data.ts` that convert a legacy address aggregate to `AddressData`. That response still needs complete runtime validation. Other legacy HTTP consumers and strict lint issues are recorded in the ratchet. Compiler flag migration and this baseline are ongoing debt.

Run `node scripts/count-type-laundering.mjs` to reproduce the application source inventory. Tests, fixtures, declaration files, import aliases, and JSX attributes are excluded. Const assertions are reported separately.

| Construct                     | Before | After |
| ----------------------------- | -----: | ----: |
| `as` assertions               |     71 |     2 |
| Angle assertions              |      0 |     0 |
| Non-null assertions           |     15 |     0 |
| Explicit `any`                |      7 |     0 |
| TypeScript comment directives |      2 |     0 |
| Allowed const assertions      |     34 |    34 |

## Runtime changes and verification

Runtime guards cover preferences, exchange rates, faucet request/status/claim/signing data, selected list and homepage responses, ABI input and read responses, compiler provenance, imported source maps, and wallet account data. Malformed input fails closed. Valid transaction fields and encoding stay unchanged. Tests assert the exact faucet signing object, the signed bytes supplied to broadcast, VM64 calldata, and malformed input rejection. Unsafe nonces and malformed persisted faucet amounts are rejected before signing.

ESLint is pinned to 9.39.5 because the installed Next plugins support ESLint 9 and ESLint 10.4 failed with the type-aware parser. The package uses flat configuration. This tooling compatibility pin can be revisited alongside a coordinated plugin/parser upgrade.

The package previously had no format-check script. Prettier checks were applied to the changed files with single quotes, ES5 trailing commas, and a print width of 100. Existing API drift, unit-test, and browser-test scripts remain available.

Local verification blocks network access. The build uses the existing `SKIP_DAPP_EXAMPLE=1` option and local font fixtures with webpack. This verifies frontend compilation without fetching the separately maintained dApp example or Google Fonts. Browser tests require fixture HTTP connections and cannot complete under that restriction. Live wallet pairing and services were not contacted.

| Check                                        | Exit code | Result                                                  |
| -------------------------------------------- | --------: | ------------------------------------------------------- |
| `npm ci --no-audit --no-fund`                |         0 | Initial install and final clean lockfile install        |
| Prettier check of changed files              |         0 | Single quotes, ES5 trailing commas, print width 100     |
| `npm run lint`                               |         0 | Exact ratchet counts match                              |
| `npm run typecheck -- --incremental false`   |         0 | Enabled compiler flags pass                             |
| `npm test -- --runInBand`                    |         0 | 706 tests across 54 suites                              |
| `npm run check:api-drift`                    |         0 | 47 backend and 3 frontend routes documented             |
| `npm run build -- --webpack`                 |         0 | Offline font fixtures and dApp example skip             |
| `npm run test:e2e -- --global-timeout=15000` |         1 | Network block stops fixture HTTP setup before tests run |

Negative lint probes returned exit code 1 for both a new assertion in a new file and an added assertion in the existing legacy file. Both probe changes were removed. Effective configuration checks confirmed the required rules have error severity for new TypeScript files and the legacy override preserves `assertionStyle: never`.
