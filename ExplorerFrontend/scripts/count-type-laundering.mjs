import { readdir, readFile } from 'node:fs/promises';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import ts from 'typescript';

const root = fileURLToPath(new URL('..', import.meta.url));
const counts = { as: 0, angleAssertions: 0, nonNull: 0, any: 0, tsComments: 0, constAssertions: 0 };
async function scan(directory) {
  for (const entry of await readdir(directory, { withFileTypes: true })) {
    const file = path.join(directory, entry.name);
    if (entry.isDirectory()) {
      await scan(file);
      continue;
    }
    if (!/\.[cm]?[jt]sx?$/.test(file) || /\.(test|spec|fixture)\.|\.d\.ts$/.test(file)) continue;
    const source = await readFile(file, 'utf8');
    const ast = ts.createSourceFile(file, source, ts.ScriptTarget.Latest, true);
    function visit(node) {
      if (ts.isAsExpression(node))
        counts[node.type.getText(ast) === 'const' ? 'constAssertions' : 'as']++;
      if (ts.isTypeAssertionExpression(node)) counts.angleAssertions++;
      if (ts.isNonNullExpression(node)) counts.nonNull++;
      if (node.kind === ts.SyntaxKind.AnyKeyword) counts.any++;
      ts.forEachChild(node, visit);
    }
    visit(ast);
    counts.tsComments += (
      source.match(/@ts-(?:ignore|expect-error|nocheck|check)\b/g) ?? []
    ).length;
  }
}
await scan(path.join(root, 'app'));
console.log(JSON.stringify(counts, null, 2));
