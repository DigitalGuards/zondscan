#!/usr/bin/env node
// hypc-runner: thin Node entry-point invoked by the Go backend's
// verification pipeline. Reads a Hyperion standard-JSON input from stdin
// (or a single source file path via --file plus a synthesised wrapper) and
// emits the compiler's JSON output to stdout. Exits non-zero only on
// runner-internal failure (compile errors are reported as a JSON output
// with `errors` populated, exit 0).
//
// Invocation modes:
//   echo '<standard-json>' | node hypc-runner.js
//   node hypc-runner.js --version          → prints the pinned compiler ID
//
// The Go layer enforces timeouts, concurrency caps, source-size caps,
// and the no-network policy via exec.CommandContext + cgroups/rlimits;
// this script just wraps @theqrl/hypc as cleanly as possible.

const hypc = require('@theqrl/hypc');

if (process.argv.includes('--version')) {
  process.stdout.write(hypc.version() + '\n');
  process.exit(0);
}

let stdin = '';
process.stdin.setEncoding('utf8');
process.stdin.on('data', (c) => { stdin += c; });
process.stdin.on('end', () => {
  let input;
  try {
    input = JSON.parse(stdin);
  } catch (e) {
    process.stderr.write('hypc-runner: invalid stdin JSON: ' + e.message + '\n');
    process.exit(2);
  }
  if (!input || input.language !== 'Hyperion') {
    process.stderr.write('hypc-runner: input.language must be "Hyperion"\n');
    process.exit(2);
  }

  // Resolve imports STRICTLY from the input.sources map, no filesystem,
  // no network. The Go caller is responsible for inlining every dependency
  // before invoking the runner; anything not present in `sources` is a
  // hard error.
  const sources = input.sources || {};
  function findImports(p) {
    if (Object.prototype.hasOwnProperty.call(sources, p)) {
      return { contents: sources[p].content };
    }
    return { error: 'hypc-runner: import not found: ' + p };
  }

  let out;
  try {
    out = hypc.compile(stdin, { import: findImports });
  } catch (e) {
    process.stderr.write('hypc-runner: compile threw: ' + (e && e.stack ? e.stack : e) + '\n');
    process.exit(3);
  }

  process.stdout.write(out);
  process.exit(0);
});
