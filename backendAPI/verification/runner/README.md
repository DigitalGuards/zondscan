# hypc verification runners

The backend verifier (`backendAPI/verification/`) compiles submitted contract
source with a pinned [`@theqrl/hypc`](https://www.npmjs.com/package/@theqrl/hypc)
build and byte-matches the result against the on-chain runtime code. The
compile itself runs in a one-shot subprocess so the Go layer keeps the
timeout / concurrency / stdin-size budget and the runner stays tiny.

## Runner wire contract

Both runners speak the same contract, so the Go side can swap between them by
config alone:

| Invocation | stdin | stdout |
| --- | --- | --- |
| `<runner> --version` | — | a single line: the exact hypc build id (no prefix) |
| `<runner>` | Hyperion standard-JSON | hypc standard-JSON output |

- **`hypc-runner.js`** — wraps the `@theqrl/hypc` WASM via Node. Invoke with
  `nodeBin: "node"`. This is the npm-published build (currently only `0.0.2`).
- **`hypc-native.sh`** — wraps a natively-built `hypc` binary. Invoke with
  `nodeBin: "/bin/sh"` and set `bin` to the absolute path of the `hypc`
  binary (exported to the runner as `HYPC_BIN`). Use this for builds that
  aren't on npm, e.g. a `0.2.0-develop` snapshot built from
  [`theqrl/hyperion`](https://github.com/theqrl/hyperion).

## Configuring the builds

The verifier supports **multiple** builds; a submission picks one via
`compilerVersion` (the default build is used when omitted).

### Preferred: `HYPC_COMPILERS` manifest

Set `HYPC_COMPILERS` to either an inline JSON array or a path to a JSON file
of `CompilerSpec` entries. See [`compilers.example.json`](./compilers.example.json).

The manifest is parsed with Go's standard `encoding/json`, so it must be
**strict JSON — no comments, no trailing commas**:

```json
[
  {
    "buildId": "0.2.0-develop.2026.4.13+commit.d5d1b977.Linux.g++",
    "nodeBin": "/bin/sh",
    "runner": "/abs/path/to/hypc-native.sh",
    "bin": "/usr/local/bin/hypc-0.2.0",
    "default": true
  },
  {
    "buildId": "0.0.2+commit.3e18e55d.Emscripten.clang",
    "runner": "/abs/path/to/hypc-runner.js"
  }
]
```

Per-entry fields:
- `buildId` *(required)* — the version string the runner must report from `--version`; the build is skipped if it reports anything else.
- `runner` *(required)* — absolute path to `hypc-runner.js` (WASM) or `hypc-native.sh` (native).
- `nodeBin` *(optional, default `"node"`)* — interpreter; use `/bin/sh` for the native wrapper.
- `bin` *(optional)* — native hypc path, exported to the runner as `HYPC_BIN`.
- `default` *(optional)* — the build used when a submission omits `compilerVersion`; if none is flagged, the first surviving build wins.

```bash
# file path
HYPC_COMPILERS=/home/ops/zondscan/backendAPI/verification/runner/compilers.json
# …or inline
HYPC_COMPILERS='[{"buildId":"0.0.2+commit.3e18e55d.Emscripten.clang","runner":"/abs/hypc-runner.js"}]'
```

Each build is probed at startup (`<runner> --version`) and must report
exactly its `buildId`. A build that fails the probe is skipped with a warning
(the others stay live); verification only boots disabled if **none** survive,
in which case the `/contract/*` endpoints answer `503`.

### Legacy: single-build env vars (still supported)

When `HYPC_COMPILERS` is unset, a one-entry registry is synthesised from the
original env vars, so existing single-build deploys keep working untouched:

```bash
HYPC_NODE_BIN=node                                          # default "node"
HYPC_RUNNER=/abs/path/to/hypc-runner.js                     # required
HYPC_BUILD_ID=0.0.2+commit.3e18e55d.Emscripten.clang        # required
HYPC_BIN=/usr/local/bin/hypc                                # optional (native runner)
```

### Shared limits (apply to all builds)

```bash
VERIFIER_MAX_CONCURRENCY=2     # total concurrent compiles across ALL builds
VERIFIER_COMPILE_TIMEOUT=30s   # per-compile hard deadline
VERIFIER_SOURCE_MAX_BYTES=262144   # standard-JSON payload cap (256 KiB)
```
