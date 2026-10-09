import { readFile, writeFile } from 'node:fs/promises';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { ESLint } from 'eslint';

const root = fileURLToPath(new URL('..', import.meta.url));
const baselinePath = path.join(root, 'eslint-ratchet.json');
const update = process.argv.includes('--update');
// Always measure against the full ruleset. The baseline only allows exact counts.
process.env.ESLINT_RATCHET = 'off';
const eslint = new ESLint({ cwd: root });
const results = await eslint.lintFiles(['.']);
const current = {};
const fatal = [];
for (const result of results) {
  const file = path.relative(root, result.filePath).split(path.sep).join('/');
  for (const message of result.messages) {
    if (!message.ruleId || message.fatal) {
      fatal.push({ ...result, messages: [message] });
      continue;
    }
    current[file] ??= {};
    current[file][message.ruleId] = (current[file][message.ruleId] ?? 0) + 1;
  }
}
const sorted = Object.fromEntries(
  Object.entries(current)
    .sort()
    .map(([file, rules]) => [file, Object.fromEntries(Object.entries(rules).sort())])
);
if (fatal.length) {
  console.error(await (await eslint.loadFormatter('stylish')).format(fatal));
  process.exitCode = 1;
} else if (update) {
  // Regeneration is explicit and reviewable; lint and build never modify the baseline.
  await writeFile(baselinePath, JSON.stringify(sorted, null, 2) + '\n');
  console.log('Updated eslint-ratchet.json. Review every added file/rule pair.');
} else {
  const baseline = JSON.parse(await readFile(baselinePath, 'utf8'));
  let failed = false;
  for (const file of new Set([...Object.keys(baseline), ...Object.keys(sorted)])) {
    const expected = baseline[file] ?? {};
    const actual = sorted[file] ?? {};
    for (const rule of new Set([...Object.keys(expected), ...Object.keys(actual)])) {
      const before = expected[rule] ?? 0;
      const after = actual[rule] ?? 0;
      if (after !== before) {
        console.error(`${file}: ${rule}: ${after} findings (baseline ${before})`);
        failed = true;
      }
    }
  }
  if (failed) {
    const regressions = results
      .map((result) => ({
        ...result,
        messages: result.messages.filter((message) => {
          const file = path.relative(root, result.filePath).split(path.sep).join('/');
          return (sorted[file]?.[message.ruleId] ?? 0) > (baseline[file]?.[message.ruleId] ?? 0);
        }),
      }))
      .filter((result) => result.messages.length);
    console.error(await (await eslint.loadFormatter('stylish')).format(regressions));
    console.error('Fix new findings; use npm run lint:ratchet to record removed debt.');
    process.exitCode = 1;
  } else {
    const pairs = Object.values(sorted).reduce((sum, rules) => sum + Object.keys(rules).length, 0);
    const findings = Object.values(sorted)
      .flatMap(Object.values)
      .reduce((sum, count) => sum + count, 0);
    console.log(`Lint passed: ${findings} existing findings in ${pairs} exact file/rule pairs.`);
  }
}
