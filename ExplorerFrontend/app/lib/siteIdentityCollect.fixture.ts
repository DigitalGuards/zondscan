/**
 * Test helper: loads every page module under app/ and returns its Next.js
 * metadata (static `metadata` export, or `generateMetadata` called with
 * sample params and a failing fetch). Used by siteIdentity.test.ts to pin
 * the default output and to scan the alternate build for the old brand.
 */
import fs from 'node:fs';
import path from 'node:path';

const appDir = path.resolve(__dirname, '..');

const SAMPLE_PARAMS = {
  query: '1',
  id: '1',
  hash: `0x${'a'.repeat(64)}`,
};

function walk(dir: string, out: string[]): void {
  for (const entry of fs.readdirSync(dir, { withFileTypes: true })) {
    const full = path.join(dir, entry.name);
    if (entry.isDirectory()) {
      if (entry.name === 'node_modules' || entry.name === 'api') continue;
      walk(full, out);
    } else if (/^page\.tsx?$/.test(entry.name)) {
      out.push(full);
    }
  }
}

export function pageFiles(): string[] {
  const files: string[] = [];
  walk(appDir, files);
  return files.sort();
}

export async function collectPageMetadata(): Promise<Record<string, unknown>> {
  const result: Record<string, unknown> = {};
  const realFetch = global.fetch;
  global.fetch = () => Promise.reject(new Error('offline'));
  try {
    for (const file of pageFiles()) {
      const rel = path.relative(appDir, file).split(path.sep).join('/');
      try {
        const mod = (await import(file)) as {
          metadata?: unknown;
          generateMetadata?: (props: unknown) => unknown;
        };
        if (mod.metadata) {
          result[rel] = mod.metadata;
        } else if (mod.generateMetadata) {
          result[rel] = await mod.generateMetadata({
            params: Promise.resolve(SAMPLE_PARAMS),
            searchParams: Promise.resolve({}),
          });
        }
      } catch (error) {
        result[rel] = `ERROR: ${(error as Error).message}`;
      }
    }
  } finally {
    global.fetch = realFetch;
  }
  return JSON.parse(JSON.stringify(result)) as Record<string, unknown>;
}
