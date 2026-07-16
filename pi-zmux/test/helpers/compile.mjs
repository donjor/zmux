import assert from 'node:assert/strict';
import { existsSync, mkdtempSync, symlinkSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { dirname, join } from 'node:path';
import { spawnSync } from 'node:child_process';

// Package root (pi-zmux/), resolved relative to this helper (test/helpers/).
export const packageRoot = new URL('../..', import.meta.url).pathname.replace(/\/$/, '');

export function findNodeModules(start, packageName) {
  let dir = start;
  while (dir !== dirname(dir)) {
    const candidate = join(dir, 'node_modules');
    if (existsSync(join(candidate, packageName))) return candidate;
    dir = dirname(dir);
  }
  throw new Error(`Could not find node_modules containing ${packageName} from ${start}`);
}

// Single source for the compile-to-tempdir bootstrap shared by test/run.mjs and
// test/dispatcher.mjs: emit the TypeScript project into a throwaway directory and
// symlink node_modules so the compiled ESM output can resolve its dependencies.
export function compileProject(options = {}) {
  const root = options.root ?? packageRoot;
  const outDir = options.outDir ?? mkdtempSync(join(tmpdir(), 'pi-zmux-compile-'));
  const tsc = join(root, 'node_modules/.bin/tsc');
  const result = spawnSync(tsc, ['-p', join(root, 'tsconfig.json'), '--outDir', outDir, '--noEmit', 'false'], { stdio: 'inherit' });
  assert.equal(result.status, 0, 'TypeScript compile failed');
  const nodeModules = findNodeModules(root, 'typebox');
  symlinkSync(nodeModules, join(outDir, 'node_modules'), 'dir');
  return { root, outDir };
}
