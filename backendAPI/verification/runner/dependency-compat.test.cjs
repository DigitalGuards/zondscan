const assert = require('node:assert/strict');
const { spawnSync } = require('node:child_process');
const { createHash } = require('node:crypto');
const fs = require('node:fs');
const path = require('node:path');
const test = require('node:test');
const tmp = require('tmp');

test('patched tmp retains the compiler dependency cleanup API', () => {
  const file = tmp.fileSync({ postfix: '.smt2' });
  try {
    fs.writeFileSync(file.name, '(check-sat)');
    assert.equal(fs.readFileSync(file.name, 'utf8'), '(check-sat)');
    assert.equal(typeof file.fd, 'number');
  } finally {
    file.removeCallback();
  }
  assert.equal(fs.existsSync(file.name), false);

  const directory = tmp.dirSync({ unsafeCleanup: true, prefix: 'hypc-js-compiler-test-' });
  try {
    fs.writeFileSync(path.join(directory.name, 'fixture.txt'), 'compatibility');
  } finally {
    directory.removeCallback();
  }
  assert.equal(fs.existsSync(directory.name), false);
});

test('dependency patch preserves the historical compiler bytes and output', () => {
  // These pins identify this disabled historical runner, never the native
  // compiler accepted by the backend's sealed execution registry.
  const compiler = require('@theqrl/hypc');
  assert.equal(compiler.version(), '0.0.2+commit.3e18e55d.Emscripten.clang');
  const compilerBytes = fs.readFileSync(require.resolve('@theqrl/hypc/hypjson.js'));
  assert.equal(createHash('sha256').update(compilerBytes).digest('hex'),
    '302198e194a504e866b633652361048f468c8fa031a5e39f9d90d2c3030074aa');
  const input = {
    language: 'Hyperion',
    sources: {
      'CompatibilitySmoke.hyp': {
        content: 'contract CompatibilitySmoke { function value() public pure returns (uint256) { return 7; } }',
      },
    },
    settings: {
      optimizer: { enabled: false },
      outputSelection: { '*': { '*': ['abi', 'evm.bytecode.object', 'evm.deployedBytecode.object'] } },
    },
  };
  const result = spawnSync(process.execPath, [path.join(__dirname, 'hypc-runner.js')], {
    input: JSON.stringify(input), encoding: 'utf8', timeout: 10000, maxBuffer: 1024 * 1024,
  });
  assert.equal(result.error, undefined);
  assert.equal(result.status, 0, result.stderr);
  const output = JSON.parse(result.stdout);
  assert.equal((output.errors || []).some((error) => error.severity === 'error'), false);
  assert.equal(createHash('sha256').update(result.stdout).digest('hex'),
    '47627b5100fabeab219da28304c11bb6bbffef7fa2a612b8a47d2ec8b8fb8a79');
});
