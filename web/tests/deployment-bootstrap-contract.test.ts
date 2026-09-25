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
  writeFileSync(legacyFile, Buffer.from('SQLite format 3\0fixture'));

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
  writeFileSync(legacyFile, Buffer.from('SQLite format 3\0fixture'));

  const result = spawnSync('bash', [helperURL.pathname, runtimeConfig, templateConfig, legacyFile], { encoding: 'utf8' });

  assert.equal(result.status, 0, result.stderr);
  assert.match(readFileSync(runtimeConfig, 'utf8'), /legacy_sqlite_path: \/custom\/archive\.db/);
});

test('deployment rejects a non-SQLite legacy snapshot before starting containers', (t) => {
  const helperURL = new URL('../../deploy/prepare-runtime-config.sh', import.meta.url);
  const fixture = mkdtempSync(join(tmpdir(), 'well-ambient-runtime-invalid-'));
  const runtimeConfig = join(fixture, 'config.yaml');
  const templateConfig = join(fixture, 'template.yaml');
  const legacyFile = join(fixture, 'well-ambient.db');
  t.after(() => rmSync(fixture, { recursive: true, force: true }));

  writeFileSync(templateConfig, 'database:\n  driver: setup\n');
  writeFileSync(legacyFile, 'incomplete upload');
  const result = spawnSync('bash', [helperURL.pathname, runtimeConfig, templateConfig, legacyFile], {
    encoding: 'utf8',
  });

  assert.notEqual(result.status, 0);
  assert.match(result.stderr, /not a valid SQLite snapshot/);
});

test('deploy verifies the mounted snapshot through the real setup status endpoint', () => {
  assert.match(deployment, /legacy_snapshot="\$runtime_dir\/data\/legacy\/well-ambient\.db"/);
  assert.match(
    deployment,
    /if \[\[ "\$database_driver" == "setup" \]\]; then[\s\S]*?up -d --force-recreate --wait --wait-timeout 240 server web/,
  );
  assert.match(deployment, /\/api\/setup\/status/);
  assert.match(deployment, /legacy_sqlite[^\n]+available/);
  assert.match(deployment, /expected container identity/);
  assert.match(deployment, /sudo chown \$container_app_uid:\$container_app_gid/);
});

test('one-command local test entrypoint reuses the isolated setup service lifecycle', () => {
  const localScriptURL = new URL('../../scripts/dev-setup.sh', import.meta.url);
  const localScript = readFileSync(localScriptURL, 'utf8');

  assert.ok((statSync(localScriptURL).mode & 0o111) !== 0, 'scripts/dev-setup.sh must be executable');
  assert.match(localScript, /run_backend/);
  assert.match(makefile, /^dev-setup:\n\t\.\/scripts\/dev-setup\.sh$/m);
});

test('deploy script supports importing local SQLite snapshot via CLI flag, env var, or candidate auto-detection', () => {
  assert.match(deployment, /--legacy-sqlite/);
  assert.match(deployment, /WELL_AMBIENT_LEGACY_SQLITE/);
  assert.match(deployment, /import_legacy_sqlite_snapshot/);
  assert.match(deployment, /sqlite3[^\n]+\.backup/);
  assert.match(deployment, /chmod 0755/);
  assert.match(makefile, /LEGACY_SQLITE \?=/);
  assert.match(makefile, /SQLITE_PATH \?=/);
});
