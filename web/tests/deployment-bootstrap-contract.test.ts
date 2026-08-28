import assert from 'node:assert/strict';
import { mkdtempSync, mkdirSync, readFileSync, rmSync, statSync, writeFileSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { spawnSync } from 'node:child_process';
import test from 'node:test';

function repoFile(relativePath: string): string {
  return readFileSync(new URL(`../../${relativePath}`, import.meta.url), 'utf8');
}

const deployment = repoFile('deploy/deploy.sh');
const makefile = repoFile('Makefile');

test('deploy backfills the mounted legacy SQLite path into an old setup config', (t) => {
  const helperURL = new URL('../../deploy/prepare-runtime-config.sh', import.meta.url);
  const fixture = mkdtempSync(join(tmpdir(), 'well-ambient-runtime-config-'));
  const runtimeConfig = join(fixture, 'config.yaml');
  const templateConfig = join(fixture, 'template.yaml');
  const legacyFile = join(fixture, 'data', 'legacy', 'well-ambient.db');
  t.after(() => rmSync(fixture, { recursive: true, force: true }));

  writeFileSync(runtimeConfig, 'database:\n  driver: setup\n  auto_migrate: false\nserver:\n  port: 8080\n');
  writeFileSync(templateConfig, 'database:\n  driver: setup\n');
  mkdirSync(join(fixture, 'data', 'legacy'), { recursive: true });
  writeFileSync(legacyFile, 'sqlite fixture');

  const result = spawnSync(
    'bash',
    [helperURL.pathname, runtimeConfig, templateConfig, legacyFile],
    { encoding: 'utf8' },
  );

  assert.equal(result.status, 0, result.stderr);
  const prepared = readFileSync(runtimeConfig, 'utf8');
  assert.match(prepared, /^\s{2}legacy_sqlite_path:\s+\/var\/lib\/well-ambient\/legacy\/well-ambient\.db$/m);
  assert.equal((prepared.match(/^\s{2}legacy_sqlite_path:/gm) || []).length, 1);
  assert.match(deployment, /prepare-runtime-config\.sh/);
});

test('runtime preparation preserves an explicitly configured legacy SQLite path', (t) => {
  const helperURL = new URL('../../deploy/prepare-runtime-config.sh', import.meta.url);
  const fixture = mkdtempSync(join(tmpdir(), 'well-ambient-runtime-custom-'));
  const runtimeConfig = join(fixture, 'config.yaml');
  const templateConfig = join(fixture, 'template.yaml');
  const legacyFile = join(fixture, 'data', 'legacy', 'well-ambient.db');
  t.after(() => rmSync(fixture, { recursive: true, force: true }));

  writeFileSync(runtimeConfig, 'database:\n  driver: setup\n  legacy_sqlite_path: /custom/archive.db\n');
  writeFileSync(templateConfig, 'database:\n  driver: setup\n');
  mkdirSync(join(fixture, 'data', 'legacy'), { recursive: true });
  writeFileSync(legacyFile, 'sqlite fixture');

  const result = spawnSync('bash', [helperURL.pathname, runtimeConfig, templateConfig, legacyFile], { encoding: 'utf8' });

  assert.equal(result.status, 0, result.stderr);
  assert.match(readFileSync(runtimeConfig, 'utf8'), /legacy_sqlite_path: \/custom\/archive\.db/);
});

test('one-command local test entrypoint reuses the isolated setup service lifecycle', () => {
  const localScriptURL = new URL('../../scripts/dev.sh', import.meta.url);
  const localScript = readFileSync(localScriptURL, 'utf8');

  assert.ok((statSync(localScriptURL).mode & 0o111) !== 0, 'scripts/dev.sh must be executable');
  assert.match(localScript, /exec "\$project_root\/scripts\/dev-setup\.sh" "\$@"/);
  assert.match(makefile, /^dev:\n\t\.\/scripts\/dev\.sh$/m);
});
